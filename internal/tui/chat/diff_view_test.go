package chat

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewAgentDiffView
// ---------------------------------------------------------------------------

func TestNewAgentDiffView(t *testing.T) {
	dv := NewAgentDiffView()
	if dv == nil {
		t.Fatal("NewAgentDiffView returned nil")
	}
	if dv.visible {
		t.Error("new diff view should not be visible")
	}
	if dv.HasContent() {
		t.Error("new diff view should have no content")
	}
	if len(dv.agentPairs) != 0 {
		t.Error("new diff view should have no agent pairs")
	}
}

// ---------------------------------------------------------------------------
// Toggle / Show / Hide / IsVisible
// ---------------------------------------------------------------------------

func TestAgentDiffView_ToggleShowHide(t *testing.T) {
	dv := NewAgentDiffView()

	dv.Toggle()
	if !dv.IsVisible() {
		t.Error("expected visible after Toggle")
	}

	dv.Toggle()
	if dv.IsVisible() {
		t.Error("expected hidden after second Toggle")
	}

	dv.Show()
	if !dv.IsVisible() {
		t.Error("expected visible after Show")
	}

	dv.Hide()
	if dv.IsVisible() {
		t.Error("expected hidden after Hide")
	}
}

// ---------------------------------------------------------------------------
// SetAgents / SetContent / HasContent
// ---------------------------------------------------------------------------

func TestAgentDiffView_SetAgentsAndContent(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)

	dv.SetAgents("claude", "gemini")
	if dv.leftAgent != "claude" || dv.rightAgent != "gemini" {
		t.Error("SetAgents did not set correctly")
	}

	dv.SetContent("hello\nworld", "hello\nearth")
	if !dv.HasContent() {
		t.Error("should have content after SetContent")
	}
	if len(dv.diffLines) == 0 {
		t.Error("diff lines should be computed")
	}
}

func TestAgentDiffView_HasContent_LeftOnly(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(80, 30)
	dv.SetContent("some text", "")
	if !dv.HasContent() {
		t.Error("should have content when left is set")
	}
}

func TestAgentDiffView_HasContent_RightOnly(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(80, 30)
	dv.SetContent("", "some text")
	if !dv.HasContent() {
		t.Error("should have content when right is set")
	}
}

// ---------------------------------------------------------------------------
// SetSize
// ---------------------------------------------------------------------------

func TestAgentDiffView_SetSize(t *testing.T) {
	dv := NewAgentDiffView()

	dv.SetSize(100, 30)
	if !dv.ready {
		t.Error("expected ready after SetSize")
	}

	// Resize
	dv.SetSize(120, 40)
	if dv.width != 120 || dv.height != 40 {
		t.Errorf("unexpected dimensions %d x %d", dv.width, dv.height)
	}
}

// ---------------------------------------------------------------------------
// Scroll
// ---------------------------------------------------------------------------

func TestAgentDiffView_Scroll(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetContent(
		strings.Repeat("left line\n", 100),
		strings.Repeat("right line\n", 100),
	)
	// Just verify no panics
	dv.ScrollDown()
	dv.ScrollUp()
}

// ---------------------------------------------------------------------------
// Agent Pairs Navigation
// ---------------------------------------------------------------------------

func TestAgentDiffView_AgentPairs_Empty(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetAgents("claude", "gemini")

	if dv.NextPair() {
		t.Error("NextPair should return false with no pairs")
	}
	if dv.PrevPair() {
		t.Error("PrevPair should return false with no pairs")
	}

	left, right := dv.GetCurrentPair()
	if left != "claude" || right != "gemini" {
		t.Errorf("GetCurrentPair should fall back to set agents, got %q %q", left, right)
	}
}

func TestAgentDiffView_AgentPairs_SinglePair(t *testing.T) {
	dv := NewAgentDiffView()
	dv.AddAgentPair("claude", "gemini")

	if dv.NextPair() {
		t.Error("NextPair should return false with single pair")
	}
	if dv.PrevPair() {
		t.Error("PrevPair should return false with single pair")
	}

	left, right := dv.GetCurrentPair()
	if left != "claude" || right != "gemini" {
		t.Errorf("unexpected pair %q %q", left, right)
	}
}

func TestAgentDiffView_AgentPairs_MultiplePairs(t *testing.T) {
	dv := NewAgentDiffView()
	dv.AddAgentPair("claude", "gemini")
	dv.AddAgentPair("codex", "copilot")
	dv.AddAgentPair("claude", "codex")

	// Navigate forward
	if !dv.NextPair() {
		t.Error("NextPair should return true with multiple pairs")
	}
	left, right := dv.GetCurrentPair()
	if left != "codex" || right != "copilot" {
		t.Errorf("expected pair 1, got %q %q", left, right)
	}

	// Navigate forward again
	dv.NextPair()
	left, right = dv.GetCurrentPair()
	if left != "claude" || right != "codex" {
		t.Errorf("expected pair 2, got %q %q", left, right)
	}

	// Wrap around
	dv.NextPair()
	left, right = dv.GetCurrentPair()
	if left != "claude" || right != "gemini" {
		t.Errorf("expected pair 0 after wrap, got %q %q", left, right)
	}

	// Navigate backward
	if !dv.PrevPair() {
		t.Error("PrevPair should return true with multiple pairs")
	}
	left, right = dv.GetCurrentPair()
	if left != "claude" || right != "codex" {
		t.Errorf("expected pair 2 after PrevPair from 0, got %q %q", left, right)
	}
}

