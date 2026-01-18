package events

import "time"

// Event type constants for workflow events.
const (
	TypeWorkflowStarted      = "workflow_started"
	TypeWorkflowStateUpdated = "workflow_state_updated"
	TypeWorkflowCompleted    = "workflow_completed"
	TypeWorkflowFailed       = "workflow_failed"
	TypeWorkflowPaused       = "workflow_paused"
	TypeWorkflowResumed      = "workflow_resumed"
)

// WorkflowStartedEvent is emitted when a workflow begins.
type WorkflowStartedEvent struct {
	BaseEvent
	Prompt string `json:"prompt"`
}

// NewWorkflowStartedEvent creates a new workflow started event.
func NewWorkflowStartedEvent(workflowID, prompt string) WorkflowStartedEvent {
	return WorkflowStartedEvent{
		BaseEvent: NewBaseEvent(TypeWorkflowStarted, workflowID),
		Prompt:    prompt,
	}
}

// WorkflowStateUpdatedEvent is emitted when workflow state changes.
// This has correct semantics - NOT mapped to WorkflowCompleted.
type WorkflowStateUpdatedEvent struct {
	BaseEvent
	Phase      string `json:"phase"`
	TotalTasks int    `json:"total_tasks"`
	Completed  int    `json:"completed"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
}

// NewWorkflowStateUpdatedEvent creates a new state updated event.
func NewWorkflowStateUpdatedEvent(workflowID, phase string, total, completed, failed, skipped int) WorkflowStateUpdatedEvent {
	return WorkflowStateUpdatedEvent{
		BaseEvent:  NewBaseEvent(TypeWorkflowStateUpdated, workflowID),
		Phase:      phase,
		TotalTasks: total,
		Completed:  completed,
		Failed:     failed,
		Skipped:    skipped,
	}
}

// WorkflowCompletedEvent is emitted when workflow finishes successfully.
// CRITICAL: This should only be emitted ONCE per workflow.
type WorkflowCompletedEvent struct {
	BaseEvent
	Duration  time.Duration `json:"duration"`
	TotalCost float64       `json:"total_cost"`
}

// NewWorkflowCompletedEvent creates a new workflow completed event.
func NewWorkflowCompletedEvent(workflowID string, duration time.Duration, totalCost float64) WorkflowCompletedEvent {
	return WorkflowCompletedEvent{
		BaseEvent: NewBaseEvent(TypeWorkflowCompleted, workflowID),
		Duration:  duration,
		TotalCost: totalCost,
	}
}

// WorkflowFailedEvent is emitted when workflow fails.
// This is a PRIORITY event - never dropped.
type WorkflowFailedEvent struct {
	BaseEvent
	Phase string `json:"phase"`
	Error string `json:"error"`
}

// NewWorkflowFailedEvent creates a new workflow failed event.
func NewWorkflowFailedEvent(workflowID, phase string, err error) WorkflowFailedEvent {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return WorkflowFailedEvent{
		BaseEvent: NewBaseEvent(TypeWorkflowFailed, workflowID),
		Phase:     phase,
		Error:     errStr,
	}
}

// WorkflowPausedEvent is emitted when workflow is paused.
type WorkflowPausedEvent struct {
	BaseEvent
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

// NewWorkflowPausedEvent creates a new workflow paused event.
func NewWorkflowPausedEvent(workflowID, phase, reason string) WorkflowPausedEvent {
	return WorkflowPausedEvent{
		BaseEvent: NewBaseEvent(TypeWorkflowPaused, workflowID),
		Phase:     phase,
		Reason:    reason,
	}
}

// WorkflowResumedEvent is emitted when workflow resumes.
type WorkflowResumedEvent struct {
	BaseEvent
	FromPhase string `json:"from_phase"`
}

// NewWorkflowResumedEvent creates a new workflow resumed event.
func NewWorkflowResumedEvent(workflowID, fromPhase string) WorkflowResumedEvent {
	return WorkflowResumedEvent{
		BaseEvent: NewBaseEvent(TypeWorkflowResumed, workflowID),
		FromPhase: fromPhase,
	}
}
