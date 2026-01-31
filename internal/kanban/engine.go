package kanban

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

const (
	// DefaultTickInterval is the interval between engine loop ticks.
	DefaultTickInterval = 5 * time.Second
)

// KanbanEngineState represents the persisted state of the Kanban engine.
type KanbanEngineState struct {
	Enabled             bool       `json:"enabled"`
	CurrentWorkflowID   *string    `json:"current_workflow_id,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CircuitBreakerOpen  bool       `json:"circuit_breaker_open"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
}

// EngineState represents the current state of the Kanban engine for API responses.
type EngineState struct {
	Enabled             bool       `json:"enabled"`
	CurrentWorkflowID   *string    `json:"current_workflow_id,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CircuitBreakerOpen  bool       `json:"circuit_breaker_open"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty"`
}

// WorkflowExecutor defines the interface for running workflows.
type WorkflowExecutor interface {
	Run(ctx context.Context, workflowID core.WorkflowID) error
}

// KanbanStateManager defines the interface for Kanban-specific state operations.
type KanbanStateManager interface {
	// LoadByID loads a workflow by its ID
	LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)

	// GetNextKanbanWorkflow returns the next workflow from the "todo" column
	// ordered by position (lowest first). Returns nil if no workflows in todo.
	GetNextKanbanWorkflow(ctx context.Context) (*core.WorkflowState, error)

	// MoveWorkflow moves a workflow to a new column with a new position.
	MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error

	// UpdateKanbanStatus updates the Kanban-specific fields on a workflow.
	UpdateKanbanStatus(ctx context.Context, workflowID, column, prURL string, prNumber int, lastError string) error

	// GetKanbanEngineState retrieves the persisted engine state.
	GetKanbanEngineState(ctx context.Context) (*KanbanEngineState, error)

	// SaveKanbanEngineState persists the engine state.
	SaveKanbanEngineState(ctx context.Context, state *KanbanEngineState) error
}

// Engine is the Kanban execution engine that processes workflows sequentially.
type Engine struct {
	executor       WorkflowExecutor
	stateManager   KanbanStateManager
	eventBus       *events.EventBus
	circuitBreaker *CircuitBreaker
	logger         *slog.Logger

	enabled     atomic.Bool
	currentWfID atomic.Value // *string

	stopCh       chan struct{}
	doneCh       chan struct{}
	tickInterval time.Duration

	// For testing
	tickerFactory func(time.Duration) *time.Ticker
}

// EngineConfig holds configuration for the Engine.
type EngineConfig struct {
	Executor     WorkflowExecutor
	StateManager KanbanStateManager
	EventBus     *events.EventBus
	Logger       *slog.Logger
	TickInterval time.Duration
}

// NewEngine creates a new Kanban execution engine.
func NewEngine(cfg EngineConfig) *Engine {
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = DefaultTickInterval
	}

	e := &Engine{
		executor:       cfg.Executor,
		stateManager:   cfg.StateManager,
		eventBus:       cfg.EventBus,
		circuitBreaker: NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:         cfg.Logger,
		tickInterval:   cfg.TickInterval,
		tickerFactory:  time.NewTicker,
	}

	e.currentWfID.Store((*string)(nil))
	return e
}

// Start begins the engine loop.
func (e *Engine) Start(ctx context.Context) error {
	// Load persisted state
	if err := e.loadState(ctx); err != nil {
		return fmt.Errorf("load engine state: %w", err)
	}

	// Recover any interrupted workflow
	if err := e.recoverInterrupted(ctx); err != nil {
		e.logger.Warn("failed to recover interrupted workflow", "error", err)
	}

	// Initialize channels
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})

	// Subscribe to workflow events
	eventTypes := []string{
		events.TypeWorkflowCompleted,
		events.TypeWorkflowFailed,
	}

	go e.runLoop(ctx, eventTypes)

	e.logger.Info("kanban engine started")
	return nil
}

