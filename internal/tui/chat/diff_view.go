package chat

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// DiffLine represents a line in the diff view
type DiffLine struct {
	Left     string
	Right    string
	IsCommon bool // true if content is the same
}

// AgentDiffView shows side-by-side comparison between agents
type AgentDiffView struct {
	leftAgent    string
	rightAgent   string
	leftContent  string
	rightContent string
	diffLines    []DiffLine
	viewport     viewport.Model
	width        int
	height       int
	visible      bool
	ready        bool
	// Navigation between agent pairs
	agentPairs  [][2]string // Available pairs to compare
	currentPair int
}

// NewAgentDiffView creates a new diff view
func NewAgentDiffView() *AgentDiffView {
	return &AgentDiffView{
		visible:    false,
		agentPairs: make([][2]string, 0),
	}
}

// SetAgents sets the agents to compare
func (d *AgentDiffView) SetAgents(left, right string) {
	d.leftAgent = left
	d.rightAgent = right
}

// SetContent sets the content from each agent
func (d *AgentDiffView) SetContent(left, right string) {
	d.leftContent = left
	d.rightContent = right
	d.computeDiff()
	d.updateViewport()
}

// AddAgentPair adds an agent pair for navigation
func (d *AgentDiffView) AddAgentPair(agent1, agent2 string) {
	d.agentPairs = append(d.agentPairs, [2]string{agent1, agent2})
}

// ClearPairs clears all agent pairs
func (d *AgentDiffView) ClearPairs() {
	d.agentPairs = make([][2]string, 0)
	d.currentPair = 0
}

// NextPair switches to the next agent pair
func (d *AgentDiffView) NextPair() bool {
	if len(d.agentPairs) <= 1 {
		return false
	}
	d.currentPair = (d.currentPair + 1) % len(d.agentPairs)
	return true
}

// PrevPair switches to the previous agent pair
func (d *AgentDiffView) PrevPair() bool {
	if len(d.agentPairs) <= 1 {
		return false
	}
	d.currentPair--
	if d.currentPair < 0 {
		d.currentPair = len(d.agentPairs) - 1
	}
	return true
}

// GetCurrentPair returns the current agent pair
func (d *AgentDiffView) GetCurrentPair() (left, right string) {
	if len(d.agentPairs) == 0 {
		return d.leftAgent, d.rightAgent
	}
	pair := d.agentPairs[d.currentPair]
	return pair[0], pair[1]
}

// Toggle toggles the diff view visibility
func (d *AgentDiffView) Toggle() {
	d.visible = !d.visible
}

// Show shows the diff view
func (d *AgentDiffView) Show() {
	d.visible = true
}

// Hide hides the diff view
func (d *AgentDiffView) Hide() {
	d.visible = false
}

// IsVisible returns whether the diff view is visible
func (d *AgentDiffView) IsVisible() bool {
	return d.visible
}

// SetSize sets the diff view dimensions
func (d *AgentDiffView) SetSize(width, height int) {
	d.width = width
	d.height = height
	if !d.ready {
		d.viewport = viewport.New(width-4, height-6)
		d.ready = true
	} else {
		d.viewport.Width = width - 4
		d.viewport.Height = height - 6
	}
	d.updateViewport()
}

// ScrollUp scrolls the viewport up
func (d *AgentDiffView) ScrollUp() {
	d.viewport.ScrollUp(1)
}

// ScrollDown scrolls the viewport down
func (d *AgentDiffView) ScrollDown() {
	d.viewport.ScrollDown(1)
}

// computeDiff computes the diff between left and right content
func (d *AgentDiffView) computeDiff() {
	leftLines := strings.Split(d.leftContent, "\n")
	rightLines := strings.Split(d.rightContent, "\n")

	// Simple line-by-line comparison
	// For more sophisticated diff, consider using a proper diff algorithm
	d.diffLines = make([]DiffLine, 0)

	maxLen := len(leftLines)
	if len(rightLines) > maxLen {
		maxLen = len(rightLines)
	}

	for i := 0; i < maxLen; i++ {
		var left, right string
		if i < len(leftLines) {
			left = leftLines[i]
		}
		if i < len(rightLines) {
			right = rightLines[i]
		}

		isCommon := strings.TrimSpace(left) == strings.TrimSpace(right)
		d.diffLines = append(d.diffLines, DiffLine{
			Left:     left,
			Right:    right,
			IsCommon: isCommon,
		})
	}
}

