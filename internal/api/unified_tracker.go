package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// ExecutionHandle represents an active workflow execution.
// It contains the ControlPlane and channels for coordination.
type ExecutionHandle struct {
	WorkflowID   core.WorkflowID
	ControlPlane *control.ControlPlane
	StartedAt    time.Time

	// Confirmation channel - closed when goroutine confirms start
	confirmCh chan struct{}
	// Error channel - receives error if start fails
	errorCh chan error
	// Done channel - closed when execution completes
	doneCh chan struct{}
}

// WaitForConfirmation blocks until the execution goroutine confirms it started.
// Returns error if confirmation times out or start fails.
func (h *ExecutionHandle) WaitForConfirmation(timeout time.Duration) error {
	select {
	case <-h.confirmCh:
		return nil
	case err := <-h.errorCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for execution confirmation")
	}
}

// ConfirmStarted signals that the execution goroutine has started successfully.
func (h *ExecutionHandle) ConfirmStarted() {
	select {
	case <-h.confirmCh:
		// Already confirmed
	default:
		close(h.confirmCh)
	}
}

// ReportError signals that the execution failed to start.
func (h *ExecutionHandle) ReportError(err error) {
	select {
	case h.errorCh <- err:
	default:
	}
}

// Done returns a channel that's closed when execution completes.
func (h *ExecutionHandle) Done() <-chan struct{} {
	return h.doneCh
}

// MarkDone signals that execution has completed.
func (h *ExecutionHandle) MarkDone() {
	select {
	case <-h.doneCh:
		// Already done
	default:
		close(h.doneCh)
	}
}

// UnifiedTracker provides centralized workflow execution tracking.
// It ensures consistency between:
// - In-memory tracking (sync.Map for fast lookups)
// - Database persistence (running_workflows table as source of truth)
// - Heartbeat management (for zombie detection)
//
// This replaces the fragmented tracking in:
// - workflows.go runningWorkflows (global mutex map)
// - executor.go running and controlPlanes (sync.Maps)
// - server.go controlPlanes (mutex map)
type UnifiedTracker struct {
	stateManager core.StateManager
	heartbeat    *workflow.HeartbeatManager
	logger       *slog.Logger

	// In-memory tracking for fast lookups
	mu      sync.RWMutex
	handles map[core.WorkflowID]*ExecutionHandle

	// Confirmation timeout
	confirmTimeout time.Duration
}

// UnifiedTrackerConfig configures the UnifiedTracker.
type UnifiedTrackerConfig struct {
	// ConfirmTimeout is how long to wait for execution confirmation (default: 5s)
	ConfirmTimeout time.Duration
}

// DefaultUnifiedTrackerConfig returns the default configuration.
func DefaultUnifiedTrackerConfig() UnifiedTrackerConfig {
	return UnifiedTrackerConfig{
		ConfirmTimeout: 5 * time.Second,
	}
}

// NewUnifiedTracker creates a new unified tracker.
func NewUnifiedTracker(
	stateManager core.StateManager,
	heartbeat *workflow.HeartbeatManager,
	logger *slog.Logger,
	config UnifiedTrackerConfig,
) *UnifiedTracker {
	if config.ConfirmTimeout == 0 {
		config.ConfirmTimeout = 5 * time.Second
	}

	return &UnifiedTracker{
		stateManager:   stateManager,
		heartbeat:      heartbeat,
		logger:         logger,
		handles:        make(map[core.WorkflowID]*ExecutionHandle),
		confirmTimeout: config.ConfirmTimeout,
	}
}

