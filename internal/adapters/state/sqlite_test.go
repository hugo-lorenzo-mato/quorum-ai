package state

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func newTestStateSQLite() *core.WorkflowState {
	now := time.Now().Truncate(time.Second) // SQLite stores with second precision
	return &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-test-123",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test workflow prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:           "task-1",
				Phase:        core.PhaseAnalyze,
				Name:         "Test Task",
				Status:       core.TaskStatusPending,
				CLI:          "claude",
				Model:        "opus",
				Dependencies: []core.TaskID{},
				TokensIn:     100,
				TokensOut:    50,
				CostUSD:      0.01,
			},
		},
		TaskOrder: []core.TaskID{"task-1"},
		Config: &core.WorkflowConfig{
			ConsensusThreshold: 0.75,
			MaxRetries:         3,
			Timeout:            time.Hour,
		},
		Metrics: &core.StateMetrics{
			TotalCostUSD:   0.05,
			TotalTokensIn:  1000,
			TotalTokensOut: 500,
		},
		Checkpoints: []core.Checkpoint{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestSQLiteStateManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	state := newTestStateSQLite()
	ctx := context.Background()

	err = manager.Save(ctx, state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify database exists
	if !manager.Exists() {
		t.Error("database file should exist after save")
	}
}

func TestSQLiteStateManager_Load(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	originalState := newTestStateSQLite()
	ctx := context.Background()

	// Save first
	if err := manager.Save(ctx, originalState); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loadedState, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loadedState == nil {
		t.Fatal("loadedState should not be nil")
	}

	if loadedState.WorkflowID != originalState.WorkflowID {
		t.Errorf("WorkflowID = %s, want %s", loadedState.WorkflowID, originalState.WorkflowID)
	}
	if loadedState.Status != originalState.Status {
		t.Errorf("Status = %s, want %s", loadedState.Status, originalState.Status)
	}
	if loadedState.Prompt != originalState.Prompt {
		t.Errorf("Prompt = %s, want %s", loadedState.Prompt, originalState.Prompt)
	}
	if loadedState.CurrentPhase != originalState.CurrentPhase {
		t.Errorf("CurrentPhase = %s, want %s", loadedState.CurrentPhase, originalState.CurrentPhase)
	}
}

func TestSQLiteStateManager_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	state, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for non-existent state", err)
	}
	if state != nil {
		t.Error("state should be nil for non-existent workflow")
	}
}

func TestSQLiteStateManager_LoadByID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create and save two workflows
	state1 := newTestStateSQLite()
	state1.WorkflowID = "wf-1"
	state1.Prompt = "First workflow"

	state2 := newTestStateSQLite()
	state2.WorkflowID = "wf-2"
	state2.Prompt = "Second workflow"

	if err := manager.Save(ctx, state1); err != nil {
		t.Fatalf("Save(state1) error = %v", err)
	}
	if err := manager.Save(ctx, state2); err != nil {
		t.Fatalf("Save(state2) error = %v", err)
	}

	// Load by specific ID
	loaded1, err := manager.LoadByID(ctx, "wf-1")
	if err != nil {
		t.Fatalf("LoadByID(wf-1) error = %v", err)
	}
	if loaded1.Prompt != "First workflow" {
		t.Errorf("Prompt = %s, want 'First workflow'", loaded1.Prompt)
	}

	loaded2, err := manager.LoadByID(ctx, "wf-2")
	if err != nil {
		t.Fatalf("LoadByID(wf-2) error = %v", err)
	}
	if loaded2.Prompt != "Second workflow" {
		t.Errorf("Prompt = %s, want 'Second workflow'", loaded2.Prompt)
	}

	// Load non-existent
	loadedNone, err := manager.LoadByID(ctx, "wf-nonexistent")
	if err != nil {
		t.Fatalf("LoadByID(nonexistent) error = %v", err)
	}
	if loadedNone != nil {
		t.Error("should return nil for non-existent workflow")
	}
}

