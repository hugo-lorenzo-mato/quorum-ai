package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// WorkflowResponse is the API response for a workflow.
type WorkflowResponse struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	CurrentPhase string    `json:"current_phase"`
	Prompt       string    `json:"prompt"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	IsActive     bool      `json:"is_active"`
	TaskCount    int       `json:"task_count"`
	Metrics      *Metrics  `json:"metrics,omitempty"`
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
	Config *WorkflowConfig `json:"config,omitempty"`
}

// WorkflowConfig represents workflow configuration in API requests.
type WorkflowConfig struct {
	ConsensusThreshold float64 `json:"consensus_threshold,omitempty"`
	MaxRetries         int     `json:"max_retries,omitempty"`
	TimeoutSeconds     int     `json:"timeout_seconds,omitempty"`
	DryRun             bool    `json:"dry_run,omitempty"`
	Sandbox            bool    `json:"sandbox,omitempty"`
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

	response := stateToWorkflowResponse(state, activeID)
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

	response := stateToWorkflowResponse(state, activeID)
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
	}

	// Create workflow state
	state := &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   workflowID,
		Status:       core.WorkflowStatusPending,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       req.Prompt,
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    make([]core.TaskID, 0),
		Config:       config,
		Metrics:      &core.StateMetrics{},
		Checkpoints:  make([]core.Checkpoint, 0),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.stateManager.Save(ctx, state); err != nil {
		s.logger.Error("failed to save workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create workflow")
		return
	}

	response := stateToWorkflowResponse(state, workflowID)
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
		Status string `json:"status,omitempty"`
		Phase  string `json:"phase,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
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
	response := stateToWorkflowResponse(state, activeID)
	respondJSON(w, http.StatusOK, response)
}

// handleDeleteWorkflow deletes a workflow.
func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, _ *http.Request) {
	// Note: StateManager interface doesn't have a Delete method yet
	// For now, return not implemented
	respondError(w, http.StatusNotImplemented, "delete not implemented")
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

	response := stateToWorkflowResponse(state, core.WorkflowID(workflowID))
	respondJSON(w, http.StatusOK, response)
}

// stateToWorkflowResponse converts a WorkflowState to a WorkflowResponse.
func stateToWorkflowResponse(state *core.WorkflowState, activeID core.WorkflowID) WorkflowResponse {
	resp := WorkflowResponse{
		ID:           string(state.WorkflowID),
		Status:       string(state.Status),
		CurrentPhase: string(state.CurrentPhase),
		Prompt:       state.Prompt,
		CreatedAt:    state.CreatedAt,
		UpdatedAt:    state.UpdatedAt,
		IsActive:     state.WorkflowID == activeID,
		TaskCount:    len(state.Tasks),
	}

	if state.Metrics != nil {
		resp.Metrics = &Metrics{
			TotalCostUSD:   state.Metrics.TotalCostUSD,
			TotalTokensIn:  state.Metrics.TotalTokensIn,
			TotalTokensOut: state.Metrics.TotalTokensOut,
			ConsensusScore: state.Metrics.ConsensusScore,
		}
	}

	return resp
}

// generateWorkflowID creates a new workflow ID.
func generateWorkflowID() core.WorkflowID {
	return core.WorkflowID("wf-" + time.Now().Format("20060102-150405"))
}
