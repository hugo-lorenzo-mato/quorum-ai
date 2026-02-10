package cmd

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// newScanner creates a scanner from a string input (simulating stdin).
func newScanner(input string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(input))
}

// --- promptPhaseReview ---

func TestPromptPhaseReview_Enter_Continue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("\n"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_C_Continue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("c\n"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_Feedback(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("f\nConsider OAuth2\n"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "Consider OAuth2" {
		t.Errorf("expected feedback 'Consider OAuth2', got %q", feedback)
	}
}

func TestPromptPhaseReview_Feedback_Empty(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("f\n\n"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_Feedback_EOF(t *testing.T) {
	t.Parallel()
	// After 'f', scanner hits EOF before feedback line
	action, feedback := promptPhaseReview(newScanner("f"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_Rerun(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("r\n"), "analysis")
	if action != "rerun" {
		t.Errorf("expected action 'rerun', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_Abort(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("q\n"), "analysis")
	if action != "abort" {
		t.Errorf("expected action 'abort', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_Unknown_DefaultsContinue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner("xyz\n"), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPhaseReview_EOF_DefaultsContinue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPhaseReview(newScanner(""), "analysis")
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

// --- promptPlanReview ---

func TestPromptPlanReview_Enter_Continue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPlanReview(newScanner("\n"))
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPlanReview_C_Continue(t *testing.T) {
	t.Parallel()
	action, feedback := promptPlanReview(newScanner("c\n"))
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPlanReview_Edit(t *testing.T) {
	t.Parallel()
	action, _ := promptPlanReview(newScanner("e\n"))
	if action != "edit" {
		t.Errorf("expected action 'edit', got %q", action)
	}
}

func TestPromptPlanReview_Replan_WithFeedback(t *testing.T) {
	t.Parallel()
	action, feedback := promptPlanReview(newScanner("r\nAdd more tests\n"))
	if action != "replan" {
		t.Errorf("expected action 'replan', got %q", action)
	}
	if feedback != "Add more tests" {
		t.Errorf("expected feedback 'Add more tests', got %q", feedback)
	}
}

func TestPromptPlanReview_Replan_NoFeedback(t *testing.T) {
	t.Parallel()
	action, feedback := promptPlanReview(newScanner("r\n\n"))
	if action != "replan" {
		t.Errorf("expected action 'replan', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPlanReview_Replan_EOF(t *testing.T) {
	t.Parallel()
	action, feedback := promptPlanReview(newScanner("r"))
	if action != "replan" {
		t.Errorf("expected action 'replan', got %q", action)
	}
	if feedback != "" {
		t.Errorf("expected empty feedback, got %q", feedback)
	}
}

func TestPromptPlanReview_Abort(t *testing.T) {
	t.Parallel()
	action, _ := promptPlanReview(newScanner("q\n"))
	if action != "abort" {
		t.Errorf("expected action 'abort', got %q", action)
	}
}

func TestPromptPlanReview_Unknown_DefaultsContinue(t *testing.T) {
	t.Parallel()
	action, _ := promptPlanReview(newScanner("xyz\n"))
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
}

func TestPromptPlanReview_EOF_DefaultsContinue(t *testing.T) {
	t.Parallel()
	action, _ := promptPlanReview(newScanner(""))
	if action != "continue" {
		t.Errorf("expected action 'continue', got %q", action)
	}
}

// --- editSingleTask ---

func testState() *core.WorkflowState {
	t1 := core.TaskID("task-1")
	t2 := core.TaskID("task-2")
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				t1: {ID: t1, Name: "Setup auth", CLI: "claude", Description: "Setup auth middleware", Phase: core.PhaseExecute, Status: core.TaskStatusPending},
				t2: {ID: t2, Name: "Add tests", CLI: "gemini", Description: "Add integration tests", Phase: core.PhaseExecute, Status: core.TaskStatusPending, Dependencies: []core.TaskID{t1}},
			},
			TaskOrder: []core.TaskID{t1, t2},
		},
	}
}

func TestEditSingleTask_ChangeAll(t *testing.T) {
	t.Parallel()
	state := testState()
	// Change name, description, and agent
	editSingleTask(newScanner("New name\nNew desc\ncodex\n"), state, 0)

	task := state.Tasks["task-1"]
	if task.Name != "New name" {
		t.Errorf("expected name 'New name', got %q", task.Name)
	}
	if task.Description != "New desc" {
		t.Errorf("expected description 'New desc', got %q", task.Description)
	}
	if task.CLI != "codex" {
		t.Errorf("expected CLI 'codex', got %q", task.CLI)
	}
}

func TestEditSingleTask_KeepAll(t *testing.T) {
	t.Parallel()
	state := testState()
	// Press enter for all fields (keep original)
	editSingleTask(newScanner("\n\n\n"), state, 0)

	task := state.Tasks["task-1"]
	if task.Name != "Setup auth" {
		t.Errorf("expected name 'Setup auth', got %q", task.Name)
	}
	if task.Description != "Setup auth middleware" {
		t.Errorf("expected original description, got %q", task.Description)
	}
	if task.CLI != "claude" {
		t.Errorf("expected CLI 'claude', got %q", task.CLI)
	}
}

func TestEditSingleTask_NotFound(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks:     map[core.TaskID]*core.TaskState{},
			TaskOrder: []core.TaskID{"nonexistent"},
		},
	}
	// Should not panic, just print "Task not found."
	editSingleTask(newScanner("\n"), state, 0)
}

func TestEditSingleTask_PartialEdit(t *testing.T) {
	t.Parallel()
	state := testState()
	// Only change the name, keep desc and agent
	editSingleTask(newScanner("Updated name\n\n\n"), state, 0)

	task := state.Tasks["task-1"]
	if task.Name != "Updated name" {
		t.Errorf("expected name 'Updated name', got %q", task.Name)
	}
	if task.CLI != "claude" {
		t.Errorf("expected CLI 'claude', got %q", task.CLI)
	}
}

// --- addTaskInteractive ---

func TestAddTaskInteractive_Success(t *testing.T) {
	t.Parallel()
	state := testState()
	origLen := len(state.TaskOrder)

	addTaskInteractive(newScanner("New task\ncopilot\nSome description\n"), state)

	if len(state.TaskOrder) != origLen+1 {
		t.Errorf("expected %d tasks, got %d", origLen+1, len(state.TaskOrder))
	}
	newID := state.TaskOrder[len(state.TaskOrder)-1]
	task := state.Tasks[newID]
	if task.Name != "New task" {
		t.Errorf("expected name 'New task', got %q", task.Name)
	}
	if task.CLI != "copilot" {
		t.Errorf("expected CLI 'copilot', got %q", task.CLI)
	}
	if task.Description != "Some description" {
		t.Errorf("expected description 'Some description', got %q", task.Description)
	}
	if task.Status != core.TaskStatusPending {
		t.Errorf("expected status 'pending', got %q", task.Status)
	}
	if task.Phase != core.PhaseExecute {
		t.Errorf("expected phase 'execute', got %q", task.Phase)
	}
}

func TestAddTaskInteractive_EmptyName(t *testing.T) {
	t.Parallel()
	state := testState()
	origLen := len(state.TaskOrder)

	addTaskInteractive(newScanner("\n"), state)

	if len(state.TaskOrder) != origLen {
		t.Errorf("expected no new task, got %d tasks", len(state.TaskOrder))
	}
}

func TestAddTaskInteractive_EmptyAgent(t *testing.T) {
	t.Parallel()
	state := testState()
	origLen := len(state.TaskOrder)

	addTaskInteractive(newScanner("New task\n\n"), state)

	if len(state.TaskOrder) != origLen {
		t.Errorf("expected no new task, got %d tasks", len(state.TaskOrder))
	}
}

func TestAddTaskInteractive_EOF_NoName(t *testing.T) {
	t.Parallel()
	state := testState()
	origLen := len(state.TaskOrder)

	addTaskInteractive(newScanner(""), state)

	if len(state.TaskOrder) != origLen {
		t.Errorf("expected no new task, got %d tasks", len(state.TaskOrder))
	}
}

func TestAddTaskInteractive_EOF_NoAgent(t *testing.T) {
	t.Parallel()
	state := testState()
	origLen := len(state.TaskOrder)

	addTaskInteractive(newScanner("New task"), state)

	if len(state.TaskOrder) != origLen {
		t.Errorf("expected no new task, got %d tasks", len(state.TaskOrder))
	}
}

func TestAddTaskInteractive_NilTasks(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks:     nil,
			TaskOrder: nil,
		},
	}

	addTaskInteractive(newScanner("First task\nclaude\nFirst task desc\n"), state)

	if len(state.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.Tasks))
	}
	if len(state.TaskOrder) != 1 {
		t.Errorf("expected 1 task in order, got %d", len(state.TaskOrder))
	}
}

// --- deleteTaskInteractive ---

func TestDeleteTaskInteractive_Success(t *testing.T) {
	t.Parallel()
	state := testState()
	// Remove the dependency so task-1 can be deleted
	state.Tasks["task-2"].Dependencies = nil

	deleteTaskInteractive(newScanner("1\n"), state)

	if len(state.TaskOrder) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.TaskOrder))
	}
	if _, ok := state.Tasks["task-1"]; ok {
		t.Error("expected task-1 to be deleted")
	}
}

func TestDeleteTaskInteractive_BlockedByDependency(t *testing.T) {
	t.Parallel()
	state := testState()
	// task-2 depends on task-1, so task-1 can't be deleted

	deleteTaskInteractive(newScanner("1\n"), state)

	// Task should NOT be deleted
	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks (blocked), got %d", len(state.TaskOrder))
	}
	if _, ok := state.Tasks["task-1"]; !ok {
		t.Error("task-1 should not have been deleted (dependency block)")
	}
}

func TestDeleteTaskInteractive_InvalidNumber(t *testing.T) {
	t.Parallel()
	state := testState()

	deleteTaskInteractive(newScanner("0\n"), state)

	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks (invalid number), got %d", len(state.TaskOrder))
	}
}

func TestDeleteTaskInteractive_TooHigh(t *testing.T) {
	t.Parallel()
	state := testState()

	deleteTaskInteractive(newScanner("99\n"), state)

	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks (too high), got %d", len(state.TaskOrder))
	}
}

