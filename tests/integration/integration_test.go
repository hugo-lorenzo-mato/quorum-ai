//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestIntegration_StateManager(t *testing.T) {
	dir := testutil.TempDir(t)
	statePath := filepath.Join(dir, ".quorum", "state", "workflow.json")

	sm := state.NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create and save state
	ws := &core.WorkflowState{
		WorkflowID:   "test-workflow-1",
		CurrentPhase: core.PhaseAnalyze,
		Status:       core.WorkflowStatusRunning,
		Tasks:        make(map[core.TaskID]*core.TaskState),
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

	configContent := `version: "1"
agents:
  claude:
    enabled: true
    model: claude-3-opus
  gemini:
    enabled: true
    model: gemini-pro
workflow:
  consensus_threshold: 0.75
  max_retries: 3
`

	configPath := filepath.Join(dir, ".quorum.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	loader := config.NewLoader(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Workflow.ConsensusThreshold != 0.75 {
		t.Errorf("threshold mismatch: got %f, want 0.75", cfg.Workflow.ConsensusThreshold)
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

func TestIntegration_ConsensusCheck(t *testing.T) {
	checker := service.NewConsensusChecker(0.75, service.DefaultWeights())

	// Test with similar outputs
	outputs := []service.AnalysisOutput{
		{
			AgentName:       "claude",
			Claims:          []string{"Code is well structured", "Follows SOLID principles"},
			Risks:           []string{"No major security vulnerabilities"},
			Recommendations: []string{"Add more tests"},
		},
		{
			AgentName:       "gemini",
			Claims:          []string{"Code structure is good", "Adheres to SOLID principles"},
			Risks:           []string{"No significant security issues"},
			Recommendations: []string{"Consider adding tests"},
		},
	}

	result := checker.Evaluate(outputs)

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("invalid score: %f", result.Score)
	}

	t.Logf("Consensus result: score=%.2f, needsV3=%v", result.Score, result.NeedsV3)
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
