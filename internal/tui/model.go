package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
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
	droppedEvents int64 // track dropped events
}

// TaskView represents a task in the TUI.
type TaskView struct {
	ID       core.TaskID
	Name     string
	Phase    core.Phase
	Status   core.TaskStatus
	Progress float64
	Duration time.Duration
	Error    string
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

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick(),
		waitForWorkflowUpdate(),
	)
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
		return m, waitForWorkflowUpdate()

	case PhaseUpdateMsg:
		m.currentPhase = msg.Phase
		return m, nil

	case TaskUpdateMsg:
		m.updateTask(msg)
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
		return m, nil

	case DroppedEventsMsg:
		m.droppedEvents = msg.Count
		return m, nil

	case SpinnerTickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case ErrorMsg:
		m.err = msg.Error
		return m, nil

	case QuitMsg:
		return m, tea.Quit
	}

	return m, nil
}

// handleKeyPress handles keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}
		return m, nil

	case "down", "j":
		if m.selectedIdx < len(m.tasks)-1 {
			m.selectedIdx++
		}
		return m, nil

	case "l":
		m.showLogs = !m.showLogs
		return m, nil

	case "enter":
		// Show task details
		return m, nil

	case "r":
		// Retry selected task
		if m.selectedIdx < len(m.tasks) {
			task := m.tasks[m.selectedIdx]
			if task.Status == core.TaskStatusFailed {
				return m, retryTask(task.ID)
			}
		}
		return m, nil
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

	return HeaderStyle.Render(
		fmt.Sprintf("Quorum AI - Phase: %s - Status: %s", phase, status))
}

// renderProgress renders the overall progress bar.
func (m Model) renderProgress() string {
	if m.workflow == nil {
		return ""
	}

	completed := 0
	total := len(m.tasks)
	for _, t := range m.tasks {
		if t.Status == core.TaskStatusCompleted {
			completed++
		}
	}

	percentage := 0.0
	if total > 0 {
		percentage = float64(completed) / float64(total) * 100
	}

	bar := renderProgressBar(percentage, m.width-20)
	return fmt.Sprintf("Progress: %s %.1f%%", bar, percentage)
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

		if task.Duration > 0 {
			line += fmt.Sprintf(" [%s]", task.Duration.Round(time.Second))
		}

		s += style.Render(line) + "\n"
	}

	return s
}

// renderFooter renders the footer with keybindings.
func (m Model) renderFooter() string {
	footer := "q: quit | j/k: navigate | l: logs | r: retry | enter: details"
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

// buildTaskViews converts workflow tasks to views.
func (m Model) buildTaskViews(state *core.WorkflowState) []*TaskView {
	if state == nil {
		return nil
	}

	views := make([]*TaskView, 0, len(state.Tasks))
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
			ID:       task.ID,
			Name:     task.Name,
			Phase:    task.Phase,
			Status:   task.Status,
			Duration: duration,
			Error:    task.Error,
		})
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
