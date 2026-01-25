package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TasksPanel displays workflow tasks with progress tracking
type TasksPanel struct {
	state        *core.WorkflowState
	width        int
	height       int
	visible      bool
	scrollY      int // Scroll offset for long task lists
	maxTasks     int // Maximum visible tasks before scrolling
	dirty        bool // Indicates state has changed since last render
	lastTaskHash string // Hash of task states for change detection
}

// NewTasksPanel creates a new tasks panel
func NewTasksPanel() *TasksPanel {
	return &TasksPanel{
		visible:  false,
		maxTasks: 15,
	}
}

// SetState updates the workflow state with change detection
func (p *TasksPanel) SetState(state *core.WorkflowState) {
	p.state = state
	// Calculate hash of current task states for change detection
	newHash := p.computeTaskHash()
	if newHash != p.lastTaskHash {
		p.dirty = true
		p.lastTaskHash = newHash
	}
}

// computeTaskHash creates a simple hash of task statuses for change detection
func (p *TasksPanel) computeTaskHash() string {
	if p.state == nil || len(p.state.TaskOrder) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, taskID := range p.state.TaskOrder {
		if task, ok := p.state.Tasks[taskID]; ok {
			sb.WriteString(string(task.Status))
		}
	}
	return sb.String()
}

// IsDirty returns true if state has changed since last render
func (p *TasksPanel) IsDirty() bool {
	return p.dirty
}

// ClearDirty clears the dirty flag after rendering
func (p *TasksPanel) ClearDirty() {
	p.dirty = false
}

// Toggle toggles panel visibility
func (p *TasksPanel) Toggle() {
	p.visible = !p.visible
	p.scrollY = 0
}

// Show makes the panel visible
func (p *TasksPanel) Show() {
	p.visible = true
}

// Hide hides the panel
func (p *TasksPanel) Hide() {
	p.visible = false
}

// IsVisible returns whether the panel is visible
func (p *TasksPanel) IsVisible() bool {
	return p.visible
}

// SetSize sets the panel dimensions
func (p *TasksPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
	// Calculate max visible tasks based on height
	p.maxTasks = (height - 10) // Account for header, footer, borders
	if p.maxTasks < 5 {
		p.maxTasks = 5
	}
}

// ScrollUp scrolls the task list up
func (p *TasksPanel) ScrollUp() {
	if p.scrollY > 0 {
		p.scrollY--
	}
}

// ScrollDown scrolls the task list down
func (p *TasksPanel) ScrollDown() {
	taskCount := p.getTaskCount()
	maxScroll := taskCount - p.maxTasks
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollY < maxScroll {
		p.scrollY++
	}
}

// getTaskCount returns the number of tasks
func (p *TasksPanel) getTaskCount() int {
	if p.state == nil {
		return 0
	}
	return len(p.state.TaskOrder)
}

// HasTasks returns true if there are tasks to display
func (p *TasksPanel) HasTasks() bool {
	return p.getTaskCount() > 0
}

// taskStats calculates task statistics
func (p *TasksPanel) taskStats() (completed, running, pending, failed, skipped, total int) {
	if p.state == nil {
		return
	}
	total = len(p.state.TaskOrder)
	for _, taskID := range p.state.TaskOrder {
		task, ok := p.state.Tasks[taskID]
		if !ok {
			continue
		}
		switch task.Status {
		case core.TaskStatusCompleted:
			completed++
		case core.TaskStatusRunning:
			running++
		case core.TaskStatusPending:
			pending++
		case core.TaskStatusFailed:
			failed++
		case core.TaskStatusSkipped:
			skipped++
		}
	}
	return
}

