package events

import "time"

// Event type constants for task events.
const (
	TypeTaskCreated   = "task_created"
	TypeTaskStarted   = "task_started"
	TypeTaskProgress  = "task_progress"
	TypeTaskCompleted = "task_completed"
	TypeTaskFailed    = "task_failed"
	TypeTaskSkipped   = "task_skipped"
	TypeTaskRetry     = "task_retry"
)

// TaskCreatedEvent is emitted when a task is created.
type TaskCreatedEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Phase  string `json:"phase"`
	Name   string `json:"name"`
	Agent  string `json:"agent"`
	Model  string `json:"model"`
}

// NewTaskCreatedEvent creates a new task created event.
func NewTaskCreatedEvent(workflowID, taskID, phase, name, agent, model string) TaskCreatedEvent {
	return TaskCreatedEvent{
		BaseEvent: NewBaseEvent(TypeTaskCreated, workflowID),
		TaskID:    taskID,
		Phase:     phase,
		Name:      name,
		Agent:     agent,
		Model:     model,
	}
}

// TaskStartedEvent is emitted when a task begins execution.
type TaskStartedEvent struct {
	BaseEvent
	TaskID       string `json:"task_id"`
	WorktreePath string `json:"worktree_path,omitempty"`
}

// NewTaskStartedEvent creates a new task started event.
func NewTaskStartedEvent(workflowID, taskID, worktreePath string) TaskStartedEvent {
	return TaskStartedEvent{
		BaseEvent:    NewBaseEvent(TypeTaskStarted, workflowID),
		TaskID:       taskID,
		WorktreePath: worktreePath,
	}
}

// TaskProgressEvent is emitted during task execution.
type TaskProgressEvent struct {
	BaseEvent
	TaskID    string  `json:"task_id"`
	Progress  float64 `json:"progress"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	Message   string  `json:"message,omitempty"`
}

// NewTaskProgressEvent creates a new task progress event.
func NewTaskProgressEvent(workflowID, taskID string, progress float64, tokensIn, tokensOut int, message string) TaskProgressEvent {
	return TaskProgressEvent{
		BaseEvent: NewBaseEvent(TypeTaskProgress, workflowID),
		TaskID:    taskID,
		Progress:  progress,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		Message:   message,
	}
}

// TaskCompletedEvent is emitted when a task finishes successfully.
type TaskCompletedEvent struct {
	BaseEvent
	TaskID    string        `json:"task_id"`
	Duration  time.Duration `json:"duration"`
	TokensIn  int           `json:"tokens_in"`
	TokensOut int           `json:"tokens_out"`
	CostUSD   float64       `json:"cost_usd"`
}

// NewTaskCompletedEvent creates a new task completed event.
func NewTaskCompletedEvent(workflowID, taskID string, duration time.Duration, tokensIn, tokensOut int, costUSD float64) TaskCompletedEvent {
	return TaskCompletedEvent{
		BaseEvent: NewBaseEvent(TypeTaskCompleted, workflowID),
		TaskID:    taskID,
		Duration:  duration,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		CostUSD:   costUSD,
	}
}

// TaskFailedEvent is emitted when a task fails.
type TaskFailedEvent struct {
	BaseEvent
	TaskID    string `json:"task_id"`
	Error     string `json:"error"`
	Retryable bool   `json:"retryable"`
}

// NewTaskFailedEvent creates a new task failed event.
func NewTaskFailedEvent(workflowID, taskID string, err error, retryable bool) TaskFailedEvent {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return TaskFailedEvent{
		BaseEvent: NewBaseEvent(TypeTaskFailed, workflowID),
		TaskID:    taskID,
		Error:     errStr,
		Retryable: retryable,
	}
}

// TaskSkippedEvent is emitted when a task is skipped.
type TaskSkippedEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// NewTaskSkippedEvent creates a new task skipped event.
func NewTaskSkippedEvent(workflowID, taskID, reason string) TaskSkippedEvent {
	return TaskSkippedEvent{
		BaseEvent: NewBaseEvent(TypeTaskSkipped, workflowID),
		TaskID:    taskID,
		Reason:    reason,
	}
}

// TaskRetryEvent is emitted when a task is being retried.
type TaskRetryEvent struct {
	BaseEvent
	TaskID      string `json:"task_id"`
	AttemptNum  int    `json:"attempt_num"`
	MaxAttempts int    `json:"max_attempts"`
	Error       string `json:"error"`
}

// NewTaskRetryEvent creates a new task retry event.
func NewTaskRetryEvent(workflowID, taskID string, attemptNum, maxAttempts int, err error) TaskRetryEvent {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	return TaskRetryEvent{
		BaseEvent:   NewBaseEvent(TypeTaskRetry, workflowID),
		TaskID:      taskID,
		AttemptNum:  attemptNum,
		MaxAttempts: maxAttempts,
		Error:       errStr,
	}
}
