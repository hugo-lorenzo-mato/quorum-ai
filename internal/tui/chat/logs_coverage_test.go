package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
)

// ---------------------------------------------------------------------------
// NewLogsPanel
// ---------------------------------------------------------------------------

func TestNewLogsPanel_Default(t *testing.T) {
	p := NewLogsPanel(0)
	if p == nil {
		t.Fatal("NewLogsPanel(0) returned nil")
	}
	if p.maxLines != 500 {
		t.Errorf("maxLines should default to 500, got %d", p.maxLines)
	}
}

func TestNewLogsPanel_Custom(t *testing.T) {
	p := NewLogsPanel(100)
	if p.maxLines != 100 {
		t.Errorf("maxLines should be 100, got %d", p.maxLines)
	}
}

func TestNewLogsPanel_Negative(t *testing.T) {
	p := NewLogsPanel(-5)
	if p.maxLines != 500 {
		t.Errorf("Negative maxLines should default to 500, got %d", p.maxLines)
	}
}

// ---------------------------------------------------------------------------
// Add / AddInfo / AddWarn / AddError / AddSuccess / AddDebug
// ---------------------------------------------------------------------------

func TestLogsPanel_Add(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	p.Add(LogLevelInfo, "claude", "hello")
	if p.Count() != 1 {
		t.Errorf("Expected 1 entry, got %d", p.Count())
	}
}

func TestLogsPanel_AddConvenience(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	p.AddInfo("claude", "info msg")
	p.AddWarn("gemini", "warn msg")
	p.AddError("codex", "error msg")
	p.AddSuccess("copilot", "success msg")
	p.AddDebug("sys", "debug msg")

	if p.Count() != 5 {
		t.Errorf("Expected 5 entries, got %d", p.Count())
	}
}

func TestLogsPanel_Add_TrimsExcess(t *testing.T) {
	p := NewLogsPanel(3)
	p.SetSize(80, 30)

	p.Add(LogLevelInfo, "a", "msg1")
	p.Add(LogLevelInfo, "a", "msg2")
	p.Add(LogLevelInfo, "a", "msg3")
	p.Add(LogLevelInfo, "a", "msg4")

	if p.Count() != 3 {
		t.Errorf("Should trim to maxLines=3, got %d", p.Count())
	}
}

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

func TestLogsPanel_Clear(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	p.AddInfo("a", "msg")
	p.AddInfo("a", "msg2")
	p.Clear()

	if p.Count() != 0 {
		t.Errorf("After Clear, count should be 0, got %d", p.Count())
	}
}

// ---------------------------------------------------------------------------
// SetSize / Width / Height
// ---------------------------------------------------------------------------

func TestLogsPanel_SetSize(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	if p.Width() != 80 {
		t.Errorf("Width should be 80, got %d", p.Width())
	}
	if p.Height() != 30 {
		t.Errorf("Height should be 30, got %d", p.Height())
	}
}

func TestLogsPanel_SetSize_Small(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(40, 5) // very small
	if p.Width() != 40 {
		t.Errorf("Width should be 40, got %d", p.Width())
	}
}

func TestLogsPanel_SetSize_Update(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.SetSize(100, 50)
	if p.Width() != 100 {
		t.Errorf("Width should be 100 after resize, got %d", p.Width())
	}
}

func TestLogsPanel_SetSize_WithFooter(t *testing.T) {
	p := NewLogsPanel(10)
	p.ToggleFooter() // enable footer
	p.SetSize(80, 30)
	if p.Width() != 80 {
		t.Errorf("Width should be 80, got %d", p.Width())
	}
}

// ---------------------------------------------------------------------------
// SetTokenStats / SetResourceStats / SetMachineStats
// ---------------------------------------------------------------------------

func TestLogsPanel_SetTokenStats(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetTokenStats([]TokenStats{
		{Model: "opus", TokensIn: 100, TokensOut: 200},
	})
	if len(p.tokenStats) != 1 {
		t.Errorf("Expected 1 token stat, got %d", len(p.tokenStats))
	}
}

func TestLogsPanel_SetResourceStats(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetResourceStats(ResourceStats{
		MemoryMB:   100,
		CPUPercent: 10,
		Goroutines: 50,
		Uptime:     5 * time.Minute,
	})
	if p.resourceStats.MemoryMB != 100 {
		t.Errorf("MemoryMB should be 100, got %f", p.resourceStats.MemoryMB)
	}
}

func TestLogsPanel_SetMachineStats(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetMachineStats(diagnostics.SystemMetrics{
		MemTotalMB: 16000,
		MemUsedMB:  8000,
	})
	if p.machineStats.MemTotalMB != 16000 {
		t.Errorf("MemTotalMB should be 16000, got %f", p.machineStats.MemTotalMB)
	}
}

// ---------------------------------------------------------------------------
// ToggleFooter
// ---------------------------------------------------------------------------

