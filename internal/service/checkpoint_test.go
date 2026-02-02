package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockStateManager implements core.StateManager for testing.
type mockStateManager struct {
	mu        sync.Mutex
	state     *core.WorkflowState
	saveError error
}

func (m *mockStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.state = state
	return nil
}

func (m *mockStateManager) Load(ctx context.Context) (*core.WorkflowState, error) {
	return m.state, nil
}

func (m *mockStateManager) AcquireLock(ctx context.Context) error { return nil }
func (m *mockStateManager) ReleaseLock(ctx context.Context) error { return nil }
func (m *mockStateManager) Exists() bool                          { return m.state != nil }
func (m *mockStateManager) Backup(ctx context.Context) error      { return nil }
func (m *mockStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
	return m.state, nil
}

func (m *mockStateManager) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	if m.state != nil && m.state.WorkflowID == id {
		return m.state, nil
	}
	return nil, nil
}

func (m *mockStateManager) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	if m.state == nil {
		return nil, nil
	}
	return []core.WorkflowSummary{{
		WorkflowID:   m.state.WorkflowID,
		Status:       m.state.Status,
		CurrentPhase: m.state.CurrentPhase,
		Prompt:       m.state.Prompt,
		CreatedAt:    m.state.CreatedAt,
		UpdatedAt:    m.state.UpdatedAt,
		IsActive:     true,
	}}, nil
}

func (m *mockStateManager) GetActiveWorkflowID(ctx context.Context) (core.WorkflowID, error) {
	if m.state != nil {
		return m.state.WorkflowID, nil
	}
	return "", nil
}

func (m *mockStateManager) SetActiveWorkflowID(ctx context.Context, id core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) DeactivateWorkflow(ctx context.Context) error {
	return nil
}

func (m *mockStateManager) ArchiveWorkflows(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockStateManager) PurgeAllWorkflows(ctx context.Context) (int, error) {
	if m.state != nil {
		m.state = nil
		return 1, nil
	}
	return 0, nil
}

func (m *mockStateManager) DeleteWorkflow(_ context.Context, id core.WorkflowID) error {
	if m.state != nil && m.state.WorkflowID == id {
		m.state = nil
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) UpdateHeartbeat(_ context.Context, id core.WorkflowID) error {
	if m.state != nil && m.state.WorkflowID == id {
		now := time.Now().UTC()
		m.state.HeartbeatAt = &now
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) FindZombieWorkflows(_ context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	if m.state == nil || m.state.Status != core.WorkflowStatusRunning {
		return nil, nil
	}
	cutoff := time.Now().UTC().Add(-staleThreshold)
	if m.state.HeartbeatAt == nil || m.state.HeartbeatAt.Before(cutoff) {
		return []*core.WorkflowState{m.state}, nil
	}
	return nil, nil
}

func (m *mockStateManager) AcquireWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ReleaseWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *mockStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	if m.state != nil && m.state.Status == core.WorkflowStatusRunning {
		return []core.WorkflowID{m.state.WorkflowID}, nil
	}
	return nil, nil
}

func (m *mockStateManager) IsWorkflowRunning(_ context.Context, id core.WorkflowID) (bool, error) {
	if m.state != nil && m.state.WorkflowID == id && m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
}

func (m *mockStateManager) UpdateWorkflowHeartbeat(_ context.Context, id core.WorkflowID) error {
	if m.state != nil && m.state.WorkflowID == id {
		now := time.Now().UTC()
		m.state.HeartbeatAt = &now
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *mockStateManager) FindWorkflowsByPrompt(_ context.Context, _ string) ([]core.DuplicateWorkflowInfo, error) {
	return nil, nil
}

func (m *mockStateManager) ExecuteAtomically(_ context.Context, fn func(core.AtomicStateContext) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomicCtx := &mockAtomicContext{m: m}
	return fn(atomicCtx)
}

type mockAtomicContext struct {
	m *mockStateManager
}

func (a *mockAtomicContext) LoadByID(id core.WorkflowID) (*core.WorkflowState, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id {
		return a.m.state, nil
	}
	return nil, nil
}

func (a *mockAtomicContext) Save(state *core.WorkflowState) error {
	a.m.state = state
	return nil
}

func (a *mockAtomicContext) SetWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicContext) ClearWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicContext) IsWorkflowRunning(id core.WorkflowID) (bool, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id && a.m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
}