func TestSQLiteStateManager_ListWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Initially empty
	summaries, err := manager.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("len(summaries) = %d, want 0", len(summaries))
	}

	// Add workflows
	state1 := newTestStateSQLite()
	state1.WorkflowID = "wf-1"
	state1.Prompt = "First workflow"

	state2 := newTestStateSQLite()
	state2.WorkflowID = "wf-2"
	state2.Prompt = "Second workflow"

	if err := manager.Save(ctx, state1); err != nil {
		t.Fatalf("Save(state1) error = %v", err)
	}
	if err := manager.Save(ctx, state2); err != nil {
		t.Fatalf("Save(state2) error = %v", err)
	}

	summaries, err = manager.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("len(summaries) = %d, want 2", len(summaries))
	}

	// Last saved should be active
	var activeCount int
	for _, s := range summaries {
		if s.IsActive {
			activeCount++
			if s.WorkflowID != "wf-2" {
				t.Errorf("active workflow = %s, want wf-2", s.WorkflowID)
			}
		}
	}
	if activeCount != 1 {
		t.Errorf("activeCount = %d, want 1", activeCount)
	}
}

func TestSQLiteStateManager_ActiveWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Initially no active workflow
	activeID, err := manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "" {
		t.Errorf("activeID = %s, want empty", activeID)
	}

	// Save sets active
	state := newTestStateSQLite()
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	activeID, err = manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != state.WorkflowID {
		t.Errorf("activeID = %s, want %s", activeID, state.WorkflowID)
	}

	// Manual set
	err = manager.SetActiveWorkflowID(ctx, "wf-manual")
	if err != nil {
		t.Fatalf("SetActiveWorkflowID() error = %v", err)
	}

	activeID, err = manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "wf-manual" {
		t.Errorf("activeID = %s, want wf-manual", activeID)
	}
}

func TestSQLiteStateManager_Tasks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create state with multiple tasks
	state := newTestStateSQLite()
	state.Tasks = map[core.TaskID]*core.TaskState{
		"task-1": {
			ID:           "task-1",
			Phase:        core.PhaseAnalyze,
			Name:         "Analyze Task",
			Status:       core.TaskStatusCompleted,
			CLI:          "claude",
			Model:        "opus",
			Dependencies: []core.TaskID{},
			TokensIn:     100,
			TokensOut:    50,
			CostUSD:      0.01,
			Output:       "Analysis result",
			ToolCalls: []core.ToolCall{
				{ID: "tc-1", Name: "read_file", Arguments: map[string]interface{}{"path": "/test"}, Result: "content"},
			},
		},
		"task-2": {
			ID:           "task-2",
			Phase:        core.PhasePlan,
			Name:         "Plan Task",
			Status:       core.TaskStatusPending,
			CLI:          "gemini",
			Dependencies: []core.TaskID{"task-1"},
		},
	}
	state.TaskOrder = []core.TaskID{"task-1", "task-2"}

	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(loaded.Tasks))
	}

	// Verify task-1
	task1 := loaded.Tasks["task-1"]
	if task1 == nil {
		t.Fatal("task-1 not found")
	}
	if task1.Status != core.TaskStatusCompleted {
		t.Errorf("task-1 status = %s, want completed", task1.Status)
	}
	if task1.Output != "Analysis result" {
		t.Errorf("task-1 output = %s, want 'Analysis result'", task1.Output)
	}
	if len(task1.ToolCalls) != 1 {
		t.Errorf("task-1 tool calls count = %d, want 1", len(task1.ToolCalls))
	}

	// Verify task-2 dependencies
	task2 := loaded.Tasks["task-2"]
	if task2 == nil {
		t.Fatal("task-2 not found")
	}
	if len(task2.Dependencies) != 1 || task2.Dependencies[0] != "task-1" {
		t.Errorf("task-2 dependencies = %v, want [task-1]", task2.Dependencies)
	}
}