func TestAgentDiffView_ClearPairs(t *testing.T) {
	dv := NewAgentDiffView()
	dv.AddAgentPair("a", "b")
	dv.AddAgentPair("c", "d")
	dv.currentPair = 1

	dv.ClearPairs()
	if len(dv.agentPairs) != 0 {
		t.Error("ClearPairs should empty pairs")
	}
	if dv.currentPair != 0 {
		t.Error("ClearPairs should reset currentPair to 0")
	}
}

// ---------------------------------------------------------------------------
// computeDiff
// ---------------------------------------------------------------------------

func TestAgentDiffView_ComputeDiff_EqualContent(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetContent("hello\nworld", "hello\nworld")

	for i, line := range dv.diffLines {
		if !line.IsCommon {
			t.Errorf("line %d should be common: left=%q right=%q", i, line.Left, line.Right)
		}
	}
}

func TestAgentDiffView_ComputeDiff_DifferentContent(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetContent("hello\nworld", "hello\nearth")

	if len(dv.diffLines) != 2 {
		t.Fatalf("expected 2 diff lines, got %d", len(dv.diffLines))
	}
	if !dv.diffLines[0].IsCommon {
		t.Error("first line should be common")
	}
	if dv.diffLines[1].IsCommon {
		t.Error("second line should differ")
	}
}

func TestAgentDiffView_ComputeDiff_UnequalLength(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetContent("a\nb\nc", "a\nb")

	if len(dv.diffLines) != 3 {
		t.Fatalf("expected 3 diff lines, got %d", len(dv.diffLines))
	}
	// Third line: left = "c", right = ""
	if dv.diffLines[2].Left != "c" {
		t.Errorf("expected left='c', got %q", dv.diffLines[2].Left)
	}
	if dv.diffLines[2].Right != "" {
		t.Errorf("expected right='', got %q", dv.diffLines[2].Right)
	}
}

// ---------------------------------------------------------------------------
// truncateOrPad
// ---------------------------------------------------------------------------

func TestTruncateOrPad_Short(t *testing.T) {
	result := truncateOrPad("hi", 10)
	if len(result) != 10 {
		t.Errorf("expected len 10, got %d", len(result))
	}
	if !strings.HasPrefix(result, "hi") {
		t.Errorf("expected prefix 'hi', got %q", result)
	}
}

func TestTruncateOrPad_Exact(t *testing.T) {
	result := truncateOrPad("hello", 5)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncateOrPad_Long(t *testing.T) {
	result := truncateOrPad("hello world this is a long line", 10)
	if len([]rune(result)) != 10 {
		t.Errorf("expected rune len 10, got %d", len([]rune(result)))
	}
	if !strings.HasSuffix(result, "\u2026") { // ellipsis
		t.Errorf("expected ellipsis suffix, got %q", result)
	}
}

func TestTruncateOrPad_WithNewlines(t *testing.T) {
	result := truncateOrPad("hello\n\r", 10)
	// Trailing newlines should be stripped
	if strings.ContainsAny(result, "\n\r") {
		t.Error("trailing newlines should be stripped")
	}
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

func TestAgentDiffView_Render_NotVisible(t *testing.T) {
	dv := NewAgentDiffView()
	if dv.Render() != "" {
		t.Error("hidden diff view should render empty string")
	}
}

func TestAgentDiffView_Render_EmptyContent(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetAgents("claude", "gemini")
	dv.Show()

	rendered := dv.Render()
	if rendered == "" {
		t.Error("visible diff view should render something")
	}
	if !strings.Contains(rendered, "No content to compare") {
		t.Error("empty diff should show 'No content to compare'")
	}
}

func TestAgentDiffView_Render_WithContent(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetAgents("claude", "gemini")
	dv.SetContent("hello\nworld", "hello\nearth")
	dv.Show()

	rendered := dv.Render()
	if rendered == "" {
		t.Fatal("rendered output should not be empty")
	}
	if !strings.Contains(rendered, "claude") {
		t.Error("render should contain left agent name")
	}
	if !strings.Contains(rendered, "gemini") {
		t.Error("render should contain right agent name")
	}
	if !strings.Contains(rendered, "Agent Diff") {
		t.Error("render should contain title")
	}
}

func TestAgentDiffView_Render_WithMultiplePairs(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.SetAgents("claude", "gemini")
	dv.AddAgentPair("claude", "gemini")
	dv.AddAgentPair("codex", "copilot")
	dv.Show()

	rendered := dv.Render()
	// Should show pair counter
	if !strings.Contains(rendered, "Agent Diff") {
		t.Error("render should contain title")
	}
}

// ---------------------------------------------------------------------------
// renderDiffContent
// ---------------------------------------------------------------------------

func TestAgentDiffView_RenderDiffContent_Empty(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	result := dv.renderDiffContent()
	if !strings.Contains(result, "No content to compare") {
		t.Error("empty diff content should show message")
	}
}

func TestAgentDiffView_RenderDiffContent_WithLines(t *testing.T) {
	dv := NewAgentDiffView()
	dv.SetSize(100, 30)
	dv.diffLines = []DiffLine{
		{Left: "same", Right: "same", IsCommon: true},
		{Left: "left val", Right: "right val", IsCommon: false},
	}
	result := dv.renderDiffContent()
	if result == "" {
		t.Error("render should produce output")
	}
}

// ---------------------------------------------------------------------------
// updateViewport - not ready
// ---------------------------------------------------------------------------

func TestAgentDiffView_UpdateViewport_NotReady(t *testing.T) {
	dv := NewAgentDiffView()
	dv.updateViewport() // should not panic
}
