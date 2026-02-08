package api

import (
	"context"
	"errors"
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

	// Cancel function for the execution context created by the API layer.
	// This allows Cancel/ForceStop to interrupt in-flight agent processes (best-effort).
	execCancelMu sync.Mutex
	execCancel   context.CancelFunc

	// Confirmation channel - closed when goroutine confirms start
	confirmCh chan struct{}
	// Error channel - receives error if start fails
	errorCh chan error
	// Done channel - closed when execution completes
	doneCh chan struct{}
}

// SetExecCancel sets the execution cancel function.
// If cancellation has already been requested via the ControlPlane, this will
// immediately cancel the execution context (best-effort).
func (h *ExecutionHandle) SetExecCancel(cancel context.CancelFunc) {
	if h == nil || cancel == nil {
		return
	}

	h.execCancelMu.Lock()
	alreadySet := h.execCancel != nil
	if !alreadySet {
		h.execCancel = cancel
	}
	shouldCancel := h.ControlPlane != nil && h.ControlPlane.IsCancelled()
	h.execCancelMu.Unlock()

	if !alreadySet && shouldCancel {
		h.CancelExec()
	}
}

// CancelExec cancels the execution context (best-effort).
// It is safe to call multiple times.
func (h *ExecutionHandle) CancelExec() {
	if h == nil {
		return
	}
	h.execCancelMu.Lock()
	cancel := h.execCancel
	// Clear to ensure idempotence and avoid holding references longer than needed.
	h.execCancel = nil
	h.execCancelMu.Unlock()

	if cancel != nil {
		cancel()
	}
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
			if errors.Is(err, core.ErrState("WORKFLOW_ALREADY_RUNNING", "")) {
				return fmt.Errorf("workflow is already running (in database)")
			}
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
// When a handle exists but the heartbeat is unhealthy, the workflow is considered
// not running (zombie). When heartbeat is disabled, trusts the handle.
// The StateManager is obtained from the context if a ProjectContext is available.
func (t *UnifiedTracker) IsRunning(ctx context.Context, workflowID core.WorkflowID) bool {
	t.mu.RLock()
	_, exists := t.handles[workflowID]
	t.mu.RUnlock()

	if exists {
		// Handle exists, but verify the heartbeat is healthy (when available).
		// A stale heartbeat means the execution goroutine is hung.
		// When heartbeat is nil (disabled), this check is skipped and we
		// trust the handle — zombies must be resolved via manual ForceStop.
		if t.heartbeat != nil && !t.heartbeat.IsHealthy(workflowID) {
			return false
		}
		return true
	}

	// Slow path: check DB (another process might be running it)
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)
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

// IsHeartbeatHealthy checks if a workflow's heartbeat is being written successfully.
// When heartbeat is disabled (nil), returns true (assume healthy — no data to say otherwise).
func (t *UnifiedTracker) IsHeartbeatHealthy(workflowID core.WorkflowID) bool {
	if t.heartbeat == nil {
		return true // No heartbeat system → assume healthy
	}
	return t.heartbeat.IsHealthy(workflowID)
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
	handle, ok := t.GetHandle(workflowID)
	if !ok || handle == nil || handle.ControlPlane == nil {
		return fmt.Errorf("workflow is not running")
	}
	cp := handle.ControlPlane
	if cp.IsCancelled() {
		// Best-effort: ensure the execution context is also cancelled even if
		// the control plane was already marked cancelled (idempotent).
		handle.CancelExec()
		return fmt.Errorf("workflow is already being cancelled")
	}
	cp.Cancel()
	handle.CancelExec()
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

// ForceStop forcibly stops a workflow, cleaning up both in-memory and DB state.
// This is the primary recovery mechanism for zombie workflows. Unlike Cancel,
// ForceStop works without a ControlPlane and removes the in-memory handle to
// prevent the "already running" error on subsequent /run requests.
func (t *UnifiedTracker) ForceStop(ctx context.Context, workflowID core.WorkflowID) error {
	// Extract and remove handle from in-memory tracking.
	t.mu.Lock()
	handle, exists := t.handles[workflowID]
	if exists {
		delete(t.handles, workflowID)
	}
	t.mu.Unlock()

	// Cancel execution if handle was present.
	if exists && handle != nil {
		if cp := handle.ControlPlane; cp != nil && !cp.IsCancelled() {
			cp.Cancel()
			t.logger.Info("workflow cancellation requested via force-stop", "workflow_id", workflowID)
		}
		handle.CancelExec()
		handle.MarkDone()

		// Wait briefly for the goroutine to finish its own cleanup (FinishExecution).
		select {
		case <-handle.Done():
		case <-time.After(2 * time.Second):
			t.logger.Debug("force-stop: goroutine did not finish within grace period", "workflow_id", workflowID)
		}
	}

	// Stop heartbeat tracking.
	if t.heartbeat != nil {
		t.heartbeat.Stop(workflowID)
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
			state.Error = "Workflow forcibly stopped (orphaned or zombie)"
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
	// Get project-scoped StateManager if available.
	stateManager := GetStateManagerFromContext(ctx, t.stateManager)

	// Get all running workflows from DB
	runningIDs, err := stateManager.ListRunningWorkflows(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing running workflows: %w", err)
	}

	cleaned := 0
	for _, id := range runningIDs {
		// If tracked in-memory, skip.
		if t.IsRunningInMemory(id) {
			continue
		}

		// Without lock-holder metadata, we cannot safely distinguish between:
		// - A workflow orphaned by a crash/restart (safe to recover)
		// - A workflow running in a different server process (unsafe to touch)
		provider, ok := stateManager.(interface {
			GetRunningWorkflowRecord(context.Context, core.WorkflowID) (*core.RunningWorkflowRecord, error)
		})
		if !ok {
			t.logger.Warn("skipping orphan cleanup: state manager does not expose running_workflows metadata",
				"workflow_id", id)
			continue
		}

		rec, err := provider.GetRunningWorkflowRecord(ctx, id)
		if err != nil {
			t.logger.Warn("skipping orphan cleanup: failed to read running_workflows record",
				"workflow_id", id,
				"error", err)
			continue
		}
		if rec == nil {
			// Not actually running anymore (or already cleared).
			continue
		}

		if !isProvablyOrphan(rec) {
			t.logger.Info("skipping orphan cleanup: lock holder still alive or remote",
				"workflow_id", id,
				"lock_holder_pid", rec.LockHolderPID,
				"lock_holder_host", rec.LockHolderHost)
			continue
		}

		t.logger.Warn("cleaning up orphaned workflow",
			"workflow_id", id,
			"lock_holder_pid", rec.LockHolderPID,
			"lock_holder_host", rec.LockHolderHost)

		// Clear running marker + update workflow state atomically.
		err = stateManager.ExecuteAtomically(ctx, func(atomic core.AtomicStateContext) error {
			if err := atomic.ClearWorkflowRunning(id); err != nil {
				return err
			}

			state, err := atomic.LoadByID(id)
			if err != nil || state == nil {
				return err
			}

			// Only update if still running.
			if state.Status == core.WorkflowStatusRunning {
				state.Status = core.WorkflowStatusFailed
				if rec.LockHolderPID != nil && rec.LockHolderHost != "" {
					state.Error = fmt.Sprintf("Orphaned workflow (previous holder pid %d on %s is not alive)", *rec.LockHolderPID, rec.LockHolderHost)
				} else {
					state.Error = "Orphaned workflow (previous holder is not alive)"
				}
				state.UpdatedAt = time.Now()
				return atomic.Save(state)
			}
			return nil
		})
		if err != nil {
			t.logger.Error("failed to recover orphaned workflow",
				"workflow_id", id,
				"error", err)
			continue
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
