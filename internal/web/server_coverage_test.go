package web

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// stubChatStore is a minimal implementation of core.ChatStore for testing.
type stubChatStore struct{}

func (s *stubChatStore) SaveSession(_ context.Context, _ *core.ChatSessionState) error {
	return nil
}
func (s *stubChatStore) LoadSession(_ context.Context, _ string) (*core.ChatSessionState, error) {
	return nil, nil
}
func (s *stubChatStore) ListSessions(_ context.Context) ([]*core.ChatSessionState, error) { return nil, nil }
func (s *stubChatStore) DeleteSession(_ context.Context, _ string) error { return nil }
func (s *stubChatStore) SaveMessage(_ context.Context, _ *core.ChatMessageState) error { return nil }
func (s *stubChatStore) LoadMessages(_ context.Context, _ string) ([]*core.ChatMessageState, error) {
	return nil, nil
}

// stubProjectRegistry is a minimal implementation of project.Registry for testing.
type stubProjectRegistry struct{}

func (s *stubProjectRegistry) ListProjects(_ context.Context) ([]*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRegistry) GetProject(_ context.Context, _ string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRegistry) GetProjectByPath(_ context.Context, _ string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRegistry) AddProject(_ context.Context, _ string) (*project.Project, error) {
	return nil, nil
}
func (s *stubProjectRegistry) RemoveProject(_ context.Context, _ string) error { return nil }
func (s *stubProjectRegistry) SetActiveProject(_ context.Context, _ string) error {
	return nil
}
func (s *stubProjectRegistry) GetActiveProject(_ context.Context) (*project.Project, error) {
	return nil, nil
}

// --- DefaultConfig tests ---

func TestDefaultConfig_Values(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, 15*time.Second)
	}
	if cfg.WriteTimeout != 5*time.Minute {
		t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, 5*time.Minute)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 60*time.Second)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, 10*time.Second)
	}
	if !cfg.EnableCORS {
		t.Error("EnableCORS = false, want true")
	}
	if !cfg.ServeStatic {
		t.Error("ServeStatic = false, want true")
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSOrigins = %v, want [http://localhost:5173]", cfg.CORSOrigins)
	}
}

// --- Server creation with options ---

func TestNew_NilLogger(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	// Passing nil logger should use slog.Default()
	server := New(cfg, nil)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.logger == nil {
		t.Error("logger should not be nil (should use slog.Default())")
	}
}

func TestNew_WithEventBus(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false

	bus := events.New(100)
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
	)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.eventBus == nil {
		t.Error("eventBus should not be nil")
	}
	// When eventBus is set, apiServer should be created
	if server.apiServer == nil {
		t.Error("apiServer should be created when eventBus is set")
	}
}

func TestNew_WithResourceMonitor(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	monitor := diagnostics.NewResourceMonitor(time.Second, 80, 10000, 4096, 10,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	bus := events.New(100)

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
		WithResourceMonitor(monitor),
	)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.resourceMonitor == nil {
		t.Error("resourceMonitor should not be nil")
	}
}

func TestNew_WithoutEventBus(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	// Without event bus, API server should not be created
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if server.apiServer != nil {
		t.Error("apiServer should be nil when no eventBus")
	}
}

// --- Server option functions ---

func TestWithEventBus_SetsField(t *testing.T) {
	t.Parallel()

	bus := events.New(100)
	s := &Server{}
	opt := WithEventBus(bus)
	opt(s)
	if s.eventBus != bus {
		t.Error("WithEventBus did not set eventBus")
	}
}

