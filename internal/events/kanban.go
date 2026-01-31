package events

import "time"

// Kanban event type constants.
const (
	TypeKanbanWorkflowMoved        = "kanban_workflow_moved"
	TypeKanbanExecutionStarted     = "kanban_execution_started"
	TypeKanbanExecutionCompleted   = "kanban_execution_completed"
	TypeKanbanExecutionFailed      = "kanban_execution_failed"
	TypeKanbanEngineStateChanged   = "kanban_engine_state_changed"
	TypeKanbanCircuitBreakerOpened = "kanban_circuit_breaker_opened"
)

// KanbanWorkflowMovedEvent is emitted when a workflow changes Kanban columns.
type KanbanWorkflowMovedEvent struct {
	BaseEvent
	FromColumn    string `json:"from_column"`
	ToColumn      string `json:"to_column"`
	NewPosition   int    `json:"new_position"`
	UserInitiated bool   `json:"user_initiated"`
}

// NewKanbanWorkflowMovedEvent creates a new workflow moved event.
func NewKanbanWorkflowMovedEvent(workflowID, fromColumn, toColumn string, newPosition int, userInitiated bool) KanbanWorkflowMovedEvent {
	return KanbanWorkflowMovedEvent{
		BaseEvent:     NewBaseEvent(TypeKanbanWorkflowMoved, workflowID),
		FromColumn:    fromColumn,
		ToColumn:      toColumn,
		NewPosition:   newPosition,
		UserInitiated: userInitiated,
	}
}

// KanbanExecutionStartedEvent is emitted when the engine picks a workflow for execution.
type KanbanExecutionStartedEvent struct {
	BaseEvent
	QueuePosition int `json:"queue_position"`
}

// NewKanbanExecutionStartedEvent creates a new execution started event.
func NewKanbanExecutionStartedEvent(workflowID string, queuePosition int) KanbanExecutionStartedEvent {
	return KanbanExecutionStartedEvent{
		BaseEvent:     NewBaseEvent(TypeKanbanExecutionStarted, workflowID),
		QueuePosition: queuePosition,
	}
}

// KanbanExecutionCompletedEvent is emitted when a workflow completes and moves to To Verify.
type KanbanExecutionCompletedEvent struct {
	BaseEvent
	PRURL    string `json:"pr_url"`
	PRNumber int    `json:"pr_number"`
}

// NewKanbanExecutionCompletedEvent creates a new execution completed event.
func NewKanbanExecutionCompletedEvent(workflowID, prURL string, prNumber int) KanbanExecutionCompletedEvent {
	return KanbanExecutionCompletedEvent{
		BaseEvent: NewBaseEvent(TypeKanbanExecutionCompleted, workflowID),
		PRURL:     prURL,
		PRNumber:  prNumber,
	}
}

// KanbanExecutionFailedEvent is emitted when a workflow fails and moves back to Refinement.
type KanbanExecutionFailedEvent struct {
	BaseEvent
	Error               string `json:"error"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
}

// NewKanbanExecutionFailedEvent creates a new execution failed event.
func NewKanbanExecutionFailedEvent(workflowID, errMsg string, consecutiveFailures int) KanbanExecutionFailedEvent {
	return KanbanExecutionFailedEvent{
		BaseEvent:           NewBaseEvent(TypeKanbanExecutionFailed, workflowID),
		Error:               errMsg,
		ConsecutiveFailures: consecutiveFailures,
	}
}

// KanbanEngineStateChangedEvent is emitted when the engine starts, stops, or changes state.
type KanbanEngineStateChangedEvent struct {
	BaseEvent
	Enabled            bool    `json:"enabled"`
	CurrentWorkflowID  *string `json:"current_workflow_id,omitempty"`
	CircuitBreakerOpen bool    `json:"circuit_breaker_open"`
}

// NewKanbanEngineStateChangedEvent creates a new engine state changed event.
func NewKanbanEngineStateChangedEvent(enabled bool, currentWorkflowID *string, circuitBreakerOpen bool) KanbanEngineStateChangedEvent {
	wfID := ""
	if currentWorkflowID != nil {
		wfID = *currentWorkflowID
	}
	return KanbanEngineStateChangedEvent{
		BaseEvent:          NewBaseEvent(TypeKanbanEngineStateChanged, wfID),
		Enabled:            enabled,
		CurrentWorkflowID:  currentWorkflowID,
		CircuitBreakerOpen: circuitBreakerOpen,
	}
}

// KanbanCircuitBreakerOpenedEvent is emitted when the circuit breaker trips.
type KanbanCircuitBreakerOpenedEvent struct {
	BaseEvent
	ConsecutiveFailures int       `json:"consecutive_failures"`
	Threshold           int       `json:"threshold"`
	LastFailureAt       time.Time `json:"last_failure_at"`
}

// NewKanbanCircuitBreakerOpenedEvent creates a new circuit breaker opened event.
func NewKanbanCircuitBreakerOpenedEvent(consecutiveFailures, threshold int, lastFailureAt time.Time) KanbanCircuitBreakerOpenedEvent {
	return KanbanCircuitBreakerOpenedEvent{
		BaseEvent:           NewBaseEvent(TypeKanbanCircuitBreakerOpened, ""),
		ConsecutiveFailures: consecutiveFailures,
		Threshold:           threshold,
		LastFailureAt:       lastFailureAt,
	}
}
