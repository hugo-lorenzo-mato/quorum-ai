package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui/components"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNew_DefaultState(t *testing.T) {
	t.Parallel()
	m := New()

	if m.tasks == nil {
		t.Error("tasks slice should be initialized")
	}
	if m.logs == nil {
		t.Error("logs slice should be initialized")
	}
	if !m.showSidebar {
		t.Error("showSidebar should default to true")
	}
	if len(m.agents) != 3 {
		t.Errorf("expected 3 default agents, got %d", len(m.agents))
	}
	if len(m.agentMap) != 3 {
		t.Errorf("expected 3 entries in agentMap, got %d", len(m.agentMap))
	}
	for _, id := range []string{"claude", "gemini", "codex"} {
		if _, ok := m.agentMap[id]; !ok {
			t.Errorf("agentMap missing %q", id)
		}
	}
}

func TestNewWithEventBus_NilBus(t *testing.T) {
	t.Parallel()
	m := NewWithEventBus(nil)
	if m.eventAdapter != nil {
		t.Error("eventAdapter should be nil when bus is nil")
	}
}

func TestNewWithControlPlane(t *testing.T) {
	t.Parallel()
	// Pass nil to avoid needing a real ControlPlane
	m := NewWithControlPlane(nil)
	if m.controlPlane != nil {
		t.Error("controlPlane should be nil when passed nil")
	}
}

func TestSetControlPlane(t *testing.T) {
	t.Parallel()
	m := New()
	if m.controlPlane != nil {
		t.Error("controlPlane should start nil")
	}
	// We cannot easily construct a real ControlPlane without the control
	// package helper, but we can at least verify the setter works with nil.
	m.SetControlPlane(nil)
	if m.controlPlane != nil {
		t.Error("controlPlane should still be nil after setting nil")
	}
}

// ---------------------------------------------------------------------------
// Init tests
// ---------------------------------------------------------------------------

func TestInit_ReturnsCmd(t *testing.T) {
	t.Parallel()
	m := New()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd (tea.Batch)")
	}
}

// ---------------------------------------------------------------------------
// Update: WindowSizeMsg
// ---------------------------------------------------------------------------

func TestUpdate_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)

	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 40 {
		t.Errorf("height = %d, want 40", model.height)
	}
	if !model.ready {
		t.Error("model should be ready after WindowSizeMsg")
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should not produce a command")
	}
}

// ---------------------------------------------------------------------------
// Update: WorkflowUpdateMsg
// ---------------------------------------------------------------------------

func TestUpdate_WorkflowUpdateMsg_NoAdapter(t *testing.T) {
	t.Parallel()
	m := New()

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Name: "Task 1", Status: core.TaskStatusPending},
			},
			TaskOrder: []core.TaskID{"t1"},
		},
	}

	updated, cmd := m.Update(WorkflowUpdateMsg{State: state})
	model := updated.(Model)

	if model.workflow != state {
		t.Error("workflow should be set from message")
	}
	if len(model.tasks) != 1 {
		t.Errorf("expected 1 task view, got %d", len(model.tasks))
	}
	if cmd == nil {
		t.Error("should return a waitForWorkflowUpdate command")
	}
}

// ---------------------------------------------------------------------------
// Update: TaskUpdateMsg
// ---------------------------------------------------------------------------

func TestUpdate_TaskUpdateMsg(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Name: "Task 1", Status: core.TaskStatusRunning},
		{ID: "t2", Name: "Task 2", Status: core.TaskStatusPending},
	}

	updated, _ := m.Update(TaskUpdateMsg{
		TaskID:   "t1",
		Status:   core.TaskStatusCompleted,
		Progress: 1.0,
		Error:    "",
	})
	model := updated.(Model)

	if model.tasks[0].Status != core.TaskStatusCompleted {
		t.Errorf("task t1 status = %s, want completed", model.tasks[0].Status)
	}
	if model.tasks[0].Progress != 1.0 {
		t.Errorf("task t1 progress = %f, want 1.0", model.tasks[0].Progress)
	}
}