func TestSQLiteStateManager_Checkpoints(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	state := newTestStateSQLite()
	state.Checkpoints = []core.Checkpoint{
		{
			ID:        "cp-1",
			Type:      "phase_complete",
			Phase:     core.PhaseAnalyze,
			TaskID:    "task-1",
			Timestamp: time.Now().Truncate(time.Second),
			Message:   "Analysis phase completed",
			Data:      []byte(`{"result": "success"}`),
		},
	}

	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Checkpoints) != 1 {
		t.Fatalf("len(Checkpoints) = %d, want 1", len(loaded.Checkpoints))
	}

	cp := loaded.Checkpoints[0]
	if cp.ID != "cp-1" {
		t.Errorf("checkpoint ID = %s, want cp-1", cp.ID)
	}
	if cp.Type != "phase_complete" {
		t.Errorf("checkpoint type = %s, want phase_complete", cp.Type)
	}
	if cp.Message != "Analysis phase completed" {
		t.Errorf("checkpoint message = %s, want 'Analysis phase completed'", cp.Message)
	}
}

func TestSQLiteStateManager_BackupRestore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save original state
	state := newTestStateSQLite()
	state.Prompt = "Original prompt"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create backup
	if err := manager.Backup(ctx); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Modify state
	state.Prompt = "Modified prompt"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify modified
	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Prompt != "Modified prompt" {
		t.Errorf("Prompt = %s, want 'Modified prompt'", loaded.Prompt)
	}

	// Restore from backup
	restored, err := manager.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Prompt != "Original prompt" {
		t.Errorf("Restored Prompt = %s, want 'Original prompt'", restored.Prompt)
	}
}

func TestSQLiteStateManager_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save initial state
	state := newTestStateSQLite()
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Update state
	state.Status = core.WorkflowStatusCompleted
	state.CurrentPhase = core.PhaseExecute
	state.Tasks["task-1"].Status = core.TaskStatusCompleted
	now := time.Now().Truncate(time.Second)
	state.Tasks["task-1"].CompletedAt = &now

	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save(updated) error = %v", err)
	}

	// Load and verify updates
	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Status != core.WorkflowStatusCompleted {
		t.Errorf("Status = %s, want completed", loaded.Status)
	}
	if loaded.CurrentPhase != core.PhaseExecute {
		t.Errorf("CurrentPhase = %s, want execute", loaded.CurrentPhase)
	}
	if loaded.Tasks["task-1"].Status != core.TaskStatusCompleted {
		t.Errorf("Task status = %s, want completed", loaded.Tasks["task-1"].Status)
	}
}

func TestSQLiteStateManager_ConfigAndMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	state := newTestStateSQLite()
	state.Config = &core.WorkflowConfig{
		ConsensusThreshold: 0.85,
		MaxRetries:         5,
		Timeout:            2 * time.Hour,
		DryRun:             true,
		Sandbox:            true,
	}
	state.Metrics = &core.StateMetrics{
		TotalCostUSD:   1.25,
		TotalTokensIn:  10000,
		TotalTokensOut: 5000,
		ConsensusScore: 0.95,
		Duration:       30 * time.Minute,
	}

	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify config
	if loaded.Config == nil {
		t.Fatal("Config should not be nil")
	}
	if loaded.Config.ConsensusThreshold != 0.85 {
		t.Errorf("ConsensusThreshold = %f, want 0.85", loaded.Config.ConsensusThreshold)
	}
	if loaded.Config.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", loaded.Config.MaxRetries)
	}
	if !loaded.Config.DryRun {
		t.Error("DryRun should be true")
	}

	// Verify metrics
	if loaded.Metrics == nil {
		t.Fatal("Metrics should not be nil")
	}
	if loaded.Metrics.TotalCostUSD != 1.25 {
		t.Errorf("TotalCostUSD = %f, want 1.25", loaded.Metrics.TotalCostUSD)
	}
	if loaded.Metrics.ConsensusScore != 0.95 {
		t.Errorf("ConsensusScore = %f, want 0.95", loaded.Metrics.ConsensusScore)
	}
}

func TestSQLiteStateManager_DeactivateWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save a workflow (this also activates it)
	state := newTestStateSQLite()
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify workflow is active
	activeID, _ := manager.GetActiveWorkflowID(ctx)
	if activeID != state.WorkflowID {
		t.Errorf("Expected active workflow %s, got %s", state.WorkflowID, activeID)
	}

	// Deactivate
	if err := manager.DeactivateWorkflow(ctx); err != nil {
		t.Fatalf("DeactivateWorkflow() error = %v", err)
	}

	// Verify no active workflow
	activeID, _ = manager.GetActiveWorkflowID(ctx)
	if activeID != "" {
		t.Errorf("Expected no active workflow, got %s", activeID)
	}

	// Workflow data should still exist
	loaded, err := manager.LoadByID(ctx, state.WorkflowID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Error("Workflow data should still exist after deactivation")
	}
}

