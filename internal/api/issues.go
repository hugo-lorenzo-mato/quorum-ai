package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
)

// Error/message constants for duplicated strings (S1192).
const (
	errInvalidRequestBody = "invalid request body: %s"
	errReadDraftsFailed   = "failed to read drafts: %v"
	msgIssuesDisabled     = "issue generation is disabled in configuration"
	msgReadIssuesFailed   = "reading generated issues failed"
)

// IssueInput represents a single issue to be created (from frontend edits).
type IssueInput struct {
	// Title is the issue title.
	Title string `json:"title"`

	// Body is the issue description.
	Body string `json:"body"`

	// Labels are the issue labels.
	Labels []string `json:"labels"`

	// Assignees are the issue assignees.
	Assignees []string `json:"assignees"`

	// IsMainIssue indicates if this is the main/parent issue.
	IsMainIssue bool `json:"is_main_issue"`

	// TaskID is the task identifier (optional).
	TaskID string `json:"task_id"`

	// FilePath is the markdown file path for this issue (optional).
	FilePath string `json:"file_path,omitempty"`
}

// GenerateIssuesRequest is the request body for generating issues.
type GenerateIssuesRequest struct {
	// DryRun previews issues without creating them.
	DryRun bool `json:"dry_run"`

	// CreateMainIssue creates a parent issue from consolidated analysis.
	CreateMainIssue bool `json:"create_main_issue"`

	// CreateSubIssues creates child issues for each task.
	CreateSubIssues bool `json:"create_sub_issues"`

	// LinkIssues links sub-issues to the main issue.
	LinkIssues bool `json:"link_issues"`

	// Labels overrides default labels.
	Labels []string `json:"labels,omitempty"`

	// Assignees overrides default assignees.
	Assignees []string `json:"assignees,omitempty"`

	// Issues contains edited issues from the frontend.
	// If provided, these will be created directly instead of reading from filesystem.
	Issues []IssueInput `json:"issues,omitempty"`
}

// GenerateIssuesResponse is the response for issue generation.
type GenerateIssuesResponse struct {
	// Success indicates if generation completed successfully.
	Success bool `json:"success"`

	// Message provides additional information.
	Message string `json:"message"`

	// MainIssue is the created/previewed main issue.
	MainIssue *IssueResponse `json:"main_issue,omitempty"`

	// SubIssues are the created/previewed sub-issues.
	SubIssues []IssueResponse `json:"sub_issues,omitempty"`

	// PreviewIssues contains previews in dry-run mode.
	PreviewIssues []IssuePreviewResponse `json:"preview_issues,omitempty"`

	// Errors contains non-fatal errors during generation.
	Errors []string `json:"errors,omitempty"`

	// AIUsed indicates whether AI generation was used (vs direct copy).
	AIUsed bool `json:"ai_used"`

	// AIErrors contains AI-specific errors for debugging.
	AIErrors []string `json:"ai_errors,omitempty"`
}

// SaveIssuesFilesRequest is the request body for saving issues to disk.
type SaveIssuesFilesRequest struct {
	Issues []IssueInput `json:"issues"`
}

// SaveIssuesFilesResponse is the response for saving issues to disk.
type SaveIssuesFilesResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Issues  []IssuePreviewResponse `json:"issues,omitempty"`
}

// IssueResponse represents a created issue.
type IssueResponse struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	State       string   `json:"state"`
	Labels      []string `json:"labels"`
	ParentIssue int      `json:"parent_issue,omitempty"`
}