func TestUpdate_TaskUpdateMsg_NotFound(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Name: "Task 1", Status: core.TaskStatusPending},
	}

	updated, _ := m.Update(TaskUpdateMsg{
		TaskID: "nonexistent",
		Status: core.TaskStatusFailed,
	})
	model := updated.(Model)

	// Original task should be unchanged
	if model.tasks[0].Status != core.TaskStatusPending {
		t.Error("existing task should be unmodified when updating a nonexistent task")
	}
}

// ---------------------------------------------------------------------------
// Update: LogMsg
// ---------------------------------------------------------------------------

func TestUpdate_LogMsg(t *testing.T) {
	t.Parallel()
	m := New()
	now := time.Now()

	updated, _ := m.Update(LogMsg{Time: now, Level: "info", Message: "test log"})
	model := updated.(Model)

	if len(model.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(model.logs))
	}
	if model.logs[0].Message != "test log" {
		t.Errorf("log message = %q, want %q", model.logs[0].Message, "test log")
	}
}

func TestUpdate_LogMsg_TruncatesAt100(t *testing.T) {
	t.Parallel()
	m := New()

	// Fill with 100 logs
	for i := 0; i < 100; i++ {
		result, _ := m.Update(LogMsg{Time: time.Now(), Level: "info", Message: "msg"})
		m = result.(Model)
	}
	if len(m.logs) != 100 {
		t.Fatalf("expected 100 logs, got %d", len(m.logs))
	}

	// Adding one more should still keep 100
	result, _ := m.Update(LogMsg{Time: time.Now(), Level: "info", Message: "overflow"})
	m = result.(Model)
	if len(m.logs) != 100 {
		t.Errorf("expected 100 logs after overflow, got %d", len(m.logs))
	}
	// Last log should be the newest
	if m.logs[99].Message != "overflow" {
		t.Errorf("last log = %q, want %q", m.logs[99].Message, "overflow")
	}
}

// ---------------------------------------------------------------------------
// Update: MetricsUpdateMsg
// ---------------------------------------------------------------------------

