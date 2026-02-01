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
	mu       sync.RWMutex
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

// DeactivateWorkflow mocks deactivating the workflow.
func (m *MockStateManager) DeactivateWorkflow(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Clear active state but keep the workflow data
	return nil
}

// ArchiveWorkflows mocks archiving workflows.
func (m *MockStateManager) ArchiveWorkflows(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && (m.state.Status == core.WorkflowStatusCompleted || m.state.Status == core.WorkflowStatusFailed) {
		m.state = nil
		return 1, nil
	}
	return 0, nil
}

// PurgeAllWorkflows mocks purging all workflows.
func (m *MockStateManager) PurgeAllWorkflows(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	if m.state != nil {
		count = 1
	}
	m.state = nil
	return count, nil
}

// DeleteWorkflow mocks deleting a single workflow.
func (m *MockStateManager) DeleteWorkflow(ctx context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && m.state.WorkflowID == id {
		m.state = nil
		return nil
	}
	return core.ErrNotFound("workflow", string(id))
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

// UpdateHeartbeat updates the heartbeat timestamp for a running workflow.
func (m *MockStateManager) UpdateHeartbeat(_ context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && m.state.WorkflowID == id {
		now := time.Now().UTC()
		m.state.HeartbeatAt = &now
		return nil
	}
	return core.ErrNotFound("workflow", string(id))
}

// FindZombieWorkflows returns workflows with stale heartbeats.
func (m *MockStateManager) FindZombieWorkflows(_ context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state == nil || m.state.Status != core.WorkflowStatusRunning {
		return nil, nil
	}

	cutoff := time.Now().UTC().Add(-staleThreshold)
	if m.state.HeartbeatAt == nil || m.state.HeartbeatAt.Before(cutoff) {
		return []*core.WorkflowState{m.state}, nil
	}
	return nil, nil
}

// AcquireWorkflowLock mocks workflow-specific lock acquisition.
func (m *MockStateManager) AcquireWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.locked {
		return core.ErrState("LOCK_ACQUIRE_FAILED", "already locked")
	}
	m.locked = true
	return nil
}

// ReleaseWorkflowLock mocks workflow-specific lock release.
func (m *MockStateManager) ReleaseWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locked = false
	return nil
}

// RefreshWorkflowLock mocks lock refresh.
func (m *MockStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// SetWorkflowRunning mocks marking a workflow as running.
func (m *MockStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// ClearWorkflowRunning mocks clearing the running state.
func (m *MockStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// ListRunningWorkflows mocks listing running workflows.
func (m *MockStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state != nil && m.state.Status == core.WorkflowStatusRunning {
		return []core.WorkflowID{m.state.WorkflowID}, nil
	}
	return nil, nil
}

// IsWorkflowRunning mocks checking if a workflow is running.
func (m *MockStateManager) IsWorkflowRunning(_ context.Context, id core.WorkflowID) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state != nil && m.state.WorkflowID == id && m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
}

// UpdateWorkflowHeartbeat mocks workflow heartbeat update.
func (m *MockStateManager) UpdateWorkflowHeartbeat(_ context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != nil && m.state.WorkflowID == id {
		now := time.Now().UTC()
		m.state.HeartbeatAt = &now
		return nil
	}
	return core.ErrNotFound("workflow", string(id))
}

// ExecuteAtomically mocks atomic execution.
func (m *MockStateManager) ExecuteAtomically(_ context.Context, fn func(core.AtomicStateContext) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomicCtx := &mockAtomicContext{m: m}
	return fn(atomicCtx)
}

// mockAtomicContext provides a mock AtomicStateContext for tests.
type mockAtomicContext struct {
	m *MockStateManager
}

func (a *mockAtomicContext) LoadByID(id core.WorkflowID) (*core.WorkflowState, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id {
		return a.m.state, nil
	}
	return nil, nil
}

func (a *mockAtomicContext) Save(state *core.WorkflowState) error {
	a.m.state = state
	return nil
}

func (a *mockAtomicContext) SetWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicContext) ClearWorkflowRunning(_ core.WorkflowID) error {
	return nil
}

func (a *mockAtomicContext) IsWorkflowRunning(id core.WorkflowID) (bool, error) {
	if a.m.state != nil && a.m.state.WorkflowID == id && a.m.state.Status == core.WorkflowStatusRunning {
		return true, nil
	}
	return false, nil
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

// ListEnabled returns names of configured and enabled agents.
// In the mock, this returns the same as List (all registered agents are considered enabled).
func (r *MockRegistry) ListEnabled() []string {
	return r.List()
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

// ListEnabledForPhase returns agent names that are configured and enabled for the given phase.
// In the mock, this just returns all agent names (can be extended for specific tests).
func (r *MockRegistry) ListEnabledForPhase(_ string) []string {
	return r.List()
}

// Ensure interfaces are implemented
var _ core.Agent = (*MockAgent)(nil)
var _ core.StateManager = (*MockStateManager)(nil)
var _ core.AgentRegistry = (*MockRegistry)(nil)