func TestSQLiteStateManager_Save_IsDurableWithoutReleaseLock(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath, WithSQLiteLockTTL(5*time.Second))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}

	ctx := context.Background()

	if err := manager.AcquireLock(ctx); err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	state := newTestStateSQLite()
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Simulate abrupt termination: close without ReleaseLock() commit semantics.
	// Save() must still be durable.
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	manager2, err := NewSQLiteStateManager(dbPath, WithSQLiteLockTTL(5*time.Second))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() (second) error = %v", err)
	}
	defer manager2.Close()

	loaded, err := manager2.LoadByID(ctx, state.WorkflowID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected workflow state to be durable even without ReleaseLock()")
	}

	// Lock should still be present and block acquisition while "fresh".
	if err := manager2.AcquireLock(ctx); err == nil {
		_ = manager2.ReleaseLock(ctx)
		t.Fatalf("AcquireLock() should fail while lock is fresh")
	}

	// Force the lock to become stale and ensure we can recover.
	lockPath := dbPath + ".lock"
	stale := lockInfo{
		PID:        os.Getpid(),
		Hostname:   "test",
		AcquiredAt: time.Now().Add(-10 * time.Second),
	}
	b, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("json.Marshal(lockInfo) error = %v", err)
	}
	if err := os.WriteFile(lockPath, b, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", lockPath, err)
	}

	if err := manager2.AcquireLock(ctx); err != nil {
		t.Fatalf("AcquireLock() error after stale lock: %v", err)
	}
	if err := manager2.ReleaseLock(ctx); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}
}

func TestSQLiteStateManager_ArchiveWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save a completed workflow
	state := newTestStateSQLite()
	state.Status = core.WorkflowStatusCompleted
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Deactivate so it can be archived
	if err := manager.DeactivateWorkflow(ctx); err != nil {
		t.Fatalf("DeactivateWorkflow() error = %v", err)
	}

	// Archive
	archived, err := manager.ArchiveWorkflows(ctx)
	if err != nil {
		t.Fatalf("ArchiveWorkflows() error = %v", err)
	}
	if archived != 1 {
		t.Errorf("Expected 1 archived workflow, got %d", archived)
	}

	// Workflow should no longer exist in DB
	loaded, err := manager.LoadByID(ctx, state.WorkflowID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded != nil {
		t.Error("Workflow should have been removed from the DB after archiving")
	}

	// But it should be preserved on disk in the archive directory
	archivePath := filepath.Join(tmpDir, "archive", string(state.WorkflowID)+".json")
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", archivePath, err)
	}
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("failed to decode archive envelope: %v", err)
	}
	if env.State == nil {
		t.Fatalf("expected archived workflow state at %s", archivePath)
	}
	if env.State.WorkflowID != state.WorkflowID {
		t.Errorf("archived WorkflowID = %s, want %s", env.State.WorkflowID, state.WorkflowID)
	}
}

func TestSQLiteStateManager_ArchiveWorkflows_SkipsRunning(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save a running workflow
	state := newTestStateSQLite()
	state.Status = core.WorkflowStatusRunning
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Deactivate
	if err := manager.DeactivateWorkflow(ctx); err != nil {
		t.Fatalf("DeactivateWorkflow() error = %v", err)
	}

	// Archive - should not archive running workflows
	archived, err := manager.ArchiveWorkflows(ctx)
	if err != nil {
		t.Fatalf("ArchiveWorkflows() error = %v", err)
	}
	if archived != 0 {
		t.Errorf("Expected 0 archived workflows (running should be skipped), got %d", archived)
	}
}

