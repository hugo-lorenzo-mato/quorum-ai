package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// =============================================================================
// Mock ChatStore for testing persistence paths
// =============================================================================

type mockChatStore struct {
	sessions    []*core.ChatSessionState
	messages    map[string][]*core.ChatMessageState
	saveErr     error
	loadErr     error
	listErr     error
	deleteErr   error
	saveCallCnt int
}

func newMockChatStore() *mockChatStore {
	return &mockChatStore{
		messages: make(map[string][]*core.ChatMessageState),
	}
}

func (m *mockChatStore) SaveSession(_ context.Context, s *core.ChatSessionState) error {
	m.saveCallCnt++
	if m.saveErr != nil {
		return m.saveErr
	}
	// Upsert
	for i, existing := range m.sessions {
		if existing.ID == s.ID {
			m.sessions[i] = s
			return nil
		}
	}
	m.sessions = append(m.sessions, s)
	return nil
}

func (m *mockChatStore) LoadSession(_ context.Context, id string) (*core.ChatSessionState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockChatStore) ListSessions(_ context.Context) ([]*core.ChatSessionState, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.sessions, nil
}

func (m *mockChatStore) DeleteSession(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, s := range m.sessions {
		if s.ID == id {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			break
		}
	}
	delete(m.messages, id)
	return nil
}

func (m *mockChatStore) SaveMessage(_ context.Context, msg *core.ChatMessageState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.messages[msg.SessionID] = append(m.messages[msg.SessionID], msg)
	return nil
}

func (m *mockChatStore) LoadMessages(_ context.Context, sessionID string) ([]*core.ChatMessageState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.messages[sessionID], nil
}

// =============================================================================
// Resolver option tests
// =============================================================================

func TestWithChatStoreResolver(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	resolver := func(_ context.Context) core.ChatStore { return store }

	h := NewChatHandler(nil, nil, nil, nil, WithChatStoreResolver(resolver))
	got := h.getChatStore(context.Background())
	if got != store {
		t.Error("getChatStore should return the store from the resolver")
	}
}

func TestWithAttachmentStoreResolver(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	resolver := func(_ context.Context) *attachments.Store { return store }

	h := NewChatHandler(nil, nil, nil, nil, WithAttachmentStoreResolver(resolver))
	got := h.getAttachmentStore(context.Background())
	if got != store {
		t.Error("getAttachmentStore should return the store from the resolver")
	}
}

func TestWithProjectRootResolver(t *testing.T) {
	t.Parallel()
	expectedRoot := "/custom/project/root"
	resolver := func(_ context.Context) string { return expectedRoot }

	h := NewChatHandler(nil, nil, nil, nil, WithProjectRootResolver(resolver))
	got := h.getProjectRoot(context.Background())
	if got != expectedRoot {
		t.Errorf("getProjectRoot = %q, want %q", got, expectedRoot)
	}
}

// =============================================================================
// getChatStore / getAttachmentStore / getProjectRoot fallback tests
// =============================================================================

func TestGetChatStore_FallbackToGlobal(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	h := NewChatHandler(nil, nil, nil, store)
	got := h.getChatStore(context.Background())
	if got != store {
		t.Error("getChatStore should fall back to global store")
	}
}

func TestGetChatStore_ResolverReturnsNilFallsBack(t *testing.T) {
	t.Parallel()
	globalStore := newMockChatStore()
	resolver := func(_ context.Context) core.ChatStore { return nil }

	h := NewChatHandler(nil, nil, nil, globalStore, WithChatStoreResolver(resolver))
	got := h.getChatStore(context.Background())
	if got != globalStore {
		t.Error("getChatStore should fall back to global store when resolver returns nil")
	}
}

func TestGetAttachmentStore_FallbackToGlobal(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	h := NewChatHandler(nil, nil, store, nil)
	got := h.getAttachmentStore(context.Background())
	if got != store {
		t.Error("getAttachmentStore should fall back to global attachment store")
	}
}