func TestLogsPanel_ToggleFooter(t *testing.T) {
	p := NewLogsPanel(10)
	if p.showFooter {
		t.Error("Footer should start hidden")
	}
	p.ToggleFooter()
	if !p.showFooter {
		t.Error("Footer should be visible after toggle")
	}
	p.ToggleFooter()
	if p.showFooter {
		t.Error("Footer should be hidden after second toggle")
	}
}

// ---------------------------------------------------------------------------
// Scroll methods
// ---------------------------------------------------------------------------

func TestLogsPanel_ScrollMethods(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 10)

	// Add enough entries to make scrolling meaningful
	for i := 0; i < 50; i++ {
		p.AddInfo("a", "log line")
	}

	// These should not panic
	p.ScrollUp()
	p.ScrollDown()
	p.PageUp()
	p.PageDown()
	p.GotoTop()
	p.GotoBottom()
}

func TestLogsPanel_ScrollMethods_NotReady(t *testing.T) {
	p := NewLogsPanel(10)
	// Not ready (no SetSize called) - should not panic
	p.ScrollUp()
	p.ScrollDown()
	p.PageUp()
	p.PageDown()
	p.GotoTop()
	p.GotoBottom()
}

func TestLogsPanel_ScrollPercent(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 10)

	for i := 0; i < 50; i++ {
		p.AddInfo("a", "log line")
	}

	pct := p.ScrollPercent()
	// After adding entries with auto-scroll, should be at bottom (1.0)
	if pct < 0 || pct > 1 {
		t.Errorf("ScrollPercent should be [0,1], got %f", pct)
	}
}

func TestLogsPanel_ScrollPercent_NotReady(t *testing.T) {
	p := NewLogsPanel(10)
	pct := p.ScrollPercent()
	if pct != 0 {
		t.Errorf("ScrollPercent when not ready should return 0, got %f", pct)
	}
}

func TestLogsPanel_AtBottom(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 10)

	for i := 0; i < 50; i++ {
		p.AddInfo("a", "log line")
	}

	if !p.AtBottom() {
		t.Error("After adding entries with auto-scroll, should be at bottom")
	}
}

