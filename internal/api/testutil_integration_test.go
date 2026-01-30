//go:build integration

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// testServer wraps an httptest.Server with convenience methods.
type testServer struct {
	*httptest.Server
	server   *Server
	eventBus *events.EventBus
	t        *testing.T
}

// newIntegrationTestServer creates a server for integration testing.
// It uses a thread-safe mock state manager and real EventBus.
func newIntegrationTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create thread-safe mock state manager for integration tests
	sm := newThreadSafeMockStateManager()

	// Create event bus
	eventBus := events.New(100)
	t.Cleanup(func() {
		eventBus.Close()
	})

	// Create mock agent registry
	registry := &mockAgentRegistryIntegration{
		agents:    make(map[string]*mockAgent),
		available: []string{"test-agent"},
	}
	registry.agents["test-agent"] = &mockAgent{name: "test-agent"}

	// Create server
	server := NewServer(
		sm,
		eventBus,
		WithAgentRegistry(registry),
		WithLogger(slog.Default()),
	)

	// Create test HTTP server
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(func() {
		ts.Close()
	})

	return &testServer{
		Server:   ts,
		server:   server,
		eventBus: eventBus,
		t:        t,
	}
}

// createWorkflow creates a workflow via the API.
func (ts *testServer) createWorkflow(prompt string) (string, error) {
	body := fmt.Sprintf(`{"prompt": %q}`, prompt)
	resp, err := http.Post(
		ts.URL+"/api/v1/workflows",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("missing or invalid id in response")
	}

	return id, nil
}

// runWorkflow starts a workflow via the API.
func (ts *testServer) runWorkflow(id string) (*http.Response, error) {
	req, err := http.NewRequest("POST", ts.URL+"/api/v1/workflows/"+id+"/run", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

// getWorkflow retrieves a workflow via the API.
func (ts *testServer) getWorkflow(id string) (map[string]interface{}, error) {
	resp, err := http.Get(ts.URL + "/api/v1/workflows/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

// sseClient connects to the SSE endpoint and collects events.
type sseClient struct {
	url       string
	events    []sseEvent
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	connected chan struct{}
	errCh     chan error
}

type sseEvent struct {
	Type string
	Data map[string]interface{}
}

func newSSEClient(url string) *sseClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &sseClient{
		url:       url + "/api/v1/sse/events",
		ctx:       ctx,
		cancel:    cancel,
		connected: make(chan struct{}),
		errCh:     make(chan error, 1),
	}
}

func (c *sseClient) connect() {
	go func() {
		req, err := http.NewRequestWithContext(c.ctx, "GET", c.url, nil)
		if err != nil {
			c.errCh <- err
			return
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.errCh <- err
			return
		}
		defer resp.Body.Close()

		close(c.connected) // Signal connected

		scanner := bufio.NewScanner(resp.Body)
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				var payload map[string]interface{}
				json.Unmarshal([]byte(data), &payload)

				c.mu.Lock()
				c.events = append(c.events, sseEvent{
					Type: eventType,
					Data: payload,
				})
				c.mu.Unlock()
			}
		}
	}()
}

func (c *sseClient) waitConnected(timeout time.Duration) error {
	select {
	case <-c.connected:
		return nil
	case err := <-c.errCh:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for SSE connection")
	}
}

func (c *sseClient) close() {
	c.cancel()
}

func (c *sseClient) getEvents() []sseEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]sseEvent, len(c.events))
	copy(result, c.events)
	return result
}

func (c *sseClient) hasEvent(eventType string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.events {
		if e.Type == eventType {
			return true
		}
	}
	return false
}

func (c *sseClient) waitForEvent(eventType string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.hasEvent(eventType) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// threadSafeMockStateManager implements core.StateManager for integration testing.
// It's thread-safe for concurrent access.
type threadSafeMockStateManager struct {
	mu           sync.RWMutex
	workflows    map[core.WorkflowID]*core.WorkflowState
	activeID     core.WorkflowID
	saveErr      error
	loadErr      error
	listErr      error
	lockAcquired bool
}

func newThreadSafeMockStateManager() *threadSafeMockStateManager {
	return &threadSafeMockStateManager{
		workflows: make(map[core.WorkflowID]*core.WorkflowState),
	}
}

func (m *threadSafeMockStateManager) Save(_ context.Context, state *core.WorkflowState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.workflows[state.WorkflowID] = state
	m.activeID = state.WorkflowID
	return nil
}

func (m *threadSafeMockStateManager) Load(_ context.Context) (*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.activeID == "" {
		return nil, nil
	}
	return m.workflows[m.activeID], nil
}

func (m *threadSafeMockStateManager) LoadByID(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.workflows[id], nil
}

func (m *threadSafeMockStateManager) ListWorkflows(_ context.Context) ([]core.WorkflowSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	summaries := make([]core.WorkflowSummary, 0, len(m.workflows))
	for id, wf := range m.workflows {
		summaries = append(summaries, core.WorkflowSummary{
			WorkflowID:   id,
			Status:       wf.Status,
			CurrentPhase: wf.CurrentPhase,
			Prompt:       wf.Prompt,
			CreatedAt:    wf.CreatedAt,
			UpdatedAt:    wf.UpdatedAt,
			IsActive:     id == m.activeID,
		})
	}
	return summaries, nil
}

func (m *threadSafeMockStateManager) GetActiveWorkflowID(_ context.Context) (core.WorkflowID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeID, nil
}

func (m *threadSafeMockStateManager) SetActiveWorkflowID(_ context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeID = id
	return nil
}

func (m *threadSafeMockStateManager) AcquireLock(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockAcquired = true
	return nil
}

func (m *threadSafeMockStateManager) ReleaseLock(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockAcquired = false
	return nil
}

func (m *threadSafeMockStateManager) Exists() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workflows) > 0
}

