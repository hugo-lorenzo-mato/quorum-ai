package chat

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ---------------------------------------------------------------------------
// Mock AgentRegistry & Agent
// ---------------------------------------------------------------------------

type mockAgent struct {
	name     string
	execFunc func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error)
}

func (a *mockAgent) Name() string                    { return a.name }
func (a *mockAgent) Capabilities() core.Capabilities { return core.Capabilities{} }
func (a *mockAgent) Ping(_ context.Context) error    { return nil }
func (a *mockAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	if a.execFunc != nil {
		return a.execFunc(ctx, opts)
	}
	return &core.ExecuteResult{Output: "mock response"}, nil
}

type mockRegistry struct {
	agents map[string]core.Agent
}

func (r *mockRegistry) Register(name string, agent core.Agent) error {
	r.agents[name] = agent
	return nil
}

func (r *mockRegistry) Get(name string) (core.Agent, error) {
	a, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (r *mockRegistry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

func (r *mockRegistry) ListEnabled() []string                                  { return r.List() }
func (r *mockRegistry) Available(_ context.Context) []string                   { return r.List() }
func (r *mockRegistry) AvailableForPhase(_ context.Context, _ string) []string { return r.List() }
func (r *mockRegistry) ListEnabledForPhase(_ string) []string                  { return r.List() }
func (r *mockRegistry) AvailableForPhaseWithConfig(_ context.Context, _ string, _ map[string][]string) []string {
	return r.List()
}

func newMockRegistry(agentNames ...string) *mockRegistry {
	reg := &mockRegistry{agents: make(map[string]core.Agent)}
	for _, name := range agentNames {
		reg.agents[name] = &mockAgent{name: name}
	}
	return reg
}

// ---------------------------------------------------------------------------
// NewChatSession
// ---------------------------------------------------------------------------

func TestNewChatSession(t *testing.T) {
	reg := newMockRegistry("claude")
	cp := control.New()
	bus := events.New(10)

	s := NewChatSession(reg, cp, bus)
	if s == nil {
		t.Fatal("NewChatSession returned nil")
	}
	if s.GetAgent() != "claude" {
		t.Errorf("expected default agent 'claude', got %q", s.GetAgent())
	}
	if s.GetModel() != "" {
		t.Errorf("expected empty default model, got %q", s.GetModel())
	}
	if s.History() == nil {
		t.Error("history should not be nil")
	}
	if s.History().Len() != 0 {
		t.Error("history should be empty initially")
	}
}

func TestNewChatSession_NilArgs(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	if s == nil {
		t.Fatal("NewChatSession should not return nil even with nil args")
	}
}

// ---------------------------------------------------------------------------
// SetAgent / GetAgent
// ---------------------------------------------------------------------------

func TestChatSession_SetGetAgent(t *testing.T) {
	s := NewChatSession(nil, nil, nil)

	s.SetAgent("gemini")
	if s.GetAgent() != "gemini" {
		t.Errorf("expected 'gemini', got %q", s.GetAgent())
	}

	s.SetAgent("codex")
	if s.GetAgent() != "codex" {
		t.Errorf("expected 'codex', got %q", s.GetAgent())
	}
}

// ---------------------------------------------------------------------------
// SetModel / GetModel
// ---------------------------------------------------------------------------

func TestChatSession_SetGetModel(t *testing.T) {
	s := NewChatSession(nil, nil, nil)

	s.SetModel("gpt-4")
	if s.GetModel() != "gpt-4" {
		t.Errorf("expected 'gpt-4', got %q", s.GetModel())
	}

	s.SetModel("opus")
	if s.GetModel() != "opus" {
		t.Errorf("expected 'opus', got %q", s.GetModel())
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety of Set/Get
// ---------------------------------------------------------------------------

func TestChatSession_ConcurrentSetGet(t *testing.T) {
	s := NewChatSession(nil, nil, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.SetAgent("agent-x")
		}()
		go func() {
			defer wg.Done()
			_ = s.GetAgent()
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// OnMessage callback
// ---------------------------------------------------------------------------

func TestChatSession_OnMessage(t *testing.T) {
	reg := newMockRegistry("claude")
	s := NewChatSession(reg, nil, nil)

	var received []Message
	s.OnMessage(func(m Message) {
		received = append(received, m)
	})

	err := s.SendMessage(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Should have received user message + agent response
	if len(received) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(received))
	}
	if received[0].Role != RoleUser {
		t.Errorf("first message should be user, got %q", received[0].Role)
	}
	if received[1].Role != RoleAgent {
		t.Errorf("second message should be agent, got %q", received[1].Role)
	}
}

// ---------------------------------------------------------------------------
// OnWorkflowUpdate callback
// ---------------------------------------------------------------------------

func TestChatSession_OnWorkflowUpdate(t *testing.T) {
	s := NewChatSession(nil, nil, nil)

	called := false
	s.OnWorkflowUpdate(func(ws *core.WorkflowState) {
		called = true
	})

	// The callback is stored but not triggered here (it's triggered by workflow events)
	if called {
		t.Error("callback should not be called just by setting it")
	}
	if s.onWorkflowUpdate == nil {
		t.Error("onWorkflowUpdate callback should be set")
	}
}

// ---------------------------------------------------------------------------
// SendMessage
// ---------------------------------------------------------------------------

func TestChatSession_SendMessage_Success(t *testing.T) {
	reg := newMockRegistry("claude")
	bus := events.New(10)
	s := NewChatSession(reg, nil, bus)

	err := s.SendMessage(context.Background(), "test message")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Check history
	if s.History().Len() != 2 { // user + agent
		t.Errorf("expected 2 messages in history, got %d", s.History().Len())
	}
}

func TestChatSession_SendMessage_NoRegistry(t *testing.T) {
	s := NewChatSession(nil, nil, nil)

	err := s.SendMessage(context.Background(), "test")
	if err == nil {
		t.Error("expected error when no registry is configured")
	}
}

func TestChatSession_SendMessage_AgentNotFound(t *testing.T) {
	reg := newMockRegistry() // empty registry
	s := NewChatSession(reg, nil, nil)
	s.SetAgent("nonexistent")

	err := s.SendMessage(context.Background(), "test")
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestChatSession_SendMessage_AgentError(t *testing.T) {
	agent := &mockAgent{
		name: "claude",
		execFunc: func(_ context.Context, _ core.ExecuteOptions) (*core.ExecuteResult, error) {
			return nil, fmt.Errorf("agent execution failed")
		},
	}
	reg := &mockRegistry{agents: map[string]core.Agent{"claude": agent}}
	s := NewChatSession(reg, nil, nil)

	var received []Message
	s.OnMessage(func(m Message) {
		received = append(received, m)
	})

	err := s.SendMessage(context.Background(), "test")
	if err == nil {
		t.Error("expected error from agent")
	}

	// Should have user message + error system message
	if len(received) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(received))
	}
	if received[1].Role != RoleSystem {
		t.Errorf("error message should be system, got %q", received[1].Role)
	}
}

func TestChatSession_SendMessage_WithModel(t *testing.T) {
	var capturedModel string
	agent := &mockAgent{
		name: "claude",
		execFunc: func(_ context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			capturedModel = opts.Model
			return &core.ExecuteResult{Output: "ok"}, nil
		},
	}
	reg := &mockRegistry{agents: map[string]core.Agent{"claude": agent}}
	s := NewChatSession(reg, nil, nil)
	s.SetModel("opus")

	err := s.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if capturedModel != "opus" {
		t.Errorf("expected model 'opus', got %q", capturedModel)
	}
}

func TestChatSession_SendMessage_WithEventBus(t *testing.T) {
	reg := newMockRegistry("claude")
	bus := events.New(10)
	s := NewChatSession(reg, nil, bus)

	err := s.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	// Event was published (no way to easily verify without subscribing,
	// but at least it should not panic)
}

func TestChatSession_SendMessage_NoEventBus(t *testing.T) {
	reg := newMockRegistry("claude")
	s := NewChatSession(reg, nil, nil)

	err := s.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage should succeed without event bus: %v", err)
	}
}

func TestChatSession_SendMessage_NoCallback(t *testing.T) {
	reg := newMockRegistry("claude")
	s := NewChatSession(reg, nil, nil)
	// No OnMessage callback set

	err := s.SendMessage(context.Background(), "test")
	if err != nil {
		t.Fatalf("SendMessage should succeed without callback: %v", err)
	}
}

// ---------------------------------------------------------------------------
// History
// ---------------------------------------------------------------------------

func TestChatSession_History(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	h := s.History()
	if h == nil {
		t.Fatal("History() returned nil")
	}
	if h.Len() != 0 {
		t.Error("initial history should be empty")
	}
}

// ---------------------------------------------------------------------------
// CancelWorkflow
// ---------------------------------------------------------------------------

func TestChatSession_CancelWorkflow_NoControlPlane(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	err := s.CancelWorkflow()
	if err == nil {
		t.Error("expected error when no control plane")
	}
}

func TestChatSession_CancelWorkflow_WithControlPlane(t *testing.T) {
	cp := control.New()
	s := NewChatSession(nil, cp, nil)

	err := s.CancelWorkflow()
	if err != nil {
		t.Errorf("CancelWorkflow should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PauseWorkflow / ResumeWorkflow
// ---------------------------------------------------------------------------

func TestChatSession_PauseWorkflow_NoControlPlane(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	s.PauseWorkflow() // should not panic
}

func TestChatSession_PauseWorkflow_WithControlPlane(t *testing.T) {
	cp := control.New()
	s := NewChatSession(nil, cp, nil)
	s.PauseWorkflow() // should not panic
}

func TestChatSession_ResumeWorkflow_NoControlPlane(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	s.ResumeWorkflow() // should not panic
}

func TestChatSession_ResumeWorkflow_WithControlPlane(t *testing.T) {
	cp := control.New()
	s := NewChatSession(nil, cp, nil)
	s.PauseWorkflow()
	s.ResumeWorkflow() // should not panic
}

// ---------------------------------------------------------------------------
// RetryTask
// ---------------------------------------------------------------------------

func TestChatSession_RetryTask_NoControlPlane(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	err := s.RetryTask("task-1")
	if err == nil {
		t.Error("expected error when no control plane")
	}
}

func TestChatSession_RetryTask_WithControlPlane(t *testing.T) {
	cp := control.New()
	s := NewChatSession(nil, cp, nil)

	err := s.RetryTask("task-1")
	if err != nil {
		t.Errorf("RetryTask should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestChatSession_Close(t *testing.T) {
	reg := newMockRegistry("claude")
	cp := control.New()
	bus := events.New(10)
	s := NewChatSession(reg, cp, bus)

	// Close should not panic
	s.Close()
}

func TestChatSession_Close_NilComponents(t *testing.T) {
	s := NewChatSession(nil, nil, nil)
	s.Close() // should not panic
}
