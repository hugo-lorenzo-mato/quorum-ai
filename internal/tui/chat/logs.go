package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// LogLevel represents log severity
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelSuccess
)

// LogEntry represents a single log entry
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Source  string // e.g., "claude", "gemini", "workflow"
	Message string
}

// TokenStats holds token usage for a model
type TokenStats struct {
	Model     string
	TokensIn  int
	TokensOut int
}

// ResourceStats holds system resource usage
type ResourceStats struct {
	MemoryMB   float64
	CPUPercent float64
	Uptime     time.Duration
	Goroutines int
}

// LogsPanel manages the logs display
type LogsPanel struct {
	mu       sync.Mutex
	entries  []LogEntry
	maxLines int
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Footer stats
	tokenStats    []TokenStats
	resourceStats ResourceStats
	showFooter    bool
	footerHeight  int // Fixed height for footer section
}

// NewLogsPanel creates a new logs panel
func NewLogsPanel(maxLines int) *LogsPanel {
	if maxLines <= 0 {
		maxLines = 500
	}
	return &LogsPanel{
		entries:      make([]LogEntry, 0, maxLines),
		maxLines:     maxLines,
		tokenStats:   make([]TokenStats, 0),
		showFooter:   true,
		footerHeight: 6, // Fixed height for stats footer
	}
}

// Add adds a new log entry
func (p *LogsPanel) Add(level LogLevel, source, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Source:  source,
		Message: message,
	}

	p.entries = append(p.entries, entry)

	// Trim if exceeds max
	if len(p.entries) > p.maxLines {
		p.entries = p.entries[len(p.entries)-p.maxLines:]
	}

	// Update viewport content
	p.updateContent()
}

// AddInfo adds an info log
func (p *LogsPanel) AddInfo(source, message string) {
	p.Add(LogLevelInfo, source, message)
}

// AddWarn adds a warning log
func (p *LogsPanel) AddWarn(source, message string) {
	p.Add(LogLevelWarn, source, message)
}

// AddError adds an error log
func (p *LogsPanel) AddError(source, message string) {
	p.Add(LogLevelError, source, message)
}

// AddSuccess adds a success log
func (p *LogsPanel) AddSuccess(source, message string) {
	p.Add(LogLevelSuccess, source, message)
}

// AddDebug adds a debug log
func (p *LogsPanel) AddDebug(source, message string) {
	p.Add(LogLevelDebug, source, message)
}

// Clear clears all logs
func (p *LogsPanel) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = p.entries[:0]
	p.updateContent()
}

// SetSize updates the panel dimensions
func (p *LogsPanel) SetSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.width = width
	p.height = height

	// Calculate viewport height (subtract header, footer, and borders)
	// Header: 2 lines (title + separator)
	// Footer: footerHeight lines when visible
	// Borders: 2 lines
	viewportHeight := height - 4 // header + borders
	if p.showFooter {
		viewportHeight -= p.footerHeight + 1 // +1 for separator
	}
	if viewportHeight < 3 {
		viewportHeight = 3
	}

	if !p.ready {
		p.viewport = viewport.New(width-4, viewportHeight)
		p.ready = true
	} else {
		p.viewport.Width = width - 4
		p.viewport.Height = viewportHeight
	}
	p.updateContent()
}

// SetTokenStats updates token statistics
func (p *LogsPanel) SetTokenStats(stats []TokenStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenStats = stats
}

// SetResourceStats updates resource statistics
func (p *LogsPanel) SetResourceStats(stats ResourceStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resourceStats = stats
}

// ToggleFooter toggles footer visibility
func (p *LogsPanel) ToggleFooter() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.showFooter = !p.showFooter
}

// Update updates the viewport
func (p *LogsPanel) Update(msg interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready {
		p.viewport, _ = p.viewport.Update(msg)
	}
}

// GotoBottom scrolls to the bottom
func (p *LogsPanel) GotoBottom() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoBottom()
	}
}

// ScrollUp scrolls the viewport up by one line
func (p *LogsPanel) ScrollUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollUp(1)
	}
}

// ScrollDown scrolls the viewport down by one line
func (p *LogsPanel) ScrollDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollDown(1)
	}
}

// PageUp scrolls the viewport up by one page
func (p *LogsPanel) PageUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.PageUp()
	}
}

// PageDown scrolls the viewport down by one page
func (p *LogsPanel) PageDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.PageDown()
	}
}

// GotoTop scrolls to the top
func (p *LogsPanel) GotoTop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoTop()
	}
}

// ScrollPercent returns the current scroll percentage
func (p *LogsPanel) ScrollPercent() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		return p.viewport.ScrollPercent()
	}
	return 0
}

