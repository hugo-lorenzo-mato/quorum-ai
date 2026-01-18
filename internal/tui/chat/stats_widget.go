package chat

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Icons for stats widget (unique, not used elsewhere)
const (
	iconClock  = "◷" // Clock face
	iconMemory = "⬢" // Hexagon (memory chip)
	iconCPU    = "⚙" // Gear (processor)
)

// ProcessStats holds current process statistics
type ProcessStats struct {
	MemoryMB    float64
	MemoryAlloc uint64
	CPUPercent  float64
	Goroutines  int
	Uptime      time.Duration
}

// StatsWidget displays process statistics
type StatsWidget struct {
	mu        sync.Mutex
	stats     ProcessStats
	startTime time.Time
	visible   bool
	width     int
	height    int

	// CPU calculation
	lastCPUTime  uint64
	lastWallTime time.Time
}

// NewStatsWidget creates a new stats widget
func NewStatsWidget() *StatsWidget {
	w := &StatsWidget{
		startTime:    time.Now(),
		visible:      false, // Hidden by default, toggle with Ctrl+5
		lastWallTime: time.Now(),
	}
	w.Update() // Initial stats
	return w
}

// Update refreshes the statistics
func (w *StatsWidget) Update() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	w.stats.MemoryAlloc = memStats.Alloc
	w.stats.MemoryMB = float64(memStats.Alloc) / 1024 / 1024

	// Goroutines
	w.stats.Goroutines = runtime.NumGoroutine()

	// Uptime
	w.stats.Uptime = time.Since(w.startTime)

	// CPU percentage (Linux-specific, falls back to 0 on other OS)
	w.stats.CPUPercent = w.calculateCPUPercent()
}

// calculateCPUPercent calculates CPU usage for the current process
func (w *StatsWidget) calculateCPUPercent() float64 {
	// Read /proc/self/stat for process CPU time
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}

	// Parse utime and stime (fields 14 and 15, 1-indexed)
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return 0
	}

	var utime, stime uint64
	fmt.Sscanf(fields[13], "%d", &utime)
	fmt.Sscanf(fields[14], "%d", &stime)

	currentCPUTime := utime + stime
	currentWallTime := time.Now()

	// Calculate CPU percentage
	if w.lastCPUTime > 0 {
		cpuDelta := float64(currentCPUTime - w.lastCPUTime)
		wallDelta := currentWallTime.Sub(w.lastWallTime).Seconds()

		if wallDelta > 0 {
			// CPU time is in clock ticks (usually 100 per second)
			clkTck := float64(100) // sysconf(_SC_CLK_TCK) is typically 100
			cpuPercent := (cpuDelta / clkTck / wallDelta) * 100

			w.lastCPUTime = currentCPUTime
			w.lastWallTime = currentWallTime

			return cpuPercent
		}
	}

	w.lastCPUTime = currentCPUTime
	w.lastWallTime = currentWallTime
	return 0
}

// Toggle toggles visibility
func (w *StatsWidget) Toggle() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.visible = !w.visible
}

// IsVisible returns visibility
func (w *StatsWidget) IsVisible() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.visible
}

// SetSize sets widget dimensions
func (w *StatsWidget) SetSize(width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.width = width
	w.height = height
}

// GetStats returns current stats
func (w *StatsWidget) GetStats() ProcessStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

// Render renders the stats widget
func (w *StatsWidget) Render() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.visible {
		return ""
	}

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#38bdf8")). // Sky blue
		Bold(true)

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a78bfa")) // Purple

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ca3af")) // Gray

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f0fdf4")). // Light
		Bold(true)

	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fbbf24")). // Amber
		Bold(true)

	// Current time
	now := time.Now()
	timeStr := now.Format("15:04:05")

	// Memory bar (0-100MB scale, adjust as needed)
	memPercent := w.stats.MemoryMB / 100 * 100
	if memPercent > 100 {
		memPercent = 100
	}
	memBar := w.renderBar(memPercent, 8, lipgloss.Color("#22c55e")) // Green

	// CPU bar
	cpuPercent := w.stats.CPUPercent
	if cpuPercent > 100 {
		cpuPercent = 100
	}
	cpuBar := w.renderBar(cpuPercent, 8, lipgloss.Color("#f97316")) // Orange

	var sb strings.Builder

	// Header with time
	header := headerStyle.Render(iconClock + " " + timeStr)
	sb.WriteString(header)
	sb.WriteString("\n")

	// Separator
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")).Render("────────────"))
	sb.WriteString("\n")

	// Memory line
	memIcon := iconStyle.Render(iconMemory)
	memLabel := labelStyle.Render(" RAM ")
	memValue := valueStyle.Render(fmt.Sprintf("%5.1fMB", w.stats.MemoryMB))
	sb.WriteString(memIcon + memLabel + memBar + " " + memValue)
	sb.WriteString("\n")

	// CPU line
	cpuIcon := iconStyle.Render(iconCPU)
	cpuLabel := labelStyle.Render(" CPU ")
	cpuValue := valueStyle.Render(fmt.Sprintf("%5.1f%%", w.stats.CPUPercent))
	sb.WriteString(cpuIcon + cpuLabel + cpuBar + " " + cpuValue)
	sb.WriteString("\n")

	// Goroutines (small detail)
	goLabel := labelStyle.Render("  ∴ ")
	goValue := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(fmt.Sprintf("%d goroutines", w.stats.Goroutines))
	sb.WriteString(goLabel + goValue)
	sb.WriteString("\n")

	// Uptime
	uptimeLabel := labelStyle.Render("  ↑ ")
	uptimeValue := timeStyle.Render(w.formatUptime(w.stats.Uptime))
	sb.WriteString(uptimeLabel + uptimeValue)

	// Box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#38bdf8")). // Sky blue border
		BorderBackground(lipgloss.Color("#1f1f23")).
		Background(lipgloss.Color("#1f1f23")).
		Padding(0, 1)

	return boxStyle.Render(sb.String())
}

// renderBar renders a progress bar
func (w *StatsWidget) renderBar(percent float64, width int, color lipgloss.Color) string {
	filled := int(percent * float64(width) / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	filledStyle := lipgloss.NewStyle().Foreground(color)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))

	return filledStyle.Render(strings.Repeat("▓", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}

// formatUptime formats duration as human-readable
func (w *StatsWidget) formatUptime(d time.Duration) string {
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