func TestDeleteTaskInteractive_NotANumber(t *testing.T) {
	t.Parallel()
	state := testState()

	deleteTaskInteractive(newScanner("abc\n"), state)

	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks (NaN), got %d", len(state.TaskOrder))
	}
}

func TestDeleteTaskInteractive_EOF(t *testing.T) {
	t.Parallel()
	state := testState()

	deleteTaskInteractive(newScanner(""), state)

	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks (EOF), got %d", len(state.TaskOrder))
	}
}

func TestDeleteTaskInteractive_LastTask(t *testing.T) {
	t.Parallel()
	state := testState()

	// Delete task-2 (no one depends on it)
	deleteTaskInteractive(newScanner("2\n"), state)

	if len(state.TaskOrder) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.TaskOrder))
	}
	if _, ok := state.Tasks["task-2"]; ok {
		t.Error("expected task-2 to be deleted")
	}
}

// --- editTasksInteractive ---

func TestEditTasksInteractive_Done(t *testing.T) {
	t.Parallel()
	state := testState()
	// Just press Enter (done)
	editTasksInteractive(newScanner("\n"), state)
	// No changes
	if state.Tasks["task-1"].Name != "Setup auth" {
		t.Error("expected no changes")
	}
}

func TestEditTasksInteractive_EditTask(t *testing.T) {
	t.Parallel()
	state := testState()
	// Edit task 1, change name, then done
	editTasksInteractive(newScanner("1\nNew name\n\n\n\n"), state)
	if state.Tasks["task-1"].Name != "New name" {
		t.Errorf("expected name 'New name', got %q", state.Tasks["task-1"].Name)
	}
}