// AtBottom returns true if scrolled to the bottom
func (p *LogsPanel) AtBottom() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		return p.viewport.AtBottom()
	}
	return true
}

// updateContent refreshes the viewport content (must be called with lock held)
func (p *LogsPanel) updateContent() {
	if !p.ready {
		return
	}

	var sb strings.Builder
	for _, entry := range p.entries {
		sb.WriteString(p.formatEntry(entry))
		sb.WriteString("\n")
	}

	p.viewport.SetContent(sb.String())
	p.viewport.GotoBottom()
}

// formatEntry formats a single log entry with word wrapping
func (p *LogsPanel) formatEntry(entry LogEntry) string {
	// Time
	timeStr := entry.Time.Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	// Level icon and color
	var levelIcon string
	var levelStyle lipgloss.Style

	switch entry.Level {
	case LogLevelDebug:
		levelIcon = "·"
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	case LogLevelInfo:
		levelIcon = "●"
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b82f6"))
	case LogLevelWarn:
		levelIcon = "▲"
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308"))
	case LogLevelError:
		levelIcon = "✗"
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
	case LogLevelSuccess:
		levelIcon = "✓"
		levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	}

	// Source with color
	sourceStyle := lipgloss.NewStyle().Foreground(GetAgentColor(entry.Source)).Bold(true)
	if entry.Source == "" {
		entry.Source = "sys"
	}

	// Truncate source to 8 chars
	source := entry.Source
	if len(source) > 8 {
		source = source[:8]
	}

	// Message style
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e5e7eb"))

	// Calculate prefix width: "HH:MM:SS ● SOURCE   " = 8 + 1 + 1 + 1 + 8 + 1 = 20
	prefixWidth := 20
	// Available width for message (account for borders and padding)
	msgWidth := p.width - prefixWidth - 6
	if msgWidth < 20 {
		msgWidth = 20
	}

	// Wrap message if too long
	message := entry.Message
	if len(message) > msgWidth {
		// Word wrap the message
		var lines []string
		indent := strings.Repeat(" ", prefixWidth)

		for message != "" {
			if len(message) <= msgWidth {
				lines = append(lines, message)
				break
			}

			// Find a good break point (space)
			breakPoint := msgWidth
			for i := msgWidth; i > msgWidth/2; i-- {
				if i < len(message) && message[i] == ' ' {
					breakPoint = i
					break
				}
			}

			lines = append(lines, message[:breakPoint])
			message = strings.TrimLeft(message[breakPoint:], " ")
		}

		// First line with prefix, subsequent lines indented
		if len(lines) > 0 {
			result := fmt.Sprintf("%s %s %s %s",
				timeStyle.Render(timeStr),
				levelStyle.Render(levelIcon),
				sourceStyle.Render(fmt.Sprintf("%-8s", source)),
				msgStyle.Render(lines[0]),
			)
			for i := 1; i < len(lines); i++ {
				result += "\n" + indent + msgStyle.Render(lines[i])
			}
			return result
		}
	}

	return fmt.Sprintf("%s %s %s %s",
		timeStyle.Render(timeStr),
		levelStyle.Render(levelIcon),
		sourceStyle.Render(fmt.Sprintf("%-8s", source)),
		msgStyle.Render(message),
	)
}

// Nerd Font icon for logs
const logsIcon = "" // nf-fa-list_alt

// Render renders the logs panel
func (p *LogsPanel) Render() string {
	return p.RenderWithFocus(false)
}

// RenderWithFocus renders the logs panel with focus indicator
func (p *LogsPanel) RenderWithFocus(focused bool) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.ready {
		return ""
	}

	// Header with icon
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a855f7")).
		Bold(true)

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	header := headerStyle.Render(logsIcon+" Logs") + " " +
		countStyle.Render(fmt.Sprintf("(%d)", len(p.entries)))

	// Scroll indicator
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#06b6d4"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	scrollInfo := ""
	if !p.viewport.AtBottom() {
		scrollPct := int(p.viewport.ScrollPercent() * 100)
		scrollInfo = scrollStyle.Render(fmt.Sprintf(" ↕%d%%", scrollPct))
	}

	// Help text (keyboard hint)
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)
	help := helpStyle.Render("^L")
	if focused {
		help = dimStyle.Render("↑↓") + " " + help
	}

	// Header line with scroll info and help on right
	headerWidth := p.width - 4
	gap := headerWidth - lipgloss.Width(header) - lipgloss.Width(scrollInfo) - lipgloss.Width(help)
	if gap < 1 {
		gap = 1
	}
	headerLine := header + scrollInfo + strings.Repeat(" ", gap) + help

	// Content
	content := p.viewport.View()

	// Combine
	var sb strings.Builder
	sb.WriteString(headerLine)
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("─", p.width-4)))
	sb.WriteString("\n")
	sb.WriteString(content)

	// Footer with stats (if enabled)
	if p.showFooter {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render(strings.Repeat("─", p.width-4)))
		sb.WriteString("\n")
		sb.WriteString(p.renderFooter())
	}

	// Box style with rounded borders - highlight if focused
	borderColor := lipgloss.Color("#374151")
	if focused {
		borderColor = lipgloss.Color("#a855f7") // Purple when focused
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(p.width).
		Height(p.height)

	return boxStyle.Render(sb.String())
}

