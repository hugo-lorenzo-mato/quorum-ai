package control

import (
	"testing"
)

func TestPauseRequestedEvent_Type(t *testing.T) {
	e := PauseRequestedEvent{}
	if got := e.Type(); got != "control_pause_requested" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestResumeRequestedEvent_Type(t *testing.T) {
	e := ResumeRequestedEvent{}
	if got := e.Type(); got != "control_resume_requested" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestCancelRequestedEvent_Type(t *testing.T) {
	e := CancelRequestedEvent{}
	if got := e.Type(); got != "control_cancel_requested" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestRetryRequestedEvent_Type(t *testing.T) {
	e := RetryRequestedEvent{}
	if got := e.Type(); got != "control_retry_requested" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestPausedEvent_Type(t *testing.T) {
	e := PausedEvent{}
	if got := e.Type(); got != "control_paused" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestResumedEvent_Type(t *testing.T) {
	e := ResumedEvent{}
	if got := e.Type(); got != "control_resumed" {
		t.Errorf("Type() = %q", got)
	}
	e.IsControlEvent()
}

func TestRetryRequestedEvent_TaskID(t *testing.T) {
	e := RetryRequestedEvent{TaskID: "task-42"}
	if e.TaskID != "task-42" {
		t.Errorf("got TaskID %q, want %q", e.TaskID, "task-42")
	}
}