func TestLogsPanel_AtBottom_NotReady(t *testing.T) {
	p := NewLogsPanel(10)
	if !p.AtBottom() {
		t.Error("AtBottom when not ready should return true")
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestLogsPanel_Update(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	// Should not panic with nil message
	p.Update(nil)
}

func TestLogsPanel_Update_NotReady(t *testing.T) {
	p := NewLogsPanel(10)
	p.Update(nil) // Should not panic
}

// ---------------------------------------------------------------------------
// Render / RenderWithFocus
// ---------------------------------------------------------------------------

func TestLogsPanel_Render_NotReady(t *testing.T) {
	p := NewLogsPanel(10)
	result := p.Render()
	if result != "" {
		t.Error("Not ready panel should render empty string")
	}
}

func TestLogsPanel_Render_Ready(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.AddInfo("claude", "hello world")

	result := p.Render()
	if result == "" {
		t.Error("Ready panel with entries should render non-empty")
	}
	if !strings.Contains(result, "Logs") {
		t.Error("Should contain 'Logs' header")
	}
}

func TestLogsPanel_RenderWithFocus_Focused(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.AddInfo("claude", "msg")

	result := p.RenderWithFocus(true)
	if result == "" {
		t.Error("Focused render should be non-empty")
	}
}

func TestLogsPanel_RenderWithFocus_NotFocused(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.AddInfo("claude", "msg")

	result := p.RenderWithFocus(false)
	if result == "" {
		t.Error("Not-focused render should be non-empty")
	}
}

func TestLogsPanel_Render_WithScrollIndicator(t *testing.T) {
	p := NewLogsPanel(100)
	p.SetSize(80, 10)

	for i := 0; i < 50; i++ {
		p.AddInfo("a", "log line")
	}

	// Scroll to top to trigger scroll indicator
	p.GotoTop()
	result := p.Render()
	if result == "" {
		t.Error("Should render something")
	}
}

func TestLogsPanel_Render_WithFooter(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.ToggleFooter()
	p.SetTokenStats([]TokenStats{
		{Model: "opus", TokensIn: 1500, TokensOut: 2500},
	})
	p.SetResourceStats(ResourceStats{
		MemoryMB:      100,
		CPUPercent:    10,
		CPURawPercent: 30,
		Goroutines:    50,
		Uptime:        5 * time.Minute,
	})
	p.SetMachineStats(diagnostics.SystemMetrics{
		MemTotalMB:  16000,
		MemUsedMB:   8000,
		CPUPercent:  25,
		MemPercent:  50,
		LoadAvg1:    1.0,
		LoadAvg5:    0.8,
		LoadAvg15:   0.5,
		DiskUsedGB:  100,
		DiskTotalGB: 500,
	})

	result := p.Render()
	if !strings.Contains(result, "Logs") {
		t.Error("Should contain 'Logs' header")
	}
}

func TestLogsPanel_Render_WithManyModels(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)
	p.ToggleFooter()
	p.SetTokenStats([]TokenStats{
		{Model: "opus", TokensIn: 1500, TokensOut: 2500},
		{Model: "sonnet", TokensIn: 1000, TokensOut: 1000},
		{Model: "haiku", TokensIn: 500, TokensOut: 500},
		{Model: "flash", TokensIn: 200, TokensOut: 200},
	})

	result := p.Render()
	if result == "" {
		t.Error("Should render something with many models")
	}
}

// ---------------------------------------------------------------------------
// GetPlainText
// ---------------------------------------------------------------------------

func TestLogsPanel_GetPlainText(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	p.AddInfo("claude", "info message")
	p.AddWarn("gemini", "warning message")
	p.AddError("codex", "error message")
	p.AddSuccess("sys", "success message")
	p.AddDebug("sys", "debug message")

	text := p.GetPlainText()

	if !strings.Contains(text, "[INFO]") {
		t.Error("Plain text should contain [INFO]")
	}
	if !strings.Contains(text, "[WARN]") {
		t.Error("Plain text should contain [WARN]")
	}
	if !strings.Contains(text, "[ERROR]") {
		t.Error("Plain text should contain [ERROR]")
	}
	if !strings.Contains(text, "[SUCCESS]") {
		t.Error("Plain text should contain [SUCCESS]")
	}
	if !strings.Contains(text, "[DEBUG]") {
		t.Error("Plain text should contain [DEBUG]")
	}
	if !strings.Contains(text, "claude") {
		t.Error("Plain text should contain source name")
	}
}

func TestLogsPanel_GetPlainText_EmptySource(t *testing.T) {
	p := NewLogsPanel(10)
	p.SetSize(80, 30)

	p.AddInfo("", "message with no source")
	text := p.GetPlainText()
	if !strings.Contains(text, "sys") {
		t.Error("Empty source should be replaced with 'sys'")
	}
}

func TestLogsPanel_GetPlainText_Empty(t *testing.T) {
	p := NewLogsPanel(10)
	text := p.GetPlainText()
	if text != "" {
		t.Error("Empty panel should return empty plain text")
	}
}

// ---------------------------------------------------------------------------
// levelString
// ---------------------------------------------------------------------------

func TestLogsPanel_LevelString(t *testing.T) {
	p := NewLogsPanel(10)

	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "DEBUG"},
		{LogLevelInfo, "INFO"},
		{LogLevelWarn, "WARN"},
		{LogLevelError, "ERROR"},
		{LogLevelSuccess, "SUCCESS"},
		{LogLevel(99), "INFO"}, // unknown defaults to INFO
	}

	for _, tt := range tests {
		result := p.levelString(tt.level)
		if result != tt.expected {
			t.Errorf("levelString(%d) = %q, want %q", tt.level, result, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// formatEntry (word wrap, long messages)
// ---------------------------------------------------------------------------

func TestLogsPanel_FormatEntry_ShortMessage(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Source:  "claude",
		Message: "short msg",
	}
	result := p.formatEntry(entry)
	if result == "" {
		t.Error("Short message should produce output")
	}
}

func TestLogsPanel_FormatEntry_LongMessage(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 60
	p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelWarn,
		Source:  "gemini",
		Message: strings.Repeat("word ", 50), // very long message
	}
	result := p.formatEntry(entry)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Error("Long message should be wrapped to multiple lines")
	}
}

func TestLogsPanel_FormatEntry_EmptySource(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Source:  "",
		Message: "msg",
	}
	result := p.formatEntry(entry)
	if !strings.Contains(result, "sys") {
		t.Error("Empty source should be replaced with 'sys'")
	}
}

func TestLogsPanel_FormatEntry_LongSource(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Source:  "verylongsourcename",
		Message: "msg",
	}
	result := p.formatEntry(entry)
	// Source should be truncated to 8 chars
	if strings.Contains(result, "verylongsourcename") {
		t.Error("Long source should be truncated")
	}
}

func TestLogsPanel_FormatEntry_AllLevels(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	levels := []LogLevel{LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelSuccess}
	for _, level := range levels {
		entry := LogEntry{
			Time:    time.Now(),
			Level:   level,
			Source:  "test",
			Message: "msg",
		}
		result := p.formatEntry(entry)
		if result == "" {
			t.Errorf("formatEntry for level %d should produce output", level)
		}
	}
}

func TestLogsPanel_FormatEntry_NarrowWidth(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 30 // very narrow, msgWidth will clamp to 20
	p.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Source:  "test",
		Message: "a short msg",
	}
	result := p.formatEntry(entry)
	if result == "" {
		t.Error("Narrow width should still produce output")
	}
}

