package core

import (
	"fmt"
	"time"
)

// TaskID uniquely identifies a task within a workflow.
type TaskID string

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped"
)

// Task represents a unit of work in the orchestration workflow.
type Task struct {
	ID           TaskID
	Phase        Phase
	Name         string
	Description  string
	Status       TaskStatus
	CLI          string // Agent CLI to use (claude, gemini, etc.)
	Model        string // Specific model override
	Dependencies []TaskID
	Outputs      []Artifact
	TokensIn     int
	TokensOut    int
	CostUSD      float64
	Retries      int
	MaxRetries   int
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Error        string
}

// NewTask creates a new task with required fields.
func NewTask(id TaskID, name string, phase Phase) *Task {
	return &Task{
		ID:         id,
		Phase:      phase,
		Name:       name,
		Status:     TaskStatusPending,
		MaxRetries: 3,
	}
}

// WithDescription sets the task description.
func (t *Task) WithDescription(desc string) *Task {
	t.Description = desc
	return t
}

// WithCLI sets the agent CLI to use.
func (t *Task) WithCLI(cli string) *Task {
	t.CLI = cli
	return t
}

// WithModel sets the model override.
func (t *Task) WithModel(model string) *Task {
	t.Model = model
	return t
}

// WithDependencies sets the task dependencies.
func (t *Task) WithDependencies(deps ...TaskID) *Task {
	t.Dependencies = deps
	return t
}

// WithMaxRetries sets the maximum retry count.
func (t *Task) WithMaxRetries(maxRetries int) *Task {
	t.MaxRetries = maxRetries
	return t
}

// IsReady returns true if all dependencies are completed.
func (t *Task) IsReady(completed map[TaskID]bool) bool {
	if t.Status != TaskStatusPending {
		return false
	}
	for _, dep := range t.Dependencies {
		if !completed[dep] {
			return false
		}
	}
	return true
}

// MarkRunning transitions the task to running state.
func (t *Task) MarkRunning() error {
	if t.Status != TaskStatusPending {
		return fmt.Errorf("cannot start task in %s state", t.Status)
	}
	t.Status = TaskStatusRunning
	now := time.Now()
	t.StartedAt = &now
	return nil
}

// MarkCompleted transitions the task to completed state.
func (t *Task) MarkCompleted(outputs []Artifact) error {
	if t.Status != TaskStatusRunning {
		return fmt.Errorf("cannot complete task in %s state", t.Status)
	}
	t.Status = TaskStatusCompleted
	t.Outputs = outputs
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// MarkFailed transitions the task to failed state.
func (t *Task) MarkFailed(err error) error {
	if t.Status != TaskStatusRunning {
		return fmt.Errorf("cannot fail task in %s state", t.Status)
	}
	t.Status = TaskStatusFailed
	t.Error = err.Error()
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// MarkSkipped transitions the task to skipped state.
func (t *Task) MarkSkipped(reason string) error {
	if t.Status != TaskStatusPending {
		return fmt.Errorf("cannot skip task in %s state", t.Status)
	}
	t.Status = TaskStatusSkipped
	t.Error = reason
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

// CanRetry returns true if the task can be retried.
func (t *Task) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.Retries < t.MaxRetries
}

// Reset prepares the task for retry.
func (t *Task) Reset() error {
	if !t.CanRetry() {
		return fmt.Errorf("cannot retry task: retries=%d, max=%d", t.Retries, t.MaxRetries)
	}
	t.Retries++
	t.Status = TaskStatusPending
	t.Error = ""
	t.StartedAt = nil
	t.CompletedAt = nil
	return nil
}

// Validate checks task invariants.
func (t *Task) Validate() error {
	if t.ID == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "TASK_ID_REQUIRED",
			Message:  "task ID cannot be empty",
		}
	}
	if t.Name == "" {
		return &DomainError{
			Category: ErrCatValidation,
			Code:     "TASK_NAME_REQUIRED",
			Message:  "task name cannot be empty",
		}
	}
	return nil
}

// Duration returns the task execution duration.
func (t *Task) Duration() time.Duration {
	if t.StartedAt == nil {
		return 0
	}
	end := time.Now()
	if t.CompletedAt != nil {
		end = *t.CompletedAt
	}
	return end.Sub(*t.StartedAt)
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusSkipped
}

// IsSuccess returns true if the task completed successfully.
func (t *Task) IsSuccess() bool {
	return t.Status == TaskStatusCompleted
}
