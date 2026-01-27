package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// addChiURLParams adds Chi URL params to the request context.
func addChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHandleRunWorkflow_WorkflowNotFound(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "nonexistent"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "workflow not found" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestHandleRunWorkflow_AlreadyRunning(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-running")] = &core.WorkflowState{
		WorkflowID:   "wf-running",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "test",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-running/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-running"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "workflow is already running" {
		t.Errorf("unexpected error message: %s", resp["error"])
	}
}

func TestHandleRunWorkflow_AlreadyCompleted(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-completed")] = &core.WorkflowState{
		WorkflowID:   "wf-completed",
		Status:       core.WorkflowStatusCompleted,
		CurrentPhase: core.PhaseExecute,
		Prompt:       "test",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-completed/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-completed"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

func TestHandleRunWorkflow_MissingStateManager(t *testing.T) {
	eb := events.New(100)
	srv := NewServer(nil, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-test/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-test"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

func TestHandleRunWorkflow_MissingConfigLoader(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-pending")] = &core.WorkflowState{
		WorkflowID:   "wf-pending",
		Status:       core.WorkflowStatusPending,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "test",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	eb := events.New(100)
	// Server without config loader - RunnerFactory() returns nil
	srv := NewServer(sm, eb,
		WithAgentRegistry(&mockAgentRegistry{}),
		WithLogger(slog.Default()),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-pending/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-pending"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected error message about missing configuration")
	}
}

func TestHandleRunWorkflow_EmptyWorkflowID(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows//run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": ""})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleRunWorkflow_DoubleExecution(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-double")] = &core.WorkflowState{
		WorkflowID:   "wf-double",
		Status:       core.WorkflowStatusPending,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "test",
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    []core.TaskID{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Manually mark as running using internal tracking
	if !markRunning("wf-double") {
		t.Fatal("failed to mark workflow as running")
	}
	defer markFinished("wf-double")

	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-double/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-double"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "workflow execution already in progress" {
		t.Errorf("unexpected error: %s", resp["error"])
	}
}

func TestHandleRunWorkflow_StateValidation(t *testing.T) {
	tests := []struct {
		name          string
		status        core.WorkflowStatus
		expectedCode  int
		expectedError string
	}{
		{
			name:          "pending workflow can run (stops at missing config)",
			status:        core.WorkflowStatusPending,
			expectedCode:  http.StatusServiceUnavailable,
			expectedError: "missing configuration",
		},
		{
			name:          "running workflow rejected",
			status:        core.WorkflowStatusRunning,
			expectedCode:  http.StatusConflict,
			expectedError: "already running",
		},
		{
			name:          "completed workflow rejected",
			status:        core.WorkflowStatusCompleted,
			expectedCode:  http.StatusConflict,
			expectedError: "already completed",
		},
		{
			name:          "failed workflow can resume (stops at missing config)",
			status:        core.WorkflowStatusFailed,
			expectedCode:  http.StatusServiceUnavailable,
			expectedError: "missing configuration",
		},
		{
			name:          "paused workflow can resume (stops at missing config)",
			status:        core.WorkflowStatusPaused,
			expectedCode:  http.StatusServiceUnavailable,
			expectedError: "missing configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := newMockStateManager()
			wfID := core.WorkflowID("wf-state-test")
			sm.workflows[wfID] = &core.WorkflowState{
				WorkflowID:   wfID,
				Status:       tt.status,
				CurrentPhase: core.PhaseAnalyze,
				Prompt:       "test",
				Tasks:        make(map[core.TaskID]*core.TaskState),
				TaskOrder:    []core.TaskID{},
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}

			eb := events.New(100)
			srv := NewServer(sm, eb,
				WithAgentRegistry(&mockAgentRegistry{}),
				WithLogger(slog.Default()),
			)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-state-test/run", nil)
			req = addChiURLParams(req, map[string]string{"workflowID": "wf-state-test"})
			w := httptest.NewRecorder()

			srv.HandleRunWorkflow(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("expected status %d, got %d: %s", tt.expectedCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleRunWorkflow_LoadError(t *testing.T) {
	sm := newMockStateManager()
	sm.loadErr = context.DeadlineExceeded // Simulate database timeout

	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-test/run", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-test"})
	w := httptest.NewRecorder()

	srv.HandleRunWorkflow(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "failed to load workflow" {
		t.Errorf("unexpected error: %s", resp["error"])
	}
}

func TestHandleGetWorkflow_IncludesReportPathAndOptimizedPrompt(t *testing.T) {
	sm := newMockStateManager()
	sm.workflows[core.WorkflowID("wf-artifacts")] = &core.WorkflowState{
		WorkflowID:      "wf-artifacts",
		Status:          core.WorkflowStatusCompleted,
		CurrentPhase:    core.PhaseExecute,
		Prompt:          "original prompt",
		OptimizedPrompt: "optimized prompt",
		ReportPath:      ".quorum/runs/wf-artifacts",
		Tasks:           make(map[core.TaskID]*core.TaskState),
		TaskOrder:       []core.TaskID{},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	sm.activeID = core.WorkflowID("wf-artifacts")

	eb := events.New(100)
	srv := NewServer(sm, eb, WithLogger(slog.Default()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-artifacts/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-artifacts"})
	w := httptest.NewRecorder()

	srv.handleGetWorkflow(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ReportPath != ".quorum/runs/wf-artifacts" {
		t.Errorf("expected report_path %q, got %q", ".quorum/runs/wf-artifacts", resp.ReportPath)
	}
	if resp.OptimizedPrompt != "optimized prompt" {
		t.Errorf("expected optimized_prompt %q, got %q", "optimized prompt", resp.OptimizedPrompt)
	}
}