func TestEditTasksInteractive_AddTask(t *testing.T) {
	t.Parallel()
	state := testState()
	editTasksInteractive(newScanner("a\nNew task\nclaude\nDesc\n\n"), state)
	if len(state.TaskOrder) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(state.TaskOrder))
	}
}

func TestEditTasksInteractive_DeleteTask(t *testing.T) {
	t.Parallel()
	state := testState()
	editTasksInteractive(newScanner("d\n2\n\n"), state)
	if len(state.TaskOrder) != 1 {
		t.Errorf("expected 1 task, got %d", len(state.TaskOrder))
	}
}

func TestEditTasksInteractive_InvalidInput(t *testing.T) {
	t.Parallel()
	state := testState()
	// Invalid input, then EOF
	editTasksInteractive(newScanner("xyz\n"), state)
	// No changes
	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(state.TaskOrder))
	}
}

func TestEditTasksInteractive_EOF(t *testing.T) {
	t.Parallel()
	state := testState()
	editTasksInteractive(newScanner(""), state)
	// No changes
	if len(state.TaskOrder) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(state.TaskOrder))
	}
}

// --- displayTaskPlan ---

func TestDisplayTaskPlan_Basic(t *testing.T) {
	t.Parallel()
	state := testState()
	// Should not panic
	displayTaskPlan(state)
}

