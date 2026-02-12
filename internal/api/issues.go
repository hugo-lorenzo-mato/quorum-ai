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

// issueGenContext holds validated inputs for issue generation.
type issueGenContext struct {
	workflowID   string
	req          GenerateIssuesRequest
	issuesCfg    config.IssuesConfig
	generator    *issues.Generator
	genCtx       context.Context
	cancelFn     context.CancelFunc
	labels       []string
	assignees    []string
	mode         string // effective generation mode
}

// handleGenerateIssues generates GitHub/GitLab issues from workflow artifacts.
// POST /api/workflows/{workflowID}/issues
func (s *Server) handleGenerateIssues(w http.ResponseWriter, r *http.Request) {
	igc, err := s.setupIssueGenContext(r)
	if err != nil {
		writeIssueSetupError(w, err)
		return
	}
	if igc.cancelFn != nil {
		defer igc.cancelFn()
	}

	result, err := s.executeIssueGeneration(igc)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, buildIssueGenerateResponse(igc.req, result))
}

// issueSetupError distinguishes validation errors (4xx) from internal errors (5xx).
type issueSetupError struct {
	status int
	msg    string
}

func (e *issueSetupError) Error() string { return e.msg }

func writeIssueSetupError(w http.ResponseWriter, err error) {
	if se, ok := err.(*issueSetupError); ok {
		respondError(w, se.status, se.msg)
		return
	}
	respondError(w, http.StatusInternalServerError, err.Error())
}

// setupIssueGenContext validates the request and builds the generation context.
func (s *Server) setupIssueGenContext(r *http.Request) (*issueGenContext, error) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		return nil, &issueSetupError{http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err)}
	}
	if state == nil {
		return nil, &issueSetupError{http.StatusNotFound, "workflow not found"}
	}

	var req GenerateIssuesRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, &issueSetupError{http.StatusBadRequest, fmt.Sprintf(errInvalidRequestBody, err.Error())}
		}
	} else {
		req = GenerateIssuesRequest{CreateMainIssue: true, CreateSubIssues: true, LinkIssues: true}
	}

	var issuesCfg config.IssuesConfig
	if configLoader := s.getProjectConfigLoader(ctx); configLoader != nil {
		if cfg, err := configLoader.Load(); err == nil {
			issuesCfg = cfg.Issues
		}
	}
	if !issuesCfg.Enabled {
		return nil, &issueSetupError{http.StatusBadRequest, msgIssuesDisabled}
	}

	reportDir := state.ReportPath
	if reportDir == "" {
		return nil, &issueSetupError{http.StatusBadRequest, "workflow has no report directory"}
	}
	projectRoot := s.getProjectRootPath(ctx)
	fullReportDir := reportDir
	if projectRoot != "" && !filepath.IsAbs(reportDir) {
		fullReportDir = filepath.Join(projectRoot, reportDir)
	}

	issueClient, clientErr := createIssueClient(issuesCfg)
	if clientErr != nil {
		// Reuse existing error classification
		var domainErr *core.DomainError
		status := http.StatusInternalServerError
		if errors.As(clientErr, &domainErr) && domainErr.Code == "GH_NOT_AUTHENTICATED" {
			status = http.StatusUnauthorized
		} else if strings.Contains(clientErr.Error(), "not yet implemented") {
			status = http.StatusNotImplemented
		} else if strings.Contains(clientErr.Error(), "unknown provider") || strings.Contains(clientErr.Error(), "invalid repository format") {
			status = http.StatusBadRequest
		} else if strings.Contains(clientErr.Error(), "not authenticated") {
			status = http.StatusUnauthorized
		}
		return nil, &issueSetupError{status, fmt.Sprintf("failed to create issue client: %v", clientErr)}
	}

	eventBus := s.getProjectEventBus(ctx)
	projectID := getProjectID(ctx)
	generator := issues.NewGenerator(issueClient, issuesCfg, projectRoot, fullReportDir, s.agentRegistry)
	generator.SetProgressReporter(newIssuesSSEProgressReporter(eventBus, projectID))
	generator.SetAgentEventHandler(newIssuesAgentEventHandler(eventBus, workflowID, projectID))

	genCtx := ctx
	var cancelFn context.CancelFunc
	if issuesCfg.Timeout != "" {
		if timeout, err := time.ParseDuration(issuesCfg.Timeout); err != nil {
			slog.Warn("invalid issues.timeout in config, using request context",
				"timeout", issuesCfg.Timeout, "error", err)
		} else {
			genCtx, cancelFn = context.WithTimeout(ctx, timeout)
		}
	}

	labels := req.Labels
	if len(labels) == 0 {
		labels = issuesCfg.Labels
	}
	assignees := req.Assignees
	if len(assignees) == 0 {
		assignees = issuesCfg.Assignees
	}

	effectiveMode := issuesCfg.Mode
	if effectiveMode == "" {
		if issuesCfg.Generator.Enabled {
			effectiveMode = core.IssueModeAgent
		} else {
			effectiveMode = core.IssueModeDirect
		}
	}

	return &issueGenContext{
		workflowID: workflowID,
		req:        req,
		issuesCfg:  issuesCfg,
		generator:  generator,
		genCtx:     genCtx,
		cancelFn:   cancelFn,
		labels:     labels,
		assignees:  assignees,
		mode:       effectiveMode,
	}, nil
}

