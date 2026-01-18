package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// Model is the main TUI model.
type Model struct {
	workflow      *core.WorkflowState
	currentPhase  core.Phase // Track current phase separately.
	tasks         []*TaskView
	selectedIdx   int
	width         int
	height        int
	ready         bool
	spinner       SpinnerModel
	logs          []LogEntry
	showLogs      bool
	err           error
	droppedEvents int64            // track dropped events
	eventAdapter  *EventBusAdapter // EventBus adapter for real-time events
	stateManager  *UIStateManager
	controlPlane  *control.ControlPlane
	isPaused      bool
}

// TaskView represents a task in the TUI.
type TaskView struct {
	ID        core.TaskID
	Name      string
	Phase     core.Phase
	Status    core.TaskStatus
	Progress  float64
	Duration  time.Duration
	StartedAt *time.Time
	Error     string
}

// LogEntry represents a log line.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

// New creates a new TUI model.
func New() Model {
	return Model{
		tasks:   make([]*TaskView, 0),
		logs:    make([]LogEntry, 0),
		spinner: NewSpinner(),
	}
}

// NewWithStateManager creates a Model with UI state persistence.
func NewWithStateManager(baseDir string) Model {
	m := New()
	m.stateManager = NewUIStateManager(baseDir)
	if err := m.stateManager.Load(); err != nil {
		// Log but continue with defaults
	}

	// Restore state
	state := m.stateManager.Get()
	m.selectedIdx = state.SelectedTask
	m.showLogs = state.ShowLogs

	return m
}

// NewWithEventBus creates a Model connected to an EventBus.
func NewWithEventBus(bus *events.EventBus) Model {
	m := New()
	if bus != nil {
		m.eventAdapter = NewEventBusAdapter(bus)
	}
	return m
}

// NewWithControlPlane creates a Model with ControlPlane support.
func NewWithControlPlane(cp *control.ControlPlane) Model {
	m := New()
	m.controlPlane = cp
	return m
}

// SetControlPlane sets the control plane for the model.
func (m *Model) SetControlPlane(cp *control.ControlPlane) {
	m.controlPlane = cp
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick(),
		durationTick(),
	}

	if m.eventAdapter != nil {
		cmds = append(cmds, waitForEventBusUpdate(m.eventAdapter))
	} else {
		cmds = append(cmds, waitForWorkflowUpdate())
	}

	return tea.Batch(cmds...)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case WorkflowUpdateMsg:
		m.workflow = msg.State
		m.tasks = m.buildTaskViews(msg.State)
		if msg.State != nil && m.stateManager != nil {
			m.stateManager.SetLastWorkflow(msg.State.WorkflowID)
		}
		if m.eventAdapter != nil {
			return m, waitForEventBusUpdate(m.eventAdapter)
		}
		return m, waitForWorkflowUpdate()

	case PhaseUpdateMsg:
		m.currentPhase = msg.Phase
		if m.eventAdapter != nil {
			return m, waitForEventBusUpdate(m.eventAdapter)
		}
		return m, nil

	case TaskUpdateMsg:
		m.updateTask(msg)
		if m.eventAdapter != nil {
			return m, waitForEventBusUpdate(m.eventAdapter)
		}
		return m, nil

	case LogMsg:
		m.logs = append(m.logs, LogEntry{
			Time:    msg.Time,
			Level:   msg.Level,
			Message: msg.Message,
		})
		// Keep last 100 logs
		if len(m.logs) > 100 {
			m.logs = m.logs[1:]
		}
		if m.eventAdapter != nil {
			return m, waitForEventBusUpdate(m.eventAdapter)
		}
		return m, nil

	case MetricsUpdateMsg:
		// Update metrics display (future implementation)
		// For now, just re-subscribe for next event
		if m.eventAdapter != nil {
			return m, waitForEventBusUpdate(m.eventAdapter)
		}
		return m, nil

	case DroppedEventsMsg:
		m.droppedEvents = msg.Count
		return m, nil

	case SpinnerTickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case DurationTickMsg:
		return m, durationTick()

	case ErrorMsg:
		m.err = msg.Error
		return m, nil

	case QuitMsg:
		return m, tea.Quit

	case PausedMsg:
		m.isPaused = true
		return m, nil

	case ResumedMsg:
		m.isPaused = false
		return m, nil

	case TaskRetryQueuedMsg:
		// Update task status to show retry queued
		for i, task := range m.tasks {
			if task.ID == msg.TaskID {
				m.tasks[i].Status = core.TaskStatusPending
			}
		}
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.stateManager != nil {
			_ = m.stateManager.Close()
		}
		return m, tea.Quit

	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
			if m.stateManager != nil {
				m.stateManager.SetSelectedTask(m.selectedIdx)
			}
		}
		return m, nil

	case "down", "j":
		if m.selectedIdx < len(m.tasks)-1 {
			m.selectedIdx++
			if m.stateManager != nil {
				m.stateManager.SetSelectedTask(m.selectedIdx)
			}
		}
		return m, nil

	case "l":
		m.showLogs = !m.showLogs
		if m.stateManager != nil {
			m.stateManager.SetShowLogs(m.showLogs)
		}
		return m, nil

	case "enter":
		// Show task details
		return m, nil

	case "r":
		// Retry selected task
		if m.selectedIdx < len(m.tasks) {
			task := m.tasks[m.selectedIdx]
			if task.Status == core.TaskStatusFailed {
				return m, RetryTaskCmd(m.controlPlane, task.ID)
			}
		}
		return m, nil

	case "p":
		// Toggle pause/resume
		if !m.isPaused {
			return m, PauseCmd(m.controlPlane)
		}
		return m, ResumeCmd(m.controlPlane)

	case "c":
		// Cancel workflow
		return m, CancelCmd(m.controlPlane)
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.err != nil {
		return m.renderError()
	}

	if m.showLogs {
		return m.renderLogs()
	}

	return m.renderMain()
}

