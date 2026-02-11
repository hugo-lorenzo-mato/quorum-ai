package chat

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewTokenPanel
// ---------------------------------------------------------------------------

func TestNewTokenPanel(t *testing.T) {
	p := NewTokenPanel()
	if p == nil {
		t.Fatal("NewTokenPanel returned nil")
	}
	if len(p.entries) != 0 {
		t.Error("New panel should have no entries")
	}
}

// ---------------------------------------------------------------------------
// SetSize / Width / Height
// ---------------------------------------------------------------------------

func TestTokenPanel_SetSize(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	if p.Width() != 80 {
		t.Errorf("Width should be 80, got %d", p.Width())
	}
	if p.Height() != 30 {
		t.Errorf("Height should be 30, got %d", p.Height())
	}
}

func TestTokenPanel_SetSize_Small(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(30, 5) // very small, viewport height clamps to 3
	if p.Width() != 30 {
		t.Errorf("Width should be 30, got %d", p.Width())
	}
}

func TestTokenPanel_SetSize_Update(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)
	p.SetSize(100, 50)
	if p.Width() != 100 {
		t.Errorf("Width should be 100 after resize, got %d", p.Width())
	}
}

// ---------------------------------------------------------------------------
// SetEntries
// ---------------------------------------------------------------------------

func TestTokenPanel_SetEntries(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	entries := []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "direct", TokensIn: 100, TokensOut: 200},
		{Scope: "workflow", CLI: "gemini", Model: "flash", Phase: "analyze", TokensIn: 500, TokensOut: 300},
	}
	p.SetEntries(entries)

	p.mu.Lock()
	count := len(p.entries)
	p.mu.Unlock()

	if count != 2 {
		t.Errorf("Expected 2 entries, got %d", count)
	}
}

func TestTokenPanel_SetEntries_Empty(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)
	p.SetEntries(nil)

	p.mu.Lock()
	count := len(p.entries)
	p.mu.Unlock()

	if count != 0 {
		t.Errorf("Expected 0 entries, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestTokenPanel_Update(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)
	// Should not panic
	p.Update(nil)
}

func TestTokenPanel_Update_NotReady(t *testing.T) {
	p := NewTokenPanel()
	// Not ready - should not panic
	p.Update(nil)
}

// ---------------------------------------------------------------------------
// Scroll methods
// ---------------------------------------------------------------------------

func TestTokenPanel_ScrollMethods(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 10)

	// Add entries to make scrolling meaningful
	entries := make([]TokenEntry, 50)
	for i := range entries {
		entries[i] = TokenEntry{
			Scope:     "chat",
			CLI:       "claude",
			Model:     "opus",
			Phase:     "direct",
			TokensIn:  100,
			TokensOut: 200,
		}
	}
	p.SetEntries(entries)

	// These should not panic
	p.ScrollDown()
	p.ScrollUp()
	p.PageDown()
	p.PageUp()
	p.GotoBottom()
	p.GotoTop()
}

func TestTokenPanel_ScrollMethods_NotReady(t *testing.T) {
	p := NewTokenPanel()
	// Not ready - should not panic
	p.ScrollDown()
	p.ScrollUp()
	p.PageDown()
	p.PageUp()
	p.GotoBottom()
	p.GotoTop()
}

// ---------------------------------------------------------------------------
// Render / RenderWithFocus
// ---------------------------------------------------------------------------

func TestTokenPanel_Render_NotReady(t *testing.T) {
	p := NewTokenPanel()
	result := p.Render()
	if result != "" {
		t.Error("Not ready panel should render empty string")
	}
}

func TestTokenPanel_Render_Empty(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	result := p.Render()
	if result == "" {
		t.Error("Ready panel should render something even with no entries")
	}
	if !strings.Contains(result, "Tokens") {
		t.Error("Should contain 'Tokens' header")
	}
}

func TestTokenPanel_Render_WithEntries(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	entries := []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "direct", TokensIn: 1500, TokensOut: 2500},
		{Scope: "workflow", CLI: "gemini", Model: "flash", Phase: "analyze", TokensIn: 500, TokensOut: 300},
	}
	p.SetEntries(entries)

	result := p.Render()
	if !strings.Contains(result, "Tokens") {
		t.Error("Should contain 'Tokens' header")
	}
}

func TestTokenPanel_RenderWithFocus_Focused(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	result := p.RenderWithFocus(true)
	if result == "" {
		t.Error("Focused render should produce output")
	}
}

func TestTokenPanel_RenderWithFocus_NotFocused(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 30)

	result := p.RenderWithFocus(false)
	if result == "" {
		t.Error("Not focused render should produce output")
	}
}

func TestTokenPanel_RenderWithFocus_NotReady(t *testing.T) {
	p := NewTokenPanel()
	result := p.RenderWithFocus(true)
	if result != "" {
		t.Error("Not ready panel should render empty even when focused")
	}
}