// executeIssueGeneration runs the appropriate generation path based on context.
func (s *Server) executeIssueGeneration(igc *issueGenContext) (*issues.GenerateResult, error) {
	if len(igc.req.Issues) > 0 {
		return s.executeFromFrontendInput(igc)
	}
	if igc.mode == core.IssueModeAgent {
		return s.executeAgentGeneration(igc)
	}
	return s.executeDirectGeneration(igc)
}

func (s *Server) executeFromFrontendInput(igc *issueGenContext) (*issues.GenerateResult, error) {
	slog.Info("creating issues from frontend input", "count", len(igc.req.Issues))
	issueInputs := convertAPIToIssueInputs(igc.req.Issues)

	previews, err := igc.generator.WriteIssuesToDisk(igc.workflowID, issueInputs)
	if err != nil {
		return nil, fmt.Errorf("saving issue files failed: %v", err)
	}
	for i := range issueInputs {
		if i < len(previews) {
			issueInputs[i].FilePath = previews[i].FilePath
		}
	}

	result, err := igc.generator.CreateIssuesFromFiles(igc.genCtx, igc.workflowID, issueInputs, igc.req.DryRun, igc.req.LinkIssues, igc.labels, igc.assignees)
	if err != nil {
		return nil, fmt.Errorf("issue creation failed: %v", err)
	}
	return result, nil
}

func (s *Server) executeAgentGeneration(igc *issueGenContext) (*issues.GenerateResult, error) {
	slog.Info("generating issues from filesystem artifacts using LLM", "mode", igc.mode)
	if _, err := igc.generator.GenerateIssueFiles(igc.genCtx, igc.workflowID); err != nil {
		return nil, fmt.Errorf("issue generation failed: %v", err)
	}

	previews, err := igc.generator.ReadGeneratedIssues(igc.workflowID)
	if err != nil {
		slog.Error(msgReadIssuesFailed, "error", err, "workflow_id", igc.workflowID)
		return nil, fmt.Errorf(msgReadIssuesFailed)
	}

	issueInputs := make([]issues.IssueInput, 0, len(previews))
	for _, preview := range previews {
		if preview.IsMainIssue && !igc.req.CreateMainIssue {
			continue
		}
		if !preview.IsMainIssue && !igc.req.CreateSubIssues {
			continue
		}
		issueInputs = append(issueInputs, issues.IssueInput{
			Title: preview.Title, Body: preview.Body,
			IsMainIssue: preview.IsMainIssue, TaskID: preview.TaskID, FilePath: preview.FilePath,
		})
	}

	result, err := igc.generator.CreateIssuesFromFiles(igc.genCtx, igc.workflowID, issueInputs, igc.req.DryRun, igc.req.LinkIssues, igc.labels, igc.assignees)
	if err != nil {
		return nil, fmt.Errorf("issue creation failed: %v", err)
	}

	for _, name := range igc.generator.LastMissingFiles {
		result.AIErrors = append(result.AIErrors, fmt.Sprintf("expected file not generated: %s", name))
	}
	return result, nil
}

func (s *Server) executeDirectGeneration(igc *issueGenContext) (*issues.GenerateResult, error) {
	slog.Info("generating issues from filesystem artifacts (direct copy)", "mode", igc.mode)
	opts := issues.GenerateOptions{
		WorkflowID: igc.workflowID, DryRun: igc.req.DryRun,
		CreateMainIssue: igc.req.CreateMainIssue, CreateSubIssues: igc.req.CreateSubIssues,
		LinkIssues: igc.req.LinkIssues, CustomLabels: igc.labels, CustomAssignees: igc.assignees,
	}
	result, err := igc.generator.Generate(igc.genCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("issue generation failed: %v", err)
	}
	return result, nil
}

// convertAPIToIssueInputs converts API issue inputs to the internal type.
func convertAPIToIssueInputs(apiInputs []IssueInput) []issues.IssueInput {
	inputs := make([]issues.IssueInput, len(apiInputs))
	for i, input := range apiInputs {
		inputs[i] = issues.IssueInput{
			Title: input.Title, Body: input.Body, Labels: input.Labels,
			Assignees: input.Assignees, IsMainIssue: input.IsMainIssue,
			TaskID: input.TaskID, FilePath: input.FilePath,
		}
	}
	return inputs
}

