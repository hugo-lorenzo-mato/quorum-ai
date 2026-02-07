package api

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// WorkflowResponse is the API response for a workflow.
type WorkflowResponse struct {
	ID              string            `json:"id"`
	ExecutionID     int               `json:"execution_id"`
	Title           string            `json:"title,omitempty"`
	Status          string            `json:"status"`
	CurrentPhase    string            `json:"current_phase"`
	Prompt          string            `json:"prompt"`
	OptimizedPrompt string            `json:"optimized_prompt,omitempty"`
	Attachments     []core.Attachment `json:"attachments,omitempty"`
	Error           string            `json:"error,omitempty"`
	Warning         string            `json:"warning,omitempty"` // Warning about potential issues (e.g., duplicates)
	ReportPath      string            `json:"report_path,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	HeartbeatAt     *time.Time        `json:"heartbeat_at,omitempty"` // Last heartbeat for zombie detection
	IsActive        bool              `json:"is_active"`
	ActuallyRunning bool              `json:"actually_running,omitempty"` // True if executing in this process
	TaskCount       int               `json:"task_count"`
	Metrics         *Metrics          `json:"metrics,omitempty"`
	AgentEvents     []core.AgentEvent `json:"agent_events,omitempty"` // Persisted agent activity
	Tasks           []TaskResponse    `json:"tasks,omitempty"`        // Persisted task state for reload
	Blueprint       *BlueprintDTO     `json:"blueprint,omitempty"`    // Workflow orchestration blueprint
}

// Metrics represents workflow metrics in API responses.
type Metrics struct {
	TotalTokensIn  int     `json:"total_tokens_in"`
	TotalTokensOut int     `json:"total_tokens_out"`
	ConsensusScore float64 `json:"consensus_score"`
}

// CreateWorkflowRequest is the request body for creating a workflow.
type CreateWorkflowRequest struct {
	Prompt    string        `json:"prompt"`
	Title     string        `json:"title,omitempty"`
	Blueprint *BlueprintDTO `json:"blueprint,omitempty"`
}

// BlueprintDTO represents the workflow blueprint in API requests/responses.
type BlueprintDTO struct {
	ConsensusThreshold float64 `json:"consensus_threshold,omitempty"`
	MaxRetries         int     `json:"max_retries,omitempty"`
	TimeoutSeconds     int     `json:"timeout_seconds,omitempty"`
	DryRun             bool    `json:"dry_run,omitempty"`
	Sandbox            bool    `json:"sandbox,omitempty"`

	// ExecutionMode determines whether to use multi-agent consensus or single-agent mode.
	// Valid values: "multi_agent" (default), "single_agent"
	ExecutionMode string `json:"execution_mode,omitempty"`

	// SingleAgentName is the name of the agent to use when execution_mode is "single_agent".
	SingleAgentName string `json:"single_agent_name,omitempty"`

	// SingleAgentModel is an optional model override for the single agent.
	SingleAgentModel string `json:"single_agent_model,omitempty"`

	// SingleAgentReasoningEffort is an optional reasoning effort override for the single agent.
	SingleAgentReasoningEffort string `json:"single_agent_reasoning_effort,omitempty"`
}

type blueprintPatch struct {
	ExecutionMode              *string `json:"execution_mode,omitempty"`
	SingleAgentName            *string `json:"single_agent_name,omitempty"`
	SingleAgentModel           *string `json:"single_agent_model,omitempty"`
	SingleAgentReasoningEffort *string `json:"single_agent_reasoning_effort,omitempty"`
}

// IsSingleAgentMode returns true if the blueprint specifies single-agent execution.
func (b *BlueprintDTO) IsSingleAgentMode() bool {
	return b != nil && b.ExecutionMode == "single_agent"
}

// GetExecutionMode returns the execution mode, defaulting to "multi_agent" if not specified.
func (b *BlueprintDTO) GetExecutionMode() string {
	if b == nil || b.ExecutionMode == "" {
		return "multi_agent"
	}
	return b.ExecutionMode
}

// RunWorkflowResponse is the response for starting a workflow.
type RunWorkflowResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	CurrentPhase string `json:"current_phase"`
	Prompt       string `json:"prompt"`
	Message      string `json:"message"`
}

// ReplanRequest is the request body for replanning with additional context.
type ReplanRequest struct {
	Context string `json:"context,omitempty"` // Additional context to prepend to analysis
}

// PhaseResponse is the response for phase-specific execution endpoints.
type PhaseResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	CurrentPhase string `json:"current_phase"`
	Message      string `json:"message"`
}

// isWorkflowRunning checks if a workflow is currently running using the UnifiedTracker.
// This provides a single source of truth for workflow execution status.
func (s *Server) isWorkflowRunning(ctx context.Context, workflowID string) bool {
	// Use UnifiedTracker as the single source of truth
	if s.unifiedTracker != nil {
		return s.unifiedTracker.IsRunning(ctx, core.WorkflowID(workflowID))
	}
	// Fallback to executor if tracker not available
	if s.executor != nil {
		return s.executor.IsRunning(workflowID)
	}
	return false
}

// handleListWorkflows returns all workflows.
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	// Return empty list if state manager is not configured
	if stateManager == nil {
		respondJSON(w, http.StatusOK, []WorkflowResponse{})
		return
	}

	workflows, err := stateManager.ListWorkflows(ctx)
	if err != nil {
		s.logger.Error("failed to list workflows", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list workflows")
		return
	}

	response := make([]WorkflowResponse, 0, len(workflows))
	for _, wf := range workflows {
		response = append(response, WorkflowResponse{
			ID:           string(wf.WorkflowID),
			Title:        wf.Title,
			Status:       string(wf.Status),
			CurrentPhase: string(wf.CurrentPhase),
			Prompt:       wf.Prompt,
			CreatedAt:    wf.CreatedAt,
			UpdatedAt:    wf.UpdatedAt,
			IsActive:     wf.IsActive,
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetWorkflow returns a specific workflow by ID.
func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	activeID, _ := stateManager.GetActiveWorkflowID(ctx)

	response := s.stateToWorkflowResponse(ctx, state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleGetActiveWorkflow returns the currently active workflow.
func (s *Server) handleGetActiveWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	// Return 404 if state manager is not configured
	if stateManager == nil {
		respondError(w, http.StatusNotFound, "no active workflow")
		return
	}

	activeID, err := stateManager.GetActiveWorkflowID(ctx)
	if err != nil {
		s.logger.Error("failed to get active workflow ID", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get active workflow")
		return
	}

	if activeID == "" {
		respondError(w, http.StatusNotFound, "no active workflow")
		return
	}

	state, err := stateManager.LoadByID(ctx, activeID)
	if err != nil {
		s.logger.Error("failed to load active workflow", "workflow_id", activeID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "active workflow not found")
		return
	}

	// Only return as "active" if the workflow is actually running
	if state.Status != core.WorkflowStatusRunning {
		respondError(w, http.StatusNotFound, "no active workflow")
		return
	}

	response := s.stateToWorkflowResponse(ctx, state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleCreateWorkflow creates a new workflow.
func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Check for duplicate prompts - block if pending/running, warn if completed/failed
	var duplicateWarning string
	duplicates, err := stateManager.FindWorkflowsByPrompt(ctx, req.Prompt)
	if err != nil {
		s.logger.Warn("failed to check for duplicate prompts", "error", err)
	} else if len(duplicates) > 0 {
		// Check for active duplicates (pending or running) - these block creation
		var activeDuplicates []core.DuplicateWorkflowInfo
		var inactiveDuplicates []core.DuplicateWorkflowInfo
		for _, dup := range duplicates {
			if dup.Status == core.WorkflowStatusPending || dup.Status == core.WorkflowStatusRunning {
				activeDuplicates = append(activeDuplicates, dup)
			} else {
				inactiveDuplicates = append(inactiveDuplicates, dup)
			}
		}

		// Block creation if there are active duplicates
		if len(activeDuplicates) > 0 {
			var ids []string
			for _, dup := range activeDuplicates {
				ids = append(ids, fmt.Sprintf("%s (%s)", dup.WorkflowID, dup.Status))
			}
			respondError(w, http.StatusConflict,
				fmt.Sprintf("Cannot create workflow: identical prompt already exists in active state. Existing workflow(s): %s. Please wait for completion or delete the existing workflow.",
					strings.Join(ids, ", ")))
			return
		}

		// Block creation if a workflow with identical prompt was completed recently (5 min deduplication window)
		// This prevents accidental duplicate submissions
		const deduplicationWindow = 5 * time.Minute
		if len(inactiveDuplicates) > 0 {
			for _, dup := range inactiveDuplicates {
				if time.Since(dup.CreatedAt) < deduplicationWindow {
					respondError(w, http.StatusConflict,
						fmt.Sprintf("A workflow with identical prompt was created %s ago (workflow %s, status: %s). Please wait %s before creating another.",
							time.Since(dup.CreatedAt).Round(time.Second),
							dup.WorkflowID,
							dup.Status,
							(deduplicationWindow-time.Since(dup.CreatedAt)).Round(time.Second)))
					return
				}
			}
		}

		// Warn about other inactive duplicates (completed/failed, older than deduplication window)
		if len(inactiveDuplicates) > 0 {
			duplicateWarning = fmt.Sprintf("Found %d completed/failed workflow(s) with identical prompt: ", len(inactiveDuplicates))
			for i, dup := range inactiveDuplicates {
				if i > 0 {
					duplicateWarning += ", "
				}
				duplicateWarning += fmt.Sprintf("%s (%s)", dup.WorkflowID, dup.Status)
				if i >= 2 && len(inactiveDuplicates) > 3 {
					duplicateWarning += fmt.Sprintf(" and %d more", len(inactiveDuplicates)-3)
					break
				}
			}
		}
	}

	// Validate execution mode configuration
	if req.Blueprint != nil {
		cfg, err := s.loadConfigForContext(ctx)
		if err != nil {
			s.logger.Error("failed to load config for validation", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to load configuration")
			return
		}
		if validationErr := ValidateBlueprint(req.Blueprint, cfg.Agents); validationErr != nil {
			respondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
				Message: "Workflow configuration validation failed",
				Errors:  []ValidationFieldError{*validationErr},
			})
			return
		}
	}

	// Generate workflow ID
	workflowID := generateWorkflowID()

	// Create report directory eagerly to ensure it exists before execution
	// This prevents issues where ReportPath is empty if execution fails early
	projectRoot := s.getProjectRootPath(ctx)
	reportPath := filepath.Join(".quorum", "runs", string(workflowID))
	fullReportPath := filepath.Join(projectRoot, reportPath)
	if err := os.MkdirAll(fullReportPath, 0o750); err != nil {
		s.logger.Warn("failed to create report directory", "path", fullReportPath, "error", err)
		// Continue anyway - the directory will be created during execution
	}

	// Build workflow blueprint
	blueprint := &core.Blueprint{
		Consensus: core.BlueprintConsensus{
			Threshold: 0.75,
		},
		MaxRetries: 3,
		Timeout:    time.Hour,
	}

	if req.Blueprint != nil {
		if req.Blueprint.ConsensusThreshold > 0 {
			blueprint.Consensus.Threshold = req.Blueprint.ConsensusThreshold
		}
		if req.Blueprint.MaxRetries > 0 {
			blueprint.MaxRetries = req.Blueprint.MaxRetries
		}
		if req.Blueprint.TimeoutSeconds > 0 {
			blueprint.Timeout = time.Duration(req.Blueprint.TimeoutSeconds) * time.Second
		}
		blueprint.DryRun = req.Blueprint.DryRun
		blueprint.Sandbox = req.Blueprint.Sandbox
		blueprint.ExecutionMode = req.Blueprint.ExecutionMode
		blueprint.SingleAgent = core.BlueprintSingleAgent{
			Agent:           req.Blueprint.SingleAgentName,
			Model:           req.Blueprint.SingleAgentModel,
			ReasoningEffort: req.Blueprint.SingleAgentReasoningEffort,
		}
	}

	// Create workflow state
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Version:    core.CurrentStateVersion,
			WorkflowID: workflowID,
			Title:      req.Title,
			Prompt:     req.Prompt,
			Blueprint:  blueprint,
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:         core.WorkflowStatusPending,
			CurrentPhase:   core.PhaseAnalyze,
			Tasks:          make(map[core.TaskID]*core.TaskState),
			TaskOrder:      make([]core.TaskID, 0),
			Metrics:        &core.StateMetrics{},
			Checkpoints:    make([]core.Checkpoint, 0),
			UpdatedAt:      time.Now(),
			ResumeCount:    0,
			MaxResumes:     3, // Enable auto-resume with default max 3 attempts
			KanbanColumn:   "refinement",
			KanbanPosition: 0,
			ReportPath:     reportPath, // Set eagerly to ensure it exists even if execution fails early
		},
	}

	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	response := s.stateToWorkflowResponse(ctx, state, workflowID)
	if duplicateWarning != "" {
		response.Warning = duplicateWarning
	}
	respondJSON(w, http.StatusCreated, response)
}

// handleUpdateWorkflow updates an existing workflow.
func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	var req struct {
		Title     string          `json:"title,omitempty"`
		Prompt    string          `json:"prompt,omitempty"`
		Status    string          `json:"status,omitempty"`
		Phase     string          `json:"phase,omitempty"`
		Blueprint *blueprintPatch `json:"blueprint,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Only allow editing prompt when workflow is pending
	// Title can be edited anytime (except when running)
	if req.Prompt != "" && state.Status != core.WorkflowStatusPending {
		respondError(w, http.StatusConflict, "cannot edit prompt after workflow has started")
		return
	}
	if req.Title != "" && state.Status == core.WorkflowStatusRunning {
		respondError(w, http.StatusConflict, "cannot edit title while workflow is running")
		return
	}
	if req.Blueprint != nil && state.Status != core.WorkflowStatusPending {
		respondError(w, http.StatusConflict, "cannot edit workflow blueprint after workflow has started")
		return
	}

	if req.Title != "" {
		state.Title = req.Title
	}
	if req.Prompt != "" {
		state.Prompt = req.Prompt
	}
	if req.Blueprint != nil {
		if state.Blueprint == nil {
			state.Blueprint = &core.Blueprint{
				Consensus: core.BlueprintConsensus{
					Threshold: 0.75,
				},
				MaxRetries: 3,
				Timeout:    time.Hour,
			}
		}

		merged := &BlueprintDTO{
			ExecutionMode:              state.Blueprint.ExecutionMode,
			SingleAgentName:            state.Blueprint.SingleAgent.Agent,
			SingleAgentModel:           state.Blueprint.SingleAgent.Model,
			SingleAgentReasoningEffort: state.Blueprint.SingleAgent.ReasoningEffort,
		}
		if req.Blueprint.ExecutionMode != nil {
			merged.ExecutionMode = *req.Blueprint.ExecutionMode
		}
		if req.Blueprint.SingleAgentName != nil {
			merged.SingleAgentName = *req.Blueprint.SingleAgentName
		}
		if req.Blueprint.SingleAgentModel != nil {
			merged.SingleAgentModel = *req.Blueprint.SingleAgentModel
		}
		if req.Blueprint.SingleAgentReasoningEffort != nil {
			merged.SingleAgentReasoningEffort = *req.Blueprint.SingleAgentReasoningEffort
		}

		cfg, err := s.loadConfigForContext(ctx)
		if err != nil {
			s.logger.Error("failed to load config for validation", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to load configuration")
			return
		}
		if validationErr := ValidateBlueprint(merged, cfg.Agents); validationErr != nil {
			respondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
				Message: "Workflow configuration validation failed",
				Errors:  []ValidationFieldError{*validationErr},
			})
			return
		}

		state.Blueprint.ExecutionMode = merged.ExecutionMode
		state.Blueprint.SingleAgent.Agent = merged.SingleAgentName
		state.Blueprint.SingleAgent.Model = merged.SingleAgentModel
		state.Blueprint.SingleAgent.ReasoningEffort = merged.SingleAgentReasoningEffort
	}
	if req.Status != "" {
		state.Status = core.WorkflowStatus(req.Status)
	}
	if req.Phase != "" {
		state.CurrentPhase = core.Phase(req.Phase)
	}
	state.UpdatedAt = time.Now()

	if err := stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to update workflow")
		return
	}

	activeID, _ := stateManager.GetActiveWorkflowID(ctx)
	response := s.stateToWorkflowResponse(ctx, state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleDeleteWorkflow deletes a workflow.
func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")

	// Load workflow to check it exists and is not running
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil || state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Prevent deletion of running workflows
	if state.Status == core.WorkflowStatusRunning {
		respondError(w, http.StatusConflict, "cannot delete running workflow")
		return
	}

	// Delete the workflow
	if err := stateManager.DeleteWorkflow(ctx, core.WorkflowID(workflowID)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Best-effort cleanup of workflow attachments on disk.
	if s.attachments != nil {
		if err := s.attachments.DeleteAll(attachments.OwnerWorkflow, workflowID); err != nil {
			s.logger.Warn("failed to delete workflow attachments", "workflow_id", workflowID, "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleActivateWorkflow sets a workflow as active.
func (s *Server) handleActivateWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	// Verify workflow exists
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	if err := stateManager.SetActiveWorkflowID(ctx, core.WorkflowID(workflowID)); err != nil {
		s.logger.Error("failed to set active workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to activate workflow")
		return
	}

	response := s.stateToWorkflowResponse(ctx, state, core.WorkflowID(workflowID))
	respondJSON(w, http.StatusOK, response)
}

// stateToWorkflowResponse converts a WorkflowState to a WorkflowResponse.
// This is a Server method to access the unified workflow running check.
func (s *Server) stateToWorkflowResponse(ctx context.Context, state *core.WorkflowState, activeID core.WorkflowID) WorkflowResponse {
	resp := WorkflowResponse{
		ID:              string(state.WorkflowID),
		ExecutionID:     state.ExecutionID,
		Title:           state.Title,
		Status:          string(state.Status),
		CurrentPhase:    string(state.CurrentPhase),
		Prompt:          state.Prompt,
		OptimizedPrompt: state.OptimizedPrompt,
		Attachments:     state.Attachments,
		Error:           state.Error,
		ReportPath:      state.ReportPath,
		CreatedAt:       state.CreatedAt,
		UpdatedAt:       state.UpdatedAt,
		HeartbeatAt:     state.HeartbeatAt,
		IsActive:        state.WorkflowID == activeID,
		ActuallyRunning: s.isWorkflowRunning(ctx, string(state.WorkflowID)),
		TaskCount:       len(state.Tasks),
		AgentEvents:     state.AgentEvents,
	}

	if state.Metrics != nil {
		resp.Metrics = &Metrics{
			TotalTokensIn:  state.Metrics.TotalTokensIn,
			TotalTokensOut: state.Metrics.TotalTokensOut,
			ConsensusScore: state.Metrics.ConsensusScore,
		}
	}

	// Populate tasks in order for frontend to restore state on reload
	if len(state.TaskOrder) > 0 {
		resp.Tasks = make([]TaskResponse, 0, len(state.TaskOrder))
		for _, taskID := range state.TaskOrder {
			if task, ok := state.Tasks[taskID]; ok {
				resp.Tasks = append(resp.Tasks, taskStateToResponse(task))
			}
		}
	}

	if state.Blueprint != nil {
		resp.Blueprint = &BlueprintDTO{
			ConsensusThreshold:         state.Blueprint.Consensus.Threshold,
			MaxRetries:                 state.Blueprint.MaxRetries,
			TimeoutSeconds:             int(state.Blueprint.Timeout.Seconds()),
			DryRun:                     state.Blueprint.DryRun,
			Sandbox:                    state.Blueprint.Sandbox,
			ExecutionMode:              state.Blueprint.ExecutionMode,
			SingleAgentName:            state.Blueprint.SingleAgent.Agent,
			SingleAgentModel:           state.Blueprint.SingleAgent.Model,
			SingleAgentReasoningEffort: state.Blueprint.SingleAgent.ReasoningEffort,
		}
	}

	return resp
}

// generateWorkflowID creates a new workflow ID.
// Format: wf-YYYYMMDD-HHMMSS-xxxxx (e.g., wf-20250121-153045-k7m9p)
// Uses UTC for consistency and a random suffix for uniqueness.
func generateWorkflowID() core.WorkflowID {
	now := time.Now().UTC()
	return core.WorkflowID(fmt.Sprintf("wf-%s-%s", now.Format("20060102-150405"), randomSuffix(5)))
}

// randomSuffix generates a random alphanumeric suffix of the given length.
// Uses base36 (0-9, a-z) for URL-safe, human-readable identifiers.
func randomSuffix(length int) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp nanoseconds if crypto/rand fails
		return fmt.Sprintf("%05d", time.Now().UnixNano()%100000)
	}
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// handleDownloadWorkflow archives and downloads the workflow report directory.
// GET /api/v1/workflows/{workflowID}/download
func (s *Server) handleDownloadWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID required")
		return
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil || state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	reportPath := state.ReportPath
	if reportPath == "" {
		respondError(w, http.StatusNotFound, "no report artifacts found")
		return
	}

	// Verify report path exists
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		respondError(w, http.StatusNotFound, "report directory does not exist")
		return
	}

	// Set headers for zip download
	filename := fmt.Sprintf("%s-artifacts.zip", workflowID)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Walk the report directory and add files to zip
	err = filepath.Walk(reportPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Create relative path for zip entry
		relPath, err := filepath.Rel(reportPath, path)
		if err != nil {
			return err
		}

		zipFile, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		return err
	})

	if err != nil {
		s.logger.Error("failed to stream zip", "workflow_id", workflowID, "error", err)
		// Cannot send error response as headers are already sent
		return
	}
}

