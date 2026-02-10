package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// newMutableWorkflowState creates a workflow state in awaiting_review with a plan.
func newMutableWorkflowState(id string) *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: core.WorkflowID(id),
			Prompt:     "test prompt",
			CreatedAt:  time.Now().Add(-1 * time.Hour),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {
					ID:     "task-1",
					Phase:  core.PhaseExecute,
					Name:   "Task 1",
					Status: core.TaskStatusPending,
					CLI:    "claude",
				},
				"task-2": {
					ID:           "task-2",
					Phase:        core.PhaseExecute,
					Name:         "Task 2",
					Status:       core.TaskStatusPending,
					CLI:          "codex",
					Dependencies: []core.TaskID{"task-1"},
				},
			},
			TaskOrder: []core.TaskID{"task-1", "task-2"},
			UpdatedAt: time.Now(),
		},
	}
}

func newTestServerWithState(t *testing.T, id string, state *core.WorkflowState) (*Server, *mockStateManager) {
	t.Helper()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID(id)] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))
	return srv, sm
}

// --- canMutateTasks ---

func TestCanMutateTasks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status core.WorkflowStatus
		phase  core.Phase
		want   bool
	}{
		{"awaiting_review+execute", core.WorkflowStatusAwaitingReview, core.PhaseExecute, true},
		{"awaiting_review+done", core.WorkflowStatusAwaitingReview, core.PhaseDone, true},
		{"completed+execute", core.WorkflowStatusCompleted, core.PhaseExecute, true},
		{"completed+done", core.WorkflowStatusCompleted, core.PhaseDone, true},
		{"running+execute", core.WorkflowStatusRunning, core.PhaseExecute, false},
		{"paused+execute", core.WorkflowStatusPaused, core.PhaseExecute, false},
		{"pending+execute", core.WorkflowStatusPending, core.PhaseExecute, false},
		{"awaiting_review+analyze", core.WorkflowStatusAwaitingReview, core.PhaseAnalyze, false},
		{"awaiting_review+plan", core.WorkflowStatusAwaitingReview, core.PhasePlan, false},
		{"failed+execute", core.WorkflowStatusFailed, core.PhaseExecute, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			state := &core.WorkflowState{
				WorkflowRun: core.WorkflowRun{
					Status:       tt.status,
					CurrentPhase: tt.phase,
				},
			}
			if got := canMutateTasks(state); got != tt.want {
				t.Errorf("canMutateTasks(%s, %s) = %v, want %v", tt.status, tt.phase, got, tt.want)
			}
		})
	}
}

// --- validateTaskDAG ---

func TestValidateTaskDAG_NoCycle(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a"},
				"b": {ID: "b", Dependencies: []core.TaskID{"a"}},
				"c": {ID: "c", Dependencies: []core.TaskID{"a", "b"}},
			},
		},
	}
	if err := validateTaskDAG(state); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateTaskDAG_WithCycle(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a", Dependencies: []core.TaskID{"c"}},
				"b": {ID: "b", Dependencies: []core.TaskID{"a"}},
				"c": {ID: "c", Dependencies: []core.TaskID{"b"}},
			},
		},
	}
	if validateTaskDAG(state) == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestValidateTaskDAG_SelfDependency(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a", Dependencies: []core.TaskID{"a"}},
			},
		},
	}
	if validateTaskDAG(state) == nil {
		t.Error("expected cycle error for self-dependency, got nil")
	}
}

func TestValidateTaskDAG_Empty(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{},
		},
	}
	if err := validateTaskDAG(state); err != nil {
		t.Errorf("expected no error for empty DAG, got %v", err)
	}
}

// --- handleCreateTask ---

func TestHandleCreateTask_Success(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, sm := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"New Task","cli":"claude","description":"A new task"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Name != "New Task" {
		t.Errorf("expected name 'New Task', got %q", resp.Name)
	}
	if resp.CLI != "claude" {
		t.Errorf("expected cli 'claude', got %q", resp.CLI)
	}

	// Verify persisted
	saved := sm.workflows[core.WorkflowID("wf-1")]
	if len(saved.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(saved.Tasks))
	}
}

