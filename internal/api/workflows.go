package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// WorkflowResponse is the API response for a workflow.
type WorkflowResponse struct {
	ID              string            `json:"id"`
	Title           string            `json:"title,omitempty"`
	Status          string            `json:"status"`
	CurrentPhase    string            `json:"current_phase"`
	Prompt          string            `json:"prompt"`
	OptimizedPrompt string            `json:"optimized_prompt,omitempty"`
	Attachments     []core.Attachment `json:"attachments,omitempty"`
	Error           string            `json:"error,omitempty"`
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
	Config          *WorkflowConfig   `json:"config,omitempty"`       // NEW: Workflow configuration
}

// Metrics represents workflow metrics in API responses.
type Metrics struct {
	TotalCostUSD   float64 `json:"total_cost_usd"`
	TotalTokensIn  int     `json:"total_tokens_in"`
	TotalTokensOut int     `json:"total_tokens_out"`
	ConsensusScore float64 `json:"consensus_score"`
}

// CreateWorkflowRequest is the request body for creating a workflow.
type CreateWorkflowRequest struct {
	Prompt string          `json:"prompt"`
	Title  string          `json:"title,omitempty"`
	Config *WorkflowConfig `json:"config,omitempty"`
}

// WorkflowConfig represents workflow configuration in API requests.
type WorkflowConfig struct {
	ConsensusThreshold float64 `json:"consensus_threshold,omitempty"`
	MaxRetries         int     `json:"max_retries,omitempty"`
	TimeoutSeconds     int     `json:"timeout_seconds,omitempty"`
	DryRun             bool    `json:"dry_run,omitempty"`
	Sandbox            bool    `json:"sandbox,omitempty"`

	// ExecutionMode determines whether to use multi-agent consensus or single-agent mode.
	// Valid values: "multi_agent" (default), "single_agent"
	// When empty, defaults to multi-agent mode (existing behavior).
	ExecutionMode string `json:"execution_mode,omitempty"`

	// SingleAgentName is the name of the agent to use when execution_mode is "single_agent".
	// Required when execution_mode is "single_agent".
	// Must be a configured and enabled agent (e.g., "claude", "gemini", "codex").
	SingleAgentName string `json:"single_agent_name,omitempty"`

	// SingleAgentModel is an optional model override for the single agent.
	// If empty, the agent's default phase model is used.
	SingleAgentModel string `json:"single_agent_model,omitempty"`

	// SingleAgentReasoningEffort is an optional reasoning effort override for the single agent.
	// If empty, the agent's configured defaults are used.
	SingleAgentReasoningEffort string `json:"single_agent_reasoning_effort,omitempty"`
}

type workflowConfigPatch struct {
	ExecutionMode              *string `json:"execution_mode,omitempty"`
	SingleAgentName            *string `json:"single_agent_name,omitempty"`
	SingleAgentModel           *string `json:"single_agent_model,omitempty"`
	SingleAgentReasoningEffort *string `json:"single_agent_reasoning_effort,omitempty"`
}

// IsSingleAgentMode returns true if the workflow is configured for single-agent execution.
func (c *WorkflowConfig) IsSingleAgentMode() bool {
	return c != nil && c.ExecutionMode == "single_agent"
}

// GetExecutionMode returns the execution mode, defaulting to "multi_agent" if not specified.
func (c *WorkflowConfig) GetExecutionMode() string {
	if c == nil || c.ExecutionMode == "" {
		return "multi_agent"
	}
	return c.ExecutionMode
}

// RunWorkflowResponse is the response for starting a workflow.
type RunWorkflowResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	CurrentPhase string `json:"current_phase"`
	Prompt       string `json:"prompt"`
	Message      string `json:"message"`
}

// runningWorkflows tracks workflows currently being executed to prevent double-execution.
var runningWorkflows = struct {
	sync.Mutex
	ids map[string]bool
}{ids: make(map[string]bool)}