// HandleRunWorkflow starts execution of a workflow.
// POST /api/v1/workflows/{workflowID}/run
//
// Returns:
//   - 202 Accepted: Workflow execution started
//   - 404 Not Found: Workflow not found
//   - 409 Conflict: Workflow already running or completed
//   - 503 Service Unavailable: Execution not available (missing dependencies)
func (s *Server) HandleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	// Get workflow ID from URL
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	// Check if state manager is available
	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	// Load workflow state for initial validation and response
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Determine if this is a resume based on original state before validation
	isResume := state.Status == core.WorkflowStatusFailed ||
		state.Status == core.WorkflowStatusPaused ||
		len(state.Checkpoints) > 0

	// Use WorkflowExecutor if available (preferred path with heartbeat support)
	if s.executor != nil {
		var execErr error
		if isResume {
			execErr = s.executor.Resume(ctx, core.WorkflowID(workflowID))
		} else {
			execErr = s.executor.Run(ctx, core.WorkflowID(workflowID))
		}

		if execErr != nil {
			// Map errors to HTTP status codes
			errMsg := execErr.Error()
			switch {
			case errMsg == "workflow is already running" || errMsg == "workflow execution already in progress":
				respondError(w, http.StatusConflict, errMsg)
			case errMsg == "workflow is already completed":
				respondError(w, http.StatusConflict, errMsg+"; create a new workflow to re-run")
			case strings.Contains(errMsg, "workflow not found"):
				respondError(w, http.StatusNotFound, "workflow not found")
			case strings.Contains(errMsg, "missing configuration"):
				respondError(w, http.StatusServiceUnavailable, errMsg)
			default:
				respondError(w, http.StatusInternalServerError, errMsg)
			}
			return
		}

		// Return 202 Accepted - executor started the workflow
		response := RunWorkflowResponse{
			ID:           workflowID,
			Status:       string(core.WorkflowStatusRunning),
			CurrentPhase: string(state.CurrentPhase),
			Prompt:       state.Prompt,
			Message:      "Workflow execution starting",
		}
		if isResume {
			response.Message = "Workflow execution resuming"
			response.CurrentPhase = string(core.PhaseExecute)
		}
		respondJSON(w, http.StatusAccepted, response)
		return
	}

	// Validate workflow state for execution first (before checking tracker)
	// This ensures proper error codes for invalid states
	switch state.Status {
	case core.WorkflowStatusRunning:
		respondError(w, http.StatusConflict, "workflow is already running")
		return
	case core.WorkflowStatusCompleted:
		respondError(w, http.StatusConflict, "workflow is already completed; create a new workflow to re-run")
		return
	case core.WorkflowStatusPending, core.WorkflowStatusFailed, core.WorkflowStatusPaused:
		// These states allow execution
	default:
		respondError(w, http.StatusConflict, "workflow is in invalid state for execution: "+string(state.Status))
		return
	}

	// Direct execution path using UnifiedTracker
	// Require UnifiedTracker for proper state synchronization
	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing tracker")
		return
	}

	// Start execution atomically using UnifiedTracker
	// This marks the workflow as running in DB and creates ControlPlane
	handle, err := s.unifiedTracker.StartExecution(ctx, core.WorkflowID(workflowID))
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			respondError(w, http.StatusConflict, "workflow execution already in progress")
		} else {
			s.logger.Error("failed to start execution", "workflow_id", workflowID, "error", err)
			respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		}
		return
	}

	// Get runner factory
	factory := s.RunnerFactoryForContext(ctx)
	if factory == nil {
		// Rollback the execution start
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "missing configuration")
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing configuration")
		return
	}

	// Create runner with execution context that preserves ProjectContext values.
	// context.WithoutCancel detaches from HTTP request cancellation while maintaining
	// access to project-scoped resources (StateManager, EventBus, etc.)
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Hour)
	handle.SetExecCancel(cancel)

	// Reload state (it was updated by StartExecution)
	state, err = stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to reload state")
		respondError(w, http.StatusInternalServerError, "failed to reload workflow state")
		return
	}

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to create runner: "+err.Error())
		s.logger.Error("failed to create runner", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	// Connect notifier to state for agent event persistence
	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	// Start execution in background
	go func() {
		// Confirm that the goroutine has started
		handle.ConfirmStarted()

		// Emit workflow started event
		notifier.WorkflowStarted(state.Prompt)

		// Execute the workflow (tracker cleanup happens in defer)
		s.executeWorkflowAsync(execCtx, cancel, runner, notifier, state, isResume, workflowID, handle)
	}()

	// Wait for confirmation that goroutine started (with timeout)
	if err := handle.WaitForConfirmation(s.unifiedTracker.ConfirmTimeout()); err != nil {
		s.logger.Error("workflow start confirmation timeout", "workflow_id", workflowID, "error", err)
		// Don't rollback - the goroutine might have started but confirmation was slow
		// The heartbeat system will detect if it's actually dead
	}

	// Return 202 Accepted with "running" status
	// DB is already updated, so any GET will return consistent state
	response := RunWorkflowResponse{
		ID:           workflowID,
		Status:       string(core.WorkflowStatusRunning),
		CurrentPhase: string(state.CurrentPhase),
		Prompt:       state.Prompt,
		Message:      "Workflow execution starting",
	}
	if isResume {
		response.Message = "Workflow execution resuming"
		response.CurrentPhase = string(core.PhaseExecute)
	}

	respondJSON(w, http.StatusAccepted, response)
}

