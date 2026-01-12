//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
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
	stateDir := filepath.Join(dir, ".quorum", "state")

	sm := state.NewJSONStateManager(stateDir)
	ctx := context.Background()

	// Create and save state
	ws := &core.WorkflowState{
		WorkflowID:   "test-workflow-1",
		CurrentPhase: core.PhaseAnalyze,
		Status:       core.WorkflowStatusRunning,
		Tasks:        []*core.Task{{ID: "task-1", Name: "Test Task", Phase: core.PhaseAnalyze}},
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

	// Delete state
	if err := sm.Delete(ctx); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if sm.Exists() {
		t.Error("state should not exist after delete")
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

func TestIntegration_WorkflowExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Build workflow
	builder := core.NewWorkflowBuilder("integration-test")

	analyzeTask := builder.AddTask("analyze-code", core.PhaseAnalyze)
	analyzeTask.Name = "Analyze Code"

	researchTask := builder.AddTask("research-docs", core.PhaseResearch)
	researchTask.Name = "Research Documentation"
	researchTask.DependsOn = []core.TaskID{"analyze-code"}

	wf := builder.Build()

	// Execute with dry-run mode
	runner := service.NewWorkflowRunner(nil)
	runner.SetMode(service.ExecutionMode{DryRun: true})

	state, err := runner.Execute(ctx, wf)
	if err != nil && !service.IsDryRunBlocked(err) {
		t.Logf("execution completed with: %v", err)
	}

	if state == nil {
		t.Fatal("state should not be nil")
	}

	if state.WorkflowID != "integration-test" {
		t.Errorf("workflow ID mismatch: got %s, want integration-test", state.WorkflowID)
	}
}

func TestIntegration_MetricsCollection(t *testing.T) {
	collector := service.NewMetricsCollector()

	collector.StartWorkflow()

	// Simulate task execution
	task := &core.Task{
		ID:    "test-task-1",
		Name:  "Test Task",
		Phase: core.PhaseAnalyze,
	}

	collector.StartTask(task, "claude")
	time.Sleep(10 * time.Millisecond)

	result := &core.ExecuteResult{
		Output:    "test output",
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.01,
	}

	collector.EndTask(task.ID, result, nil)
	collector.EndWorkflow()

	metrics := collector.GetWorkflowMetrics()

	if metrics.TasksTotal != 1 {
		t.Errorf("tasks total mismatch: got %d, want 1", metrics.TasksTotal)
	}

	if metrics.TasksCompleted != 1 {
		t.Errorf("tasks completed mismatch: got %d, want 1", metrics.TasksCompleted)
	}

	if metrics.TotalTokensIn != 100 {
		t.Errorf("tokens in mismatch: got %d, want 100", metrics.TotalTokensIn)
	}

	if metrics.TotalCostUSD != 0.01 {
		t.Errorf("cost mismatch: got %f, want 0.01", metrics.TotalCostUSD)
	}
}

func TestIntegration_DAGExecution(t *testing.T) {
	// Test DAG-based task execution order
	dag := service.NewDAG()

	// Create tasks with dependencies
	dag.AddNode("task-a")
	dag.AddNode("task-b")
	dag.AddNode("task-c")
	dag.AddNode("task-d")

	// b depends on a, c depends on a, d depends on b and c
	dag.AddEdge("task-a", "task-b")
	dag.AddEdge("task-a", "task-c")
	dag.AddEdge("task-b", "task-d")
	dag.AddEdge("task-c", "task-d")

	order, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("topological sort failed: %v", err)
	}

	// Verify order constraints
	positions := make(map[string]int)
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
	checker := service.NewConsensusChecker(0.75)

	// Test with similar outputs
	outputs := []service.AnalysisOutput{
		{
			AgentName: "claude",
			Sections: map[string]string{
				"claims": "The code is well structured and follows SOLID principles",
				"risks":  "No major security vulnerabilities detected",
			},
		},
		{
			AgentName: "gemini",
			Sections: map[string]string{
				"claims": "Code structure is good and adheres to SOLID principles",
				"risks":  "No significant security issues found",
			},
		},
	}

	result := checker.Evaluate(outputs)

	if result.Score < 0 || result.Score > 1 {
		t.Errorf("invalid score: %f", result.Score)
	}

	t.Logf("Consensus result: score=%.2f, needsV3=%v", result.Score, result.NeedsV3)
}