// IssuePreviewResponse represents an issue preview.
type IssuePreviewResponse struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	IsMainIssue bool     `json:"is_main_issue"`
	TaskID      string   `json:"task_id,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
}

// handleGenerateIssues generates GitHub/GitLab issues from workflow artifacts.
// POST /api/workflows/{workflowID}/issues
//
//nolint:gocyclo // Handler orchestrates many validation and IO steps.
func (s *Server) handleGenerateIssues(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	// Get workflow state
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Parse request body
	var req GenerateIssuesRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf(errInvalidRequestBody, err.Error()))
			return
		}
	} else {
		req = GenerateIssuesRequest{
			CreateMainIssue: true,
			CreateSubIssues: true,
			LinkIssues:      true,
		}
	}

	// Get issues config from loader
	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	// Check if issues are enabled
	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, msgIssuesDisabled)
		return
	}

	// Determine report directory (cheap check before expensive client creation)
	reportDir := state.ReportPath
	if reportDir == "" {
		respondError(w, http.StatusBadRequest, "workflow has no report directory")
		return
	}

	// Create issue client based on provider
	issueClient, clientErr := createIssueClient(issuesCfg)
	if clientErr != nil {
		writeIssueClientError(w, clientErr)
		return
	}

	// Create generator with agent registry for LLM-based generation
	generator := issues.NewGenerator(issueClient, issuesCfg, "", reportDir, s.agentRegistry)
	generator.SetProgressReporter(newIssuesSSEProgressReporter(s.getProjectEventBus(ctx), getProjectID(ctx)))

	// Apply timeout from config
	genCtx := ctx
	if issuesCfg.Timeout != "" {
		timeout, err := time.ParseDuration(issuesCfg.Timeout)
		if err != nil {
			slog.Warn("invalid issues.timeout in config, using request context",
				"timeout", issuesCfg.Timeout, "error", err)
		} else {
			var cancel context.CancelFunc
			genCtx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
	}

	var result *issues.GenerateResult

	labels := req.Labels
	if len(labels) == 0 {
		labels = issuesCfg.Labels
	}
	assignees := req.Assignees
	if len(assignees) == 0 {
		assignees = issuesCfg.Assignees
	}

	// Resolve effective mode: use config Mode, falling back to Generator.Enabled for backward compat.
	effectiveMode := issuesCfg.Mode
	if effectiveMode == "" {
		if issuesCfg.Generator.Enabled {
			effectiveMode = core.IssueModeAgent
		} else {
			effectiveMode = core.IssueModeDirect
		}
	}

	// If issues are provided in the request, write to disk then create from files
	if len(req.Issues) > 0 {
		slog.Info("creating issues from frontend input", "count", len(req.Issues))

		issueInputs := make([]issues.IssueInput, len(req.Issues))
		for i, input := range req.Issues {
			issueInputs[i] = issues.IssueInput{
				Title:       input.Title,
				Body:        input.Body,
				Labels:      input.Labels,
				Assignees:   input.Assignees,
				IsMainIssue: input.IsMainIssue,
				TaskID:      input.TaskID,
				FilePath:    input.FilePath,
			}
		}

		previews, err := generator.WriteIssuesToDisk(workflowID, issueInputs)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("saving issue files failed: %v", err))
			return
		}
		for i := range issueInputs {
			if i < len(previews) {
				issueInputs[i].FilePath = previews[i].FilePath
			}
		}

		result, err = generator.CreateIssuesFromFiles(genCtx, workflowID, issueInputs, req.DryRun, req.LinkIssues, labels, assignees)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue creation failed: %v", err))
			return
		}
	} else if effectiveMode == core.IssueModeAgent {
		slog.Info("generating issues from filesystem artifacts using LLM", "mode", effectiveMode)
		if _, err := generator.GenerateIssueFiles(genCtx, workflowID); err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue generation failed: %v", err))
			return
		}

		previews, err := generator.ReadGeneratedIssues(workflowID)
		if err != nil {
			slog.Error(msgReadIssuesFailed, "error", err, "workflow_id", workflowID)
			respondError(w, http.StatusInternalServerError, msgReadIssuesFailed)
			return
		}

		issueInputs := make([]issues.IssueInput, 0, len(previews))
		for _, preview := range previews {
			if preview.IsMainIssue && !req.CreateMainIssue {
				continue
			}
			if !preview.IsMainIssue && !req.CreateSubIssues {
				continue
			}
			issueInputs = append(issueInputs, issues.IssueInput{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      nil,
				Assignees:   nil,
				IsMainIssue: preview.IsMainIssue,
				TaskID:      preview.TaskID,
				FilePath:    preview.FilePath,
			})
		}

		result, err = generator.CreateIssuesFromFiles(genCtx, workflowID, issueInputs, req.DryRun, req.LinkIssues, labels, assignees)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue creation failed: %v", err))
			return
		}
	} else {
		// Direct mode: read from filesystem (direct copy)
		slog.Info("generating issues from filesystem artifacts (direct copy)", "mode", effectiveMode)
		opts := issues.GenerateOptions{
			WorkflowID:      workflowID,
			DryRun:          req.DryRun,
			CreateMainIssue: req.CreateMainIssue,
			CreateSubIssues: req.CreateSubIssues,
			LinkIssues:      req.LinkIssues,
			CustomLabels:    labels,
			CustomAssignees: assignees,
		}

		result, err = generator.Generate(genCtx, opts)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue generation failed: %v", err))
			return
		}
	}

	// Build response
	response := GenerateIssuesResponse{
		Success: true,
	}

	if req.DryRun {
		response.Message = fmt.Sprintf("Preview: %d issues would be created", len(result.PreviewIssues))
		for _, preview := range result.PreviewIssues {
			response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      preview.Labels,
				Assignees:   preview.Assignees,
				IsMainIssue: preview.IsMainIssue,
				TaskID:      preview.TaskID,
				FilePath:    preview.FilePath,
			})
		}
	} else {
		count := result.IssueSet.TotalCount()
		response.Message = fmt.Sprintf("Created %d issues", count)

		if result.IssueSet.MainIssue != nil {
			response.MainIssue = &IssueResponse{
				Number: result.IssueSet.MainIssue.Number,
				Title:  result.IssueSet.MainIssue.Title,
				URL:    result.IssueSet.MainIssue.URL,
				State:  result.IssueSet.MainIssue.State,
				Labels: result.IssueSet.MainIssue.Labels,
			}
		}

		for _, sub := range result.IssueSet.SubIssues {
			response.SubIssues = append(response.SubIssues, IssueResponse{
				Number:      sub.Number,
				Title:       sub.Title,
				URL:         sub.URL,
				State:       sub.State,
				Labels:      sub.Labels,
				ParentIssue: sub.ParentIssue,
			})
		}
	}

	// Add any non-fatal errors
	for _, err := range result.Errors {
		response.Errors = append(response.Errors, err.Error())
	}

	// Add AI generation info for debugging
	response.AIUsed = result.AIUsed
	response.AIErrors = result.AIErrors

	respondJSON(w, http.StatusOK, response)
}

// handleSaveIssuesFiles saves issues to markdown files on disk.
// POST /api/workflows/{workflowID}/issues/files
func (s *Server) handleSaveIssuesFiles(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	// Get workflow state
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Parse request body
	var req SaveIssuesFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}
	if len(req.Issues) == 0 {
		respondError(w, http.StatusBadRequest, "issues are required")
		return
	}

	// Get issues config from loader
	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, msgIssuesDisabled)
		return
	}

	reportDir := state.ReportPath
	if reportDir == "" {
		respondError(w, http.StatusBadRequest, "workflow has no report directory")
		return
	}

	generator := issues.NewGenerator(nil, issuesCfg, "", reportDir, s.agentRegistry)

	issueInputs := make([]issues.IssueInput, len(req.Issues))
	for i, input := range req.Issues {
		issueInputs[i] = issues.IssueInput{
			Title:       input.Title,
			Body:        input.Body,
			Labels:      input.Labels,
			Assignees:   input.Assignees,
			IsMainIssue: input.IsMainIssue,
			TaskID:      input.TaskID,
			FilePath:    input.FilePath,
		}
	}

	previews, err := generator.WriteIssuesToDisk(workflowID, issueInputs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("saving issues failed: %v", err))
		return
	}

	response := SaveIssuesFilesResponse{
		Success: true,
		Message: fmt.Sprintf("Saved %d issue file(s)", len(previews)),
	}
	for _, preview := range previews {
		response.Issues = append(response.Issues, IssuePreviewResponse{
			Title:       preview.Title,
			Body:        preview.Body,
			Labels:      preview.Labels,
			Assignees:   preview.Assignees,
			IsMainIssue: preview.IsMainIssue,
			TaskID:      preview.TaskID,
			FilePath:    preview.FilePath,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// handlePreviewIssues previews issues without creating them.
// GET /api/workflows/{workflowID}/issues/preview
// Query params:
//   - fast=true: skip LLM generation for faster response (returns raw markdown)
//   - fast=false: use AI to generate polished markdown files
func (s *Server) handlePreviewIssues(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	// Check for fast mode (skip LLM generation)
	fastMode := r.URL.Query().Get("fast") == "true"

	// Get workflow state
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Get issues config
	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, "issue generation is disabled")
		return
	}

	// Determine report directory
	reportDir := state.ReportPath
	if reportDir == "" {
		respondError(w, http.StatusBadRequest, "workflow has no report directory")
		return
	}

	response := GenerateIssuesResponse{
		Success: true,
	}

	// Resolve effective mode for preview: fast mode forces direct, otherwise use config.
	effectiveMode := issuesCfg.Mode
	if effectiveMode == "" {
		if issuesCfg.Generator.Enabled {
			effectiveMode = core.IssueModeAgent
		} else {
			effectiveMode = core.IssueModeDirect
		}
	}
	if fastMode {
		effectiveMode = core.IssueModeDirect
	}

	if effectiveMode == core.IssueModeDirect {
		// Direct mode: use direct copy without AI
		issuesCfg.Generator.Enabled = false
		generator := issues.NewGenerator(nil, issuesCfg, "", reportDir, nil)

		opts := issues.GenerateOptions{
			WorkflowID:      workflowID,
			DryRun:          true,
			CreateMainIssue: false, // All sub-issues per user request
			CreateSubIssues: true,
		}

		result, err := generator.Generate(ctx, opts)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("preview failed: %v", err))
			return
		}

		response.Message = fmt.Sprintf("Preview: %d issues (direct mode)", len(result.PreviewIssues))
		for _, preview := range result.PreviewIssues {
			response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      preview.Labels,
				Assignees:   preview.Assignees,
				IsMainIssue: preview.IsMainIssue,
				TaskID:      preview.TaskID,
				FilePath:    preview.FilePath,
			})
		}
		response.AIUsed = false
	} else {
		// Agent mode: generate markdown files using LLM
		if s.agentRegistry == nil {
			respondError(w, http.StatusInternalServerError, "agent registry not available")
			return
		}

		generator := issues.NewGenerator(nil, issuesCfg, "", reportDir, s.agentRegistry)
		generator.SetProgressReporter(newIssuesSSEProgressReporter(s.getProjectEventBus(ctx), getProjectID(ctx)))

		// Generate the issue files
		files, err := generator.GenerateIssueFiles(ctx, workflowID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("AI generation failed: %v", err))
			return
		}

		// Read the generated files
		previews, err := generator.ReadGeneratedIssues(workflowID)
		if err != nil {
			slog.Error(msgReadIssuesFailed, "error", err, "workflow_id", workflowID)
			respondError(w, http.StatusInternalServerError, msgReadIssuesFailed)
			return
		}

		response.Message = fmt.Sprintf("Preview: %d issues (AI generated)", len(previews))
		response.AIUsed = true

		for _, preview := range previews {
			response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      preview.Labels,
				Assignees:   preview.Assignees,
				IsMainIssue: preview.IsMainIssue,
				TaskID:      preview.TaskID,
				FilePath:    preview.FilePath,
			})
		}

		// Log generated files
		slog.Debug("generated issue files", "count", len(files))
		for _, f := range files {
			slog.Debug("generated issue file", "path", f)
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetIssuesConfig returns the current issues configuration.
// GET /api/config/issues
func (s *Server) handleGetIssuesConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var issuesCfg config.IssuesConfig

	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	respondJSON(w, http.StatusOK, issuesCfg)
}

// CreateSingleIssueRequest is the request body for creating a single issue.
type CreateSingleIssueRequest struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	IsMainIssue bool     `json:"is_main_issue"`
	TaskID      string   `json:"task_id,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
}