// Render renders the tasks panel
func (p *TasksPanel) Render() string {
	if !p.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	completedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e"))

	runningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#eab308"))

	pendingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	failedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444"))

	skippedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Strikethrough(true)

	progressBarFilled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e"))

	progressBarEmpty := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#374151"))

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2).
		Width(p.width - 2)

	// Calculate stats
	completed, running, pending, failed, skipped, total := p.taskStats()

	var sb strings.Builder

	// Header with stats
	statsStr := fmt.Sprintf("(%d/%d)", completed, total)
	if running > 0 {
		statsStr = fmt.Sprintf("(%d/%d, %d running)", completed, total, running)
	}
	sb.WriteString(headerStyle.Render("◆ Issues " + statsStr))
	sb.WriteString("\n\n")

	// If no tasks, show empty state
	if !p.HasTasks() {
		sb.WriteString(dimStyle.Render("No issues yet.\n\n"))
		sb.WriteString(dimStyle.Render("Issues will appear during the execute phase.\n"))
		sb.WriteString(dimStyle.Render("Run /run or /execute to start.\n\n"))
		sb.WriteString(dimStyle.Render("Press Ctrl+I or Esc to close"))
		return boxStyle.Render(sb.String())
	}

	// Determine visible range
	taskCount := p.getTaskCount()
	startIdx := p.scrollY
	endIdx := startIdx + p.maxTasks
	if endIdx > taskCount {
		endIdx = taskCount
	}

	// Scroll indicator (top)
	if startIdx > 0 {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more above\n", startIdx)))
	}

	// Task list
	for i := startIdx; i < endIdx; i++ {
		taskID := p.state.TaskOrder[i]
		task, ok := p.state.Tasks[taskID]
		if !ok {
			continue
		}

		var icon, line string
		var style lipgloss.Style

		switch task.Status {
		case core.TaskStatusCompleted:
			icon = "✓"
			style = completedStyle
		case core.TaskStatusRunning:
			icon = "●"
			style = runningStyle
		case core.TaskStatusPending:
			icon = "○"
			style = pendingStyle
		case core.TaskStatusFailed:
			icon = "✗"
			style = failedStyle
		case core.TaskStatusSkipped:
			icon = "⊘"
			style = skippedStyle
		default:
			icon = "○"
			style = pendingStyle
		}

		// Task name (truncate if too long)
		name := task.Name
		if name == "" {
			name = string(task.ID)
		}
		maxNameLen := p.width - 20
		if maxNameLen < 20 {
			maxNameLen = 20
		}
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}

		line = fmt.Sprintf("  %s %s", icon, name)

		// Add error indicator for failed tasks
		if task.Status == core.TaskStatusFailed && task.Error != "" {
			errMsg := task.Error
			if len(errMsg) > 30 {
				errMsg = errMsg[:27] + "..."
			}
			line += dimStyle.Render(fmt.Sprintf(" (%s)", errMsg))
		}

		sb.WriteString(style.Render(line))
		sb.WriteString("\n")
	}

	// Scroll indicator (bottom)
	remaining := taskCount - endIdx
	if remaining > 0 {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more below\n", remaining)))
	}

	// Progress summary bar
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  ─────────────────────────────────────"))
	sb.WriteString("\n")

	// Overall progress bar
	barWidth := 30
	progress := 0.0
	if total > 0 {
		progress = float64(completed) / float64(total) * 100
	}
	filled := int(float64(barWidth) * progress / 100)
	if filled > barWidth {
		filled = barWidth
	}
	progressBar := progressBarFilled.Render(strings.Repeat("█", filled)) +
		progressBarEmpty.Render(strings.Repeat("░", barWidth-filled))

	sb.WriteString(fmt.Sprintf("  Progress: %s %.0f%%\n", progressBar, progress))

	// Status summary
	var statusParts []string
	if completed > 0 {
		statusParts = append(statusParts, completedStyle.Render(fmt.Sprintf("%d done", completed)))
	}
	if running > 0 {
		statusParts = append(statusParts, runningStyle.Render(fmt.Sprintf("%d running", running)))
	}
	if pending > 0 {
		statusParts = append(statusParts, pendingStyle.Render(fmt.Sprintf("%d pending", pending)))
	}
	if failed > 0 {
		statusParts = append(statusParts, failedStyle.Render(fmt.Sprintf("%d failed", failed)))
	}
	if skipped > 0 {
		statusParts = append(statusParts, skippedStyle.Render(fmt.Sprintf("%d skipped", skipped)))
	}

	if len(statusParts) > 0 {
		sb.WriteString("  ")
		sb.WriteString(strings.Join(statusParts, dimStyle.Render(" · ")))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  ↑/↓ scroll · Ctrl+I or Esc to close"))

	return boxStyle.Render(sb.String())
}

// CompactRender renders a single-line version for the header/status bar
func (p *TasksPanel) CompactRender() string {
	if !p.HasTasks() {
		return ""
	}

	completed, running, _, failed, _, total := p.taskStats()

	// Determine color based on status
	var color lipgloss.Color
	if failed > 0 {
		color = lipgloss.Color("#ef4444") // Red
	} else if running > 0 {
		color = lipgloss.Color("#eab308") // Yellow
	} else if completed == total {
		color = lipgloss.Color("#22c55e") // Green
	} else {
		color = lipgloss.Color("#6B7280") // Gray
	}

	style := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	// Mini progress indicator
	icon := "○"
	if completed == total && total > 0 {
		icon = "✓"
	} else if running > 0 {
		icon = "●"
	} else if failed > 0 {
		icon = "✗"
	}

	return fmt.Sprintf("%s %s", style.Render(icon), dimStyle.Render(fmt.Sprintf("%d/%d", completed, total)))
}
