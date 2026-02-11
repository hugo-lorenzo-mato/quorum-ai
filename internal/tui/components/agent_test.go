package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// --- AgentStatus constants ---

func TestAgentStatusConstants(t *testing.T) {
	t.Parallel()

	if StatusIdle != 0 {
		t.Errorf("StatusIdle = %d, want 0", StatusIdle)
	}
	if StatusWorking != 1 {
		t.Errorf("StatusWorking = %d, want 1", StatusWorking)
	}
	if StatusDone != 2 {
		t.Errorf("StatusDone = %d, want 2", StatusDone)
	}
	if StatusError != 3 {
		t.Errorf("StatusError = %d, want 3", StatusError)
	}
}

// --- AgentColors map ---

func TestAgentColors_KnownAgents(t *testing.T) {
	t.Parallel()

	expected := map[string]lipgloss.Color{
		"claude":  lipgloss.Color("#7c3aed"),
		"gemini":  lipgloss.Color("#3b82f6"),
		"codex":   lipgloss.Color("#10b981"),
		"openai":  lipgloss.Color("#10b981"),
		"gpt":     lipgloss.Color("#10b981"),
		"default": lipgloss.Color("#6b7280"),
	}

	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			got, ok := AgentColors[name]
			if !ok {
				t.Fatalf("AgentColors[%q] not found", name)
			}
			if got != want {
				t.Errorf("AgentColors[%q] = %v, want %v", name, got, want)
			}
		})
	}
}

func TestAgentColors_Length(t *testing.T) {
	t.Parallel()
	if len(AgentColors) != 6 {
		t.Errorf("len(AgentColors) = %d, want 6", len(AgentColors))
	}
}

// --- GetAgentColor ---

func TestGetAgentColor_Known(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want lipgloss.Color
	}{
		{"claude", lipgloss.Color("#7c3aed")},
		{"gemini", lipgloss.Color("#3b82f6")},
		{"codex", lipgloss.Color("#10b981")},
		{"openai", lipgloss.Color("#10b981")},
		{"gpt", lipgloss.Color("#10b981")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAgentColor(tt.name)
			if got != tt.want {
				t.Errorf("GetAgentColor(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetAgentColor_Unknown(t *testing.T) {
	t.Parallel()

	unknowns := []string{"copilot", "opencode", "unknown", "", "CLAUDE"}
	for _, name := range unknowns {
		t.Run(name, func(t *testing.T) {
			got := GetAgentColor(name)
			want := AgentColors["default"]
			if got != want {
				t.Errorf("GetAgentColor(%q) = %v, want default %v", name, got, want)
			}
		})
	}
}

// --- Agent struct ---

func TestAgentStruct_Fields(t *testing.T) {
	t.Parallel()

	agent := Agent{
		ID:       "test-1",
		Name:     "claude",
		Status:   StatusWorking,
		Color:    lipgloss.Color("#7c3aed"),
		Duration: 500 * time.Millisecond,
		Output:   "some output",
		Error:    "",
	}

	if agent.ID != "test-1" {
		t.Errorf("ID = %q, want %q", agent.ID, "test-1")
	}
	if agent.Name != "claude" {
		t.Errorf("Name = %q, want %q", agent.Name, "claude")
	}
	if agent.Status != StatusWorking {
		t.Errorf("Status = %d, want StatusWorking", agent.Status)
	}
	if agent.Duration != 500*time.Millisecond {
		t.Errorf("Duration = %v, want 500ms", agent.Duration)
	}
	if agent.Output != "some output" {
		t.Errorf("Output = %q, want %q", agent.Output, "some output")
	}
	if agent.Error != "" {
		t.Errorf("Error = %q, want empty", agent.Error)
	}
}

func TestAgentStruct_ZeroValue(t *testing.T) {
	t.Parallel()

	var agent Agent
	if agent.ID != "" {
		t.Errorf("zero value ID = %q, want empty", agent.ID)
	}
	if agent.Status != StatusIdle {
		t.Errorf("zero value Status = %d, want StatusIdle (0)", agent.Status)
	}
	if agent.Duration != 0 {
		t.Errorf("zero value Duration = %v, want 0", agent.Duration)
	}
}

// --- Styles ---

func TestAgentCardStyle_NotNil(t *testing.T) {
	t.Parallel()
	// agentCardStyle is a package-level var; verify it renders without panic
	result := agentCardStyle.Render("test")
	if len(result) == 0 {
		t.Error("agentCardStyle.Render returned empty string")
	}
}

func TestAgentActiveStyle_Function(t *testing.T) {
	t.Parallel()
	color := lipgloss.Color("#ff0000")
	style := agentActiveStyle(color)
	result := style.Render("test")
	if len(result) == 0 {
		t.Error("agentActiveStyle returned style that renders empty")
	}
}

func TestAgentNameStyle_Function(t *testing.T) {
	t.Parallel()
	color := lipgloss.Color("#00ff00")
	style := agentNameStyle(color)
	result := style.Render("test")
	if len(result) == 0 {
		t.Error("agentNameStyle returned style that renders empty")
	}
}

func TestDimStyle(t *testing.T) {
	t.Parallel()
	result := dimStyle.Render("dim text")
	if !strings.Contains(result, "dim text") {
		t.Error("dimStyle.Render does not contain input text")
	}
}

func TestGreenStyle(t *testing.T) {
	t.Parallel()
	result := greenStyle.Render("green text")
	if !strings.Contains(result, "green text") {
		t.Error("greenStyle.Render does not contain input text")
	}
}

func TestErrorStyle(t *testing.T) {
	t.Parallel()
	result := errorStyle.Render("error text")
	if !strings.Contains(result, "error text") {
		t.Error("errorStyle.Render does not contain input text")
	}
}

// --- RenderAgentCard ---

func newSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return sp
}

func TestRenderAgentCard_StatusIdle(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "claude",
		Status: StatusIdle,
		Color:  lipgloss.Color("#7c3aed"),
	}
	sp := newSpinner()

	result := RenderAgentCard(agent, sp)
	if len(result) == 0 {
		t.Fatal("RenderAgentCard returned empty string for idle agent")
	}
	if !strings.Contains(result, "claude") {
		t.Error("idle card should contain agent name 'claude'")
	}
	// Idle should not have "processing..." or duration
	if strings.Contains(result, "processing...") {
		t.Error("idle card should not contain 'processing...'")
	}
}

func TestRenderAgentCard_StatusWorking(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "gemini",
		Status: StatusWorking,
		Color:  lipgloss.Color("#3b82f6"),
	}
	sp := newSpinner()

	result := RenderAgentCard(agent, sp)
	if len(result) == 0 {
		t.Fatal("RenderAgentCard returned empty string for working agent")
	}
	if !strings.Contains(result, "gemini") {
		t.Error("working card should contain agent name 'gemini'")
	}
	if !strings.Contains(result, "processing...") {
		t.Error("working card should contain 'processing...'")
	}
}

