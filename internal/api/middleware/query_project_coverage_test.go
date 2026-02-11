package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- QueryProjectMiddleware tests ---

func TestQueryProjectMiddleware_ExplicitProject_Found(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	mockCtx := &mockProjectContext{id: "proj-explicit", root: "/explicit"}
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{
			"proj-explicit": mockCtx,
		},
	}
	registry := &mockRegistry{
		projects: map[string]bool{
			"proj-explicit": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	var receivedID string
	var receivedRoot string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pc := GetProjectContext(r.Context())
		if pc != nil {
			receivedID = pc.ProjectID()
			receivedRoot = pc.ProjectRoot()
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows?project=proj-explicit", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if receivedID != "proj-explicit" {
		t.Errorf("expected proj-explicit, got %s", receivedID)
	}
	if receivedRoot != "/explicit" {
		t.Errorf("expected /explicit, got %s", receivedRoot)
	}
	// Should NOT have deprecation headers (not using default).
	if rec.Header().Get("Deprecation") != "" {
		t.Error("explicit project should not have deprecation header")
	}
	if !mockCtx.touched {
		t.Error("expected Touch() to be called")
	}
}

func TestQueryProjectMiddleware_ExplicitProject_NotFound(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{contexts: map[string]*mockProjectContext{}}
	registry := &mockRegistry{
		projects: map[string]bool{},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows?project=unknown", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "project not found") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestQueryProjectMiddleware_DefaultProject_Found(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	mockCtx := &mockProjectContext{id: "proj-default", root: "/default"}
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{
			"proj-default": mockCtx,
		},
	}
	registry := &mockRegistry{
		defaultProject: "proj-default",
		projects: map[string]bool{
			"proj-default": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	var receivedID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pc := GetProjectContext(r.Context())
		if pc != nil {
			receivedID = pc.ProjectID()
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil) // No ?project=
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if receivedID != "proj-default" {
		t.Errorf("expected proj-default, got %s", receivedID)
	}
	// Should have deprecation headers.
	if rec.Header().Get("Deprecation") != "true" {
		t.Error("expected Deprecation header")
	}
	if rec.Header().Get("Sunset") != "2026-06-01" {
		t.Error("expected Sunset header")
	}
	if rec.Header().Get("X-Project-ID") != "proj-default" {
		t.Errorf("expected X-Project-ID header, got %q", rec.Header().Get("X-Project-ID"))
	}
}

func TestQueryProjectMiddleware_NoProjectAndNoDefault(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{contexts: map[string]*mockProjectContext{}}
	registry := &mockRegistry{
		defaultProject: "", // No default.
		projects:       map[string]bool{},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		// No project context should be set.
		pc := GetProjectContext(r.Context())
		if pc != nil {
			t.Error("expected no project context in legacy mode")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Error("expected handler to be called in legacy mode")
	}
}

func TestQueryProjectMiddleware_PoolError(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{},
		err:      errors.New("pool failure"),
	}
	registry := &mockRegistry{
		projects: map[string]bool{
			"proj-err": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows?project=proj-err", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "project unavailable") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestQueryProjectMiddleware_ClosedContext(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{
			"proj-closed": {id: "proj-closed", root: "/closed", closed: true},
		},
	}
	registry := &mockRegistry{
		projects: map[string]bool{
			"proj-closed": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows?project=proj-closed", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "project context is unavailable") {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestQueryProjectMiddleware_DefaultProject_NotExist(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{contexts: map[string]*mockProjectContext{}}
	registry := &mockRegistry{
		defaultProject: "proj-ghost",
		projects:       map[string]bool{}, // Default project not in registry.
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil) // No explicit project.
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent default project, got %d", rec.Code)
	}
}

func TestQueryProjectMiddleware_DefaultProject_PoolError(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{},
		err:      errors.New("pool error"),
	}
	registry := &mockRegistry{
		defaultProject: "proj-default",
		projects: map[string]bool{
			"proj-default": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestQueryProjectMiddleware_DefaultProject_ClosedContext(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{
			"proj-default": {id: "proj-default", root: "/default", closed: true},
		},
	}
	registry := &mockRegistry{
		defaultProject: "proj-default",
		projects: map[string]bool{
			"proj-default": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- Context propagation ---

func TestQueryProjectMiddleware_ProjectIDInContext(t *testing.T) {
	t.Parallel()
	logger := testLogger()
	mockCtx := &mockProjectContext{id: "proj-ctx-test", root: "/ctx"}
	pool := &mockPool{
		contexts: map[string]*mockProjectContext{
			"proj-ctx-test": mockCtx,
		},
	}
	registry := &mockRegistry{
		projects: map[string]bool{
			"proj-ctx-test": true,
		},
	}

	mw := QueryProjectMiddleware(pool, registry, logger)

	var ctxProjectID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxProjectID = GetProjectID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/v1/workflows?project=proj-ctx-test", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	if ctxProjectID != "proj-ctx-test" {
		t.Errorf("expected 'proj-ctx-test' in context, got %q", ctxProjectID)
	}
}

// --- WithProjectContext round-trip ---

func TestWithProjectContext_RoundTrip(t *testing.T) {
	t.Parallel()
	mock := &mockProjectContext{id: "round-trip", root: "/rt"}
	ctx := WithProjectContext(context.Background(), mock)

	pc := GetProjectContext(ctx)
	if pc == nil {
		t.Fatal("expected non-nil project context")
	}
	if pc.ProjectID() != "round-trip" {
		t.Errorf("expected 'round-trip', got %q", pc.ProjectID())
	}

	id := GetProjectID(ctx)
	if id != "round-trip" {
		t.Errorf("expected 'round-trip', got %q", id)
	}
}