func TestSQLiteStateManager_PurgeAllWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Save two workflows
	state1 := newTestStateSQLite()
	state1.WorkflowID = "wf-1"
	if err := manager.Save(ctx, state1); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	state2 := newTestStateSQLite()
	state2.WorkflowID = "wf-2"
	if err := manager.Save(ctx, state2); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Purge all
	deleted, err := manager.PurgeAllWorkflows(ctx)
	if err != nil {
		t.Fatalf("PurgeAllWorkflows() error = %v", err)
	}
	if deleted != 2 {
		t.Errorf("Expected 2 deleted workflows, got %d", deleted)
	}

	// No active workflow
	activeID, _ := manager.GetActiveWorkflowID(ctx)
	if activeID != "" {
		t.Errorf("Expected no active workflow, got %s", activeID)
	}

	// List should be empty
	workflows, err := manager.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows() error = %v", err)
	}
	if len(workflows) != 0 {
		t.Errorf("Expected 0 workflows, got %d", len(workflows))
	}
}

func TestSQLiteStateManager_AcquireWorkflowLock_Success(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow first
	state := newTestStateSQLite()
	state.WorkflowID = "wf-lock-test-001"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Acquire lock
	if err := manager.AcquireWorkflowLock(ctx, "wf-lock-test-001"); err != nil {
		t.Fatalf("AcquireWorkflowLock() error = %v", err)
	}

	// Release lock
	if err := manager.ReleaseWorkflowLock(ctx, "wf-lock-test-001"); err != nil {
		t.Fatalf("ReleaseWorkflowLock() error = %v", err)
	}
}

func TestSQLiteStateManager_AcquireWorkflowLock_AlreadyHeld(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create and save workflow
	state := newTestStateSQLite()
	state.WorkflowID = "wf-lock-test-002"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Acquire lock
	if err := manager.AcquireWorkflowLock(ctx, "wf-lock-test-002"); err != nil {
		t.Fatalf("AcquireWorkflowLock() error = %v", err)
	}

	// Try to acquire again (same process - should fail)
	err = manager.AcquireWorkflowLock(ctx, "wf-lock-test-002")
	if err == nil {
		t.Error("Expected error when acquiring already held lock, got nil")
	} else if domainErr, ok := err.(*core.DomainError); !ok || domainErr.Code != "WORKFLOW_LOCK_HELD" {
		t.Errorf("Expected WORKFLOW_LOCK_HELD error, got %v", err)
	}

	// Cleanup
	_ = manager.ReleaseWorkflowLock(ctx, "wf-lock-test-002")
}

func TestSQLiteStateManager_MultipleWorkflowLocks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create two workflows
	for _, id := range []string{"wf-multi-001", "wf-multi-002"} {
		state := newTestStateSQLite()
		state.WorkflowID = core.WorkflowID(id)
		if err := manager.Save(ctx, state); err != nil {
			t.Fatalf("Save(%s) error = %v", id, err)
		}
	}

	// Lock both workflows (concurrent locks should work)
	if err := manager.AcquireWorkflowLock(ctx, "wf-multi-001"); err != nil {
		t.Fatalf("AcquireWorkflowLock(wf-multi-001) error = %v", err)
	}

	if err := manager.AcquireWorkflowLock(ctx, "wf-multi-002"); err != nil {
		t.Fatalf("AcquireWorkflowLock(wf-multi-002) error = %v", err)
	}

	// Release both
	if err := manager.ReleaseWorkflowLock(ctx, "wf-multi-001"); err != nil {
		t.Fatalf("ReleaseWorkflowLock(wf-multi-001) error = %v", err)
	}
	if err := manager.ReleaseWorkflowLock(ctx, "wf-multi-002"); err != nil {
		t.Fatalf("ReleaseWorkflowLock(wf-multi-002) error = %v", err)
	}
}

func TestSQLiteStateManager_RefreshWorkflowLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow
	state := newTestStateSQLite()
	state.WorkflowID = "wf-refresh-001"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Acquire lock
	if err := manager.AcquireWorkflowLock(ctx, "wf-refresh-001"); err != nil {
		t.Fatalf("AcquireWorkflowLock() error = %v", err)
	}

	// Refresh lock
	if err := manager.RefreshWorkflowLock(ctx, "wf-refresh-001"); err != nil {
		t.Fatalf("RefreshWorkflowLock() error = %v", err)
	}

	// Refresh for non-held lock should fail
	err = manager.RefreshWorkflowLock(ctx, "wf-non-existent")
	if err == nil {
		t.Error("Expected error when refreshing non-held lock, got nil")
	}

	// Cleanup
	_ = manager.ReleaseWorkflowLock(ctx, "wf-refresh-001")
}

