package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// WorkflowExecutor handles workflow execution and lifecycle management.
// It centralizes the logic for running, resuming, and tracking workflows,
// making it available to both the API handlers and the zombie detector.
type WorkflowExecutor struct {
	runnerFactory *RunnerFactory
	stateManager  core.StateManager
	eventBus      *events.EventBus
	logger        *slog.Logger

	// Track running workflows to prevent double-execution
	running sync.Map // map[string]bool

	// Track control planes for pause/resume/cancel
	controlPlanes sync.Map // map[string]*control.ControlPlane

	// Execution timeout
	executionTimeout time.Duration
}

// NewWorkflowExecutor creates a new workflow executor.
func NewWorkflowExecutor(
	runnerFactory *RunnerFactory,
	stateManager core.StateManager,
	eventBus *events.EventBus,
	logger *slog.Logger,
) *WorkflowExecutor {
	return &WorkflowExecutor{
		runnerFactory:    runnerFactory,
		stateManager:     stateManager,
		eventBus:         eventBus,
		logger:           logger,
		executionTimeout: 4 * time.Hour,
	}
}

// WithExecutionTimeout sets the execution timeout.
func (e *WorkflowExecutor) WithExecutionTimeout(timeout time.Duration) *WorkflowExecutor {
	e.executionTimeout = timeout
	return e
}

// IsRunning checks if a workflow is currently executing.
func (e *WorkflowExecutor) IsRunning(workflowID string) bool {
	_, exists := e.running.Load(workflowID)
	return exists
}

// Run starts a workflow from its initial state.
func (e *WorkflowExecutor) Run(ctx context.Context, workflowID core.WorkflowID) error {
	return e.execute(ctx, workflowID, false)
}

// Resume continues a workflow from where it left off.
func (e *WorkflowExecutor) Resume(ctx context.Context, workflowID core.WorkflowID) error {
	return e.execute(ctx, workflowID, true)
}

// execute handles both run and resume operations.
func (e *WorkflowExecutor) execute(ctx context.Context, workflowID core.WorkflowID, isResume bool) error {
	id := string(workflowID)

	// Load workflow state
	state, err := e.stateManager.LoadByID(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("loading workflow: %w", err)
	}
	if state == nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Validate state for execution
	switch state.Status {
	case core.WorkflowStatusRunning:
		return fmt.Errorf("workflow is already running")
	case core.WorkflowStatusCompleted:
		return fmt.Errorf("workflow is already completed")
	case core.WorkflowStatusPending, core.WorkflowStatusFailed, core.WorkflowStatusPaused:
		// OK to execute
	default:
		return fmt.Errorf("workflow is in invalid state: %s", state.Status)
	}

	// Prevent double-execution
	if _, loaded := e.running.LoadOrStore(id, true); loaded {
		return fmt.Errorf("workflow execution already in progress")
	}

	// Check runner factory
	if e.runnerFactory == nil {
		e.running.Delete(id)
		return fmt.Errorf("workflow execution not available: missing configuration")
	}

	// Create execution context
	execCtx, cancel := context.WithTimeout(context.Background(), e.executionTimeout)

	// Create ControlPlane for this workflow
	cp := control.New()
	e.controlPlanes.Store(id, cp)

	// Create runner
	runner, notifier, err := e.runnerFactory.CreateRunner(execCtx, id, cp, state.Config)
	if err != nil {
		cancel()
		e.controlPlanes.Delete(id)
		e.running.Delete(id)
		return fmt.Errorf("creating runner: %w", err)
	}

	// Connect notifier to state for agent event persistence
	notifier.SetState(state)
	notifier.SetStateSaver(e.stateManager)

	// Update status to running
	state.Status = core.WorkflowStatusRunning
	state.Error = ""
	state.UpdatedAt = time.Now()
	now := time.Now().UTC()
	state.HeartbeatAt = &now

	if err := e.stateManager.Save(ctx, state); err != nil {
		cancel()
		e.controlPlanes.Delete(id)
		e.running.Delete(id)
		return fmt.Errorf("saving workflow state: %w", err)
	}

	// Start execution in background
	go e.executeAsync(execCtx, cancel, runner, notifier, state, isResume, id)

	return nil
}

// executeAsync runs the workflow in a background goroutine.
func (e *WorkflowExecutor) executeAsync(
	ctx context.Context,
	cancel context.CancelFunc,
	runner *workflow.Runner,
	notifier interface {
		WorkflowStarted(prompt string)
		WorkflowCompleted(duration time.Duration, totalCost float64)
		WorkflowFailed(phase string, err error)
		FlushState()
	},
	state *core.WorkflowState,
	isResume bool,
	workflowID string,
) {
	defer cancel()
	defer e.running.Delete(workflowID)
	defer e.controlPlanes.Delete(workflowID)
	defer notifier.FlushState()

	// Emit workflow started event
	notifier.WorkflowStarted(state.Prompt)

	startTime := time.Now()
	var runErr error

	// Execute workflow
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
		e.logger.Error("workflow execution failed",
			"workflow_id", state.WorkflowID,
			"error", runErr,
		)
		notifier.WorkflowFailed(string(state.CurrentPhase), runErr)

		// Publish SSE event
		if e.eventBus != nil {
			e.eventBus.Publish(events.NewWorkflowFailedEvent(workflowID, string(state.CurrentPhase), runErr))
		}
	} else {
		e.logger.Info("workflow execution completed",
			"workflow_id", state.WorkflowID,
			"duration", duration,
			"cost", totalCost,
		)
		notifier.WorkflowCompleted(duration, totalCost)

		// Publish SSE event
		if e.eventBus != nil {
			e.eventBus.Publish(events.NewWorkflowCompletedEvent(workflowID, duration, totalCost))
		}
	}
}

// GetControlPlane returns the control plane for a running workflow.
func (e *WorkflowExecutor) GetControlPlane(workflowID string) (*control.ControlPlane, bool) {
	if cp, ok := e.controlPlanes.Load(workflowID); ok {
		return cp.(*control.ControlPlane), true
	}
	return nil, false
}

// Cancel cancels a running workflow.
func (e *WorkflowExecutor) Cancel(workflowID string) error {
	cp, ok := e.GetControlPlane(workflowID)
	if !ok {
		return fmt.Errorf("workflow is not running")
	}
	if cp.IsCancelled() {
		return fmt.Errorf("workflow is already being cancelled")
	}
	cp.Cancel()
	e.logger.Info("workflow cancellation requested", "workflow_id", workflowID)
	return nil
}

// Pause pauses a running workflow.
func (e *WorkflowExecutor) Pause(workflowID string) error {
	cp, ok := e.GetControlPlane(workflowID)
	if !ok {
		return fmt.Errorf("workflow is not running")
	}
	if cp.IsPaused() {
		return fmt.Errorf("workflow is already paused")
	}
	cp.Pause()
	e.logger.Info("workflow pause requested", "workflow_id", workflowID)
	return nil
}

// StateManager returns the state manager for external use.
func (e *WorkflowExecutor) StateManager() core.StateManager {
	return e.stateManager
}

// EventBus returns the event bus for external use.
func (e *WorkflowExecutor) EventBus() *events.EventBus {
	return e.eventBus
}
