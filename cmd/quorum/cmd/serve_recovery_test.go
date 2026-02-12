package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// =============================================================================
// Mock state manager for serve tests
// =============================================================================

// mockServeSM is a configurable mock that implements core.StateManager.
// Only the methods used by recoverZombieWorkflows and migrateWorkflowsToKanban
// are implemented with real logic; the rest are stubs.
type mockServeSM struct {
	listWorkflowsFn       func(ctx context.Context) ([]core.WorkflowSummary, error)
	loadByIDFn            func(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)
	saveFn                func(ctx context.Context, state *core.WorkflowState) error
	clearWorkflowRunFn    func(ctx context.Context, id core.WorkflowID) error
	savedStates           []*core.WorkflowState
	clearedWorkflowRunIDs []core.WorkflowID
}

func (m *mockServeSM) Save(ctx context.Context, state *core.WorkflowState) error {
	m.savedStates = append(m.savedStates, state)
	if m.saveFn != nil {
		return m.saveFn(ctx, state)
	}
	return nil
}
func (m *mockServeSM) Load(context.Context) (*core.WorkflowState, error) { return nil, nil }
func (m *mockServeSM) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	if m.loadByIDFn != nil {
		return m.loadByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockServeSM) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	if m.listWorkflowsFn != nil {
		return m.listWorkflowsFn(ctx)
	}
	return nil, nil
}
func (m *mockServeSM) GetActiveWorkflowID(context.Context) (core.WorkflowID, error) { return "", nil }
func (m *mockServeSM) SetActiveWorkflowID(context.Context, core.WorkflowID) error   { return nil }
func (m *mockServeSM) AcquireLock(context.Context) error                            { return nil }
func (m *mockServeSM) ReleaseLock(context.Context) error                            { return nil }
func (m *mockServeSM) AcquireWorkflowLock(context.Context, core.WorkflowID) error   { return nil }
func (m *mockServeSM) ReleaseWorkflowLock(context.Context, core.WorkflowID) error   { return nil }
func (m *mockServeSM) RefreshWorkflowLock(context.Context, core.WorkflowID) error   { return nil }
func (m *mockServeSM) SetWorkflowRunning(context.Context, core.WorkflowID) error    { return nil }
func (m *mockServeSM) ClearWorkflowRunning(ctx context.Context, id core.WorkflowID) error {
	m.clearedWorkflowRunIDs = append(m.clearedWorkflowRunIDs, id)
	if m.clearWorkflowRunFn != nil {
		return m.clearWorkflowRunFn(ctx, id)
	}
	return nil
}
func (m *mockServeSM) ListRunningWorkflows(context.Context) ([]core.WorkflowID, error) {
	return nil, nil
}
func (m *mockServeSM) IsWorkflowRunning(context.Context, core.WorkflowID) (bool, error) {
	return false, nil
}
func (m *mockServeSM) UpdateWorkflowHeartbeat(context.Context, core.WorkflowID) error { return nil }
func (m *mockServeSM) Exists() bool                                                   { return true }
func (m *mockServeSM) Backup(context.Context) error                                   { return nil }
func (m *mockServeSM) Restore(context.Context) (*core.WorkflowState, error)           { return nil, nil }
func (m *mockServeSM) DeactivateWorkflow(context.Context) error                       { return nil }
func (m *mockServeSM) ArchiveWorkflows(context.Context) (int, error)                  { return 0, nil }
func (m *mockServeSM) PurgeAllWorkflows(context.Context) (int, error)                 { return 0, nil }
func (m *mockServeSM) DeleteWorkflow(context.Context, core.WorkflowID) error          { return nil }
func (m *mockServeSM) UpdateHeartbeat(context.Context, core.WorkflowID) error         { return nil }
func (m *mockServeSM) FindZombieWorkflows(context.Context, time.Duration) ([]*core.WorkflowState, error) {
	return nil, nil
}
func (m *mockServeSM) FindWorkflowsByPrompt(context.Context, string) ([]core.DuplicateWorkflowInfo, error) {
	return nil, nil
}
func (m *mockServeSM) ExecuteAtomically(_ context.Context, fn func(core.AtomicStateContext) error) error {
	return fn(nil)
}

