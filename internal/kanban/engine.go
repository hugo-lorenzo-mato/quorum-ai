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

// currentExecution tracks the currently executing workflow and its project.
type currentExecution struct {
	WorkflowID string
	ProjectID  string
}

// Engine is the Kanban execution engine that processes workflows sequentially.
type Engine struct {
	executor        WorkflowExecutor
	projectProvider ProjectStateProvider
	globalEventBus  *events.EventBus
	circuitBreaker  *CircuitBreaker
	logger          *slog.Logger

	// Legacy single-project mode (for backward compatibility)
	legacyStateManager KanbanStateManager

	enabled    atomic.Bool
	currentExe atomic.Value // *currentExecution

	stopCh       chan struct{}
	doneCh       chan struct{}
	tickInterval time.Duration

	// For testing
	tickerFactory func(time.Duration) *time.Ticker
}

// EngineConfig holds configuration for the Engine.
type EngineConfig struct {
	Executor     WorkflowExecutor
	StateManager KanbanStateManager // Legacy: single StateManager (deprecated, use ProjectProvider)
	EventBus     *events.EventBus   // Global event bus for subscriptions

	// Multi-project support
	ProjectProvider ProjectStateProvider // Provider for project-scoped StateManagers

	Logger       *slog.Logger
	TickInterval time.Duration
}

// NewEngine creates a new Kanban execution engine.
func NewEngine(cfg EngineConfig) *Engine {
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = DefaultTickInterval
	}

	e := &Engine{
		executor:           cfg.Executor,
		projectProvider:    cfg.ProjectProvider,
		globalEventBus:     cfg.EventBus,
		legacyStateManager: cfg.StateManager,
		circuitBreaker:     NewCircuitBreaker(DefaultCircuitBreakerThreshold),
		logger:             cfg.Logger,
		tickInterval:       cfg.TickInterval,
		tickerFactory:      time.NewTicker,
	}

	// If no ProjectProvider but we have a legacy StateManager, create a single-project provider
	if e.projectProvider == nil && e.legacyStateManager != nil {
		e.projectProvider = NewSingleProjectProvider(e.legacyStateManager, cfg.EventBus)
		e.logger.Info("kanban engine using single-project mode (legacy)")
	}

	e.currentExe.Store((*currentExecution)(nil))
	return e
}

// Start begins the engine loop.
func (e *Engine) Start(ctx context.Context) error {
	if e.projectProvider == nil {
		return fmt.Errorf("no project provider configured")
	}

	// Load persisted state (uses legacy state manager if available)
	if err := e.loadState(ctx); err != nil {
		e.logger.Warn("failed to load engine state, starting fresh", "error", err)
	}

	// Recover any interrupted workflow
	if err := e.recoverInterrupted(ctx); err != nil {
		e.logger.Warn("failed to recover interrupted workflow", "error", err)
	}

	// Initialize channels
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})

	// Subscribe to workflow events from global event bus
	eventTypes := []string{
		events.TypeWorkflowCompleted,
		events.TypeWorkflowFailed,
	}

	go e.runLoop(ctx, eventTypes)

	e.logger.Info("kanban engine started", "multi_project", e.projectProvider != nil)
	return nil
}

