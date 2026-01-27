package api

import (
	"context"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestNewRunnerFactory(t *testing.T) {
	factory := NewRunnerFactory(nil, nil, nil, nil, nil)
	if factory == nil {
		t.Error("NewRunnerFactory returned nil")
	}
}

func TestRunnerFactory_CreateRunner_MissingStateManager(t *testing.T) {
	eventBus := events.New(10)
	defer eventBus.Close()
	factory := NewRunnerFactory(nil, nil, eventBus, nil, nil)

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil)
	if err == nil {
		t.Error("expected error for missing state manager")
	}
	if err.Error() != "state manager not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunnerFactory_CreateRunner_MissingAgentRegistry(t *testing.T) {
	eventBus := events.New(10)
	defer eventBus.Close()
	factory := NewRunnerFactory(
		newMockStateManager(),
		nil, // missing registry
		eventBus,
		nil,
		nil,
	)

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil)
	if err == nil {
		t.Error("expected error for missing agent registry")
	}
	if err.Error() != "agent registry not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunnerFactory_CreateRunner_MissingEventBus(t *testing.T) {
	factory := NewRunnerFactory(
		newMockStateManager(),
		&mockAgentRegistry{},
		nil, // missing event bus
		nil,
		nil,
	)

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil)
	if err == nil {
		t.Error("expected error for missing event bus")
	}
	if err.Error() != "event bus not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunnerFactory_CreateRunner_MissingConfigLoader(t *testing.T) {
	eventBus := events.New(10)
	defer eventBus.Close()
	factory := NewRunnerFactory(
		newMockStateManager(),
		&mockAgentRegistry{},
		eventBus,
		nil, // missing config loader
		nil,
	)

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil)
	if err == nil {
		t.Error("expected error for missing config loader")
	}
	if err.Error() != "config loader not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// mockAgentRegistry for testing
type mockAgentRegistry struct{}

func (m *mockAgentRegistry) Register(_ string, _ core.Agent) error {
	return nil
}

func (m *mockAgentRegistry) Get(_ string) (core.Agent, error) {
	return nil, nil
}

func (m *mockAgentRegistry) List() []string {
	return nil
}

func (m *mockAgentRegistry) ListEnabled() []string {
	return nil
}

func (m *mockAgentRegistry) Available(_ context.Context) []string {
	return nil
}

func (m *mockAgentRegistry) AvailableForPhase(_ context.Context, _ string) []string {
	return nil
}

func TestBuildAgentPhaseModels(t *testing.T) {
	agents := config.AgentsConfig{
		Claude: config.AgentConfig{
			Enabled: true,
			PhaseModels: map[string]string{
				"analyze": "claude-opus",
				"plan":    "claude-sonnet",
			},
		},
		Gemini: config.AgentConfig{
			Enabled: true,
			PhaseModels: map[string]string{
				"execute": "gemini-flash",
			},
		},
		Codex: config.AgentConfig{
			Enabled: false, // Disabled agent should be excluded
			PhaseModels: map[string]string{
				"analyze": "gpt-4",
			},
		},
		Copilot: config.AgentConfig{
			Enabled: true,
			// No PhaseModels - should not appear in result
		},
	}

	result := buildAgentPhaseModels(agents)

	// Verify claude phase models
	if result["claude"]["analyze"] != "claude-opus" {
		t.Errorf("expected claude.analyze=claude-opus, got %s", result["claude"]["analyze"])
	}
	if result["claude"]["plan"] != "claude-sonnet" {
		t.Errorf("expected claude.plan=claude-sonnet, got %s", result["claude"]["plan"])
	}

	// Verify gemini phase models
	if result["gemini"]["execute"] != "gemini-flash" {
		t.Errorf("expected gemini.execute=gemini-flash, got %s", result["gemini"]["execute"])
	}

	// Verify codex is excluded (disabled agent)
	if _, exists := result["codex"]; exists {
		t.Error("expected codex to be excluded (disabled agent)")
	}

	// Verify copilot is excluded (no phase models)
	if _, exists := result["copilot"]; exists {
		t.Error("expected copilot to be excluded (no phase models)")
	}
}

func TestBuildAgentPhaseModels_Empty(t *testing.T) {
	// Test with no agents enabled or no phase models
	agents := config.AgentsConfig{}

	result := buildAgentPhaseModels(agents)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d entries", len(result))
	}
}