// markRunning marks a workflow as running. Returns false if already running.
func markRunning(id string) bool {
	runningWorkflows.Lock()
	defer runningWorkflows.Unlock()
	if runningWorkflows.ids[id] {
		return false
	}
	runningWorkflows.ids[id] = true
	return true
}

// markFinished marks a workflow as no longer running.
func markFinished(id string) {
	runningWorkflows.Lock()
	defer runningWorkflows.Unlock()
	delete(runningWorkflows.ids, id)
}

// isWorkflowActuallyRunning checks if a workflow is tracked in the direct
// execution map (runningWorkflows). This is an internal function - external
// callers should use Server.isWorkflowRunning for unified tracking.
func isWorkflowActuallyRunning(workflowID string) bool {
	runningWorkflows.Lock()
	defer runningWorkflows.Unlock()
	return runningWorkflows.ids[workflowID]
}

// isWorkflowRunning checks if a workflow is running in EITHER tracking system:
// - The WorkflowExecutor (used by Kanban engine and API with executor)
// - The runningWorkflows map (used by direct execution when executor is nil)
// This unified check prevents false negatives when a workflow is tracked in
// one system but not the other.
func (s *Server) isWorkflowRunning(workflowID string) bool {
	// Check executor first (used by Kanban and API when available)
	if s.executor != nil && s.executor.IsRunning(workflowID) {
		return true
	}
	// Check direct execution tracking
	return isWorkflowActuallyRunning(workflowID)
}

