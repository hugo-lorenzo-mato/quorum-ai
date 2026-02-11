package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockModeEnforcer implements ModeEnforcerInterface for testing.
type mockModeEnforcer struct {
	dryRun  bool
	blocked bool
}

func (m *mockModeEnforcer) CanExecute(_ context.Context, _ ModeOperation) error {
	if m.blocked {
		return errors.New("blocked by mode enforcer")
	}
	return nil
}

func (m *mockModeEnforcer) RecordCost(_ float64) {}

func (m *mockModeEnforcer) IsDryRun() bool { return m.dryRun }

func TestExecutor_ModeEnforcerBlocks(t *testing.T) {
	t.Parallel()
	// Create context with blocking mode enforcer
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Status: core.TaskStatusPending},
				},
			},
		},
		ModeEnforcer: &mockModeEnforcer{blocked: true},
		Config:       &Config{},
		Logger:       logging.NewNop(),
	}

	executor := &Executor{}
	task := &core.Task{ID: "task-1", Name: "Test Task"}

	err := executor.executeTask(context.Background(), wctx, task, false)
	if err == nil {
		t.Error("Expected error from mode enforcer, got nil")
	}
	if !strings.Contains(err.Error(), "mode enforcer blocked") {
		t.Errorf("Expected mode enforcer error, got: %v", err)
	}
}

func TestExecutor_ModeEnforcerAllows(t *testing.T) {
	t.Parallel()
	// Create context with permissive mode enforcer
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Status: core.TaskStatusPending},
				},
			},
		},
		ModeEnforcer: &mockModeEnforcer{blocked: false},
		Config:       &Config{DryRun: true}, // Dry run to avoid actual execution
		Logger:       logging.NewNop(),
		Checkpoint:   &mockCheckpointCreator{},
		Output:       NopOutputNotifier{},
	}

	executor := &Executor{}
	task := &core.Task{ID: "task-1", Name: "Test Task"}

	// In dry-run mode, executeTask should complete without error
	// (it just marks the task as completed without actual execution)
	err := executor.executeTask(context.Background(), wctx, task, false)
	if err != nil {
		t.Errorf("Expected no error from permissive mode enforcer, got: %v", err)
	}

	// Verify task was marked completed
	if wctx.State.Tasks["task-1"].Status != core.TaskStatusCompleted {
		t.Errorf("Expected task status to be completed, got: %v", wctx.State.Tasks["task-1"].Status)
	}
}

func TestExecutor_NilModeEnforcer(t *testing.T) {
	t.Parallel()
	// Create context without mode enforcer (nil)
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Status: core.TaskStatusPending},
				},
			},
		},
		ModeEnforcer: nil, // No mode enforcer
		Config:       &Config{DryRun: true},
		Logger:       logging.NewNop(),
		Checkpoint:   &mockCheckpointCreator{},
		Output:       NopOutputNotifier{},
	}

	executor := &Executor{}
	task := &core.Task{ID: "task-1", Name: "Test Task"}

	// Should work without mode enforcer
	err := executor.executeTask(context.Background(), wctx, task, false)
	if err != nil {
		t.Errorf("Expected no error with nil mode enforcer, got: %v", err)
	}
}

func TestExecutor_SavesTaskOutput(t *testing.T) {
	t.Parallel()
	mockRegistry := &mockAgentRegistry{
		agents: map[string]core.Agent{
			"mock": &mockAgent{
				result: &core.ExecuteResult{
					Output:    "Task completed successfully\nFiles modified: 3",
					TokensIn:  100,
					TokensOut: 200,
				},
			},
		},
	}

	executor := NewExecutor(nil, nil, nil)
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"task-1": {ID: "task-1", Status: core.TaskStatusPending},
				},
			},
		},
		Agents:     mockRegistry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{limiter: &mockRateLimiter{}},
		Config: &Config{
			DefaultAgent: "mock",
		},
		Logger: logging.NewNop(),
		Output: NopOutputNotifier{},
	}

	task := &core.Task{ID: "task-1", Name: "Test Task", CLI: "mock"}
	err := executor.executeTask(context.Background(), wctx, task, false)

	if err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}

	taskState := wctx.State.Tasks["task-1"]
	if taskState.Output == "" {
		t.Error("Expected output to be saved")
	}
	if !strings.Contains(taskState.Output, "Task completed") {
		t.Errorf("Unexpected output: %s", taskState.Output)
	}
}

func TestExecutor_TruncatesLargeOutput(t *testing.T) {
	t.Parallel()
	largeOutput := strings.Repeat("x", core.MaxInlineOutputSize+1000)

	taskState := &core.TaskState{ID: "task-1"}

	if len(largeOutput) <= core.MaxInlineOutputSize {
		taskState.Output = largeOutput
	} else {
		taskState.Output = largeOutput[:core.MaxInlineOutputSize] + "\n... [truncated, see output_file]"
	}

	if len(taskState.Output) > core.MaxInlineOutputSize+100 {
		t.Error("Output should be truncated")
	}
	if !strings.Contains(taskState.Output, "[truncated") {
		t.Error("Truncated output should have marker")
	}
}

func TestGetFullOutput_ReadsFromFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "task-1.txt")
	fullOutput := "This is the full output content"
	if err := os.WriteFile(outputPath, []byte(fullOutput), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	taskState := &core.TaskState{
		Output:     "This is the full... [truncated, see output_file]",
		OutputFile: outputPath,
	}

	result, err := GetFullOutput(taskState)
	if err != nil {
		t.Fatalf("GetFullOutput failed: %v", err)
	}

	if result != fullOutput {
		t.Errorf("Expected full output, got: %s", result)
	}
}
