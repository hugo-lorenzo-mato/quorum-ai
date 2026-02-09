package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestModel_PhaseUpdateMsg(t *testing.T) {
	t.Parallel()
	model := New()

	updated, _ := model.Update(PhaseUpdateMsg{Phase: core.PhaseAnalyze})
	m := updated.(Model)

	if m.currentPhase != core.PhaseAnalyze {
		t.Errorf("expected phase analyze, got %v", m.currentPhase)
	}
}

func TestRenderHeader_ShowsCurrentPhase(t *testing.T) {
	t.Parallel()
	model := Model{
		currentPhase: core.PhaseExecute,
		workflow:     &core.WorkflowState{WorkflowRun: core.WorkflowRun{Status: core.WorkflowStatusRunning}},
		width:        80,
	}

	header := model.renderHeader()

	if !strings.Contains(header, "execute") {
		t.Errorf("header should show execute phase, got: %s", header)
	}
	if !strings.Contains(header, "running") {
		t.Errorf("header should show running status, got: %s", header)
	}
}

func TestRenderHeader_NoWorkflow(t *testing.T) {
	t.Parallel()
	model := Model{
		currentPhase: core.PhaseRefine,
		workflow:     nil,
		width:        80,
	}

	header := model.renderHeader()

	if !strings.Contains(header, "refine") {
		t.Errorf("header should show refine phase, got: %s", header)
	}
}

func TestRenderProgress_CountsAllTerminalStates(t *testing.T) {
	t.Parallel()
	model := Model{
		workflow: &core.WorkflowState{},
		tasks: []*TaskView{
			{ID: "1", Status: core.TaskStatusCompleted},
			{ID: "2", Status: core.TaskStatusCompleted},
			{ID: "3", Status: core.TaskStatusFailed},
			{ID: "4", Status: core.TaskStatusSkipped},
			{ID: "5", Status: core.TaskStatusPending},
		},
		width: 80,
	}

	output := model.renderProgress()

	// Should show 80% (4 finished out of 5)
	if !strings.Contains(output, "80.0%") {
		t.Errorf("Expected 80%% progress, got: %s", output)
	}

	// Should mention failed and skipped
	if !strings.Contains(output, "failed") || !strings.Contains(output, "skipped") {
		t.Errorf("Expected failed/skipped breakdown, got: %s", output)
	}
}

func TestRenderProgress_100PercentWithFailures(t *testing.T) {
	t.Parallel()
	model := Model{
		workflow: &core.WorkflowState{},
		tasks: []*TaskView{
			{ID: "1", Status: core.TaskStatusCompleted},
			{ID: "2", Status: core.TaskStatusFailed},
		},
		width: 80,
	}

	output := model.renderProgress()

	// Should show 100% even with failure
	if !strings.Contains(output, "100.0%") {
		t.Errorf("Expected 100%% when all tasks finished, got: %s", output)
	}
}

func TestRenderProgress_NoBreakdownForCleanRun(t *testing.T) {
	t.Parallel()
	model := Model{
		workflow: &core.WorkflowState{},
		tasks: []*TaskView{
			{ID: "1", Status: core.TaskStatusCompleted},
			{ID: "2", Status: core.TaskStatusCompleted},
			{ID: "3", Status: core.TaskStatusCompleted},
		},
		width: 80,
	}

	output := model.renderProgress()

	// Should NOT mention failed/skipped for clean runs
	if strings.Contains(output, "failed") || strings.Contains(output, "skipped") {
		t.Errorf("Clean run shouldn't show failed/skipped, got: %s", output)
	}
}

func TestProgressStats_Percentage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		stats    progressStats
		expected float64
	}{
		{progressStats{total: 10, completed: 5, failed: 0, skipped: 0}, 50.0},
		{progressStats{total: 10, completed: 5, failed: 3, skipped: 2}, 100.0},
		{progressStats{total: 4, completed: 1, failed: 1, skipped: 1}, 75.0},
		{progressStats{total: 0, completed: 0, failed: 0, skipped: 0}, 0.0},
	}

	for _, tc := range tests {
		result := tc.stats.percentage()
		if result != tc.expected {
			t.Errorf("Expected %.1f%%, got %.1f%%", tc.expected, result)
		}
	}
}