// runLoop is the main engine loop.
func (e *Engine) runLoop(ctx context.Context, eventTypes []string) {
	defer close(e.doneCh)

	ticker := e.tickerFactory(e.tickInterval)
	defer ticker.Stop()

	// Subscribe to workflow events from global event bus
	eventCh := e.globalEventBus.Subscribe(eventTypes...)
	defer e.globalEventBus.Unsubscribe(eventCh)

	for {
		select {
		case <-e.stopCh:
			e.logger.Info("kanban engine stopping")
			// Wait for current workflow if running
			if exe := e.getCurrentExecution(); exe != nil {
				e.logger.Info("waiting for current workflow to complete",
					"workflow_id", exe.WorkflowID, "project_id", exe.ProjectID)
				e.waitForWorkflowCompletion(ctx, eventCh, exe.WorkflowID)
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
	if e.getCurrentExecution() != nil {
		return // Already executing a workflow
	}

	// Get list of active projects
	projects, err := e.projectProvider.ListActiveProjects(ctx)
	if err != nil {
		e.logger.Error("failed to list active projects", "error", err)
		return
	}

	// Iterate over projects looking for a workflow to execute
	for _, proj := range projects {
		stateManager, err := e.projectProvider.GetProjectStateManager(ctx, proj.ID)
		if err != nil {
			e.logger.Warn("failed to get state manager for project",
				"project_id", proj.ID, "error", err)
			continue
		}
		if stateManager == nil {
			continue
		}

		// Get next workflow from this project's To Do queue
		workflow, err := stateManager.GetNextKanbanWorkflow(ctx)
		if err != nil {
			e.logger.Warn("failed to get next kanban workflow",
				"project_id", proj.ID, "error", err)
			continue
		}
		if workflow == nil {
			continue // This project's queue is empty
		}

		// Found a workflow - start execution
		e.startExecutionForProject(ctx, workflow, proj.ID, stateManager)
		return // Only execute one workflow at a time
	}
}

// startExecutionForProject moves a workflow to in_progress and starts execution.
// This is the project-aware version that uses project-specific StateManager.
func (e *Engine) startExecutionForProject(ctx context.Context, workflow *core.WorkflowState, projectID string, stateManager KanbanStateManager) {
	workflowID := string(workflow.WorkflowID)
	fromColumn := workflow.KanbanColumn
	if fromColumn == "" {
		fromColumn = "todo"
	}

	e.logger.Info("starting kanban workflow execution",
		"workflow_id", workflowID,
		"project_id", projectID,
		"title", workflow.Title,
	)

	// Move to in_progress using project-specific StateManager
	if err := stateManager.MoveWorkflow(ctx, workflowID, "in_progress", 0); err != nil {
		e.logger.Error("failed to move workflow to in_progress",
			"workflow_id", workflowID, "project_id", projectID, "error", err)
		return
	}

	// Update current execution (workflow + project)
	e.currentExe.Store(&currentExecution{
		WorkflowID: workflowID,
		ProjectID:  projectID,
	})

	// Update engine state in DB
	if err := e.persistState(ctx); err != nil {
		e.logger.Error("failed to persist engine state", "error", err)
	}

	// Get project-specific EventBus for SSE events
	projectEventBus := e.projectProvider.GetProjectEventBus(ctx, projectID)
	e.publishEvent(projectEventBus, events.NewKanbanWorkflowMovedEvent(
		workflowID, projectID, fromColumn, "in_progress", 0, false,
	))
	e.publishEvent(projectEventBus, events.NewKanbanExecutionStartedEvent(workflowID, projectID, 0))

	// Start workflow execution in background
	go func() {
		baseCtx := context.Background() // Independent context for execution
		execCtx, ctxErr := e.projectProvider.GetProjectExecutionContext(baseCtx, projectID)
		if ctxErr != nil {
			e.logger.Error("failed to create project execution context",
				"workflow_id", workflowID, "project_id", projectID, "error", ctxErr)
			e.handleWorkflowFailedForProject(baseCtx, workflowID, projectID, ctxErr.Error())
			return
		}

		err := e.executor.Run(execCtx, core.WorkflowID(workflowID))
		if err != nil {
			// executor.Run() can fail in two ways:
			// 1. "Early failure" - validation errors before execution starts (no event published)
			// 2. "Execution failure" - failure during execution (WorkflowFailedEvent published by executor)
			//
			// For early failures, we must handle state cleanup here because the executor
			// won't publish a WorkflowFailedEvent. We detect early failures by checking
			// if the executor immediately returned an error (like "already running", "not found", etc.)
			e.logger.Error("workflow execution error",
				"workflow_id", workflowID, "project_id", projectID, "error", err)

			// Handle early failure: update state directly since no event will be published
			// This covers cases like validation errors, missing config, etc.
			errMsg := err.Error()
			e.handleWorkflowFailedForProject(baseCtx, workflowID, projectID, errMsg)
		}
		// Note: For successful async execution, WorkflowCompletedEvent/WorkflowFailedEvent
		// will be published by the executor and handled in handleWorkflowEvent()
	}()
}

// publishEvent publishes an event to the given EventBus (or global if nil).
func (e *Engine) publishEvent(projectEventBus EventPublisher, event events.Event) {
	if projectEventBus != nil {
		projectEventBus.Publish(event)
	}
	// Also publish to global event bus for backward compatibility
	if e.globalEventBus != nil {
		e.globalEventBus.Publish(event)
	}
}

// handleWorkflowEvent processes workflow completion/failure events.
func (e *Engine) handleWorkflowEvent(ctx context.Context, event events.Event) {
	currentExe := e.getCurrentExecution()
	if currentExe == nil {
		return // No current workflow, ignore event
	}

	if event.WorkflowID() != currentExe.WorkflowID {
		return // Not our workflow
	}

	switch evt := event.(type) {
	case events.WorkflowCompletedEvent:
		e.handleWorkflowCompletedForProject(ctx, currentExe.WorkflowID, currentExe.ProjectID)

	case events.WorkflowFailedEvent:
		errMsg := evt.Error
		if errMsg == "" {
			errMsg = "workflow failed"
		}
		e.handleWorkflowFailedForProject(ctx, currentExe.WorkflowID, currentExe.ProjectID, errMsg)
	}
}

// handleWorkflowCompletedForProject handles successful workflow completion with project context.
func (e *Engine) handleWorkflowCompletedForProject(ctx context.Context, workflowID, projectID string) {
	e.logger.Info("kanban workflow completed", "workflow_id", workflowID, "project_id", projectID)

	// Get project-specific StateManager
	stateManager, err := e.projectProvider.GetProjectStateManager(ctx, projectID)
	if err != nil || stateManager == nil {
		e.logger.Error("failed to get state manager for completed workflow",
			"workflow_id", workflowID, "project_id", projectID, "error", err)
		e.clearCurrentExecution(ctx)
		return
	}

	// Load workflow to get branch info
	workflow, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
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

	// Move to to_verify using project-specific StateManager
	if err := stateManager.UpdateKanbanStatus(ctx, workflowID, "to_verify", prURL, prNumber, ""); err != nil {
		e.logger.Error("failed to move workflow to to_verify", "error", err)
	}

	// Reset circuit breaker on success
	e.circuitBreaker.RecordSuccess()

	// Clear current execution
	e.clearCurrentExecution(ctx)

	// Get project-specific EventBus for SSE events
	projectEventBus := e.projectProvider.GetProjectEventBus(ctx, projectID)
	e.publishEvent(projectEventBus, events.NewKanbanWorkflowMovedEvent(
		workflowID, projectID, "in_progress", "to_verify", 0, false,
	))
	e.publishEvent(projectEventBus, events.NewKanbanExecutionCompletedEvent(workflowID, projectID, prURL, prNumber))
}

// handleWorkflowFailedForProject handles workflow failure with project context.
func (e *Engine) handleWorkflowFailedForProject(ctx context.Context, workflowID, projectID, errMsg string) {
	e.logger.Warn("kanban workflow failed", "workflow_id", workflowID, "project_id", projectID, "error", errMsg)

	// Get project-specific StateManager
	stateManager, err := e.projectProvider.GetProjectStateManager(ctx, projectID)
	if err != nil || stateManager == nil {
		e.logger.Error("failed to get state manager for failed workflow",
			"workflow_id", workflowID, "project_id", projectID, "error", err)
		// Continue with cleanup even if we can't update state
	} else {
		// Move to refinement using project-specific StateManager
		if err := stateManager.UpdateKanbanStatus(ctx, workflowID, "refinement", "", 0, errMsg); err != nil {
			e.logger.Error("failed to move workflow to refinement", "error", err)
		}
	}

	// Record failure in circuit breaker
	tripped := e.circuitBreaker.RecordFailure()
	consecutiveFailures := e.circuitBreaker.ConsecutiveFailures()

	// Clear current execution
	e.clearCurrentExecution(ctx)

	// Get project-specific EventBus for SSE events
	projectEventBus := e.projectProvider.GetProjectEventBus(ctx, projectID)

	// If circuit breaker tripped, disable engine
	if tripped {
		e.logger.Warn("circuit breaker tripped, disabling engine",
			"consecutive_failures", consecutiveFailures)
		e.enabled.Store(false)

		failures, _, lastFailure := e.circuitBreaker.GetState()
		e.publishEvent(projectEventBus, events.NewKanbanCircuitBreakerOpenedEvent(
			projectID, failures, e.circuitBreaker.Threshold(), lastFailure,
		))
	}

	// Emit events
	e.publishEvent(projectEventBus, events.NewKanbanWorkflowMovedEvent(
		workflowID, projectID, "in_progress", "refinement", 0, false,
	))
	e.publishEvent(projectEventBus, events.NewKanbanExecutionFailedEvent(workflowID, projectID, errMsg, consecutiveFailures))
}

// clearCurrentExecution clears the current execution and persists state.
func (e *Engine) clearCurrentExecution(ctx context.Context) {
	e.currentExe.Store((*currentExecution)(nil))
	if err := e.persistState(ctx); err != nil {
		e.logger.Error("failed to persist engine state", "error", err)
	}
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
	if e.globalEventBus != nil {
		e.globalEventBus.Publish(events.NewKanbanEngineStateChangedEvent(
			"", true, currentWfID, false,
		))
	}

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
	if e.globalEventBus != nil {
		e.globalEventBus.Publish(events.NewKanbanEngineStateChangedEvent(
			"", false, currentWfID, e.circuitBreaker.IsOpen(),
		))
	}

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

// CurrentProjectID returns the project ID of the currently executing workflow.
func (e *Engine) CurrentProjectID() *string {
	exe := e.getCurrentExecution()
	if exe == nil {
		return nil
	}
	return &exe.ProjectID
}

// ResetCircuitBreaker resets the circuit breaker.
func (e *Engine) ResetCircuitBreaker(ctx context.Context) error {
	e.circuitBreaker.Reset()

	if err := e.persistState(ctx); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}

	if e.globalEventBus != nil {
		e.globalEventBus.Publish(events.NewKanbanEngineStateChangedEvent(
			"", e.enabled.Load(), e.getCurrentWorkflowID(), false,
		))
	}

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

// getCurrentExecution returns the current execution (workflow + project).
func (e *Engine) getCurrentExecution() *currentExecution {
	v := e.currentExe.Load()
	if v == nil {
		return nil
	}
	return v.(*currentExecution)
}

// getCurrentWorkflowID returns just the workflow ID for backward compatibility.
func (e *Engine) getCurrentWorkflowID() *string {
	exe := e.getCurrentExecution()
	if exe == nil {
		return nil
	}
	return &exe.WorkflowID
}

// getEngineStateManager returns a StateManager for persisting engine state.
// Uses legacy StateManager if available, otherwise tries the first project.
func (e *Engine) getEngineStateManager(ctx context.Context) KanbanStateManager {
	// Prefer legacy state manager for engine state (global state)
	if e.legacyStateManager != nil {
		return e.legacyStateManager
	}

	// Fallback: try to get from first project
	if e.projectProvider != nil {
		projects, err := e.projectProvider.ListActiveProjects(ctx)
		if err == nil && len(projects) > 0 {
			sm, _ := e.projectProvider.GetProjectStateManager(ctx, projects[0].ID)
			return sm
		}
	}

	return nil
}

func (e *Engine) loadState(ctx context.Context) error {
	sm := e.getEngineStateManager(ctx)
	if sm == nil {
		// No state manager available, start with defaults
		return nil
	}

	state, err := sm.GetKanbanEngineState(ctx)
	if err != nil {
		return err
	}

	if state == nil {
		// No persisted state, use defaults
		return nil
	}

	e.enabled.Store(state.Enabled)

	// Restore current execution if we have a workflow ID
	// Note: We don't have the project ID in the persisted state, so we can't fully restore
	// This is a limitation - after restart, interrupted workflows may not be properly recovered
	if state.CurrentWorkflowID != nil {
		e.logger.Warn("engine state has current workflow but project ID is unknown, clearing",
			"workflow_id", *state.CurrentWorkflowID)
		// We'll handle this in recoverInterrupted by searching all projects
	}

	var lastFailure time.Time
	if state.LastFailureAt != nil {
		lastFailure = *state.LastFailureAt
	}
	e.circuitBreaker.SetState(state.ConsecutiveFailures, state.CircuitBreakerOpen, lastFailure)

	return nil
}

func (e *Engine) persistState(ctx context.Context) error {
	sm := e.getEngineStateManager(ctx)
	if sm == nil {
		// No state manager available, can't persist
		return nil
	}

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

	return sm.SaveKanbanEngineState(ctx, state)
}

func (e *Engine) recoverInterrupted(ctx context.Context) error {
	// Try to recover from persisted engine state
	sm := e.getEngineStateManager(ctx)
	if sm == nil {
		return nil
	}

	state, err := sm.GetKanbanEngineState(ctx)
	if err != nil {
		e.currentExe.Store((*currentExecution)(nil))
		return err
	}
	if state == nil || state.CurrentWorkflowID == nil {
		// No interrupted workflow to recover
		e.currentExe.Store((*currentExecution)(nil))
		return nil
	}

	wfID := *state.CurrentWorkflowID
	e.logger.Info("recovering interrupted workflow", "workflow_id", wfID)

	// Load the workflow to check its current status
	workflow, err := sm.LoadByID(ctx, core.WorkflowID(wfID))
	if err != nil {
		e.logger.Warn("failed to load interrupted workflow", "workflow_id", wfID, "error", err)
		e.currentExe.Store((*currentExecution)(nil))
		return nil
	}

	if workflow == nil {
		e.logger.Warn("interrupted workflow was deleted", "workflow_id", wfID)
		e.currentExe.Store((*currentExecution)(nil))
		return nil
	}

	// Move workflow to appropriate column based on its status
	switch workflow.Status {
	case core.WorkflowStatusCompleted:
		// Move to to_verify
		if err := sm.UpdateKanbanStatus(ctx, wfID, "to_verify", workflow.PRURL, workflow.PRNumber, ""); err != nil {
			e.logger.Error("failed to move recovered workflow to to_verify", "error", err)
		} else {
			e.logger.Info("recovered completed workflow to to_verify", "workflow_id", wfID)
		}

	case core.WorkflowStatusFailed:
		// Move to refinement
		errMsg := workflow.Error
		if errMsg == "" {
			errMsg = "interrupted during execution"
		}
		if err := sm.UpdateKanbanStatus(ctx, wfID, "refinement", "", 0, errMsg); err != nil {
			e.logger.Error("failed to move recovered workflow to refinement", "error", err)
		} else {
			e.logger.Info("recovered failed workflow to refinement", "workflow_id", wfID)
		}

	case core.WorkflowStatusRunning:
		// Still marked as running - move to refinement as it was interrupted
		if err := sm.UpdateKanbanStatus(ctx, wfID, "refinement", "", 0, "workflow interrupted during execution (server restart)"); err != nil {
			e.logger.Error("failed to move interrupted workflow to refinement", "error", err)
		} else {
			e.logger.Info("recovered interrupted workflow to refinement", "workflow_id", wfID)
		}
	}

	// Clear current execution state
	e.currentExe.Store((*currentExecution)(nil))
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