// ---------------------------------------------------------------------------
// formatTokenCount
// ---------------------------------------------------------------------------

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{10000, "10.0k"},
		{100000, "100.0k"},
	}

	for _, tt := range tests {
		result := formatTokenCount(tt.tokens)
		if result != tt.expected {
			t.Errorf("formatTokenCount(%d) = %q, want %q", tt.tokens, result, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{3600 * time.Second, "1h00m"},
		{3661 * time.Second, "1h01m"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, result, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// truncateModel
// ---------------------------------------------------------------------------

func TestTruncateModel(t *testing.T) {
	// Short name should not be truncated
	result := truncateModel("opus", 8)
	if result != "opus" {
		t.Errorf("Short name should not be truncated, got %q", result)
	}

	// Exact length should not be truncated
	result = truncateModel("12345678", 8)
	if result != "12345678" {
		t.Errorf("Exact length should not be truncated, got %q", result)
	}

	// Long name should be truncated
	result = truncateModel("claude-opus-4-6", 8)
	if !strings.HasSuffix(result, "\u2026") {
		t.Errorf("Long name should end with ellipsis, got %q", result)
	}
	// The truncated name should be shorter than or equal to the original
	if len(result) >= len("claude-opus-4-6") {
		t.Errorf("Truncated name should be shorter than original, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// getTopModels
// ---------------------------------------------------------------------------

func TestLogsPanel_GetTopModels(t *testing.T) {
	p := NewLogsPanel(10)
	p.tokenStats = []TokenStats{
		{Model: "a", TokensIn: 100, TokensOut: 100},
		{Model: "b", TokensIn: 500, TokensOut: 500},
		{Model: "c", TokensIn: 200, TokensOut: 200},
		{Model: "d", TokensIn: 300, TokensOut: 300},
	}

	top := p.getTopModels(2)
	if len(top) != 2 {
		t.Fatalf("Expected 2 top models, got %d", len(top))
	}
	// First should be "b" (highest total)
	if top[0].Model != "b" {
		t.Errorf("Top model should be 'b', got %q", top[0].Model)
	}
}

func TestLogsPanel_GetTopModels_FewerThanN(t *testing.T) {
	p := NewLogsPanel(10)
	p.tokenStats = []TokenStats{
		{Model: "a", TokensIn: 100, TokensOut: 100},
	}

	top := p.getTopModels(5)
	if len(top) != 1 {
		t.Errorf("When fewer than N, should return all, got %d", len(top))
	}
}

// ---------------------------------------------------------------------------
// formatTwoColumns
// ---------------------------------------------------------------------------

func TestLogsPanel_FormatTwoColumns(t *testing.T) {
	p := NewLogsPanel(10)
	result := p.formatTwoColumns("left", "right", 20)
	if !strings.Contains(result, "left") || !strings.Contains(result, "right") {
		t.Error("Should contain both left and right text")
	}
}

func TestLogsPanel_FormatTwoColumns_NarrowWidth(t *testing.T) {
	p := NewLogsPanel(10)
	result := p.formatTwoColumns("very long left text", "right", 5) // narrow
	if !strings.Contains(result, "right") {
		t.Error("Should still contain right text even with narrow width")
	}
}

// ---------------------------------------------------------------------------
// renderFooter
// ---------------------------------------------------------------------------

func TestLogsPanel_RenderFooter(t *testing.T) {
	p := NewLogsPanel(10)
	p.mu.Lock()
	p.width = 80
	p.tokenStats = []TokenStats{
		{Model: "opus", TokensIn: 1500, TokensOut: 2500},
		{Model: "sonnet", TokensIn: 1000, TokensOut: 1000},
	}
	p.resourceStats = ResourceStats{
		MemoryMB:      100.5,
		CPUPercent:    15.3,
		CPURawPercent: 45.9,
		Goroutines:    50,
		Uptime:        130 * time.Second,
	}
	p.machineStats = diagnostics.SystemMetrics{
		MemTotalMB:  16384,
		MemUsedMB:   8192,
		CPUPercent:  25.0,
		MemPercent:  50.0,
		LoadAvg1:    1.5,
		LoadAvg5:    1.0,
		LoadAvg15:   0.7,
		DiskUsedGB:  250,
		DiskTotalGB: 500,
	}
	p.mu.Unlock()

	result := p.renderFooter()
	if result == "" {
		t.Error("renderFooter should produce output")
	}
	if !strings.Contains(result, "Tokens") {
		t.Error("Should contain 'Tokens' header")
	}
	if !strings.Contains(result, "Quorum") {
		t.Error("Should contain 'Quorum' section")
	}
	if !strings.Contains(result, "Machine") {
		t.Error("Should contain 'Machine' section")
	}
}