// CreateSingleIssueResponse is the response for creating a single issue.
type CreateSingleIssueResponse struct {
	Success bool          `json:"success"`
	Issue   IssueResponse `json:"issue"`
	Error   string        `json:"error,omitempty"`
}

// handleCreateSingleIssue creates a single issue directly.
// POST /api/workflows/{workflowID}/issues/single
func (s *Server) handleCreateSingleIssue(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	// Get workflow state
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Parse request body
	var req CreateSingleIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
		return
	}

	// Validate
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	// Get issues config
	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, "issue generation is disabled")
		return
	}

	// Determine report directory (cheap check before expensive client creation)
	reportDir := state.ReportPath
	if reportDir == "" {
		respondError(w, http.StatusBadRequest, "workflow has no report directory")
		return
	}

	// Create issue client based on provider
	issueClient, clientErr := createIssueClient(issuesCfg)
	if clientErr != nil {
		writeIssueClientError(w, clientErr)
		return
	}

	// Use default labels if none provided
	labels := req.Labels
	if len(labels) == 0 {
		labels = issuesCfg.Labels
	}

	// Use default assignees if none provided
	assignees := req.Assignees
	if len(assignees) == 0 {
		assignees = issuesCfg.Assignees
	}

	generator := issues.NewGenerator(issueClient, issuesCfg, "", reportDir, s.agentRegistry)
	generator.SetProgressReporter(newIssuesSSEProgressReporter(s.getProjectEventBus(ctx), getProjectID(ctx)))

	input := issues.IssueInput{
		Title:       req.Title,
		Body:        req.Body,
		Labels:      labels,
		Assignees:   assignees,
		IsMainIssue: req.IsMainIssue,
		TaskID:      req.TaskID,
		FilePath:    req.FilePath,
	}

	if input.FilePath == "" {
		previews, err := generator.WriteIssuesToDisk(workflowID, []issues.IssueInput{input})
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save issue file: %v", err))
			return
		}
		if len(previews) > 0 {
			input.FilePath = previews[0].FilePath
		}
	}

	// Check for existing parent issue to enable sub-issue linking
	linkIssues := false
	if !input.IsMainIssue {
		mapping, _ := generator.ReadIssueMapping(workflowID)
		if mapping != nil {
			for _, entry := range mapping.Issues {
				if entry.IsMain && entry.IssueNumber > 0 {
					linkIssues = true
					break
				}
			}
		}
	}

	result, err := generator.CreateIssuesFromFiles(ctx, workflowID, []issues.IssueInput{input}, false, linkIssues, labels, assignees)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create issue: %v", err))
		return
	}

	var created *core.Issue
	if result.IssueSet.MainIssue != nil {
		created = result.IssueSet.MainIssue
	} else if len(result.IssueSet.SubIssues) > 0 {
		created = result.IssueSet.SubIssues[0]
	}
	if created == nil {
		respondError(w, http.StatusInternalServerError, "issue creation failed")
		return
	}

	respondJSON(w, http.StatusOK, CreateSingleIssueResponse{
		Success: true,
		Issue: IssueResponse{
			Number: created.Number,
			Title:  created.Title,
			URL:    created.URL,
			State:  created.State,
			Labels: created.Labels,
		},
	})
}

