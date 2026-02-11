package events

import "time"

// Event type constants for phase events.
const (
	TypePhaseStarted        = "phase_started"
	TypePhaseCompleted      = "phase_completed"
	TypePhaseAwaitingReview = "phase_awaiting_review"
	TypePhaseReviewApproved = "phase_review_approved"
	TypePhaseReviewRejected = "phase_review_rejected"
)

// PhaseStartedEvent is emitted when a phase begins.
type PhaseStartedEvent struct {
	BaseEvent
	Phase string `json:"phase"`
}

// NewPhaseStartedEvent creates a new phase started event.
func NewPhaseStartedEvent(workflowID, projectID, phase string) PhaseStartedEvent {
	return PhaseStartedEvent{
		BaseEvent: NewBaseEvent(TypePhaseStarted, workflowID, projectID),
		Phase:     phase,
	}
}

// PhaseCompletedEvent is emitted when a phase finishes.
type PhaseCompletedEvent struct {
	BaseEvent
	Phase    string        `json:"phase"`
	Duration time.Duration `json:"duration"`
}

// NewPhaseCompletedEvent creates a new phase completed event.
func NewPhaseCompletedEvent(workflowID, projectID, phase string, duration time.Duration) PhaseCompletedEvent {
	return PhaseCompletedEvent{
		BaseEvent: NewBaseEvent(TypePhaseCompleted, workflowID, projectID),
		Phase:     phase,
		Duration:  duration,
	}
}

// PhaseAwaitingReviewEvent is emitted when an interactive workflow pauses for user review.
type PhaseAwaitingReviewEvent struct {
	BaseEvent
	Phase string `json:"phase"`
}

// NewPhaseAwaitingReviewEvent creates a new phase awaiting review event.
func NewPhaseAwaitingReviewEvent(workflowID, projectID, phase string) PhaseAwaitingReviewEvent {
	return PhaseAwaitingReviewEvent{
		BaseEvent: NewBaseEvent(TypePhaseAwaitingReview, workflowID, projectID),
		Phase:     phase,
	}
}

// PhaseReviewApprovedEvent is emitted when the user approves a phase review.
type PhaseReviewApprovedEvent struct {
	BaseEvent
	Phase string `json:"phase"`
}

// NewPhaseReviewApprovedEvent creates a new phase review approved event.
func NewPhaseReviewApprovedEvent(workflowID, projectID, phase string) PhaseReviewApprovedEvent {
	return PhaseReviewApprovedEvent{
		BaseEvent: NewBaseEvent(TypePhaseReviewApproved, workflowID, projectID),
		Phase:     phase,
	}
}

// PhaseReviewRejectedEvent is emitted when the user rejects a phase review.
type PhaseReviewRejectedEvent struct {
	BaseEvent
	Phase    string `json:"phase"`
	Feedback string `json:"feedback,omitempty"`
}

// NewPhaseReviewRejectedEvent creates a new phase review rejected event.
func NewPhaseReviewRejectedEvent(workflowID, projectID, phase, feedback string) PhaseReviewRejectedEvent {
	return PhaseReviewRejectedEvent{
		BaseEvent: NewBaseEvent(TypePhaseReviewRejected, workflowID, projectID),
		Phase:     phase,
		Feedback:  feedback,
	}
}
