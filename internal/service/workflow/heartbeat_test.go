package workflow

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// heartbeatMockStateManager is a minimal mock for heartbeat tests.
type heartbeatMockStateManager struct {
	mu              sync.Mutex
	updateErr       error
	updateCallCount int
	zombies         []*core.WorkflowState
}

func (m *heartbeatMockStateManager) UpdateHeartbeat(_ context.Context, _ core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCallCount++
	return m.updateErr
}

func (m *heartbeatMockStateManager) FindZombieWorkflows(_ context.Context, _ time.Duration) ([]*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.zombies, nil
}

// Stub all other StateManager methods (not used by heartbeat).
func (m *heartbeatMockStateManager) Save(context.Context, *core.WorkflowState) error     { return nil }
func (m *heartbeatMockStateManager) Load(context.Context) (*core.WorkflowState, error)    { return nil, nil }
func (m *heartbeatMockStateManager) LoadByID(context.Context, core.WorkflowID) (*core.WorkflowState, error) {
	return nil, nil
}
func (m *heartbeatMockStateManager) ListWorkflows(context.Context) ([]core.WorkflowSummary, error) {
	return nil, nil
}
func (m *heartbeatMockStateManager) GetActiveWorkflowID(context.Context) (core.WorkflowID, error) {
	return "", nil
}
func (m *heartbeatMockStateManager) SetActiveWorkflowID(context.Context, core.WorkflowID) error {
	return nil
}
func (m *heartbeatMockStateManager) AcquireLock(context.Context) error                   { return nil }
func (m *heartbeatMockStateManager) ReleaseLock(context.Context) error                   { return nil }
func (m *heartbeatMockStateManager) AcquireWorkflowLock(context.Context, core.WorkflowID) error { return nil }
func (m *heartbeatMockStateManager) ReleaseWorkflowLock(context.Context, core.WorkflowID) error { return nil }
func (m *heartbeatMockStateManager) RefreshWorkflowLock(context.Context, core.WorkflowID) error { return nil }
func (m *heartbeatMockStateManager) SetWorkflowRunning(context.Context, core.WorkflowID) error  { return nil }
func (m *heartbeatMockStateManager) ClearWorkflowRunning(context.Context, core.WorkflowID) error {
	return nil
}
func (m *heartbeatMockStateManager) ListRunningWorkflows(context.Context) ([]core.WorkflowID, error) {
	return nil, nil
}
func (m *heartbeatMockStateManager) IsWorkflowRunning(context.Context, core.WorkflowID) (bool, error) {
	return false, nil
}
func (m *heartbeatMockStateManager) UpdateWorkflowHeartbeat(context.Context, core.WorkflowID) error {
	return nil
}
func (m *heartbeatMockStateManager) Exists() bool                                   { return true }
func (m *heartbeatMockStateManager) Backup(context.Context) error                   { return nil }
func (m *heartbeatMockStateManager) Restore(context.Context) (*core.WorkflowState, error) { return nil, nil }
func (m *heartbeatMockStateManager) DeactivateWorkflow(context.Context) error              { return nil }
func (m *heartbeatMockStateManager) ArchiveWorkflows(context.Context) (int, error)         { return 0, nil }
func (m *heartbeatMockStateManager) PurgeAllWorkflows(context.Context) (int, error)        { return 0, nil }
func (m *heartbeatMockStateManager) DeleteWorkflow(context.Context, core.WorkflowID) error { return nil }
func (m *heartbeatMockStateManager) FindWorkflowsByPrompt(context.Context, string) ([]core.DuplicateWorkflowInfo, error) {
	return nil, nil
}
func (m *heartbeatMockStateManager) ExecuteAtomically(_ context.Context, _ func(core.AtomicStateContext) error) error {
	return nil
}

var _ core.StateManager = (*heartbeatMockStateManager)(nil)

