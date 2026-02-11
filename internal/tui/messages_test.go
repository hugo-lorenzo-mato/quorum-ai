package tui

import (
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ---------------------------------------------------------------------------
// Message type construction tests
// ---------------------------------------------------------------------------

func TestWorkflowUpdateMsg_Fields(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{Status: core.WorkflowStatusRunning},
	}
	msg := WorkflowUpdateMsg{State: state}
	if msg.State != state {
		t.Error("WorkflowUpdateMsg.State should hold the provided state")
	}
}

func TestPhaseUpdateMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := PhaseUpdateMsg{Phase: core.PhaseAnalyze}
	if msg.Phase != core.PhaseAnalyze {
		t.Errorf("PhaseUpdateMsg.Phase = %s, want analyze", msg.Phase)
	}
}

func TestTaskUpdateMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := TaskUpdateMsg{
		TaskID:   "task-1",
		Status:   core.TaskStatusFailed,
		Progress: 0.5,
		Error:    "timeout",
	}
	if msg.TaskID != "task-1" {
		t.Errorf("TaskID = %s, want task-1", msg.TaskID)
	}
	if msg.Status != core.TaskStatusFailed {
		t.Errorf("Status = %s, want failed", msg.Status)
	}
	if msg.Progress != 0.5 {
		t.Errorf("Progress = %f, want 0.5", msg.Progress)
	}
	if msg.Error != "timeout" {
		t.Errorf("Error = %q, want %q", msg.Error, "timeout")
	}
}

func TestLogMsg_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	msg := LogMsg{Time: now, Level: "error", Message: "disk full"}
	if msg.Time != now {
		t.Error("LogMsg.Time mismatch")
	}
	if msg.Level != "error" {
		t.Errorf("Level = %q, want %q", msg.Level, "error")
	}
	if msg.Message != "disk full" {
		t.Errorf("Message = %q, want %q", msg.Message, "disk full")
	}
}

func TestErrorMsg_Fields(t *testing.T) {
	t.Parallel()
	err := errors.New("test err")
	msg := ErrorMsg{Error: err}
	if msg.Error != err {
		t.Error("ErrorMsg.Error mismatch")
	}
}

func TestDroppedEventsMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := DroppedEventsMsg{Count: 42}
	if msg.Count != 42 {
		t.Errorf("Count = %d, want 42", msg.Count)
	}
}

func TestMetricsUpdateMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := MetricsUpdateMsg{
		TotalTokensIn:  1000,
		TotalTokensOut: 500,
		Duration:       5 * time.Second,
	}
	if msg.TotalTokensIn != 1000 {
		t.Errorf("TotalTokensIn = %d, want 1000", msg.TotalTokensIn)
	}
	if msg.TotalTokensOut != 500 {
		t.Errorf("TotalTokensOut = %d, want 500", msg.TotalTokensOut)
	}
	if msg.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", msg.Duration)
	}
}

func TestSpinnerTickMsg_IsTimeAlias(t *testing.T) {
	t.Parallel()
	now := time.Now()
	msg := SpinnerTickMsg(now)
	if time.Time(msg) != now {
		t.Error("SpinnerTickMsg should convert back to time.Time")
	}
}

func TestDurationTickMsg_ZeroValue(t *testing.T) {
	t.Parallel()
	msg := DurationTickMsg{}
	_ = msg // ensure it's usable as tea.Msg
}

func TestQuitMsg_ZeroValue(t *testing.T) {
	t.Parallel()
	msg := QuitMsg{}
	_ = msg
}

func TestPausedMsg_ZeroValue(t *testing.T) {
	t.Parallel()
	_ = PausedMsg{}
}

func TestResumedMsg_ZeroValue(t *testing.T) {
	t.Parallel()
	_ = ResumedMsg{}
}

func TestCancelledMsg_ZeroValue(t *testing.T) {
	t.Parallel()
	_ = CancelledMsg{}
}

func TestControlPlaneMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := ControlPlaneMsg{Control: nil}
	if msg.Control != nil {
		t.Error("Control should be nil")
	}
}

func TestTaskRetryQueuedMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := TaskRetryQueuedMsg{TaskID: "t-retry"}
	if msg.TaskID != "t-retry" {
		t.Errorf("TaskID = %s, want t-retry", msg.TaskID)
	}
}

func TestAgentStatusUpdateMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := AgentStatusUpdateMsg{
		AgentID:  "claude",
		Status:   2,
		Duration: 3 * time.Second,
		Output:   "done",
		Error:    "",
	}
	if msg.AgentID != "claude" {
		t.Errorf("AgentID = %q, want %q", msg.AgentID, "claude")
	}
	if msg.Status != 2 {
		t.Errorf("Status = %d, want 2", msg.Status)
	}
	if msg.Duration != 3*time.Second {
		t.Errorf("Duration = %v, want 3s", msg.Duration)
	}
	if msg.Output != "done" {
		t.Errorf("Output = %q, want %q", msg.Output, "done")
	}
}

func TestWorkflowProgressMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := WorkflowProgressMsg{
		Title:      "Executing",
		Percentage: 0.8,
		Requests:   10,
	}
	if msg.Title != "Executing" {
		t.Errorf("Title = %q, want %q", msg.Title, "Executing")
	}
	if msg.Percentage != 0.8 {
		t.Errorf("Percentage = %f, want 0.8", msg.Percentage)
	}
	if msg.Requests != 10 {
		t.Errorf("Requests = %d, want 10", msg.Requests)
	}
}

func TestAgentEventMsg_Fields(t *testing.T) {
	t.Parallel()
	msg := AgentEventMsg{
		Kind:    "tool_use",
		Agent:   "gemini",
		Message: "Reading file",
		Data:    map[string]interface{}{"path": "/tmp/foo"},
	}
	if msg.Kind != "tool_use" {
		t.Errorf("Kind = %q, want %q", msg.Kind, "tool_use")
	}
	if msg.Agent != "gemini" {
		t.Errorf("Agent = %q, want %q", msg.Agent, "gemini")
	}
	if msg.Message != "Reading file" {
		t.Errorf("Message = %q, want %q", msg.Message, "Reading file")
	}
	if msg.Data["path"] != "/tmp/foo" {
		t.Errorf("Data[path] = %v, want /tmp/foo", msg.Data["path"])
	}
}

// ---------------------------------------------------------------------------
// Command constructors
// ---------------------------------------------------------------------------