func TestUpdate_MetricsUpdateMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(MetricsUpdateMsg{TotalTokensIn: 100, TotalTokensOut: 50})
	_ = updated.(Model)
	// Without eventAdapter, cmd should be nil (no re-subscribe)
	if cmd != nil {
		t.Error("MetricsUpdateMsg without adapter should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: DroppedEventsMsg
// ---------------------------------------------------------------------------

func TestUpdate_DroppedEventsMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(DroppedEventsMsg{Count: 42})
	model := updated.(Model)

	if model.droppedEvents != 42 {
		t.Errorf("droppedEvents = %d, want 42", model.droppedEvents)
	}
	if cmd != nil {
		t.Error("DroppedEventsMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: ErrorMsg
// ---------------------------------------------------------------------------

func TestUpdate_ErrorMsg(t *testing.T) {
	t.Parallel()
	m := New()
	testErr := errors.New("something went wrong")

	updated, cmd := m.Update(ErrorMsg{Error: testErr})
	model := updated.(Model)

	if model.err != testErr {
		t.Errorf("err = %v, want %v", model.err, testErr)
	}
	if cmd != nil {
		t.Error("ErrorMsg should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// Update: QuitMsg
// ---------------------------------------------------------------------------

func TestUpdate_QuitMsg(t *testing.T) {
	t.Parallel()
	m := New()
	_, cmd := m.Update(QuitMsg{})
	if cmd == nil {
		t.Error("QuitMsg should produce tea.Quit command")
	}
}

// ---------------------------------------------------------------------------
// Update: PausedMsg / ResumedMsg
// ---------------------------------------------------------------------------

func TestUpdate_PausedMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, _ := m.Update(PausedMsg{})
	model := updated.(Model)
	if !model.isPaused {
		t.Error("isPaused should be true after PausedMsg")
	}
}

func TestUpdate_ResumedMsg(t *testing.T) {
	t.Parallel()
	m := New()
	m.isPaused = true
	updated, _ := m.Update(ResumedMsg{})
	model := updated.(Model)
	if model.isPaused {
		t.Error("isPaused should be false after ResumedMsg")
	}
}

// ---------------------------------------------------------------------------
// Update: TaskRetryQueuedMsg
// ---------------------------------------------------------------------------

func TestUpdate_TaskRetryQueuedMsg(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Status: core.TaskStatusFailed},
		{ID: "t2", Status: core.TaskStatusCompleted},
	}

	updated, _ := m.Update(TaskRetryQueuedMsg{TaskID: "t1"})
	model := updated.(Model)

	if model.tasks[0].Status != core.TaskStatusPending {
		t.Errorf("retried task status = %s, want pending", model.tasks[0].Status)
	}
	// Unrelated task untouched
	if model.tasks[1].Status != core.TaskStatusCompleted {
		t.Errorf("unrelated task status = %s, want completed", model.tasks[1].Status)
	}
}

func TestUpdate_TaskRetryQueuedMsg_NoMatch(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Status: core.TaskStatusFailed},
	}

	updated, _ := m.Update(TaskRetryQueuedMsg{TaskID: "nonexistent"})
	model := updated.(Model)

	if model.tasks[0].Status != core.TaskStatusFailed {
		t.Error("task should remain unchanged when retry msg has no match")
	}
}

// ---------------------------------------------------------------------------
// Update: AgentStatusUpdateMsg
// ---------------------------------------------------------------------------

func TestUpdate_AgentStatusUpdateMsg(t *testing.T) {
	t.Parallel()
	m := New()

	updated, _ := m.Update(AgentStatusUpdateMsg{
		AgentID:  "claude",
		Status:   int(components.StatusWorking),
		Duration: 5 * time.Second,
		Output:   "some output",
		Error:    "",
	})
	model := updated.(Model)

	agent := model.agentMap["claude"]
	if agent.Status != components.StatusWorking {
		t.Errorf("agent status = %d, want %d", agent.Status, components.StatusWorking)
	}
	if agent.Duration != 5*time.Second {
		t.Errorf("agent duration = %v, want 5s", agent.Duration)
	}
	if agent.Output != "some output" {
		t.Errorf("agent output = %q, want %q", agent.Output, "some output")
	}
}

func TestUpdate_AgentStatusUpdateMsg_UnknownAgent(t *testing.T) {
	t.Parallel()
	m := New()

	// Should not panic when agent is not in the map
	updated, _ := m.Update(AgentStatusUpdateMsg{AgentID: "unknown-agent", Status: 1})
	_ = updated.(Model)
}

// ---------------------------------------------------------------------------
// Update: WorkflowProgressMsg
// ---------------------------------------------------------------------------

func TestUpdate_WorkflowProgressMsg(t *testing.T) {
	t.Parallel()
	m := New()

	updated, _ := m.Update(WorkflowProgressMsg{
		Title:      "Analyzing",
		Percentage: 0.75,
		Requests:   12,
	})
	model := updated.(Model)

	if model.workflowTitle != "Analyzing" {
		t.Errorf("workflowTitle = %q, want %q", model.workflowTitle, "Analyzing")
	}
	if model.workflowPct != 0.75 {
		t.Errorf("workflowPct = %f, want 0.75", model.workflowPct)
	}
	if model.totalRequests != 12 {
		t.Errorf("totalRequests = %d, want 12", model.totalRequests)
	}
}

// ---------------------------------------------------------------------------
// Update: SpinnerTickMsg
// ---------------------------------------------------------------------------

func TestUpdate_SpinnerTickMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(SpinnerTickMsg(time.Now()))
	_ = updated.(Model)
	if cmd == nil {
		t.Error("SpinnerTickMsg should produce a follow-up tick command")
	}
}

// ---------------------------------------------------------------------------
// Update: spinner.TickMsg (bubbles)
// ---------------------------------------------------------------------------

func TestUpdate_BubblesSpinnerTickMsg(t *testing.T) {
	t.Parallel()
	m := New()
	// Get the bubbles spinner tick command
	cmd := m.spinner.bubblesSpinner.Tick
	if cmd == nil {
		t.Skip("cannot produce bubbles tick in test")
	}
	// Execute the command to get the tick message
	msg := cmd()
	if msg == nil {
		t.Skip("bubbles tick returned nil msg")
	}
	// We just verify it does not panic
	updated, _ := m.Update(msg)
	_ = updated.(Model)
}

// ---------------------------------------------------------------------------
// Update: DurationTickMsg
// ---------------------------------------------------------------------------

func TestUpdate_DurationTickMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(DurationTickMsg{})
	_ = updated.(Model)
	if cmd == nil {
		t.Error("DurationTickMsg should produce a durationTick() command")
	}
}

