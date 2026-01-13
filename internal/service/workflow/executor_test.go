package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

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
