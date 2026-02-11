package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// --- OutputStyle ---

func TestOutputStyle_ReturnsStyle(t *testing.T) {
	t.Parallel()

	color := lipgloss.Color("#7c3aed")
	style := OutputStyle(color)
	result := style.Render("hello world")
	if len(result) == 0 {
		t.Error("OutputStyle.Render returned empty string")
	}
	if !strings.Contains(result, "hello world") {
		t.Error("OutputStyle.Render should contain input text")
	}
}

func TestOutputStyle_DifferentColors(t *testing.T) {
	t.Parallel()

	colors := []lipgloss.Color{
		lipgloss.Color("#7c3aed"),
		lipgloss.Color("#3b82f6"),
		lipgloss.Color("#10b981"),
		lipgloss.Color("#ef4444"),
	}

	for _, color := range colors {
		style := OutputStyle(color)
		result := style.Render("test")
		if len(result) == 0 {
			t.Errorf("OutputStyle(%v).Render returned empty", color)
		}
	}
}

// --- RenderAgentOutput ---

func TestRenderAgentOutput_EmptyOutput(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "claude",
		Output: "",
		Color:  lipgloss.Color("#7c3aed"),
	}

	result := RenderAgentOutput(agent, 80)
	if result != "" {
		t.Errorf("RenderAgentOutput with empty output should return empty, got %q", result)
	}
}

func TestRenderAgentOutput_WithOutput(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "claude",
		Output: "This is the response",
		Color:  lipgloss.Color("#7c3aed"),
	}

	result := RenderAgentOutput(agent, 80)
	if len(result) == 0 {
		t.Fatal("RenderAgentOutput returned empty string with non-empty output")
	}
	if !strings.Contains(result, "claude") {
		t.Error("output should contain agent name 'claude'")
	}
	if !strings.Contains(result, "This is the response") {
		t.Error("output should contain the agent's output text")
	}
}

func TestRenderAgentOutput_WithDuration(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:     "gemini",
		Output:   "Some output",
		Color:    lipgloss.Color("#3b82f6"),
		Duration: 567 * time.Millisecond,
	}

	result := RenderAgentOutput(agent, 80)
	if !strings.Contains(result, "gemini") {
		t.Error("output should contain agent name 'gemini'")
	}
	if !strings.Contains(result, "567ms") {
		t.Error("output should contain duration '567ms'")
	}
}

func TestRenderAgentOutput_ZeroDuration(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:     "codex",
		Output:   "Result here",
		Color:    lipgloss.Color("#10b981"),
		Duration: 0,
	}

	result := RenderAgentOutput(agent, 80)
	if !strings.Contains(result, "codex") {
		t.Error("output should contain agent name 'codex'")
	}
	if !strings.Contains(result, "Result here") {
		t.Error("output should contain agent output")
	}
	// With zero duration, should not show "0ms" in the header
	if strings.Contains(result, "0ms") {
		t.Error("output with zero duration should not show '0ms'")
	}
}

func TestRenderAgentOutput_LargeDuration(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:     "claude",
		Output:   "Done",
		Color:    lipgloss.Color("#7c3aed"),
		Duration: 12345 * time.Millisecond,
	}

	result := RenderAgentOutput(agent, 120)
	if !strings.Contains(result, "12345ms") {
		t.Error("output should contain duration '12345ms'")
	}
}

func TestRenderAgentOutput_NarrowWidth(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "claude",
		Output: "This is some text",
		Color:  lipgloss.Color("#7c3aed"),
	}

	// Should not panic with narrow widths
	result := RenderAgentOutput(agent, 20)
	if len(result) == 0 {
		t.Error("RenderAgentOutput should return non-empty even with narrow width")
	}
}

func TestRenderAgentOutput_MultilineOutput(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "gemini",
		Output: "line1\nline2\nline3",
		Color:  lipgloss.Color("#3b82f6"),
	}

	result := RenderAgentOutput(agent, 80)
	if !strings.Contains(result, "line1") {
		t.Error("output should contain 'line1'")
	}
	if !strings.Contains(result, "line2") {
		t.Error("output should contain 'line2'")
	}
	if !strings.Contains(result, "line3") {
		t.Error("output should contain 'line3'")
	}
}
