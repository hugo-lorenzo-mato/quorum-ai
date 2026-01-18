package tui

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestModel_PhaseUpdateMsg(t *testing.T) {
	model := New()

	updated, _ := model.Update(PhaseUpdateMsg{Phase: core.PhaseAnalyze})
	m := updated.(Model)

	if m.currentPhase != core.PhaseAnalyze {
		t.Errorf("expected phase analyze, got %v", m.currentPhase)
	}
}

func TestRenderHeader_ShowsCurrentPhase(t *testing.T) {
	model := Model{
		currentPhase: core.PhaseExecute,
		workflow:     &core.WorkflowState{Status: core.WorkflowStatusRunning},
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
	model := Model{
		currentPhase: core.PhaseOptimize,
		workflow:     nil,
		width:        80,
	}

	header := model.renderHeader()

	if !strings.Contains(header, "optimize") {
		t.Errorf("header should show optimize phase, got: %s", header)
	}
}

func TestRenderProgress_CountsAllTerminalStates(t *testing.T) {
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
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-3": {ID: "task-3", Name: "Third"},
			"task-1": {ID: "task-1", Name: "First"},
			"task-2": {ID: "task-2", Name: "Second"},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2", "task-3"},
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
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"a": {ID: "a", Name: "Task A"},
			"b": {ID: "b", Name: "Task B"},
			"c": {ID: "c", Name: "Task C"},
			"d": {ID: "d", Name: "Task D"},
			"e": {ID: "e", Name: "Task E"},
		},
		TaskOrder: []core.TaskID{"a", "b", "c", "d", "e"},
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
	// Old state file without TaskOrder
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task 1"},
		},
		TaskOrder: nil, // Empty
	}

	model := New()
	views := model.buildTaskViews(state)

	// Should still return the task
	if len(views) != 1 {
		t.Errorf("Expected 1 view for fallback case, got %d", len(views))
	}
}

func TestBuildTaskViews_HandlesMissingTask(t *testing.T) {
	// TaskOrder references a task that doesn't exist
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task 1"},
		},
		TaskOrder: []core.TaskID{"task-1", "task-missing"},
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
