package events

import "time"

// Event type constants for phase events.
const (
	TypePhaseStarted   = "phase_started"
	TypePhaseCompleted = "phase_completed"
)

// PhaseStartedEvent is emitted when a phase begins.
type PhaseStartedEvent struct {
	BaseEvent
	Phase string `json:"phase"`
}

// NewPhaseStartedEvent creates a new phase started event.
func NewPhaseStartedEvent(workflowID, phase string) PhaseStartedEvent {
	return PhaseStartedEvent{
		BaseEvent: NewBaseEvent(TypePhaseStarted, workflowID),
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
func NewPhaseCompletedEvent(workflowID, phase string, duration time.Duration) PhaseCompletedEvent {
	return PhaseCompletedEvent{
		BaseEvent: NewBaseEvent(TypePhaseCompleted, workflowID),
		Phase:     phase,
		Duration:  duration,
	}
}