func TestRenderAgentCard_StatusDone_WithDuration(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:     "codex",
		Status:   StatusDone,
		Color:    lipgloss.Color("#10b981"),
		Duration: 1234 * time.Millisecond,
	}
	sp := newSpinner()

	result := RenderAgentCard(agent, sp)
	if len(result) == 0 {
		t.Fatal("RenderAgentCard returned empty string for done agent")
	}
	if !strings.Contains(result, "codex") {
		t.Error("done card should contain agent name 'codex'")
	}
	if !strings.Contains(result, "1234ms") {
		t.Error("done card should contain duration '1234ms'")
	}
}

func TestRenderAgentCard_StatusDone_ZeroDuration(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:     "claude",
		Status:   StatusDone,
		Color:    lipgloss.Color("#7c3aed"),
		Duration: 0,
	}
	sp := newSpinner()

	result := RenderAgentCard(agent, sp)
	if len(result) == 0 {
		t.Fatal("RenderAgentCard returned empty string")
	}
	if !strings.Contains(result, "claude") {
		t.Error("done card with zero duration should contain agent name")
	}
	// Zero duration: the "if agent.Duration > 0" branch is false
	if strings.Contains(result, "ms") {
		t.Error("done card with zero duration should not show milliseconds")
	}
}

func TestRenderAgentCard_StatusError(t *testing.T) {
	t.Parallel()

	agent := &Agent{
		Name:   "gemini",
		Status: StatusError,
		Color:  lipgloss.Color("#3b82f6"),
		Error:  "timeout",
	}
	sp := newSpinner()

	result := RenderAgentCard(agent, sp)
	if len(result) == 0 {
		t.Fatal("RenderAgentCard returned empty string for error agent")
	}
	if !strings.Contains(result, "gemini") {
		t.Error("error card should contain agent name 'gemini'")
	}
	if !strings.Contains(result, "error") {
		t.Error("error card should contain 'error'")
	}
}

func TestRenderAgentCard_AllStatuses_NoPanic(t *testing.T) {
	t.Parallel()

	statuses := []AgentStatus{StatusIdle, StatusWorking, StatusDone, StatusError}
	sp := newSpinner()

	for _, status := range statuses {
		agent := &Agent{
			Name:     "test",
			Status:   status,
			Color:    lipgloss.Color("#ffffff"),
			Duration: 100 * time.Millisecond,
		}
		result := RenderAgentCard(agent, sp)
		if len(result) == 0 {
			t.Errorf("RenderAgentCard returned empty string for status %d", status)
		}
	}
}
