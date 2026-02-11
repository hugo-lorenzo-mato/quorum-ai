package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// --- ExecutionHandle tests ---

func TestExecutionHandle_SetExecCancel_NilReceiver(t *testing.T) {
	t.Parallel()
	var h *ExecutionHandle
	// Should not panic.
	h.SetExecCancel(func() {})
}

func TestExecutionHandle_SetExecCancel_NilCancel(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-nil-cancel")
	// Should not panic.
	h.SetExecCancel(nil)
}

func TestExecutionHandle_SetExecCancel_DoubleSet(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-double-set")
	called1 := false
	called2 := false
	h.SetExecCancel(func() { called1 = true })
	// Second call should be a no-op (first function wins).
	h.SetExecCancel(func() { called2 = true })

	h.CancelExec()
	if !called1 {
		t.Error("expected first cancel func to be called")
	}
	if called2 {
		t.Error("second cancel func should not have been stored")
	}
}

func TestExecutionHandle_CancelExec_NilReceiver(t *testing.T) {
	t.Parallel()
	var h *ExecutionHandle
	// Should not panic.
	h.CancelExec()
}

func TestExecutionHandle_CancelExec_NoCancelFunc(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-no-cancel")
	// Should not panic when no cancel func is set.
	h.CancelExec()
}

func TestExecutionHandle_CancelExec_Idempotent(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-idempotent-cancel")
	callCount := 0
	h.SetExecCancel(func() { callCount++ })

	h.CancelExec()
	h.CancelExec() // Second call should be safe.

	if callCount != 1 {
		t.Errorf("expected cancel to be called once, called %d times", callCount)
	}
}

func TestExecutionHandle_WaitForConfirmation_Success(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-confirm-ok")
	go func() {
		time.Sleep(10 * time.Millisecond)
		h.ConfirmStarted()
	}()

	err := h.WaitForConfirmation(1 * time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestExecutionHandle_WaitForConfirmation_Error(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-confirm-err")
	go func() {
		time.Sleep(10 * time.Millisecond)
		h.ReportError(fmt.Errorf("start failed"))
	}()

	err := h.WaitForConfirmation(1 * time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "start failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecutionHandle_WaitForConfirmation_Timeout(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-confirm-timeout")

	err := h.WaitForConfirmation(50 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecutionHandle_ConfirmStarted_Idempotent(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-confirm-twice")
	h.ConfirmStarted()
	// Second call should not panic (channel already closed).
	h.ConfirmStarted()
}

func TestExecutionHandle_ReportError_DoesNotBlock(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-report-err")
	// Fill the error channel.
	h.ReportError(fmt.Errorf("first"))
	// Second call should not block (buffer full, select default branch).
	h.ReportError(fmt.Errorf("second"))
}

func TestExecutionHandle_Done(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-done")
	select {
	case <-h.Done():
		t.Fatal("done channel should not be closed yet")
	default:
		// Expected.
	}

	h.MarkDone()
	select {
	case <-h.Done():
		// Expected.
	default:
		t.Fatal("done channel should be closed after MarkDone")
	}
}

func TestExecutionHandle_MarkDone_Idempotent(t *testing.T) {
	t.Parallel()
	h := newTestHandle("wf-mark-done-twice")
	h.MarkDone()
	// Second call should not panic.
	h.MarkDone()
}

// --- UnifiedTracker tests ---

func TestNewUnifiedTracker_DefaultConfirmTimeout(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), UnifiedTrackerConfig{})
	if tracker.ConfirmTimeout() != 5*time.Second {
		t.Errorf("expected default confirm timeout 5s, got %v", tracker.ConfirmTimeout())
	}
}

func TestNewUnifiedTracker_CustomConfirmTimeout(t *testing.T) {
	t.Parallel()
	cfg := UnifiedTrackerConfig{ConfirmTimeout: 10 * time.Second}
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), cfg)
	if tracker.ConfirmTimeout() != 10*time.Second {
		t.Errorf("expected confirm timeout 10s, got %v", tracker.ConfirmTimeout())
	}
}

func TestDefaultUnifiedTrackerConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultUnifiedTrackerConfig()
	if cfg.ConfirmTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", cfg.ConfirmTimeout)
	}
}

