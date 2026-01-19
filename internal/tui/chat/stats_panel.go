package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// StatsPanel displays token usage and system metrics
type StatsPanel struct {
	mu       sync.Mutex
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Stats data
	tokenStats    []TokenStats
	resourceStats ResourceStats
	machineStats  MachineStats
}

// NewStatsPanel creates a new stats panel
func NewStatsPanel() *StatsPanel {
	return &StatsPanel{
		tokenStats: make([]TokenStats, 0),
	}
}

// SetSize updates the panel dimensions
func (p *StatsPanel) SetSize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.width = width
	p.height = height

	// Viewport height: total - header(2) - borders(2)
	viewportHeight := height - 4
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
func (p *StatsPanel) SetTokenStats(stats []TokenStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenStats = stats
	p.updateContent()
}

// SetResourceStats updates Quorum process statistics
func (p *StatsPanel) SetResourceStats(stats ResourceStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resourceStats = stats
	p.updateContent()
}

// SetMachineStats updates machine-wide statistics
func (p *StatsPanel) SetMachineStats(stats MachineStats) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.machineStats = stats
	p.updateContent()
}

// Update handles viewport updates
func (p *StatsPanel) Update(msg interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ready {
		p.viewport, _ = p.viewport.Update(msg)
	}
}

// ScrollUp scrolls the viewport up
func (p *StatsPanel) ScrollUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollUp(1)
	}
}

// ScrollDown scrolls the viewport down
func (p *StatsPanel) ScrollDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.ScrollDown(1)
	}
}

// Width returns panel width
func (p *StatsPanel) Width() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width
}

// updateContent refreshes the viewport content
func (p *StatsPanel) updateContent() {
	if !p.ready {
		return
	}

	content := p.renderContent()
	p.viewport.SetContent(content)
}

// Nerd Font icons
const (
	statsIcon   = "" // nf-md-chart_box
	tokenIcon   = "" // nf-md-currency_sign
	quorumIcon  = "" // nf-md-application
	machineIcon = "" // nf-md-server
)