// updateViewport updates the viewport content
func (d *AgentDiffView) updateViewport() {
	if !d.ready {
		return
	}
	d.viewport.SetContent(d.renderDiffContent())
}

// renderDiffContent renders the diff lines
func (d *AgentDiffView) renderDiffContent() string {
	if len(d.diffLines) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
		return dimStyle.Render("No content to compare")
	}

	// Styles
	commonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")) // Green
	diffStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308"))   // Yellow
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	// Calculate column width (half of available width minus separator)
	colWidth := (d.width - 7) / 2
	if colWidth < 20 {
		colWidth = 20
	}

	var sb strings.Builder

	for _, line := range d.diffLines {
		// Truncate and pad lines
		left := truncateOrPad(line.Left, colWidth)
		right := truncateOrPad(line.Right, colWidth)

		var indicator string
		var leftRendered, rightRendered string

		if line.IsCommon {
			indicator = commonStyle.Render("✓")
			leftRendered = textStyle.Render(left)
			rightRendered = textStyle.Render(right)
		} else {
			indicator = diffStyle.Render("≠")
			leftRendered = diffStyle.Render(left)
			rightRendered = diffStyle.Render(right)
		}

		sb.WriteString(leftRendered)
		sb.WriteString(dimStyle.Render(" │ "))
		sb.WriteString(rightRendered)
		sb.WriteString(" ")
		sb.WriteString(indicator)
		sb.WriteString("\n")
	}

	return sb.String()
}

// truncateOrPad truncates or pads a string to exact width
func truncateOrPad(s string, width int) string {
	// Remove any trailing newlines
	s = strings.TrimRight(s, "\n\r")

	runeCount := len([]rune(s))
	if runeCount > width {
		runes := []rune(s)
		return string(runes[:width-1]) + "…"
	}
	return s + strings.Repeat(" ", width-runeCount)
}

// Render renders the full diff view as an overlay
func (d *AgentDiffView) Render() string {
	if !d.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	agentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#06b6d4")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#374151"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 1)

	// Calculate column width
	colWidth := (d.width - 7) / 2
	if colWidth < 20 {
		colWidth = 20
	}

	var sb strings.Builder

	// Title
	title := headerStyle.Render(" Agent Diff")
	if len(d.agentPairs) > 1 {
		title += dimStyle.Render(strings.Repeat(" ", 3) + "(" +
			string('1'+rune(d.currentPair)) + "/" +
			string('0'+rune(len(d.agentPairs))) + ")")
	}
	sb.WriteString(title)
	sb.WriteString("\n")

	// Column headers
	leftHeader := agentStyle.Render(truncateOrPad(d.leftAgent, colWidth))
	rightHeader := agentStyle.Render(truncateOrPad(d.rightAgent, colWidth))
	sb.WriteString(leftHeader)
	sb.WriteString(dimStyle.Render(" │ "))
	sb.WriteString(rightHeader)
	sb.WriteString("\n")

	// Separator
	sep := borderStyle.Render(strings.Repeat("─", colWidth) + "─┼─" + strings.Repeat("─", colWidth))
	sb.WriteString(sep)
	sb.WriteString("\n")

	// Diff content (viewport)
	sb.WriteString(d.viewport.View())
	sb.WriteString("\n")

	// Footer with keybindings
	footer := keyStyle.Render("←→") + dimStyle.Render(" agents") +
		"  " + keyStyle.Render("Tab") + dimStyle.Render(" next pair") +
		"  " + keyStyle.Render("↑↓") + dimStyle.Render(" scroll") +
		"  " + keyStyle.Render("Esc") + dimStyle.Render(" close")
	sb.WriteString(footer)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")). // Purple border for overlay
		BorderBackground(lipgloss.Color("#1f1f23")).
		Background(lipgloss.Color("#1f1f23")).
		Padding(0, 1).
		Width(d.width - 2).
		Height(d.height - 2)

	return boxStyle.Render(sb.String())
}

// HasContent returns true if there's content to diff
func (d *AgentDiffView) HasContent() bool {
	return d.leftContent != "" || d.rightContent != ""
}