func newTestWorkflowState() *core.WorkflowState {
	return &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Test Task 1",
				Status: core.TaskStatusPending,
			},
			"task-2": {
				ID:     "task-2",
				Name:   "Test Task 2",
				Status: core.TaskStatusCompleted,
			},
		},
		TaskOrder:   []core.TaskID{"task-1", "task-2"},
		Checkpoints: []core.Checkpoint{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestCheckpointManager_Create(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	ctx := context.Background()

	err := manager.CreateCheckpoint(ctx, state, CheckpointPhaseStart, map[string]interface{}{
		"phase": "analyze",
	})
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	if len(state.Checkpoints) != 1 {
		t.Errorf("len(Checkpoints) = %d, want 1", len(state.Checkpoints))
	}

	cp := state.Checkpoints[0]
	if cp.Type != string(CheckpointPhaseStart) {
		t.Errorf("Type = %s, want %s", cp.Type, CheckpointPhaseStart)
	}
	if cp.Phase != core.PhaseAnalyze {
		t.Errorf("Phase = %s, want %s", cp.Phase, core.PhaseAnalyze)
	}
	if cp.ID == "" {
		t.Error("ID should not be empty")
	}

	// Verify metadata
	var metadata map[string]interface{}
	if err := json.Unmarshal(cp.Data, &metadata); err != nil {
		t.Fatalf("Unmarshal metadata error = %v", err)
	}
	if metadata["phase"] != "analyze" {
		t.Errorf("metadata[phase] = %v, want 'analyze'", metadata["phase"])
	}
}

func TestCheckpointManager_PhaseCheckpoint(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	ctx := context.Background()

	// Phase start checkpoint
	err := manager.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, false)
	if err != nil {
		t.Fatalf("PhaseCheckpoint() error = %v", err)
	}

	cp := state.Checkpoints[0]
	if cp.Type != string(CheckpointPhaseStart) {
		t.Errorf("Type = %s, want %s", cp.Type, CheckpointPhaseStart)
	}

	// Phase complete checkpoint
	err = manager.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, true)
	if err != nil {
		t.Fatalf("PhaseCheckpoint() error = %v", err)
	}

	cp = state.Checkpoints[1]
	if cp.Type != string(CheckpointPhaseComplete) {
		t.Errorf("Type = %s, want %s", cp.Type, CheckpointPhaseComplete)
	}
}

func TestCheckpointManager_TaskCheckpoint(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	ctx := context.Background()

	task := &core.Task{
		ID:     "task-1",
		Name:   "Test Task",
		Status: core.TaskStatusRunning,
	}

	// Task start checkpoint
	err := manager.TaskCheckpoint(ctx, state, task, false)
	if err != nil {
		t.Fatalf("TaskCheckpoint() error = %v", err)
	}

	cp := state.Checkpoints[0]
	if cp.Type != string(CheckpointTaskStart) {
		t.Errorf("Type = %s, want %s", cp.Type, CheckpointTaskStart)
	}

	var metadata map[string]interface{}
	json.Unmarshal(cp.Data, &metadata)
	if metadata["task_id"] != "task-1" {
		t.Errorf("metadata[task_id] = %v, want 'task-1'", metadata["task_id"])
	}
}

func TestCheckpointManager_ErrorCheckpoint(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	ctx := context.Background()

	testError := core.ErrExecution("TEST_ERROR", "something went wrong")

	err := manager.ErrorCheckpoint(ctx, state, testError)
	if err != nil {
		t.Fatalf("ErrorCheckpoint() error = %v", err)
	}

	cp := state.Checkpoints[0]
	if cp.Type != string(CheckpointError) {
		t.Errorf("Type = %s, want %s", cp.Type, CheckpointError)
	}

	var metadata map[string]interface{}
	json.Unmarshal(cp.Data, &metadata)
	if metadata["error"] == "" {
		t.Error("metadata[error] should not be empty")
	}
}