func TestUnifiedTracker_StartExecution_AlreadyRunningInMemory(t *testing.T) {
	t.Parallel()
	sm := testutil.NewMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-dup")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	_, err := tracker.StartExecution(context.Background(), id)
	if err == nil {
		t.Fatal("expected error for duplicate execution")
	}
	if err.Error() != "workflow is already running (in memory)" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnifiedTracker_StartExecution_AlreadyRunningInDB(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-db-dup")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-db-dup",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	}

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	_, err := tracker.StartExecution(context.Background(), core.WorkflowID("wf-db-dup"))
	if err == nil {
		t.Fatal("expected error for workflow already running in DB")
	}
}

func TestUnifiedTracker_StartExecution_Success(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-start-ok")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-start-ok",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusPending,
		},
	}

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	handle, err := tracker.StartExecution(context.Background(), "wf-start-ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle")
	}
	if handle.WorkflowID != "wf-start-ok" {
		t.Errorf("handle workflow ID = %q, want 'wf-start-ok'", handle.WorkflowID)
	}
	if handle.ControlPlane == nil {
		t.Error("expected non-nil ControlPlane")
	}

	// Verify in-memory tracking.
	if !tracker.IsRunningInMemory("wf-start-ok") {
		t.Error("expected workflow to be tracked in memory")
	}

	// Clean up.
	_ = tracker.FinishExecution(context.Background(), "wf-start-ok")
}

func TestUnifiedTracker_FinishExecution_NotTracked(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	// Should not error even if workflow is not tracked.
	err := tracker.FinishExecution(context.Background(), "wf-not-tracked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnifiedTracker_FinishExecution_ClearsHandleAndMarksDone(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-finish-ok")
	handle := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = handle
	tracker.mu.Unlock()

	err := tracker.FinishExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tracker.IsRunningInMemory(id) {
		t.Error("expected workflow to be removed from memory")
	}

	select {
	case <-handle.Done():
		// Expected.
	default:
		t.Error("expected done channel to be closed")
	}
}

func TestUnifiedTracker_IsRunning_NotInMemoryNotInDB(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	if tracker.IsRunning(context.Background(), "wf-nonexistent") {
		t.Error("expected IsRunning to return false")
	}
}

func TestUnifiedTracker_IsRunningInMemory_False(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	if tracker.IsRunningInMemory("wf-nope") {
		t.Error("expected false for non-tracked workflow")
	}
}

func TestUnifiedTracker_GetHandle_NotFound(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	handle, exists := tracker.GetHandle("wf-missing")
	if exists || handle != nil {
		t.Error("expected not found")
	}
}

func TestUnifiedTracker_GetHandle_Found(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-found")
	expected := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = expected
	tracker.mu.Unlock()

	handle, exists := tracker.GetHandle(id)
	if !exists {
		t.Fatal("expected handle to exist")
	}
	if handle != expected {
		t.Error("handle mismatch")
	}
}

func TestUnifiedTracker_GetControlPlane_NotFound(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	cp, ok := tracker.GetControlPlane("wf-missing")
	if ok || cp != nil {
		t.Error("expected not found")
	}
}

func TestUnifiedTracker_GetControlPlane_Found(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-cp")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	cp, ok := tracker.GetControlPlane(id)
	if !ok {
		t.Fatal("expected control plane to exist")
	}
	if cp != h.ControlPlane {
		t.Error("control plane mismatch")
	}
}

func TestUnifiedTracker_Cancel_NotRunning(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	err := tracker.Cancel("wf-missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnifiedTracker_Cancel_AlreadyCancelled(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-already-cancelled")
	h := newTestHandle(id)
	h.ControlPlane.Cancel() // Pre-cancel.
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Cancel(id)
	if err == nil || err.Error() != "workflow is already being cancelled" {
		t.Errorf("expected 'already being cancelled' error, got %v", err)
	}
}