// DraftResponse represents a single draft issue in API responses.
type DraftResponse struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	IsMainIssue bool     `json:"is_main_issue"`
	TaskID      string   `json:"task_id,omitempty"`
	FilePath    string   `json:"file_path,omitempty"`
}

// DraftsListResponse is the response for listing draft files.
type DraftsListResponse struct {
	WorkflowID string          `json:"workflow_id"`
	Drafts     []DraftResponse `json:"drafts"`
}

// DraftUpdateRequest is the request body for editing a draft.
type DraftUpdateRequest struct {
	Title     *string   `json:"title"`
	Body      *string   `json:"body"`
	Labels    *[]string `json:"labels"`
	Assignees *[]string `json:"assignees"`
}

// PublishRequest is the request body for publishing drafts.
type PublishRequest struct {
	DryRun     bool     `json:"dry_run"`
	LinkIssues bool     `json:"link_issues"`
	TaskIDs    []string `json:"task_ids"`
}

// PublishResponse is the response for publishing drafts.
type PublishResponse struct {
	WorkflowID string            `json:"workflow_id"`
	Published  []PublishedRecord `json:"published"`
}

// PublishedRecord represents a published issue.
type PublishedRecord struct {
	TaskID      string `json:"task_id,omitempty"`
	FilePath    string `json:"file_path"`
	IssueNumber int    `json:"issue_number,omitempty"`
	IssueURL    string `json:"issue_url,omitempty"`
	IsMain      bool   `json:"is_main_issue"`
	Error       string `json:"error,omitempty"`
}