// ---------------------------------------------------------------------------
// Update: unknown message type (default case)
// ---------------------------------------------------------------------------

func TestUpdate_UnknownMsg(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update("unknown-msg-type")
	_ = updated.(Model)
	if cmd != nil {
		t.Error("unknown message should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// handleKeyPress tests
// ---------------------------------------------------------------------------

func TestHandleKeyPress_Quit_Q(t *testing.T) {
	t.Parallel()
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Error("'q' key should produce tea.Quit")
	}
}

func TestHandleKeyPress_Quit_CtrlC(t *testing.T) {
	t.Parallel()
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should produce tea.Quit")
	}
}

func TestHandleKeyPress_NavigateUp(t *testing.T) {
	t.Parallel()
	m := New()
	m.selectedIdx = 2
	m.tasks = []*TaskView{{}, {}, {}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := updated.(Model)

	if model.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d, want 1", model.selectedIdx)
	}
}

func TestHandleKeyPress_NavigateUp_AtZero(t *testing.T) {
	t.Parallel()
	m := New()
	m.selectedIdx = 0
	m.tasks = []*TaskView{{}, {}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	model := updated.(Model)

	if model.selectedIdx != 0 {
		t.Error("selectedIdx should stay at 0 when already at top")
	}
}

func TestHandleKeyPress_NavigateDown(t *testing.T) {
	t.Parallel()
	m := New()
	m.selectedIdx = 0
	m.tasks = []*TaskView{{}, {}, {}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := updated.(Model)

	if model.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d, want 1", model.selectedIdx)
	}
}

func TestHandleKeyPress_NavigateDown_AtBottom(t *testing.T) {
	t.Parallel()
	m := New()
	m.selectedIdx = 2
	m.tasks = []*TaskView{{}, {}, {}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)

	if model.selectedIdx != 2 {
		t.Error("selectedIdx should stay at bottom")
	}
}

func TestHandleKeyPress_ToggleLogs(t *testing.T) {
	t.Parallel()
	m := New()
	if m.showLogs {
		t.Error("showLogs should start false")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model := updated.(Model)
	if !model.showLogs {
		t.Error("showLogs should be true after 'l'")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model = updated.(Model)
	if model.showLogs {
		t.Error("showLogs should be false after second 'l'")
	}
}

func TestHandleKeyPress_Enter(t *testing.T) {
	t.Parallel()
	m := New()
	// Enter currently does nothing, just ensure no panic
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("enter should return nil cmd")
	}
}

func TestHandleKeyPress_Retry_FailedTask(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Status: core.TaskStatusFailed},
	}
	m.selectedIdx = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd == nil {
		t.Error("retry on failed task should produce a command")
	}
}

func TestHandleKeyPress_Retry_NonFailedTask(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Status: core.TaskStatusCompleted},
	}
	m.selectedIdx = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd != nil {
		t.Error("retry on non-failed task should return nil cmd")
	}
}

func TestHandleKeyPress_Retry_OutOfBounds(t *testing.T) {
	t.Parallel()
	m := New()
	m.tasks = []*TaskView{}
	m.selectedIdx = 0

	// Should not panic
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if cmd != nil {
		t.Error("retry with no tasks should return nil cmd")
	}
}

func TestHandleKeyPress_Pause(t *testing.T) {
	t.Parallel()
	m := New()
	m.isPaused = false

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if cmd == nil {
		t.Error("pause should produce a PauseCmd")
	}
}

