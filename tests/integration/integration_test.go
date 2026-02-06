//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestIntegration_StateManager(t *testing.T) {
	dir := testutil.TempDir(t)
	statePath := filepath.Join(dir, ".quorum", "state", "workflow.json")

	sm, err := state.NewStateManager(statePath)
	if err != nil {
		t.Fatalf("create state manager: %v", err)
	}
	defer state.CloseStateManager(sm)
	ctx := context.Background()

	// Create and save state
	ws := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "test-workflow-1"},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseAnalyze,
			Status:       core.WorkflowStatusRunning,
			Tasks:        make(map[core.TaskID]*core.TaskState),
		},
	}
	ws.Tasks["task-1"] = &core.TaskState{
		ID:    "task-1",
		Name:  "Test Task",
		Phase: core.PhaseAnalyze,
	}

	if err := sm.Save(ctx, ws); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load state
	loaded, err := sm.Load(ctx)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.WorkflowID != ws.WorkflowID {
		t.Errorf("workflow ID mismatch: got %s, want %s", loaded.WorkflowID, ws.WorkflowID)
	}

	// Verify state file exists
	if !sm.Exists() {
		t.Error("state file should exist")
	}
}

func TestIntegration_ConfigLoader(t *testing.T) {
	dir := testutil.TempDir(t)

	configContent := `log:
  level: info
agents:
  default: claude
  claude:
    enabled: true
    path: claude
    model: claude-3-opus
  gemini:
    enabled: true
    path: gemini
    model: gemini-pro
phases:
  analyze:
    moderator:
      enabled: true
      agent: claude
      threshold: 0.75
workflow:
  max_retries: 3
`

	configPath := filepath.Join(dir, ".quorum.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	loader := config.NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Phases.Analyze.Moderator.Threshold != 0.75 {
		t.Errorf("threshold mismatch: got %f, want 0.75", cfg.Phases.Analyze.Moderator.Threshold)
	}

	if cfg.Agents.Claude.Model != "claude-3-opus" {
		t.Errorf("claude model mismatch: got %s, want claude-3-opus", cfg.Agents.Claude.Model)
	}
}

func TestIntegration_DAGExecution(t *testing.T) {
	// Test DAG-based task execution order
	dag := service.NewDAGBuilder()

	// Create tasks
	taskA := core.NewTask("task-a", "Task A", core.PhaseAnalyze)
	taskB := core.NewTask("task-b", "Task B", core.PhaseAnalyze)
	taskC := core.NewTask("task-c", "Task C", core.PhasePlan)
	taskD := core.NewTask("task-d", "Task D", core.PhaseExecute)

	// Add tasks to DAG
	if err := dag.AddTask(taskA); err != nil {
		t.Fatalf("adding task-a: %v", err)
	}
	if err := dag.AddTask(taskB); err != nil {
		t.Fatalf("adding task-b: %v", err)
	}
	if err := dag.AddTask(taskC); err != nil {
		t.Fatalf("adding task-c: %v", err)
	}
	if err := dag.AddTask(taskD); err != nil {
		t.Fatalf("adding task-d: %v", err)
	}

	// b depends on a, c depends on a, d depends on b and c
	if err := dag.AddDependency("task-b", "task-a"); err != nil {
		t.Fatalf("adding dependency b->a: %v", err)
	}
	if err := dag.AddDependency("task-c", "task-a"); err != nil {
		t.Fatalf("adding dependency c->a: %v", err)
	}
	if err := dag.AddDependency("task-d", "task-b"); err != nil {
		t.Fatalf("adding dependency d->b: %v", err)
	}
	if err := dag.AddDependency("task-d", "task-c"); err != nil {
		t.Fatalf("adding dependency d->c: %v", err)
	}

	// Build DAG to get topological order
	dagState, err := dag.Build()
	if err != nil {
		t.Fatalf("building DAG: %v", err)
	}

	order := dagState.Order

	// Verify order constraints
	positions := make(map[core.TaskID]int)
	for i, node := range order {
		positions[node] = i
	}

	// a must come before b and c
	if positions["task-a"] >= positions["task-b"] {
		t.Errorf("task-a should come before task-b")
	}
	if positions["task-a"] >= positions["task-c"] {
		t.Errorf("task-a should come before task-c")
	}

	// b and c must come before d
	if positions["task-b"] >= positions["task-d"] {
		t.Errorf("task-b should come before task-d")
	}
	if positions["task-c"] >= positions["task-d"] {
		t.Errorf("task-c should come before task-d")
	}
}

