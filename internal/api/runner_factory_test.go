package api

import (
	"context"
	"testing"

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

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil)
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

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil)
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

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil)
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

	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil)
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

func (m *mockAgentRegistry) ListEnabledForPhase(_ string) []string {
	return nil
}

func (m *mockAgentRegistry) AvailableForPhase(_ context.Context, _ string) []string {
	return nil
}

func (m *mockAgentRegistry) AvailableForPhaseWithConfig(_ context.Context, _ string, _ map[string][]string) []string {
	return nil
}

// Note: TestBuildAgentPhaseModels moved to internal/service/workflow package
// since buildAgentPhaseModels is now private to that package.
