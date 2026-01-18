package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// WorkflowUpdateMsg signals workflow state change.
type WorkflowUpdateMsg struct {
	State *core.WorkflowState
}

// PhaseUpdateMsg signals a phase change.
type PhaseUpdateMsg struct {
	Phase core.Phase
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

// DroppedEventsMsg notifies the UI of dropped events.
type DroppedEventsMsg struct {
	Count int64
}

// MetricsUpdateMsg provides real-time metrics updates.
type MetricsUpdateMsg struct {
	TotalCostUSD   float64
	TotalTokensIn  int
	TotalTokensOut int
	Duration       time.Duration
}

// SpinnerTickMsg updates spinner animation.
type SpinnerTickMsg time.Time

// DurationTickMsg triggers duration refresh for running tasks.
type DurationTickMsg struct{}

// QuitMsg signals that the TUI should quit.
type QuitMsg struct{}

// waitForEventBusUpdate creates a command that waits for events from EventBus.
// This replaces the old polling stub with real event subscription.
func waitForEventBusUpdate(adapter *EventBusAdapter) tea.Cmd {
	return func() tea.Msg {
		if adapter == nil {
			// Fallback for when adapter not available
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		msg, ok := <-adapter.MsgChannel()
		if !ok {
			return QuitMsg{}
		}
		return msg
	}
}

// waitForWorkflowUpdate is the legacy stub - kept for compatibility.
// Use waitForEventBusUpdate when EventBus is available.
func waitForWorkflowUpdate() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
}

// durationTick creates a command that triggers duration updates.
func durationTick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return DurationTickMsg{}
	})
}

// ControlPlaneMsg wraps control plane operations.
type ControlPlaneMsg struct {
	Control *control.ControlPlane
}

// PausedMsg signals that the workflow has been paused.
type PausedMsg struct{}

// ResumedMsg signals that the workflow has been resumed.
type ResumedMsg struct{}

// CancelledMsg signals that the workflow has been cancelled.
type CancelledMsg struct{}

// TaskRetryQueuedMsg signals that a task has been queued for retry.
type TaskRetryQueuedMsg struct {
	TaskID core.TaskID
}

// PauseCmd creates a command to pause the workflow.
func PauseCmd(cp *control.ControlPlane) tea.Cmd {
	return func() tea.Msg {
		if cp != nil {
			cp.Pause()
		}
		return PausedMsg{}
	}
}

// ResumeCmd creates a command to resume the workflow.
func ResumeCmd(cp *control.ControlPlane) tea.Cmd {
	return func() tea.Msg {
		if cp != nil {
			cp.Resume()
		}
		return ResumedMsg{}
	}
}

// RetryTaskCmd creates a command to retry a task.
func RetryTaskCmd(cp *control.ControlPlane, taskID core.TaskID) tea.Cmd {
	return func() tea.Msg {
		if cp != nil {
			cp.RetryTask(taskID)
		}
		return TaskRetryQueuedMsg{TaskID: taskID}
	}
}

// CancelCmd creates a command to cancel the workflow.
func CancelCmd(cp *control.ControlPlane) tea.Cmd {
	return func() tea.Msg {
		if cp != nil {
			cp.Cancel()
		}
		return CancelledMsg{}
	}
}

// retryTask creates a command to retry a task (legacy stub for backwards compatibility).
func retryTask(taskID core.TaskID) tea.Cmd {
	return RetryTaskCmd(nil, taskID)
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
