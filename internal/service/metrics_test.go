package service_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestMetricsCollector_Workflow(t *testing.T) {
	collector := service.NewMetricsCollector()

	collector.StartWorkflow()
	time.Sleep(10 * time.Millisecond)
	collector.EndWorkflow()

	metrics := collector.GetWorkflowMetrics()
	testutil.AssertTrue(t, metrics.TotalDuration > 0, "duration should be positive")
	testutil.AssertTrue(t, !metrics.StartTime.IsZero(), "start time should be set")
	testutil.AssertTrue(t, !metrics.EndTime.IsZero(), "end time should be set")
}

func TestMetricsCollector_TaskTracking(t *testing.T) {
	collector := service.NewMetricsCollector()

	task := &core.Task{
		ID:    "task-1",
		Name:  "Test Task",
		Phase: core.PhaseAnalyze,
	}

	collector.StartTask(task, "claude")

	result := &core.ExecuteResult{
		Output:    "test output",
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.01,
	}

	collector.EndTask(task.ID, result, nil)

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.TasksTotal, 1)
	testutil.AssertEqual(t, wm.TasksCompleted, 1)
	testutil.AssertEqual(t, wm.TotalTokensIn, 100)
	testutil.AssertEqual(t, wm.TotalTokensOut, 50)
	testutil.AssertEqual(t, wm.TotalCostUSD, 0.01)
}

func TestMetricsCollector_TaskFailed(t *testing.T) {
	collector := service.NewMetricsCollector()

	task := &core.Task{
		ID:    "task-1",
		Name:  "Failing Task",
		Phase: core.PhaseAnalyze,
	}

	collector.StartTask(task, "claude")
	collector.EndTask(task.ID, nil, testutil.ErrTest)

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.TasksFailed, 1)
	testutil.AssertEqual(t, wm.TasksCompleted, 0)

	tm, ok := collector.GetTaskMetrics(task.ID)
	testutil.AssertTrue(t, ok, "task metrics should exist")
	testutil.AssertFalse(t, tm.Success, "task should not be successful")
}

func TestMetricsCollector_Retries(t *testing.T) {
	collector := service.NewMetricsCollector()

	task := &core.Task{ID: "task-1", Name: "Retry Task"}
	collector.StartTask(task, "claude")

	collector.RecordRetry(task.ID)
	collector.RecordRetry(task.ID)

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.RetriesTotal, 2)

	tm, _ := collector.GetTaskMetrics(task.ID)
	testutil.AssertEqual(t, tm.Retries, 2)
}

func TestMetricsCollector_AgentMetrics(t *testing.T) {
	collector := service.NewMetricsCollector()

	// Multiple tasks for same agent
	for i := 0; i < 3; i++ {
		task := &core.Task{ID: core.TaskID(string(rune('a' + i))), Name: "Task"}
		collector.StartTask(task, "claude")
		collector.EndTask(task.ID, &core.ExecuteResult{
			TokensIn:  100,
			TokensOut: 50,
			CostUSD:   0.01,
		}, nil)
	}

	agents := collector.GetAgentMetrics()
	testutil.AssertEqual(t, len(agents), 1)

	claude := agents["claude"]
	testutil.AssertEqual(t, claude.Invocations, 3)
	testutil.AssertEqual(t, claude.TotalTokensIn, 300)
	testutil.AssertEqual(t, claude.TotalCostUSD, 0.03)
}

func TestMetricsCollector_Consensus(t *testing.T) {
	collector := service.NewMetricsCollector()

	result := service.ConsensusResult{
		Score:   0.85,
		NeedsV3: true,
		CategoryScores: map[string]float64{
			"claims":          0.9,
			"risks":           0.8,
			"recommendations": 0.85,
		},
		Divergences: []service.Divergence{
			{Category: "claims", JaccardScore: 0.5},
		},
	}

	collector.RecordConsensus(result, core.PhaseAnalyze)

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.V3Invocations, 1)

	consensus := collector.GetConsensusMetrics()
	testutil.AssertLen(t, consensus, 1)
	testutil.AssertEqual(t, consensus[0].Score, 0.85)
	testutil.AssertEqual(t, consensus[0].V3Required, true)
}

func TestMetricsCollector_Skipped(t *testing.T) {
	collector := service.NewMetricsCollector()

	collector.RecordSkipped("task-1")

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.TasksSkipped, 1)
}

func TestMetricsCollector_Reset(t *testing.T) {
	collector := service.NewMetricsCollector()

	task := &core.Task{ID: "task-1", Name: "Task"}
	collector.StartTask(task, "claude")
	collector.EndTask(task.ID, &core.ExecuteResult{TokensIn: 100}, nil)

	collector.Reset()

	wm := collector.GetWorkflowMetrics()
	testutil.AssertEqual(t, wm.TasksTotal, 0)
	testutil.AssertEqual(t, wm.TotalTokensIn, 0)
}

func TestReportGenerator_TextReport(t *testing.T) {
	collector := service.NewMetricsCollector()
	collector.StartWorkflow()

	task := &core.Task{ID: "task-1", Name: "Test Task", Phase: core.PhaseAnalyze}
	collector.StartTask(task, "claude")
	collector.EndTask(task.ID, &core.ExecuteResult{
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.01,
	}, nil)

	collector.EndWorkflow()

	generator := service.NewReportGenerator(collector)

	var buf bytes.Buffer
	err := generator.GenerateTextReport(&buf)
	testutil.AssertNoError(t, err)

	report := buf.String()
	testutil.AssertContains(t, report, "WORKFLOW REPORT")
	testutil.AssertContains(t, report, "SUMMARY")
	testutil.AssertContains(t, report, "TOKEN USAGE")
	testutil.AssertContains(t, report, "COST")
}

func TestReportGenerator_JSONReport(t *testing.T) {
	collector := service.NewMetricsCollector()
	collector.StartWorkflow()

	task := &core.Task{ID: "task-1", Name: "Test Task", Phase: core.PhaseAnalyze}
	collector.StartTask(task, "claude")
	collector.EndTask(task.ID, &core.ExecuteResult{
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.01,
	}, nil)

	collector.EndWorkflow()

	generator := service.NewReportGenerator(collector)

	var buf bytes.Buffer
	err := generator.GenerateJSONReport(&buf)
	testutil.AssertNoError(t, err)

	report := buf.String()
	testutil.AssertContains(t, report, "generated_at")
	testutil.AssertContains(t, report, "workflow")
	testutil.AssertContains(t, report, "agents")
}

func TestReportGenerator_Summary(t *testing.T) {
	collector := service.NewMetricsCollector()
	collector.StartWorkflow()

	task := &core.Task{ID: "task-1", Name: "Test Task"}
	collector.StartTask(task, "claude")
	collector.EndTask(task.ID, &core.ExecuteResult{CostUSD: 0.01}, nil)

	collector.EndWorkflow()

	generator := service.NewReportGenerator(collector)
	summary := generator.GenerateSummary()

	testutil.AssertContains(t, summary, "Duration:")
	testutil.AssertContains(t, summary, "Tasks: 1/1")
	testutil.AssertContains(t, summary, "Cost:")
}