func TestHandleKeyPress_Resume(t *testing.T) {
	t.Parallel()
	m := New()
	m.isPaused = true

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if cmd == nil {
		t.Error("resume should produce a ResumeCmd")
	}
}

func TestHandleKeyPress_Cancel(t *testing.T) {
	t.Parallel()
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Error("cancel should produce a CancelCmd")
	}
}

func TestHandleKeyPress_UnboundKey(t *testing.T) {
	t.Parallel()
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Error("unbound key should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// View tests
// ---------------------------------------------------------------------------

func TestView_NotReady(t *testing.T) {
	t.Parallel()
	m := New()
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("View() = %q, want %q", view, "Initializing...")
	}
}

func TestView_Error(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.err = errors.New("test error")

	view := m.View()
	if !strings.Contains(view, "test error") {
		t.Errorf("error view should contain error message, got: %s", view)
	}
}

func TestView_ShowLogs(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.showLogs = true
	m.logs = []LogEntry{
		{Time: time.Now(), Level: "info", Message: "hello world"},
	}

	view := m.View()
	if !strings.Contains(view, "Logs") {
		t.Error("log view should contain 'Logs' header")
	}
	if !strings.Contains(view, "hello world") {
		t.Error("log view should contain log message")
	}
}

func TestView_MainView(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.width = 100
	m.height = 30

	view := m.View()
	if !strings.Contains(view, "Quorum AI") {
		t.Error("main view should contain header")
	}
}

// ---------------------------------------------------------------------------
// renderHeader tests
// ---------------------------------------------------------------------------

func TestRenderHeader_EmptyPhase(t *testing.T) {
	t.Parallel()
	m := Model{width: 80}
	header := m.renderHeader()
	if !strings.Contains(header, "init") {
		t.Errorf("header should show 'init' for empty phase, got: %s", header)
	}
}

func TestRenderHeader_WithError(t *testing.T) {
	t.Parallel()
	m := Model{
		width: 80,
		err:   errors.New("boom"),
	}
	header := m.renderHeader()
	if !strings.Contains(header, "error") {
		t.Errorf("header should show 'error' status, got: %s", header)
	}
}

func TestRenderHeader_Paused(t *testing.T) {
	t.Parallel()
	m := Model{
		width:    80,
		isPaused: true,
	}
	header := m.renderHeader()
	if !strings.Contains(header, "PAUSED") {
		t.Errorf("header should show 'PAUSED', got: %s", header)
	}
}

func TestRenderHeader_SmallWidth(t *testing.T) {
	t.Parallel()
	m := Model{width: 10}
	// Should not panic even with very narrow width
	header := m.renderHeader()
	if header == "" {
		t.Error("header should not be empty")
	}
}

// ---------------------------------------------------------------------------
// renderProgress tests
// ---------------------------------------------------------------------------

func TestRenderProgress_NilWorkflow(t *testing.T) {
	t.Parallel()
	m := Model{workflow: nil}
	result := m.renderProgress()
	if result != "" {
		t.Errorf("renderProgress with nil workflow = %q, want empty", result)
	}
}

func TestRenderProgress_ZeroTasks(t *testing.T) {
	t.Parallel()
	m := Model{
		workflow: &core.WorkflowState{},
		tasks:    []*TaskView{},
		width:    80,
	}
	result := m.renderProgress()
	if !strings.Contains(result, "0.0%") {
		t.Errorf("expected 0%% for zero tasks, got: %s", result)
	}
}

// ---------------------------------------------------------------------------
// renderTasks tests
// ---------------------------------------------------------------------------

func TestRenderTasks_Empty(t *testing.T) {
	t.Parallel()
	m := New()
	result := m.renderTasks()
	if result != "No tasks" {
		t.Errorf("renderTasks with empty tasks = %q, want %q", result, "No tasks")
	}
}

func TestRenderTasks_WithTasks(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-10 * time.Second)
	m := New()
	m.tasks = []*TaskView{
		{ID: "t1", Name: "First", Status: core.TaskStatusCompleted, Duration: 5 * time.Second},
		{ID: "t2", Name: "Second", Status: core.TaskStatusRunning, StartedAt: &started},
		{ID: "t3", Name: "Third", Status: core.TaskStatusPending},
	}
	m.selectedIdx = 1

	result := m.renderTasks()
	if !strings.Contains(result, "Tasks:") {
		t.Error("should contain 'Tasks:' header")
	}
	if !strings.Contains(result, "First") {
		t.Error("should contain first task name")
	}
	if !strings.Contains(result, "Second") {
		t.Error("should contain second task name")
	}
}