// StartExecution atomically starts tracking a workflow execution.
// It performs the following in a single transaction:
// 1. Checks if workflow is already running (in memory or DB)
// 2. Marks workflow as running in DB
// 3. Creates ExecutionHandle with ControlPlane
// 4. Starts heartbeat tracking
//
// Returns an ExecutionHandle that the caller uses to coordinate with the goroutine.
// The StateManager is obtained from the context if a ProjectContext is available,
// otherwise falls back to the tracker's default StateManager.
func (t *UnifiedTracker) StartExecution(ctx context.Context, workflowID core.WorkflowID) (*ExecutionHandle, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Fast path: check in-memory first
	if _, exists := t.handles[workflowID]; exists {
		return nil, fmt.Errorf("workflow is already running (in memory)")
	}

	// Get project-scoped StateManager if available, otherwise use default
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Create handle before transaction (so we can rollback)
	handle := &ExecutionHandle{
		WorkflowID:   workflowID,
		ControlPlane: control.New(),
		StartedAt:    time.Now(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}

	// Atomic transaction: check DB + mark running
	err := stateManager.ExecuteAtomically(ctx, func(atomic core.AtomicStateContext) error {
		// Check if running in DB (another process might have started it)
		isRunning, err := atomic.IsWorkflowRunning(workflowID)
		if err != nil {
			return fmt.Errorf("checking running status: %w", err)
		}
		if isRunning {
			return fmt.Errorf("workflow is already running (in database)")
		}

		// Mark as running in DB
		if err := atomic.SetWorkflowRunning(workflowID); err != nil {
			return fmt.Errorf("marking workflow as running: %w", err)
		}

		// Load and update workflow state
		state, err := atomic.LoadByID(workflowID)
		if err != nil {
			return fmt.Errorf("loading workflow state: %w", err)
		}
		if state == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}

		// Update status
		state.Status = core.WorkflowStatusRunning
		state.Error = ""
		state.UpdatedAt = time.Now()
		now := time.Now().UTC()
		state.HeartbeatAt = &now

		if err := atomic.Save(state); err != nil {
			return fmt.Errorf("saving workflow state: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Transaction succeeded - register in memory
	t.handles[workflowID] = handle

	// Start heartbeat tracking
	if t.heartbeat != nil {
		t.heartbeat.Start(workflowID)
	}

	t.logger.Debug("started execution tracking",
		"workflow_id", workflowID,
		"started_at", handle.StartedAt)

	return handle, nil
}

// FinishExecution cleans up tracking when a workflow completes.
// It performs the following:
// 1. Clears running status in DB
// 2. Removes in-memory handle
// 3. Stops heartbeat tracking
// The StateManager is obtained from the context if a ProjectContext is available.
func (t *UnifiedTracker) FinishExecution(ctx context.Context, workflowID core.WorkflowID) error {
	t.mu.Lock()
	handle, exists := t.handles[workflowID]
	if exists {
		delete(t.handles, workflowID)
	}
	t.mu.Unlock()

	// Mark handle as done
	if handle != nil {
		handle.MarkDone()
	}

	// Stop heartbeat tracking
	if t.heartbeat != nil {
		t.heartbeat.Stop(workflowID)
	}

	// Get project-scoped StateManager if available
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Clear running status in DB
	if err := stateManager.ClearWorkflowRunning(ctx, workflowID); err != nil {
		t.logger.Warn("failed to clear running status in DB",
			"workflow_id", workflowID,
			"error", err)
		// Don't return error - memory is already cleaned up
	}

	t.logger.Debug("finished execution tracking",
		"workflow_id", workflowID,
		"was_tracked", exists)

	return nil
}

// IsRunning checks if a workflow is currently running.
// Checks in-memory first (fast path), then DB (authoritative).
// The StateManager is obtained from the context if a ProjectContext is available.
func (t *UnifiedTracker) IsRunning(ctx context.Context, workflowID core.WorkflowID) bool {
	// Fast path: check in-memory
	t.mu.RLock()
	_, exists := t.handles[workflowID]
	t.mu.RUnlock()

	if exists {
		return true
	}

	// Get project-scoped StateManager if available
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Slow path: check DB (another process might be running it)
	isRunning, err := stateManager.IsWorkflowRunning(ctx, workflowID)
	if err != nil {
		t.logger.Warn("failed to check running status in DB",
			"workflow_id", workflowID,
			"error", err)
		return false
	}

	return isRunning
}

// IsRunningInMemory checks only in-memory tracking (for local operations).
func (t *UnifiedTracker) IsRunningInMemory(workflowID core.WorkflowID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, exists := t.handles[workflowID]
	return exists
}

// GetHandle returns the ExecutionHandle for a running workflow.
func (t *UnifiedTracker) GetHandle(workflowID core.WorkflowID) (*ExecutionHandle, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	handle, exists := t.handles[workflowID]
	return handle, exists
}

// GetControlPlane returns the ControlPlane for a running workflow.
func (t *UnifiedTracker) GetControlPlane(workflowID core.WorkflowID) (*control.ControlPlane, bool) {
	handle, exists := t.GetHandle(workflowID)
	if !exists || handle == nil {
		return nil, false
	}
	return handle.ControlPlane, true
}

// Cancel cancels a running workflow.
func (t *UnifiedTracker) Cancel(workflowID core.WorkflowID) error {
	cp, ok := t.GetControlPlane(workflowID)
	if !ok {
		return fmt.Errorf("workflow is not running")
	}
	if cp.IsCancelled() {
		return fmt.Errorf("workflow is already being cancelled")
	}
	cp.Cancel()
	t.logger.Info("workflow cancellation requested", "workflow_id", workflowID)
	return nil
}

// Pause pauses a running workflow.
func (t *UnifiedTracker) Pause(workflowID core.WorkflowID) error {
	cp, ok := t.GetControlPlane(workflowID)
	if !ok {
		return fmt.Errorf("workflow is not running")
	}
	if cp.IsPaused() {
		return fmt.Errorf("workflow is already paused")
	}
	cp.Pause()
	t.logger.Info("workflow pause requested", "workflow_id", workflowID)
	return nil
}

// Resume resumes a paused workflow.
func (t *UnifiedTracker) Resume(workflowID core.WorkflowID) error {
	cp, ok := t.GetControlPlane(workflowID)
	if !ok {
		return fmt.Errorf("workflow is not running")
	}
	if !cp.IsPaused() {
		return fmt.Errorf("workflow is not paused")
	}
	cp.Resume()
	t.logger.Info("workflow resume requested", "workflow_id", workflowID)
	return nil
}

// ForceStop forcibly stops a workflow, even if it doesn't have an active handle.
// This is used for zombie workflows that appear running in the DB but have no in-memory state
// (e.g., after server restart). Unlike Cancel, ForceStop works without a ControlPlane.
func (t *UnifiedTracker) ForceStop(ctx context.Context, workflowID core.WorkflowID) error {
	// Try graceful cancel if handle exists
	t.mu.RLock()
	handle, exists := t.handles[workflowID]
	t.mu.RUnlock()

	if exists && handle != nil {
		if cp := handle.ControlPlane; cp != nil && !cp.IsCancelled() {
			cp.Cancel()
			t.logger.Info("workflow cancellation requested via force-stop", "workflow_id", workflowID)
		}
	}

	// Get state manager from context or use default
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Clear running_workflows entry
	if clearer, ok := stateManager.(interface {
		ClearWorkflowRunning(context.Context, core.WorkflowID) error
	}); ok {
		if err := clearer.ClearWorkflowRunning(ctx, workflowID); err != nil {
			t.logger.Warn("failed to clear running_workflows entry", "workflow_id", workflowID, "error", err)
		}
	}

	// Mark workflow as failed atomically
	if err := stateManager.ExecuteAtomically(ctx, func(atomic core.AtomicStateContext) error {
		state, err := atomic.LoadByID(workflowID)
		if err != nil || state == nil {
			return err
		}

		// Only update if still running
		if state.Status == core.WorkflowStatusRunning {
			state.Status = core.WorkflowStatusFailed
			state.Error = "Workflow forcibly stopped (orphaned after server restart)"
			state.UpdatedAt = time.Now()
			state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
				ID:        fmt.Sprintf("force-stop-%d", time.Now().UnixNano()),
				Type:      "force_stop",
				Phase:     state.CurrentPhase,
				Timestamp: time.Now(),
				Message:   "Workflow was forcibly stopped",
			})
			return atomic.Save(state)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update workflow state: %w", err)
	}

	t.logger.Info("workflow force-stopped", "workflow_id", workflowID)
	return nil
}

// ListRunning returns all workflows currently tracked as running.
func (t *UnifiedTracker) ListRunning() []core.WorkflowID {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ids := make([]core.WorkflowID, 0, len(t.handles))
	for id := range t.handles {
		ids = append(ids, id)
	}
	return ids
}

// WaitForConfirmation waits for the execution to confirm it started.
// This should be called after StartExecution to ensure the goroutine is running.
func (t *UnifiedTracker) WaitForConfirmation(workflowID core.WorkflowID) error {
	handle, exists := t.GetHandle(workflowID)
	if !exists {
		return fmt.Errorf("workflow not being tracked")
	}
	return handle.WaitForConfirmation(t.confirmTimeout)
}

// RollbackExecution cleans up a failed execution start.
// This should be called if StartExecution succeeded but the goroutine failed to start.
// The StateManager is obtained from the context if a ProjectContext is available.
func (t *UnifiedTracker) RollbackExecution(ctx context.Context, workflowID core.WorkflowID, failureReason string) error {
	t.logger.Warn("rolling back execution",
		"workflow_id", workflowID,
		"reason", failureReason)

	// Finish tracking (clears memory and DB)
	if err := t.FinishExecution(ctx, workflowID); err != nil {
		t.logger.Error("failed to finish execution during rollback",
			"workflow_id", workflowID,
			"error", err)
	}

	// Get project-scoped StateManager if available
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Update workflow state to failed
	err := stateManager.ExecuteAtomically(ctx, func(atomic core.AtomicStateContext) error {
		state, err := atomic.LoadByID(workflowID)
		if err != nil {
			return err
		}
		if state == nil {
			return nil // Already deleted?
		}

		state.Status = core.WorkflowStatusFailed
		state.Error = failureReason
		state.UpdatedAt = time.Now()

		return atomic.Save(state)
	})

	return err
}

// CleanupOrphanedWorkflows finds and cleans up workflows that are marked as running
// in the DB but have no in-memory handle (orphaned due to crash/restart).
func (t *UnifiedTracker) CleanupOrphanedWorkflows(ctx context.Context) (int, error) {
	// Get all running workflows from DB
	runningIDs, err := t.stateManager.ListRunningWorkflows(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing running workflows: %w", err)
	}

	cleaned := 0
	for _, id := range runningIDs {
		// If not in memory, it's orphaned
		if t.IsRunningInMemory(id) {
			continue
		}

		t.logger.Warn("cleaning up orphaned workflow",
			"workflow_id", id)

		// Clear running status
		if err := t.stateManager.ClearWorkflowRunning(ctx, id); err != nil {
			t.logger.Error("failed to clear orphaned workflow",
				"workflow_id", id,
				"error", err)
			continue
		}

		// Update state to failed
		state, err := t.stateManager.LoadByID(ctx, id)
		if err == nil && state != nil {
			state.Status = core.WorkflowStatusFailed
			state.Error = "Orphaned workflow (server restarted during execution)"
			state.UpdatedAt = time.Now()
			_ = t.stateManager.Save(ctx, state)
		}

		cleaned++
	}

	if cleaned > 0 {
		t.logger.Info("cleaned up orphaned workflows", "count", cleaned)
	}

	return cleaned, nil
}

// Shutdown stops all tracking and cleans up resources.
func (t *UnifiedTracker) Shutdown(_ context.Context) {
	t.mu.Lock()
	handles := make([]*ExecutionHandle, 0, len(t.handles))
	for _, h := range t.handles {
		handles = append(handles, h)
	}
	t.handles = make(map[core.WorkflowID]*ExecutionHandle)
	t.mu.Unlock()

	// Mark all handles as done
	for _, h := range handles {
		h.MarkDone()
	}

	// Stop heartbeats
	if t.heartbeat != nil {
		for _, h := range handles {
			t.heartbeat.Stop(h.WorkflowID)
		}
	}

	t.logger.Info("unified tracker shutdown complete", "tracked_count", len(handles))
}

// ConfirmTimeout returns the confirmation timeout setting.
func (t *UnifiedTracker) ConfirmTimeout() time.Duration {
	return t.confirmTimeout
}