func TestGetAttachmentStore_ResolverReturnsNilFallsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	globalStore := attachments.NewStore(dir)
	resolver := func(_ context.Context) *attachments.Store { return nil }

	h := NewChatHandler(nil, nil, globalStore, nil, WithAttachmentStoreResolver(resolver))
	got := h.getAttachmentStore(context.Background())
	if got != globalStore {
		t.Error("getAttachmentStore should fall back when resolver returns nil")
	}
}

func TestGetProjectRoot_FallbackToCwd(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	got := h.getProjectRoot(context.Background())
	cwd, _ := os.Getwd()
	if got != cwd {
		t.Errorf("getProjectRoot = %q, want cwd %q", got, cwd)
	}
}

func TestGetProjectRoot_ResolverReturnsEmptyFallsBack(t *testing.T) {
	t.Parallel()
	resolver := func(_ context.Context) string { return "" }
	h := NewChatHandler(nil, nil, nil, nil, WithProjectRootResolver(resolver))
	got := h.getProjectRoot(context.Background())
	cwd, _ := os.Getwd()
	if got != cwd {
		t.Errorf("getProjectRoot = %q, want cwd %q", got, cwd)
	}
}

// =============================================================================
// CreateSession with persistence
// =============================================================================

func TestCreateSession_PersistsToStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	registry := newMockAgentRegistry()
	eventBus := events.New(10)
	defer eventBus.Close()

	h := NewChatHandler(registry, eventBus, nil, store)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/chat/sessions", bytes.NewBufferString(`{"agent":"claude"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusCreated)
	}

	if store.saveCallCnt == 0 {
		t.Error("expected session to be persisted to store")
	}
	if len(store.sessions) != 1 {
		t.Errorf("expected 1 persisted session, got %d", len(store.sessions))
	}
}

func TestCreateSession_WithModel(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	h := NewChatHandler(registry, nil, nil, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/chat/sessions", bytes.NewBufferString(`{"agent":"claude","model":"opus"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusCreated)
	}

	var session ChatSession
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if session.Model != "opus" {
		t.Errorf("got model %q, want %q", session.Model, "opus")
	}
}

// =============================================================================
// UpdateSession tests
// =============================================================================