// ---------------------------------------------------------------------------
// renderFooter tests
// ---------------------------------------------------------------------------

func TestRenderFooter_NotPaused(t *testing.T) {
	t.Parallel()
	m := New()
	footer := m.renderFooter()
	if !strings.Contains(footer, "p: pause") {
		t.Errorf("footer should show 'p: pause', got: %s", footer)
	}
}

func TestRenderFooter_Paused(t *testing.T) {
	t.Parallel()
	m := New()
	m.isPaused = true
	footer := m.renderFooter()
	if !strings.Contains(footer, "p: resume") {
		t.Errorf("footer should show 'p: resume', got: %s", footer)
	}
}

func TestRenderFooter_DroppedEvents(t *testing.T) {
	t.Parallel()
	m := New()
	m.droppedEvents = 5
	footer := m.renderFooter()
	if !strings.Contains(footer, "5 dropped") {
		t.Errorf("footer should show dropped events count, got: %s", footer)
	}
}

// ---------------------------------------------------------------------------
// renderLogs tests
// ---------------------------------------------------------------------------

func TestRenderLogs_ErrorLevel(t *testing.T) {
	t.Parallel()
	m := New()
	m.logs = []LogEntry{
		{Time: time.Now(), Level: "error", Message: "something broke"},
	}
	result := m.renderLogs()
	if !strings.Contains(result, "something broke") {
		t.Error("error log message should be present")
	}
}

func TestRenderLogs_WarnLevel(t *testing.T) {
	t.Parallel()
	m := New()
	m.logs = []LogEntry{
		{Time: time.Now(), Level: "warn", Message: "be careful"},
	}
	result := m.renderLogs()
	if !strings.Contains(result, "be careful") {
		t.Error("warn log message should be present")
	}
}

func TestRenderLogs_ManyLogs(t *testing.T) {
	t.Parallel()
	m := New()
	// Add 30 logs; only last 20 should show
	for i := 0; i < 30; i++ {
		m.logs = append(m.logs, LogEntry{
			Time:    time.Now(),
			Level:   "info",
			Message: "msg",
		})
	}
	result := m.renderLogs()
	// Should contain "Logs" header
	if !strings.Contains(result, "Logs") {
		t.Error("log view should contain header")
	}
}

// ---------------------------------------------------------------------------
// statusIcon tests
// ---------------------------------------------------------------------------

func TestStatusIcon(t *testing.T) {
	t.Parallel()
	m := Model{}
	tests := []struct {
		status core.TaskStatus
		icon   string
	}{
		{core.TaskStatusPending, "○"},
		{core.TaskStatusRunning, "●"},
		{core.TaskStatusCompleted, "✓"},
		{core.TaskStatusFailed, "✗"},
		{core.TaskStatusSkipped, "⊘"},
		{core.TaskStatus("unknown"), "?"},
	}

	for _, tc := range tests {
		got := m.statusIcon(tc.status)
		if got != tc.icon {
			t.Errorf("statusIcon(%s) = %s, want %s", tc.status, got, tc.icon)
		}
	}
}

// ---------------------------------------------------------------------------
// buildTaskViews tests (additional coverage)
// ---------------------------------------------------------------------------

func TestBuildTaskViews_NilState(t *testing.T) {
	t.Parallel()
	m := New()
	views := m.buildTaskViews(nil)
	if views != nil {
		t.Error("buildTaskViews(nil) should return nil")
	}
}