// Verify mockServeSM implements core.StateManager.
var _ core.StateManager = (*mockServeSM)(nil)

// mockServeKanbanSM extends mockServeSM with KanbanStateManager support.
type mockServeKanbanSM struct {
	mockServeSM
	moveWorkflowFn func(ctx context.Context, workflowID, toColumn string, position int) error
	movedWorkflows []movedWorkflow
}

type movedWorkflow struct {
	WorkflowID string
	ToColumn   string
	Position   int
}

func (m *mockServeKanbanSM) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	return m.mockServeSM.LoadByID(ctx, id)
}

func (m *mockServeKanbanSM) GetNextKanbanWorkflow(context.Context) (*core.WorkflowState, error) {
	return nil, nil
}

func (m *mockServeKanbanSM) MoveWorkflow(ctx context.Context, workflowID, toColumn string, position int) error {
	m.movedWorkflows = append(m.movedWorkflows, movedWorkflow{workflowID, toColumn, position})
	if m.moveWorkflowFn != nil {
		return m.moveWorkflowFn(ctx, workflowID, toColumn, position)
	}
	return nil
}

func (m *mockServeKanbanSM) UpdateKanbanStatus(context.Context, string, string, string, int, string) error {
	return nil
}

func (m *mockServeKanbanSM) GetKanbanEngineState(context.Context) (*kanban.KanbanEngineState, error) {
	return nil, nil
}

func (m *mockServeKanbanSM) SaveKanbanEngineState(context.Context, *kanban.KanbanEngineState) error {
	return nil
}

// Verify interface implementation.
var _ core.StateManager = (*mockServeKanbanSM)(nil)
var _ kanban.KanbanStateManager = (*mockServeKanbanSM)(nil)

// =============================================================================
// Helper
// =============================================================================

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func makeWorkflowState(id string, status core.WorkflowStatus, phase core.Phase) *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: core.WorkflowID(id),
			Prompt:     "test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       status,
			CurrentPhase: phase,
			Tasks:        map[core.TaskID]*core.TaskState{},
			Checkpoints:  []core.Checkpoint{},
			UpdatedAt:    time.Now(),
		},
	}
}

// =============================================================================
// recoverZombieWorkflows tests
// =============================================================================

func TestRecoverZombieWorkflows_EmptyList(t *testing.T) {
	t.Parallel()
	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return nil, nil
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("expected 0 recovered, got %d", recovered)
	}
}

func TestRecoverZombieWorkflows_ListError(t *testing.T) {
	t.Parallel()
	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return nil, fmt.Errorf("db connection failed")
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if recovered != 0 {
		t.Fatalf("expected 0 recovered, got %d", recovered)
	}
}

func TestRecoverZombieWorkflows_SkipNonRunning(t *testing.T) {
	t.Parallel()
	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-1", Status: core.WorkflowStatusCompleted},
				{WorkflowID: "wf-2", Status: core.WorkflowStatusFailed},
				{WorkflowID: "wf-3", Status: core.WorkflowStatusPending},
			}, nil
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("expected 0 recovered, got %d", recovered)
	}
	if len(sm.savedStates) != 0 {
		t.Fatalf("expected no saves, got %d", len(sm.savedStates))
	}
}

func TestRecoverZombieWorkflows_LoadByIDReturnsNil(t *testing.T) {
	t.Parallel()
	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-1", Status: core.WorkflowStatusRunning},
			}, nil
		},
		loadByIDFn: func(_ context.Context, _ core.WorkflowID) (*core.WorkflowState, error) {
			return nil, nil
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("expected 0 recovered, got %d", recovered)
	}
}

func TestRecoverZombieWorkflows_LoadByIDReturnsError(t *testing.T) {
	t.Parallel()
	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-1", Status: core.WorkflowStatusRunning},
			}, nil
		},
		loadByIDFn: func(_ context.Context, _ core.WorkflowID) (*core.WorkflowState, error) {
			return nil, fmt.Errorf("corrupt state")
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 0 {
		t.Fatalf("expected 0 recovered (error logged, continue), got %d", recovered)
	}
}