func TestSQLiteStateManager_StaleLockDetection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath, WithSQLiteLockTTL(1*time.Second))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow
	state := newTestStateSQLite()
	state.WorkflowID = "wf-stale-001"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Insert a "stale" lock (expired) directly into the database
	now := time.Now().UTC()
	_, err = manager.db.ExecContext(ctx, `
		INSERT INTO workflow_locks (workflow_id, holder_pid, holder_host, acquired_at, expires_at)
		VALUES (?, 99999, 'dead-host', ?, ?)
	`, "wf-stale-001", now.Add(-2*time.Hour), now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("INSERT stale lock error = %v", err)
	}

	// Should be able to acquire lock (stale lock should be removed)
	if err := manager.AcquireWorkflowLock(ctx, "wf-stale-001"); err != nil {
		t.Fatalf("AcquireWorkflowLock() error = %v (should have acquired after stale lock cleanup)", err)
	}

	// Cleanup
	_ = manager.ReleaseWorkflowLock(ctx, "wf-stale-001")
}

func TestSQLiteStateManager_RunningWorkflowsTracking(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflows
	for _, id := range []string{"wf-run-001", "wf-run-002", "wf-run-003"} {
		state := newTestStateSQLite()
		state.WorkflowID = core.WorkflowID(id)
		if err := manager.Save(ctx, state); err != nil {
			t.Fatalf("Save(%s) error = %v", id, err)
		}
	}

	// Set some as running
	if err := manager.SetWorkflowRunning(ctx, "wf-run-001"); err != nil {
		t.Fatalf("SetWorkflowRunning(wf-run-001) error = %v", err)
	}
	if err := manager.SetWorkflowRunning(ctx, "wf-run-002"); err != nil {
		t.Fatalf("SetWorkflowRunning(wf-run-002) error = %v", err)
	}

	// List running
	running, err := manager.ListRunningWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListRunningWorkflows() error = %v", err)
	}
	if len(running) != 2 {
		t.Errorf("Expected 2 running workflows, got %d", len(running))
	}

	// Check individual
	isRunning, err := manager.IsWorkflowRunning(ctx, "wf-run-001")
	if err != nil {
		t.Fatalf("IsWorkflowRunning(wf-run-001) error = %v", err)
	}
	if !isRunning {
		t.Error("Expected wf-run-001 to be running")
	}

	isRunning, err = manager.IsWorkflowRunning(ctx, "wf-run-003")
	if err != nil {
		t.Fatalf("IsWorkflowRunning(wf-run-003) error = %v", err)
	}
	if isRunning {
		t.Error("Expected wf-run-003 to NOT be running")
	}

	// Clear one
	if err := manager.ClearWorkflowRunning(ctx, "wf-run-001"); err != nil {
		t.Fatalf("ClearWorkflowRunning(wf-run-001) error = %v", err)
	}

	running, err = manager.ListRunningWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListRunningWorkflows() error = %v", err)
	}
	if len(running) != 1 {
		t.Errorf("Expected 1 running workflow, got %d", len(running))
	}
}

func TestSQLiteStateManager_ZombieWorkflowDetection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow
	state := newTestStateSQLite()
	state.WorkflowID = "wf-zombie-001"
	state.Status = core.WorkflowStatusRunning
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Set as running
	if err := manager.SetWorkflowRunning(ctx, "wf-zombie-001"); err != nil {
		t.Fatalf("SetWorkflowRunning() error = %v", err)
	}

	// Set old heartbeat directly
	oldTime := time.Now().UTC().Add(-1 * time.Hour)
	result, err := manager.db.ExecContext(ctx, `
		UPDATE running_workflows SET heartbeat_at = ? WHERE workflow_id = ?
	`, oldTime, "wf-zombie-001")
	if err != nil {
		t.Fatalf("UPDATE heartbeat error = %v", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Fatalf("Expected 1 row affected by UPDATE, got %d", rowsAffected)
	}

	// Find zombies (threshold 5 minutes)
	zombies, err := manager.FindZombieWorkflows(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("FindZombieWorkflows() error = %v", err)
	}
	if len(zombies) != 1 {
		t.Errorf("Expected 1 zombie workflow, got %d", len(zombies))
	}
	if len(zombies) > 0 && zombies[0].WorkflowID != "wf-zombie-001" {
		t.Errorf("Expected zombie workflow wf-zombie-001, got %s", zombies[0].WorkflowID)
	}
}

