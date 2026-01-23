package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// MockAgent implements Agent for testing.
type MockAgent struct {
	name         string
	capabilities core.Capabilities
	executeFunc  func(context.Context, core.ExecuteOptions) (*core.ExecuteResult, error)
	pingFunc     func(context.Context) error
	calls        []MockCall
	mu           sync.Mutex
}

// MockCall records a call to the mock.
type MockCall struct {
	Method    string
	Args      interface{}
	Timestamp time.Time
}

// NewMockAgent creates a new mock agent.
func NewMockAgent(name string) *MockAgent {
	return &MockAgent{
		name: name,
		capabilities: core.Capabilities{
			SupportsJSON:      true,
			SupportsStreaming: false,
			SupportsImages:    false,
			SupportsTools:     true,
			MaxContextTokens:  100000,
			MaxOutputTokens:   8192,
		},
		calls: make([]MockCall, 0),
	}
}

// Name returns the mock name.
func (m *MockAgent) Name() string {
	return m.name
}

// Capabilities returns mock capabilities.
func (m *MockAgent) Capabilities() core.Capabilities {
	return m.capabilities
}

// Ping mocks availability check.
func (m *MockAgent) Ping(ctx context.Context) error {
	m.recordCall("Ping", nil)
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	return nil
}

// Execute mocks prompt execution.
func (m *MockAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	m.recordCall("Execute", opts)
	if m.executeFunc != nil {
		return m.executeFunc(ctx, opts)
	}

	promptPreview := opts.Prompt
	if len(promptPreview) > 50 {
		promptPreview = promptPreview[:50]
	}

	return &core.ExecuteResult{
		Output:    fmt.Sprintf("Mock response for: %s", promptPreview),
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.001,
		Duration:  time.Millisecond * 100,
	}, nil
}

// WithExecuteFunc sets a custom execute function.
func (m *MockAgent) WithExecuteFunc(fn func(context.Context, core.ExecuteOptions) (*core.ExecuteResult, error)) *MockAgent {
	m.executeFunc = fn
	return m
}

// WithPingFunc sets a custom ping function.
func (m *MockAgent) WithPingFunc(fn func(context.Context) error) *MockAgent {
	m.pingFunc = fn
	return m
}

// WithCapabilities sets capabilities.
func (m *MockAgent) WithCapabilities(caps core.Capabilities) *MockAgent {
	m.capabilities = caps
	return m
}

// WithError configures the mock to return an error.
func (m *MockAgent) WithError(err error) *MockAgent {
	m.executeFunc = func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
		return nil, err
	}
	return m
}

// WithResponse configures a fixed response.
func (m *MockAgent) WithResponse(output string) *MockAgent {
	m.executeFunc = func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
		return &core.ExecuteResult{
			Output:    output,
			TokensIn:  100,
			TokensOut: len(output) / 4,
			Duration:  time.Millisecond * 50,
		}, nil
	}
	return m
}

// Calls returns recorded calls.
func (m *MockAgent) Calls() []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]MockCall{}, m.calls...)
}

// CallCount returns number of calls to a method.
func (m *MockAgent) CallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.Method == method {
			count++
		}
	}
	return count
}

// Reset clears call history.
func (m *MockAgent) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MockCall, 0)
}

func (m *MockAgent) recordCall(method string, args interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MockCall{
		Method:    method,
		Args:      args,
		Timestamp: time.Now(),
	})
}

// MockStateManager implements StateManager for testing.
type MockStateManager struct {
	state    *core.WorkflowState
	locked   bool
	saveFunc func(*core.WorkflowState) error
	mu       sync.Mutex
}

// NewMockStateManager creates a new mock state manager.
func NewMockStateManager() *MockStateManager {
	return &MockStateManager{}
}

// Save mocks state saving.
func (m *MockStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveFunc != nil {
		return m.saveFunc(state)
	}
	m.state = state
	return nil
}

// Load mocks state loading.
func (m *MockStateManager) Load(ctx context.Context) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return nil, nil
	}
	return m.state, nil
}

// AcquireLock mocks lock acquisition.
func (m *MockStateManager) AcquireLock(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locked {
		return core.ErrState("LOCK_ACQUIRE_FAILED", "already locked")
	}
	m.locked = true
	return nil
}

// ReleaseLock mocks lock release.
func (m *MockStateManager) ReleaseLock(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locked = false
	return nil
}

// Exists mocks existence check.
func (m *MockStateManager) Exists() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state != nil
}

// Backup mocks backup creation.
func (m *MockStateManager) Backup(ctx context.Context) error {
	return nil
}

// Restore mocks restore from backup.
func (m *MockStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
	return m.Load(ctx)
}

// LoadByID mocks loading a specific workflow by ID.
func (m *MockStateManager) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && m.state.WorkflowID == id {
		return m.state, nil
	}
	return nil, nil
}

// ListWorkflows mocks listing all workflows.
func (m *MockStateManager) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return nil, nil
	}
	return []core.WorkflowSummary{{
		WorkflowID:   m.state.WorkflowID,
		Status:       m.state.Status,
		CurrentPhase: m.state.CurrentPhase,
		Prompt:       m.state.Prompt,
		CreatedAt:    m.state.CreatedAt,
		UpdatedAt:    m.state.UpdatedAt,
		IsActive:     true,
	}}, nil
}

// GetActiveWorkflowID mocks getting the active workflow ID.
func (m *MockStateManager) GetActiveWorkflowID(ctx context.Context) (core.WorkflowID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return "", nil
	}
	return m.state.WorkflowID, nil
}

// SetActiveWorkflowID mocks setting the active workflow ID.
func (m *MockStateManager) SetActiveWorkflowID(ctx context.Context, id core.WorkflowID) error {
	return nil
}

// SetState sets the mock state.
func (m *MockStateManager) SetState(state *core.WorkflowState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
}

// WithSaveError configures save to return an error.
func (m *MockStateManager) WithSaveError(err error) *MockStateManager {
	m.saveFunc = func(state *core.WorkflowState) error {
		return err
	}
	return m
}

// MockRegistry implements AgentRegistry for testing.
type MockRegistry struct {
	agents map[string]*MockAgent
	mu     sync.RWMutex
}

// NewMockRegistry creates a new mock registry.
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		agents: make(map[string]*MockAgent),
	}
}

// Add adds a mock agent.
func (r *MockRegistry) Add(name string, agent *MockAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = agent
}

// Register adds an agent to the registry.
func (r *MockRegistry) Register(name string, agent core.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mock, ok := agent.(*MockAgent); ok {
		r.agents[name] = mock
	}
	return nil
}

// Get returns an agent.
func (r *MockRegistry) Get(name string) (core.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if agent, ok := r.agents[name]; ok {
		return agent, nil
	}
	return nil, core.ErrNotFound("agent", name)
}

// List returns agent names.
func (r *MockRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Available returns agents that pass Ping.
func (r *MockRegistry) Available(ctx context.Context) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	available := make([]string, 0)
	for name, agent := range r.agents {
		if agent.Ping(ctx) == nil {
			available = append(available, name)
		}
	}
	return available
}

// AvailableForPhase returns agents that pass Ping and are enabled for the given phase.
// In the mock, this just returns all available agents (can be extended for specific tests).
func (r *MockRegistry) AvailableForPhase(ctx context.Context, _ string) []string {
	return r.Available(ctx)
}

// Ensure interfaces are implemented
var _ core.Agent = (*MockAgent)(nil)
var _ core.StateManager = (*MockStateManager)(nil)
var _ core.AgentRegistry = (*MockRegistry)(nil)
