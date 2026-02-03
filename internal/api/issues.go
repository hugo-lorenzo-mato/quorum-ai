package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
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
}

// handleGenerateIssues generates GitHub/GitLab issues from workflow artifacts.
// POST /api/workflows/{workflowID}/issues
func (s *Server) handleGenerateIssues(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	ctx := r.Context()

	// Get workflow state
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Use defaults if no body provided
		req = GenerateIssuesRequest{
			CreateMainIssue: true,
			CreateSubIssues: true,
			LinkIssues:      true,
		}
	}

	// Get issues config from loader
	var issuesCfg config.IssuesConfig
	if s.configLoader != nil {
		cfg, err := s.configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	// Check if issues are enabled
	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, "issue generation is disabled in configuration")
		return
	}

	// Create issue client based on provider
	var issueClient core.IssueClient
	switch issuesCfg.Provider {
	case "github", "":
		client, err := github.NewIssueClientFromRepo()
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create GitHub client: %v", err))
			return
		}
		issueClient = client
	case "gitlab":
		// TODO: Implement GitLab client
		respondError(w, http.StatusNotImplemented, "GitLab issue generation not yet implemented")
		return
	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown provider: %s", issuesCfg.Provider))
		return
	}

	// Determine report directory
	reportDir := state.ReportPath
	if reportDir == "" {
		respondError(w, http.StatusBadRequest, "workflow has no report directory")
		return
	}

	// Create generator with agent registry for LLM-based generation
	generator := issues.NewGenerator(issueClient, issuesCfg, reportDir, s.agentRegistry)

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

	// If issues are provided in the request, create them directly (frontend edited flow)
	if len(req.Issues) > 0 {
		slog.Info("creating issues from frontend input", "count", len(req.Issues))
		
		// Convert API types to service types
		issueInputs := make([]issues.IssueInput, len(req.Issues))
		for i, input := range req.Issues {
			issueInputs[i] = issues.IssueInput{
				Title:       input.Title,
				Body:        input.Body,
				Labels:      input.Labels,
				Assignees:   input.Assignees,
				IsMainIssue: input.IsMainIssue,
				TaskID:      input.TaskID,
			}
		}
		
		result, err = generator.CreateIssuesFromInput(genCtx, issueInputs, req.DryRun, req.LinkIssues, req.Labels, req.Assignees)
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue creation failed: %v", err))
			return
		}
	} else {
		// Otherwise, use traditional flow (read from filesystem)
		slog.Info("generating issues from filesystem artifacts")
		opts := issues.GenerateOptions{
			WorkflowID:      workflowID,
			DryRun:          req.DryRun,
			CreateMainIssue: req.CreateMainIssue,
			CreateSubIssues: req.CreateSubIssues,
			LinkIssues:      req.LinkIssues,
			CustomLabels:    req.Labels,
			CustomAssignees: req.Assignees,
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
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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
	if s.configLoader != nil {
		cfg, err := s.configLoader.Load()
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

	if fastMode {
		// Fast mode: use direct copy without AI
		issuesCfg.Generator.Enabled = false
		generator := issues.NewGenerator(nil, issuesCfg, reportDir, nil)

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

		response.Message = fmt.Sprintf("Preview: %d issues (fast mode)", len(result.PreviewIssues))
		for _, preview := range result.PreviewIssues {
			response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
				Title:       preview.Title,
				Body:        preview.Body,
				Labels:      preview.Labels,
				Assignees:   preview.Assignees,
				IsMainIssue: false,
				TaskID:      preview.TaskID,
			})
		}
		response.AIUsed = false
	} else {
		// AI mode: generate markdown files using LLM
		if s.agentRegistry == nil {
			respondError(w, http.StatusInternalServerError, "agent registry not available")
			return
		}

		generator := issues.NewGenerator(nil, issuesCfg, reportDir, s.agentRegistry)

		// Generate the issue files
		files, err := generator.GenerateIssueFiles(ctx, workflowID)
		if err != nil {
			response.Success = false
			response.AIErrors = append(response.AIErrors, err.Error())
			// Try to read any existing generated files as fallback
			previews, readErr := generator.ReadGeneratedIssues(workflowID)
			if readErr == nil && len(previews) > 0 {
				for _, preview := range previews {
					response.PreviewIssues = append(response.PreviewIssues, IssuePreviewResponse{
						Title:       preview.Title,
						Body:        preview.Body,
						Labels:      preview.Labels,
						Assignees:   preview.Assignees,
						IsMainIssue: false,
						TaskID:      preview.TaskID,
					})
				}
				response.Success = true
				response.Message = fmt.Sprintf("Preview: %d issues (from cache)", len(previews))
			} else {
				respondError(w, http.StatusInternalServerError, fmt.Sprintf("AI generation failed: %v", err))
				return
			}
		} else {
			// Read the generated files
			previews, err := generator.ReadGeneratedIssues(workflowID)
			if err != nil {
				respondError(w, http.StatusInternalServerError, fmt.Sprintf("reading generated issues: %v", err))
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
					IsMainIssue: false,
					TaskID:      preview.TaskID,
				})
			}

			// Log generated files
			for _, f := range files {
				fmt.Printf("Generated: %s\n", f)
			}
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetIssuesConfig returns the current issues configuration.
// GET /api/config/issues
func (s *Server) handleGetIssuesConfig(w http.ResponseWriter, r *http.Request) {
	var issuesCfg config.IssuesConfig

	if s.configLoader != nil {
		cfg, err := s.configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	respondJSON(w, http.StatusOK, issuesCfg)
}

// CreateSingleIssueRequest is the request body for creating a single issue.
type CreateSingleIssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
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
	ctx := r.Context()

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
	if s.configLoader != nil {
		cfg, err := s.configLoader.Load()
		if err == nil {
			issuesCfg = cfg.Issues
		}
	}

	if !issuesCfg.Enabled {
		respondError(w, http.StatusBadRequest, "issue generation is disabled")
		return
	}

	// Create issue client based on provider
	var issueClient core.IssueClient
	switch issuesCfg.Provider {
	case "github", "":
		client, err := github.NewIssueClientFromRepo()
		if err != nil {
			respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create GitHub client: %v", err))
			return
		}
		issueClient = client
	case "gitlab":
		respondError(w, http.StatusNotImplemented, "GitLab not yet implemented")
		return
	default:
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unknown provider: %s", issuesCfg.Provider))
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

	// Create the issue
	issue, err := issueClient.CreateIssue(ctx, core.CreateIssueOptions{
		Title:     req.Title,
		Body:      req.Body,
		Labels:    labels,
		Assignees: assignees,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create issue: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, CreateSingleIssueResponse{
		Success: true,
		Issue: IssueResponse{
			Number: issue.Number,
			Title:  issue.Title,
			URL:    issue.URL,
			State:  issue.State,
			Labels: issue.Labels,
		},
	})
}