func TestSQLiteStateManager_UpdateWorkflowHeartbeat(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow
	state := newTestStateSQLite()
	state.WorkflowID = "wf-heartbeat-001"
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Set as running
	if err := manager.SetWorkflowRunning(ctx, "wf-heartbeat-001"); err != nil {
		t.Fatalf("SetWorkflowRunning() error = %v", err)
	}

	// Update heartbeat
	if err := manager.UpdateWorkflowHeartbeat(ctx, "wf-heartbeat-001"); err != nil {
		t.Fatalf("UpdateWorkflowHeartbeat() error = %v", err)
	}

	// Verify heartbeat was updated (workflow should not be a zombie)
	zombies, err := manager.FindZombieWorkflows(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("FindZombieWorkflows() error = %v", err)
	}
	if len(zombies) != 0 {
		t.Errorf("Expected no zombies after heartbeat update, got %d", len(zombies))
	}

	// Test heartbeat for non-running workflow (should add it to running_workflows)
	state2 := newTestStateSQLite()
	state2.WorkflowID = "wf-heartbeat-002"
	if err := manager.Save(ctx, state2); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := manager.UpdateWorkflowHeartbeat(ctx, "wf-heartbeat-002"); err != nil {
		t.Fatalf("UpdateWorkflowHeartbeat(wf-heartbeat-002) error = %v", err)
	}

	// Should now be in running workflows
	isRunning, err := manager.IsWorkflowRunning(ctx, "wf-heartbeat-002")
	if err != nil {
		t.Fatalf("IsWorkflowRunning() error = %v", err)
	}
	if !isRunning {
		t.Error("Expected wf-heartbeat-002 to be running after heartbeat update")
	}
}

func TestSQLiteStateManager_WorkflowBranchPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow with WorkflowBranch
	state := newTestStateSQLite()
	state.WorkflowID = "wf-branch-test"
	state.WorkflowBranch = "quorum/wf-branch-test"

	// Save
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify
	loaded, err := manager.LoadByID(ctx, "wf-branch-test")
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadByID() returned nil")
	}

	if loaded.WorkflowBranch != "quorum/wf-branch-test" {
		t.Errorf("WorkflowBranch = %q, want %q", loaded.WorkflowBranch, "quorum/wf-branch-test")
	}
}

func TestSQLiteStateManager_TaskMergeFieldsPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow with tasks that have merge fields
	state := newTestStateSQLite()
	state.WorkflowID = "wf-merge-test"
	state.Tasks = map[core.TaskID]*core.TaskState{
		"task-merge-pending": {
			ID:           "task-merge-pending",
			Phase:        core.PhaseAnalyze,
			Name:         "Task with merge pending",
			Status:       core.TaskStatusCompleted,
			MergePending: true,
			MergeCommit:  "",
		},
		"task-merged": {
			ID:           "task-merged",
			Phase:        core.PhaseAnalyze,
			Name:         "Task that was merged",
			Status:       core.TaskStatusCompleted,
			MergePending: false,
			MergeCommit:  "abc123def456",
		},
	}
	state.TaskOrder = []core.TaskID{"task-merge-pending", "task-merged"}

	// Save
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify
	loaded, err := manager.LoadByID(ctx, "wf-merge-test")
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadByID() returned nil")
	}

	// Verify task with MergePending=true
	taskPending := loaded.Tasks["task-merge-pending"]
	if taskPending == nil {
		t.Fatal("task-merge-pending not found")
	}
	if !taskPending.MergePending {
		t.Error("task-merge-pending.MergePending = false, want true")
	}
	if taskPending.MergeCommit != "" {
		t.Errorf("task-merge-pending.MergeCommit = %q, want empty", taskPending.MergeCommit)
	}

	// Verify task with MergeCommit
	taskMerged := loaded.Tasks["task-merged"]
	if taskMerged == nil {
		t.Fatal("task-merged not found")
	}
	if taskMerged.MergePending {
		t.Error("task-merged.MergePending = true, want false")
	}
	if taskMerged.MergeCommit != "abc123def456" {
		t.Errorf("task-merged.MergeCommit = %q, want %q", taskMerged.MergeCommit, "abc123def456")
	}
}

