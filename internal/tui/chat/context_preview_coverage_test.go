package chat

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewContextPreviewPanel
// ---------------------------------------------------------------------------

func TestNewContextPreviewPanel(t *testing.T) {
	p := NewContextPreviewPanel()
	if p == nil {
		t.Fatal("NewContextPreviewPanel returned nil")
	}
	if !p.visible {
		t.Error("new panel should be visible by default")
	}
	if len(p.files) != 0 {
		t.Error("new panel should have no files")
	}
	if len(p.directories) != 0 {
		t.Error("new panel should have no directories")
	}
	if p.tokensMax != 100000 {
		t.Errorf("default tokensMax should be 100000, got %d", p.tokensMax)
	}
}

// ---------------------------------------------------------------------------
// Toggle / IsVisible
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_ToggleVisibility(t *testing.T) {
	p := NewContextPreviewPanel()

	if !p.IsVisible() {
		t.Error("should be visible initially")
	}

	p.Toggle()
	if p.IsVisible() {
		t.Error("should be hidden after toggle")
	}

	p.Toggle()
	if !p.IsVisible() {
		t.Error("should be visible again after second toggle")
	}
}

// ---------------------------------------------------------------------------
// SetFiles / AddFile / AddDirectory
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_SetFiles(t *testing.T) {
	p := NewContextPreviewPanel()
	files := []ContextFile{
		{Path: "/foo/bar.go", Size: 1024},
		{Path: "/foo/baz.rs", Size: 2048},
	}
	p.SetFiles(files)
	if len(p.files) != 2 {
		t.Errorf("expected 2 files, got %d", len(p.files))
	}
}

func TestContextPreviewPanel_AddFile(t *testing.T) {
	p := NewContextPreviewPanel()
	p.AddFile("/test/file.go", 512)
	if len(p.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(p.files))
	}
	if p.files[0].Path != "/test/file.go" || p.files[0].Size != 512 {
		t.Error("file was not added correctly")
	}
}

func TestContextPreviewPanel_AddDirectory(t *testing.T) {
	p := NewContextPreviewPanel()
	p.AddDirectory("/src")
	p.AddDirectory("/test")
	if len(p.directories) != 2 {
		t.Errorf("expected 2 directories, got %d", len(p.directories))
	}
}

// ---------------------------------------------------------------------------
// SetMessageCount / SetTokens / SetCurrentAgent
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_SetMessageCount(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetMessageCount(42)
	if p.messageCount != 42 {
		t.Errorf("expected 42, got %d", p.messageCount)
	}
}

func TestContextPreviewPanel_SetTokens(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetTokens(5000, 50000)
	if p.tokensUsed != 5000 || p.tokensMax != 50000 {
		t.Errorf("unexpected tokens: used=%d max=%d", p.tokensUsed, p.tokensMax)
	}
}

func TestContextPreviewPanel_SetCurrentAgent(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetCurrentAgent("claude")
	if p.currentAgent != "claude" {
		t.Errorf("expected 'claude', got %q", p.currentAgent)
	}
}

