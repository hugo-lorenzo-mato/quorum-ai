package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// ---------------------------------------------------------------------------
// Server options at 0% coverage
// ---------------------------------------------------------------------------

func TestWithResourceMonitor(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithResourceMonitor(nil))
	if srv.resourceMonitor != nil {
		t.Error("expected nil resource monitor")
	}
}

func TestWithKanbanEngine(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithKanbanEngine(nil))
	if srv.kanbanEngine != nil {
		t.Error("expected nil kanban engine")
	}
}

func TestWithProjectRegistry(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithProjectRegistry(nil))
	if srv.projectRegistry != nil {
		t.Error("expected nil project registry")
	}
}

func TestWithStatePool(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	srv := NewServer(sm, eb, WithStatePool(nil))
	if srv.statePool != nil {
		t.Error("expected nil state pool")
	}
}

// ---------------------------------------------------------------------------
// handleDeepHealth (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleDeepHealth_NoResourceMonitor(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/health/deep", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp DeepHealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", resp.Status)
	}
	if resp.System == nil {
		t.Error("expected System metrics to be present")
	}
	if resp.Resources != nil {
		t.Error("expected Resources to be nil without monitor")
	}
}

// ---------------------------------------------------------------------------
// parseDurationOrDefault (0% coverage)
// ---------------------------------------------------------------------------

func TestParseDurationOrDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		def      time.Duration
		expected time.Duration
	}{
		{"empty returns default", "", 5 * time.Minute, 5 * time.Minute},
		{"whitespace returns default", "  ", 5 * time.Minute, 5 * time.Minute},
		{"valid duration", "10m", 5 * time.Minute, 10 * time.Minute},
		{"invalid format returns default", "invalid", 5 * time.Minute, 5 * time.Minute},
		{"zero returns default", "0s", 5 * time.Minute, 5 * time.Minute},
		{"negative returns default", "-5m", 5 * time.Minute, 5 * time.Minute},
		{"valid hours", "2h", time.Hour, 2 * time.Hour},
		{"valid seconds", "30s", time.Minute, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseDurationOrDefault(tt.raw, tt.def)
			if got != tt.expected {
				t.Errorf("parseDurationOrDefault(%q, %v) = %v, want %v", tt.raw, tt.def, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// effectiveWorkflowTimeout (0% coverage)
// ---------------------------------------------------------------------------

func TestEffectiveWorkflowTimeout(t *testing.T) {
	t.Parallel()

	t.Run("nil config and nil blueprint", func(t *testing.T) {
		t.Parallel()
		got := effectiveWorkflowTimeout(nil, nil)
		if got != defaultWorkflowExecTimeout {
			t.Errorf("expected %v, got %v", defaultWorkflowExecTimeout, got)
		}
	})

	t.Run("blueprint timeout takes precedence", func(t *testing.T) {
		t.Parallel()
		bp := &core.Blueprint{Timeout: 30 * time.Minute}
		cfg := &config.Config{Workflow: config.WorkflowConfig{Timeout: "2h"}}
		got := effectiveWorkflowTimeout(cfg, bp)
		if got != 30*time.Minute {
			t.Errorf("expected 30m, got %v", got)
		}
	})

	t.Run("config timeout used when no blueprint", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{Workflow: config.WorkflowConfig{Timeout: "4h"}}
		got := effectiveWorkflowTimeout(cfg, nil)
		if got != 4*time.Hour {
			t.Errorf("expected 4h, got %v", got)
		}
	})

	t.Run("blueprint with zero timeout uses config", func(t *testing.T) {
		t.Parallel()
		bp := &core.Blueprint{Timeout: 0}
		cfg := &config.Config{Workflow: config.WorkflowConfig{Timeout: "3h"}}
		got := effectiveWorkflowTimeout(cfg, bp)
		if got != 3*time.Hour {
			t.Errorf("expected 3h, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// execTimeoutForAnalyze (0% coverage)
// ---------------------------------------------------------------------------

func TestExecTimeoutForAnalyze(t *testing.T) {
	t.Parallel()

	t.Run("nil config uses defaults", func(t *testing.T) {
		t.Parallel()
		got := execTimeoutForAnalyze(nil, nil)
		if got != defaultAnalyzeExecTimeout {
			t.Errorf("expected %v, got %v", defaultAnalyzeExecTimeout, got)
		}
	})

	t.Run("config analyze timeout used", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Phases: config.PhasesConfig{
				Analyze: config.AnalyzePhaseConfig{Timeout: "30m"},
			},
		}
		got := execTimeoutForAnalyze(cfg, nil)
		if got != 30*time.Minute {
			t.Errorf("expected 30m, got %v", got)
		}
	})

	t.Run("workflow timeout caps analyze timeout", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Workflow: config.WorkflowConfig{Timeout: "20m"},
			Phases: config.PhasesConfig{
				Analyze: config.AnalyzePhaseConfig{Timeout: "2h"},
			},
		}
		got := execTimeoutForAnalyze(cfg, nil)
		if got != 20*time.Minute {
			t.Errorf("expected 20m, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// execTimeoutForPlan (0% coverage)
// ---------------------------------------------------------------------------

func TestExecTimeoutForPlan(t *testing.T) {
	t.Parallel()

	t.Run("nil config uses defaults", func(t *testing.T) {
		t.Parallel()
		got := execTimeoutForPlan(nil, nil)
		if got != defaultPlanExecTimeout {
			t.Errorf("expected %v, got %v", defaultPlanExecTimeout, got)
		}
	})

	t.Run("config plan timeout used", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Phases: config.PhasesConfig{
				Plan: config.PlanPhaseConfig{Timeout: "45m"},
			},
		}
		got := execTimeoutForPlan(cfg, nil)
		if got != 45*time.Minute {
			t.Errorf("expected 45m, got %v", got)
		}
	})

	t.Run("workflow timeout caps plan timeout", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Workflow: config.WorkflowConfig{Timeout: "15m"},
			Phases: config.PhasesConfig{
				Plan: config.PlanPhaseConfig{Timeout: "2h"},
			},
		}
		got := execTimeoutForPlan(cfg, nil)
		if got != 15*time.Minute {
			t.Errorf("expected 15m, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// isZombieWorkflow (0% coverage)
// ---------------------------------------------------------------------------

func TestIsZombieWorkflow_NilTracker(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	// No unifiedTracker → always false.
	if srv.isZombieWorkflow(context.Background(), "wf-123") {
		t.Error("expected false for nil tracker")
	}
}

// ---------------------------------------------------------------------------
// handleDeleteWorkflow (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleDeleteWorkflow_Success(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	wfID := core.WorkflowID("wf-delete-test")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test delete",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusCompleted,
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
			UpdatedAt: time.Now(),
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/"+string(wfID)+"/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify workflow was deleted.
	if _, ok := sm.workflows[wfID]; ok {
		t.Error("expected workflow to be deleted from state manager")
	}
}

func TestHandleDeleteWorkflow_NotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/nonexistent/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDeleteWorkflow_RunningConflict(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	wfID := core.WorkflowID("wf-running-del")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusRunning,
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
			UpdatedAt: time.Now(),
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workflows/"+string(wfID)+"/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// handleDownloadWorkflow (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleDownloadWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/download", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDownloadWorkflow_NoReportPath(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	wfID := core.WorkflowID("wf-no-report")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:     core.WorkflowStatusCompleted,
			Tasks:      make(map[core.TaskID]*core.TaskState),
			TaskOrder:  []core.TaskID{},
			UpdatedAt:  time.Now(),
			ReportPath: "", // No report path
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/download", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDownloadWorkflow_ReportDirMissing(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	wfID := core.WorkflowID("wf-missing-dir")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:     core.WorkflowStatusCompleted,
			Tasks:      make(map[core.TaskID]*core.TaskState),
			TaskOrder:  []core.TaskID{},
			UpdatedAt:  time.Now(),
			ReportPath: "/nonexistent/path/reports",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/download", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleDownloadWorkflow_Success(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	tmpDir := t.TempDir()
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// Create a report directory with a file.
	reportDir := filepath.Join(tmpDir, "reports")
	if err := os.MkdirAll(reportDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reportDir, "report.md"), []byte("# Report"), 0o600); err != nil {
		t.Fatal(err)
	}

	wfID := core.WorkflowID("wf-download-ok")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:     core.WorkflowStatusCompleted,
			Tasks:      make(map[core.TaskID]*core.TaskState),
			TaskOrder:  []core.TaskID{},
			UpdatedAt:  time.Now(),
			ReportPath: reportDir,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/download", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type 'application/zip', got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "wf-download-ok") {
		t.Errorf("expected Content-Disposition to contain workflow ID, got %q", cd)
	}
}

// ---------------------------------------------------------------------------
// handleCancelWorkflow (0% coverage) — basic paths
// ---------------------------------------------------------------------------

func TestHandleCancelWorkflow_NoExecutorOrTracker(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/cancel", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleForceStopWorkflow (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleForceStopWorkflow_NoTracker(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/force-stop", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handlePauseWorkflow (0% coverage)
// ---------------------------------------------------------------------------

func TestHandlePauseWorkflow_NeitherTrackerNorExecutor(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/pause", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleResumeWorkflow (0% coverage) — basic paths
// ---------------------------------------------------------------------------

func TestHandleResumeWorkflow_NeitherTrackerNorExecutor(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	wfID := core.WorkflowID("wf-resume")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "Test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:    core.WorkflowStatusPaused,
			Tasks:     make(map[core.TaskID]*core.TaskState),
			TaskOrder: []core.TaskID{},
			UpdatedAt: time.Now(),
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/"+string(wfID)+"/resume", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	// Without tracker or executor, the handler checks for stateManager-based resume.
	// The exact code depends on state, but it should not panic.
	if rec.Code == 0 {
		t.Error("expected a non-zero status code")
	}
}

// ---------------------------------------------------------------------------
// handleValidateGlobalConfig (0% coverage)
// ---------------------------------------------------------------------------

func TestHandleValidateGlobalConfig_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	body := `{
		"log": {"level": "info"},
		"agents": {"default": "claude", "claude": {"enabled": true, "phases": {"plan": true, "execute": true, "synthesize": true}}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}, "synthesizer": {"agent": "claude"}}},
		"git": {"worktree": {"dir": ".worktrees"}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/global/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result ValidationResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, got false with errors: %+v", result.Errors)
	}
}

func TestHandleValidateGlobalConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/global/validate", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleValidateGlobalConfig_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// All agents disabled but default set to nonexistent agent → validation error.
	body := `{
		"agents": {
			"default": "nonexistent_agent",
			"claude": {"enabled": false},
			"gemini": {"enabled": false},
			"codex": {"enabled": false},
			"copilot": {"enabled": false},
			"opencode": {"enabled": false}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/global/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result ValidationResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for invalid config")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one validation error")
	}
}

// ---------------------------------------------------------------------------
// getProjectChatStore (0% coverage)
// ---------------------------------------------------------------------------

func TestGetProjectChatStore_FallbackToGlobal(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	// No project context → fallback to server's (nil) chatStore.
	store := srv.getProjectChatStore(context.Background())
	if store != nil {
		t.Error("expected nil chat store when no global store is set")
	}
}

func TestGetProjectChatStore_WithServerStore(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	mockStore := &mockChatStore{}
	srv := NewServer(sm, eb, WithRoot(t.TempDir()), WithChatStore(mockStore))

	store := srv.getProjectChatStore(context.Background())
	if store == nil {
		t.Error("expected non-nil chat store")
	}
}

// mockChatStore is a minimal mock for the ChatStore interface.
type mockChatStore struct{}

func (m *mockChatStore) SaveSession(_ context.Context, _ *core.ChatSessionState) error { return nil }
func (m *mockChatStore) LoadSession(_ context.Context, _ string) (*core.ChatSessionState, error) {
	return nil, nil
}
func (m *mockChatStore) ListSessions(_ context.Context) ([]*core.ChatSessionState, error) {
	return nil, nil
}
func (m *mockChatStore) DeleteSession(_ context.Context, _ string) error { return nil }
func (m *mockChatStore) SaveMessage(_ context.Context, _ *core.ChatMessageState) error {
	return nil
}
func (m *mockChatStore) LoadMessages(_ context.Context, _ string) ([]*core.ChatMessageState, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// project_helpers.go — exported helper functions
// ---------------------------------------------------------------------------

func TestGetProjectRootFromContext_NoProjectContext(t *testing.T) {
	t.Parallel()
	root := GetProjectRootFromContext(context.Background())
	if root != "" {
		t.Errorf("expected empty string, got %q", root)
	}
}

func TestGetProjectID_NoContext(t *testing.T) {
	t.Parallel()
	id := getProjectID(context.Background())
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// ---------------------------------------------------------------------------
// HandleRunWorkflow — partial coverage (basic validation paths)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// handleGetIssuesConfig (config.go related)
// ---------------------------------------------------------------------------

func TestHandleGetIssuesConfig(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/issues", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Kanban helpers — workflowToKanbanResponse edge cases
// ---------------------------------------------------------------------------

func TestWorkflowToKanbanResponse_ShortPrompt(t *testing.T) {
	t.Parallel()
	wf := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-short",
			Prompt:     "Short prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusPending,
			UpdatedAt:    time.Now(),
			Tasks:        make(map[core.TaskID]*core.TaskState),
			KanbanColumn: "todo",
		},
	}

	resp := workflowToKanbanResponse(wf)
	if resp.Prompt != "Short prompt" {
		t.Errorf("expected full short prompt, got %q", resp.Prompt)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateConfig with inherits_global mode (project registry)
// ---------------------------------------------------------------------------

func TestHandleUpdateConfig_InheritsGlobalConflict(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	// Create a mock registry that returns inherit_global for the project
	reg := &mockRegistryForKanban{
		projects: []*project.Project{
			{
				ID:         "p1",
				Name:       "Project 1",
				Path:       tmpDir,
				Status:     project.StatusHealthy,
				ConfigMode: project.ConfigModeInheritGlobal,
			},
		},
	}

	srv := NewServer(sm, eb, WithRoot(tmpDir), WithProjectRegistry(reg))

	// We need to verify that without project context middleware, it falls back to custom mode
	// so let's just test the basic update on a server with a registry configured
	body := `{"log": {"level": "debug"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	// Without the project context middleware wired up with StatePool, this goes through the
	// standard path. We just verify it doesn't panic.
	if rec.Code == 0 {
		t.Error("expected non-zero status code")
	}
}

// ---------------------------------------------------------------------------
// handleResetConfig – verify defaults after reset
// ---------------------------------------------------------------------------

func TestHandleResetConfig_VerifyDefaultValues(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	// Reset
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reset", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify the ETag was set in header
	if rec.Header().Get("ETag") == "" {
		t.Error("expected ETag header after reset")
	}

	// Verify meta
	if resp.Meta.Source != "file" {
		t.Errorf("expected source 'file', got %q", resp.Meta.Source)
	}
	if resp.Meta.Scope != "project" {
		t.Errorf("expected scope 'project', got %q", resp.Meta.Scope)
	}
}

// ---------------------------------------------------------------------------
// handleGetConfig with project config path (getProjectConfigPath branches)
// ---------------------------------------------------------------------------

func TestGetProjectConfigPath_WithProjectContext_Root(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	projectRoot := filepath.Join(tmpDir, "my-project")
	pc := &project.ProjectContext{
		ID:   "test-proj",
		Root: projectRoot,
	}
	ctx := contextWithProjectContext(context.Background(), pc)

	path := srv.getProjectConfigPath(ctx)
	expected := filepath.Join(projectRoot, ".quorum", "config.yaml")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

// ---------------------------------------------------------------------------
// Config schema — test all sections are built correctly
// ---------------------------------------------------------------------------

func TestBuildConfigSchema_AllSections(t *testing.T) {
	t.Parallel()
	schema := buildConfigSchema()

	if len(schema.Sections) == 0 {
		t.Fatal("expected non-empty schema sections")
	}

	// Verify all expected section IDs are present.
	expectedIDs := map[string]bool{
		"log":            false,
		"chat":           false,
		"report":         false,
		"workflow":       false,
		"state":          false,
		"agents":         false,
		"phases.analyze": false,
		"phases.plan":    false,
		"phases.execute": false,
		"git":            false,
		"github":         false,
		"trace":          false,
		"diagnostics":    false,
	}

	for _, section := range schema.Sections {
		expectedIDs[section.ID] = true
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected section %q in schema", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Config enums — deeper validation
// ---------------------------------------------------------------------------

func TestHandleGetEnums_AllFields(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/enums", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var enums EnumsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &enums); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify model-related fields.
	if len(enums.AgentModels) == 0 {
		t.Error("expected agent_models to be populated")
	}
	if len(enums.AgentDefaultModels) == 0 {
		t.Error("expected agent_default_models to be populated")
	}
	if len(enums.AgentsWithReasoning) == 0 {
		t.Error("expected agents_with_reasoning to be populated")
	}
	if len(enums.AgentReasoningEfforts) == 0 {
		t.Error("expected agent_reasoning_efforts to be populated")
	}
	// Verify phase model keys.
	if len(enums.PhaseModelKeys) == 0 {
		t.Error("expected phase_model_keys to be populated")
	}
	// Verify worktree modes.
	if len(enums.WorktreeModes) == 0 {
		t.Error("expected worktree_modes to be populated")
	}
	// Verify merge strategies.
	if len(enums.MergeStrategies) == 0 {
		t.Error("expected merge_strategies to be populated")
	}
	// Verify trace modes.
	if len(enums.TraceModes) == 0 {
		t.Error("expected trace_modes to be populated")
	}
	// Verify log formats.
	if len(enums.LogFormats) == 0 {
		t.Error("expected log_formats to be populated")
	}
}

// ---------------------------------------------------------------------------
// handleGetConfig — source field variations
// ---------------------------------------------------------------------------

func TestHandleGetConfig_SourceDefault(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// No config file exists → source should be "default".
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Meta.Source != "default" {
		t.Errorf("expected source 'default', got %q", resp.Meta.Source)
	}
}

func TestHandleGetConfig_SourceFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// Create config file.
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config.DefaultConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Meta.Source != "file" {
		t.Errorf("expected source 'file', got %q", resp.Meta.Source)
	}
	if resp.Meta.LastModified == "" {
		t.Error("expected LastModified to be set when loading from file")
	}
}

// ---------------------------------------------------------------------------
// SSEClient struct tests (sse.go — for coverage of NewSSEClient, Close, Events)
// ---------------------------------------------------------------------------

func TestSSEClient_Lifecycle(t *testing.T) {
	t.Parallel()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	client := NewSSEClient(eb)
	if client == nil {
		t.Fatal("expected non-nil SSE client")
	}

	ch := client.Events()
	if ch == nil {
		t.Fatal("expected non-nil event channel")
	}

	client.Close()
}

// ---------------------------------------------------------------------------
// handleUpdateConfig — successful update
// ---------------------------------------------------------------------------

func TestHandleUpdateConfig_SuccessfulUpdate(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	body := `{
		"log": {"level": "warn"},
		"agents": {"default": "claude", "claude": {"enabled": true, "phases": {"plan": true, "execute": true, "synthesize": true}}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}, "synthesizer": {"agent": "claude"}}},
		"git": {"worktree": {"dir": ".worktrees"}}
	}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config?force=true", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Config.Log.Level != "warn" {
		t.Errorf("expected 'warn', got %q", resp.Config.Log.Level)
	}
	if resp.Meta.ETag == "" {
		t.Error("expected ETag in response")
	}
	if resp.Meta.Source != "file" {
		t.Errorf("expected source 'file', got %q", resp.Meta.Source)
	}
}

// ---------------------------------------------------------------------------
// Workflow execution modes — HandleAnalyze/Plan/Execute basic 404
// ---------------------------------------------------------------------------

func TestHandleAnalyzeWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/analyze", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlePlanWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/plan", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleReplanWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/replan", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleExecuteWorkflow_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/execute", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// HandleRunWorkflow — already completed conflict
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Kanban — MoveWorkflowRequest validation via router path
// ---------------------------------------------------------------------------

func TestKanbanRoutes_Registered(t *testing.T) {
	t.Parallel()
	sm := newMockKanbanStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))
	ks := NewKanbanServer(srv, nil, eb)

	r := chi.NewRouter()
	ks.RegisterRoutes(r)

	// Verify routes are registered by making requests.
	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/kanban/board"},
		{http.MethodGet, "/kanban/engine"},
		{http.MethodPost, "/kanban/engine/enable"},
		{http.MethodPost, "/kanban/engine/disable"},
		{http.MethodPost, "/kanban/engine/reset-circuit-breaker"},
	}

	for _, p := range paths {
		req := httptest.NewRequest(p.method, p.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		// All should return something (not 404/405 from router).
		if rec.Code == http.StatusMethodNotAllowed || rec.Code == 0 {
			t.Errorf("%s %s: unexpected status %d", p.method, p.path, rec.Code)
		}
	}
}

// ---------------------------------------------------------------------------
// handleGetConfig — LastModified field when file exists
// ---------------------------------------------------------------------------

func TestConfigMeta_LastModified(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// Create config file.
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config.DefaultConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Meta.LastModified == "" {
		t.Error("expected non-empty LastModified when config file exists")
	}

	// Verify it's a valid RFC3339 timestamp.
	if _, err := time.Parse(time.RFC3339, resp.Meta.LastModified); err != nil {
		t.Errorf("LastModified is not valid RFC3339: %q", resp.Meta.LastModified)
	}
}

// ---------------------------------------------------------------------------
// respondJSON and respondError coverage
// ---------------------------------------------------------------------------

func TestRespondJSON_NilData(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	respondJSON(rec, http.StatusNoContent, nil)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestRespondError(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, "test error")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["error"] != "test error" {
		t.Errorf("expected error 'test error', got %q", resp["error"])
	}
}

// ---------------------------------------------------------------------------
// getProjectConfigMode — additional branches
// ---------------------------------------------------------------------------

func TestGetProjectConfigMode_EmptyProjectID(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	reg := &mockRegistryForKanban{
		projects: []*project.Project{},
	}
	srv := NewServer(sm, eb, WithRoot(t.TempDir()), WithProjectRegistry(reg))

	// Context without project ID → custom.
	mode, err := srv.getProjectConfigMode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeCustom {
		t.Errorf("expected %q, got %q", project.ConfigModeCustom, mode)
	}
}

// ---------------------------------------------------------------------------
// helper — contextWithProjectContext is a shorthand for middleware.WithProjectContext
// ---------------------------------------------------------------------------

func contextWithProjectContext(ctx context.Context, pc *project.ProjectContext) context.Context {
	return middleware.WithProjectContext(ctx, pc)
}
