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

// newReviewableWorkflowState creates a workflow in awaiting_review with a Blueprint.
func newReviewableWorkflowState(id string, phase core.Phase) *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: core.WorkflowID(id),
			Prompt:     "test prompt",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeInteractive},
			CreatedAt:  time.Now().Add(-1 * time.Hour),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: phase,
			Tasks:        map[core.TaskID]*core.TaskState{},
			TaskOrder:    []core.TaskID{},
			UpdatedAt:    time.Now(),
		},
	}
}

// --- HandleReviewWorkflow ---

func TestHandleReviewWorkflow_ApproveAnalyze(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze","feedback":"looks good"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "running" {
		t.Errorf("expected status 'running', got %q", resp["status"])
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.InteractiveReview == nil {
		t.Fatal("expected InteractiveReview to be set")
	}
	if saved.InteractiveReview.AnalysisFeedback != "looks good" {
		t.Errorf("expected feedback 'looks good', got %q", saved.InteractiveReview.AnalysisFeedback)
	}
}

func TestHandleReviewWorkflow_ApprovePlan(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhaseExecute)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"plan","feedback":"plan feedback"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.InteractiveReview == nil {
		t.Fatal("expected InteractiveReview to be set")
	}
	if saved.InteractiveReview.PlanFeedback != "plan feedback" {
		t.Errorf("expected plan feedback, got %q", saved.InteractiveReview.PlanFeedback)
	}
}

func TestHandleReviewWorkflow_ApprovePlan_WithExecuteOptionsSelection(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhaseExecute)
	// Planned tasks (all pending) with deps: t2 depends on t1.
	state.Tasks = map[core.TaskID]*core.TaskState{
		"t1": {ID: "t1", Name: "t1", Status: core.TaskStatusPending},
		"t2": {ID: "t2", Name: "t2", Status: core.TaskStatusPending, Dependencies: []core.TaskID{"t1"}},
		"t3": {ID: "t3", Name: "t3", Status: core.TaskStatusPending},
	}
	state.TaskOrder = []core.TaskID{"t1", "t2", "t3"}

	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"plan","execute_options":{"selected_task_ids":["t2"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Tasks["t3"].Status != core.TaskStatusSkipped {
		t.Fatalf("expected t3 skipped, got %s", saved.Tasks["t3"].Status)
	}
	if saved.Tasks["t1"].Status != core.TaskStatusPending {
		t.Fatalf("expected t1 pending (dependency of selected), got %s", saved.Tasks["t1"].Status)
	}
	if saved.Tasks["t2"].Status != core.TaskStatusPending {
		t.Fatalf("expected t2 pending (selected), got %s", saved.Tasks["t2"].Status)
	}
}

func TestHandleReviewWorkflow_ApprovePlan_WithExecuteOptionsEmptySelectionRejected(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhaseExecute)
	state.Tasks = map[core.TaskID]*core.TaskState{
		"t1": {ID: "t1", Name: "t1", Status: core.TaskStatusPending},
	}
	state.TaskOrder = []core.TaskID{"t1"}
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"plan","execute_options":{"selected_task_ids":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_ApproveAnalyze_WithExecuteOptionsRejected(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze","execute_options":{"selected_task_ids":["t1"]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_ApproveWithContinueUnattended(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze","continue_unattended":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Blueprint.ExecutionMode != core.ExecutionModeMultiAgent {
		t.Errorf("expected mode switch to multi_agent, got %q", saved.Blueprint.ExecutionMode)
	}
}

func TestHandleReviewWorkflow_RejectAnalyze(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"reject","phase":"analyze","feedback":"needs more detail"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Status != core.WorkflowStatusPending {
		t.Errorf("expected status pending after reject analyze, got %s", saved.Status)
	}
	if saved.CurrentPhase != core.PhaseAnalyze {
		t.Errorf("expected phase analyze after reject, got %s", saved.CurrentPhase)
	}
	if saved.InteractiveReview != nil {
		t.Error("expected InteractiveReview to be nil after reject")
	}
}

func TestHandleReviewWorkflow_RejectPlan(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhaseExecute)
	state.Tasks = map[core.TaskID]*core.TaskState{
		"t1": {ID: "t1", Name: "test"},
	}
	state.TaskOrder = []core.TaskID{"t1"}
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"reject","phase":"plan"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Status != core.WorkflowStatusCompleted {
		t.Errorf("expected status completed after reject plan, got %s", saved.Status)
	}
	if saved.CurrentPhase != core.PhasePlan {
		t.Errorf("expected phase plan after reject, got %s", saved.CurrentPhase)
	}
	if len(saved.Tasks) != 0 {
		t.Errorf("expected tasks cleared after plan reject, got %d", len(saved.Tasks))
	}
	if len(saved.TaskOrder) != 0 {
		t.Errorf("expected task order cleared after plan reject, got %d", len(saved.TaskOrder))
	}
}

func TestHandleReviewWorkflow_InvalidAction(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"invalid","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_NotAwaitingReview(t *testing.T) {
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

	body := `{"action":"approve","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"approve","phase":"analyze"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReviewWorkflow_InvalidBody(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhasePlan)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString("{bad json"))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleReviewWorkflow_RejectInvalidPhase(t *testing.T) {
	t.Parallel()
	state := newReviewableWorkflowState("wf-1", core.PhaseExecute)
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = state
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	body := `{"action":"reject","phase":"execute"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/review", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid reject phase, got %d: %s", w.Code, w.Body.String())
	}
}

// --- HandleSwitchInteractive ---

func TestHandleSwitchInteractive_Success(t *testing.T) {
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
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Blueprint.ExecutionMode != core.ExecutionModeInteractive {
		t.Errorf("expected interactive mode, got %q", saved.Blueprint.ExecutionMode)
	}
}

func TestHandleSwitchInteractive_AlreadyInteractive(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeInteractive},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "Workflow is already in interactive mode" {
		t.Errorf("expected already-interactive message, got %q", resp["message"])
	}
}

func TestHandleSwitchInteractive_NotRunning(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeMultiAgent},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			CurrentPhase: core.PhaseDone,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSwitchInteractive_NotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSwitchInteractive_NilBlueprint(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-1")] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  nil,
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/switch-interactive", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	saved := sm.workflows[core.WorkflowID("wf-1")]
	if saved.Blueprint == nil {
		t.Fatal("expected Blueprint to be created")
	}
	if saved.Blueprint.ExecutionMode != core.ExecutionModeInteractive {
		t.Errorf("expected interactive mode, got %q", saved.Blueprint.ExecutionMode)
	}
}