// ---------------------------------------------------------------------------
// renderContent
// ---------------------------------------------------------------------------

func TestTokenPanel_RenderContent_Empty(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 80
	p.mu.Unlock()

	content := p.renderContent()
	if !strings.Contains(content, "No token data yet") {
		t.Error("Empty entries should show 'No token data yet'")
	}
}

func TestTokenPanel_RenderContent_ChatOnly(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 80
	p.entries = []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "direct", TokensIn: 100, TokensOut: 200},
	}
	p.mu.Unlock()

	content := p.renderContent()
	if !strings.Contains(content, "Chat") {
		t.Error("Should contain 'Chat' section header")
	}
}

func TestTokenPanel_RenderContent_WorkflowOnly(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 80
	p.entries = []TokenEntry{
		{Scope: "workflow", CLI: "gemini", Model: "flash", Phase: "analyze", TokensIn: 500, TokensOut: 300},
	}
	p.mu.Unlock()

	content := p.renderContent()
	if !strings.Contains(content, "Workflow") {
		t.Error("Should contain 'Workflow' section header")
	}
}

func TestTokenPanel_RenderContent_BothScopes(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 80
	p.entries = []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "direct", TokensIn: 100, TokensOut: 200},
		{Scope: "workflow", CLI: "gemini", Model: "flash", Phase: "analyze", TokensIn: 500, TokensOut: 300},
	}
	p.mu.Unlock()

	content := p.renderContent()
	if !strings.Contains(content, "Chat") {
		t.Error("Should contain 'Chat' section")
	}
	if !strings.Contains(content, "Workflow") {
		t.Error("Should contain 'Workflow' section")
	}
	if !strings.Contains(content, "ALL") {
		t.Error("Should contain 'ALL' grand total")
	}
}

func TestTokenPanel_RenderContent_SortsByTotalTokens(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 80
	p.entries = []TokenEntry{
		{Scope: "chat", CLI: "low", Model: "m1", Phase: "p", TokensIn: 10, TokensOut: 10},
		{Scope: "chat", CLI: "high", Model: "m2", Phase: "p", TokensIn: 1000, TokensOut: 1000},
		{Scope: "chat", CLI: "mid", Model: "m3", Phase: "p", TokensIn: 100, TokensOut: 100},
	}
	p.mu.Unlock()

	content := p.renderContent()
	// "high" should appear before "low" in the output due to sorting
	highIdx := strings.Index(content, "high")
	lowIdx := strings.Index(content, "low")
	if highIdx == -1 || lowIdx == -1 {
		t.Error("Both 'high' and 'low' entries should appear in output")
	} else if highIdx > lowIdx {
		t.Error("Higher token entries should appear first")
	}
}

func TestTokenPanel_RenderContent_NarrowWidth(t *testing.T) {
	p := NewTokenPanel()
	p.mu.Lock()
	p.width = 25 // very narrow, innerWidth clamps to 20
	p.entries = []TokenEntry{
		{Scope: "chat", CLI: "claude", Model: "opus", Phase: "direct", TokensIn: 100, TokensOut: 200},
	}
	p.mu.Unlock()

	content := p.renderContent()
	if content == "" {
		t.Error("Narrow width should still produce content")
	}
}

// ---------------------------------------------------------------------------
// padOrTrim
// ---------------------------------------------------------------------------

func TestPadOrTrim_Short(t *testing.T) {
	result := padOrTrim("abc", 8)
	if len(result) != 8 {
		t.Errorf("Should pad to width 8, got len %d: %q", len(result), result)
	}
	if !strings.HasPrefix(result, "abc") {
		t.Error("Should start with original string")
	}
}

func TestPadOrTrim_Exact(t *testing.T) {
	result := padOrTrim("12345678", 8)
	if result != "12345678" {
		t.Errorf("Exact width should not change, got %q", result)
	}
}

func TestPadOrTrim_Long(t *testing.T) {
	result := padOrTrim("this is too long", 8)
	w := len(result)
	if w > 8 {
		t.Errorf("Should trim to width 8, got len %d: %q", w, result)
	}
}

// ---------------------------------------------------------------------------
// Render with scroll indicator
// ---------------------------------------------------------------------------

func TestTokenPanel_Render_WithScroll(t *testing.T) {
	p := NewTokenPanel()
	p.SetSize(80, 10) // small viewport

	// Add many entries to make viewport scroll
	entries := make([]TokenEntry, 30)
	for i := range entries {
		entries[i] = TokenEntry{
			Scope:     "chat",
			CLI:       "claude",
			Model:     "opus",
			Phase:     "direct",
			TokensIn:  100,
			TokensOut: 200,
		}
	}
	p.SetEntries(entries)

	// Scroll to top to trigger scroll indicator
	p.GotoTop()
	result := p.Render()
	if result == "" {
		t.Error("Should render something with scroll indicator")
	}
}