// executeWorkflowAsync runs the workflow in a background goroutine.
func (s *Server) executeWorkflowAsync(
	ctx context.Context,
	cancel context.CancelFunc,
	runner *workflow.Runner,
	notifier *webadapters.WebOutputNotifier,
	state *core.WorkflowState,
	isResume bool,
	workflowID string,
	handle *ExecutionHandle,
) {
	// Capture cleanup context at the start to preserve ProjectContext for FinishExecution.
	// This ensures cleanup happens in the correct project's DB even if the execution times out.
	cleanupCtx := context.WithoutCancel(ctx)

	defer cancel()
	defer notifier.FlushState() // Ensure all pending agent events are saved

	// Cleanup tracking when done
	if s.unifiedTracker != nil && handle != nil {
		defer func() {
			_ = s.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID))
		}()
	}

	startTime := time.Now()
	var runErr error

	// Execute workflow using state-aware methods to avoid duplicate workflow creation.
	// These methods use the pre-created state instead of generating a new workflow ID.
	if isResume {
		runErr = runner.ResumeWithState(ctx, state)
	} else {
		runErr = runner.RunWithState(ctx, state)
	}

	duration := time.Since(startTime)

	// Emit lifecycle event
	if runErr != nil {
		s.logger.Error("workflow execution failed",
			"workflow_id", state.WorkflowID,
			"error", runErr,
		)
		notifier.WorkflowFailed(string(state.CurrentPhase), runErr)
	} else {
		s.logger.Info("workflow execution completed",
			"workflow_id", state.WorkflowID,
			"duration", duration,
		)
		notifier.WorkflowCompleted(duration)
	}
}