// IssuesStatusResponse is the response for issue generation status.
type IssuesStatusResponse struct {
	WorkflowID    string `json:"workflow_id"`
	HasDrafts     bool   `json:"has_drafts"`
	DraftCount    int    `json:"draft_count"`
	HasPublished  bool   `json:"has_published"`
	PublishedCount int   `json:"published_count"`
}

// createIssueClient creates an IssueClient based on provider config, using
// configurable Repository when set (overrides auto-detection).
// It is a package-level variable so tests can swap it with a mock.
var createIssueClient = newIssueClient

func newIssueClient(cfg config.IssuesConfig) (core.IssueClient, error) {
	switch cfg.Provider {
	case "github", "":
		if cfg.Repository != "" {
			parts := strings.SplitN(cfg.Repository, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("invalid repository format %q, expected owner/repo", cfg.Repository)
			}
			return github.NewIssueClient(parts[0], parts[1])
		}
		return github.NewIssueClientFromRepo()
	case "gitlab":
		return nil, fmt.Errorf("GitLab issue generation not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// writeIssueClientError writes the appropriate HTTP error for issue client creation failures.
func writeIssueClientError(w http.ResponseWriter, err error) {
	var domainErr *core.DomainError
	if errors.As(err, &domainErr) && domainErr.Code == "GH_NOT_AUTHENTICATED" {
		respondError(w, http.StatusUnauthorized, `GitHub CLI is not authenticated. Run "gh auth login" to authenticate.`)
		return
	}
	if strings.Contains(err.Error(), "gh auth login") || strings.Contains(err.Error(), "not authenticated") {
		respondError(w, http.StatusUnauthorized, `GitHub CLI is not authenticated. Run "gh auth login" to authenticate.`)
		return
	}
	if strings.Contains(err.Error(), "not yet implemented") {
		respondError(w, http.StatusNotImplemented, err.Error())
		return
	}
	if strings.Contains(err.Error(), "unknown provider") || strings.Contains(err.Error(), "invalid repository format") {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create issue client: %v", err))
}

// handleListDrafts lists current draft files for a workflow.
// GET /api/v1/workflows/{workflowID}/issues/drafts
func (s *Server) handleListDrafts(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	projectRoot := s.getProjectRootPath(ctx)
	generator := issues.NewGenerator(nil, issuesCfg, projectRoot, "", s.agentRegistry)

	drafts, err := generator.ReadAllDrafts(workflowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf(errReadDraftsFailed, err))
		return
	}

	response := DraftsListResponse{
		WorkflowID: workflowID,
		Drafts:     make([]DraftResponse, 0, len(drafts)),
	}
	for _, d := range drafts {
		response.Drafts = append(response.Drafts, DraftResponse{
			Title:       d.Title,
			Body:        d.Body,
			Labels:      d.Labels,
			Assignees:   d.Assignees,
			IsMainIssue: d.IsMainIssue,
			TaskID:      d.TaskID,
			FilePath:    d.FilePath,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// handleEditDraft edits a specific draft file.
// PUT /api/v1/workflows/{workflowID}/issues/drafts/{taskId}
func (s *Server) handleEditDraft(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	taskID := chi.URLParam(r, "taskId")
	ctx := r.Context()

	var req DraftUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf(errInvalidRequestBody, err.Error()))
		return
	}

	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	projectRoot := s.getProjectRootPath(ctx)
	generator := issues.NewGenerator(nil, issuesCfg, projectRoot, "", s.agentRegistry)

	drafts, err := generator.ReadAllDrafts(workflowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf(errReadDraftsFailed, err))
		return
	}

	var target *issues.IssuePreview
	for i := range drafts {
		if drafts[i].TaskID == taskID || (taskID == "main" && drafts[i].IsMainIssue) {
			target = &drafts[i]
			break
		}
	}

	if target == nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("draft not found for task: %s", taskID))
		return
	}

	// Apply updates
	applyDraftUpdates(target, req)

	// Write back using draft file APIs
	fileName := filepath.Base(target.FilePath)
	fm := issues.DraftFrontmatter{
		Title:       target.Title,
		Labels:      target.Labels,
		Assignees:   target.Assignees,
		IsMainIssue: target.IsMainIssue,
		TaskID:      target.TaskID,
		Status:      "draft",
	}
	if _, err := generator.WriteDraftFile(workflowID, fileName, fm, target.Body); err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save draft: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, DraftResponse{
		Title:       target.Title,
		Body:        target.Body,
		Labels:      target.Labels,
		Assignees:   target.Assignees,
		IsMainIssue: target.IsMainIssue,
		TaskID:      target.TaskID,
		FilePath:    target.FilePath,
	})
}

// applyDraftUpdates applies non-nil fields from the update request to the target draft.
func applyDraftUpdates(target *issues.IssuePreview, req DraftUpdateRequest) {
	if req.Title != nil {
		target.Title = *req.Title
	}
	if req.Body != nil {
		target.Body = *req.Body
	}
	if req.Labels != nil {
		target.Labels = *req.Labels
	}
	if req.Assignees != nil {
		target.Assignees = *req.Assignees
	}
}

// handlePublishDrafts publishes draft issues to GitHub.
// POST /api/v1/workflows/{workflowID}/issues/publish
func (s *Server) handlePublishDrafts(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	var req PublishRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf(errInvalidRequestBody, err.Error()))
			return
		}
	}

	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, msgIssuesDisabled)
		return
	}

	// Create issue client
	issueClient, clientErr := createIssueClient(issuesCfg)
	if clientErr != nil {
		writeIssueClientError(w, clientErr)
		return
	}

	projectRoot := s.getProjectRootPath(ctx)
	generator := issues.NewGenerator(issueClient, issuesCfg, projectRoot, "", s.agentRegistry)
	generator.SetProgressReporter(newIssuesSSEProgressReporter(s.getProjectEventBus(ctx), getProjectID(ctx)))

	// Read all drafts
	drafts, err := generator.ReadAllDrafts(workflowID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf(errReadDraftsFailed, err))
		return
	}
	if len(drafts) == 0 {
		respondError(w, http.StatusBadRequest, "no drafts to publish")
		return
	}

	// Filter by task IDs if specified
	drafts = filterDraftsByTaskIDs(drafts, req.TaskIDs)

	// Convert to IssueInputs for CreateIssuesFromFiles
	inputs := draftsToInputs(drafts)

	labels := issuesCfg.Labels
	assignees := issuesCfg.Assignees

	result, err := generator.CreateIssuesFromFiles(ctx, workflowID, inputs, req.DryRun, req.LinkIssues, labels, assignees)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("publish failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, buildPublishResponse(workflowID, result))
}