// renderMain renders the main view.
func (m Model) renderMain() string {
	s := m.renderHeader()
	s += "\n\n"
	s += m.renderProgress()
	s += "\n\n"
	s += m.renderTasks()
	s += "\n\n"
	s += m.renderFooter()

	return s
}

// renderHeader renders the header section.
func (m Model) renderHeader() string {
	phase := string(m.currentPhase)
	if phase == "" {
		phase = "initializing"
	}

	status := "running"
	if m.workflow != nil {
		status = string(m.workflow.Status)
	}

	if m.err != nil {
		status = "error"
	}

	if m.isPaused {
		status = "PAUSED"
	}

	return HeaderStyle.Render(
		fmt.Sprintf("Quorum AI - Phase: %s - Status: %s", phase, status))
}

// progressStats calculates task completion statistics.
type progressStats struct {
	total     int
	completed int
	failed    int
	skipped   int
	running   int
	pending   int
}

func (m Model) getProgressStats() progressStats {
	stats := progressStats{
		total: len(m.tasks),
	}

	for _, t := range m.tasks {
		switch t.Status {
		case core.TaskStatusPending:
			stats.pending++
		case core.TaskStatusRunning:
			stats.running++
		case core.TaskStatusCompleted:
			stats.completed++
		case core.TaskStatusFailed:
			stats.failed++
		case core.TaskStatusSkipped:
			stats.skipped++
		}
	}

	return stats
}

// finished returns the count of terminal-state tasks.
func (s progressStats) finished() int {
	return s.completed + s.failed + s.skipped
}

// percentage returns the completion percentage.
func (s progressStats) percentage() float64 {
	if s.total == 0 {
		return 0
	}
	return float64(s.finished()) / float64(s.total) * 100
}

// renderProgress renders the overall progress bar.
// Progress is calculated as (completed + failed + skipped) / total.
func (m Model) renderProgress() string {
	if m.workflow == nil {
		return ""
	}

	stats := m.getProgressStats()
	percentage := stats.percentage()
	bar := renderProgressBar(percentage, m.width-30)

	// Show breakdown if there are failures or skips
	if stats.failed > 0 || stats.skipped > 0 {
		return fmt.Sprintf("Progress: %s %.1f%% (%d/%d done, %d failed, %d skipped)",
			bar, percentage, stats.completed, stats.total, stats.failed, stats.skipped)
	}

	return fmt.Sprintf("Progress: %s %.1f%% (%d/%d)", bar, percentage, stats.completed, stats.total)
}

// renderTasks renders the task list.
func (m Model) renderTasks() string {
	if len(m.tasks) == 0 {
		return "No tasks"
	}

	s := "Tasks:\n"

	for i, task := range m.tasks {
		style := TaskStyle
		if i == m.selectedIdx {
			style = SelectedTaskStyle
		}

		icon := m.statusIcon(task.Status)
		line := fmt.Sprintf("%s %s (%s)", icon, task.Name, task.Status)

		if task.Status == core.TaskStatusRunning {
			line += " " + m.spinner.View()
		}

		duration := m.getTaskDuration(task)
		if duration > 0 {
			line += fmt.Sprintf(" [%s]", formatDuration(duration))
		}

		s += style.Render(line) + "\n"
	}

	return s
}