func TestHandleCreateTask_MissingName(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateTask_WorkflowNotMutable(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseExecute,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"name":"New","cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_WithDependencies(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"New Task","cli":"claude","dependencies":["task-1"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_InvalidDependency(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"New","cli":"claude","dependencies":["nonexistent"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_CyclicDependency(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a", Dependencies: []core.TaskID{"b"}},
				"b": {ID: "b"},
			},
			TaskOrder: []core.TaskID{"a", "b"},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	// Create task "c" depending on "a", then set "b" to depend on "c" — but actually
	// we need to create a cycle. Let's create a task that depends on "a",
	// where "a" depends on "b". If "b" now depends on this new task, it's a cycle.
	// But we can't set b's deps here. Instead, create a task that depends on itself-ish.
	// Actually, the simpler test is: "a" depends on "b" (existing). Create "c" depending on "a".
	// Then update "b" to depend on "c". But that's update, not create.
	// For create: the cycle would only happen if the new task creates a cycle.
	// Since the new task is a leaf (nothing depends on it yet), it can't create a cycle
	// by having dependencies on existing tasks.
	// The cycle detection in create matters when existing tasks already form a near-cycle.
	// Let's test this indirectly through the update handler instead.
	// For now, verify that create with valid deps works.
	body := `{"name":"New","cli":"claude","dependencies":["a"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	// This should succeed since "new" → "a" → "b" has no cycle
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"name":"New","cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleUpdateTask ---

func TestHandleUpdateTask_Success(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"Updated Task","description":"new desc"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Name != "Updated Task" {
		t.Errorf("expected name 'Updated Task', got %q", resp.Name)
	}
}

func TestHandleUpdateTask_EmptyName(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	empty := ""
	reqBody := UpdateTaskRequest{Name: &empty}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_EmptyCLI(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	empty := ""
	reqBody := UpdateTaskRequest{CLI: &empty}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_TaskNotFound(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"x"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/nonexistent", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_SelfDependency(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"dependencies":["task-1"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_ChangeCLI(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	cli := "gemini"
	reqBody := UpdateTaskRequest{CLI: &cli}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.CLI != "gemini" {
		t.Errorf("expected cli 'gemini', got %q", resp.CLI)
	}
}

// --- handleDeleteTask ---

func TestHandleDeleteTask_Success(t *testing.T) {
	t.Parallel()
	// Create state with task-1 (no dependents) and task-2 (depends on task-1)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Phase: core.PhaseExecute, Name: "Task 1", CLI: "claude"},
				"task-2": {ID: "task-2", Phase: core.PhaseExecute, Name: "Task 2", CLI: "codex"},
			},
			TaskOrder: []core.TaskID{"task-1", "task-2"},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	// Delete task-2 (no dependents)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-1/tasks/task-2", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if len(saved.Tasks) != 1 {
		t.Errorf("expected 1 task after delete, got %d", len(saved.Tasks))
	}
	if len(saved.TaskOrder) != 1 {
		t.Errorf("expected 1 task in order, got %d", len(saved.TaskOrder))
	}
}

func TestHandleDeleteTask_HasDependents(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	// Try to delete task-1 which task-2 depends on
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-1/tasks/task-1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleDeleteTask_NotFound(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/wf-1/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleReorderTasks ---

func TestHandleReorderTasks_Success(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, sm := newTestServerWithState(t, "wf-1", state)

	body := `{"task_order":["task-2","task-1"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/tasks/reorder", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.TaskOrder[0] != "task-2" || saved.TaskOrder[1] != "task-1" {
		t.Errorf("expected reversed order, got %v", saved.TaskOrder)
	}
}