func TestBuildTaskViews_WithDurations(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-10 * time.Second)
	completed := time.Now().Add(-5 * time.Second)

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Name: "Done", Status: core.TaskStatusCompleted,
					StartedAt: &started, CompletedAt: &completed},
				"t2": {ID: "t2", Name: "Running", Status: core.TaskStatusRunning,
					StartedAt: &started},
				"t3": {ID: "t3", Name: "Pending", Status: core.TaskStatusPending},
			},
			TaskOrder: []core.TaskID{"t1", "t2", "t3"},
		},
	}

	m := New()
	views := m.buildTaskViews(state)

	if len(views) != 3 {
		t.Fatalf("expected 3 views, got %d", len(views))
	}

	// Completed task should have ~5s duration
	if views[0].Duration < 4*time.Second || views[0].Duration > 6*time.Second {
		t.Errorf("completed task duration = %v, want ~5s", views[0].Duration)
	}

	// Running task should have ~10s duration
	if views[1].Duration < 9*time.Second {
		t.Errorf("running task duration = %v, want ~10s", views[1].Duration)
	}

	// Pending task should have 0 duration
	if views[2].Duration != 0 {
		t.Errorf("pending task duration = %v, want 0", views[2].Duration)
	}
}

// ---------------------------------------------------------------------------
// renderProgressBar tests
// ---------------------------------------------------------------------------

func TestRenderProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		percentage float64
		width      int
	}{
		{0, 20},
		{50, 20},
		{100, 20},
		{25, 0},  // zero width should default to 20
		{75, -5}, // negative width should default to 20
	}

	for _, tc := range tests {
		result := renderProgressBar(tc.percentage, tc.width)
		if result == "" {
			t.Errorf("renderProgressBar(%f, %d) returned empty", tc.percentage, tc.width)
		}
	}
}

func TestRenderProgressBar_FullBar(t *testing.T) {
	t.Parallel()
	result := renderProgressBar(100, 10)
	if !strings.Contains(result, "█") {
		t.Error("100% bar should contain filled blocks")
	}
}

func TestRenderProgressBar_EmptyBar(t *testing.T) {
	t.Parallel()
	result := renderProgressBar(0, 10)
	if strings.Contains(result, "█") {
		t.Error("0% bar should not contain filled blocks")
	}
}

// ---------------------------------------------------------------------------
// progressStats tests
// ---------------------------------------------------------------------------

func TestProgressStats_Finished(t *testing.T) {
	t.Parallel()
	s := progressStats{completed: 3, failed: 1, skipped: 2}
	if s.finished() != 6 {
		t.Errorf("finished() = %d, want 6", s.finished())
	}
}

func TestGetProgressStats(t *testing.T) {
	t.Parallel()
	m := Model{
		tasks: []*TaskView{
			{Status: core.TaskStatusPending},
			{Status: core.TaskStatusRunning},
			{Status: core.TaskStatusCompleted},
			{Status: core.TaskStatusFailed},
			{Status: core.TaskStatusSkipped},
		},
	}

	stats := m.getProgressStats()
	if stats.total != 5 {
		t.Errorf("total = %d, want 5", stats.total)
	}
	if stats.pending != 1 {
		t.Errorf("pending = %d, want 1", stats.pending)
	}
	if stats.running != 1 {
		t.Errorf("running = %d, want 1", stats.running)
	}
	if stats.completed != 1 {
		t.Errorf("completed = %d, want 1", stats.completed)
	}
	if stats.failed != 1 {
		t.Errorf("failed = %d, want 1", stats.failed)
	}
	if stats.skipped != 1 {
		t.Errorf("skipped = %d, want 1", stats.skipped)
	}
}

// ---------------------------------------------------------------------------
// SetWorkflow / AddLog tests
// ---------------------------------------------------------------------------

func TestSetWorkflow(t *testing.T) {
	t.Parallel()
	m := New()

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Name: "Task 1"},
			},
			TaskOrder: []core.TaskID{"t1"},
		},
	}

	m.SetWorkflow(state)
	if m.workflow != state {
		t.Error("SetWorkflow should set the workflow")
	}
	if len(m.tasks) != 1 {
		t.Errorf("SetWorkflow should build task views, got %d", len(m.tasks))
	}
}

