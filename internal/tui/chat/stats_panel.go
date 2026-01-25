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
	resourceStats ResourceStats
	machineStats  MachineStats
}

// NewStatsPanel creates a new stats panel
func NewStatsPanel() *StatsPanel {
	return &StatsPanel{}
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

// PageUp scrolls up by half a page
func (p *StatsPanel) PageUp() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.HalfViewUp()
	}
}

// PageDown scrolls down by half a page
func (p *StatsPanel) PageDown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.HalfViewDown()
	}
}

// GotoTop scrolls to the top
func (p *StatsPanel) GotoTop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoTop()
	}
}

// GotoBottom scrolls to the bottom
func (p *StatsPanel) GotoBottom() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ready {
		p.viewport.GotoBottom()
	}
}

// Width returns panel width
func (p *StatsPanel) Width() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width
}

// Height returns panel height
func (p *StatsPanel) Height() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.height
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

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6b7280"))

	resourceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#38bdf8")) // Sky blue

	machineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f97316")) // Orange

	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	barWidth := 12

	var sb strings.Builder

	// === SECTION 1: Quorum Process ===
	sb.WriteString(resourceStyle.Render(quorumIcon + " Quorum Process"))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  CPU proc: total/core"))
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

	// CPU with bar (show normalized / raw)
	cpuPercent := p.resourceStats.CPUPercent
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	cpuBar := p.renderBar(cpuPercent, barWidth, "#f97316")
	// Show both: normalized (0-100%) / raw (can exceed 100% on multi-core)
	cpuValue := valueStyle.Render(fmt.Sprintf("%.1f%%", p.resourceStats.CPUPercent))
	cpuRaw := dimStyle.Render(fmt.Sprintf("/%.0f%%", p.resourceStats.CPURawPercent))
	sb.WriteString(fmt.Sprintf("  %s %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "CPU:")),
		cpuBar,
		cpuValue+cpuRaw,
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

	// Hardware info
	sb.WriteString("\n")
	sb.WriteString(sectionStyle.Render("Hardware"))
	sb.WriteString("\n\n")

	cpuInfo := p.machineStats.CPUModel
	if cpuInfo == "" {
		cpuInfo = "n/a"
	}
	if p.machineStats.CPUCores > 0 || p.machineStats.CPUThreads > 0 {
		cpuInfo = fmt.Sprintf("%s (%dC/%dT)", cpuInfo, p.machineStats.CPUCores, p.machineStats.CPUThreads)
	}
	cpuInfo = truncateToWidth(cpuInfo, p.width-10)
	sb.WriteString(fmt.Sprintf("  %s %s",
		labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "CPU:")),
		dimStyle.Render(cpuInfo),
	))
	sb.WriteString("\n")

	if p.machineStats.MemTotalMB > 0 {
		sb.WriteString(fmt.Sprintf("  %s %s",
			labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "RAM:")),
			dimStyle.Render(fmt.Sprintf("%.1f GB", p.machineStats.MemTotalMB/1024)),
		))
	} else {
		sb.WriteString(fmt.Sprintf("  %s %s",
			labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "RAM:")),
			dimStyle.Render("n/a"),
		))
	}
	sb.WriteString("\n")

	if len(p.machineStats.GPUInfos) == 0 {
		sb.WriteString(fmt.Sprintf("  %s %s",
			labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, "GPU:")),
			dimStyle.Render("n/a"),
		))
		sb.WriteString("\n")
	} else {
		for i, gpu := range p.machineStats.GPUInfos {
			label := "GPU:"
			if i > 0 {
				label = fmt.Sprintf("GPU%d:", i+1)
			}
			info := gpu.Name
			if gpu.UtilValid {
				info += fmt.Sprintf(" %.0f%%", gpu.UtilPercent)
			}
			if gpu.MemValid {
				info += fmt.Sprintf(" %.1f/%.1f GB", gpu.MemUsedMB/1024, gpu.MemTotalMB/1024)
			}
			if gpu.TempValid {
				info += fmt.Sprintf(" %.0f°C", gpu.TempC)
			}
			info = truncateToWidth(info, p.width-10)
			sb.WriteString(fmt.Sprintf("  %s %s",
				labelStyle.Render(fmt.Sprintf("%-*s", labelWidth, label)),
				dimStyle.Render(info),
			))
			sb.WriteString("\n")
		}
	}

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
	help := helpStyle.Render("^R")

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