func TestUpdateSession_UpdatesTitle(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	h := NewChatHandler(nil, nil, nil, store)
	r := setupTestRouter(h)

	now := time.Now()
	h.sessions["session-1"] = &chatSessionState{
		session:     ChatSession{ID: "session-1", Agent: "claude", CreatedAt: now, UpdatedAt: now},
		messages:    make([]ChatMessage, 0),
		agent:       "claude",
		title:       "",
		projectRoot: "/test",
	}

	title := "My New Title"
	body, _ := json.Marshal(UpdateSessionRequest{Title: &title})
	req := httptest.NewRequest(http.MethodPatch, "/chat/sessions/session-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var session ChatSession
	if err := json.NewDecoder(w.Body).Decode(&session); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if session.Title != title {
		t.Errorf("got title %q, want %q", session.Title, title)
	}

	// Verify persisted
	if len(store.sessions) != 1 {
		t.Errorf("expected 1 persisted session, got %d", len(store.sessions))
	}
}

func TestUpdateSession_InvalidBody(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	req := httptest.NewRequest(http.MethodPatch, "/chat/sessions/session-1", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateSession_NotFound(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	title := "Title"
	body, _ := json.Marshal(UpdateSessionRequest{Title: &title})
	req := httptest.NewRequest(http.MethodPatch, "/chat/sessions/missing", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUpdateSession_NoChanges(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	h := NewChatHandler(nil, nil, nil, store)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	// Empty update (no title field)
	req := httptest.NewRequest(http.MethodPatch, "/chat/sessions/session-1", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	// No persistence call for unchanged data
	if store.saveCallCnt != 0 {
		t.Errorf("expected no save calls when nothing changed, got %d", store.saveCallCnt)
	}
}

// =============================================================================
// DeleteSession with persistence + attachments
// =============================================================================

func TestDeleteSession_PersistsAndCleansUp(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	dir := t.TempDir()
	attStore := attachments.NewStore(dir)

	h := NewChatHandler(nil, nil, attStore, store)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	req := httptest.NewRequest(http.MethodDelete, "/chat/sessions/session-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNoContent)
	}

	// Session should be removed from in-memory map
	if _, exists := h.sessions["session-1"]; exists {
		t.Error("session should be removed from memory")
	}
}

// =============================================================================
// GetMessages tests
// =============================================================================

func TestGetMessages_NotFound(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions/missing/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetMessages_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:  ChatSession{ID: "session-1"},
		messages: []ChatMessage{},
	}

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions/session-1/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var messages []ChatMessage
	if err := json.NewDecoder(w.Body).Decode(&messages); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

// =============================================================================
// SendMessage coverage
// =============================================================================

func TestSendMessage_WithAgentOverride(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	eventBus := events.New(10)
	defer eventBus.Close()
	store := newMockChatStore()

	h := NewChatHandler(registry, eventBus, nil, store)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:  ChatSession{ID: "session-1", Agent: "claude"},
		messages: make([]ChatMessage, 0),
		agent:    "claude",
		model:    "default-model",
	}

	// Override agent and model in the message
	body := `{"content":"Hello","agent":"gemini","model":"custom-model"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/sessions/session-1/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp ChatMessage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Agent != "gemini" {
		t.Errorf("got agent %q, want %q", resp.Agent, "gemini")
	}

	// Verify messages were persisted
	if len(store.messages["session-1"]) < 2 {
		t.Errorf("expected at least 2 persisted messages (user + agent), got %d", len(store.messages["session-1"]))
	}
}

func TestSendMessage_AgentError(t *testing.T) {
	t.Parallel()

	failingAgent := &mockAgent{name: "claude", err: fmt.Errorf("agent failed")}
	registry := &mockAgentRegistry{
		agents: map[string]core.Agent{
			"claude": failingAgent,
		},
	}

	h := NewChatHandler(registry, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:  ChatSession{ID: "session-1", Agent: "claude"},
		messages: make([]ChatMessage, 0),
		agent:    "claude",
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/sessions/session-1/messages", bytes.NewBufferString(`{"content":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}

	// Verify error message was added to session
	h.mu.RLock()
	state := h.sessions["session-1"]
	lastMsg := state.messages[len(state.messages)-1]
	h.mu.RUnlock()
	if lastMsg.Role != "system" {
		t.Errorf("expected system error message, got role %q", lastMsg.Role)
	}
}

func TestSendMessage_NilAgentRegistry(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:  ChatSession{ID: "session-1", Agent: "claude"},
		messages: make([]ChatMessage, 0),
		agent:    "claude",
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/sessions/session-1/messages", bytes.NewBufferString(`{"content":"Hello"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// =============================================================================
// SetAgent coverage
// =============================================================================

func TestSetAgent_InvalidBody(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/agent", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetAgent_PersistsToStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	registry := newMockAgentRegistry()

	h := NewChatHandler(registry, nil, nil, store)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:     ChatSession{ID: "session-1", Agent: "claude", CreatedAt: time.Now()},
		agent:       "claude",
		projectRoot: "/test",
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/agent", bytes.NewBufferString(`{"agent":"gemini"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if store.saveCallCnt == 0 {
		t.Error("expected session update to be persisted")
	}
}

// =============================================================================
// SetModel coverage
// =============================================================================

func TestSetModel_NotFound(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/missing/model", bytes.NewBufferString(`{"model":"opus"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSetModel_InvalidBody(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/model", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSetModel_PersistsToStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	h := NewChatHandler(nil, nil, nil, store)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session:     ChatSession{ID: "session-1", CreatedAt: time.Now()},
		agent:       "claude",
		projectRoot: "/test",
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/model", bytes.NewBufferString(`{"model":"opus"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if store.saveCallCnt == 0 {
		t.Error("expected session update to be persisted")
	}
}

func TestSetModel_EmptyModel(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := setupTestRouter(h)

	h.sessions["session-1"] = &chatSessionState{
		session: ChatSession{ID: "session-1"},
		model:   "old-model",
	}

	req := httptest.NewRequest(http.MethodPut, "/chat/sessions/session-1/model", bytes.NewBufferString(`{"model":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if h.sessions["session-1"].model != "" {
		t.Errorf("model should be cleared, got %q", h.sessions["session-1"].model)
	}
}

// =============================================================================
// parseFileReferences tests
// =============================================================================

func TestParseFileReferences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single reference",
			content: "Please look at @path/to/file.go",
			want:    []string{"path/to/file.go"},
		},
		{
			name:    "multiple references",
			content: "Compare @file1.go with @dir/file2.ts",
			want:    []string{"file1.go", "dir/file2.ts"},
		},
		{
			name:    "duplicate references",
			content: "Check @file.go and @file.go again",
			want:    []string{"file.go"},
		},
		{
			name:    "no references",
			content: "No file references here",
			want:    nil,
		},
		{
			name:    "email should not match",
			content: "Send to user@example.com",
			want:    []string{"example.com"},
		},
		{
			name:    "multiple extensions",
			content: "@main.go @styles.css @README.md",
			want:    []string{"main.go", "styles.css", "README.md"},
		},
		{
			name:    "empty input",
			content: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFileReferences(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseFileReferences(%q) = %v (len %d), want %v (len %d)", tt.content, got, len(got), tt.want, len(tt.want))
				return
			}
			for i, path := range got {
				if path != tt.want[i] {
					t.Errorf("parseFileReferences(%q)[%d] = %q, want %q", tt.content, i, path, tt.want[i])
				}
			}
		})
	}
}

// =============================================================================
// isImageFile tests
// =============================================================================

func TestIsImageFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"photo.png", true},
		{"photo.PNG", true},
		{"photo.jpg", true},
		{"photo.jpeg", true},
		{"photo.gif", true},
		{"photo.bmp", true},
		{"photo.webp", true},
		{"photo.svg", true},
		{"photo.ico", true},
		{"photo.tiff", true},
		{"doc.pdf", false},
		{"code.go", false},
		{"data.json", false},
		{"noext", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isImageFile(tt.path); got != tt.want {
				t.Errorf("isImageFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// =============================================================================
// loadFileContent tests
// =============================================================================

func TestLoadFileContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := NewChatHandler(nil, nil, nil, nil)

	// Create a test file
	testContent := "hello world"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(testContent), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Load relative path
	content, err := h.loadFileContent("test.txt", dir)
	if err != nil {
		t.Fatalf("loadFileContent: %v", err)
	}
	if content != testContent {
		t.Errorf("got %q, want %q", content, testContent)
	}

	// Load absolute path
	absPath := filepath.Join(dir, "test.txt")
	content, err = h.loadFileContent(absPath, dir)
	if err != nil {
		t.Fatalf("loadFileContent absolute: %v", err)
	}
	if content != testContent {
		t.Errorf("got %q, want %q", content, testContent)
	}
}

func TestLoadFileContent_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := NewChatHandler(nil, nil, nil, nil)

	_, err := h.loadFileContent("nonexistent.txt", dir)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFileContent_Directory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	h := NewChatHandler(nil, nil, nil, nil)
	_, err := h.loadFileContent("subdir", dir)
	if err == nil {
		t.Error("expected error for directory path")
	}
}

func TestLoadFileContent_OutsideProject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := NewChatHandler(nil, nil, nil, nil)

	_, err := h.loadFileContent("../../etc/passwd", dir)
	if err == nil {
		t.Error("expected error for path outside project")
	}
}

// =============================================================================
// buildChatSystemPrompt test
// =============================================================================

func TestBuildChatSystemPrompt(t *testing.T) {
	t.Parallel()
	prompt := buildChatSystemPrompt()
	if prompt == "" {
		t.Error("system prompt should not be empty")
	}
}

// =============================================================================
// executeAgent tests
// =============================================================================

func TestExecuteAgent_NoRegistry(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	_, err := h.executeAgent(context.Background(), executeAgentOptions{
		agentName: "claude",
		history:   []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Error("expected error with nil registry")
	}
}

func TestExecuteAgent_UnknownAgent(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	h := NewChatHandler(registry, nil, nil, nil)
	_, err := h.executeAgent(context.Background(), executeAgentOptions{
		agentName: "unknown",
		history:   []ChatMessage{{Role: "user", Content: "hello"}},
	})
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestExecuteAgent_WithLongHistory(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	eventBus := events.New(10)
	defer eventBus.Close()
	h := NewChatHandler(registry, eventBus, nil, nil)

	// Build history longer than 10 messages to trigger truncation
	history := make([]ChatMessage, 15)
	for i := range history {
		role := "user"
		if i%2 == 1 {
			role = "agent"
		}
		history[i] = ChatMessage{Role: role, Content: fmt.Sprintf("message %d", i)}
	}

	msg, err := h.executeAgent(context.Background(), executeAgentOptions{
		agentName: "claude",
		history:   history,
	})
	if err != nil {
		t.Fatalf("executeAgent: %v", err)
	}
	if msg.Content == "" {
		t.Error("response content should not be empty")
	}
	if msg.Agent != "claude" {
		t.Errorf("agent = %q, want %q", msg.Agent, "claude")
	}
}

func TestExecuteAgent_WithFileAttachments(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	dir := t.TempDir()

	// Create test files
	if err := os.WriteFile(filepath.Join(dir, "data.txt"), []byte("test data"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	h := NewChatHandler(registry, nil, nil, nil)
	msg, err := h.executeAgent(context.Background(), executeAgentOptions{
		agentName:   "claude",
		projectRoot: dir,
		attachments: []string{"data.txt"},
		history:     []ChatMessage{{Role: "user", Content: "analyze @data.txt"}},
	})
	if err != nil {
		t.Fatalf("executeAgent: %v", err)
	}
	if msg.Content == "" {
		t.Error("response content should not be empty")
	}
}

func TestExecuteAgent_WithImageAttachment(t *testing.T) {
	t.Parallel()
	registry := newMockAgentRegistry()
	dir := t.TempDir()

	// Create a fake image file
	if err := os.WriteFile(filepath.Join(dir, "photo.png"), []byte("fake png"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	h := NewChatHandler(registry, nil, nil, nil)
	msg, err := h.executeAgent(context.Background(), executeAgentOptions{
		agentName:   "claude",
		projectRoot: dir,
		attachments: []string{"photo.png"},
		history:     []ChatMessage{{Role: "user", Content: "describe this image"}},
	})
	if err != nil {
		t.Fatalf("executeAgent: %v", err)
	}
	if msg.Content == "" {
		t.Error("response content should not be empty")
	}
}

// =============================================================================
// ensureSessionLoaded tests
// =============================================================================

func TestEnsureSessionLoaded_AlreadyInMemory(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	h.sessions["s1"] = &chatSessionState{session: ChatSession{ID: "s1"}}

	ok := h.ensureSessionLoaded(context.Background(), "s1")
	if !ok {
		t.Error("expected true for session already in memory")
	}
}

func TestEnsureSessionLoaded_LoadFromStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()

	now := time.Now()
	store.sessions = append(store.sessions, &core.ChatSessionState{
		ID:          "s2",
		Agent:       "claude",
		CreatedAt:   now,
		UpdatedAt:   now,
		ProjectRoot: "/test",
	})
	store.messages["s2"] = []*core.ChatMessageState{
		{ID: "m1", SessionID: "s2", Role: "user", Content: "hello", Timestamp: now},
	}

	h := NewChatHandler(nil, nil, nil, store)
	ok := h.ensureSessionLoaded(context.Background(), "s2")
	if !ok {
		t.Error("expected true after loading from store")
	}

	h.mu.RLock()
	state, exists := h.sessions["s2"]
	h.mu.RUnlock()
	if !exists {
		t.Fatal("session should exist in memory after loading")
	}
	if len(state.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(state.messages))
	}
}

func TestEnsureSessionLoaded_NotInStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	h := NewChatHandler(nil, nil, nil, store)

	ok := h.ensureSessionLoaded(context.Background(), "nonexistent")
	if ok {
		t.Error("expected false for session not in store")
	}
}

func TestEnsureSessionLoaded_NilStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)

	ok := h.ensureSessionLoaded(context.Background(), "nonexistent")
	if ok {
		t.Error("expected false with nil store")
	}
}

func TestEnsureSessionLoaded_StoreLoadError(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	store.loadErr = fmt.Errorf("load error")

	h := NewChatHandler(nil, nil, nil, store)
	ok := h.ensureSessionLoaded(context.Background(), "s1")
	if ok {
		t.Error("expected false when store.LoadSession errors")
	}
}

func TestEnsureSessionLoaded_MessageLoadError(t *testing.T) {
	t.Parallel()
	store := &messageLoadErrorStore{
		mockChatStore: newMockChatStore(),
	}
	now := time.Now()
	store.sessions = append(store.sessions, &core.ChatSessionState{
		ID:        "s1",
		Agent:     "claude",
		CreatedAt: now,
		UpdatedAt: now,
	})

	h := NewChatHandler(nil, nil, nil, store)
	ok := h.ensureSessionLoaded(context.Background(), "s1")
	if !ok {
		t.Error("expected true even when messages fail to load")
	}
}

// messageLoadErrorStore is a mock that fails on LoadMessages but not on LoadSession.
type messageLoadErrorStore struct {
	*mockChatStore
}

func (m *messageLoadErrorStore) LoadMessages(_ context.Context, _ string) ([]*core.ChatMessageState, error) {
	return nil, fmt.Errorf("message load error")
}

// =============================================================================
// ensureProjectSessionsLoaded tests
// =============================================================================

func TestEnsureProjectSessionsLoaded(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	now := time.Now()
	store.sessions = append(store.sessions, &core.ChatSessionState{
		ID:          "s1",
		Agent:       "claude",
		CreatedAt:   now,
		UpdatedAt:   now,
		ProjectRoot: "/project-a",
	})

	h := NewChatHandler(nil, nil, nil, nil,
		WithChatStoreResolver(func(_ context.Context) core.ChatStore { return store }),
		WithProjectRootResolver(func(_ context.Context) string { return "/project-a" }),
	)

	h.ensureProjectSessionsLoaded(context.Background(), "/project-a")

	h.mu.RLock()
	_, exists := h.sessions["s1"]
	h.mu.RUnlock()
	if !exists {
		t.Error("session should be loaded from store")
	}

	// Second call should be a no-op (already loaded)
	h.ensureProjectSessionsLoaded(context.Background(), "/project-a")
}

func TestEnsureProjectSessionsLoaded_NilStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	// Should not panic
	h.ensureProjectSessionsLoaded(context.Background(), "/some/path")
}

// =============================================================================
// loadPersistedSessions tests
// =============================================================================

func TestLoadPersistedSessions_ListError(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	store.listErr = fmt.Errorf("list error")

	h := &ChatHandler{
		sessions:           make(map[string]*chatSessionState),
		loadedProjectRoots: make(map[string]bool),
		chatStore:          store,
	}
	h.loadPersistedSessions()

	if len(h.sessions) != 0 {
		t.Error("sessions should be empty on list error")
	}
}

func TestLoadPersistedSessions_Success(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	now := time.Now()
	store.sessions = append(store.sessions, &core.ChatSessionState{
		ID:          "s1",
		Agent:       "claude",
		CreatedAt:   now,
		UpdatedAt:   now,
		ProjectRoot: "/root",
	})
	store.messages["s1"] = []*core.ChatMessageState{
		{ID: "m1", SessionID: "s1", Role: "user", Content: "hi", Timestamp: now},
	}

	h := NewChatHandler(nil, nil, nil, store)

	if len(h.sessions) != 1 {
		t.Errorf("expected 1 session loaded, got %d", len(h.sessions))
	}
	state := h.sessions["s1"]
	if len(state.messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(state.messages))
	}
	if state.projectRoot != "/root" {
		t.Errorf("projectRoot = %q, want %q", state.projectRoot, "/root")
	}
}

// =============================================================================
// loadPersistedSessionsFromStore tests
// =============================================================================

func TestLoadPersistedSessionsFromStore_ProjectIsolation(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	now := time.Now()

	// Session belonging to project A
	store.sessions = append(store.sessions,
		&core.ChatSessionState{ID: "s1", Agent: "claude", CreatedAt: now, UpdatedAt: now, ProjectRoot: "/project-a"},
		&core.ChatSessionState{ID: "s2", Agent: "gemini", CreatedAt: now, UpdatedAt: now, ProjectRoot: "/project-b"},
		&core.ChatSessionState{ID: "s3", Agent: "claude", CreatedAt: now, UpdatedAt: now, ProjectRoot: ""}, // empty = belongs to any
	)

	h := &ChatHandler{
		sessions:           make(map[string]*chatSessionState),
		loadedProjectRoots: make(map[string]bool),
	}

	h.loadPersistedSessionsFromStore(context.Background(), store, "/project-a")

	if _, ok := h.sessions["s1"]; !ok {
		t.Error("s1 (project-a) should be loaded")
	}
	if _, ok := h.sessions["s2"]; ok {
		t.Error("s2 (project-b) should NOT be loaded")
	}
	if _, ok := h.sessions["s3"]; !ok {
		t.Error("s3 (empty project) should be loaded (belongs to current project)")
	}
}

func TestLoadPersistedSessionsFromStore_PreferNewerData(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	now := time.Now()
	later := now.Add(time.Hour)

	store.sessions = append(store.sessions,
		&core.ChatSessionState{ID: "s1", Agent: "claude", CreatedAt: now, UpdatedAt: later, ProjectRoot: "/root"},
	)
	store.messages["s1"] = []*core.ChatMessageState{
		{ID: "m1", SessionID: "s1", Role: "user", Content: "hi", Timestamp: now},
		{ID: "m2", SessionID: "s1", Role: "agent", Content: "hello", Timestamp: later},
	}

	h := &ChatHandler{
		sessions:           make(map[string]*chatSessionState),
		loadedProjectRoots: make(map[string]bool),
	}

	// Pre-populate with an older version
	h.sessions["s1"] = &chatSessionState{
		session:  ChatSession{ID: "s1", UpdatedAt: now},
		messages: []ChatMessage{{ID: "m1"}},
	}

	h.loadPersistedSessionsFromStore(context.Background(), store, "/root")

	state := h.sessions["s1"]
	if len(state.messages) != 2 {
		t.Errorf("expected 2 messages (newer version), got %d", len(state.messages))
	}
}

// =============================================================================
// ListSessions with project isolation
// =============================================================================

func TestListSessions_ProjectIsolation(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil,
		WithProjectRootResolver(func(_ context.Context) string { return "/project-a" }),
	)

	h.sessions["s1"] = &chatSessionState{
		session:     ChatSession{ID: "s1", Agent: "claude"},
		projectRoot: "/project-a",
	}
	h.sessions["s2"] = &chatSessionState{
		session:     ChatSession{ID: "s2", Agent: "gemini"},
		projectRoot: "/project-b",
	}

	r := setupTestRouter(h)
	req := httptest.NewRequest(http.MethodGet, "/chat/sessions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var sessions []ChatSession
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (project-a only), got %d", len(sessions))
	}
}

// =============================================================================
// Attachment handler tests
// =============================================================================

func TestListAttachments_NoStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions/s1/attachments", nil)
	h.ListAttachments(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestListAttachments_SessionNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	h := NewChatHandler(nil, nil, store, nil)

	r := chi.NewRouter()
	r.Get("/chat/sessions/{sessionID}/attachments", h.ListAttachments)

	req := httptest.NewRequest(http.MethodGet, "/chat/sessions/nonexistent/attachments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestUploadAttachments_NoStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sessions/s1/attachments", nil)
	h.UploadAttachments(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestUploadAttachments_SessionNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	h := NewChatHandler(nil, nil, store, nil)

	r := chi.NewRouter()
	r.Post("/chat/sessions/{sessionID}/attachments", h.UploadAttachments)

	req := httptest.NewRequest(http.MethodPost, "/chat/sessions/nonexistent/attachments", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDownloadAttachment_NoStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions/s1/attachments/a1", nil)
	h.DownloadAttachment(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestDownloadAttachment_MissingIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	h := NewChatHandler(nil, nil, store, nil)

	// No chi context for IDs - they will be empty
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions//attachments/", nil)
	h.DownloadAttachment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteAttachment_NoStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/sessions/s1/attachments/a1", nil)
	h.DeleteAttachment(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestDeleteAttachment_MissingIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store := attachments.NewStore(dir)
	h := NewChatHandler(nil, nil, store, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/chat/sessions//attachments/", nil)
	h.DeleteAttachment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// =============================================================================
// writeJSON / writeError coverage
// =============================================================================

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("got %q, want %q", result["key"], "value")
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["error"] != "something went wrong" {
		t.Errorf("got %q, want %q", result["error"], "something went wrong")
	}
}

// =============================================================================
// RegisterRoutes test
// =============================================================================

func TestRegisterRoutes(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	// Verify routes are registered by making requests
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/chat/sessions"},
		{http.MethodGet, "/chat/sessions"},
		{http.MethodGet, "/chat/sessions/test-id"},
		{http.MethodPatch, "/chat/sessions/test-id"},
		{http.MethodDelete, "/chat/sessions/test-id"},
		{http.MethodGet, "/chat/sessions/test-id/messages"},
		{http.MethodPost, "/chat/sessions/test-id/messages"},
		{http.MethodPut, "/chat/sessions/test-id/agent"},
		{http.MethodPut, "/chat/sessions/test-id/model"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			// Should not be 405 Method Not Allowed (route exists)
			if w.Code == http.StatusMethodNotAllowed {
				t.Errorf("route %s %s returned 405, expected route to exist", route.method, route.path)
			}
		})
	}
}

// =============================================================================
// NewChatHandler with persistent store
// =============================================================================

func TestNewChatHandler_LoadsFromGlobalStore(t *testing.T) {
	t.Parallel()
	store := newMockChatStore()
	now := time.Now()
	store.sessions = append(store.sessions, &core.ChatSessionState{
		ID:        "s1",
		Agent:     "claude",
		CreatedAt: now,
		UpdatedAt: now,
	})
	store.messages["s1"] = []*core.ChatMessageState{
		{ID: "m1", SessionID: "s1", Role: "user", Content: "hello", Timestamp: now},
	}

	h := NewChatHandler(nil, nil, nil, store)
	if len(h.sessions) != 1 {
		t.Errorf("expected 1 session loaded from store, got %d", len(h.sessions))
	}
}

func TestNewChatHandler_NilStore(t *testing.T) {
	t.Parallel()
	h := NewChatHandler(nil, nil, nil, nil)
	if h == nil {
		t.Error("handler should not be nil")
		return
	}
	if len(h.sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(h.sessions))
	}
}
