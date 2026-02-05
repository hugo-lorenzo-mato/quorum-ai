package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
)

// KanbanStateManagerAdapter extends core.StateManager with Kanban-specific operations.
// Implementations should embed SQLiteStateManager or provide these methods.
type KanbanStateManagerAdapter interface {
	core.StateManager
	GetNextKanbanWorkflow(ctx context.Context) (*core.WorkflowState, error)
	MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error
	UpdateKanbanStatus(ctx context.Context, workflowID, column, prURL string, prNumber int, lastError string) error
	GetKanbanEngineState(ctx context.Context) (*kanban.KanbanEngineState, error)
	SaveKanbanEngineState(ctx context.Context, state *kanban.KanbanEngineState) error
	ListWorkflowsByKanbanColumn(ctx context.Context, column string) ([]*core.WorkflowState, error)
	GetKanbanBoard(ctx context.Context) (map[string][]*core.WorkflowState, error)
}

// KanbanBoardResponse represents the full Kanban board state.
type KanbanBoardResponse struct {
	Columns map[string][]KanbanWorkflowResponse `json:"columns"`
	Engine  KanbanEngineStateResponse           `json:"engine"`
}

// KanbanWorkflowResponse represents a workflow in Kanban context.
type KanbanWorkflowResponse struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	Status               string     `json:"status"`
	KanbanColumn         string     `json:"kanban_column"`
	KanbanPosition       int        `json:"kanban_position"`
	PRURL                string     `json:"pr_url,omitempty"`
	PRNumber             int        `json:"pr_number,omitempty"`
	KanbanStartedAt      *time.Time `json:"kanban_started_at,omitempty"`
	KanbanCompletedAt    *time.Time `json:"kanban_completed_at,omitempty"`
	KanbanExecutionCount int        `json:"kanban_execution_count"`
	KanbanLastError      string     `json:"kanban_last_error,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	Prompt               string     `json:"prompt"`
	TaskCount            int        `json:"task_count"`
}

// KanbanEngineStateResponse represents the engine state for API responses.
type KanbanEngineStateResponse struct {
	Enabled             bool       `json:"enabled"`
	CurrentWorkflowID   *string    `json:"current_workflow_id,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CircuitBreakerOpen  bool       `json:"circuit_breaker_open"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
}

// MoveWorkflowRequest is the request body for moving a workflow.
type MoveWorkflowRequest struct {
	ToColumn string `json:"to_column"`
	Position int    `json:"position"`
}

// KanbanServer wraps Server with Kanban-specific functionality.
type KanbanServer struct {
	server   *Server
	engine   *kanban.Engine
	eventBus *events.EventBus
}

// NewKanbanServer creates a new Kanban server wrapper.
func NewKanbanServer(server *Server, engine *kanban.Engine, eventBus *events.EventBus) *KanbanServer {
	return &KanbanServer{
		server:   server,
		engine:   engine,
		eventBus: eventBus,
	}
}

// RegisterRoutes adds Kanban routes to the router.
func (ks *KanbanServer) RegisterRoutes(r chi.Router) {
	r.Route("/kanban", func(r chi.Router) {
		// Board operations
		r.Get("/board", ks.handleGetBoard)

		// Workflow operations
		r.Post("/workflows/{workflowID}/move", ks.handleMoveWorkflow)

		// Engine control
		r.Get("/engine", ks.handleGetEngineState)
		r.Post("/engine/enable", ks.handleEnableEngine)
		r.Post("/engine/disable", ks.handleDisableEngine)
		r.Post("/engine/reset-circuit-breaker", ks.handleResetCircuitBreaker)
	})
}