func TestDisplayTaskPlan_Empty(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks:     map[core.TaskID]*core.TaskState{},
			TaskOrder: nil,
		},
	}
	// Should not panic
	displayTaskPlan(state)
}

func TestDisplayTaskPlan_MissingTask(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks:     map[core.TaskID]*core.TaskState{},
			TaskOrder: []core.TaskID{"nonexistent"},
		},
	}
	// Should not panic (skips missing tasks)
	displayTaskPlan(state)
}

func TestDisplayTaskPlan_WithDependencies(t *testing.T) {
	t.Parallel()
	state := testState()
	// task-2 depends on task-1, should print "(depends: 1)"
	displayTaskPlan(state)
}

// --- displayTruncated ---

func TestDisplayTruncated_ShortText(t *testing.T) {
	t.Parallel()
	// Should not panic - prints all lines
	displayTruncated("line1\nline2\nline3", 10)
}

func TestDisplayTruncated_ExactLimit(t *testing.T) {
	t.Parallel()
	// 3 lines, limit 3 - should print all
	displayTruncated("line1\nline2\nline3", 3)
}

func TestDisplayTruncated_Exceeds(t *testing.T) {
	t.Parallel()
	// 5 lines, limit 2 - should truncate
	displayTruncated("line1\nline2\nline3\nline4\nline5", 2)
}

func TestDisplayTruncated_Empty(t *testing.T) {
	t.Parallel()
	displayTruncated("", 10)
}

func TestDisplayTruncated_SingleLine(t *testing.T) {
	t.Parallel()
	displayTruncated("hello world", 5)
}

// --- runInteractiveWorkflow error paths ---

func TestRunInteractiveWorkflow_NoPrompt(t *testing.T) {
	// No t.Parallel(): mutates package-level runFile
	origFile := runFile
	defer func() { runFile = origFile }()
	runFile = ""

	err := runInteractiveWorkflow(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing prompt")
	}
}

func TestRunInteractiveWorkflow_EmptyArgsAndNoFile(t *testing.T) {
	// No t.Parallel(): mutates package-level runFile
	origFile := runFile
	defer func() { runFile = origFile }()
	runFile = ""

	err := runInteractiveWorkflow(context.Background(), []string{})
	if err == nil {
		t.Error("expected error for empty args and no file")
	}
}

func TestRunInteractiveWorkflow_NonexistentFile(t *testing.T) {
	// No t.Parallel(): mutates package-level runFile
	origFile := runFile
	defer func() { runFile = origFile }()
	runFile = "/nonexistent/path/to/prompt.txt"

	err := runInteractiveWorkflow(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRunInteractiveWorkflow_InitPhaseRunnerError(t *testing.T) {
	// No t.Parallel(): reads package-level runFile which other tests mutate
	// Provide a prompt but use a cancelled context so InitPhaseRunner fails
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runInteractiveWorkflow(ctx, []string{"test prompt"})
	if err == nil {
		t.Error("expected error from InitPhaseRunner or downstream")
	}
}