func newTestHeartbeatManager(sm core.StateManager) *HeartbeatManager {
	return NewHeartbeatManager(
		HeartbeatConfig{
			Interval:       100 * time.Millisecond,
			StaleThreshold: 500 * time.Millisecond,
			CheckInterval:  100 * time.Millisecond,
		},
		sm,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func TestHeartbeatManager_IsHealthy_TrackedAndRecent(t *testing.T) {
	sm := &heartbeatMockStateManager{}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	wfID := core.WorkflowID("wf-test-1")
	hm.Start(wfID)

	if !hm.IsHealthy(wfID) {
		t.Fatal("expected IsHealthy to return true immediately after Start")
	}
}

func TestHeartbeatManager_IsHealthy_TrackedButStale(t *testing.T) {
	sm := &heartbeatMockStateManager{}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	wfID := core.WorkflowID("wf-test-2")

	// Manually set a stale lastWriteSuccess
	hm.mu.Lock()
	hm.active[wfID] = func() {} // dummy cancel
	hm.lastWriteSuccess[wfID] = time.Now().Add(-time.Second) // older than 500ms threshold
	hm.mu.Unlock()

	if hm.IsHealthy(wfID) {
		t.Fatal("expected IsHealthy to return false for stale heartbeat")
	}
}

func TestHeartbeatManager_IsHealthy_NotTracked(t *testing.T) {
	sm := &heartbeatMockStateManager{}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	if hm.IsHealthy(core.WorkflowID("wf-nonexistent")) {
		t.Fatal("expected IsHealthy to return false for untracked workflow")
	}
}

func TestHeartbeatManager_IsHealthy_AfterStop(t *testing.T) {
	sm := &heartbeatMockStateManager{}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	wfID := core.WorkflowID("wf-test-3")
	hm.Start(wfID)

	if !hm.IsHealthy(wfID) {
		t.Fatal("expected IsHealthy to be true after Start")
	}

	hm.Stop(wfID)

	if hm.IsHealthy(wfID) {
		t.Fatal("expected IsHealthy to return false after Stop")
	}
}

func TestHeartbeatManager_WriteHeartbeat_UpdatesLastSuccess(t *testing.T) {
	sm := &heartbeatMockStateManager{}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	wfID := core.WorkflowID("wf-test-4")

	// Initialize tracking so lastWriteSuccess exists
	hm.mu.Lock()
	hm.lastWriteSuccess[wfID] = time.Now().Add(-time.Minute) // start old
	hm.mu.Unlock()

	before := time.Now()
	hm.writeHeartbeat(wfID)

	hm.mu.Lock()
	lastWrite := hm.lastWriteSuccess[wfID]
	hm.mu.Unlock()

	if lastWrite.Before(before) {
		t.Fatal("expected lastWriteSuccess to be updated after successful write")
	}
}

func TestHeartbeatManager_WriteHeartbeat_FailureDoesNotUpdateLastSuccess(t *testing.T) {
	sm := &heartbeatMockStateManager{updateErr: errors.New("db error")}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	wfID := core.WorkflowID("wf-test-5")

	oldTime := time.Now().Add(-time.Minute)
	hm.mu.Lock()
	hm.lastWriteSuccess[wfID] = oldTime
	hm.mu.Unlock()

	hm.writeHeartbeat(wfID)

	hm.mu.Lock()
	lastWrite := hm.lastWriteSuccess[wfID]
	hm.mu.Unlock()

	if !lastWrite.Equal(oldTime) {
		t.Fatal("expected lastWriteSuccess to remain unchanged after failed write")
	}
}

func TestHeartbeatManager_DetectZombies_SkipsRecentTracked(t *testing.T) {
	recentHeartbeat := time.Now().Add(-100 * time.Millisecond) // well within 3x threshold (1.5s)
	zombieState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-tracked-recent",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: "execute",
			HeartbeatAt:  &recentHeartbeat,
		},
	}
	sm := &heartbeatMockStateManager{zombies: []*core.WorkflowState{zombieState}}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	// Mark as tracked
	hm.Start(zombieState.WorkflowID)

	var handlerCalled bool
	hm.zombieHandler = func(state *core.WorkflowState) {
		handlerCalled = true
	}

	hm.detectZombies()

	if handlerCalled {
		t.Fatal("expected zombie handler NOT to be called for recently tracked workflow")
	}
}

func TestHeartbeatManager_DetectZombies_CatchesCriticallyStaleTracked(t *testing.T) {
	// HeartbeatAt older than 3x stale threshold (3 * 500ms = 1.5s)
	staleHeartbeat := time.Now().Add(-2 * time.Second)
	zombieState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-tracked-stale",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: "execute",
			HeartbeatAt:  &staleHeartbeat,
		},
	}
	sm := &heartbeatMockStateManager{zombies: []*core.WorkflowState{zombieState}}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	// Mark as tracked
	hm.mu.Lock()
	hm.active[zombieState.WorkflowID] = func() {}
	hm.lastWriteSuccess[zombieState.WorkflowID] = time.Now()
	hm.mu.Unlock()

	var handlerCalled bool
	hm.zombieHandler = func(state *core.WorkflowState) {
		handlerCalled = true
	}

	hm.detectZombies()

	if !handlerCalled {
		t.Fatal("expected zombie handler to be called for critically stale tracked workflow")
	}

	// Verify tracking was cleaned up
	if hm.IsTracking(zombieState.WorkflowID) {
		t.Fatal("expected tracking to be cleaned up after zombie detection")
	}
}

func TestHeartbeatManager_DetectZombies_CatchesUntrackedZombie(t *testing.T) {
	staleHeartbeat := time.Now().Add(-time.Hour)
	zombieState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-untracked",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: "execute",
			HeartbeatAt:  &staleHeartbeat,
		},
	}
	sm := &heartbeatMockStateManager{zombies: []*core.WorkflowState{zombieState}}
	hm := newTestHeartbeatManager(sm)
	defer hm.Shutdown()

	var handlerCalled bool
	hm.zombieHandler = func(state *core.WorkflowState) {
		handlerCalled = true
	}

	hm.detectZombies()

	if !handlerCalled {
		t.Fatal("expected zombie handler to be called for untracked zombie workflow")
	}
}
