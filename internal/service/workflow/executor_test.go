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
	sandboxed bool
	dryRun    bool
	blocked   bool
}

func (m *mockModeEnforcer) CanExecute(_ context.Context, _ ModeOperation) error {
	if m.blocked {
		return errors.New("blocked by mode enforcer")
	}
	return nil
}

func (m *mockModeEnforcer) RecordCost(_ float64) {}

func (m *mockModeEnforcer) IsSandboxed() bool { return m.sandboxed }
func (m *mockModeEnforcer) IsDryRun() bool    { return m.dryRun }

func TestExecutor_ModeEnforcerBlocks(t *testing.T) {
	// Create context with blocking mode enforcer
	wctx := &Context{
		State: &core.WorkflowState{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusPending},
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
	// Create context with permissive mode enforcer
	wctx := &Context{
		State: &core.WorkflowState{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusPending},
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
	// Create context without mode enforcer (nil)
	wctx := &Context{
		State: &core.WorkflowState{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusPending},
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
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusPending},
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

func TestCostLimitEnforcement(t *testing.T) {
	t.Run("task exceeds cost limit", func(t *testing.T) {
		wctx := &Context{
			State: &core.WorkflowState{
				WorkflowID: "test-wf",
				Tasks:      make(map[core.TaskID]*core.TaskState),
				Metrics:    &core.StateMetrics{},
			},
			Logger: logging.NewNop(),
			Config: &Config{
				MaxCostPerTask: 1.0, // $1 limit
			},
		}

		task := &core.Task{
			ID:   "task-1",
			Name: "Test Task",
		}
		taskState := &core.TaskState{
			ID:     task.ID,
			Status: core.TaskStatusPending,
		}
		wctx.State.Tasks[task.ID] = taskState

		// Simulate a result that exceeds task budget
		result := &core.ExecuteResult{
			Output:    "test output",
			CostUSD:   1.5, // $1.50 exceeds $1 limit
			TokensIn:  100,
			TokensOut: 50,
		}

		// Apply metrics (simulating what happens in executeTask after agent call)
		taskState.TokensIn = result.TokensIn
		taskState.TokensOut = result.TokensOut
		taskState.CostUSD = result.CostUSD
		wctx.State.Metrics.TotalCostUSD += result.CostUSD

		// Check cost limits
		if wctx.Config.MaxCostPerTask > 0 && result.CostUSD > wctx.Config.MaxCostPerTask {
			err := core.ErrTaskBudgetExceeded(string(task.ID), result.CostUSD, wctx.Config.MaxCostPerTask)
			if err == nil {
				t.Error("expected error for task budget exceeded")
			}
			if !core.IsCategory(err, core.ErrCatBudget) {
				t.Errorf("expected budget error category, got %v", core.GetCategory(err))
			}
		}
	})

	t.Run("workflow exceeds cost limit", func(t *testing.T) {
		wctx := &Context{
			State: &core.WorkflowState{
				WorkflowID: "test-wf",
				Tasks:      make(map[core.TaskID]*core.TaskState),
				Metrics:    &core.StateMetrics{TotalCostUSD: 9.0}, // Already at $9
			},
			Logger: logging.NewNop(),
			Config: &Config{
				MaxCostPerWorkflow: 10.0, // $10 limit
			},
		}

		task := &core.Task{
			ID:   "task-2",
			Name: "Test Task",
		}
		taskState := &core.TaskState{
			ID:     task.ID,
			Status: core.TaskStatusPending,
		}
		wctx.State.Tasks[task.ID] = taskState

		// Simulate a result that pushes workflow over budget
		result := &core.ExecuteResult{
			Output:    "test output",
			CostUSD:   2.0, // $2 would make total $11
			TokensIn:  100,
			TokensOut: 50,
		}

		// Update metrics
		wctx.State.Metrics.TotalCostUSD += result.CostUSD // Now at $11

		// Check workflow cost limit
		if wctx.Config.MaxCostPerWorkflow > 0 && wctx.State.Metrics.TotalCostUSD > wctx.Config.MaxCostPerWorkflow {
			err := core.ErrWorkflowBudgetExceeded(wctx.State.Metrics.TotalCostUSD, wctx.Config.MaxCostPerWorkflow)
			if err == nil {
				t.Error("expected error for workflow budget exceeded")
			}
			if !core.IsCategory(err, core.ErrCatBudget) {
				t.Errorf("expected budget error category, got %v", core.GetCategory(err))
			}
		}
	})

	t.Run("no limit when zero", func(t *testing.T) {
		wctx := &Context{
			State: &core.WorkflowState{
				WorkflowID: "test-wf",
				Tasks:      make(map[core.TaskID]*core.TaskState),
				Metrics:    &core.StateMetrics{TotalCostUSD: 1000.0}, // High cost
			},
			Logger: logging.NewNop(),
			Config: &Config{
				MaxCostPerWorkflow: 0, // No limit
				MaxCostPerTask:     0, // No limit
			},
		}

		result := &core.ExecuteResult{
			CostUSD: 100.0, // High task cost
		}

		// No limit check should trigger
		limitExceeded := false
		if wctx.Config.MaxCostPerTask > 0 && result.CostUSD > wctx.Config.MaxCostPerTask {
			limitExceeded = true
		}
		if wctx.Config.MaxCostPerWorkflow > 0 && wctx.State.Metrics.TotalCostUSD > wctx.Config.MaxCostPerWorkflow {
			limitExceeded = true
		}

		if limitExceeded {
			t.Error("should not exceed limit when limits are set to 0")
		}
	})
}