// getTaskDuration returns the task duration, calculating live for running tasks.
func (m Model) getTaskDuration(task *TaskView) time.Duration {
	if task.Status == core.TaskStatusRunning && task.StartedAt != nil {
		return time.Since(*task.StartedAt)
	}
	return task.Duration
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%02ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", hours, mins)
}

// renderFooter renders the footer with keybindings.
func (m Model) renderFooter() string {
	pauseKey := "p: pause"
	if m.isPaused {
		pauseKey = "p: resume"
	}
	footer := fmt.Sprintf("q: quit | j/k: navigate | l: logs | r: retry | %s | c: cancel", pauseKey)
	if m.droppedEvents > 0 {
		footer += fmt.Sprintf(" | ⚠ %d dropped", m.droppedEvents)
	}
	return FooterStyle.Render(footer)
}

// renderError renders error view.
func (m Model) renderError() string {
	return ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
}

// renderLogs renders the logs view.
func (m Model) renderLogs() string {
	s := HeaderStyle.Render("Logs (press 'l' to return)") + "\n\n"

	start := 0
	if len(m.logs) > 20 {
		start = len(m.logs) - 20
	}

	for _, log := range m.logs[start:] {
		style := LogStyle
		if log.Level == "error" {
			style = ErrorLogStyle
		} else if log.Level == "warn" {
			style = WarnLogStyle
		}

		s += style.Render(fmt.Sprintf("[%s] %s: %s",
			log.Time.Format("15:04:05"),
			log.Level,
			log.Message)) + "\n"
	}

	return s
}

// statusIcon returns an icon for task status.
func (m Model) statusIcon(status core.TaskStatus) string {
	switch status {
	case core.TaskStatusPending:
		return "○"
	case core.TaskStatusRunning:
		return "●"
	case core.TaskStatusCompleted:
		return "✓"
	case core.TaskStatusFailed:
		return "✗"
	case core.TaskStatusSkipped:
		return "⊘"
	default:
		return "?"
	}
}

// buildTaskViews converts workflow tasks to views using TaskOrder for stable ordering.
func (m Model) buildTaskViews(state *core.WorkflowState) []*TaskView {
	if state == nil {
		return nil
	}

	// Use TaskOrder for deterministic ordering
	views := make([]*TaskView, 0, len(state.TaskOrder))
	for _, taskID := range state.TaskOrder {
		task, exists := state.Tasks[taskID]
		if !exists {
			// Task in order but not in map - skip (shouldn't happen, but be safe)
			continue
		}

		var duration time.Duration
		if task.StartedAt != nil {
			if task.CompletedAt != nil {
				duration = task.CompletedAt.Sub(*task.StartedAt)
			} else {
				duration = time.Since(*task.StartedAt)
			}
		}

		views = append(views, &TaskView{
			ID:        task.ID,
			Name:      task.Name,
			Phase:     task.Phase,
			Status:    task.Status,
			Duration:  duration,
			StartedAt: task.StartedAt,
			Error:     task.Error,
		})
	}

	// If TaskOrder is empty but Tasks has items, fall back to map iteration
	// (for backwards compatibility with old state files)
	if len(views) == 0 && len(state.Tasks) > 0 {
		for _, task := range state.Tasks {
			var duration time.Duration
			if task.StartedAt != nil {
				if task.CompletedAt != nil {
					duration = task.CompletedAt.Sub(*task.StartedAt)
				} else {
					duration = time.Since(*task.StartedAt)
				}
			}

			views = append(views, &TaskView{
				ID:        task.ID,
				Name:      task.Name,
				Phase:     task.Phase,
				Status:    task.Status,
				Duration:  duration,
				StartedAt: task.StartedAt,
				Error:     task.Error,
			})
		}
	}

	return views
}

// updateTask updates a specific task view.
func (m *Model) updateTask(msg TaskUpdateMsg) {
	for i, task := range m.tasks {
		if task.ID == msg.TaskID {
			m.tasks[i].Status = msg.Status
			m.tasks[i].Progress = msg.Progress
			m.tasks[i].Error = msg.Error
			break
		}
	}
}

// renderProgressBar renders a progress bar.
func renderProgressBar(percentage float64, width int) string {
	if width <= 0 {
		width = 20
	}
	filled := int(float64(width) * percentage / 100)
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	return bar
}

// SetWorkflow sets the workflow state.
func (m *Model) SetWorkflow(state *core.WorkflowState) {
	m.workflow = state
	m.tasks = m.buildTaskViews(state)
}

// AddLog adds a log entry.
func (m *Model) AddLog(level, message string) {
	m.logs = append(m.logs, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
	if len(m.logs) > 100 {
		m.logs = m.logs[1:]
	}
}