func TestHandleReorderTasks_MismatchedCount(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"task_order":["task-1"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/tasks/reorder", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReorderTasks_DuplicateTask(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"task_order":["task-1","task-1"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/tasks/reorder", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReorderTasks_UnknownTask(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"task_order":["task-1","nonexistent"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/tasks/reorder", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleListTasks ---

func TestHandleListTasks_Success(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/tasks", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp []TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(resp))
	}
}

func TestHandleListTasks_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/tasks", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- loadMutableTaskState helper ---

func TestLoadMutableTaskState_SaveError(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = newMutableWorkflowState("wf-1")
	sm.saveErr = json.Unmarshal([]byte("bad"), nil) // any non-nil error
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"name":"New","cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on save error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoadMutableTaskState_LoadError(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.loadErr = json.Unmarshal([]byte("bad"), nil) // any non-nil error
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"name":"New","cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on load error, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleGetTask ---

func TestHandleGetTask_Success(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/tasks/task-1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Name != "Task 1" {
		t.Errorf("expected name 'Task 1', got %q", resp.Name)
	}
	if resp.CLI != "claude" {
		t.Errorf("expected cli 'claude', got %q", resp.CLI)
	}
	if resp.ID != "task-1" {
		t.Errorf("expected id 'task-1', got %q", resp.ID)
	}
}

func TestHandleGetTask_NotFound(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetTask_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/tasks/task-1", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleCreateTask additional tests ---

func TestHandleCreateTask_MissingCLI(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"name":"New Task"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing CLI, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_InvalidBody(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreateTask_NilTasks(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			Tasks:        nil,
			TaskOrder:    nil,
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"name":"First","cli":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/tasks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleUpdateTask additional tests ---

func TestHandleUpdateTask_InvalidBody(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_UpdateDependencies(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	// Update task-2 to depend on nothing (remove dependency on task-1)
	body := `{"dependencies":[]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-2", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies, got %v", resp.Dependencies)
	}
}

func TestHandleUpdateTask_DependencyNotFound(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	body := `{"dependencies":["nonexistent"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_CyclicDependency(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"a": {ID: "a", Phase: core.PhaseExecute, Name: "A", CLI: "claude", Dependencies: []core.TaskID{"b"}},
				"b": {ID: "b", Phase: core.PhaseExecute, Name: "B", CLI: "claude"},
			},
			TaskOrder: []core.TaskID{"a", "b"},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	// Update b to depend on a → creates cycle a→b→a
	body := `{"dependencies":["a"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/b", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for cycle, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleUpdateTask_ChangeDescription(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	desc := "Updated description"
	reqBody := UpdateTaskRequest{Description: &desc}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/wf-1/tasks/task-1", bytes.NewReader(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TaskResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", resp.Description)
	}
}

// --- handleReorderTasks additional tests ---

func TestHandleReorderTasks_InvalidBody(t *testing.T) {
	t.Parallel()
	state := newMutableWorkflowState("wf-1")
	srv, _ := newTestServerWithState(t, "wf-1", state)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/tasks/reorder", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- HandleReviewWorkflow additional tests ---

func TestHandleReviewWorkflow_SaveError(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	sm.saveErr = json.Unmarshal([]byte("bad"), nil)
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_RejectSaveError(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	sm.saveErr = json.Unmarshal([]byte("bad"), nil)
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"reject","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_ApproveNoFeedback(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.InteractiveReview == nil {
		t.Fatal("expected InteractiveReview")
	}
	if saved.InteractiveReview.AnalysisFeedback != "" {
		t.Errorf("expected empty feedback, got %q", saved.InteractiveReview.AnalysisFeedback)
	}
}

// --- HandleSwitchInteractive additional tests ---

func TestHandleSwitchInteractive_SaveError(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeMultiAgent},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	sm.saveErr = json.Unmarshal([]byte("bad"), nil)
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSwitchInteractive_LoadError(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.loadErr = json.Unmarshal([]byte("bad"), nil)
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