func TestSQLiteStateManager_WorkflowIsolationFieldsPersistence(t *testing.T) {
	// Comprehensive test for all workflow isolation fields
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create workflow with all isolation fields populated
	state := newTestStateSQLite()
	state.WorkflowID = "wf-isolation-test"
	state.WorkflowBranch = "quorum/wf-isolation-test"
	state.Tasks = map[core.TaskID]*core.TaskState{
		"task-1": {
			ID:           "task-1",
			Phase:        core.PhaseAnalyze,
			Name:         "Task with worktree",
			Status:       core.TaskStatusRunning,
			WorktreePath: "/tmp/worktrees/task-1",
			Branch:       "quorum/wf-isolation-test__task-1",
			LastCommit:   "commit123",
			MergePending: false,
			MergeCommit:  "",
		},
		"task-2": {
			ID:           "task-2",
			Phase:        core.PhaseAnalyze,
			Name:         "Completed task with merge",
			Status:       core.TaskStatusCompleted,
			WorktreePath: "",
			Branch:       "quorum/wf-isolation-test__task-2",
			LastCommit:   "commit456",
			MergePending: false,
			MergeCommit:  "merge789",
		},
		"task-3": {
			ID:           "task-3",
			Phase:        core.PhaseAnalyze,
			Name:         "Task with merge conflict",
			Status:       core.TaskStatusCompleted,
			WorktreePath: "",
			Branch:       "quorum/wf-isolation-test__task-3",
			LastCommit:   "commitabc",
			MergePending: true,
			MergeCommit:  "",
		},
	}
	state.TaskOrder = []core.TaskID{"task-1", "task-2", "task-3"}

	// Save
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify all fields
	loaded, err := manager.LoadByID(ctx, "wf-isolation-test")
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadByID() returned nil")
	}

	// Verify workflow-level fields
	if loaded.WorkflowBranch != "quorum/wf-isolation-test" {
		t.Errorf("WorkflowBranch = %q, want %q", loaded.WorkflowBranch, "quorum/wf-isolation-test")
	}

	// Verify task-1 (running with worktree)
	task1 := loaded.Tasks["task-1"]
	if task1 == nil {
		t.Fatal("task-1 not found")
	}
	if task1.WorktreePath != "/tmp/worktrees/task-1" {
		t.Errorf("task-1.WorktreePath = %q, want %q", task1.WorktreePath, "/tmp/worktrees/task-1")
	}
	if task1.Branch != "quorum/wf-isolation-test__task-1" {
		t.Errorf("task-1.Branch = %q, want %q", task1.Branch, "quorum/wf-isolation-test__task-1")
	}
	if task1.LastCommit != "commit123" {
		t.Errorf("task-1.LastCommit = %q, want %q", task1.LastCommit, "commit123")
	}
	if task1.MergePending {
		t.Error("task-1.MergePending = true, want false")
	}

	// Verify task-2 (completed with merge commit)
	task2 := loaded.Tasks["task-2"]
	if task2 == nil {
		t.Fatal("task-2 not found")
	}
	if task2.MergeCommit != "merge789" {
		t.Errorf("task-2.MergeCommit = %q, want %q", task2.MergeCommit, "merge789")
	}
	if task2.MergePending {
		t.Error("task-2.MergePending = true, want false")
	}

	// Verify task-3 (merge pending due to conflict)
	task3 := loaded.Tasks["task-3"]
	if task3 == nil {
		t.Fatal("task-3 not found")
	}
	if !task3.MergePending {
		t.Error("task-3.MergePending = false, want true")
	}
	if task3.MergeCommit != "" {
		t.Errorf("task-3.MergeCommit = %q, want empty", task3.MergeCommit)
	}
}
