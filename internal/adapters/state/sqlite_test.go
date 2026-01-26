package state

import (
	"context"
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

	// Workflow should be deleted (in SQLite, archive means delete)
	loaded, err := manager.LoadByID(ctx, state.WorkflowID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded != nil {
		t.Error("Workflow should have been deleted (archived)")
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
