package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
)

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
// POST /api/workflows/{id}/issues
func (s *Server) handleGenerateIssues(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	ctx := r.Context()

	// Get workflow state
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
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

	// Generate issues
	opts := issues.GenerateOptions{
		WorkflowID:      workflowID,
		DryRun:          req.DryRun,
		CreateMainIssue: req.CreateMainIssue,
		CreateSubIssues: req.CreateSubIssues,
		LinkIssues:      req.LinkIssues,
		CustomLabels:    req.Labels,
		CustomAssignees: req.Assignees,
	}

	result, err := generator.Generate(ctx, opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("issue generation failed: %v", err))
		return
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

	respondJSON(w, http.StatusOK, response)
}

// handlePreviewIssues previews issues without creating them (alias for dry-run).
// GET /api/workflows/{id}/issues/preview
func (s *Server) handlePreviewIssues(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "id")
	ctx := r.Context()

	// Get workflow state
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusNotFound, fmt.Sprintf("workflow not found: %v", err))
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

	// Create generator (no client needed for preview, but agent registry for LLM)
	generator := issues.NewGenerator(nil, issuesCfg, reportDir, s.agentRegistry)

	// Generate previews
	opts := issues.GenerateOptions{
		WorkflowID:      workflowID,
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	}

	result, err := generator.Generate(ctx, opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("preview failed: %v", err))
		return
	}

	// Build response
	response := GenerateIssuesResponse{
		Success: true,
		Message: fmt.Sprintf("Preview: %d issues", len(result.PreviewIssues)),
	}

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
