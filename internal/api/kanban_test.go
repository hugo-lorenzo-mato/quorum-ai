package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// ---------------------------------------------------------------------------
// Mock types for Kanban tests
// ---------------------------------------------------------------------------

// mockKanbanStateManager implements core.StateManager + KanbanStateManagerAdapter
type mockKanbanStateManager struct {
	mockStateManager
	board             map[string][]*core.WorkflowState
	boardErr          error
	moveWorkflowErr   error
	movedWorkflowID   string
	movedToColumn     string
	movedPosition     int
	kanbanEngineState *kanban.KanbanEngineState
}

func newMockKanbanStateManager() *mockKanbanStateManager {
	return &mockKanbanStateManager{
		mockStateManager: mockStateManager{
			workflows: make(map[core.WorkflowID]*core.WorkflowState),
		},
		board: map[string][]*core.WorkflowState{
			"refinement":  {},
			"todo":        {},
			"in_progress": {},
			"to_verify":   {},
			"done":        {},
		},
		kanbanEngineState: &kanban.KanbanEngineState{},
	}
}

func (m *mockKanbanStateManager) GetKanbanBoard(_ context.Context) (map[string][]*core.WorkflowState, error) {
	if m.boardErr != nil {
		return nil, m.boardErr
	}
	return m.board, nil
}

func (m *mockKanbanStateManager) MoveWorkflow(_ context.Context, workflowID, toColumn string, position int) error {
	if m.moveWorkflowErr != nil {
		return m.moveWorkflowErr
	}
	m.movedWorkflowID = workflowID
	m.movedToColumn = toColumn
	m.movedPosition = position
	// Update the workflow in the map.
	if wf, ok := m.workflows[core.WorkflowID(workflowID)]; ok {
		wf.KanbanColumn = toColumn
		wf.KanbanPosition = position
	}
	return nil
}

func (m *mockKanbanStateManager) UpdateKanbanStatus(_ context.Context, _, _, _ string, _ int, _ string) error {
	return nil
}

func (m *mockKanbanStateManager) GetKanbanEngineState(_ context.Context) (*kanban.KanbanEngineState, error) {
	return m.kanbanEngineState, nil
}

func (m *mockKanbanStateManager) SaveKanbanEngineState(_ context.Context, state *kanban.KanbanEngineState) error {
	m.kanbanEngineState = state
	return nil
}

func (m *mockKanbanStateManager) ListWorkflowsByKanbanColumn(_ context.Context, column string) ([]*core.WorkflowState, error) {
	return m.board[column], nil
}

func (m *mockKanbanStateManager) GetNextKanbanWorkflow(_ context.Context) (*core.WorkflowState, error) {
	if wfs := m.board["todo"]; len(wfs) > 0 {
		return wfs[0], nil
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helper to build Kanban test infrastructure
// ---------------------------------------------------------------------------

func setupKanbanTestServer(t *testing.T, sm *mockKanbanStateManager, engine *kanban.Engine) (*chi.Mux, *KanbanServer) {
	t.Helper()

	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	ks := NewKanbanServer(srv, engine, eb)

	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	return r, ks
}

// ---------------------------------------------------------------------------
// handleGetBoard
// ---------------------------------------------------------------------------

func TestHandleGetBoard_Success(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "Build feature",
			Title:      "Feature 1",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPending,
			UpdatedAt: time.Now(),
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1"},
				"t2": {ID: "t2"},
			},
			KanbanColumn: "todo",
		},
	}
	sm.workflows["wf-1"] = wf
	sm.board["todo"] = []*core.WorkflowState{wf}

	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodGet, "/kanban/board", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp KanbanBoardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Columns["todo"]) != 1 {
		t.Errorf("expected 1 workflow in todo, got %d", len(resp.Columns["todo"]))
	}
	if resp.Columns["todo"][0].Title != "Feature 1" {
		t.Errorf("expected 'Feature 1', got %q", resp.Columns["todo"][0].Title)
	}
	if resp.Columns["todo"][0].TaskCount != 2 {
		t.Errorf("expected task count 2, got %d", resp.Columns["todo"][0].TaskCount)
	}
}