// handleListWorkflows returns all workflows.
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	// Return empty list if state manager is not configured
	if s.stateManager == nil {
		respondJSON(w, http.StatusOK, []WorkflowResponse{})
		return
	}

	ctx := r.Context()

	workflows, err := s.stateManager.ListWorkflows(ctx)
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
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	activeID, _ := s.stateManager.GetActiveWorkflowID(ctx)

	response := s.stateToWorkflowResponse(state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleGetActiveWorkflow returns the currently active workflow.
func (s *Server) handleGetActiveWorkflow(w http.ResponseWriter, r *http.Request) {
	// Return 404 if state manager is not configured
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "no active workflow")
		return
	}

	ctx := r.Context()

	activeID, err := s.stateManager.GetActiveWorkflowID(ctx)
	if err != nil {
		s.logger.Error("failed to get active workflow ID", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get active workflow")
		return
	}

	if activeID == "" {
		respondError(w, http.StatusNotFound, "no active workflow")
		return
	}

	state, err := s.stateManager.LoadByID(ctx, activeID)
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

	response := s.stateToWorkflowResponse(state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleCreateWorkflow creates a new workflow.
func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	if s.stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	ctx := r.Context()

	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Prompt == "" {
		respondError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Validate execution mode configuration
	if req.Config != nil {
		cfg, err := s.loadConfig()
		if err != nil {
			s.logger.Error("failed to load config for validation", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to load configuration")
			return
		}
		if validationErr := ValidateWorkflowConfig(req.Config, cfg.Agents); validationErr != nil {
			respondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
				Message: "Workflow configuration validation failed",
				Errors:  []ValidationFieldError{*validationErr},
			})
			return
		}
	}

	// Generate workflow ID
	workflowID := generateWorkflowID()

	// Build workflow config
	config := &core.WorkflowConfig{
		ConsensusThreshold: 0.75,
		MaxRetries:         3,
		Timeout:            time.Hour,
	}

	if req.Config != nil {
		if req.Config.ConsensusThreshold > 0 {
			config.ConsensusThreshold = req.Config.ConsensusThreshold
		}
		if req.Config.MaxRetries > 0 {
			config.MaxRetries = req.Config.MaxRetries
		}
		if req.Config.TimeoutSeconds > 0 {
			config.Timeout = time.Duration(req.Config.TimeoutSeconds) * time.Second
		}
		config.DryRun = req.Config.DryRun
		config.Sandbox = req.Config.Sandbox
		config.ExecutionMode = req.Config.ExecutionMode
		config.SingleAgentName = req.Config.SingleAgentName
		config.SingleAgentModel = req.Config.SingleAgentModel
		config.SingleAgentReasoningEffort = req.Config.SingleAgentReasoningEffort
	}

	// Create workflow state
	state := &core.WorkflowState{
		Version:       core.CurrentStateVersion,
		WorkflowID:    workflowID,
		Title:         req.Title,
		Status:        core.WorkflowStatusPending,
		CurrentPhase:  core.PhaseAnalyze,
		Prompt:        req.Prompt,
		Tasks:         make(map[core.TaskID]*core.TaskState),
		TaskOrder:     make([]core.TaskID, 0),
		Config:        config,
		Metrics:       &core.StateMetrics{},
		Checkpoints:   make([]core.Checkpoint, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ResumeCount:   0,
		MaxResumes:    3, // Enable auto-resume with default max 3 attempts
		KanbanColumn:  "refinement",
		KanbanPosition: 0,
	}

	if err := s.stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	response := s.stateToWorkflowResponse(state, workflowID)
	respondJSON(w, http.StatusCreated, response)
}

// handleUpdateWorkflow updates an existing workflow.
func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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
		Title  string               `json:"title,omitempty"`
		Prompt string               `json:"prompt,omitempty"`
		Status string               `json:"status,omitempty"`
		Phase  string               `json:"phase,omitempty"`
		Config *workflowConfigPatch `json:"config,omitempty"`
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
	if req.Config != nil && state.Status != core.WorkflowStatusPending {
		respondError(w, http.StatusConflict, "cannot edit workflow config after workflow has started")
		return
	}

	if req.Title != "" {
		state.Title = req.Title
	}
	if req.Prompt != "" {
		state.Prompt = req.Prompt
	}
	if req.Config != nil {
		if state.Config == nil {
			state.Config = &core.WorkflowConfig{
				ConsensusThreshold: 0.75,
				MaxRetries:         3,
				Timeout:            time.Hour,
			}
		}

		merged := &WorkflowConfig{
			ExecutionMode:              state.Config.ExecutionMode,
			SingleAgentName:            state.Config.SingleAgentName,
			SingleAgentModel:           state.Config.SingleAgentModel,
			SingleAgentReasoningEffort: state.Config.SingleAgentReasoningEffort,
		}
		if req.Config.ExecutionMode != nil {
			merged.ExecutionMode = *req.Config.ExecutionMode
		}
		if req.Config.SingleAgentName != nil {
			merged.SingleAgentName = *req.Config.SingleAgentName
		}
		if req.Config.SingleAgentModel != nil {
			merged.SingleAgentModel = *req.Config.SingleAgentModel
		}
		if req.Config.SingleAgentReasoningEffort != nil {
			merged.SingleAgentReasoningEffort = *req.Config.SingleAgentReasoningEffort
		}

		cfg, err := s.loadConfig()
		if err != nil {
			s.logger.Error("failed to load config for validation", "error", err)
			respondError(w, http.StatusInternalServerError, "failed to load configuration")
			return
		}
		if validationErr := ValidateWorkflowConfig(merged, cfg.Agents); validationErr != nil {
			respondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
				Message: "Workflow configuration validation failed",
				Errors:  []ValidationFieldError{*validationErr},
			})
			return
		}

		state.Config.ExecutionMode = merged.ExecutionMode
		state.Config.SingleAgentName = merged.SingleAgentName
		state.Config.SingleAgentModel = merged.SingleAgentModel
		state.Config.SingleAgentReasoningEffort = merged.SingleAgentReasoningEffort
	}
	if req.Status != "" {
		state.Status = core.WorkflowStatus(req.Status)
	}
	if req.Phase != "" {
		state.CurrentPhase = core.Phase(req.Phase)
	}
	state.UpdatedAt = time.Now()

	if err := s.stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to update workflow")
		return
	}

	activeID, _ := s.stateManager.GetActiveWorkflowID(ctx)
	response := s.stateToWorkflowResponse(state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleDeleteWorkflow deletes a workflow.
func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	// Load workflow to check it exists and is not running
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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
	if err := s.stateManager.DeleteWorkflow(ctx, core.WorkflowID(workflowID)); err != nil {
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
	if s.stateManager == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	// Verify workflow exists
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		s.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}

	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	if err := s.stateManager.SetActiveWorkflowID(ctx, core.WorkflowID(workflowID)); err != nil {
		s.logger.Error("failed to set active workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to activate workflow")
		return
	}

	response := s.stateToWorkflowResponse(state, core.WorkflowID(workflowID))
	respondJSON(w, http.StatusOK, response)
}

