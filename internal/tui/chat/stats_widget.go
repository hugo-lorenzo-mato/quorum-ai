package chat

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
)

// Icons for stats widget (unique, not used elsewhere)
const (
	iconClock  = "◷" // Clock face
	iconMemory = "⬢" // Hexagon (memory chip)
	iconCPU    = "⚙" // Gear (processor)
)

// ProcessStats holds current process statistics
type ProcessStats struct {
	MemoryMB      float64
	MemoryAlloc   uint64
	CPUPercent    float64 // Normalized (0-100%, relative to total system capacity)
	CPURawPercent float64 // Raw (can exceed 100% on multi-core, like top/htop)
	Goroutines    int
	Uptime        time.Duration
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
	lastCPUTime  float64
	lastWallTime time.Time

	proc     *process.Process
	cpuCount int
}

// NewStatsWidget creates a new stats widget
func NewStatsWidget() *StatsWidget {
	pid := os.Getpid()
	// #nosec G115 - PID is guaranteed to fit in int32 on all supported platforms
	proc, _ := process.NewProcess(int32(pid))
	cpuCount, err := cpu.Counts(true)
	if err != nil || cpuCount <= 0 {
		cpuCount = runtime.NumCPU()
	}
	w := &StatsWidget{
		startTime:    time.Now(),
		visible:      false, // Hidden by default, toggle with Ctrl+5
		lastWallTime: time.Now(),
		proc:         proc,
		cpuCount:     cpuCount,
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
	if w.proc != nil {
		if memInfo, err := w.proc.MemoryInfo(); err == nil {
			w.stats.MemoryMB = float64(memInfo.RSS) / 1024 / 1024
		} else {
			w.stats.MemoryMB = float64(memStats.Alloc) / 1024 / 1024
		}
	} else {
		w.stats.MemoryMB = float64(memStats.Alloc) / 1024 / 1024
	}

	// Goroutines
	w.stats.Goroutines = runtime.NumGoroutine()

	// Uptime
	w.stats.Uptime = time.Since(w.startTime)

	// CPU percentage (process-level, cross-platform)
	rawCPU := w.calculateCPURaw()
	w.stats.CPURawPercent = rawCPU
	denom := float64(w.cpuCount)
	if denom <= 0 {
		denom = float64(runtime.NumCPU())
	}
	if denom <= 0 {
		denom = 1
	}
	w.stats.CPUPercent = rawCPU / denom // Normalized to system capacity
}

// calculateCPURaw calculates raw CPU usage for the current process (can exceed 100% on multi-core)
func (w *StatsWidget) calculateCPURaw() float64 {
	if w.proc == nil {
		return 0
	}

	// Process CPU time in seconds
	times, err := w.proc.Times()
	if err != nil {
		return 0
	}

	currentCPUTime := times.User + times.System
	currentWallTime := time.Now()

	// Calculate CPU percentage
	if w.lastCPUTime > 0 {
		cpuDelta := currentCPUTime - w.lastCPUTime
		wallDelta := currentWallTime.Sub(w.lastWallTime).Seconds()

		if wallDelta > 0 {
			cpuPercent := (cpuDelta / wallDelta) * 100

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

	// CPU line (normalized % / raw %)
	cpuIcon := iconStyle.Render(iconCPU)
	cpuLabel := labelStyle.Render(" CPU ")
	// Show both: normalized (for bar) and raw (actual cores usage)
	cpuValue := valueStyle.Render(fmt.Sprintf("%4.1f%%", w.stats.CPUPercent))
	cpuRaw := labelStyle.Render(fmt.Sprintf("/%3.0f%%", w.stats.CPURawPercent))
	cpuHint := labelStyle.Render(" t/c")
	sb.WriteString(cpuIcon + cpuLabel + cpuBar + " " + cpuValue + cpuRaw + cpuHint)
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

	// Box style - no fixed background to work with any terminal color
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#38bdf8")). // Sky blue border
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