func TestCheckpointManager_GetLastCheckpoint(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()

	// No checkpoints
	cp := manager.GetLastCheckpoint(state)
	if cp != nil {
		t.Error("GetLastCheckpoint should return nil for empty checkpoints")
	}

	// Add checkpoints
	state.Checkpoints = []core.Checkpoint{
		{ID: "cp-1", Type: "phase_start"},
		{ID: "cp-2", Type: "task_start"},
		{ID: "cp-3", Type: "task_complete"},
	}

	cp = manager.GetLastCheckpoint(state)
	if cp == nil {
		t.Fatal("GetLastCheckpoint should not return nil")
	}
	if cp.ID != "cp-3" {
		t.Errorf("ID = %s, want cp-3", cp.ID)
	}
}

func TestCheckpointManager_GetLastCheckpointOfType(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	state.Checkpoints = []core.Checkpoint{
		{ID: "cp-1", Type: string(CheckpointPhaseStart)},
		{ID: "cp-2", Type: string(CheckpointTaskStart)},
		{ID: "cp-3", Type: string(CheckpointPhaseComplete)},
		{ID: "cp-4", Type: string(CheckpointTaskStart)},
	}

	cp := manager.GetLastCheckpointOfType(state, CheckpointTaskStart)
	if cp == nil {
		t.Fatal("GetLastCheckpointOfType should not return nil")
	}
	if cp.ID != "cp-4" {
		t.Errorf("ID = %s, want cp-4", cp.ID)
	}

	// Non-existent type
	cp = manager.GetLastCheckpointOfType(state, CheckpointConsensus)
	if cp != nil {
		t.Error("GetLastCheckpointOfType should return nil for non-existent type")
	}
}

func TestCheckpointManager_GetResumePoint(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	ctx := context.Background()

	t.Run("no checkpoints", func(t *testing.T) {
		state := newTestWorkflowState()
		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if !resumePoint.FromStart {
			t.Error("FromStart should be true for no checkpoints")
		}
		if resumePoint.Phase != core.PhaseAnalyze {
			t.Errorf("Phase = %s, want %s", resumePoint.Phase, core.PhaseAnalyze)
		}
	})

	t.Run("phase start checkpoint", func(t *testing.T) {
		state := newTestWorkflowState()
		manager.PhaseCheckpoint(ctx, state, core.PhasePlan, false)

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if !resumePoint.RestartPhase {
			t.Error("RestartPhase should be true for phase_start checkpoint")
		}
	})

	t.Run("task start checkpoint", func(t *testing.T) {
		state := newTestWorkflowState()
		task := &core.Task{ID: "task-1", Name: "Test Task", Status: core.TaskStatusRunning}
		manager.TaskCheckpoint(ctx, state, task, false)

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if !resumePoint.RestartTask {
			t.Error("RestartTask should be true for task_start checkpoint")
		}
		if resumePoint.TaskID != "task-1" {
			t.Errorf("TaskID = %s, want task-1", resumePoint.TaskID)
		}
	})

	t.Run("error checkpoint", func(t *testing.T) {
		state := newTestWorkflowState()
		manager.ErrorCheckpoint(ctx, state, core.ErrExecution("TEST", "error"))

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if !resumePoint.AfterError {
			t.Error("AfterError should be true for error checkpoint")
		}
	})

	t.Run("phase complete advances to next phase", func(t *testing.T) {
		state := newTestWorkflowState()
		state.CurrentPhase = core.PhaseAnalyze
		// Create a phase_complete checkpoint for analyze
		manager.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, true)

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		// Should advance to PhasePlan (next phase after Analyze)
		if resumePoint.Phase != core.PhasePlan {
			t.Errorf("Phase = %s, want %s (should advance after phase_complete)", resumePoint.Phase, core.PhasePlan)
		}
		if resumePoint.RestartPhase {
			t.Error("RestartPhase should be false for phase_complete")
		}
	})

	t.Run("phase complete refine advances to analyze", func(t *testing.T) {
		state := newTestWorkflowState()
		state.CurrentPhase = core.PhaseRefine
		manager.PhaseCheckpoint(ctx, state, core.PhaseRefine, true)

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if resumePoint.Phase != core.PhaseAnalyze {
			t.Errorf("Phase = %s, want %s", resumePoint.Phase, core.PhaseAnalyze)
		}
	})

	t.Run("phase complete plan advances to execute", func(t *testing.T) {
		state := newTestWorkflowState()
		state.CurrentPhase = core.PhasePlan
		manager.PhaseCheckpoint(ctx, state, core.PhasePlan, true)

		resumePoint, err := manager.GetResumePoint(state)
		if err != nil {
			t.Fatalf("GetResumePoint() error = %v", err)
		}
		if resumePoint.Phase != core.PhaseExecute {
			t.Errorf("Phase = %s, want %s", resumePoint.Phase, core.PhaseExecute)
		}
	})
}

