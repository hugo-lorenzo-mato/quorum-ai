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
// The StateManager is obtained from the context if a ProjectContext is available,
// otherwise falls back to the executor's default StateManager.
func (e *WorkflowExecutor) execute(ctx context.Context, workflowID core.WorkflowID, isResume bool) error {
	id := string(workflowID)

	// Require unified tracker for proper state synchronization
	if e.unifiedTracker == nil {
		return fmt.Errorf("workflow execution not available: missing tracker")
	}

	// Get project-scoped StateManager if available
	stateManager := GetStateManagerFromContext(ctx, e.stateManager)

	// Load workflow state using project-scoped StateManager
	state, err := stateManager.LoadByID(ctx, workflowID)
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

	// Create execution context that preserves ProjectContext values but detaches from HTTP request cancellation.
	// This ensures the workflow continues running after the HTTP request completes,
	// while maintaining access to project-scoped resources (StateManager, EventBus, etc.)
	execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.executionTimeout)

	// Create runner using the ControlPlane from the handle
	runner, notifier, err := e.runnerFactory.CreateRunner(execCtx, id, handle.ControlPlane, state.Blueprint)
	if err != nil {
		cancel()
		if rollbackErr := e.unifiedTracker.RollbackExecution(ctx, workflowID, err.Error()); rollbackErr != nil && e.logger != nil {
			e.logger.Error("failed to rollback execution", "workflow_id", workflowID, "error", rollbackErr)
		}
		return fmt.Errorf("creating runner: %w", err)
	}

	// Connect notifier to state for agent event persistence
	notifier.SetState(state)
	notifier.SetStateSaver(stateManager)

	// Reload state to get atomic updates from StartExecution
	state, err = stateManager.LoadByID(ctx, workflowID)
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
		WorkflowCompleted(duration time.Duration)
		WorkflowFailed(phase string, err error)
		FlushState()
	},
	state *core.WorkflowState,
	isResume bool,
	workflowID string,
	handle *ExecutionHandle,
) {
	// Capture cleanup context at the start to preserve ProjectContext for FinishExecution.
	// This ensures cleanup happens in the correct project's DB even if the execution times out.
	cleanupCtx := context.WithoutCancel(ctx)

	defer cancel()
	defer func() {
		// Clean up via UnifiedTracker using the preserved project context
		if e.unifiedTracker != nil {
			if finishErr := e.unifiedTracker.FinishExecution(cleanupCtx, core.WorkflowID(workflowID)); finishErr != nil && e.logger != nil {
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

	duration := time.Since(startTime)

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
		)
		notifier.WorkflowCompleted(duration)

		// Publish SSE event
		if e.eventBus != nil {
			e.eventBus.Publish(events.NewWorkflowCompletedEvent(workflowID, "", duration))
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