// stateToWorkflowResponse converts a WorkflowState to a WorkflowResponse.
// This is a Server method to access the unified workflow running check.
func (s *Server) stateToWorkflowResponse(state *core.WorkflowState, activeID core.WorkflowID) WorkflowResponse {
	resp := WorkflowResponse{
		ID:              string(state.WorkflowID),
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
		ActuallyRunning: s.isWorkflowRunning(string(state.WorkflowID)),
		TaskCount:       len(state.Tasks),
		AgentEvents:     state.AgentEvents,
	}

	if state.Metrics != nil {
		resp.Metrics = &Metrics{
			TotalCostUSD:   state.Metrics.TotalCostUSD,
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

	if state.Config != nil {
		resp.Config = &WorkflowConfig{
			ConsensusThreshold:         state.Config.ConsensusThreshold,
			MaxRetries:                 state.Config.MaxRetries,
			TimeoutSeconds:             int(state.Config.Timeout.Seconds()),
			DryRun:                     state.Config.DryRun,
			Sandbox:                    state.Config.Sandbox,
			ExecutionMode:              state.Config.ExecutionMode,
			SingleAgentName:            state.Config.SingleAgentName,
			SingleAgentModel:           state.Config.SingleAgentModel,
			SingleAgentReasoningEffort: state.Config.SingleAgentReasoningEffort,
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

	// Check if state manager is available
	if s.stateManager == nil {
		respondError(w, http.StatusServiceUnavailable, "workflow management not available")
		return
	}

	// Load workflow state for initial validation and response
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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

	// Direct execution path (when executor is not available)
	// Validate workflow state for execution
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

	// Prevent double-execution race condition
	if !markRunning(workflowID) {
		respondError(w, http.StatusConflict, "workflow execution already in progress")
		return
	}

	// Get runner factory
	factory := s.RunnerFactory()
	if factory == nil {
		markFinished(workflowID)
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: missing configuration")
		return
	}

	// Create runner with execution context
	// Use background context since the HTTP request will complete before workflow finishes
	execCtx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)

	// Create ControlPlane for this workflow (enables pause/resume/cancel)
	cp := control.New()
	s.registerControlPlane(workflowID, cp)

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, cp, state.Config)
	if err != nil {
		cancel()
		s.unregisterControlPlane(workflowID)
		markFinished(workflowID)
		s.logger.Error("failed to create runner", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusServiceUnavailable, "workflow execution not available: "+err.Error())
		return
	}

	// Connect notifier to state for agent event persistence
	notifier.SetState(state)
	notifier.SetStateSaver(s.stateManager)

	// Update status to "running" BEFORE responding to ensure DB consistency
	// This way, any subsequent GET will return the correct status
	state.Status = core.WorkflowStatusRunning
	state.Error = "" // Clear previous error on restart
	state.UpdatedAt = time.Now()
	// Initialize heartbeat for zombie detection - CRITICAL for direct execution path
	now := time.Now().UTC()
	state.HeartbeatAt = &now
	if err := s.stateManager.Save(r.Context(), state); err != nil {
		s.logger.Error("failed to update workflow status to running", "workflow_id", workflowID, "error", err)
		cancel()
		s.unregisterControlPlane(workflowID)
		markFinished(workflowID)
		respondError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		return
	}

	// Start execution in background
	go func() {
		// Emit workflow started event
		notifier.WorkflowStarted(state.Prompt)

		// Execute the workflow (unregisters ControlPlane on completion)
		s.executeWorkflowAsync(execCtx, cancel, runner, notifier, state, isResume, workflowID)
	}()

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
) {
	defer cancel()
	defer markFinished(workflowID)
	defer s.unregisterControlPlane(workflowID)
	defer notifier.FlushState() // Ensure all pending agent events are saved

	startTime := time.Now()
	var runErr error

	// Execute workflow using state-aware methods to avoid duplicate workflow creation.
	// These methods use the pre-created state instead of generating a new workflow ID.
	if isResume {
		runErr = runner.ResumeWithState(ctx, state)
	} else {
		runErr = runner.RunWithState(ctx, state)
	}

	// Get final state for metrics
	finalState, _ := runner.GetState(ctx)
	duration := time.Since(startTime)
	var totalCost float64
	if finalState != nil && finalState.Metrics != nil {
		totalCost = finalState.Metrics.TotalCostUSD
	}

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
			"cost", totalCost,
		)
		notifier.WorkflowCompleted(duration, totalCost)
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

	// Get the ControlPlane for this workflow
	cp, ok := s.getControlPlane(workflowID)
	if !ok {
		// No ControlPlane means workflow is not running
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Check if already cancelled
	if cp.IsCancelled() {
		respondError(w, http.StatusConflict, "workflow is already being cancelled")
		return
	}

	// Cancel the workflow
	cp.Cancel()
	s.logger.Info("workflow cancellation requested", "workflow_id", workflowID)

	// Return success response (actual state change happens asynchronously)
	respondJSON(w, http.StatusAccepted, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "cancelling",
		Message: "Workflow cancellation requested. The workflow will stop after the current task completes.",
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

	// Get the ControlPlane for this workflow
	cp, ok := s.getControlPlane(workflowID)
	if !ok {
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Check if already paused
	if cp.IsPaused() {
		respondError(w, http.StatusConflict, "workflow is already paused")
		return
	}

	// Check if cancelled
	if cp.IsCancelled() {
		respondError(w, http.StatusConflict, "workflow is being cancelled")
		return
	}

	// Pause the workflow
	cp.Pause()
	s.logger.Info("workflow paused", "workflow_id", workflowID)

	// Update persisted state
	ctx := r.Context()
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err == nil && state != nil {
		state.Status = core.WorkflowStatusPaused
		state.UpdatedAt = time.Now()
		if saveErr := s.stateManager.Save(ctx, state); saveErr != nil {
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

	// Get the ControlPlane for this workflow
	cp, ok := s.getControlPlane(workflowID)
	if !ok {
		respondError(w, http.StatusConflict, "workflow is not running")
		return
	}

	// Check if not paused
	if !cp.IsPaused() {
		respondError(w, http.StatusConflict, "workflow is not paused")
		return
	}

	// Check if cancelled
	if cp.IsCancelled() {
		respondError(w, http.StatusConflict, "workflow is being cancelled")
		return
	}

	// Resume the workflow
	cp.Resume()
	s.logger.Info("workflow resumed", "workflow_id", workflowID)

	// Update persisted state
	ctx := r.Context()
	state, err := s.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err == nil && state != nil {
		state.Status = core.WorkflowStatusRunning
		state.Error = "" // Clear previous error on resume
		state.UpdatedAt = time.Now()
		if saveErr := s.stateManager.Save(ctx, state); saveErr != nil {
			s.logger.Warn("failed to persist running status", "workflow_id", workflowID, "error", saveErr)
		}
	}

	respondJSON(w, http.StatusOK, WorkflowControlResponse{
		ID:      workflowID,
		Status:  "running",
		Message: "Workflow resumed. Execution will continue.",
	})
}