// filterDraftsByTaskIDs filters drafts to only include those matching the given task IDs.
// If taskIDs is empty, all drafts are returned unchanged.
func filterDraftsByTaskIDs(drafts []issues.IssuePreview, taskIDs []string) []issues.IssuePreview {
	if len(taskIDs) == 0 {
		return drafts
	}
	taskIDSet := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		taskIDSet[id] = true
	}
	filtered := make([]issues.IssuePreview, 0, len(drafts))
	for _, d := range drafts {
		if taskIDSet[d.TaskID] || (d.IsMainIssue && taskIDSet["main"]) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// draftsToInputs converts issue previews to issue inputs for CreateIssuesFromFiles.
func draftsToInputs(drafts []issues.IssuePreview) []issues.IssueInput {
	inputs := make([]issues.IssueInput, 0, len(drafts))
	for _, d := range drafts {
		inputs = append(inputs, issues.IssueInput{
			Title:       d.Title,
			Body:        d.Body,
			Labels:      d.Labels,
			Assignees:   d.Assignees,
			IsMainIssue: d.IsMainIssue,
			TaskID:      d.TaskID,
			FilePath:    d.FilePath,
		})
	}
	return inputs
}

// buildPublishResponse converts a GenerateResult into a PublishResponse.
func buildPublishResponse(workflowID string, result *issues.GenerateResult) PublishResponse {
	var records []PublishedRecord
	if result.IssueSet.MainIssue != nil {
		records = append(records, PublishedRecord{
			IssueNumber: result.IssueSet.MainIssue.Number,
			IssueURL:    result.IssueSet.MainIssue.URL,
			IsMain:      true,
		})
	}
	for _, sub := range result.IssueSet.SubIssues {
		records = append(records, PublishedRecord{
			IssueNumber: sub.Number,
			IssueURL:    sub.URL,
			IsMain:      false,
		})
	}
	for _, preview := range result.PreviewIssues {
		records = append(records, PublishedRecord{
			TaskID:   preview.TaskID,
			FilePath: preview.FilePath,
			IsMain:   preview.IsMainIssue,
		})
	}
	return PublishResponse{
		WorkflowID: workflowID,
		Published:  records,
	}
}

// handleIssuesStatus returns the current status of issue drafts and published issues.
// GET /api/v1/workflows/{workflowID}/issues/status
func (s *Server) handleIssuesStatus(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	var issuesCfg config.IssuesConfig
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	projectRoot := s.getProjectRootPath(ctx)
	generator := issues.NewGenerator(nil, issuesCfg, projectRoot, "", s.agentRegistry)

	drafts, _ := generator.ReadAllDrafts(workflowID)
	mapping, _ := generator.ReadIssueMapping(workflowID)

	publishedCount := 0
	if mapping != nil {
		publishedCount = len(mapping.Issues)
	}

	respondJSON(w, http.StatusOK, IssuesStatusResponse{
		WorkflowID:     workflowID,
		HasDrafts:      len(drafts) > 0,
		DraftCount:     len(drafts),
		HasPublished:   publishedCount > 0,
		PublishedCount: publishedCount,
	})
}