func TestIntegration_WorkflowCreation(t *testing.T) {
	// Test workflow creation and state management
	wf := core.NewWorkflow("test-wf", "Analyze the codebase", nil)

	// Add tasks
	task1 := core.NewTask("analyze", "Analyze Code", core.PhaseAnalyze)
	task2 := core.NewTask("plan", "Create Plan", core.PhasePlan).WithDependencies("analyze")
	task3 := core.NewTask("execute", "Execute Changes", core.PhaseExecute).WithDependencies("plan")

	if err := wf.AddTask(task1); err != nil {
		t.Fatalf("adding task1: %v", err)
	}
	if err := wf.AddTask(task2); err != nil {
		t.Fatalf("adding task2: %v", err)
	}
	if err := wf.AddTask(task3); err != nil {
		t.Fatalf("adding task3: %v", err)
	}

	// Verify workflow state
	if wf.Status != core.WorkflowStatusPending {
		t.Errorf("expected pending status, got %s", wf.Status)
	}

	// Start workflow
	if err := wf.Start(); err != nil {
		t.Fatalf("starting workflow: %v", err)
	}

	if wf.Status != core.WorkflowStatusRunning {
		t.Errorf("expected running status, got %s", wf.Status)
	}

	// Get ready tasks
	ready := wf.ReadyTasks()
	if len(ready) != 1 || ready[0].ID != "analyze" {
		t.Errorf("expected analyze task to be ready, got %v", ready)
	}
}

// TestIntegration_BackendSelection tests the factory function creates correct backend types.
func TestIntegration_StateManager_SQLiteOnly(t *testing.T) {
	dir := testutil.TempDir(t)
	legacyPath := filepath.Join(dir, "state.legacy") // intentionally not .db

	sm, err := state.NewStateManager(legacyPath)
	testutil.AssertNoError(t, err)
	defer state.CloseStateManager(sm)

	// Runtime is SQLite-only now.
	if _, ok := sm.(*state.SQLiteStateManager); !ok {
		t.Fatalf("expected SQLiteStateManager, got %T", sm)
	}

	normalizedDBPath := strings.TrimSuffix(legacyPath, filepath.Ext(legacyPath)) + ".db"
	if _, err := os.Stat(normalizedDBPath); err != nil {
		t.Fatalf("expected sqlite db to exist at %s: %v", normalizedDBPath, err)
	}
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatalf("did not expect non-db path to exist at %s", legacyPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected stat error for %s: %v", legacyPath, err)
	}

	// Verify the manager works by saving and loading
	ctx := context.Background()
	ws := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "test-wf"},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseAnalyze,
			Status:       core.WorkflowStatusRunning,
			Tasks:        make(map[core.TaskID]*core.TaskState),
		},
	}

	testutil.AssertNoError(t, sm.Save(ctx, ws))
	loaded, err := sm.Load(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, loaded.WorkflowID, "test-wf")
}

