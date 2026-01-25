package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// mockAgent implements core.Agent for testing.
type mockAgent struct {
	name   string
	result *core.ExecuteResult
	err    error
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Capabilities() core.Capabilities {
	return core.Capabilities{DefaultModel: "test-model"}
}
func (m *mockAgent) Ping(ctx context.Context) error { return nil }
func (m *mockAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &core.ExecuteResult{
		Output:    "Test response",
		TokensIn:  10,
		TokensOut: 20,
		CostUSD:   0.001,
		Duration:  100 * time.Millisecond,
	}, nil
}

// mockAgentRegistry implements core.AgentRegistry for testing.
type mockAgentRegistry struct {
	agents map[string]core.Agent
}

func newMockAgentRegistry() *mockAgentRegistry {
	return &mockAgentRegistry{
		agents: map[string]core.Agent{
			"claude": &mockAgent{name: "claude"},
			"gemini": &mockAgent{name: "gemini"},
		},
	}
}

func (r *mockAgentRegistry) Get(name string) (core.Agent, error) {
	if agent, ok := r.agents[name]; ok {
		return agent, nil
	}
	return nil, fmt.Errorf("agent %s not found", name)
}

func (r *mockAgentRegistry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

func (r *mockAgentRegistry) Register(name string, agent core.Agent) error { return nil }

func (r *mockAgentRegistry) Available(ctx context.Context) []string {
	return r.List()
}

func (r *mockAgentRegistry) AvailableForPhase(ctx context.Context, phase string) []string {
	return r.List()
}

func setupTestRouter(h *ChatHandler) *chi.Mux {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func TestCreateSession(t *testing.T) {
	registry := newMockAgentRegistry()
	eventBus := events.New(10)
	defer eventBus.Close()

	h := NewChatHandler(registry, eventBus)
	r := setupTestRouter(h)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantAgent  string
	}{
		{
			name:       "create with defaults",
			body:       `{}`,
			wantStatus: http.StatusCreated,
			wantAgent:  "claude",
		},
		{
			name:       "create with agent",
			body:       `{"agent": "gemini"}`,
			wantStatus: http.StatusCreated,
			wantAgent:  "gemini",
		},
		{
			name:       "create with invalid agent",
			body:       `{"agent": "invalid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create with empty body",
			body:       ``,
			wantStatus: http.StatusCreated,
			wantAgent:  "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/chat/sessions", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusCreated {
				var session ChatSession
				if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if session.ID == "" {
					t.Error("session ID should not be empty")
				}
				if session.Agent != tt.wantAgent {
					t.Errorf("got agent %q, want %q", session.Agent, tt.wantAgent)
				}
			}
		})
	}
}

func TestListSessions(t *testing.T) {
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	// Create some sessions
	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1", Agent: "claude"},
		messages: []ChatMessage{
			{ID: "msg-1", Content: "Hello"},
		},
	}
	h.sessions["session-2"] = &chatSessionState{
		session: ChatSession{ID: "session-2", Agent: "gemini"},
	}

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp ListSessionsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("got total %d, want 2", resp.Total)
	}
}

func TestGetSession(t *testing.T) {
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1", Agent: "claude"},
		messages: []ChatMessage{
			{ID: "msg-1", Content: "Hello"},
			{ID: "msg-2", Content: "World"},
		},
	}

	tests := []struct {
		name       string
		sessionID  string
		wantStatus int
	}{
		{
			name:       "existing session",
			sessionID:  "session-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existing session",
			sessionID:  "session-404",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/chat/sessions/"+tt.sessionID, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var session ChatSession
				if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(session.Messages) != 2 {
					t.Errorf("got %d messages, want 2", len(session.Messages))
				}
			}
		})
	}
}

func TestDeleteSession(t *testing.T) {
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	// Delete existing session
	req := httptest.NewRequest(http.MethodDelete, "/chat/sessions/session-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify it's deleted
	if _, exists := h.sessions["session-1"]; exists {
		t.Error("session should have been deleted")
	}

	// Delete non-existing session
	req = httptest.NewRequest(http.MethodDelete, "/chat/sessions/session-404", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetMessages(t *testing.T) {
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
		messages: []ChatMessage{
			{ID: "msg-1", Role: "user", Content: "Hello"},
			{ID: "msg-2", Role: "agent", Content: "Hi there!"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions/session-1/messages", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Messages []ChatMessage `json:"messages"`
		Total    int           `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("got total %d, want 2", resp.Total)
	}
}

func TestSendMessage(t *testing.T) {
	registry := newMockAgentRegistry()
	eventBus := events.New(10)
	defer eventBus.Close()

	h := NewChatHandler(registry, eventBus)
	r := setupTestRouter(h)

	// Create a session first
	h.sessions["session-1"] = &chatSessionState{
		session:  ChatSession{ID: "session-1", Agent: "claude"},
		messages: make([]ChatMessage, 0),
		agent:    "claude",
	}

	tests := []struct {
		name       string
		sessionID  string
		body       string
		wantStatus int
	}{
		{
			name:       "valid message",
			sessionID:  "session-1",
			body:       `{"content": "Hello, how are you?"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty content",
			sessionID:  "session-1",
			body:       `{"content": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existing session",
			sessionID:  "session-404",
			body:       `{"content": "Hello"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid json",
			sessionID:  "session-1",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/chat/sessions/"+tt.sessionID+"/messages", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp SendMessageResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.UserMessage.Content == "" {
					t.Error("user message content should not be empty")
				}
				if resp.AgentMessage.Content == "" {
					t.Error("agent message content should not be empty")
				}
				if resp.AgentMessage.Tokens == nil {
					t.Error("agent message should have token info")
				}
			}
		})
	}
}

func TestSetAgent(t *testing.T) {
	registry := newMockAgentRegistry()
	h := NewChatHandler(registry, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1", Agent: "claude"},
		agent:   "claude",
	}

	tests := []struct {
		name       string
		sessionID  string
		body       string
		wantStatus int
	}{
		{
			name:       "valid agent",
			sessionID:  "session-1",
			body:       `{"agent": "gemini"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid agent",
			sessionID:  "session-1",
			body:       `{"agent": "invalid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty agent",
			sessionID:  "session-1",
			body:       `{"agent": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existing session",
			sessionID:  "session-404",
			body:       `{"agent": "claude"}`,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/chat/sessions/"+tt.sessionID+"/agent", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSetModel(t *testing.T) {
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/model", bytes.NewBufferString(`{"model": "claude-opus-4-5"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	// Verify the model was set
	if h.sessions["session-1"].model != "claude-opus-4-5" {
		t.Errorf("got model %q, want %q", h.sessions["session-1"].model, "claude-opus-4-5")
	}
}

func TestChatHandlerWithNilDependencies(t *testing.T) {
	// Handler should work with nil dependencies for basic operations
	h := NewChatHandler(nil, nil)
	r := setupTestRouter(h)

	// Create session should still work
	req := httptest.NewRequest(http.MethodPost, "/chat/sessions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("got status %d, want %d", w.Code, http.StatusCreated)
	}
}