func TestCheckpointManager_Cleanup(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()

	now := time.Now()
	state.Checkpoints = []core.Checkpoint{
		{ID: "cp-1", Timestamp: now.Add(-48 * time.Hour)}, // Old
		{ID: "cp-2", Timestamp: now.Add(-25 * time.Hour)}, // Old
		{ID: "cp-3", Timestamp: now.Add(-1 * time.Hour)},  // Recent
		{ID: "cp-4", Timestamp: now},                      // Current
	}

	cleaned := manager.CleanupOldCheckpoints(state, 24*time.Hour)

	if cleaned != 2 {
		t.Errorf("cleaned = %d, want 2", cleaned)
	}
	if len(state.Checkpoints) != 2 {
		t.Errorf("len(Checkpoints) = %d, want 2", len(state.Checkpoints))
	}
	if state.Checkpoints[0].ID != "cp-3" {
		t.Error("first remaining checkpoint should be cp-3")
	}
}

func TestCheckpointManager_GetProgress(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	state.Tasks = map[core.TaskID]*core.TaskState{
		"task-1": {ID: "task-1", Status: core.TaskStatusCompleted},
		"task-2": {ID: "task-2", Status: core.TaskStatusCompleted},
		"task-3": {ID: "task-3", Status: core.TaskStatusRunning},
		"task-4": {ID: "task-4", Status: core.TaskStatusFailed},
		"task-5": {ID: "task-5", Status: core.TaskStatusPending},
	}
	state.Checkpoints = []core.Checkpoint{
		{ID: "cp-1"},
		{ID: "cp-2"},
	}

	progress := manager.GetProgress(state)

	if progress.TotalTasks != 5 {
		t.Errorf("TotalTasks = %d, want 5", progress.TotalTasks)
	}
	if progress.CompletedTasks != 2 {
		t.Errorf("CompletedTasks = %d, want 2", progress.CompletedTasks)
	}
	if progress.RunningTasks != 1 {
		t.Errorf("RunningTasks = %d, want 1", progress.RunningTasks)
	}
	if progress.FailedTasks != 1 {
		t.Errorf("FailedTasks = %d, want 1", progress.FailedTasks)
	}
	if progress.Percentage != 40.0 {
		t.Errorf("Percentage = %v, want 40.0", progress.Percentage)
	}
	if progress.TotalCheckpoints != 2 {
		t.Errorf("TotalCheckpoints = %d, want 2", progress.TotalCheckpoints)
	}
}

func TestCheckpointManager_NoMetadata(t *testing.T) {
	stateManager := &mockStateManager{}
	logger := logging.NewNop()
	manager := NewCheckpointManager(stateManager, logger)

	state := newTestWorkflowState()
	ctx := context.Background()

	err := manager.CreateCheckpoint(ctx, state, CheckpointPhaseStart, nil)
	if err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}

	if len(state.Checkpoints[0].Data) != 0 {
		t.Error("Data should be empty when no metadata provided")
	}
}
