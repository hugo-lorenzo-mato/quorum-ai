package chat

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NewStatsWidget
// ---------------------------------------------------------------------------

func TestNewStatsWidget(t *testing.T) {
	w := NewStatsWidget()
	if w == nil {
		t.Fatal("NewStatsWidget returned nil")
	}
	if w.visible {
		t.Error("Widget should start hidden")
	}
	if w.startTime.IsZero() {
		t.Error("startTime should be set")
	}
}

// ---------------------------------------------------------------------------
// Toggle / IsVisible
// ---------------------------------------------------------------------------

func TestStatsWidget_Toggle(t *testing.T) {
	w := NewStatsWidget()
	if w.IsVisible() {
		t.Error("Should start hidden")
	}
	w.Toggle()
	if !w.IsVisible() {
		t.Error("Should be visible after toggle")
	}
	w.Toggle()
	if w.IsVisible() {
		t.Error("Should be hidden after second toggle")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestStatsWidget_SetSize(t *testing.T) {
	w := NewStatsWidget()
	w.SetSize(40, 15)
	w.mu.Lock()
	width := w.width
	height := w.height
	w.mu.Unlock()
	if width != 40 || height != 15 {
		t.Errorf("Size should be 40x15, got %dx%d", width, height)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestStatsWidget_Update(t *testing.T) {
	w := NewStatsWidget()
	// Add small delay to ensure measurable uptime on fast systems
	time.Sleep(10 * time.Millisecond)
	// Update should populate stats
	w.Update()

	stats := w.GetStats()
	if stats.Goroutines <= 0 {
		t.Error("Goroutines should be > 0")
	}
	// Uptime should be non-negative (may be 0 on very fast systems)
	if stats.Uptime < 0 {
		t.Error("Uptime should not be negative")
	}
}

func TestStatsWidget_Update_Multiple(t *testing.T) {
	w := NewStatsWidget()
	// Multiple updates should not panic and should refine CPU stats
	w.Update()
	time.Sleep(10 * time.Millisecond) // small delay for CPU measurement
	w.Update()

	stats := w.GetStats()
	if stats.Goroutines <= 0 {
		t.Error("Goroutines should be > 0 after multiple updates")
	}
}

// ---------------------------------------------------------------------------
// GetStats
// ---------------------------------------------------------------------------

func TestStatsWidget_GetStats(t *testing.T) {
	w := NewStatsWidget()
	w.Update()

	stats := w.GetStats()

	if stats.MemoryMB < 0 {
		t.Error("MemoryMB should not be negative")
	}
	if stats.Goroutines <= 0 {
		t.Error("Goroutines should be > 0")
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestStatsWidget_Render_Hidden(t *testing.T) {
	w := NewStatsWidget()
	result := w.Render()
	if result != "" {
		t.Error("Hidden widget should render empty string")
	}
}

func TestStatsWidget_Render_Visible(t *testing.T) {
	w := NewStatsWidget()
	w.Toggle() // make visible
	w.Update()

	result := w.Render()
	if result == "" {
		t.Error("Visible widget should render non-empty string")
	}
}

func TestStatsWidget_Render_ContainsStats(t *testing.T) {
	w := NewStatsWidget()
	w.Toggle()
	w.Update()

	result := w.Render()

	if !strings.Contains(result, "RAM") {
		t.Error("Should contain 'RAM' label")
	}
	if !strings.Contains(result, "CPU") {
		t.Error("Should contain 'CPU' label")
	}
	if !strings.Contains(result, "goroutines") {
		t.Error("Should contain goroutines info")
	}
	if !strings.Contains(result, "MB") {
		t.Error("Should contain memory in MB")
	}
}

func TestStatsWidget_Render_ContainsTime(t *testing.T) {
	w := NewStatsWidget()
	w.Toggle()
	w.Update()

	result := w.Render()
	// Should contain current time in HH:MM:SS format
	if !strings.Contains(result, ":") {
		t.Error("Should contain time separator ':'")
	}
}

// ---------------------------------------------------------------------------
// renderBar
// ---------------------------------------------------------------------------

func TestStatsWidget_RenderBar_Zero(t *testing.T) {
	w := NewStatsWidget()
	bar := w.renderBar(0, 10, "#22c55e")
	if bar == "" {
		t.Error("Bar at 0% should still render (all empty)")
	}
}

func TestStatsWidget_RenderBar_Full(t *testing.T) {
	w := NewStatsWidget()
	bar := w.renderBar(100, 10, "#22c55e")
	if bar == "" {
		t.Error("Bar at 100% should render")
	}
}

func TestStatsWidget_RenderBar_OverFull(t *testing.T) {
	w := NewStatsWidget()
	bar := w.renderBar(150, 10, "#22c55e")
	if bar == "" {
		t.Error("Bar at 150% should render (capped)")
	}
}

func TestStatsWidget_RenderBar_Negative(t *testing.T) {
	w := NewStatsWidget()
	bar := w.renderBar(-10, 10, "#22c55e")
	if bar == "" {
		t.Error("Bar at -10% should render (capped to 0)")
	}
}

func TestStatsWidget_RenderBar_Partial(t *testing.T) {
	w := NewStatsWidget()
	bar := w.renderBar(50, 10, "#22c55e")
	if bar == "" {
		t.Error("Bar at 50% should render")
	}
}

// ---------------------------------------------------------------------------
// formatUptime
// ---------------------------------------------------------------------------

func TestStatsWidget_FormatUptime_Seconds(t *testing.T) {
	w := NewStatsWidget()
	result := w.formatUptime(30 * time.Second)
	if result != "30s" {
		t.Errorf("Expected '30s', got %q", result)
	}
}

func TestStatsWidget_FormatUptime_Minutes(t *testing.T) {
	w := NewStatsWidget()
	result := w.formatUptime(90 * time.Second)
	if result != "1m30s" {
		t.Errorf("Expected '1m30s', got %q", result)
	}
}

func TestStatsWidget_FormatUptime_Hours(t *testing.T) {
	w := NewStatsWidget()
	result := w.formatUptime(3661 * time.Second)
	if result != "1h01m" {
		t.Errorf("Expected '1h01m', got %q", result)
	}
}

func TestStatsWidget_FormatUptime_Zero(t *testing.T) {
	w := NewStatsWidget()
	result := w.formatUptime(0)
	if result != "0s" {
		t.Errorf("Expected '0s', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// calculateCPURaw
// ---------------------------------------------------------------------------

func TestStatsWidget_CalculateCPURaw_NoProc(t *testing.T) {
	w := &StatsWidget{
		proc:         nil,
		lastWallTime: time.Now(),
	}
	result := w.calculateCPURaw()
	if result != 0 {
		t.Errorf("No proc should return 0, got %f", result)
	}
}

func TestStatsWidget_CalculateCPURaw_WithProc(t *testing.T) {
	w := NewStatsWidget()
	// First call sets baseline
	_ = w.calculateCPURaw()
	time.Sleep(10 * time.Millisecond)
	// Second call should calculate delta
	result := w.calculateCPURaw()
	// Result may be 0 if very little CPU was used, that's ok
	if result < 0 {
		t.Errorf("CPU raw should not be negative, got %f", result)
	}
}

// ---------------------------------------------------------------------------
// Memory high CPU coverage
// ---------------------------------------------------------------------------

func TestStatsWidget_Render_HighCPU(t *testing.T) {
	w := NewStatsWidget()
	w.Toggle()
	w.mu.Lock()
	w.stats.CPUPercent = 150 // over 100, should cap in bar
	w.stats.CPURawPercent = 450
	w.stats.MemoryMB = 200
	w.stats.Goroutines = 100
	w.stats.Uptime = 2 * time.Hour
	w.mu.Unlock()

	result := w.Render()
	if result == "" {
		t.Error("Should render even with high CPU")
	}
}

func TestStatsWidget_Render_HighMemory(t *testing.T) {
	w := NewStatsWidget()
	w.Toggle()
	w.mu.Lock()
	w.stats.MemoryMB = 250 // above 100MB scale, memPercent will cap at 100
	w.mu.Unlock()

	result := w.Render()
	if result == "" {
		t.Error("Should render even with high memory")
	}
}