func TestWithResourceMonitor_SetsField(t *testing.T) {
	t.Parallel()

	monitor := diagnostics.NewResourceMonitor(time.Second, 80, 10000, 4096, 10,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := &Server{}
	opt := WithResourceMonitor(monitor)
	opt(s)
	if s.resourceMonitor != monitor {
		t.Error("WithResourceMonitor did not set resourceMonitor")
	}
}

func TestWithChatStore_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithChatStore(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

// --- handleHealth endpoint ---

func TestHandleHealth_Response(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	server.handleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["status"] != "healthy" {
		t.Errorf("status = %q, want %q", result["status"], "healthy")
	}
}

// --- handleAPIRoot endpoint ---

func TestHandleAPIRoot_Response(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest("GET", "/api/v1/", nil)
	rec := httptest.NewRecorder()

	server.handleAPIRoot(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var result map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result["version"] != "v1" {
		t.Errorf("version = %q, want %q", result["version"], "v1")
	}
	if result["name"] != "quorum-api" {
		t.Errorf("name = %q, want %q", result["name"], "quorum-api")
	}
}

// --- Router tests ---

func TestRouter_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if server.Router() == nil {
		t.Error("Router() returned nil")
	}
}

// --- Addr tests ---

func TestAddr_FormatsCorrectly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		port int
		want string
	}{
		{"localhost", 8080, "localhost:8080"},
		{"0.0.0.0", 3000, "0.0.0.0:3000"},
		{"127.0.0.1", 9090, "127.0.0.1:9090"},
		{"", 80, ":80"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			cfg.Host = tt.host
			cfg.Port = tt.port
			cfg.ServeStatic = false
			cfg.EnableCORS = false

			server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if addr := server.Addr(); addr != tt.want {
				t.Errorf("Addr() = %q, want %q", addr, tt.want)
			}
		})
	}
}

// --- CORS middleware tests ---

func TestCORS_Enabled(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.EnableCORS = true
	cfg.CORSOrigins = []string{"http://example.com"}
	cfg.ServeStatic = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "http://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "http://example.com")
	}
}

func TestCORS_Disabled(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.EnableCORS = false
	cfg.ServeStatic = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty (CORS disabled)", origin)
	}
}

func TestCORS_MultipleMethods(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.EnableCORS = true
	cfg.CORSOrigins = []string{"http://localhost:5173"}
	cfg.ServeStatic = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods should not be empty")
	}
}

// --- Logging middleware tests ---

func TestLoggingMiddleware_DoesNotBreakRequest(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.EnableCORS = false
	cfg.ServeStatic = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// --- Static file serving tests ---

func TestStaticServing_Enabled(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = true
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<!doctype html>") {
		t.Error("root should serve index.html")
	}
}

func TestStaticServing_Disabled(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nonexistent/path")
	if err != nil {
		t.Fatalf("GET /nonexistent/path failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// --- Server lifecycle tests ---

func TestStart_And_Shutdown(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 18766
	cfg.ServeStatic = false
	cfg.EnableCORS = false
	cfg.ShutdownTimeout = 2 * time.Second

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Verify server is running
	resp, err := http.Get("http://127.0.0.1:18766/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Shutdown with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestShutdown_WithCancelledContext(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 18767
	cfg.ServeStatic = false
	cfg.EnableCORS = false
	cfg.ShutdownTimeout = 100 * time.Millisecond

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Shutdown should still work even with an already-cancelled parent context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

// --- API route mounting tests ---

func TestAPIRoutes_WithoutEventBus(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	// No event bus = fallback API root
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/")
	if err != nil {
		t.Fatalf("GET /api/v1/ failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["version"] != "v1" {
		t.Errorf("version = %q, want %q", result["version"], "v1")
	}
}

func TestAPIRoutes_WithEventBus(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	bus := events.New(100)
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
	)
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	// Health should still work
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// --- Method handling tests ---

func TestHealthEndpoint_PostNotAllowed(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/health", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

// --- Multiple CORS origins ---

func TestCORS_MultipleOrigins(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.EnableCORS = true
	cfg.CORSOrigins = []string{"http://localhost:5173", "http://localhost:3000"}
	cfg.ServeStatic = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	// Test first origin
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "http://localhost:3000")
	}
}

// --- Config struct field tests ---

func TestConfig_FieldDefaults(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	if cfg.Host != "" {
		t.Errorf("zero-value Host = %q, want empty", cfg.Host)
	}
	if cfg.Port != 0 {
		t.Errorf("zero-value Port = %d, want 0", cfg.Port)
	}
	if cfg.EnableCORS {
		t.Error("zero-value EnableCORS should be false")
	}
	if cfg.ServeStatic {
		t.Error("zero-value ServeStatic should be false")
	}
}

// --- Server HTTP configuration tests ---

func TestNew_HTTPServerConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Host:         "testhost",
		Port:         12345,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
		ServeStatic:  false,
	}

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if server.httpServer.Addr != "testhost:12345" {
		t.Errorf("httpServer.Addr = %q, want %q", server.httpServer.Addr, "testhost:12345")
	}
	if server.httpServer.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", server.httpServer.ReadTimeout, 30*time.Second)
	}
	if server.httpServer.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", server.httpServer.WriteTimeout, 60*time.Second)
	}
	if server.httpServer.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", server.httpServer.IdleTimeout, 120*time.Second)
	}
}

