package control

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ControlEvent represents a control plane event.
type ControlEvent interface {
	events.Event
	IsControlEvent()
}

// PauseRequestedEvent signals a pause request.
type PauseRequestedEvent struct {
	events.BaseEvent
}

func (e PauseRequestedEvent) Type() string      { return "control_pause_requested" }
func (e PauseRequestedEvent) IsControlEvent()   {}

// ResumeRequestedEvent signals a resume request.
type ResumeRequestedEvent struct {
	events.BaseEvent
}

func (e ResumeRequestedEvent) Type() string      { return "control_resume_requested" }
func (e ResumeRequestedEvent) IsControlEvent()   {}

// CancelRequestedEvent signals a cancel request.
type CancelRequestedEvent struct {
	events.BaseEvent
}

func (e CancelRequestedEvent) Type() string      { return "control_cancel_requested" }
func (e CancelRequestedEvent) IsControlEvent()   {}

// RetryRequestedEvent signals a retry request.
type RetryRequestedEvent struct {
	events.BaseEvent
	TaskID core.TaskID `json:"task_id"`
}

func (e RetryRequestedEvent) Type() string      { return "control_retry_requested" }
func (e RetryRequestedEvent) IsControlEvent()   {}

// PausedEvent signals that workflow is now paused.
type PausedEvent struct {
	events.BaseEvent
}

func (e PausedEvent) Type() string      { return "control_paused" }
func (e PausedEvent) IsControlEvent()   {}

// ResumedEvent signals that workflow has resumed.
type ResumedEvent struct {
	events.BaseEvent
}

func (e ResumedEvent) Type() string      { return "control_resumed" }
func (e ResumedEvent) IsControlEvent()   {}