func TestHandleGetBoard_WithEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()

	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	engine := kanban.NewEngine(kanban.EngineConfig{
		StateManager: sm,
		EventBus:     eb,
		Logger:       slog.Default(),
	})

	r, _ := setupKanbanTestServer(t, sm, engine)

	req := httptest.NewRequest(http.MethodGet, "/kanban/board", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp KanbanBoardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Engine state should be present.
	// Engine is disabled by default, so we expect Enabled to be false
	if resp.Engine.Enabled {
		t.Error("expected engine to be disabled by default")
	}
}

func TestHandleGetBoard_BoardError(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	sm.boardErr = fmt.Errorf("db error")

	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodGet, "/kanban/board", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleGetBoard_NonKanbanStateManager(t *testing.T) {
	t.Parallel()
	// Use a plain mockStateManager that does NOT implement KanbanStateManagerAdapter.
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithRoot(t.TempDir()))
	ks := NewKanbanServer(srv, nil, eb)
	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/kanban/board", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// handleMoveWorkflow
// ---------------------------------------------------------------------------

func TestHandleMoveWorkflow_Success(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "Do something",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			UpdatedAt:    time.Now(),
			Tasks:        make(map[core.TaskID]*core.TaskState),
			KanbanColumn: "refinement",
		},
	}
	sm.workflows["wf-1"] = wf

	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))
	ks := NewKanbanServer(srv, nil, eb)
	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	body := `{"to_column": "todo", "position": 1}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp KanbanWorkflowResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.KanbanColumn != "todo" {
		t.Errorf("expected 'todo', got %q", resp.KanbanColumn)
	}
}

func TestHandleMoveWorkflow_InvalidJSON(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleMoveWorkflow_InvalidColumn(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	body := `{"to_column": "invalid_column", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleMoveWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	body := `{"to_column": "todo", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/nonexistent/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleMoveWorkflow_MoveError(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	sm.moveWorkflowErr = fmt.Errorf("db error")
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			UpdatedAt:    time.Now(),
			Tasks:        make(map[core.TaskID]*core.TaskState),
			KanbanColumn: "refinement",
		},
	}
	sm.workflows["wf-1"] = wf

	r, _ := setupKanbanTestServer(t, sm, nil)

	body := `{"to_column": "todo", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleMoveWorkflow_NonKanbanStateManager(t *testing.T) {
	t.Parallel()
	// Plain mockStateManager - does not implement Kanban interface.
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))
	ks := NewKanbanServer(srv, nil, eb)
	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	body := `{"to_column": "todo", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// handleGetEngineState
// ---------------------------------------------------------------------------

func TestHandleGetEngineState_NoEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodGet, "/kanban/engine", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleGetEngineState_WithEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	engine := kanban.NewEngine(kanban.EngineConfig{
		StateManager: sm,
		EventBus:     eb,
		Logger:       slog.Default(),
	})

	r, _ := setupKanbanTestServer(t, sm, engine)

	req := httptest.NewRequest(http.MethodGet, "/kanban/engine", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp KanbanEngineStateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// ---------------------------------------------------------------------------
// handleEnableEngine / handleDisableEngine
// ---------------------------------------------------------------------------

func TestHandleEnableEngine_NoEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/enable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleDisableEngine_NoEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/disable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleEnableEngine_Success(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	engine := kanban.NewEngine(kanban.EngineConfig{
		StateManager: sm,
		EventBus:     eb,
		Logger:       slog.Default(),
	})

	r, _ := setupKanbanTestServer(t, sm, engine)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/enable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDisableEngine_Success(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	engine := kanban.NewEngine(kanban.EngineConfig{
		StateManager: sm,
		EventBus:     eb,
		Logger:       slog.Default(),
	})

	r, _ := setupKanbanTestServer(t, sm, engine)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/disable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleResetCircuitBreaker
// ---------------------------------------------------------------------------

func TestHandleResetCircuitBreaker_NoEngine(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	r, _ := setupKanbanTestServer(t, sm, nil)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/reset-circuit-breaker", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

func TestHandleResetCircuitBreaker_Success(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	engine := kanban.NewEngine(kanban.EngineConfig{
		StateManager: sm,
		EventBus:     eb,
		Logger:       slog.Default(),
	})

	r, _ := setupKanbanTestServer(t, sm, engine)

	req := httptest.NewRequest(http.MethodPost, "/kanban/engine/reset-circuit-breaker", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// workflowToKanbanResponse
// ---------------------------------------------------------------------------

func TestWorkflowToKanbanResponse(t *testing.T) {
	t.Parallel()
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour)
	completedAt := now

	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "Build a feature that does many things " + strings.Repeat("x", 200),
			Title:      "Feature 1",
			CreatedAt:  now,
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusCompleted,
			UpdatedAt: now,
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1"},
			},
			KanbanColumn:         "done",
			KanbanPosition:       3,
			PRURL:                "https://github.com/org/repo/pull/42",
			PRNumber:             42,
			KanbanStartedAt:      &startedAt,
			KanbanCompletedAt:    &completedAt,
			KanbanExecutionCount: 2,
			KanbanLastError:      "timeout",
		},
	}

	resp := workflowToKanbanResponse(wf)

	if resp.ID != "wf-1" {
		t.Errorf("expected 'wf-1', got %q", resp.ID)
	}
	if resp.KanbanColumn != "done" {
		t.Errorf("expected 'done', got %q", resp.KanbanColumn)
	}
	if resp.KanbanPosition != 3 {
		t.Errorf("expected 3, got %d", resp.KanbanPosition)
	}
	if resp.PRURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected PR URL, got %q", resp.PRURL)
	}
	if resp.PRNumber != 42 {
		t.Errorf("expected 42, got %d", resp.PRNumber)
	}
	if resp.TaskCount != 1 {
		t.Errorf("expected 1, got %d", resp.TaskCount)
	}
	if resp.KanbanExecutionCount != 2 {
		t.Errorf("expected 2, got %d", resp.KanbanExecutionCount)
	}
	if resp.KanbanLastError != "timeout" {
		t.Errorf("expected 'timeout', got %q", resp.KanbanLastError)
	}
	// Prompt should be truncated to 200 + "..."
	if len(resp.Prompt) != 203 {
		t.Errorf("expected truncated prompt length 203, got %d", len(resp.Prompt))
	}
}

func TestWorkflowToKanbanResponse_EmptyColumn(t *testing.T) {
	t.Parallel()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			UpdatedAt:    time.Now(),
			Tasks:        make(map[core.TaskID]*core.TaskState),
			KanbanColumn: "", // empty -> should default to "refinement"
		},
	}

	resp := workflowToKanbanResponse(wf)
	if resp.KanbanColumn != "refinement" {
		t.Errorf("expected 'refinement', got %q", resp.KanbanColumn)
	}
}

// ---------------------------------------------------------------------------
// ParseKanbanPosition
// ---------------------------------------------------------------------------

func TestParseKanbanPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"empty", "", 0},
		{"valid", "5", 5},
		{"zero", "0", 0},
		{"negative", "-1", 0},
		{"non-numeric", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/test"
			if tt.query != "" {
				url += "?position=" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			got := ParseKanbanPosition(req)
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// KanbanStatePoolProvider
// ---------------------------------------------------------------------------

// mockRegistryForKanban implements project.Registry for testing.
type mockRegistryForKanban struct {
	projects []*project.Project
	getErr   error
}

func (r *mockRegistryForKanban) ListProjects(_ context.Context) ([]*project.Project, error) {
	return r.projects, nil
}
func (r *mockRegistryForKanban) GetProject(_ context.Context, id string) (*project.Project, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, p := range r.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *mockRegistryForKanban) GetProjectByPath(_ context.Context, _ string) (*project.Project, error) {
	return nil, nil
}
func (r *mockRegistryForKanban) AddProject(_ context.Context, _ string, _ *project.AddProjectOptions) (*project.Project, error) {
	return nil, nil
}
func (r *mockRegistryForKanban) RemoveProject(_ context.Context, _ string) error { return nil }
func (r *mockRegistryForKanban) UpdateProject(_ context.Context, _ *project.Project) error {
	return nil
}
func (r *mockRegistryForKanban) ValidateProject(_ context.Context, _ string) error { return nil }
func (r *mockRegistryForKanban) ValidateAll(_ context.Context) error               { return nil }
func (r *mockRegistryForKanban) GetDefaultProject(_ context.Context) (*project.Project, error) {
	return nil, nil
}
func (r *mockRegistryForKanban) SetDefaultProject(_ context.Context, _ string) error { return nil }
func (r *mockRegistryForKanban) TouchProject(_ context.Context, _ string) error      { return nil }
func (r *mockRegistryForKanban) Reload() error                                       { return nil }
func (r *mockRegistryForKanban) Close() error                                        { return nil }
func (r *mockRegistryForKanban) Exists(_ string) bool                                { return true }

func TestKanbanStatePoolProvider_ListActiveProjects(t *testing.T) {
	t.Parallel()

	reg := &mockRegistryForKanban{
		projects: []*project.Project{
			{ID: "p1", Name: "Project 1", Path: "/p1", Status: project.StatusHealthy},
			{ID: "p2", Name: "Project 2", Path: "/p2", Status: project.StatusOffline},
			{ID: "p3", Name: "Project 3", Path: "/p3", Status: project.StatusDegraded},
		},
	}

	provider := NewKanbanStatePoolProvider(nil, reg)

	projects, err := provider.ListActiveProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should include p1 (healthy) and p3 (degraded), but not p2 (offline).
	if len(projects) != 2 {
		t.Fatalf("expected 2 active projects, got %d", len(projects))
	}

	ids := map[string]bool{}
	for _, p := range projects {
		ids[p.ID] = true
	}
	if !ids["p1"] {
		t.Error("expected p1 to be active")
	}
	if ids["p2"] {
		t.Error("expected p2 to NOT be active")
	}
	if !ids["p3"] {
		t.Error("expected p3 to be active")
	}
}

func TestKanbanStatePoolProvider_ListActiveProjects_NilRegistry(t *testing.T) {
	t.Parallel()
	provider := NewKanbanStatePoolProvider(nil, nil)

	_, err := provider.ListActiveProjects(context.Background())
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

func TestKanbanStatePoolProvider_ListLoadedProjects_NilPool(t *testing.T) {
	t.Parallel()
	provider := NewKanbanStatePoolProvider(nil, nil)

	projects, err := provider.ListLoadedProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects != nil {
		t.Errorf("expected nil, got %v", projects)
	}
}

func TestKanbanStatePoolProvider_GetProjectStateManager_NilPool(t *testing.T) {
	t.Parallel()
	provider := NewKanbanStatePoolProvider(nil, nil)

	_, err := provider.GetProjectStateManager(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

func TestKanbanStatePoolProvider_GetProjectEventBus_NilPool(t *testing.T) {
	t.Parallel()
	provider := NewKanbanStatePoolProvider(nil, nil)

	eb := provider.GetProjectEventBus(context.Background(), "p1")
	if eb != nil {
		t.Errorf("expected nil, got %v", eb)
	}
}

func TestKanbanStatePoolProvider_GetProjectExecutionContext_NilPool(t *testing.T) {
	t.Parallel()
	provider := NewKanbanStatePoolProvider(nil, nil)

	_, err := provider.GetProjectExecutionContext(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

// ---------------------------------------------------------------------------
// MoveWorkflow with empty fromColumn defaults to "refinement"
// ---------------------------------------------------------------------------

func TestHandleMoveWorkflow_EmptyFromColumn(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			UpdatedAt:    time.Now(),
			Tasks:        make(map[core.TaskID]*core.TaskState),
			KanbanColumn: "", // empty
		},
	}
	sm.workflows["wf-1"] = wf

	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))
	ks := NewKanbanServer(srv, nil, eb)
	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	body := `{"to_column": "todo", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// MoveWorkflow load error
// ---------------------------------------------------------------------------

func TestHandleMoveWorkflow_LoadError(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	sm.loadErr = fmt.Errorf("load error")

	r, _ := setupKanbanTestServer(t, sm, nil)

	body := `{"to_column": "todo", "position": 0}`
	req := httptest.NewRequest(http.MethodPost, "/kanban/workflows/wf-1/move", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}