func (m *threadSafeMockStateManager) Backup(_ context.Context) error {
	return nil
}

func (m *threadSafeMockStateManager) Restore(_ context.Context) (*core.WorkflowState, error) {
	return nil, nil
}

func (m *threadSafeMockStateManager) DeactivateWorkflow(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeID = ""
	return nil
}

func (m *threadSafeMockStateManager) ArchiveWorkflows(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, wf := range m.workflows {
		if wf.Status == core.WorkflowStatusCompleted || wf.Status == core.WorkflowStatusFailed {
			if id != m.activeID {
				delete(m.workflows, id)
				count++
			}
		}
	}
	return count, nil
}

func (m *threadSafeMockStateManager) PurgeAllWorkflows(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := len(m.workflows)
	m.workflows = make(map[core.WorkflowID]*core.WorkflowState)
	m.activeID = ""
	return count, nil
}

func (m *threadSafeMockStateManager) DeleteWorkflow(_ context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workflows[id]; !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	delete(m.workflows, id)
	if m.activeID == id {
		m.activeID = ""
	}
	return nil
}

func (m *threadSafeMockStateManager) UpdateHeartbeat(_ context.Context, id core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if wf, ok := m.workflows[id]; ok {
		now := time.Now().UTC()
		wf.HeartbeatAt = &now
		return nil
	}
	return fmt.Errorf("workflow not found: %s", id)
}

func (m *threadSafeMockStateManager) FindZombieWorkflows(_ context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var zombies []*core.WorkflowState
	cutoff := time.Now().UTC().Add(-staleThreshold)
	for _, wf := range m.workflows {
		if wf.Status == core.WorkflowStatusRunning {
			if wf.HeartbeatAt == nil || wf.HeartbeatAt.Before(cutoff) {
				zombies = append(zombies, wf)
			}
		}
	}
	return zombies, nil
}

func (m *threadSafeMockStateManager) AcquireWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockAcquired = true
	return nil
}

func (m *threadSafeMockStateManager) ReleaseWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lockAcquired = false
	return nil
}

func (m *threadSafeMockStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *threadSafeMockStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *threadSafeMockStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil
}

func (m *threadSafeMockStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	return nil, nil
}

func (m *threadSafeMockStateManager) IsWorkflowRunning(_ context.Context, _ core.WorkflowID) (bool, error) {
	return false, nil
}

func (m *threadSafeMockStateManager) UpdateWorkflowHeartbeat(_ context.Context, id core.WorkflowID) error {
	return m.UpdateHeartbeat(context.Background(), id)
}

// mockAgentRegistryIntegration for integration tests - separate from unit test mock
type mockAgentRegistryIntegration struct {
	agents    map[string]*mockAgent
	available []string
}

func (m *mockAgentRegistryIntegration) Register(_ string, _ core.Agent) error {
	return nil
}

func (m *mockAgentRegistryIntegration) Get(name string) (core.Agent, error) {
	if a, ok := m.agents[name]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

func (m *mockAgentRegistryIntegration) List() []string {
	return m.available
}

func (m *mockAgentRegistryIntegration) ListEnabled() []string {
	return m.available
}

func (m *mockAgentRegistryIntegration) Available(_ context.Context) []string {
	return m.available
}

func (m *mockAgentRegistryIntegration) ListEnabledForPhase(_ string) []string {
	return m.available
}

func (m *mockAgentRegistryIntegration) AvailableForPhase(_ context.Context, _ string) []string {
	return m.available
}

// mockAgent is a fast-completing agent for testing.
// Implements core.Agent interface.
type mockAgent struct {
	name string
}

func (a *mockAgent) Name() string {
	return a.name
}

func (a *mockAgent) Capabilities() core.Capabilities {
	return core.Capabilities{
		SupportsStreaming: false,
		SupportsTools:     false,
		SupportedModels:   []string{"mock-model"},
		DefaultModel:      "mock-model",
	}
}

func (a *mockAgent) Ping(_ context.Context) error {
	return nil
}

func (a *mockAgent) Execute(_ context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	// Simulate quick processing
	time.Sleep(10 * time.Millisecond)
	return &core.ExecuteResult{
		Output:   "Mock response for: " + opts.Prompt,
		Model:    "mock-model",
		Duration: 10 * time.Millisecond,
	}, nil
}
