package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

func TestRunnerFactory_WithHeartbeat(t *testing.T) {
	t.Parallel()
	factory := NewRunnerFactory(nil, nil, nil, nil, nil)
	if factory.heartbeat != nil {
		t.Error("expected nil heartbeat initially")
	}

	hb := &workflow.HeartbeatManager{}
	result := factory.WithHeartbeat(hb)
	if result != factory {
		t.Error("WithHeartbeat should return same factory for chaining")
	}
	if factory.heartbeat != hb {
		t.Error("expected heartbeat to be set")
	}
}

func TestRunnerFactory_Fields(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	ar := &mockAgentRegistry{}
	eb := events.New(10)
	defer eb.Close()

	factory := NewRunnerFactory(sm, ar, eb, nil, nil)
	if factory.stateManager != sm {
		t.Error("stateManager not set")
	}
	if factory.agentRegistry != ar {
		t.Error("agentRegistry not set")
	}
	if factory.eventBus != eb {
		t.Error("eventBus not set")
	}
}

func TestServerRunnerFactory_NilDependencies(t *testing.T) {
	t.Parallel()
	// Server with no stateManager or eventBus.
	s := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}

	factory := s.RunnerFactory()
	if factory != nil {
		t.Error("expected nil factory when dependencies are missing")
	}
}

func TestServerRunnerFactoryForContext_NilDependencies(t *testing.T) {
	t.Parallel()
	s := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}

	factory := s.RunnerFactoryForContext(context.Background())
	if factory != nil {
		t.Error("expected nil factory when dependencies are missing")
	}
}

func TestServerRunnerFactoryForContext_WithDependencies(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()

	s := &Server{
		stateManager:  sm,
		eventBus:      eb,
		agentRegistry: &mockAgentRegistry{},
		logger:        slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}

	factory := s.RunnerFactoryForContext(context.Background())
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	if factory.stateManager != sm {
		t.Error("stateManager not set correctly")
	}
	if factory.eventBus != eb {
		t.Error("eventBus not set correctly")
	}
}

func TestServerRunnerFactoryForContext_WithHeartbeat(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()
	hb := &workflow.HeartbeatManager{}

	s := &Server{
		stateManager:  sm,
		eventBus:      eb,
		agentRegistry: &mockAgentRegistry{},
		logger:        slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		heartbeat:     hb,
	}

	factory := s.RunnerFactoryForContext(context.Background())
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	if factory.heartbeat != hb {
		t.Error("expected heartbeat to be set on factory")
	}
}

func TestServerRunnerFactory_UsesGlobalDeps(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()

	s := &Server{
		stateManager:  sm,
		eventBus:      eb,
		agentRegistry: &mockAgentRegistry{},
		logger:        slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
	}

	factory := s.RunnerFactory()
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}

func TestRunnerFactory_CreateRunner_NilLogger(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()

	// Factory with nil logger: should default to NopLogger.
	factory := NewRunnerFactory(sm, &mockAgentRegistry{}, eb, nil, nil)

	// This will fail because there's no ProjectContext, but it should
	// not panic due to nil logger.
	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil, nil)
	if err == nil {
		t.Error("expected error (missing project context)")
	}
}

// mockAgentRegistryWithAvailable extends mockAgentRegistry with AvailableForPhaseWithConfig.
type mockAgentRegistryWithAvailable struct {
	mockAgentRegistry
}

func (m *mockAgentRegistryWithAvailable) AvailableForPhaseWithConfig(_ context.Context, _ string, _ map[string][]string) []string {
	return nil
}

func TestRunnerFactory_CreateRunner_NilStateManagerInFactory(t *testing.T) {
	t.Parallel()
	eb := events.New(10)
	defer eb.Close()
	// Factory with nil stateManager should return "state manager not configured".
	factory := NewRunnerFactory(nil, &mockAgentRegistry{}, eb, nil, nil)
	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "state manager not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunnerFactory_CreateRunner_NilEventBusInFactory(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	factory := NewRunnerFactory(sm, &mockAgentRegistry{}, nil, nil, nil)
	_, _, err := factory.CreateRunner(context.Background(), "wf-test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "event bus not configured" {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Server option tests ---

func TestServerOptions(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	ar := &mockAgentRegistry{}

	s := NewServer(sm, eb,
		WithLogger(logger),
		WithAgentRegistry(ar),
		WithRoot("/tmp/test"),
	)

	if s.logger != logger {
		t.Error("WithLogger not applied")
	}
	if s.agentRegistry != ar {
		t.Error("WithAgentRegistry not applied")
	}
	if s.root != "/tmp/test" {
		t.Error("WithRoot not applied")
	}
}

func TestWithWorkflowExecutor(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()
	executor := &WorkflowExecutor{}

	s := NewServer(sm, eb, WithWorkflowExecutor(executor))
	if s.executor != executor {
		t.Error("WithWorkflowExecutor not applied")
	}
}

func TestWithHeartbeatManager(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()
	hb := &workflow.HeartbeatManager{}

	s := NewServer(sm, eb, WithHeartbeatManager(hb))
	if s.heartbeat != hb {
		t.Error("WithHeartbeatManager not applied")
	}
}

func TestWithChatStore(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()

	s := NewServer(sm, eb, WithChatStore(nil))
	if s.chatStore != nil {
		t.Error("WithChatStore not applied correctly")
	}
}

func TestWithUnifiedTracker(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(10)
	defer eb.Close()
	tracker := &UnifiedTracker{}

	s := NewServer(sm, eb, WithUnifiedTracker(tracker))
	if s.unifiedTracker != tracker {
		t.Error("WithUnifiedTracker not applied")
	}
}

// --- Project helper tests ---

func TestGetStateManagerFromContext_Fallback(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	result := GetStateManagerFromContext(context.Background(), sm)
	if result != sm {
		t.Error("expected fallback state manager")
	}
}

func TestGetEventBusFromContext_Fallback(t *testing.T) {
	t.Parallel()
	eb := events.New(10)
	defer eb.Close()
	result := GetEventBusFromContext(context.Background(), eb)
	if result != eb {
		t.Error("expected fallback event bus")
	}
}

func TestGetProjectRootFromContext_Empty(t *testing.T) {
	t.Parallel()
	result := GetProjectRootFromContext(context.Background())
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestGetProjectID_Empty(t *testing.T) {
	t.Parallel()
	result := getProjectID(context.Background())
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// --- mockAgentRegistry is already defined in runner_factory_test.go ---
// We use the existing mockAgentRegistry from the same package.

// Verify mockAgentRegistry implements the interface (compile-time check).
var _ core.AgentRegistry = (*mockAgentRegistry)(nil)