func TestRecoverZombieWorkflows_SuccessfulRecovery(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-recover", core.WorkflowStatusRunning, core.PhaseExecute)
	ws.Checkpoints = []core.Checkpoint{
		{ID: "cp-1", Type: "phase", Phase: core.PhaseAnalyze},
	}
	ws.Tasks = map[core.TaskID]*core.TaskState{
		"task-1": {ID: "task-1", Phase: core.PhaseExecute, Status: "running"},
	}

	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-recover", Status: core.WorkflowStatusRunning},
			}, nil
		},
		loadByIDFn: func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
			if id == "wf-recover" {
				return ws, nil
			}
			return nil, nil
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered, got %d", recovered)
	}

	// Verify saved state
	if len(sm.savedStates) != 1 {
		t.Fatalf("expected 1 save, got %d", len(sm.savedStates))
	}
	saved := sm.savedStates[0]
	if saved.Status != core.WorkflowStatusFailed {
		t.Errorf("expected status failed, got %s", saved.Status)
	}
	if saved.Error == "" {
		t.Error("expected error message to be set")
	}
	// Should have original checkpoint + recovery checkpoint
	if len(saved.Checkpoints) != 2 {
		t.Errorf("expected 2 checkpoints (1 original + 1 recovery), got %d", len(saved.Checkpoints))
	}
	lastCP := saved.Checkpoints[len(saved.Checkpoints)-1]
	if lastCP.Type != "recovery" {
		t.Errorf("expected last checkpoint type 'recovery', got '%s'", lastCP.Type)
	}
	if lastCP.Phase != core.PhaseExecute {
		t.Errorf("expected checkpoint phase 'execute', got '%s'", lastCP.Phase)
	}

	// Verify ClearWorkflowRunning was called (mockServeSM implements it)
	if len(sm.clearedWorkflowRunIDs) != 1 {
		t.Fatalf("expected 1 ClearWorkflowRunning call, got %d", len(sm.clearedWorkflowRunIDs))
	}
	if sm.clearedWorkflowRunIDs[0] != "wf-recover" {
		t.Errorf("expected cleared ID 'wf-recover', got '%s'", sm.clearedWorkflowRunIDs[0])
	}
}

func TestRecoverZombieWorkflows_SaveError(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-save-err", core.WorkflowStatusRunning, core.PhaseAnalyze)

	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-save-err", Status: core.WorkflowStatusRunning},
			}, nil
		},
		loadByIDFn: func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
			if id == "wf-save-err" {
				return ws, nil
			}
			return nil, nil
		},
		saveFn: func(_ context.Context, _ *core.WorkflowState) error {
			return fmt.Errorf("disk full")
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Save failed, so not counted as recovered
	if recovered != 0 {
		t.Fatalf("expected 0 recovered (save failed), got %d", recovered)
	}
}

func TestRecoverZombieWorkflows_MultipleWorkflowsMixed(t *testing.T) {
	t.Parallel()
	wsRunning1 := makeWorkflowState("wf-r1", core.WorkflowStatusRunning, core.PhaseExecute)
	wsRunning2 := makeWorkflowState("wf-r2", core.WorkflowStatusRunning, core.PhasePlan)
	// wf-done is completed, should be skipped.

	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-r1", Status: core.WorkflowStatusRunning},
				{WorkflowID: "wf-done", Status: core.WorkflowStatusCompleted},
				{WorkflowID: "wf-r2", Status: core.WorkflowStatusRunning},
				{WorkflowID: "wf-pending", Status: core.WorkflowStatusPending},
			}, nil
		},
		loadByIDFn: func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
			switch id {
			case "wf-r1":
				return wsRunning1, nil
			case "wf-r2":
				return wsRunning2, nil
			default:
				return nil, nil
			}
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recovered != 2 {
		t.Fatalf("expected 2 recovered, got %d", recovered)
	}
	if len(sm.savedStates) != 2 {
		t.Fatalf("expected 2 saves, got %d", len(sm.savedStates))
	}
}

