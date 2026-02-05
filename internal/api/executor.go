package api

import (
	"context"
	"fmt"
	"log/slog"
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

	// Unified tracker for workflow state synchronization
	unifiedTracker *UnifiedTracker

	// Execution timeout
	executionTimeout time.Duration
}

// NewWorkflowExecutor creates a new workflow executor.
func NewWorkflowExecutor(
	runnerFactory *RunnerFactory,
	stateManager core.StateManager,
	eventBus *events.EventBus,
	logger *slog.Logger,
	unifiedTracker *UnifiedTracker,
) *WorkflowExecutor {
	return &WorkflowExecutor{
		runnerFactory:    runnerFactory,
		stateManager:     stateManager,
		eventBus:         eventBus,
		logger:           logger,
		unifiedTracker:   unifiedTracker,
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
	if e.unifiedTracker == nil {
		return false
	}
	return e.unifiedTracker.IsRunning(context.Background(), core.WorkflowID(workflowID))
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

	// Require unified tracker for proper state synchronization
	if e.unifiedTracker == nil {
		return fmt.Errorf("workflow execution not available: missing tracker")
	}

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

	// Check runner factory
	if e.runnerFactory == nil {
		return fmt.Errorf("workflow execution not available: missing configuration")
	}

	// Start execution atomically via UnifiedTracker
	handle, err := e.unifiedTracker.StartExecution(ctx, workflowID)
	if err != nil {
		return fmt.Errorf("starting execution: %w", err)
	}

	// Create execution context
	execCtx, cancel := context.WithTimeout(context.Background(), e.executionTimeout)

	// Create runner using the ControlPlane from the handle
	runner, notifier, err := e.runnerFactory.CreateRunner(execCtx, id, handle.ControlPlane, state.Config)
	if err != nil {
		cancel()
		if rollbackErr := e.unifiedTracker.RollbackExecution(ctx, workflowID, err.Error()); rollbackErr != nil && e.logger != nil {
			e.logger.Error("failed to rollback execution", "workflow_id", workflowID, "error", rollbackErr)
		}
		return fmt.Errorf("creating runner: %w", err)
	}

	// Connect notifier to state for agent event persistence
	notifier.SetState(state)
	notifier.SetStateSaver(e.stateManager)

	// Reload state to get atomic updates from StartExecution
	state, err = e.stateManager.LoadByID(ctx, workflowID)
	if err != nil {
		cancel()
		if rollbackErr := e.unifiedTracker.RollbackExecution(ctx, workflowID, err.Error()); rollbackErr != nil && e.logger != nil {
			e.logger.Error("failed to rollback execution", "workflow_id", workflowID, "error", rollbackErr)
		}
		return fmt.Errorf("reloading workflow state: %w", err)
	}

	// Start execution in background
	go e.executeAsync(execCtx, cancel, runner, notifier, state, isResume, id, handle)

	// Wait for confirmation from the goroutine
	if err := handle.WaitForConfirmation(5 * time.Second); err != nil {
		// Goroutine failed to start or timed out - rollback
		if rollbackErr := e.unifiedTracker.RollbackExecution(ctx, workflowID, err.Error()); rollbackErr != nil && e.logger != nil {
			e.logger.Error("failed to rollback execution", "workflow_id", workflowID, "error", rollbackErr)
		}
		return fmt.Errorf("workflow failed to start: %w", err)
	}

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
	handle *ExecutionHandle,
) {
	defer cancel()
	defer func() {
		// Clean up via UnifiedTracker
		if e.unifiedTracker != nil {
			if finishErr := e.unifiedTracker.FinishExecution(context.Background(), core.WorkflowID(workflowID)); finishErr != nil && e.logger != nil {
				e.logger.Error("failed to finish execution", "workflow_id", workflowID, "error", finishErr)
			}
		}
	}()
	defer notifier.FlushState()

	// Confirm that we've started (unblocks the caller waiting on handle.WaitForConfirmation)
	handle.ConfirmStarted()

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
			e.eventBus.Publish(events.NewWorkflowFailedEvent(workflowID, "", string(state.CurrentPhase), runErr))
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
			e.eventBus.Publish(events.NewWorkflowCompletedEvent(workflowID, "", duration, totalCost))
		}
	}
}

// GetControlPlane returns the control plane for a running workflow.
func (e *WorkflowExecutor) GetControlPlane(workflowID string) (*control.ControlPlane, bool) {
	if e.unifiedTracker == nil {
		return nil, false
	}
	return e.unifiedTracker.GetControlPlane(core.WorkflowID(workflowID))
}

// Cancel cancels a running workflow.
func (e *WorkflowExecutor) Cancel(workflowID string) error {
	if e.unifiedTracker == nil {
		return fmt.Errorf("workflow execution not available: missing tracker")
	}
	if err := e.unifiedTracker.Cancel(core.WorkflowID(workflowID)); err != nil {
		return err
	}
	e.logger.Info("workflow cancellation requested", "workflow_id", workflowID)
	return nil
}

// Pause pauses a running workflow.
func (e *WorkflowExecutor) Pause(workflowID string) error {
	if e.unifiedTracker == nil {
		return fmt.Errorf("workflow execution not available: missing tracker")
	}
	if err := e.unifiedTracker.Pause(core.WorkflowID(workflowID)); err != nil {
		return err
	}
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
