package middleware

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockProjectContext implements ProjectContext for testing.
type mockProjectContext struct {
	id      string
	root    string
	closed  bool
	touched bool
}

func (m *mockProjectContext) ProjectID() string   { return m.id }
func (m *mockProjectContext) ProjectRoot() string { return m.root }
func (m *mockProjectContext) IsClosed() bool      { return m.closed }
func (m *mockProjectContext) Touch()              { m.touched = true }

// mockRegistry implements ProjectRegistry for testing.
type mockRegistry struct {
	defaultProject string
	projects       map[string]bool
}

func (m *mockRegistry) GetDefaultProject() string { return m.defaultProject }
func (m *mockRegistry) Exists(id string) bool     { return m.projects[id] }

// mockPool implements ProjectStatePool for testing.
type mockPool struct {
	contexts map[string]*mockProjectContext
	err      error
}

func (m *mockPool) GetContext(_ context.Context, projectID string) (ProjectContext, error) {
	if m.err != nil {
		return nil, m.err
	}
	if pc, ok := m.contexts[projectID]; ok {
		return pc, nil
	}
	return nil, errors.New("project not found")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestGetProjectContext(t *testing.T) {
	t.Run("returns nil for empty context", func(t *testing.T) {
		ctx := context.Background()
		pc := GetProjectContext(ctx)
		if pc != nil {
			t.Error("expected nil, got context")
		}
	})

	t.Run("returns context when set", func(t *testing.T) {
		mock := &mockProjectContext{id: "proj-123", root: "/test"}
		ctx := WithProjectContext(context.Background(), mock)

		pc := GetProjectContext(ctx)
		if pc == nil {
			t.Fatal("expected context, got nil")
		}
		if pc.ProjectID() != "proj-123" {
			t.Errorf("expected id proj-123, got %s", pc.ProjectID())
		}
	})
}

func TestGetProjectID(t *testing.T) {
	t.Run("returns empty for empty context", func(t *testing.T) {
		ctx := context.Background()
		id := GetProjectID(ctx)
		if id != "" {
			t.Errorf("expected empty string, got %s", id)
		}
	})

	t.Run("returns ID when context is set", func(t *testing.T) {
		mock := &mockProjectContext{id: "proj-456", root: "/test"}
		ctx := WithProjectContext(context.Background(), mock)

		id := GetProjectID(ctx)
		if id != "proj-456" {
			t.Errorf("expected proj-456, got %s", id)
		}
	})
}

func TestWithProjectContext(t *testing.T) {
	t.Run("stores context and ID", func(t *testing.T) {
		mock := &mockProjectContext{id: "proj-789", root: "/path"}
		ctx := WithProjectContext(context.Background(), mock)

		pc := GetProjectContext(ctx)
		id := GetProjectID(ctx)

		if pc == nil {
			t.Fatal("expected context")
		}
		if id != "proj-789" {
			t.Errorf("expected proj-789, got %s", id)
		}
	})

	t.Run("handles nil context", func(t *testing.T) {
		ctx := WithProjectContext(context.Background(), nil)

		pc := GetProjectContext(ctx)
		id := GetProjectID(ctx)

		if pc != nil {
			t.Error("expected nil context")
		}
		if id != "" {
			t.Errorf("expected empty id, got %s", id)
		}
	})
}

func TestProjectContextMiddleware(t *testing.T) {
	logger := testLogger()

	t.Run("returns 400 when projectID missing", func(t *testing.T) {
		pool := &mockPool{contexts: map[string]*mockProjectContext{}}
		mw := ProjectContextMiddleware(pool, logger)

		// Create a route without projectID parameter
		r := chi.NewRouter()
		r.With(mw).Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "projectID is required") {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("returns 404 when project not found", func(t *testing.T) {
		pool := &mockPool{contexts: map[string]*mockProjectContext{}}
		mw := ProjectContextMiddleware(pool, logger)

		r := chi.NewRouter()
		r.Route("/projects/{projectID}", func(r chi.Router) {
			r.With(mw).Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/projects/unknown-id", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 503 when context is closed", func(t *testing.T) {
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-closed": {id: "proj-closed", root: "/path", closed: true},
			},
		}
		mw := ProjectContextMiddleware(pool, logger)

		r := chi.NewRouter()
		r.Route("/projects/{projectID}", func(r chi.Router) {
			r.With(mw).Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/projects/proj-closed", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("passes context to handler on success", func(t *testing.T) {
		mockCtx := &mockProjectContext{id: "proj-ok", root: "/valid/path"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-ok": mockCtx,
			},
		}
		mw := ProjectContextMiddleware(pool, logger)

		var receivedID string
		var receivedRoot string

		r := chi.NewRouter()
		r.Route("/projects/{projectID}", func(r chi.Router) {
			r.With(mw).Get("/", func(w http.ResponseWriter, req *http.Request) {
				pc := GetProjectContext(req.Context())
				if pc != nil {
					receivedID = pc.ProjectID()
					receivedRoot = pc.ProjectRoot()
				}
				w.WriteHeader(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/projects/proj-ok", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if receivedID != "proj-ok" {
			t.Errorf("expected proj-ok, got %s", receivedID)
		}
		if receivedRoot != "/valid/path" {
			t.Errorf("expected /valid/path, got %s", receivedRoot)
		}
		if !mockCtx.touched {
			t.Error("expected Touch() to be called")
		}
	})
}

func TestDefaultProjectMiddleware(t *testing.T) {
	logger := testLogger()

	t.Run("returns 503 when no default project", func(t *testing.T) {
		registry := &mockRegistry{defaultProject: ""}
		pool := &mockPool{contexts: map[string]*mockProjectContext{}}
		mw := DefaultProjectMiddleware(registry, pool, logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/workflows", nil)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "no default project") {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("returns 503 when default project unavailable", func(t *testing.T) {
		registry := &mockRegistry{defaultProject: "proj-default"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{},
			err:      errors.New("context unavailable"),
		}
		mw := DefaultProjectMiddleware(registry, pool, logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/workflows", nil)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("sets deprecation headers", func(t *testing.T) {
		mockCtx := &mockProjectContext{id: "proj-default", root: "/default"}
		registry := &mockRegistry{defaultProject: "proj-default"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-default": mockCtx,
			},
		}
		mw := DefaultProjectMiddleware(registry, pool, logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/workflows", nil)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Header().Get("Deprecation") != "true" {
			t.Error("expected Deprecation header")
		}
		if rec.Header().Get("Sunset") == "" {
			t.Error("expected Sunset header")
		}
		if !strings.Contains(rec.Header().Get("Link"), "/api/v1/projects/proj-default") {
			t.Errorf("expected successor link, got %s", rec.Header().Get("Link"))
		}
	})

	t.Run("uses default project context", func(t *testing.T) {
		mockCtx := &mockProjectContext{id: "proj-default", root: "/default/path"}
		registry := &mockRegistry{defaultProject: "proj-default"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-default": mockCtx,
			},
		}
		mw := DefaultProjectMiddleware(registry, pool, logger)

		var receivedID string
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pc := GetProjectContext(r.Context())
			if pc != nil {
				receivedID = pc.ProjectID()
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/workflows", nil)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if receivedID != "proj-default" {
			t.Errorf("expected proj-default, got %s", receivedID)
		}
		if !mockCtx.touched {
			t.Error("expected Touch() to be called")
		}
	})
}

func TestRequireProjectContext(t *testing.T) {
	logger := testLogger()

	t.Run("returns 500 when no context", func(t *testing.T) {
		mw := RequireProjectContext(logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "project context not initialized") {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("returns 503 when context closed", func(t *testing.T) {
		mw := RequireProjectContext(logger)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ctx := WithProjectContext(context.Background(), &mockProjectContext{
			id:     "proj-closed",
			closed: true,
		})
		req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("passes through when context valid", func(t *testing.T) {
		mw := RequireProjectContext(logger)

		handlerCalled := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		ctx := WithProjectContext(context.Background(), &mockProjectContext{
			id:     "proj-valid",
			closed: false,
		})
		req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if !handlerCalled {
			t.Error("expected handler to be called")
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	logger := testLogger()

	t.Run("ProjectContextMiddleware + RequireProjectContext", func(t *testing.T) {
		mockCtx := &mockProjectContext{id: "proj-chain", root: "/chain"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-chain": mockCtx,
			},
		}

		var receivedID string
		r := chi.NewRouter()
		r.Route("/projects/{projectID}", func(r chi.Router) {
			r.Use(ProjectContextMiddleware(pool, logger))
			r.Use(RequireProjectContext(logger))
			r.Get("/data", func(w http.ResponseWriter, req *http.Request) {
				pc := GetProjectContext(req.Context())
				receivedID = pc.ProjectID()
				w.WriteHeader(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/projects/proj-chain/data", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if receivedID != "proj-chain" {
			t.Errorf("expected proj-chain, got %s", receivedID)
		}
	})

	t.Run("DefaultProjectMiddleware + RequireProjectContext", func(t *testing.T) {
		mockCtx := &mockProjectContext{id: "proj-default", root: "/default"}
		registry := &mockRegistry{defaultProject: "proj-default"}
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-default": mockCtx,
			},
		}

		var receivedID string
		r := chi.NewRouter()
		r.Group(func(r chi.Router) {
			r.Use(DefaultProjectMiddleware(registry, pool, logger))
			r.Use(RequireProjectContext(logger))
			r.Get("/workflows", func(w http.ResponseWriter, req *http.Request) {
				pc := GetProjectContext(req.Context())
				receivedID = pc.ProjectID()
				w.WriteHeader(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/workflows", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if receivedID != "proj-default" {
			t.Errorf("expected proj-default, got %s", receivedID)
		}
	})
}