// buildIssueGenerateResponse builds the HTTP response from generation results.
func buildIssueGenerateResponse(req GenerateIssuesRequest, result *issues.GenerateResult) GenerateIssuesResponse {
	response := GenerateIssuesResponse{Success: true}

	if req.DryRun {
		response.Message = fmt.Sprintf("Preview: %d issues would be created", len(result.PreviewIssues))
		for _, preview := range result.PreviewIssues {
			response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
				Title: preview.Title, Body: preview.Body, Labels: preview.Labels,
				Assignees: preview.Assignees, IsMainIssue: preview.IsMainIssue,
				TaskID: preview.TaskID, FilePath: preview.FilePath,
			})
		}
	} else {
		count := result.IssueSet.TotalCount()
		response.Message = fmt.Sprintf("Created %d issues", count)

		if result.IssueSet.MainIssue != nil {
			response.MainIssue = &IssueResponse{
				Number: result.IssueSet.MainIssue.Number, Title: result.IssueSet.MainIssue.Title,
				URL: result.IssueSet.MainIssue.URL, State: result.IssueSet.MainIssue.State,
				Labels: result.IssueSet.MainIssue.Labels,
			}
		}
		for _, sub := range result.IssueSet.SubIssues {
			response.SubIssues = append(response.SubIssues, IssueResponse{
				Number: sub.Number, Title: sub.Title, URL: sub.URL,
				State: sub.State, Labels: sub.Labels, ParentIssue: sub.ParentIssue,
			})
		}
	}

	for _, err := range result.Errors {
		response.Errors = append(response.Errors, err.Error())
	}
	response.AIUsed = result.AIUsed
	response.AIErrors = result.AIErrors
	return response
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

	projectRoot := s.getProjectRootPath(ctx)
	fullReportDir := reportDir
	if projectRoot != "" && !filepath.IsAbs(reportDir) {
		fullReportDir = filepath.Join(projectRoot, reportDir)
	}
	generator := issues.NewGenerator(nil, issuesCfg, projectRoot, fullReportDir, s.agentRegistry)

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

	// Preview mode override via query param.
	// Docs: fast=true => direct mode; fast=false => AI/agent mode.
	// If omitted, fall back to config-driven behavior.
	fastParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("fast")))
	forceDirect := fastParam == "true"
	forceAgent := fastParam == "false"

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
	defaultAgent := ""
	configLoader := s.getProjectConfigLoader(ctx)
	if configLoader != nil {
		cfg, err := configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
			defaultAgent = cfg.Agents.Default
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

	projectRoot := s.getProjectRootPath(ctx)
	fullReportDir := reportDir
	if projectRoot != "" && !filepath.IsAbs(reportDir) {
		fullReportDir = filepath.Join(projectRoot, reportDir)
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
	// Explicit override from query param wins over config.
	if forceDirect {
		effectiveMode = core.IssueModeDirect
	} else if forceAgent {
		effectiveMode = core.IssueModeAgent
	}

	if effectiveMode == core.IssueModeDirect {
		// Direct mode: use direct copy without AI
		issuesCfg.Generator.Enabled = false
		generator := issues.NewGenerator(nil, issuesCfg, projectRoot, fullReportDir, nil)

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

		// If no explicit generator agent is configured, fall back to the project's default agent.
		// This avoids surprising failures when users request AI preview without configuring issues.generator.agent.
		if strings.TrimSpace(issuesCfg.Generator.Agent) == "" {
			if strings.TrimSpace(defaultAgent) != "" {
				issuesCfg.Generator.Agent = defaultAgent
			} else {
				enabled := s.agentRegistry.ListEnabled()
				if len(enabled) > 0 {
					issuesCfg.Generator.Agent = enabled[0]
				}
			}
		}
		if strings.TrimSpace(issuesCfg.Generator.Agent) == "" {
			respondError(w, http.StatusBadRequest, "issues.generator.agent is required for AI generation")
			return
		}

		previewBus := s.getProjectEventBus(ctx)
		previewProjectID := getProjectID(ctx)
		generator := issues.NewGenerator(nil, issuesCfg, projectRoot, fullReportDir, s.agentRegistry)
		generator.SetProgressReporter(newIssuesSSEProgressReporter(previewBus, previewProjectID))
		generator.SetAgentEventHandler(newIssuesAgentEventHandler(previewBus, workflowID, previewProjectID))

		// Apply timeout from config (same as handleGenerateIssues)
		genCtx := ctx
		if issuesCfg.Timeout != "" {
			timeout, parseErr := time.ParseDuration(issuesCfg.Timeout)
			if parseErr != nil {
				slog.Warn("invalid issues.timeout in config, using request context",
					"timeout", issuesCfg.Timeout, "error", parseErr)
			} else {
				var cancel context.CancelFunc
				genCtx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
		}

		// Generate the issue files
		files, err := generator.GenerateIssueFiles(genCtx, workflowID)
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

		// Report any missing files as AI warnings
		for _, name := range generator.LastMissingFiles {
			response.AIErrors = append(response.AIErrors, fmt.Sprintf("expected file not generated: %s", name))
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

	projectRoot := s.getProjectRootPath(ctx)
	fullReportDir := reportDir
	if projectRoot != "" && !filepath.IsAbs(reportDir) {
		fullReportDir = filepath.Join(projectRoot, reportDir)
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

	generator := issues.NewGenerator(issueClient, issuesCfg, projectRoot, fullReportDir, s.agentRegistry)
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
	WorkflowID     string `json:"workflow_id"`
	HasDrafts      bool   `json:"has_drafts"`
	DraftCount     int    `json:"draft_count"`
	HasPublished   bool   `json:"has_published"`
	PublishedCount int    `json:"published_count"`
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
