package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// WorkflowUpdateMsg signals workflow state change.
type WorkflowUpdateMsg struct {
	State *core.WorkflowState
}

// TaskUpdateMsg signals task status change.
type TaskUpdateMsg struct {
	TaskID   core.TaskID
	Status   core.TaskStatus
	Progress float64
	Error    string
}

// LogMsg adds a log entry.
type LogMsg struct {
	Time    time.Time
	Level   string
	Message string
}

// ErrorMsg signals an error.
type ErrorMsg struct {
	Error error
}

// SpinnerTickMsg updates spinner animation.
type SpinnerTickMsg time.Time

// QuitMsg signals that the TUI should quit.
type QuitMsg struct{}

// waitForWorkflowUpdate creates a command to wait for updates.
func waitForWorkflowUpdate() tea.Cmd {
	return func() tea.Msg {
		// This would be connected to actual workflow updates via channel
		time.Sleep(100 * time.Millisecond)
		return nil
	}
}

// retryTask creates a command to retry a task.
func retryTask(id core.TaskID) tea.Cmd {
	return func() tea.Msg {
		// Signal retry to workflow runner (would use channel in real impl)
		return nil
	}
}

// SendWorkflowUpdate creates a workflow update message.
func SendWorkflowUpdate(state *core.WorkflowState) tea.Msg {
	return WorkflowUpdateMsg{State: state}
}

// SendTaskUpdate creates a task update message.
func SendTaskUpdate(taskID core.TaskID, status core.TaskStatus, progress float64, err string) tea.Msg {
	return TaskUpdateMsg{
		TaskID:   taskID,
		Status:   status,
		Progress: progress,
		Error:    err,
	}
}

// SendLog creates a log message.
func SendLog(level, message string) tea.Msg {
	return LogMsg{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}
}

// SendError creates an error message.
func SendError(err error) tea.Msg {
	return ErrorMsg{Error: err}
}