// handleGetBoard returns the full Kanban board state.
func (ks *KanbanServer) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stateManager := ks.server.getProjectStateManager(ctx)

	// Type assert to get Kanban-specific methods
	kanbanMgr, ok := stateManager.(interface {
		GetKanbanBoard(ctx context.Context) (map[string][]*core.WorkflowState, error)
	})
	if !ok {
		respondError(w, http.StatusServiceUnavailable, "Kanban features not available")
		return
	}

	board, err := kanbanMgr.GetKanbanBoard(ctx)
	if err != nil {
		ks.server.logger.Error("failed to get kanban board", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get board")
		return
	}

	// Convert to response format
	columns := make(map[string][]KanbanWorkflowResponse)
	columnNames := []string{"refinement", "todo", "in_progress", "to_verify", "done"}

	for _, col := range columnNames {
		workflows := board[col]
		columnResp := make([]KanbanWorkflowResponse, 0, len(workflows))
		for _, wf := range workflows {
			columnResp = append(columnResp, workflowToKanbanResponse(wf))
		}
		columns[col] = columnResp
	}

	// Get engine state
	var engineResp KanbanEngineStateResponse
	if ks.engine != nil {
		state := ks.engine.GetState()
		engineResp = KanbanEngineStateResponse{
			Enabled:             state.Enabled,
			CurrentWorkflowID:   state.CurrentWorkflowID,
			ConsecutiveFailures: state.ConsecutiveFailures,
			CircuitBreakerOpen:  state.CircuitBreakerOpen,
			LastFailureAt:       state.LastFailureAt,
		}
	}

	response := KanbanBoardResponse{
		Columns: columns,
		Engine:  engineResp,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleMoveWorkflow moves a workflow to a different column.
func (ks *KanbanServer) handleMoveWorkflow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workflowID := chi.URLParam(r, "workflowID")

	if workflowID == "" {
		respondError(w, http.StatusBadRequest, "workflow ID is required")
		return
	}

	var req MoveWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate column
	validColumns := map[string]bool{
		"refinement":  true,
		"todo":        true,
		"in_progress": true,
		"to_verify":   true,
		"done":        true,
	}
	if !validColumns[req.ToColumn] {
		respondError(w, http.StatusBadRequest, "invalid column: "+req.ToColumn)
		return
	}

	stateManager := ks.server.getProjectStateManager(ctx)

	// Type assert to get Kanban-specific methods
	kanbanMgr, ok := stateManager.(interface {
		MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error
		LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)
	})
	if !ok {
		respondError(w, http.StatusServiceUnavailable, "Kanban features not available")
		return
	}

	// Get current state to determine fromColumn
	state, err := kanbanMgr.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		ks.server.logger.Error("failed to load workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load workflow")
		return
	}
	if state == nil {
		respondError(w, http.StatusNotFound, "workflow not found")
		return
	}

	fromColumn := state.KanbanColumn
	if fromColumn == "" {
		fromColumn = "refinement"
	}

	// Prevent moving workflows that are currently executing
	if fromColumn == "in_progress" && ks.engine != nil {
		currentWfID := ks.engine.CurrentWorkflowID()
		if currentWfID != nil && *currentWfID == workflowID {
			respondError(w, http.StatusConflict, "cannot move workflow that is currently executing")
			return
		}
	}

	// Move the workflow
	if err := kanbanMgr.MoveWorkflow(ctx, workflowID, req.ToColumn, req.Position); err != nil {
		ks.server.logger.Error("failed to move workflow", "workflow_id", workflowID, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to move workflow")
		return
	}

	// Emit event for SSE
	if ks.eventBus != nil {
		ks.eventBus.Publish(events.NewKanbanWorkflowMovedEvent(
			workflowID, "", fromColumn, req.ToColumn, req.Position, true,
		))
	}

	// Return updated workflow
	updatedState, _ := kanbanMgr.LoadByID(ctx, core.WorkflowID(workflowID))
	if updatedState != nil {
		respondJSON(w, http.StatusOK, workflowToKanbanResponse(updatedState))
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleGetEngineState returns the current engine state.
func (ks *KanbanServer) handleGetEngineState(w http.ResponseWriter, _ *http.Request) {
	if ks.engine == nil {
		respondError(w, http.StatusServiceUnavailable, "Kanban engine not available")
		return
	}

	state := ks.engine.GetState()
	response := KanbanEngineStateResponse{
		Enabled:             state.Enabled,
		CurrentWorkflowID:   state.CurrentWorkflowID,
		ConsecutiveFailures: state.ConsecutiveFailures,
		CircuitBreakerOpen:  state.CircuitBreakerOpen,
		LastFailureAt:       state.LastFailureAt,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleEnableEngine enables the Kanban execution engine.
func (ks *KanbanServer) handleEnableEngine(w http.ResponseWriter, r *http.Request) {
	if ks.engine == nil {
		respondError(w, http.StatusServiceUnavailable, "Kanban engine not available")
		return
	}

	ctx := r.Context()
	if err := ks.engine.Enable(ctx); err != nil {
		ks.server.logger.Error("failed to enable kanban engine", "error", err)
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "Kanban engine enabled",
	})
}

// handleDisableEngine disables the Kanban execution engine.
func (ks *KanbanServer) handleDisableEngine(w http.ResponseWriter, r *http.Request) {
	if ks.engine == nil {
		respondError(w, http.StatusServiceUnavailable, "Kanban engine not available")
		return
	}

	ctx := r.Context()
	if err := ks.engine.Disable(ctx); err != nil {
		ks.server.logger.Error("failed to disable kanban engine", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": false,
		"message": "Kanban engine disabled",
	})
}

// handleResetCircuitBreaker resets the circuit breaker.
func (ks *KanbanServer) handleResetCircuitBreaker(w http.ResponseWriter, r *http.Request) {
	if ks.engine == nil {
		respondError(w, http.StatusServiceUnavailable, "Kanban engine not available")
		return
	}

	ctx := r.Context()
	if err := ks.engine.ResetCircuitBreaker(ctx); err != nil {
		ks.server.logger.Error("failed to reset circuit breaker", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"circuit_breaker_open": false,
		"message":              "Circuit breaker reset",
	})
}

// workflowToKanbanResponse converts a WorkflowState to KanbanWorkflowResponse.
func workflowToKanbanResponse(wf *core.WorkflowState) KanbanWorkflowResponse {
	kanbanColumn := wf.KanbanColumn
	if kanbanColumn == "" {
		kanbanColumn = "refinement"
	}

	// Truncate prompt for display
	prompt := wf.Prompt
	if len(prompt) > 200 {
		prompt = prompt[:200] + "..."
	}

	return KanbanWorkflowResponse{
		ID:                   string(wf.WorkflowID),
		Title:                wf.Title,
		Status:               string(wf.Status),
		KanbanColumn:         kanbanColumn,
		KanbanPosition:       wf.KanbanPosition,
		PRURL:                wf.PRURL,
		PRNumber:             wf.PRNumber,
		KanbanStartedAt:      wf.KanbanStartedAt,
		KanbanCompletedAt:    wf.KanbanCompletedAt,
		KanbanExecutionCount: wf.KanbanExecutionCount,
		KanbanLastError:      wf.KanbanLastError,
		CreatedAt:            wf.CreatedAt,
		UpdatedAt:            wf.UpdatedAt,
		Prompt:               prompt,
		TaskCount:            len(wf.Tasks),
	}
}

// ParseKanbanPosition parses a position from query parameter.
func ParseKanbanPosition(r *http.Request) int {
	posStr := r.URL.Query().Get("position")
	if posStr == "" {
		return 0
	}
	pos, err := strconv.Atoi(posStr)
	if err != nil || pos < 0 {
		return 0
	}
	return pos
}