// TestIntegration_SQLiteStateManager_CRUD tests SQLite-specific CRUD operations.
func TestIntegration_SQLiteStateManager_CRUD(t *testing.T) {
	dir := testutil.TempDir(t)
	dbPath := filepath.Join(dir, "workflow.db")

	sm, err := state.NewStateManager(dbPath)
	testutil.AssertNoError(t, err)
	defer state.CloseStateManager(sm)

	ctx := context.Background()

	// Test 1: Create and save workflow with tasks
	now := time.Now().Truncate(time.Second)
	ws := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Version:    1,
			WorkflowID: "wf-integration-test",
			Prompt:     "Integration test prompt",
			Blueprint: &core.Blueprint{
				Consensus:  core.BlueprintConsensus{Threshold: 0.8},
				MaxRetries: 3,
				Timeout:    time.Hour,
			},
			CreatedAt: now,
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {
					ID:           "task-1",
					Phase:        core.PhaseAnalyze,
					Name:         "Analyze Task",
					Status:       core.TaskStatusCompleted,
					CLI:          "claude",
					Model:        "opus",
					Dependencies: []core.TaskID{},
					TokensIn:     500,
					TokensOut:    200,
					CostUSD:      float64(0.05),
					Output:       "Analysis complete",
				},
				"task-2": {
					ID:           "task-2",
					Phase:        core.PhasePlan,
					Name:         "Plan Task",
					Status:       core.TaskStatusPending,
					Dependencies: []core.TaskID{"task-1"},
				},
			},
			TaskOrder: []core.TaskID{"task-1", "task-2"},
			Metrics: &core.StateMetrics{
				TotalCostUSD:   0.05,
				TotalTokensIn:  500,
				TotalTokensOut: 200,
			},
			Checkpoints: []core.Checkpoint{
				{
					ID:        "cp-1",
					Type:      "task_complete",
					Phase:     core.PhaseAnalyze,
					TaskID:    "task-1",
					Timestamp: now,
					Message:   "Task completed",
				},
			},
			UpdatedAt: now,
		},
	}

	err = sm.Save(ctx, ws)
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, sm.Exists(), "state should exist after save")

	// Test 2: Load and verify all fields
	loaded, err := sm.Load(ctx)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, loaded.WorkflowID, ws.WorkflowID)
	testutil.AssertEqual(t, loaded.Status, ws.Status)
	testutil.AssertEqual(t, loaded.CurrentPhase, ws.CurrentPhase)
	testutil.AssertEqual(t, loaded.Prompt, ws.Prompt)
	if len(loaded.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loaded.Tasks))
	}
	testutil.AssertLen(t, loaded.TaskOrder, 2)

	// Verify task details
	task1 := loaded.Tasks["task-1"]
	testutil.AssertEqual(t, task1.Status, core.TaskStatusCompleted)
	testutil.AssertEqual(t, task1.Output, "Analysis complete")
	testutil.AssertEqual(t, task1.TokensIn, 500)
	testutil.AssertEqual(t, task1.CostUSD, 0.05)

	task2 := loaded.Tasks["task-2"]
	testutil.AssertLen(t, task2.Dependencies, 1)
	testutil.AssertEqual(t, task2.Dependencies[0], core.TaskID("task-1"))

	// Verify blueprint
	testutil.AssertEqual(t, loaded.Blueprint.Consensus.Threshold, 0.8)
	testutil.AssertEqual(t, loaded.Blueprint.MaxRetries, 3)

	// Verify metrics
	testutil.AssertEqual(t, loaded.Metrics.TotalCostUSD, 0.05)

	// Verify checkpoints
	testutil.AssertLen(t, loaded.Checkpoints, 1)
	testutil.AssertEqual(t, loaded.Checkpoints[0].ID, "cp-1")

	// Test 3: Update workflow
	ws.Status = core.WorkflowStatusCompleted
	ws.CurrentPhase = core.PhaseExecute
	ws.Tasks["task-2"].Status = core.TaskStatusCompleted
	completedAt := time.Now().Truncate(time.Second)
	ws.Tasks["task-2"].CompletedAt = &completedAt

	err = sm.Save(ctx, ws)
	testutil.AssertNoError(t, err)

	loaded, err = sm.Load(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, loaded.Status, core.WorkflowStatusCompleted)
	testutil.AssertEqual(t, loaded.Tasks["task-2"].Status, core.TaskStatusCompleted)
}