// WorkflowControlResponse is returned for workflow control operations (cancel, pause, resume).
type WorkflowControlResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// handleCancelWorkflow cancels a running workflow.
// POST /api/v1/workflows/{workflowID}/cancel
func (s *Server) handleCancelWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID required")
		return
	}

	// Use UnifiedTracker for cancel operation
	if s.unifiedTracker != nil {
		if err := s.unifiedTracker.Cancel(core.WorkflowID(workflowID)); err != nil {
			if strings.Contains(err.Error(), "not running") {
				respondError(w, http.StatusConflict, "workflow is not running")
			} else if strings.Contains(err.Error(), "already being cancelled") {
				respondError(w, http.StatusConflict, "workflow is already being cancelled")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else if s.executor != nil {
		// Fallback to executor if tracker not available
		if err := s.executor.Cancel(workflowID); err != nil {
			if strings.Contains(err.Error(), "not running") {
				respondError(w, http.StatusConflict, "workflow is not running")
			} else if strings.Contains(err.Error(), "already being cancelled") {
				respondError(w, http.StatusConflict, "workflow is already being cancelled")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else {
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Return success response (actual state change happens asynchronously)
	respondJSON(w, http.StatusAccepted, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "cancelling",
		Message: "Workflow cancellation requested. In-flight agent processes will be interrupted.",
	})
}

// handleForceStopWorkflow forcibly stops a workflow, even if it doesn't have an active handle.
// This is useful for zombie workflows that appear running in the DB but have no in-memory state.
// POST /api/v1/workflows/{workflowID}/force-stop
func (s *Server) handleForceStopWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID required")
		return
	}

	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	if err := s.unifiedTracker.ForceStop(r.Context(), core.WorkflowID(workflowID)); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "stopped",
		Message: "Workflow has been forcibly stopped.",
	})
}

// handlePauseWorkflow pauses a running workflow.
// POST /api/v1/workflows/{workflowID}/pause
func (s *Server) handlePauseWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID required")
		return
	}

	// Use UnifiedTracker for pause operation
	if s.unifiedTracker != nil {
		if err := s.unifiedTracker.Pause(core.WorkflowID(workflowID)); err != nil {
			if strings.Contains(err.Error(), "not running") {
				respondError(w, http.StatusConflict, "workflow is not running")
			} else if strings.Contains(err.Error(), "already paused") {
				respondError(w, http.StatusConflict, "workflow is already paused")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else if s.executor != nil {
		// Fallback to executor if tracker not available
		if err := s.executor.Pause(workflowID); err != nil {
			if strings.Contains(err.Error(), "not running") {
				respondError(w, http.StatusConflict, "workflow is not running")
			} else if strings.Contains(err.Error(), "already paused") {
				respondError(w, http.StatusConflict, "workflow is already paused")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else {
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Update persisted state
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err == nil && state != nil {
		state.Status = core.WorkflowStatusPaused
		state.UpdatedAt = time.Now()
		if saveErr := stateManager.Save(ctx, state); saveErr != nil {
			s.logger.Warn("failed to persist paused status", "workflow_id", workflowID, "error", saveErr)
		}
	}

	respondJSON(w, http.StatusOK, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "paused",
		Message: "Workflow paused. Running tasks will complete, then execution will pause.",
	})
}

// handleResumeWorkflow resumes a paused workflow.
// POST /api/v1/workflows/{workflowID}/resume
func (s *Server) handleResumeWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID required")
		return
	}

	// Use UnifiedTracker for resume operation
	if s.unifiedTracker != nil {
		if err := s.unifiedTracker.Resume(core.WorkflowID(workflowID)); err != nil {
			if strings.Contains(err.Error(), "not running") {
				respondError(w, http.StatusConflict, "workflow is not running")
			} else if strings.Contains(err.Error(), "not paused") {
				respondError(w, http.StatusConflict, "workflow is not paused")
			} else {
				respondError(w, http.StatusInternalServerError, err.Error())
			}
			return
		}
	} else {
		// Without UnifiedTracker, cannot resume
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Update persisted state
	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err == nil && state != nil {
		state.Status = core.WorkflowStatusRunning
		state.Error = "" // Clear previous error on resume
		state.UpdatedAt = time.Now()
		if saveErr := stateManager.Save(ctx, state); saveErr != nil {
			s.logger.Warn("failed to persist running status", "workflow_id", workflowID, "error", saveErr)
		}
	}

	respondJSON(w, http.StatusOK, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "running",
		Message: "Workflow resumed. Execution will continue.",
	})
}

// HandleAnalyzeWorkflow executes only the analyze phase of a workflow.
// POST /api/v1/workflows/{workflowID}/analyze
//
// This endpoint runs the refine (if enabled) and analyze phases without
// proceeding to plan or execute. After completion, CurrentPhase will be "plan".
//
// Returns:
//   - 202 Accepted: Analyze phase execution started
//   - 400 Bad Request: Invalid workflow ID
//   - 404 Not Found: Workflow not found
//   - 409 Conflict: Workflow already running, already analyzed, or in invalid state
//   - 503 Service Unavailable: Execution not available
func (s *Server) HandleAnalyzeWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	// Load workflow state
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Validate workflow state for analyze phase
	switch state.Status {
	case core.WorkflowStatusRunning:
		respondError(w, http.StatusConflict, "workflow is already running")
		return
	case core.WorkflowStatusCompleted:
		// Check if analyze already done
		if state.CurrentPhase != core.PhaseRefine && state.CurrentPhase != core.PhaseAnalyze && state.CurrentPhase != "" {
			respondError(w, http.StatusConflict, "analyze phase already completed; use /plan to continue")
			return
		}
	case core.WorkflowStatusPending:
		// Valid for starting analyze
	case core.WorkflowStatusFailed:
		// Allow retry of analyze phase
	default:
		respondError(w, http.StatusConflict, "workflow is in invalid state for analyze: "+string(state.Status))
		return
	}

	// Use UnifiedTracker for execution
	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing tracker")
		return
	}

	// Start execution atomically using UnifiedTracker
	handle, err := s.unifiedTracker.StartExecution(ctx, core.WorkflowID(workflowID))
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			respondError(w, http.StatusConflict, "workflow execution already in progress")
		} else {
			s.logger.Error("failed to start execution", "workflow_id", workflowID, "error", err)
			respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		}
		return
	}

	// Get runner factory
	factory := s.RunnerFactoryForContext(ctx)
	if factory == nil {
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "missing configuration")
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing configuration")
		return
	}

	// Create execution context that preserves ProjectContext values.
	// context.WithoutCancel detaches from HTTP request cancellation while maintaining
	// access to project-scoped resources (StateManager, EventBus, etc.)
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Hour)
	handle.SetExecCancel(cancel)

	// Reload state (it was updated by StartExecution)
	state, err = stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to reload state")
		respondError(w, http.StatusInternalServerError, "failed to reload workflow state")
		return
	}

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to create runner: "+err.Error())
		s.logger.Error("failed to create runner", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	// Execute analyze phase in background
	go func() {
		// Capture cleanup context to preserve ProjectContext for cleanup operations
		cleanupCtx := context.WithoutCancel(execCtx)

		defer cancel()
		defer notifier.FlushState()
		defer func() {
			_ = s.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID))
		}()

		handle.ConfirmStarted()
		notifier.WorkflowStarted(state.Prompt)

		err := runner.AnalyzeWithState(execCtx, state)
		if err != nil {
			s.logger.Error("analyze phase failed", "workflow_id", workflowID, "error", err)
			notifier.WorkflowFailed(string(core.PhaseAnalyze), err)
			return
		}

		// Load final state and emit completion
		finalState, _ := stateManager.LoadByID(cleanupCtx, core.WorkflowID(workflowID))
		if finalState != nil {
			notifier.PhaseCompleted(string(core.PhaseAnalyze), time.Since(state.UpdatedAt))
		}
	}()

	// Wait for confirmation
	if err := handle.WaitForConfirmation(s.unifiedTracker.ConfirmTimeout()); err != nil {
		s.logger.Error("workflow start confirmation timeout", "workflow_id", workflowID, "error", err)
	}

	response := PhaseResponse{
		ID:           workflowID,
		Status:       string(core.WorkflowStatusRunning),
		CurrentPhase: string(core.PhaseAnalyze),
		Message:      "Analyze phase execution started",
	}
	respondJSON(w, http.StatusAccepted, response)
}