func TestUnifiedTracker_Pause_NotRunning(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	err := tracker.Pause("wf-missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnifiedTracker_Pause_Success(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-pause")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Pause(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.ControlPlane.IsPaused() {
		t.Error("expected control plane to be paused")
	}
}

func TestUnifiedTracker_Pause_AlreadyPaused(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-already-paused")
	h := newTestHandle(id)
	h.ControlPlane.Pause()
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Pause(id)
	if err == nil || err.Error() != "workflow is already paused" {
		t.Errorf("expected 'already paused' error, got %v", err)
	}
}

func TestUnifiedTracker_Resume_NotRunning(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	err := tracker.Resume("wf-missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnifiedTracker_Resume_NotPaused(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-not-paused")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Resume(id)
	if err == nil || err.Error() != "workflow is not paused" {
		t.Errorf("expected 'not paused' error, got %v", err)
	}
}

func TestUnifiedTracker_Resume_Success(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-resume")
	h := newTestHandle(id)
	h.ControlPlane.Pause()
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Resume(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ControlPlane.IsPaused() {
		t.Error("expected control plane to not be paused after resume")
	}
}

func TestUnifiedTracker_ForceStop_NotTracked(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-force-not-tracked")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force-not-tracked",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	}
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	// ForceStop on a workflow not tracked in memory but present in DB.
	err := tracker.ForceStop(context.Background(), "wf-force-not-tracked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnifiedTracker_ForceStop_CancelsControlPlane(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-force-cp")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force-cp",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	}
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-force-cp")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.ForceStop(context.Background(), id)
	if err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}

	if !h.ControlPlane.IsCancelled() {
		t.Error("expected control plane to be cancelled after ForceStop")
	}
}

func TestUnifiedTracker_ForceStop_AlreadyCancelled(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-force-already")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-force-already",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	}
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-force-already")
	h := newTestHandle(id)
	h.ControlPlane.Cancel() // Pre-cancel.
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	// Should still succeed (idempotent).
	err := tracker.ForceStop(context.Background(), id)
	if err != nil {
		t.Fatalf("ForceStop() error = %v", err)
	}
}

func TestUnifiedTracker_ListRunning_Empty(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	ids := tracker.ListRunning()
	if len(ids) != 0 {
		t.Errorf("expected 0, got %d", len(ids))
	}
}

func TestUnifiedTracker_ListRunning_Multiple(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	for _, id := range []string{"wf-a", "wf-b", "wf-c"} {
		h := newTestHandle(core.WorkflowID(id))
		tracker.mu.Lock()
		tracker.handles[core.WorkflowID(id)] = h
		tracker.mu.Unlock()
	}

	ids := tracker.ListRunning()
	if len(ids) != 3 {
		t.Errorf("expected 3, got %d", len(ids))
	}
}

func TestUnifiedTracker_WaitForConfirmation_NotTracked(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	err := tracker.WaitForConfirmation("wf-missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnifiedTracker_WaitForConfirmation_Success(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), UnifiedTrackerConfig{ConfirmTimeout: 2 * time.Second})

	id := core.WorkflowID("wf-wait-ok")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	go func() {
		time.Sleep(10 * time.Millisecond)
		h.ConfirmStarted()
	}()

	err := tracker.WaitForConfirmation(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnifiedTracker_RollbackExecution(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-rollback")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-rollback",
		},
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
		},
	}
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	h := newTestHandle(core.WorkflowID("wf-rollback"))
	tracker.mu.Lock()
	tracker.handles[core.WorkflowID("wf-rollback")] = h
	tracker.mu.Unlock()

	err := tracker.RollbackExecution(context.Background(), "wf-rollback", "test failure")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Handle should be removed.
	if tracker.IsRunningInMemory("wf-rollback") {
		t.Error("expected handle to be removed after rollback")
	}

	// Workflow should be marked as failed.
	state := sm.workflows[core.WorkflowID("wf-rollback")]
	if state.Status != core.WorkflowStatusFailed {
		t.Errorf("expected status failed, got %v", state.Status)
	}
	if state.Error != "test failure" {
		t.Errorf("expected error 'test failure', got %q", state.Error)
	}
}

