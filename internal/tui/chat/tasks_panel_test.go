package chat

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestNewTasksPanel(t *testing.T) {
	panel := NewTasksPanel()
	if panel == nil {
		t.Fatal("NewTasksPanel() returned nil")
	}
	if panel.visible {
		t.Error("New panel should not be visible")
	}
	if panel.HasTasks() {
		t.Error("New panel should have no tasks")
	}
}

func TestTasksPanel_Toggle(t *testing.T) {
	panel := NewTasksPanel()

	if panel.IsVisible() {
		t.Error("Panel should start hidden")
	}

	panel.Toggle()
	if !panel.IsVisible() {
		t.Error("Panel should be visible after toggle")
	}

	panel.Toggle()
	if panel.IsVisible() {
		t.Error("Panel should be hidden after second toggle")
	}
}

func TestTasksPanel_SetState(t *testing.T) {
	panel := NewTasksPanel()

	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task One", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Name: "Task Two", Status: core.TaskStatusRunning},
			"task-3": {ID: "task-3", Name: "Task Three", Status: core.TaskStatusPending},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2", "task-3"},
	}

	panel.SetState(state)

	if !panel.HasTasks() {
		t.Error("Panel should have tasks after SetState")
	}

	completed, running, pending, failed, skipped, total := panel.taskStats()
	if total != 3 {
		t.Errorf("Expected 3 total tasks, got %d", total)
	}
	if completed != 1 {
		t.Errorf("Expected 1 completed task, got %d", completed)
	}
	if running != 1 {
		t.Errorf("Expected 1 running task, got %d", running)
	}
	if pending != 1 {
		t.Errorf("Expected 1 pending task, got %d", pending)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed tasks, got %d", failed)
	}
	if skipped != 0 {
		t.Errorf("Expected 0 skipped tasks, got %d", skipped)
	}
}

func TestTasksPanel_Render(t *testing.T) {
	panel := NewTasksPanel()
	panel.SetSize(80, 30)

	// Hidden panel returns empty
	if panel.Render() != "" {
		t.Error("Hidden panel should render empty string")
	}

	panel.Toggle()

	// Visible but empty
	rendered := panel.Render()
	if rendered == "" {
		t.Error("Visible panel should render something")
	}
	if !contains(rendered, "No issues yet") {
		t.Error("Empty panel should show 'No issues yet'")
	}

	// With tasks
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Test Task", Status: core.TaskStatusCompleted},
		},
		TaskOrder: []core.TaskID{"task-1"},
	}
	panel.SetState(state)

	rendered = panel.Render()
	if !contains(rendered, "Test Task") {
		t.Error("Panel should show task name")
	}
	if !contains(rendered, "✓") {
		t.Error("Completed task should show checkmark")
	}
}

func TestTasksPanel_CompactRender(t *testing.T) {
	panel := NewTasksPanel()

	// No tasks - empty
	if panel.CompactRender() != "" {
		t.Error("CompactRender should be empty with no tasks")
	}

	// With tasks
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task One", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Name: "Task Two", Status: core.TaskStatusPending},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2"},
	}
	panel.SetState(state)

	compact := panel.CompactRender()
	if compact == "" {
		t.Error("CompactRender should return something with tasks")
	}
	if !contains(compact, "1/2") {
		t.Error("CompactRender should show progress (1/2)")
	}
}

func TestTasksPanel_Scroll(t *testing.T) {
	panel := NewTasksPanel()
	panel.SetSize(80, 20)
	panel.maxTasks = 2 // Force small view

	// Create many tasks
	state := &core.WorkflowState{
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Name: "Task 1", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Name: "Task 2", Status: core.TaskStatusCompleted},
			"task-3": {ID: "task-3", Name: "Task 3", Status: core.TaskStatusPending},
			"task-4": {ID: "task-4", Name: "Task 4", Status: core.TaskStatusPending},
		},
		TaskOrder: []core.TaskID{"task-1", "task-2", "task-3", "task-4"},
	}
	panel.SetState(state)

	// Initial scroll
	if panel.scrollY != 0 {
		t.Error("Initial scroll should be 0")
	}

	// Scroll down
	panel.ScrollDown()
	if panel.scrollY != 1 {
		t.Errorf("Expected scrollY=1, got %d", panel.scrollY)
	}

	// Scroll up
	panel.ScrollUp()
	if panel.scrollY != 0 {
		t.Errorf("Expected scrollY=0, got %d", panel.scrollY)
	}

	// Can't scroll past 0
	panel.ScrollUp()
	if panel.scrollY != 0 {
		t.Error("scrollY should not go negative")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