// HandlePlanWorkflow executes only the plan phase of a workflow.
// POST /api/v1/workflows/{workflowID}/plan
//
// Requires a completed analyze phase with consolidated analysis.
// After completion, CurrentPhase will be "execute".
//
// Returns:
//   - 202 Accepted: Plan phase execution started
//   - 400 Bad Request: Invalid workflow ID
//   - 404 Not Found: Workflow not found
//   - 409 Conflict: Workflow running, missing analysis, or in invalid state
//   - 503 Service Unavailable: Execution not available
func (s *Server) HandlePlanWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Validate state for plan phase
	switch state.Status {
	case core.WorkflowStatusRunning:
		respondError(w, http.StatusConflict, "workflow is already running")
		return
	case core.WorkflowStatusCompleted:
		// Valid if CurrentPhase is plan (analyze done) or already at execute
		if state.CurrentPhase == core.PhaseExecute {
			respondError(w, http.StatusConflict, "plan phase already completed; use /execute to continue")
			return
		}
		if state.CurrentPhase != core.PhasePlan {
			respondError(w, http.StatusConflict, "workflow must complete analyze phase first; use /analyze")
			return
		}
	case core.WorkflowStatusPending:
		respondError(w, http.StatusConflict, "workflow must complete analyze phase first; use /analyze")
		return
	case core.WorkflowStatusFailed:
		// Allow retry if analyze was completed
		if state.CurrentPhase != core.PhasePlan {
			respondError(w, http.StatusConflict, "analyze phase not completed; use /analyze first")
			return
		}
	default:
		respondError(w, http.StatusConflict, "workflow is in invalid state: "+string(state.Status))
		return
	}

	// Use UnifiedTracker for execution
	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing tracker")
		return
	}

	handle, err := s.unifiedTracker.StartExecution(ctx, core.WorkflowID(workflowID))
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			respondError(w, http.StatusConflict, "workflow execution already in progress")
		} else {
			respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		}
		return
	}

	factory := s.RunnerFactoryForContext(ctx)
	if factory == nil {
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "missing configuration")
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing configuration")
		return
	}

	// Create execution context that preserves ProjectContext values.
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Hour)
	handle.SetExecCancel(cancel)

	state, err = stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to reload state")
		respondError(w, http.StatusInternalServerError, "failed to reload workflow state")
		return
	}

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to create runner: "+err.Error())
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	go func() {
		// Capture cleanup context to preserve ProjectContext for cleanup operations
		cleanupCtx := context.WithoutCancel(execCtx)

		defer cancel()
		defer notifier.FlushState()
		defer func() {
			_ = s.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID))
		}()

		handle.ConfirmStarted()
		notifier.PhaseStarted(core.PhasePlan)

		err := runner.PlanWithState(execCtx, state)
		if err != nil {
			s.logger.Error("plan phase failed", "workflow_id", workflowID, "error", err)
			notifier.WorkflowFailed(string(core.PhasePlan), err)
			return
		}

		notifier.PhaseCompleted(string(core.PhasePlan), time.Since(state.UpdatedAt))
	}()

	if err := handle.WaitForConfirmation(s.unifiedTracker.ConfirmTimeout()); err != nil {
		s.logger.Error("workflow start confirmation timeout", "workflow_id", workflowID, "error", err)
	}

	response := PhaseResponse{
		ID:           workflowID,
		Status:       string(core.WorkflowStatusRunning),
		CurrentPhase: string(core.PhasePlan),
		Message:      "Plan phase execution started",
	}
	respondJSON(w, http.StatusAccepted, response)
}