// renderContent builds the stats content
func (p *StatsPanel) renderContent() string {
	// Styles
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")).
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

	machineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f97316")) // Orange

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	barWidth := 12

	var sb strings.Builder

	// === SECTION 1: Tokens ===
	sb.WriteString(sectionStyle.Render(tokenIcon+" Tokens") + " " + dimStyle.Render("(↑out ↓in)"))
	sb.WriteString("\n")

	// Calculate totals
	totalIn := 0
	totalOut := 0
	for _, ts := range p.tokenStats {
		totalIn += ts.TokensIn
		totalOut += ts.TokensOut
	}

	// Show all models
	if len(p.tokenStats) == 0 {
		sb.WriteString(dimStyle.Render("  No token data yet"))
		sb.WriteString("\n")
	} else {
		for _, ts := range p.tokenStats {
			modelName := ts.Model
			if len(modelName) > 10 {
				modelName = modelName[:9] + "…"
			}
			line := fmt.Sprintf("  %s  ↑%s  ↓%s",
				labelStyle.Render(fmt.Sprintf("%-10s", modelName)),
				inStyle.Render(fmt.Sprintf("%6s", formatTokenCount(ts.TokensIn))),
				outStyle.Render(fmt.Sprintf("%6s", formatTokenCount(ts.TokensOut))),
			)
			sb.WriteString(line)
			sb.WriteString("\n")
		}

		// Separator line
		sb.WriteString(dimStyle.Render("  " + strings.Repeat("─", p.width-8)))
		sb.WriteString("\n")

		// Total line
		totalLine := fmt.Sprintf("  %s  ↑%s  ↓%s",
			labelStyle.Render(fmt.Sprintf("%-10s", "Total")),
			inStyle.Render(fmt.Sprintf("%6s", formatTokenCount(totalIn))),
			outStyle.Render(fmt.Sprintf("%6s", formatTokenCount(totalOut))),
		)
		sb.WriteString(totalLine)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// === SECTION 2: Quorum Process ===
	sb.WriteString(resourceStyle.Render(quorumIcon + " Quorum Process"))
	sb.WriteString("\n\n")

	// Label width for alignment (longest is "Goroutines:" but we use shorter labels)
	const labelWidth = 5 // "RAM: ", "CPU: ", "Up:  ", "Go:  "

	// RAM with bar
	ramPercent := p.resourceStats.MemoryMB / 100 * 100 // Scale to 100MB
	if ramPercent > 100 {
		ramPercent = 100
	}
	ramBar := p.renderBar(ramPercent, barWidth, "#22c55e")
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "RAM:")),
		ramBar,
		valueStyle.Render(fmt.Sprintf("%.1f MB", p.resourceStats.MemoryMB)),
	))
	sb.WriteString("\n")

	// CPU with bar
	cpuPercent := p.resourceStats.CPUPercent
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	cpuBar := p.renderBar(cpuPercent, barWidth, "#f97316")
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "CPU:")),
		cpuBar,
		valueStyle.Render(fmt.Sprintf("%.1f%%", p.resourceStats.CPUPercent)),
	))
	sb.WriteString("\n")

	// Goroutines (no bar, just value)
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "Go:")),
		strings.Repeat(" ", barWidth), // Empty space to align with bars
		dimStyle.Render(fmt.Sprintf("%d routines", p.resourceStats.Goroutines)),
	))
	sb.WriteString("\n")

	// Uptime (no bar, just value)
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "Up:")),
		strings.Repeat(" ", barWidth), // Empty space to align
		dimStyle.Render(formatDuration(p.resourceStats.Uptime)),
	))
	sb.WriteString("\n\n")

	// === SECTION 3: Machine ===
	sb.WriteString(machineStyle.Render(machineIcon + " Machine"))
	sb.WriteString("\n\n")

	// Machine RAM with bar
	machineRamBar := p.renderBar(p.machineStats.MemPercent, barWidth, "#3b82f6")
	sb.WriteString(fmt.Sprintf("  %s %s %.1f/%.1f GB",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "RAM:")),
		machineRamBar,
		p.machineStats.MemUsedMB/1024,
		p.machineStats.MemTotalMB/1024,
	))
	sb.WriteString("\n")

	// Machine CPU with bar
	machineCpuBar := p.renderBar(p.machineStats.CPUPercent, barWidth, "#ef4444")
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "CPU:")),
		machineCpuBar,
		valueStyle.Render(fmt.Sprintf("%.1f%%", p.machineStats.CPUPercent)),
	))
	sb.WriteString("\n")

	// Machine Disk with bar
	diskBar := p.renderBar(p.machineStats.DiskPercent, barWidth, "#a855f7")
	sb.WriteString(fmt.Sprintf("  %s %s %.0f/%.0f GB",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "Disk:")),
		diskBar,
		p.machineStats.DiskUsedGB,
		p.machineStats.DiskTotalGB,
	))
	sb.WriteString("\n")

	// Load average (no bar)
	sb.WriteString(fmt.Sprintf("  %s %s %.2f %.2f %.2f",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "Load:")),
		strings.Repeat(" ", barWidth), // Empty space to align
		p.machineStats.LoadAvg1,
		p.machineStats.LoadAvg5,
		p.machineStats.LoadAvg15,
	))
	sb.WriteString("\n")

	// Separator
	sb.WriteString("\n")
	sb.WriteString(sepStyle.Render(strings.Repeat("─", p.width-6)))
	sb.WriteString("\n")

	// Timestamp
	now := time.Now().Format("15:04:05")
	sb.WriteString(dimStyle.Render(fmt.Sprintf("  Updated: %s", now)))

	return sb.String()
}

// renderBar renders a progress bar
func (p *StatsPanel) renderBar(percent float64, width int, color string) string {
	filled := int(percent * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	return filledStyle.Render(strings.Repeat("▓", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// Render renders the stats panel
func (p *StatsPanel) Render() string {
	return p.RenderWithFocus(false)
}

// RenderWithFocus renders the stats panel with focus indicator
func (p *StatsPanel) RenderWithFocus(focused bool) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.ready {
		return ""
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")).
		Bold(true)

	header := headerStyle.Render(statsIcon + " Stats")

	// Scroll indicator
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#06b6d4"))
	scrollInfo := ""
	if !p.viewport.AtBottom() {
		scrollPct := int(p.viewport.ScrollPercent() * 100)
		scrollInfo = scrollStyle.Render(fmt.Sprintf(" ↕%d%%", scrollPct))
	}

	// Help hint
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)
	help := helpStyle.Render("^S")

	// Header line
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

	// Box style
	borderColor := lipgloss.Color("#374151")
	if focused {
		borderColor = lipgloss.Color("#10b981") // Green when focused
	}

	// lipgloss Width/Height set CONTENT size, borders are added OUTSIDE.
	// Formula: Width(X-2) + borders(2) = total X
	// DO NOT use MaxWidth/MaxHeight - they truncate AFTER borders, cutting them off.
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(p.width - 2).
		Height(p.height - 2)

	return boxStyle.Render(sb.String())
}