// Count returns the number of log entries
func (p *LogsPanel) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// renderFooter renders the stats footer (must be called with lock held)
func (p *LogsPanel) renderFooter() string {
	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")). // Green
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f0fdf4")).
		Bold(true)

	inStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22d3ee")) // Cyan for input

	outStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a78bfa")) // Purple for output

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6b7280"))

	resourceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#38bdf8")) // Sky blue

	// Calculate column widths
	colWidth := (p.width - 6) / 2
	if colWidth < 15 {
		colWidth = 15
	}

	var lines []string

	// === LEFT COLUMN: Tokens ===
	// === RIGHT COLUMN: Resources ===

	// Line 1: Headers
	leftHeader := headerStyle.Render("Tokens") + " " + dimStyle.Render("(in→out)")
	rightHeader := resourceStyle.Render("Resources")
	line1 := p.formatTwoColumns(leftHeader, rightHeader, colWidth)
	lines = append(lines, line1)

	// Line 2-4: Token stats per model (left) and resources (right)
	totalIn := 0
	totalOut := 0

	// Prepare token lines
	var tokenLines []string
	for _, ts := range p.tokenStats {
		totalIn += ts.TokensIn
		totalOut += ts.TokensOut
		tokenStr := fmt.Sprintf("%s %s→%s",
			labelStyle.Render(fmt.Sprintf("%-8s", ts.Model)),
			inStyle.Render(formatTokenCount(ts.TokensIn)),
			outStyle.Render(formatTokenCount(ts.TokensOut)),
		)
		tokenLines = append(tokenLines, tokenStr)
	}

	// Prepare resource lines
	var resourceLines []string

	// RAM
	ramLine := fmt.Sprintf("%s %s",
		labelStyle.Render("RAM:"),
		valueStyle.Render(fmt.Sprintf("%.1f MB", p.resourceStats.MemoryMB)),
	)
	resourceLines = append(resourceLines, ramLine)

	// CPU
	cpuLine := fmt.Sprintf("%s %s",
		labelStyle.Render("CPU:"),
		valueStyle.Render(fmt.Sprintf("%.1f%%", p.resourceStats.CPUPercent)),
	)
	resourceLines = append(resourceLines, cpuLine)

	// Uptime
	uptimeLine := fmt.Sprintf("%s %s",
		labelStyle.Render("Up:"),
		dimStyle.Render(formatDuration(p.resourceStats.Uptime)),
	)
	resourceLines = append(resourceLines, uptimeLine)

	// Combine token and resource lines
	maxLines := len(tokenLines)
	if len(resourceLines) > maxLines {
		maxLines = len(resourceLines)
	}

	for i := 0; i < maxLines; i++ {
		left := ""
		right := ""
		if i < len(tokenLines) {
			left = tokenLines[i]
		}
		if i < len(resourceLines) {
			right = resourceLines[i]
		}
		lines = append(lines, p.formatTwoColumns(left, right, colWidth))
	}

	// Total line
	totalLine := fmt.Sprintf("%s %s→%s",
		labelStyle.Render("Total:"),
		inStyle.Render(formatTokenCount(totalIn)),
		outStyle.Render(formatTokenCount(totalOut)),
	)
	goroutinesLine := fmt.Sprintf("%s %s",
		dimStyle.Render("∴"),
		dimStyle.Render(fmt.Sprintf("%d go", p.resourceStats.Goroutines)),
	)
	lines = append(lines, p.formatTwoColumns(totalLine, goroutinesLine, colWidth))

	return strings.Join(lines, "\n")
}

// formatTwoColumns formats two strings into columns
func (p *LogsPanel) formatTwoColumns(left, right string, colWidth int) string {
	leftWidth := lipgloss.Width(left)

	// Pad left column
	padding := colWidth - leftWidth
	if padding < 1 {
		padding = 1
	}

	// Separator
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render("│")

	return left + strings.Repeat(" ", padding) + sep + " " + right
}

// formatTokenCount formats token count with K suffix
func formatTokenCount(tokens int) string {
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatDuration formats duration as human-readable
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