// HandleReplanWorkflow clears existing plan and regenerates with optional context.
// POST /api/v1/workflows/{workflowID}/replan
//
// Request Body (optional):
//
//	{ "context": "Additional context to prepend to analysis" }
//
// Requires a completed analyze phase. Clears existing tasks and regenerates.
//
// Returns:
//   - 202 Accepted: Replan execution started
//   - 400 Bad Request: Invalid request
//   - 404 Not Found: Workflow not found
//   - 409 Conflict: Workflow running or missing analysis
//   - 503 Service Unavailable: Execution not available
func (s *Server) HandleReplanWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	// Parse optional request body
	var req ReplanRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Validate state - must have completed analyze
	switch state.Status {
	case core.WorkflowStatusRunning:
		respondError(w, http.StatusConflict, "workflow is already running")
		return
	case core.WorkflowStatusCompleted:
		// Valid if analyze was done (CurrentPhase is plan or execute)
		if state.CurrentPhase != core.PhasePlan && state.CurrentPhase != core.PhaseExecute {
			respondError(w, http.StatusConflict, "analyze phase must be completed first")
			return
		}
	case core.WorkflowStatusFailed:
		// Allow replan if analyze was completed
		if state.CurrentPhase != core.PhasePlan && state.CurrentPhase != core.PhaseExecute {
			respondError(w, http.StatusConflict, "analyze phase must be completed first")
			return
		}
	default:
		respondError(w, http.StatusConflict, "workflow must complete analyze phase first")
		return
	}

	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing tracker")
		return
	}

	handle, err := s.unifiedTracker.StartExecution(ctx, core.WorkflowID(workflowID))
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			respondError(w, http.StatusConflict, "workflow execution already in progress")
		} else {
			respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		}
		return
	}

	factory := s.RunnerFactoryForContext(ctx)
	if factory == nil {
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "missing configuration")
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available")
		return
	}

	// Create execution context that preserves ProjectContext values.
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Hour)
	handle.SetExecCancel(cancel)

	state, err = stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to reload state")
		respondError(w, http.StatusInternalServerError, "failed to reload workflow state")
		return
	}

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to create runner: "+err.Error())
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	additionalContext := req.Context

	go func() {
		// Capture cleanup context to preserve ProjectContext for cleanup operations
		cleanupCtx := context.WithoutCancel(execCtx)

		defer cancel()
		defer notifier.FlushState()
		defer func() {
			_ = s.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID))
		}()

		handle.ConfirmStarted()
		notifier.PhaseStarted(core.PhasePlan)

		err := runner.ReplanWithState(execCtx, state, additionalContext)
		if err != nil {
			s.logger.Error("replan failed", "workflow_id", workflowID, "error", err)
			notifier.WorkflowFailed(string(core.PhasePlan), err)
			return
		}

		notifier.PhaseCompleted(string(core.PhasePlan), time.Since(state.UpdatedAt))
	}()

	if err := handle.WaitForConfirmation(s.unifiedTracker.ConfirmTimeout()); err != nil {
		s.logger.Error("workflow start confirmation timeout", "workflow_id", workflowID, "error", err)
	}

	response := PhaseResponse{
		ID:           workflowID,
		Status:       string(core.WorkflowStatusRunning),
		CurrentPhase: string(core.PhasePlan),
		Message:      "Replan execution started",
	}
	if additionalContext != "" {
		response.Message = "Replan execution started with additional context"
	}
	respondJSON(w, http.StatusAccepted, response)
}