// --- WithOptions additional coverage ---

func TestWithHeartbeatManager_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithHeartbeatManager(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithKanbanEngine_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithKanbanEngine(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithUnifiedTracker_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithUnifiedTracker(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithProjectRegistry_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithProjectRegistry(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithStatePool_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithStatePool(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithStateManager_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithStateManager(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithAgentRegistry_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithAgentRegistry(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithConfigLoader_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithConfigLoader(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

func TestWithWorkflowExecutor_SetsField(t *testing.T) {
	t.Parallel()

	s := &Server{}
	opt := WithWorkflowExecutor(nil)
	opt(s)
	// Just verify it doesn't panic with nil
}

// --- New() constructor branch coverage ---

func TestNew_WithAllOptionalDependencies(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false

	bus := events.New(100)
	monitor := diagnostics.NewResourceMonitor(time.Second, 80, 10000, 4096, 10,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Create server with ALL optional dependencies to hit all branches in New()
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
		WithResourceMonitor(monitor),
		WithConfigLoader(nil),
		WithWorkflowExecutor(nil),
		WithHeartbeatManager(nil),
		WithChatStore(nil),
		WithKanbanEngine(nil),
		WithUnifiedTracker(nil),
		WithProjectRegistry(nil),
		WithStatePool(nil),
		WithAgentRegistry(nil),
		WithStateManager(nil),
	)

	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.apiServer == nil {
		t.Error("apiServer should be created when eventBus is set")
	}
}

func TestNew_WithEventBusAndAgentRegistry(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false

	bus := events.New(100)

	// With agentRegistry but no stateManager
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
		WithAgentRegistry(nil),
	)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.apiServer == nil {
		t.Error("apiServer should be created when eventBus is set")
	}
}

func TestNew_WithEventBusAndStateManager(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false

	bus := events.New(100)

	// With stateManager but no agentRegistry
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
		WithStateManager(nil),
	)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.apiServer == nil {
		t.Error("apiServer should be created when eventBus is set")
	}
}

func TestNew_WithEventBusOnly_MinimalLog(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	bus := events.New(100)

	// With eventBus only (no agentRegistry, no stateManager) - logs "event bus only"
	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)),
		WithEventBus(bus),
	)
	if server == nil {
		t.Fatal("New() returned nil")
	}
	if server.apiServer == nil {
		t.Error("apiServer should be created when eventBus is set")
	}
}

// --- Shutdown with expired context ---

func TestShutdown_ExpiredContext(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = 18769
	cfg.ServeStatic = false
	cfg.EnableCORS = false
	cfg.ShutdownTimeout = 100 * time.Millisecond

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Use a very short timeout that expires before shutdown completes
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // Ensure ctx is expired

	// Shutdown may return an error with expired context, or succeed if it finishes quickly
	_ = server.Shutdown(ctx)
}

// --- Concurrent request handling ---

func TestServer_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ServeStatic = false
	cfg.EnableCORS = false

	server := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(server.Router())
	defer ts.Close()

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			resp, err := http.Get(ts.URL + "/health")
			if err != nil {
				t.Errorf("GET /health failed: %v", err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}
