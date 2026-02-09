package api

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestUnifiedTracker_CancelCancelsExecContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := &UnifiedTracker{
		logger:         logger,
		handles:        make(map[core.WorkflowID]*ExecutionHandle),
		confirmTimeout: 5 * time.Second,
	}

	id := core.WorkflowID("wf-1")
	handle := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}

	called := make(chan struct{})
	handle.SetExecCancel(func() { close(called) })

	tracker.handles[id] = handle

	if err := tracker.Cancel(id); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	select {
	case <-called:
		// Expected
	default:
		t.Fatal("expected exec cancel to be called")
	}

	if !handle.ControlPlane.IsCancelled() {
		t.Fatal("expected control plane to be cancelled")
	}
}

func TestUnifiedTracker_CancelBeforeSetExecCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := &UnifiedTracker{
		logger:         logger,
		handles:        make(map[core.WorkflowID]*ExecutionHandle),
		confirmTimeout: 5 * time.Second,
	}

	id := core.WorkflowID("wf-2")
	handle := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}
	tracker.handles[id] = handle

	// Cancel before the exec cancel func is known.
	if err := tracker.Cancel(id); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	called := make(chan struct{})
	handle.SetExecCancel(func() { close(called) })

	select {
	case <-called:
		// Expected: SetExecCancel should immediately cancel if already requested.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected SetExecCancel to immediately cancel when already cancelled")
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandle(id core.WorkflowID) *ExecutionHandle {
	return &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		StartedAt:    time.Now(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}
}

func newTestHeartbeat(sm core.StateManager) *workflow.HeartbeatManager {
	return workflow.NewHeartbeatManager(
		workflow.HeartbeatConfig{
			Interval:       100 * time.Millisecond,
			StaleThreshold: 500 * time.Millisecond,
			CheckInterval:  100 * time.Millisecond,
		},
		sm,
		newTestLogger(),
	)
}

func TestUnifiedTracker_IsRunning_ReturnsFalseWhenHeartbeatUnhealthy(t *testing.T) {
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()

	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-zombie")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	// Heartbeat is NOT started for this workflow → IsHealthy returns false
	ctx := context.Background()
	if tracker.IsRunning(ctx, id) {
		t.Fatal("expected IsRunning to return false when heartbeat is unhealthy")
	}
}

func TestUnifiedTracker_IsRunning_ReturnsTrueWhenHeartbeatHealthy(t *testing.T) {
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()

	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-healthy")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	// Start heartbeat → IsHealthy returns true
	hb.Start(id)
	defer hb.Stop(id)

	ctx := context.Background()
	if !tracker.IsRunning(ctx, id) {
		t.Fatal("expected IsRunning to return true when heartbeat is healthy")
	}
}

func TestUnifiedTracker_IsRunning_ReturnsTrueWhenNoHeartbeat(t *testing.T) {
	sm := testutil.NewMockStateManager()

	// heartbeat is nil — disabled mode
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-no-hb")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	if !tracker.IsRunning(ctx, id) {
		t.Fatal("expected IsRunning to return true when heartbeat is nil (disabled)")
	}
}

func TestUnifiedTracker_IsRunning_FallsBackToDBWhenNoHandle(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-db-only",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	ctx := context.Background()
	// No in-memory handle, but DB says running
	// Note: MockStateManager.IsWorkflowRunning checks if state matches
	if !tracker.IsRunning(ctx, "wf-db-only") {
		t.Fatal("expected IsRunning to fall back to DB when no handle exists")
	}
}

func TestUnifiedTracker_ForceStop_CleansUpHandle(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-force")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	if err := tracker.ForceStop(ctx, id); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}

	if tracker.IsRunningInMemory(id) {
		t.Fatal("expected handle to be removed after ForceStop")
	}
}

func TestUnifiedTracker_ForceStop_StopsHeartbeat(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force-hb",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()

	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-force-hb")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	hb.Start(id)

	if !hb.IsTracking(id) {
		t.Fatal("expected heartbeat to be tracking before ForceStop")
	}

	ctx := context.Background()
	if err := tracker.ForceStop(ctx, id); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}

	if hb.IsTracking(id) {
		t.Fatal("expected heartbeat to stop tracking after ForceStop")
	}
}

func TestUnifiedTracker_ForceStop_WorksWithoutHeartbeat(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force-no-hb",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	// nil heartbeat
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-force-no-hb")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	if err := tracker.ForceStop(ctx, id); err != nil {
		t.Fatalf("ForceStop() should not panic with nil heartbeat, got error = %v", err)
	}

	if tracker.IsRunningInMemory(id) {
		t.Fatal("expected handle to be removed after ForceStop")
	}
}

func TestUnifiedTracker_ForceStop_FinishExecutionAfterForceStop_Idempotent(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-idempotent",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-idempotent")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	if err := tracker.ForceStop(ctx, id); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}

	// FinishExecution after ForceStop should be a no-op (handle already removed)
	if err := tracker.FinishExecution(ctx, id); err != nil {
		t.Fatalf("FinishExecution() after ForceStop should not error, got = %v", err)
	}
}

func TestUnifiedTracker_ForceStop_NewRunAfterForceStop(t *testing.T) {
	sm := testutil.NewMockStateManager()
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-rerun",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-rerun")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	if err := tracker.ForceStop(ctx, id); err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}

	// After ForceStop, state is "failed" — update it to pending for re-run
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-rerun",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusPending,
		},
	})

	// StartExecution should succeed (no "already running" error)
	newHandle, err := tracker.StartExecution(ctx, id)
	if err != nil {
		t.Fatalf("StartExecution() after ForceStop should succeed, got error = %v", err)
	}
	if newHandle == nil {
		t.Fatal("expected a new handle from StartExecution")
	}

	// Clean up
	_ = tracker.FinishExecution(ctx, id)
}

func TestUnifiedTracker_IsHeartbeatHealthy_NilHeartbeat(t *testing.T) {
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	if !tracker.IsHeartbeatHealthy("wf-any") {
		t.Fatal("expected IsHeartbeatHealthy to return true when heartbeat is nil")
	}
}

func TestUnifiedTracker_CleanupOrphanedWorkflows_DetectsFinishedButUncleaned(t *testing.T) {
	sm := testutil.NewMockStateManager()
	id := core.WorkflowID("wf-uncleaned")
	sm.SetState(&core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: id,
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	})

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	// Create a handle with a closed Done() channel (simulates goroutine that finished
	// but FinishExecution was never called, e.g. due to a panic).
	handle := newTestHandle(id)
	handle.MarkDone() // Close the done channel
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	ctx := context.Background()
	cleaned, err := tracker.CleanupOrphanedWorkflows(ctx)
	if err != nil {
		t.Fatalf("CleanupOrphanedWorkflows() error = %v", err)
	}

	if cleaned != 1 {
		t.Errorf("CleanupOrphanedWorkflows() cleaned = %d, want 1", cleaned)
	}

	// Handle should be removed after cleanup
	if tracker.IsRunningInMemory(id) {
		t.Fatal("expected handle to be removed after cleanup of finished-but-uncleaned workflow")
	}
}