// HandleExecuteWorkflow executes only the execute phase of a workflow.
// POST /api/v1/workflows/{workflowID}/execute
//
// Requires a completed plan phase with tasks defined.
// This is essentially a resume from the plan phase to execution.
//
// Returns:
//   - 202 Accepted: Execute phase started
//   - 400 Bad Request: Invalid workflow ID
//   - 404 Not Found: Workflow not found
//   - 409 Conflict: Workflow running, no plan, or in invalid state
//   - 503 Service Unavailable: Execution not available
func (s *Server) HandleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	ctx := r.Context()
	stateManager := s.getProjectStateManager(ctx)

	if stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	// Validate state for execute phase
	switch state.Status {
	case core.WorkflowStatusRunning:
		respondError(w, http.StatusConflict, "workflow is already running")
		return
	case core.WorkflowStatusCompleted:
		// Check if there are tasks to execute
		if state.CurrentPhase != core.PhaseExecute {
			if state.CurrentPhase == "" {
				respondError(w, http.StatusConflict, "workflow already fully completed")
				return
			}
			respondError(w, http.StatusConflict, "plan phase must be completed first; use /plan")
			return
		}
		if len(state.Tasks) == 0 {
			respondError(w, http.StatusConflict, "no tasks to execute; run /plan first")
			return
		}
	case core.WorkflowStatusPaused:
		// Allow resume from paused state
	case core.WorkflowStatusFailed:
		// Allow retry if plan was completed
		if state.CurrentPhase != core.PhaseExecute && len(state.Tasks) == 0 {
			respondError(w, http.StatusConflict, "plan phase not completed; use /plan first")
			return
		}
	default:
		respondError(w, http.StatusConflict, "workflow must complete plan phase first")
		return
	}

	if s.unifiedTracker == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing tracker")
		return
	}

	handle, err := s.unifiedTracker.StartExecution(ctx, core.WorkflowID(workflowID))
	if err != nil {
		if strings.Contains(err.Error(), "already running") {
			respondError(w, http.StatusConflict, "workflow execution already in progress")
		} else {
			respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		}
		return
	}

	factory := s.RunnerFactoryForContext(ctx)
	if factory == nil {
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "missing configuration")
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available")
		return
	}

	// Create execution context that preserves ProjectContext values.
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Hour)
	handle.SetExecCancel(cancel)

	state, err = stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to reload state")
		respondError(w, http.StatusInternalServerError, "failed to reload workflow state")
		return
	}

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		_ = s.unifiedTracker.RollbackExecution(ctx, core.WorkflowID(workflowID), "failed to create runner: "+err.Error())
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	go func() {
		// Capture cleanup context to preserve ProjectContext for cleanup operations
		cleanupCtx := context.WithoutCancel(execCtx)

		defer cancel()
		defer notifier.FlushState()
		defer func() {
			_ = s.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID))
		}()

		handle.ConfirmStarted()
		notifier.PhaseStarted(core.PhaseExecute)

		err := runner.ResumeWithState(execCtx, state)
		if err != nil {
			s.logger.Error("execute phase failed", "workflow_id", workflowID, "error", err)
			notifier.WorkflowFailed(string(core.PhaseExecute), err)
			return
		}

		finalState, _ := stateManager.LoadByID(cleanupCtx, core.WorkflowID(workflowID))
		if finalState != nil && finalState.Status == core.WorkflowStatusCompleted {
			notifier.WorkflowCompleted(time.Since(state.UpdatedAt))
		}
	}()

	if err := handle.WaitForConfirmation(s.unifiedTracker.ConfirmTimeout()); err != nil {
		s.logger.Error("workflow start confirmation timeout", "workflow_id", workflowID, "error", err)
	}

	response := PhaseResponse{
		ID:           workflowID,
		Status:       string(core.WorkflowStatusRunning),
		CurrentPhase: string(core.PhaseExecute),
		Message:      "Execute phase started",
	}
	respondJSON(w, http.StatusAccepted, response)
}