// runLoop is the main engine loop.
func (e *Engine) runLoop(ctx context.Context, eventTypes []string) {
	defer close(e.doneCh)

	ticker := e.tickerFactory(e.tickInterval)
	defer ticker.Stop()

	// Subscribe to workflow events
	eventCh := e.eventBus.Subscribe(eventTypes...)
	defer e.eventBus.Unsubscribe(eventCh)

	for {
		select {
		case <-e.stopCh:
			e.logger.Info("kanban engine stopping")
			// Wait for current workflow if running
			if wfID := e.getCurrentWorkflowID(); wfID != nil {
				e.logger.Info("waiting for current workflow to complete", "workflow_id", *wfID)
				e.waitForWorkflowCompletion(ctx, eventCh, *wfID)
			}
			return

		case event := <-eventCh:
			e.handleWorkflowEvent(ctx, event)

		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

// tick processes one iteration of the engine loop.
func (e *Engine) tick(ctx context.Context) {
	// Check if we should pick a new workflow
	if !e.enabled.Load() {
		return
	}
	if e.circuitBreaker.IsOpen() {
		return
	}
	if e.getCurrentWorkflowID() != nil {
		return
	}

	// Get next workflow from To Do queue
	workflow, err := e.stateManager.GetNextKanbanWorkflow(ctx)
	if err != nil {
		e.logger.Error("failed to get next kanban workflow", "error", err)
		return
	}
	if workflow == nil {
		return // Queue is empty
	}

	// Start execution
	e.startExecution(ctx, workflow)
}

// startExecution moves a workflow to in_progress and starts execution.
func (e *Engine) startExecution(ctx context.Context, workflow *core.WorkflowState) {
	workflowID := string(workflow.WorkflowID)
	fromColumn := workflow.KanbanColumn
	if fromColumn == "" {
		fromColumn = "todo"
	}

	e.logger.Info("starting kanban workflow execution",
		"workflow_id", workflowID,
		"title", workflow.Title,
	)

	// Move to in_progress
	if err := e.stateManager.MoveWorkflow(ctx, workflowID, "in_progress", 0); err != nil {
		e.logger.Error("failed to move workflow to in_progress", "error", err)
		return
	}

	// Update current workflow
	e.currentWfID.Store(&workflowID)

	// Update engine state in DB
	if err := e.persistState(ctx); err != nil {
		e.logger.Error("failed to persist engine state", "error", err)
	}

	// Emit event
	e.eventBus.Publish(events.NewKanbanWorkflowMovedEvent(
		workflowID, fromColumn, "in_progress", 0, false,
	))
	e.eventBus.Publish(events.NewKanbanExecutionStartedEvent(workflowID, 0))

	// Start workflow execution in background
	go func() {
		execCtx := context.Background() // Independent context for execution
		err := e.executor.Run(execCtx, core.WorkflowID(workflowID))
		if err != nil {
			// executor.Run() can fail in two ways:
			// 1. "Early failure" - validation errors before execution starts (no event published)
			// 2. "Execution failure" - failure during execution (WorkflowFailedEvent published by executor)
			//
			// For early failures, we must handle state cleanup here because the executor
			// won't publish a WorkflowFailedEvent. We detect early failures by checking
			// if the executor immediately returned an error (like "already running", "not found", etc.)
			e.logger.Error("workflow execution error", "workflow_id", workflowID, "error", err)

			// Handle early failure: update state directly since no event will be published
			// This covers cases like validation errors, missing config, etc.
			errMsg := err.Error()
			e.handleWorkflowFailed(execCtx, workflowID, errMsg)
		}
		// Note: For successful async execution, WorkflowCompletedEvent/WorkflowFailedEvent
		// will be published by the executor and handled in handleWorkflowEvent()
	}()
}

// handleWorkflowEvent processes workflow completion/failure events.
func (e *Engine) handleWorkflowEvent(ctx context.Context, event events.Event) {
	currentWfID := e.getCurrentWorkflowID()
	if currentWfID == nil {
		return // No current workflow, ignore event
	}

	if event.WorkflowID() != *currentWfID {
		return // Not our workflow
	}

	switch evt := event.(type) {
	case events.WorkflowCompletedEvent:
		e.handleWorkflowCompleted(ctx, *currentWfID)

	case events.WorkflowFailedEvent:
		errMsg := evt.Error
		if errMsg == "" {
			errMsg = "workflow failed"
		}
		e.handleWorkflowFailed(ctx, *currentWfID, errMsg)
	}
}

// handleWorkflowCompleted handles successful workflow completion.
func (e *Engine) handleWorkflowCompleted(ctx context.Context, workflowID string) {
	e.logger.Info("kanban workflow completed", "workflow_id", workflowID)

	// Load workflow to get branch info
	workflow, err := e.stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		e.logger.Error("failed to load completed workflow", "error", err)
	}

	// Try to get PR info
	prURL := ""
	prNumber := 0
	if workflow != nil && workflow.PRURL != "" {
		// PR info already set (if finalizer was updated to persist it)
		prURL = workflow.PRURL
		prNumber = workflow.PRNumber
	} else if workflow != nil && workflow.WorkflowBranch != "" {
		// Try to find PR by branch name (fallback)
		// Note: This would require GitHub client access
		e.logger.Warn("PR URL not found in workflow state, branch lookup not implemented",
			"branch", workflow.WorkflowBranch)
	}

	// Move to to_verify
	if err := e.stateManager.UpdateKanbanStatus(ctx, workflowID, "to_verify", prURL, prNumber, ""); err != nil {
		e.logger.Error("failed to move workflow to to_verify", "error", err)
	}

	// Reset circuit breaker on success
	e.circuitBreaker.RecordSuccess()

	// Clear current workflow
	e.currentWfID.Store((*string)(nil))

	// Persist state
	if err := e.persistState(ctx); err != nil {
		e.logger.Error("failed to persist engine state", "error", err)
	}

	// Emit events
	e.eventBus.Publish(events.NewKanbanWorkflowMovedEvent(
		workflowID, "in_progress", "to_verify", 0, false,
	))
	e.eventBus.Publish(events.NewKanbanExecutionCompletedEvent(workflowID, prURL, prNumber))
}

// handleWorkflowFailed handles workflow failure.
func (e *Engine) handleWorkflowFailed(ctx context.Context, workflowID string, errMsg string) {
	e.logger.Warn("kanban workflow failed", "workflow_id", workflowID, "error", errMsg)

	// Move to refinement
	if err := e.stateManager.UpdateKanbanStatus(ctx, workflowID, "refinement", "", 0, errMsg); err != nil {
		e.logger.Error("failed to move workflow to refinement", "error", err)
	}

	// Record failure in circuit breaker
	tripped := e.circuitBreaker.RecordFailure()
	consecutiveFailures := e.circuitBreaker.ConsecutiveFailures()

	// Clear current workflow
	e.currentWfID.Store((*string)(nil))

	// If circuit breaker tripped, disable engine
	if tripped {
		e.logger.Warn("circuit breaker tripped, disabling engine",
			"consecutive_failures", consecutiveFailures)
		e.enabled.Store(false)

		failures, _, lastFailure := e.circuitBreaker.GetState()
		e.eventBus.Publish(events.NewKanbanCircuitBreakerOpenedEvent(
			failures, e.circuitBreaker.Threshold(), lastFailure,
		))
	}

	// Persist state
	if err := e.persistState(ctx); err != nil {
		e.logger.Error("failed to persist engine state", "error", err)
	}

	// Emit events
	e.eventBus.Publish(events.NewKanbanWorkflowMovedEvent(
		workflowID, "in_progress", "refinement", 0, false,
	))
	e.eventBus.Publish(events.NewKanbanExecutionFailedEvent(workflowID, errMsg, consecutiveFailures))
}

// Stop gracefully stops the engine.
func (e *Engine) Stop(ctx context.Context) error {
	close(e.stopCh)

	select {
	case <-e.doneCh:
		e.logger.Info("kanban engine stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Enable enables the engine to pick workflows.
func (e *Engine) Enable(ctx context.Context) error {
	if e.circuitBreaker.IsOpen() {
		return fmt.Errorf("cannot enable: circuit breaker is open")
	}

	e.enabled.Store(true)

	if err := e.persistState(ctx); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}

	currentWfID := e.getCurrentWorkflowID()
	e.eventBus.Publish(events.NewKanbanEngineStateChangedEvent(
		true, currentWfID, false,
	))

	e.logger.Info("kanban engine enabled")
	return nil
}

// Disable disables the engine (waits for current workflow).
func (e *Engine) Disable(ctx context.Context) error {
	e.enabled.Store(false)

	if err := e.persistState(ctx); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}

	currentWfID := e.getCurrentWorkflowID()
	e.eventBus.Publish(events.NewKanbanEngineStateChangedEvent(
		false, currentWfID, e.circuitBreaker.IsOpen(),
	))

	e.logger.Info("kanban engine disabled")
	return nil
}

// IsEnabled returns true if engine is enabled.
func (e *Engine) IsEnabled() bool {
	return e.enabled.Load()
}

// CurrentWorkflowID returns the currently executing workflow ID.
func (e *Engine) CurrentWorkflowID() *string {
	return e.getCurrentWorkflowID()
}

// ResetCircuitBreaker resets the circuit breaker.
func (e *Engine) ResetCircuitBreaker(ctx context.Context) error {
	e.circuitBreaker.Reset()

	if err := e.persistState(ctx); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}

	e.eventBus.Publish(events.NewKanbanEngineStateChangedEvent(
		e.enabled.Load(), e.getCurrentWorkflowID(), false,
	))

	e.logger.Info("circuit breaker reset")
	return nil
}

// GetState returns engine state for API responses.
func (e *Engine) GetState() EngineState {
	failures, isOpen, lastFailure := e.circuitBreaker.GetState()

	state := EngineState{
		Enabled:             e.enabled.Load(),
		CurrentWorkflowID:   e.getCurrentWorkflowID(),
		ConsecutiveFailures: failures,
		CircuitBreakerOpen:  isOpen,
	}

	if !lastFailure.IsZero() {
		state.LastFailureAt = &lastFailure
	}

	return state
}

// Helper methods

func (e *Engine) getCurrentWorkflowID() *string {
	v := e.currentWfID.Load()
	if v == nil {
		return nil
	}
	return v.(*string)
}

func (e *Engine) loadState(ctx context.Context) error {
	state, err := e.stateManager.GetKanbanEngineState(ctx)
	if err != nil {
		return err
	}

	if state == nil {
		// No persisted state, use defaults
		return nil
	}

	e.enabled.Store(state.Enabled)
	e.currentWfID.Store(state.CurrentWorkflowID)

	var lastFailure time.Time
	if state.LastFailureAt != nil {
		lastFailure = *state.LastFailureAt
	}
	e.circuitBreaker.SetState(state.ConsecutiveFailures, state.CircuitBreakerOpen, lastFailure)

	return nil
}

func (e *Engine) persistState(ctx context.Context) error {
	failures, isOpen, lastFailure := e.circuitBreaker.GetState()

	state := &KanbanEngineState{
		Enabled:             e.enabled.Load(),
		CurrentWorkflowID:   e.getCurrentWorkflowID(),
		ConsecutiveFailures: failures,
		CircuitBreakerOpen:  isOpen,
	}
	if !lastFailure.IsZero() {
		state.LastFailureAt = &lastFailure
	}

	return e.stateManager.SaveKanbanEngineState(ctx, state)
}

func (e *Engine) recoverInterrupted(ctx context.Context) error {
	currentWfID := e.getCurrentWorkflowID()
	if currentWfID == nil {
		return nil
	}

	e.logger.Info("recovering interrupted workflow", "workflow_id", *currentWfID)

	// Load workflow to check its status
	workflow, err := e.stateManager.LoadByID(ctx, core.WorkflowID(*currentWfID))
	if err != nil {
		return fmt.Errorf("load interrupted workflow: %w", err)
	}

	if workflow == nil {
		// Workflow was deleted
		e.currentWfID.Store((*string)(nil))
		return nil
	}

	switch workflow.Status {
	case core.WorkflowStatusCompleted:
		// Workflow completed while we were down
		e.handleWorkflowCompleted(ctx, *currentWfID)

	case core.WorkflowStatusFailed:
		// Workflow failed while we were down
		e.handleWorkflowFailed(ctx, *currentWfID, workflow.Error)

	default:
		// Workflow in unknown state, move to refinement
		e.logger.Warn("interrupted workflow in unexpected state, moving to refinement",
			"workflow_id", *currentWfID,
			"status", workflow.Status,
		)
		if err := e.stateManager.UpdateKanbanStatus(ctx, *currentWfID, "refinement", "", 0,
			fmt.Sprintf("Workflow interrupted in %s state", workflow.Status)); err != nil {
			e.logger.Error("failed to move interrupted workflow to refinement", "error", err)
		}
		e.currentWfID.Store((*string)(nil))
	}

	return nil
}

func (e *Engine) waitForWorkflowCompletion(ctx context.Context, eventCh <-chan events.Event, workflowID string) {
	timeout := time.After(5 * time.Minute) // Max wait time for graceful shutdown

	for {
		select {
		case event := <-eventCh:
			if event.WorkflowID() == workflowID {
				switch event.(type) {
				case events.WorkflowCompletedEvent, events.WorkflowFailedEvent:
					e.handleWorkflowEvent(ctx, event)
					return
				}
			}
		case <-timeout:
			e.logger.Warn("timeout waiting for workflow completion during shutdown",
				"workflow_id", workflowID)
			return
		case <-ctx.Done():
			return
		}
	}
}