func TestRecoverZombieWorkflows_ClearWorkflowRunningError(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-clear-err", core.WorkflowStatusRunning, core.PhaseRefine)

	sm := &mockServeSM{
		listWorkflowsFn: func(context.Context) ([]core.WorkflowSummary, error) {
			return []core.WorkflowSummary{
				{WorkflowID: "wf-clear-err", Status: core.WorkflowStatusRunning},
			}, nil
		},
		loadByIDFn: func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
			if id == "wf-clear-err" {
				return ws, nil
			}
			return nil, nil
		},
		clearWorkflowRunFn: func(_ context.Context, _ core.WorkflowID) error {
			return fmt.Errorf("clear failed")
		},
	}

	recovered, err := recoverZombieWorkflows(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The workflow is still counted as recovered even if ClearWorkflowRunning fails
	// (it logs a warning but continues)
	if recovered != 1 {
		t.Fatalf("expected 1 recovered, got %d", recovered)
	}
}

// =============================================================================
// migrateWorkflowsToKanban tests
// =============================================================================

func TestMigrateWorkflowsToKanban_NonKanbanSM(t *testing.T) {
	t.Parallel()
	// Use mockServeSM which does NOT implement KanbanStateManager.
	sm := &mockServeSM{}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("expected 0 migrated (non-kanban SM), got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_EmptyList(t *testing.T) {
	t.Parallel()
	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("expected 0, got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_ListError(t *testing.T) {
	t.Parallel()
	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return nil, fmt.Errorf("list failed")
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if migrated != 0 {
		t.Fatalf("expected 0, got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_AlreadyHasColumn(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-has-col", core.WorkflowStatusCompleted, core.PhaseDone)
	ws.KanbanColumn = "done"

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-has-col", Status: core.WorkflowStatusCompleted},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-has-col" {
			return ws, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("expected 0 migrated (already has column), got %d", migrated)
	}
	if len(sm.movedWorkflows) != 0 {
		t.Fatalf("expected no moves, got %d", len(sm.movedWorkflows))
	}
}

func TestMigrateWorkflowsToKanban_LoadByIDReturnsNil(t *testing.T) {
	t.Parallel()
	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-nil", Status: core.WorkflowStatusPending},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, _ core.WorkflowID) (*core.WorkflowState, error) {
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("expected 0, got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_LoadByIDReturnsError(t *testing.T) {
	t.Parallel()
	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-err", Status: core.WorkflowStatusRunning},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, _ core.WorkflowID) (*core.WorkflowState, error) {
		return nil, fmt.Errorf("load error")
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 0 {
		t.Fatalf("expected 0 (load error, continue), got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_CompletedToDone(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-completed", core.WorkflowStatusCompleted, core.PhaseDone)

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-completed", Status: core.WorkflowStatusCompleted},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-completed" {
			return ws, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("expected 1 migrated, got %d", migrated)
	}
	if len(sm.movedWorkflows) != 1 {
		t.Fatalf("expected 1 move, got %d", len(sm.movedWorkflows))
	}
	if sm.movedWorkflows[0].ToColumn != "done" {
		t.Errorf("expected column 'done', got '%s'", sm.movedWorkflows[0].ToColumn)
	}
	if sm.movedWorkflows[0].WorkflowID != "wf-completed" {
		t.Errorf("expected workflow ID 'wf-completed', got '%s'", sm.movedWorkflows[0].WorkflowID)
	}
}

func TestMigrateWorkflowsToKanban_RunningToInProgress(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-running", core.WorkflowStatusRunning, core.PhaseExecute)

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-running", Status: core.WorkflowStatusRunning},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-running" {
			return ws, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("expected 1 migrated, got %d", migrated)
	}
	if sm.movedWorkflows[0].ToColumn != "in_progress" {
		t.Errorf("expected column 'in_progress', got '%s'", sm.movedWorkflows[0].ToColumn)
	}
}

func TestMigrateWorkflowsToKanban_FailedToRefinement(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-failed", core.WorkflowStatusFailed, core.PhaseExecute)

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-failed", Status: core.WorkflowStatusFailed},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-failed" {
			return ws, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("expected 1 migrated, got %d", migrated)
	}
	if sm.movedWorkflows[0].ToColumn != "refinement" {
		t.Errorf("expected column 'refinement', got '%s'", sm.movedWorkflows[0].ToColumn)
	}
}

func TestMigrateWorkflowsToKanban_PendingToRefinement(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-pending", core.WorkflowStatusPending, core.PhaseRefine)

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-pending", Status: core.WorkflowStatusPending},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-pending" {
			return ws, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 1 {
		t.Fatalf("expected 1 migrated, got %d", migrated)
	}
	if sm.movedWorkflows[0].ToColumn != "refinement" {
		t.Errorf("expected column 'refinement' for pending status, got '%s'", sm.movedWorkflows[0].ToColumn)
	}
}

func TestMigrateWorkflowsToKanban_MoveWorkflowError(t *testing.T) {
	t.Parallel()
	ws := makeWorkflowState("wf-move-err", core.WorkflowStatusCompleted, core.PhaseDone)

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-move-err", Status: core.WorkflowStatusCompleted},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		if id == "wf-move-err" {
			return ws, nil
		}
		return nil, nil
	}
	sm.moveWorkflowFn = func(context.Context, string, string, int) error {
		return fmt.Errorf("move failed")
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Move failed, so not counted as migrated
	if migrated != 0 {
		t.Fatalf("expected 0 migrated (move failed), got %d", migrated)
	}
}

func TestMigrateWorkflowsToKanban_MultipleMixed(t *testing.T) {
	t.Parallel()
	wsCompleted := makeWorkflowState("wf-c", core.WorkflowStatusCompleted, core.PhaseDone)
	wsFailed := makeWorkflowState("wf-f", core.WorkflowStatusFailed, core.PhaseExecute)
	wsRunning := makeWorkflowState("wf-r", core.WorkflowStatusRunning, core.PhaseAnalyze)
	wsWithColumn := makeWorkflowState("wf-col", core.WorkflowStatusCompleted, core.PhaseDone)
	wsWithColumn.KanbanColumn = "done"

	sm := &mockServeKanbanSM{}
	sm.listWorkflowsFn = func(context.Context) ([]core.WorkflowSummary, error) {
		return []core.WorkflowSummary{
			{WorkflowID: "wf-c", Status: core.WorkflowStatusCompleted},
			{WorkflowID: "wf-f", Status: core.WorkflowStatusFailed},
			{WorkflowID: "wf-r", Status: core.WorkflowStatusRunning},
			{WorkflowID: "wf-col", Status: core.WorkflowStatusCompleted},
		}, nil
	}
	sm.loadByIDFn = func(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
		switch id {
		case "wf-c":
			return wsCompleted, nil
		case "wf-f":
			return wsFailed, nil
		case "wf-r":
			return wsRunning, nil
		case "wf-col":
			return wsWithColumn, nil
		}
		return nil, nil
	}

	migrated, err := migrateWorkflowsToKanban(context.Background(), sm, discardLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrated != 3 {
		t.Fatalf("expected 3 migrated (wf-col skipped), got %d", migrated)
	}
	if len(sm.movedWorkflows) != 3 {
		t.Fatalf("expected 3 moves, got %d", len(sm.movedWorkflows))
	}

	// Verify columns: wf-c → done, wf-f → refinement, wf-r → in_progress
	columnByID := make(map[string]string)
	for _, mw := range sm.movedWorkflows {
		columnByID[mw.WorkflowID] = mw.ToColumn
	}
	if columnByID["wf-c"] != "done" {
		t.Errorf("expected wf-c → done, got %s", columnByID["wf-c"])
	}
	if columnByID["wf-f"] != "refinement" {
		t.Errorf("expected wf-f → refinement, got %s", columnByID["wf-f"])
	}
	if columnByID["wf-r"] != "in_progress" {
		t.Errorf("expected wf-r → in_progress, got %s", columnByID["wf-r"])
	}
}

// =============================================================================
// buildHeartbeatConfig tests (additional coverage)
// =============================================================================

func TestBuildHeartbeatConfig_EmptyReturnsDefaults(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{}
	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()

	if result.Interval != defaults.Interval {
		t.Errorf("expected default interval %v, got %v", defaults.Interval, result.Interval)
	}
	if result.StaleThreshold != defaults.StaleThreshold {
		t.Errorf("expected default stale threshold %v, got %v", defaults.StaleThreshold, result.StaleThreshold)
	}
	if result.CheckInterval != defaults.CheckInterval {
		t.Errorf("expected default check interval %v, got %v", defaults.CheckInterval, result.CheckInterval)
	}
	if result.AutoResume != false {
		t.Error("expected AutoResume=false")
	}
	if result.MaxResumes != defaults.MaxResumes {
		t.Errorf("expected default MaxResumes %d, got %d", defaults.MaxResumes, result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_ValidInterval(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{Interval: "5s"}
	result := buildHeartbeatConfig(cfg)
	if result.Interval != 5*time.Second {
		t.Errorf("expected 5s, got %v", result.Interval)
	}
}

func TestBuildHeartbeatConfig_InvalidIntervalFallsBack(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{Interval: "bad"}
	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	if result.Interval != defaults.Interval {
		t.Errorf("expected default interval for invalid, got %v", result.Interval)
	}
}

func TestBuildHeartbeatConfig_ValidStaleThreshold(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{StaleThreshold: "3m"}
	result := buildHeartbeatConfig(cfg)
	if result.StaleThreshold != 3*time.Minute {
		t.Errorf("expected 3m, got %v", result.StaleThreshold)
	}
}

func TestBuildHeartbeatConfig_ValidCheckInterval(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{CheckInterval: "45s"}
	result := buildHeartbeatConfig(cfg)
	if result.CheckInterval != 45*time.Second {
		t.Errorf("expected 45s, got %v", result.CheckInterval)
	}
}

func TestBuildHeartbeatConfig_AutoResumeTrue(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{AutoResume: true}
	result := buildHeartbeatConfig(cfg)
	if !result.AutoResume {
		t.Error("expected AutoResume=true")
	}
}

func TestBuildHeartbeatConfig_MaxResumesPositive(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{MaxResumes: 7}
	result := buildHeartbeatConfig(cfg)
	if result.MaxResumes != 7 {
		t.Errorf("expected MaxResumes=7, got %d", result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_MaxResumesZeroUsesDefault(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{MaxResumes: 0}
	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	if result.MaxResumes != defaults.MaxResumes {
		t.Errorf("expected default MaxResumes %d, got %d", defaults.MaxResumes, result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_NegativeMaxResumesUsesDefault(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{MaxResumes: -1}
	result := buildHeartbeatConfig(cfg)
	defaults := workflow.DefaultHeartbeatConfig()
	if result.MaxResumes != defaults.MaxResumes {
		t.Errorf("expected default MaxResumes %d, got %d", defaults.MaxResumes, result.MaxResumes)
	}
}

func TestBuildHeartbeatConfig_AllFieldsCustom(t *testing.T) {
	t.Parallel()
	cfg := config.HeartbeatConfig{
		Interval:       "20s",
		StaleThreshold: "5m",
		CheckInterval:  "90s",
		AutoResume:     true,
		MaxResumes:     10,
	}
	result := buildHeartbeatConfig(cfg)
	if result.Interval != 20*time.Second {
		t.Errorf("expected 20s, got %v", result.Interval)
	}
	if result.StaleThreshold != 5*time.Minute {
		t.Errorf("expected 5m, got %v", result.StaleThreshold)
	}
	if result.CheckInterval != 90*time.Second {
		t.Errorf("expected 90s, got %v", result.CheckInterval)
	}
	if !result.AutoResume {
		t.Error("expected AutoResume=true")
	}
	if result.MaxResumes != 10 {
		t.Errorf("expected 10, got %d", result.MaxResumes)
	}
}
