package events

// Event type constants for control events.
const (
	TypePauseRequest  = "pause_request"
	TypeResumeRequest = "resume_request"
	TypeAbortRequest  = "abort_request"
	TypeRetryRequest  = "retry_request"
	TypeSkipRequest   = "skip_request"
)

// PauseRequestEvent requests workflow pause.
type PauseRequestEvent struct {
	BaseEvent
	Reason string `json:"reason"`
}

// NewPauseRequestEvent creates a new pause request event.
func NewPauseRequestEvent(workflowID, projectID, reason string) PauseRequestEvent {
	return PauseRequestEvent{
		BaseEvent: NewBaseEvent(TypePauseRequest, workflowID, projectID),
		Reason:    reason,
	}
}

// ResumeRequestEvent requests workflow resume.
type ResumeRequestEvent struct {
	BaseEvent
}

// NewResumeRequestEvent creates a new resume request event.
func NewResumeRequestEvent(workflowID, projectID string) ResumeRequestEvent {
	return ResumeRequestEvent{
		BaseEvent: NewBaseEvent(TypeResumeRequest, workflowID, projectID),
	}
}

// AbortRequestEvent requests workflow abort.
type AbortRequestEvent struct {
	BaseEvent
	Reason string `json:"reason"`
	Force  bool   `json:"force"`
}

// NewAbortRequestEvent creates a new abort request event.
func NewAbortRequestEvent(workflowID, projectID, reason string, force bool) AbortRequestEvent {
	return AbortRequestEvent{
		BaseEvent: NewBaseEvent(TypeAbortRequest, workflowID, projectID),
		Reason:    reason,
		Force:     force,
	}
}

// RetryRequestEvent requests task retry.
type RetryRequestEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
}

// NewRetryRequestEvent creates a new retry request event.
func NewRetryRequestEvent(workflowID, projectID, taskID string) RetryRequestEvent {
	return RetryRequestEvent{
		BaseEvent: NewBaseEvent(TypeRetryRequest, workflowID, projectID),
		TaskID:    taskID,
	}
}

// SkipRequestEvent requests task skip.
type SkipRequestEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// NewSkipRequestEvent creates a new skip request event.
func NewSkipRequestEvent(workflowID, projectID, taskID, reason string) SkipRequestEvent {
	return SkipRequestEvent{
		BaseEvent: NewBaseEvent(TypeSkipRequest, workflowID, projectID),
		TaskID:    taskID,
		Reason:    reason,
	}
}