func TestUnifiedTracker_RollbackExecution_NoState(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	// Rollback for non-existent workflow should not fail.
	err := tracker.RollbackExecution(context.Background(), "wf-nonexistent", "reason")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUnifiedTracker_Shutdown(t *testing.T) {
	t.Parallel()
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	// Add some handles.
	handles := make([]*ExecutionHandle, 3)
	for i, id := range []string{"wf-s1", "wf-s2", "wf-s3"} {
		wID := core.WorkflowID(id)
		h := newTestHandle(wID)
		handles[i] = h
		tracker.mu.Lock()
		tracker.handles[wID] = h
		tracker.mu.Unlock()
		hb.Start(wID, nil)
	}

	tracker.Shutdown(context.Background())

	// All handles should be marked done.
	for _, h := range handles {
		select {
		case <-h.Done():
			// Expected.
		default:
			t.Errorf("handle for %s should be marked done", h.WorkflowID)
		}
	}

	// No handles should remain.
	if len(tracker.ListRunning()) != 0 {
		t.Error("expected no running workflows after shutdown")
	}
}

func TestUnifiedTracker_IsHeartbeatHealthy_WithHeartbeat(t *testing.T) {
	t.Parallel()
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()
	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-hb-healthy")
	hb.Start(id, nil)
	defer hb.Stop(id)

	if !tracker.IsHeartbeatHealthy(id) {
		t.Error("expected healthy heartbeat")
	}
}

func TestUnifiedTracker_IsHeartbeatHealthy_Unhealthy(t *testing.T) {
	t.Parallel()
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()
	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-hb-unhealthy")
	// Not started — should be unhealthy.
	if tracker.IsHeartbeatHealthy(id) {
		t.Error("expected unhealthy heartbeat for unstarted workflow")
	}
}

func TestUnifiedTracker_Cancel_NilControlPlane(t *testing.T) {
	t.Parallel()
	tracker := NewUnifiedTracker(nil, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-nil-cp")
	h := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: nil, // nil ControlPlane
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	err := tracker.Cancel(id)
	if err == nil {
		t.Fatal("expected error for nil ControlPlane")
	}
}

func TestUnifiedTracker_SetExecCancel_ImmediateCancelWhenAlreadyCancelled(t *testing.T) {
	t.Parallel()
	id := core.WorkflowID("wf-immediate-cancel")
	h := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}

	// Cancel the control plane first.
	h.ControlPlane.Cancel()

	// Now set the exec cancel func — it should be called immediately.
	called := make(chan struct{})
	h.SetExecCancel(func() { close(called) })

	select {
	case <-called:
		// Expected: SetExecCancel should immediately cancel.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected immediate cancel when ControlPlane already cancelled")
	}
}

func TestUnifiedTracker_CleanupOrphanedWorkflows_NoOrphans(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	cleaned, err := tracker.CleanupOrphanedWorkflows(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned, got %d", cleaned)
	}
}

func TestUnifiedTracker_CleanupOrphanedWorkflows_StillRunningInMemory(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	id := core.WorkflowID("wf-still-running")
	sm.workflows[id] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: id},
		WorkflowRun:        core.WorkflowRun{Status: core.WorkflowStatusRunning},
	}

	tracker := NewUnifiedTracker(sm, nil, newTestLogger(), DefaultUnifiedTrackerConfig())

	h := newTestHandle(id)
	// Do NOT close done channel, simulating a still-running goroutine.
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()

	cleaned, err := tracker.CleanupOrphanedWorkflows(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned (still running), got %d", cleaned)
	}
}

func TestUnifiedTracker_FinishExecution_WithHeartbeat(t *testing.T) {
	t.Parallel()
	sm := testutil.NewMockStateManager()
	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()
	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	id := core.WorkflowID("wf-finish-hb")
	h := newTestHandle(id)
	tracker.mu.Lock()
	tracker.handles[id] = h
	tracker.mu.Unlock()
	hb.Start(id, nil)

	if !hb.IsTracking(id) {
		t.Fatal("expected heartbeat to be tracking")
	}

	err := tracker.FinishExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hb.IsTracking(id) {
		t.Error("expected heartbeat to stop tracking after finish")
	}
}

func TestUnifiedTracker_StartExecution_WithHeartbeat(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-start-hb")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-start-hb"},
		WorkflowRun:        core.WorkflowRun{Status: core.WorkflowStatusPending},
	}

	hb := newTestHeartbeat(sm)
	defer hb.Shutdown()
	tracker := NewUnifiedTracker(sm, hb, newTestLogger(), DefaultUnifiedTrackerConfig())

	handle, err := tracker.StartExecution(context.Background(), "wf-start-hb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle")
	}

	if !hb.IsTracking("wf-start-hb") {
		t.Error("expected heartbeat to be tracking after start")
	}

	_ = tracker.FinishExecution(context.Background(), "wf-start-hb")
}