func TestPauseCmd_NilControlPlane(t *testing.T) {
	t.Parallel()
	cmd := PauseCmd(nil)
	if cmd == nil {
		t.Fatal("PauseCmd(nil) should return a non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(PausedMsg); !ok {
		t.Errorf("PauseCmd(nil)() = %T, want PausedMsg", msg)
	}
}

func TestPauseCmd_WithControlPlane(t *testing.T) {
	t.Parallel()
	cp := control.New()
	cmd := PauseCmd(cp)
	msg := cmd()
	if _, ok := msg.(PausedMsg); !ok {
		t.Errorf("PauseCmd()() = %T, want PausedMsg", msg)
	}
	if !cp.IsPaused() {
		t.Error("ControlPlane should be paused after PauseCmd")
	}
}

func TestResumeCmd_NilControlPlane(t *testing.T) {
	t.Parallel()
	cmd := ResumeCmd(nil)
	if cmd == nil {
		t.Fatal("ResumeCmd(nil) should return a non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(ResumedMsg); !ok {
		t.Errorf("ResumeCmd(nil)() = %T, want ResumedMsg", msg)
	}
}

func TestResumeCmd_WithControlPlane(t *testing.T) {
	t.Parallel()
	cp := control.New()
	cp.Pause()
	if !cp.IsPaused() {
		t.Fatal("precondition: should be paused")
	}

	cmd := ResumeCmd(cp)
	msg := cmd()
	if _, ok := msg.(ResumedMsg); !ok {
		t.Errorf("ResumeCmd()() = %T, want ResumedMsg", msg)
	}
	if cp.IsPaused() {
		t.Error("ControlPlane should be resumed after ResumeCmd")
	}
}

func TestRetryTaskCmd_NilControlPlane(t *testing.T) {
	t.Parallel()
	cmd := RetryTaskCmd(nil, "task-1")
	if cmd == nil {
		t.Fatal("RetryTaskCmd(nil, ...) should return a non-nil command")
	}
	msg := cmd()
	retryMsg, ok := msg.(TaskRetryQueuedMsg)
	if !ok {
		t.Fatalf("RetryTaskCmd(nil, ...)() = %T, want TaskRetryQueuedMsg", msg)
	}
	if retryMsg.TaskID != "task-1" {
		t.Errorf("TaskID = %s, want task-1", retryMsg.TaskID)
	}
}

func TestRetryTaskCmd_WithControlPlane(t *testing.T) {
	t.Parallel()
	cp := control.New()
	cmd := RetryTaskCmd(cp, "task-42")
	msg := cmd()

	retryMsg, ok := msg.(TaskRetryQueuedMsg)
	if !ok {
		t.Fatalf("RetryTaskCmd()() = %T, want TaskRetryQueuedMsg", msg)
	}
	if retryMsg.TaskID != "task-42" {
		t.Errorf("TaskID = %s, want task-42", retryMsg.TaskID)
	}
}

func TestCancelCmd_NilControlPlane(t *testing.T) {
	t.Parallel()
	cmd := CancelCmd(nil)
	if cmd == nil {
		t.Fatal("CancelCmd(nil) should return a non-nil command")
	}
	msg := cmd()
	if _, ok := msg.(CancelledMsg); !ok {
		t.Errorf("CancelCmd(nil)() = %T, want CancelledMsg", msg)
	}
}

func TestCancelCmd_WithControlPlane(t *testing.T) {
	t.Parallel()
	cp := control.New()
	cmd := CancelCmd(cp)
	msg := cmd()
	if _, ok := msg.(CancelledMsg); !ok {
		t.Errorf("CancelCmd()() = %T, want CancelledMsg", msg)
	}
	if !cp.IsCancelled() {
		t.Error("ControlPlane should be cancelled after CancelCmd")
	}
}

// ---------------------------------------------------------------------------
// Factory functions: SendWorkflowUpdate, SendTaskUpdate, SendLog, SendError
// ---------------------------------------------------------------------------

func TestSendWorkflowUpdate(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{Status: core.WorkflowStatusCompleted},
	}
	msg := SendWorkflowUpdate(state)
	wuMsg, ok := msg.(WorkflowUpdateMsg)
	if !ok {
		t.Fatalf("SendWorkflowUpdate() = %T, want WorkflowUpdateMsg", msg)
	}
	if wuMsg.State != state {
		t.Error("state mismatch")
	}
}

func TestSendTaskUpdate(t *testing.T) {
	t.Parallel()
	msg := SendTaskUpdate("t1", core.TaskStatusRunning, 0.3, "")
	tuMsg, ok := msg.(TaskUpdateMsg)
	if !ok {
		t.Fatalf("SendTaskUpdate() = %T, want TaskUpdateMsg", msg)
	}
	if tuMsg.TaskID != "t1" {
		t.Errorf("TaskID = %s, want t1", tuMsg.TaskID)
	}
	if tuMsg.Status != core.TaskStatusRunning {
		t.Errorf("Status = %s, want running", tuMsg.Status)
	}
	if tuMsg.Progress != 0.3 {
		t.Errorf("Progress = %f, want 0.3", tuMsg.Progress)
	}
}

func TestSendLog(t *testing.T) {
	t.Parallel()
	msg := SendLog("warn", "disk space low")
	logMsg, ok := msg.(LogMsg)
	if !ok {
		t.Fatalf("SendLog() = %T, want LogMsg", msg)
	}
	if logMsg.Level != "warn" {
		t.Errorf("Level = %q, want %q", logMsg.Level, "warn")
	}
	if logMsg.Message != "disk space low" {
		t.Errorf("Message = %q, want %q", logMsg.Message, "disk space low")
	}
	if logMsg.Time.IsZero() {
		t.Error("Time should be set")
	}
}

func TestSendError(t *testing.T) {
	t.Parallel()
	err := errors.New("critical failure")
	msg := SendError(err)
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("SendError() = %T, want ErrorMsg", msg)
	}
	if errMsg.Error != err {
		t.Error("error mismatch")
	}
}

// ---------------------------------------------------------------------------
// waitForWorkflowUpdate (legacy stub)
// ---------------------------------------------------------------------------

func TestWaitForWorkflowUpdate_ReturnsCmd(t *testing.T) {
	t.Parallel()
	cmd := waitForWorkflowUpdate()
	if cmd == nil {
		t.Fatal("waitForWorkflowUpdate should return a non-nil command")
	}
	// Execute the command; it sleeps and returns nil
	msg := cmd()
	if msg != nil {
		t.Errorf("waitForWorkflowUpdate()() = %v, want nil", msg)
	}
}

// ---------------------------------------------------------------------------
// waitForEventBusUpdate with nil adapter
// ---------------------------------------------------------------------------

func TestWaitForEventBusUpdate_NilAdapter(t *testing.T) {
	t.Parallel()
	cmd := waitForEventBusUpdate(nil)
	if cmd == nil {
		t.Fatal("waitForEventBusUpdate(nil) should return a non-nil command")
	}
	// Execute: it sleeps 100ms and returns nil
	msg := cmd()
	if msg != nil {
		t.Errorf("waitForEventBusUpdate(nil)() = %v, want nil", msg)
	}
}

// ---------------------------------------------------------------------------
// durationTick
// ---------------------------------------------------------------------------

func TestDurationTick_ReturnsCmd(t *testing.T) {
	t.Parallel()
	cmd := durationTick()
	if cmd == nil {
		t.Fatal("durationTick should return a non-nil command")
	}
}