func TestAddLog(t *testing.T) {
	t.Parallel()
	m := New()
	m.AddLog("warn", "test warning")

	if len(m.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(m.logs))
	}
	if m.logs[0].Level != "warn" {
		t.Errorf("log level = %q, want %q", m.logs[0].Level, "warn")
	}
	if m.logs[0].Message != "test warning" {
		t.Errorf("log message = %q, want %q", m.logs[0].Message, "test warning")
	}
}

func TestAddLog_TruncatesAt100(t *testing.T) {
	t.Parallel()
	m := New()
	for i := 0; i < 105; i++ {
		m.AddLog("info", "msg")
	}
	if len(m.logs) != 100 {
		t.Errorf("expected 100 logs, got %d", len(m.logs))
	}
}

// ---------------------------------------------------------------------------
// renderMain layout tests
// ---------------------------------------------------------------------------

func TestRenderMain_NarrowTerminal(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.width = 60 // Less than 80
	m.height = 20

	// Should not panic and should use vertical layout
	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output even for narrow terminal")
	}
}

func TestRenderMain_WideTerminal(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.width = 120
	m.height = 30
	m.showSidebar = true

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output for wide terminal")
	}
}

func TestRenderMain_SidebarHidden(t *testing.T) {
	t.Parallel()
	m := New()
	m.ready = true
	m.width = 120
	m.height = 30
	m.showSidebar = false

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output when sidebar is hidden")
	}
}

// ---------------------------------------------------------------------------
// renderMainContent tests
// ---------------------------------------------------------------------------

func TestRenderMainContent_WithWorkflowProgress(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 120
	m.height = 30
	m.showSidebar = true
	m.workflow = &core.WorkflowState{}
	m.workflowPct = 0.5
	m.workflowTitle = "Running"

	result := m.renderMainContent()
	if result == "" {
		t.Error("renderMainContent should produce output")
	}
}

func TestRenderMainContent_WithDoneAgents(t *testing.T) {
	t.Parallel()
	m := New()
	m.width = 120
	m.height = 30
	m.showSidebar = true

	// Set an agent to done with output
	m.agentMap["claude"].Status = components.StatusDone
	m.agentMap["claude"].Output = "Analysis complete"

	result := m.renderMainContent()
	if result == "" {
		t.Error("renderMainContent should produce output with done agents")
	}
}

// ---------------------------------------------------------------------------
// getTaskDuration tests (additional)
// ---------------------------------------------------------------------------

func TestGetTaskDuration_RunningNoStartedAt(t *testing.T) {
	t.Parallel()
	task := &TaskView{
		ID:     "t1",
		Status: core.TaskStatusRunning,
		// StartedAt is nil
	}

	m := Model{}
	d := m.getTaskDuration(task)
	if d != 0 {
		t.Errorf("duration for running task with no StartedAt = %v, want 0", d)
	}
}

// ---------------------------------------------------------------------------
// formatDuration additional edge cases
// ---------------------------------------------------------------------------

func TestFormatDuration_ExactMinute(t *testing.T) {
	t.Parallel()
	result := formatDuration(60 * time.Second)
	if result != "1m00s" {
		t.Errorf("formatDuration(60s) = %q, want %q", result, "1m00s")
	}
}

func TestFormatDuration_ExactHour(t *testing.T) {
	t.Parallel()
	result := formatDuration(60 * time.Minute)
	if result != "1h00m" {
		t.Errorf("formatDuration(1h) = %q, want %q", result, "1h00m")
	}
}

// ---------------------------------------------------------------------------
// PhaseUpdateMsg without adapter
// ---------------------------------------------------------------------------

func TestUpdate_PhaseUpdateMsg_NoAdapter(t *testing.T) {
	t.Parallel()
	m := New()
	updated, cmd := m.Update(PhaseUpdateMsg{Phase: core.PhaseExecute})
	model := updated.(Model)

	if model.currentPhase != core.PhaseExecute {
		t.Errorf("phase = %s, want execute", model.currentPhase)
	}
	if cmd != nil {
		t.Error("without adapter, PhaseUpdateMsg should return nil cmd")
	}
}