// ---------------------------------------------------------------------------
// Clear
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_Clear(t *testing.T) {
	p := NewContextPreviewPanel()
	p.AddFile("/f.go", 100)
	p.AddDirectory("/d")
	p.SetMessageCount(10)
	p.SetTokens(500, 1000)

	p.Clear()

	if len(p.files) != 0 {
		t.Error("files should be cleared")
	}
	if len(p.directories) != 0 {
		t.Error("directories should be cleared")
	}
	if p.messageCount != 0 {
		t.Error("messageCount should be cleared")
	}
	if p.tokensUsed != 0 {
		t.Error("tokensUsed should be cleared")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_SetSize(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	if p.width != 60 || p.height != 20 {
		t.Errorf("unexpected size %d x %d", p.width, p.height)
	}
}

// ---------------------------------------------------------------------------
// formatTokens
// ---------------------------------------------------------------------------

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{50000, "50.0k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tc := range tests {
		got := formatTokens(tc.input)
		if got != tc.expected {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_Render_NotVisible(t *testing.T) {
	p := NewContextPreviewPanel()
	p.Toggle() // make it hidden
	if p.Render() != "" {
		t.Error("hidden panel should render empty string")
	}
}

func TestContextPreviewPanel_Render_Visible_Empty(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)

	rendered := p.Render()
	if rendered == "" {
		t.Fatal("visible panel should render something")
	}
	if !strings.Contains(rendered, "Context") {
		t.Error("render should contain 'Context' header")
	}
}

func TestContextPreviewPanel_Render_WithAgent(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.SetCurrentAgent("gemini")

	rendered := p.Render()
	if !strings.Contains(rendered, "gemini") {
		t.Error("render should contain agent name")
	}
}

func TestContextPreviewPanel_Render_WithFiles(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.AddFile("/project/main.go", 2048)
	p.AddFile("/project/util.go", 1024)

	rendered := p.Render()
	if !strings.Contains(rendered, "main.go") {
		t.Error("render should contain file name")
	}
}

func TestContextPreviewPanel_Render_WithDirectories(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.AddDirectory("/project/src")
	p.AddDirectory("/project/test")

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output with directories")
	}
}

func TestContextPreviewPanel_Render_DotDirectory(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.AddDirectory(".")

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output")
	}
}

func TestContextPreviewPanel_Render_EmptyDirName(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.AddDirectory("")

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output")
	}
}

func TestContextPreviewPanel_Render_Truncation(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	// Add more than maxItems (4) items
	for i := 0; i < 6; i++ {
		p.AddFile(fmt.Sprintf("/project/file%d.go", i), int64(i*100))
	}

	rendered := p.Render()
	if !strings.Contains(rendered, "more...") {
		t.Error("render should show truncation indicator")
	}
}

func TestContextPreviewPanel_Render_TokenColors(t *testing.T) {
	tests := []struct {
		name      string
		used, max int
	}{
		{"green (low usage)", 1000, 100000},
		{"yellow (70%+)", 75000, 100000},
		{"red (90%+)", 95000, 100000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewContextPreviewPanel()
			p.SetSize(60, 20)
			p.SetTokens(tc.used, tc.max)

			rendered := p.Render()
			if rendered == "" {
				t.Error("render should produce output")
			}
		})
	}
}

func TestContextPreviewPanel_Render_SmallBarWidth(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(15, 20) // width - 20 < 8
	p.SetTokens(5000, 10000)

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output even with small width")
	}
}

func TestContextPreviewPanel_Render_ZeroMaxTokens(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.SetTokens(0, 0)

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output with zero max tokens")
	}
}

func TestContextPreviewPanel_Render_TokenBarOverflow(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetSize(60, 20)
	p.SetTokens(200000, 100000) // used > max

	rendered := p.Render()
	if rendered == "" {
		t.Error("render should produce output")
	}
}

// ---------------------------------------------------------------------------
// CompactRender
// ---------------------------------------------------------------------------

func TestContextPreviewPanel_CompactRender_Empty(t *testing.T) {
	p := NewContextPreviewPanel()
	compact := p.CompactRender()
	if compact != "" {
		t.Error("compact render with no data should be empty")
	}
}

func TestContextPreviewPanel_CompactRender_WithFiles(t *testing.T) {
	p := NewContextPreviewPanel()
	p.AddFile("/f.go", 100)
	compact := p.CompactRender()
	if compact == "" {
		t.Error("compact render should include file count")
	}
}

func TestContextPreviewPanel_CompactRender_WithMessages(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetMessageCount(5)
	compact := p.CompactRender()
	if !strings.Contains(compact, "5") {
		t.Error("compact render should include message count")
	}
}

func TestContextPreviewPanel_CompactRender_WithTokens(t *testing.T) {
	p := NewContextPreviewPanel()
	p.SetTokens(1500, 10000)
	compact := p.CompactRender()
	if !strings.Contains(compact, "1.5k") {
		t.Error("compact render should include token count")
	}
}

func TestContextPreviewPanel_CompactRender_Full(t *testing.T) {
	p := NewContextPreviewPanel()
	p.AddFile("/f.go", 100)
	p.SetMessageCount(3)
	p.SetTokens(2000, 10000)

	compact := p.CompactRender()
	if compact == "" {
		t.Error("compact render should produce output")
	}
}