func TestBuildTaskViews_UsesTaskOrder(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-3": {ID: "task-3", Name: "Third"},
				"task-1": {ID: "task-1", Name: "First"},
				"task-2": {ID: "task-2", Name: "Second"},
			},
			TaskOrder: []core.TaskID{"task-1", "task-2", "task-3"},
		},
	}

	model := New()
	views := model.buildTaskViews(state)

	if len(views) != 3 {
		t.Fatalf("Expected 3 views, got %d", len(views))
	}

	// Verify order matches TaskOrder
	expected := []core.TaskID{"task-1", "task-2", "task-3"}
	for i, view := range views {
		if view.ID != expected[i] {
			t.Errorf("View %d: expected %s, got %s", i, expected[i], view.ID)
		}
	}
}

func TestBuildTaskViews_StableAcrossRenders(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a", Name: "Task A"},
				"b": {ID: "b", Name: "Task B"},
				"c": {ID: "c", Name: "Task C"},
				"d": {ID: "d", Name: "Task D"},
				"e": {ID: "e", Name: "Task E"},
			},
			TaskOrder: []core.TaskID{"a", "b", "c", "d", "e"},
		},
	}

	model := New()

	// Render 100 times and verify order is always the same
	for i := 0; i < 100; i++ {
		views := model.buildTaskViews(state)
		for j, view := range views {
			expected := state.TaskOrder[j]
			if view.ID != expected {
				t.Errorf("Render %d, position %d: expected %s, got %s", i, j, expected, view.ID)
			}
		}
	}
}

func TestBuildTaskViews_HandlesEmptyTaskOrder(t *testing.T) {
	t.Parallel()
	// Old state file without TaskOrder
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Name: "Task 1"},
			},
			TaskOrder: nil, // Empty
		},
	}

	model := New()
	views := model.buildTaskViews(state)

	// Should still return the task
	if len(views) != 1 {
		t.Errorf("Expected 1 view for fallback case, got %d", len(views))
	}
}

func TestBuildTaskViews_HandlesMissingTask(t *testing.T) {
	t.Parallel()
	// TaskOrder references a task that doesn't exist
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Name: "Task 1"},
			},
			TaskOrder: []core.TaskID{"task-1", "task-missing"},
		},
	}

	model := New()
	views := model.buildTaskViews(state)

	// Should skip missing task
	if len(views) != 1 {
		t.Errorf("Expected 1 view (missing skipped), got %d", len(views))
	}
	if views[0].ID != "task-1" {
		t.Errorf("Expected task-1, got %s", views[0].ID)
	}
}

func TestGetTaskDuration_LiveForRunning(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-5 * time.Second)
	task := &TaskView{
		ID:        "task-1",
		Status:    core.TaskStatusRunning,
		StartedAt: &started,
	}

	model := Model{}
	duration := model.getTaskDuration(task)

	if duration < 4*time.Second || duration > 6*time.Second {
		t.Errorf("Expected ~5s, got %v", duration)
	}
}

func TestGetTaskDuration_StaticForCompleted(t *testing.T) {
	t.Parallel()
	task := &TaskView{
		ID:       "task-1",
		Status:   core.TaskStatusCompleted,
		Duration: 30 * time.Second,
	}

	model := Model{}
	duration := model.getTaskDuration(task)

	if duration != 30*time.Second {
		t.Errorf("Expected 30s for completed task, got %v", duration)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m30s"},
		{5*time.Minute + 30*time.Second, "5m30s"},
		{1*time.Hour + 15*time.Minute, "1h15m"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.input)
		if result != tc.expected {
			t.Errorf("formatDuration(%v) = %s, want %s", tc.input, result, tc.expected)
		}
	}
}