// TestIntegration_SQLiteMultipleWorkflows tests managing multiple workflows in SQLite.
func TestIntegration_SQLiteMultipleWorkflows(t *testing.T) {
	dir := testutil.TempDir(t)
	dbPath := filepath.Join(dir, "multi-workflow.db")

	sm, err := state.NewStateManager(dbPath)
	testutil.AssertNoError(t, err)
	defer state.CloseStateManager(sm)

	ctx := context.Background()

	// Create first workflow
	ws1 := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "First workflow",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			CurrentPhase: core.PhaseExecute,
			Tasks:        make(map[core.TaskID]*core.TaskState),
		},
	}
	testutil.AssertNoError(t, sm.Save(ctx, ws1))

	// Create second workflow
	ws2 := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-2",
			Prompt:     "Second workflow",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        make(map[core.TaskID]*core.TaskState),
		},
	}
	testutil.AssertNoError(t, sm.Save(ctx, ws2))

	// SQLite-specific: cast to access ListWorkflows and LoadByID
	sqliteSM, ok := sm.(*state.SQLiteStateManager)
	if !ok {
		t.Fatal("expected SQLiteStateManager type")
	}

	// List workflows should return both
	summaries, err := sqliteSM.ListWorkflows(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, summaries, 2)

	// Last saved should be active
	activeID, err := sqliteSM.GetActiveWorkflowID(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, activeID, "wf-2")

	// Load specific workflow by ID
	loaded1, err := sqliteSM.LoadByID(ctx, "wf-1")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, loaded1.Prompt, "First workflow")

	// Set active workflow
	testutil.AssertNoError(t, sqliteSM.SetActiveWorkflowID(ctx, "wf-1"))
	activeID, err = sqliteSM.GetActiveWorkflowID(ctx)
	testutil.AssertNoError(t, err)
	// Completed workflows are auto-cleaned from active tracking.
	testutil.AssertEqual(t, activeID, "")
}

// TestIntegration_BackendFromConfig tests creating state manager from config.
// TestIntegration_SQLiteBackupRestore tests SQLite backup and restore functionality.
func TestIntegration_SQLiteBackupRestore(t *testing.T) {
	dir := testutil.TempDir(t)
	dbPath := filepath.Join(dir, "backup-test.db")

	sm, err := state.NewStateManager(dbPath)
	testutil.AssertNoError(t, err)
	defer state.CloseStateManager(sm)

	sqliteSM := sm.(*state.SQLiteStateManager)
	ctx := context.Background()

	// Create initial state
	ws := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-backup",
			Prompt:     "Original state",
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        make(map[core.TaskID]*core.TaskState),
		},
	}
	testutil.AssertNoError(t, sqliteSM.Save(ctx, ws))

	// Create backup
	testutil.AssertNoError(t, sqliteSM.Backup(ctx))

	// Modify state
	ws.Prompt = "Modified state"
	ws.Status = core.WorkflowStatusCompleted
	testutil.AssertNoError(t, sqliteSM.Save(ctx, ws))

	// Verify modification
	loaded, err := sqliteSM.Load(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, loaded.Prompt, "Modified state")

	// Restore from backup
	restored, err := sqliteSM.Restore(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, restored.Prompt, "Original state")
	testutil.AssertEqual(t, restored.Status, core.WorkflowStatusRunning)
}

// TestIntegration_StateManagerLifecycle tests proper cleanup of state managers.
func TestIntegration_StateManagerLifecycle(t *testing.T) {
	dir := testutil.TempDir(t)

	t.Run("sqlite manager cleanup", func(t *testing.T) {
		path := filepath.Join(dir, "lifecycle-sqlite.db")
		sm, err := state.NewStateManager(path)
		testutil.AssertNoError(t, err)

		// Save something
		ctx := context.Background()
		ws := &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "lifecycle-test"},
			WorkflowRun: core.WorkflowRun{
				Status: core.WorkflowStatusPending,
				Tasks:  make(map[core.TaskID]*core.TaskState),
			},
		}
		testutil.AssertNoError(t, sm.Save(ctx, ws))

		// Close should properly close SQLite connection
		testutil.AssertNoError(t, state.CloseStateManager(sm))
	})

	t.Run("nil manager cleanup", func(t *testing.T) {
		// Should not panic or error
		testutil.AssertNoError(t, state.CloseStateManager(nil))
	})
}
