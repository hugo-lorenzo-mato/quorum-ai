package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// WorkflowResponse is the API response for a workflow.
type WorkflowResponse struct {
	ID              string              `json:"id"`
	Title           string              `json:"title,omitempty"`
	Status          string              `json:"status"`
	CurrentPhase    string              `json:"current_phase"`
	Prompt          string              `json:"prompt"`
	OptimizedPrompt string              `json:"optimized_prompt,omitempty"`
	Error           string              `json:"error,omitempty"`
	ReportPath      string              `json:"report_path,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	IsActive        bool                `json:"is_active"`
	TaskCount       int                 `json:"task_count"`
	Metrics         *Metrics            `json:"metrics,omitempty"`
	AgentEvents     []core.AgentEvent   `json:"agent_events,omitempty"` // Persisted agent activity
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
		Title  string `json:"title,omitempty"`
		Prompt string `json:"prompt,omitempty"`
		Status string `json:"status,omitempty"`
		Phase  string `json:"phase,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Only allow editing title and prompt when workflow is pending
	if req.Title != "" || req.Prompt != "" {
		if state.Status != core.WorkflowStatusPending {
			respondError(w, http.StatusConflict, "cannot edit title or prompt after workflow has started")
			return
		}
	}

	if req.Title != "" {
		state.Title = req.Title
	}
	if req.Prompt != "" {
		state.Prompt = req.Prompt
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
		ID:              string(state.WorkflowID),
		Title:           state.Title,
		Status:          string(state.Status),
		CurrentPhase:    string(state.CurrentPhase),
		Prompt:          state.Prompt,
		OptimizedPrompt: state.OptimizedPrompt,
		Error:           state.Error,
		ReportPath:      state.ReportPath,
		CreatedAt:       state.CreatedAt,
		UpdatedAt:       state.UpdatedAt,
		IsActive:        state.WorkflowID == activeID,
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

	// Load workflow state
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

	runner, notifier, err := factory.CreateRunner(execCtx, workflowID, cp)
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

	// Start execution in background
	// Note: Status is updated to "running" INSIDE the goroutine to avoid race condition
	// where status is saved before the goroutine actually starts (causing zombie workflows)
	go func() {
		// Update status to running inside goroutine (after goroutine has started)
		state.Status = core.WorkflowStatusRunning
		state.Error = "" // Clear previous error on restart
		state.UpdatedAt = time.Now()
		if err := s.stateManager.Save(context.Background(), state); err != nil {
			s.logger.Error("failed to update workflow status to running", "workflow_id", workflowID, "error", err)
			notifier.WorkflowFailed(string(state.CurrentPhase), err)
			cancel()
			s.unregisterControlPlane(workflowID)
			markFinished(workflowID)
			return
		}

		// Emit workflow started event
		notifier.WorkflowStarted(state.Prompt)

		// Execute the workflow (unregisters ControlPlane on completion)
		s.executeWorkflowAsync(execCtx, cancel, runner, notifier, state, isResume, workflowID)
	}()

	// Return 202 Accepted
	// Note: Status is "pending" here; it will change to "running" once the goroutine confirms
	// The client will receive status updates via SSE
	response := RunWorkflowResponse{
		ID:           workflowID,
		Status:       string(state.Status), // Will be "pending" or previous status
		CurrentPhase: string(state.CurrentPhase),
		Prompt:       state.Prompt,
		Message:      "Workflow execution starting",
	}
	if isResume {
		response.Message = "Workflow execution resuming"
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

	// Execute workflow
	if isResume {
		runErr = runner.Resume(ctx)
	} else {
		runErr = runner.Run(ctx, state.Prompt)
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
