package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// issueTestServer is a convenience wrapper for issue handler tests.
type issueTestServer struct {
	srv *Server
	sm  *mockStateManager
	eb  *events.EventBus
}

// newIssueTestServer creates a Server with a mock state manager, an event bus,
// and an optional config loader.  The server has no project registry / state
// pool, so getProjectStateManager and getProjectConfigLoader fall back to the
// server-level fields.
func newIssueTestServer(t *testing.T, opts ...ServerOption) *issueTestServer {
	t.Helper()

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })

	defaults := []ServerOption{WithLogger(slog.Default())}
	defaults = append(defaults, opts...)

	srv := NewServer(sm, eb, defaults...)
	return &issueTestServer{srv: srv, sm: sm, eb: eb}
}

// addWorkflow inserts a workflow into the mock state manager.
func (its *issueTestServer) addWorkflow(id, reportPath string) {
	wfID := core.WorkflowID(id)
	its.sm.workflows[wfID] = &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: wfID,
			Prompt:     "test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusCompleted,
			CurrentPhase: core.PhaseExecute,
			ReportPath:   reportPath,
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			UpdatedAt:    time.Now(),
		},
	}
}

// configLoaderWithIssues creates a config.Loader that returns a Config with
// the provided IssuesConfig.  It writes a minimal YAML config file into a temp
// directory and returns the loader.
func configLoaderWithIssues(t *testing.T, issuesCfg config.IssuesConfig) *config.Loader {
	t.Helper()

	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	// Build a minimal YAML config that sets the issues section.
	enabledStr := "false"
	if issuesCfg.Enabled {
		enabledStr = "true"
	}
	provider := issuesCfg.Provider
	if provider == "" {
		provider = "github"
	}
	repo := issuesCfg.Repository

	yamlContent := fmt.Sprintf(`
issues:
  enabled: %s
  provider: %s
  repository: %q
`, enabledStr, provider, repo)

	cfgFile := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	loader := config.NewLoader().WithConfigFile(cfgFile)
	return loader
}

// decodeErrorResponse decodes a JSON error response and returns the "error" value.
func decodeErrorResponse(t *testing.T, body *bytes.Buffer) string {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return resp["error"]
}

// ---------------------------------------------------------------------------
// handleGenerateIssues tests
// ---------------------------------------------------------------------------

func TestHandleGenerateIssues_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	// No workflow added -- should get 404.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "nonexistent"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if errMsg != "workflow not found" {
		t.Errorf("unexpected error: %s", errMsg)
	}
}

func TestHandleGenerateIssues_WorkflowLoadError(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.sm.loadErr = fmt.Errorf("database unavailable")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "database unavailable") {
		t.Errorf("expected error to mention database unavailable, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_InvalidRequestBody(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	body := bytes.NewBufferString(`{invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", body)
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "invalid request body") {
		t.Errorf("expected 'invalid request body' error, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_IssuesDisabled(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_NoConfigLoader_IssuesDisabled(t *testing.T) {
	t.Parallel()
	// When no config loader is set the zero-value IssuesConfig has Enabled=false.
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_EmptyReportPath(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	// Workflow with empty report path
	ts.addWorkflow("wf-1", "")

	body, _ := json.Marshal(GenerateIssuesRequest{DryRun: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "no report directory") {
		t.Errorf("expected 'no report directory' error, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_GitLabNotImplemented(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "gitlab"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected %d, got %d: %s", http.StatusNotImplemented, w.Code, w.Body.String())
	}
}

func TestHandleGenerateIssues_UnknownProvider(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "jira"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "unknown provider") {
		t.Errorf("expected 'unknown provider' error, got: %s", errMsg)
	}
}

func TestHandleGenerateIssues_InvalidRepositoryFormat(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "invalid-no-slash"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %s", errMsg)
	}
}

// ---------------------------------------------------------------------------
// handleSaveIssuesFiles tests
// ---------------------------------------------------------------------------

func TestHandleSaveIssuesFiles_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{{Title: "test", Body: "body"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "nonexistent"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleSaveIssuesFiles_WorkflowLoadError(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.sm.loadErr = fmt.Errorf("storage error")

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{{Title: "test", Body: "body"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "storage error") {
		t.Errorf("expected error to mention storage error, got: %s", errMsg)
	}
}

func TestHandleSaveIssuesFiles_InvalidRequestBody(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/files", body)
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "invalid request") {
		t.Errorf("expected 'invalid request' error, got: %s", errMsg)
	}
}

func TestHandleSaveIssuesFiles_EmptyIssues(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	body, _ := json.Marshal(SaveIssuesFilesRequest{Issues: []IssueInput{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "issues are required") {
		t.Errorf("expected 'issues are required' error, got: %s", errMsg)
	}
}

func TestHandleSaveIssuesFiles_IssuesDisabled(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{{Title: "test", Body: "body"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandleSaveIssuesFiles_EmptyReportPath(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "") // empty report path

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{{Title: "test", Body: "body"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "no report directory") {
		t.Errorf("expected 'no report directory' error, got: %s", errMsg)
	}
}

func TestHandleSaveIssuesFiles_SuccessWritesToDisk(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-save")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}

	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-save", reportDir)

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{
			{Title: "Issue One", Body: "Body one", IsMainIssue: false, TaskID: "task-1"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-save/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-save"})
	w := httptest.NewRecorder()

	ts.srv.handleSaveIssuesFiles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp SaveIssuesFilesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}
	if len(resp.Issues) != 1 {
		t.Errorf("expected 1 issue in response, got %d", len(resp.Issues))
	}
	if resp.Issues[0].Title != "Issue One" {
		t.Errorf("expected title 'Issue One', got %q", resp.Issues[0].Title)
	}
}

// ---------------------------------------------------------------------------
// handlePreviewIssues (handleGetIssuesPreview) tests
// ---------------------------------------------------------------------------

func TestHandlePreviewIssues_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/issues/preview", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "nonexistent"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlePreviewIssues_WorkflowLoadError(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.sm.loadErr = fmt.Errorf("io error")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/preview", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlePreviewIssues_IssuesDisabled(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/preview", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandlePreviewIssues_EmptyReportPath(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/preview", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "no report directory") {
		t.Errorf("expected 'no report directory' error, got: %s", errMsg)
	}
}

func TestHandlePreviewIssues_AgentModeNoRegistry(t *testing.T) {
	t.Parallel()
	// Issues enabled with mode=agent but no agentRegistry on server
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	// Set mode to agent in the config -- we need to write a more complete config file.
	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgFile := filepath.Join(cfgDir, "config.yaml")
	yamlContent := `
issues:
  enabled: true
  provider: github
  repository: "owner/repo"
  mode: agent
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	agentModeLoader := config.NewLoader().WithConfigFile(cfgFile)
	ts.srv.configLoader = agentModeLoader

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/preview", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "agent registry not available") {
		t.Errorf("expected 'agent registry not available' error, got: %s", errMsg)
	}
}

func TestHandlePreviewIssues_FastModeForcesDirect(t *testing.T) {
	t.Parallel()
	// When fast=true, the handler should use direct mode (not agent mode).
	// We set mode=agent in config but pass ?fast=true.
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-fast")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir report: %v", err)
	}

	cfgDir := filepath.Join(tmpDir, ".quorum")
	cfgFile := filepath.Join(cfgDir, "config.yaml")
	yamlContent := `
issues:
  enabled: true
  provider: github
  repository: "owner/repo"
  mode: agent
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	loader := config.NewLoader().WithConfigFile(cfgFile)

	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-fast", reportDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-fast/issues/preview?fast=true", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-fast"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	// In direct mode with no report artifacts it may return 200 with 0 previews
	// or 500 if the directory doesn't have the right structure.
	// We mainly verify it doesn't return 500 with "agent registry not available".
	errMsg := ""
	if w.Code != http.StatusOK {
		errMsg = decodeErrorResponse(t, w.Body)
	}
	if strings.Contains(errMsg, "agent registry") {
		t.Errorf("fast mode should not require agent registry, got error: %s", errMsg)
	}
}

// ---------------------------------------------------------------------------
// handleGetIssuesConfig tests
// ---------------------------------------------------------------------------

func TestHandleGetIssuesConfig_NoLoader(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/issues", nil)
	w := httptest.NewRecorder()

	ts.srv.handleGetIssuesConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var cfg config.IssuesConfig
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected issues disabled by default, got enabled=true")
	}
}

func TestHandleGetIssuesConfig_WithLoader(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/issues", nil)
	w := httptest.NewRecorder()

	ts.srv.handleGetIssuesConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var cfg config.IssuesConfig
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !cfg.Enabled {
		t.Errorf("expected issues enabled, got enabled=false")
	}
}

// ---------------------------------------------------------------------------
// handleCreateSingleIssue tests
// ---------------------------------------------------------------------------

func TestHandleCreateSingleIssue_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	body, _ := json.Marshal(CreateSingleIssueRequest{Title: "Bug", Body: "description"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/single", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "nonexistent"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleCreateSingleIssue_InvalidBody(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	body := bytes.NewBufferString(`{bad`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/single", body)
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandleCreateSingleIssue_MissingTitle(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	body, _ := json.Marshal(CreateSingleIssueRequest{Title: "", Body: "description"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/single", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "title is required") {
		t.Errorf("expected 'title is required' error, got: %s", errMsg)
	}
}

func TestHandleCreateSingleIssue_IssuesDisabled(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	body, _ := json.Marshal(CreateSingleIssueRequest{Title: "Bug", Body: "desc"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/single", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandleCreateSingleIssue_EmptyReportPath(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "github", Repository: "owner/repo"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "")

	body, _ := json.Marshal(CreateSingleIssueRequest{Title: "Bug", Body: "desc"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/single", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "no report directory") {
		t.Errorf("expected 'no report directory' error, got: %s", errMsg)
	}
}

func TestHandleCreateSingleIssue_GitLabNotImplemented(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "gitlab"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-1", "/some/report")

	body, _ := json.Marshal(CreateSingleIssueRequest{Title: "Bug", Body: "desc"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/single", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected %d, got %d: %s", http.StatusNotImplemented, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleListDrafts tests
// ---------------------------------------------------------------------------

func TestHandleListDrafts_EmptyDrafts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Use a minimal loader
	ts := newIssueTestServer(t, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/drafts", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleListDrafts(w, req)

	// It should succeed -- even with no drafts directory it returns empty list or 500.
	// The handler calls ReadAllDrafts which may fail gracefully.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	if w.Code == http.StatusOK {
		var resp DraftsListResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.WorkflowID != "wf-1" {
			t.Errorf("expected workflow_id 'wf-1', got %q", resp.WorkflowID)
		}
	}
}

// ---------------------------------------------------------------------------
// handleEditDraft tests
// ---------------------------------------------------------------------------

func TestHandleEditDraft_InvalidBody(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	body := bytes.NewBufferString(`{bad json`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-1/issues/drafts/task-1", body)
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1", "taskId": "task-1"})
	w := httptest.NewRecorder()

	ts.srv.handleEditDraft(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handlePublishDrafts tests
// ---------------------------------------------------------------------------

func TestHandlePublishDrafts_InvalidBody(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	body := bytes.NewBufferString(`{bad json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/publish", body)
	req.Header.Set("Content-Type", "application/json")
	// Set ContentLength to trigger body parsing
	req.ContentLength = int64(len(`{bad json`))
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandlePublishDrafts_IssuesDisabled(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/publish", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

func TestHandlePublishDrafts_GitLabNotImplemented(t *testing.T) {
	t.Parallel()
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: true, Provider: "gitlab"})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/publish", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected %d, got %d: %s", http.StatusNotImplemented, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleIssuesStatus tests
// ---------------------------------------------------------------------------

func TestHandleIssuesStatus_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ts := newIssueTestServer(t, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-1/issues/status", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleIssuesStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp IssuesStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WorkflowID != "wf-1" {
		t.Errorf("expected workflow_id 'wf-1', got %q", resp.WorkflowID)
	}
	if resp.HasDrafts {
		t.Error("expected has_drafts=false for non-existent drafts directory")
	}
	if resp.DraftCount != 0 {
		t.Errorf("expected draft_count=0, got %d", resp.DraftCount)
	}
	if resp.HasPublished {
		t.Error("expected has_published=false")
	}
	if resp.PublishedCount != 0 {
		t.Errorf("expected published_count=0, got %d", resp.PublishedCount)
	}
}

// ---------------------------------------------------------------------------
// createIssueClient / writeIssueClientError tests
// ---------------------------------------------------------------------------

func TestCreateIssueClient_GitLabNotImplemented(t *testing.T) {
	t.Parallel()
	_, err := createIssueClient(config.IssuesConfig{Provider: "gitlab"})
	if err == nil {
		t.Fatal("expected error for gitlab provider")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

func TestCreateIssueClient_UnknownProvider(t *testing.T) {
	t.Parallel()
	_, err := createIssueClient(config.IssuesConfig{Provider: "jira"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' error, got: %v", err)
	}
}

func TestCreateIssueClient_InvalidRepositoryFormat(t *testing.T) {
	t.Parallel()
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "no-slash"})
	if err == nil {
		t.Fatal("expected error for invalid repository format")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestCreateIssueClient_EmptyOwnerInRepository(t *testing.T) {
	t.Parallel()
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "/repo"})
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestCreateIssueClient_EmptyRepoInRepository(t *testing.T) {
	t.Parallel()
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "owner/"})
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestWriteIssueClientError_GitLabNotImplemented(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("GitLab issue generation not yet implemented"))

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected %d, got %d", http.StatusNotImplemented, w.Code)
	}
}

func TestWriteIssueClientError_UnknownProvider(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("unknown provider: jira"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWriteIssueClientError_InvalidRepositoryFormat(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("invalid repository format \"bad\", expected owner/repo"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWriteIssueClientError_AuthError_GHNotAuthenticated(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, &core.DomainError{
		Code:    "GH_NOT_AUTHENTICATED",
		Message: "not authenticated",
	})

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWriteIssueClientError_AuthError_StringMatch(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("gh auth login required"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWriteIssueClientError_AuthError_NotAuthenticated(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("GitHub CLI not authenticated"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWriteIssueClientError_GenericError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("some random error"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// ---------------------------------------------------------------------------
// Integration-like test: handleGenerateIssues via full router
// ---------------------------------------------------------------------------

func TestHandleGenerateIssues_ViaRouter_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/", nil)
	w := httptest.NewRecorder()

	ts.srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleSaveIssuesFiles_ViaRouter_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	body, _ := json.Marshal(SaveIssuesFilesRequest{
		Issues: []IssueInput{{Title: "test", Body: "body"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/files", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ts.srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandlePreviewIssues_ViaRouter_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/nonexistent/issues/preview", nil)
	w := httptest.NewRecorder()

	ts.srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleGenerateIssues: empty body defaults
// ---------------------------------------------------------------------------

func TestHandleGenerateIssues_EmptyBodyDefaultsApplied(t *testing.T) {
	t.Parallel()
	// When r.Body is nil and ContentLength is 0, the handler uses defaults:
	// CreateMainIssue=true, CreateSubIssues=true, LinkIssues=true.
	// We verify this doesn't panic and gets past body parsing to the
	// config check (issues disabled).
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-1", "/some/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-1/issues/", nil)
	req.ContentLength = 0
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-1"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	// Should fail at "issues disabled" since we have no config loader.
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "disabled") {
		t.Errorf("expected error about disabled issues, got: %s", errMsg)
	}
}

// ---------------------------------------------------------------------------
// Verify context fallback
// ---------------------------------------------------------------------------

func TestIssueHandlers_UseServerFallbackStateManager(t *testing.T) {
	t.Parallel()
	// When there is no project context in the request, the handler should use
	// the server's state manager.  We add a workflow to the server's mock SM
	// and verify it is found.
	loader := configLoaderWithIssues(t, config.IssuesConfig{Enabled: false})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-ctx", "/report")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-ctx/issues/", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-ctx"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	// It should find the workflow (from server SM fallback) and fail at
	// "issues disabled".
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d (issues disabled), got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Verify request body with context cancellation doesn't panic
// ---------------------------------------------------------------------------

func TestHandleGenerateIssues_CancelledContext(t *testing.T) {
	t.Parallel()
	ts := newIssueTestServer(t)
	ts.addWorkflow("wf-cancel", "/report")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-cancel/issues/", nil)
	req = req.WithContext(ctx)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-cancel"})
	w := httptest.NewRecorder()

	// Should not panic even with cancelled context
	ts.srv.handleGenerateIssues(w, req)

	// The exact status code depends on where the cancellation is caught,
	// but it should not be 0 (no response written).
	if w.Code == 0 {
		t.Error("expected a response to be written even with cancelled context")
	}
}

// ===========================================================================
// Mock IssueClient for testing handlers that call createIssueClient
// ===========================================================================

// mockIssueClient implements core.IssueClient for testing.
type mockIssueClient struct {
	createFunc func(ctx context.Context, opts core.CreateIssueOptions) (*core.Issue, error)
	linkFunc   func(ctx context.Context, parent, child int) error
	nextNumber int // auto-incrementing issue number
}

func (m *mockIssueClient) CreateIssue(_ context.Context, opts core.CreateIssueOptions) (*core.Issue, error) {
	if m.createFunc != nil {
		return m.createFunc(context.Background(), opts)
	}
	m.nextNumber++
	return &core.Issue{
		Number: m.nextNumber,
		Title:  opts.Title,
		Body:   opts.Body,
		URL:    fmt.Sprintf("https://github.com/owner/repo/issues/%d", m.nextNumber),
		State:  "open",
		Labels: opts.Labels,
	}, nil
}

func (m *mockIssueClient) UpdateIssue(_ context.Context, _ int, _, _ string) error { return nil }
func (m *mockIssueClient) CloseIssue(_ context.Context, _ int) error              { return nil }
func (m *mockIssueClient) AddIssueComment(_ context.Context, _ int, _ string) error {
	return nil
}
func (m *mockIssueClient) GetIssue(_ context.Context, _ int) (*core.Issue, error) {
	return nil, nil
}
func (m *mockIssueClient) LinkIssues(_ context.Context, parent, child int) error {
	if m.linkFunc != nil {
		return m.linkFunc(context.Background(), parent, child)
	}
	return nil
}

// swapCreateIssueClient replaces createIssueClient for testing and restores it on cleanup.
func swapCreateIssueClient(t *testing.T, client core.IssueClient) {
	t.Helper()
	orig := createIssueClient
	createIssueClient = func(_ config.IssuesConfig) (core.IssueClient, error) {
		return client, nil
	}
	t.Cleanup(func() { createIssueClient = orig })
}

// testProjectContext implements middleware.ProjectContext for test requests.
type testProjectContext struct {
	id   string
	root string
}

func (t *testProjectContext) ProjectID() string  { return t.id }
func (t *testProjectContext) ProjectRoot() string { return t.root }
func (t *testProjectContext) IsClosed() bool      { return false }
func (t *testProjectContext) Touch()              {}

// withProjectRoot injects a ProjectContext into the request so that
// getProjectRootPath returns the given root.
func withProjectRoot(req *http.Request, root string) *http.Request {
	ctx := middleware.WithProjectContext(req.Context(), &testProjectContext{
		id:   "test-project",
		root: root,
	})
	return req.WithContext(ctx)
}

// configLoaderFull creates a config.Loader with extended IssuesConfig fields
// (enabled, provider, repo, mode, labels, assignees, timeout).
func configLoaderFull(t *testing.T, issuesCfg config.IssuesConfig) *config.Loader {
	t.Helper()

	tmpDir := t.TempDir()
	cfgDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	enabledStr := "false"
	if issuesCfg.Enabled {
		enabledStr = "true"
	}
	provider := issuesCfg.Provider
	if provider == "" {
		provider = "github"
	}
	mode := issuesCfg.Mode
	if mode == "" {
		mode = "direct"
	}
	timeout := issuesCfg.Timeout

	var labelsYAML, assigneesYAML string
	if len(issuesCfg.Labels) > 0 {
		parts := make([]string, len(issuesCfg.Labels))
		for i, l := range issuesCfg.Labels {
			parts[i] = fmt.Sprintf("    - %q", l)
		}
		labelsYAML = "  labels:\n" + strings.Join(parts, "\n") + "\n"
	}
	if len(issuesCfg.Assignees) > 0 {
		parts := make([]string, len(issuesCfg.Assignees))
		for i, a := range issuesCfg.Assignees {
			parts[i] = fmt.Sprintf("    - %q", a)
		}
		assigneesYAML = "  assignees:\n" + strings.Join(parts, "\n") + "\n"
	}

	var timeoutLine string
	if timeout != "" {
		timeoutLine = fmt.Sprintf("  timeout: %q\n", timeout)
	}

	yamlContent := fmt.Sprintf(`issues:
  enabled: %s
  provider: %s
  repository: %q
  mode: %s
%s%s%s`, enabledStr, provider, issuesCfg.Repository, mode, labelsYAML, assigneesYAML, timeoutLine)

	cfgFile := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return config.NewLoader().WithConfigFile(cfgFile)
}

// writeDraftFile creates a markdown draft file with YAML frontmatter in the
// expected draft directory layout: {projectRoot}/.quorum/issues/{workflowID}/draft/{filename}
func writeDraftFile(t *testing.T, projectRoot, workflowID, filename string, fm issues.DraftFrontmatter, body string) {
	t.Helper()
	draftDir := filepath.Join(projectRoot, ".quorum", "issues", workflowID, "draft")
	if err := os.MkdirAll(draftDir, 0o755); err != nil {
		t.Fatalf("mkdir draft dir: %v", err)
	}

	var labelsYAML string
	if len(fm.Labels) > 0 {
		parts := make([]string, len(fm.Labels))
		for i, l := range fm.Labels {
			parts[i] = fmt.Sprintf("  - %q", l)
		}
		labelsYAML = "labels:\n" + strings.Join(parts, "\n")
	} else {
		labelsYAML = "labels: []"
	}

	var assigneesYAML string
	if len(fm.Assignees) > 0 {
		parts := make([]string, len(fm.Assignees))
		for i, a := range fm.Assignees {
			parts[i] = fmt.Sprintf("  - %q", a)
		}
		assigneesYAML = "assignees:\n" + strings.Join(parts, "\n")
	} else {
		assigneesYAML = "assignees: []"
	}

	content := fmt.Sprintf(`---
title: %q
%s
%s
is_main_issue: %v
task_id: %q
status: %q
---
%s
`, fm.Title, labelsYAML, assigneesYAML, fm.IsMainIssue, fm.TaskID, fm.Status, body)

	filePath := filepath.Join(draftDir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write draft file: %v", err)
	}
}

// ===========================================================================
// Draft handler tests (no mock client needed)
// ===========================================================================

func TestHandleListDrafts_WithDrafts(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create draft files
	writeDraftFile(t, projectRoot, "wf-drafts", "01-setup-auth.md", issues.DraftFrontmatter{
		Title:       "Setup Authentication",
		Labels:      []string{"enhancement"},
		Assignees:   []string{"alice"},
		IsMainIssue: false,
		TaskID:      "task-1",
		Status:      "draft",
	}, "Implement OAuth2 flow")

	writeDraftFile(t, projectRoot, "wf-drafts", "02-add-tests.md", issues.DraftFrontmatter{
		Title:       "Add Unit Tests",
		Labels:      []string{"testing"},
		Assignees:   []string{"bob"},
		IsMainIssue: false,
		TaskID:      "task-2",
		Status:      "draft",
	}, "Write tests for auth module")

	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-drafts/issues/drafts", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-drafts"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleListDrafts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp DraftsListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WorkflowID != "wf-drafts" {
		t.Errorf("expected workflow_id 'wf-drafts', got %q", resp.WorkflowID)
	}
	if len(resp.Drafts) != 2 {
		t.Fatalf("expected 2 drafts, got %d", len(resp.Drafts))
	}

	// Verify first draft
	found := false
	for _, d := range resp.Drafts {
		if d.TaskID == "task-1" {
			found = true
			if d.Title != "Setup Authentication" {
				t.Errorf("expected title 'Setup Authentication', got %q", d.Title)
			}
			if len(d.Labels) != 1 || d.Labels[0] != "enhancement" {
				t.Errorf("expected labels [enhancement], got %v", d.Labels)
			}
			if d.IsMainIssue {
				t.Error("expected is_main_issue=false")
			}
		}
	}
	if !found {
		t.Error("task-1 draft not found in response")
	}
}

func TestHandleEditDraft_Success(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	writeDraftFile(t, projectRoot, "wf-edit", "01-task.md", issues.DraftFrontmatter{
		Title:       "Original Title",
		Labels:      []string{"bug"},
		Assignees:   []string{"alice"},
		IsMainIssue: false,
		TaskID:      "task-1",
		Status:      "draft",
	}, "Original body")

	ts := newIssueTestServer(t)

	newTitle := "Updated Title"
	newBody := "Updated body content"
	newLabels := []string{"feature", "priority-high"}
	updateReq := DraftUpdateRequest{
		Title:  &newTitle,
		Body:   &newBody,
		Labels: &newLabels,
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-edit/issues/drafts/task-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-edit", "taskId": "task-1"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleEditDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp DraftResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", resp.Title)
	}
	if resp.Body != "Updated body content" {
		t.Errorf("expected body 'Updated body content', got %q", resp.Body)
	}
	if len(resp.Labels) != 2 || resp.Labels[0] != "feature" {
		t.Errorf("expected labels [feature, priority-high], got %v", resp.Labels)
	}
	// Assignees should be preserved since we didn't update them
	if len(resp.Assignees) != 1 || resp.Assignees[0] != "alice" {
		t.Errorf("expected assignees [alice] (preserved), got %v", resp.Assignees)
	}
}

func TestHandleEditDraft_NotFound(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a draft with task-1, but try to edit task-99
	writeDraftFile(t, projectRoot, "wf-edit-nf", "01-task.md", issues.DraftFrontmatter{
		Title:  "Some Task",
		TaskID: "task-1",
		Status: "draft",
	}, "body")

	ts := newIssueTestServer(t)

	newTitle := "New Title"
	updateReq := DraftUpdateRequest{Title: &newTitle}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-edit-nf/issues/drafts/task-99", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-edit-nf", "taskId": "task-99"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleEditDraft(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "draft not found") {
		t.Errorf("expected 'draft not found' error, got: %s", errMsg)
	}
}

func TestHandleEditDraft_PartialUpdate(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	writeDraftFile(t, projectRoot, "wf-partial", "01-task.md", issues.DraftFrontmatter{
		Title:       "Original Title",
		Labels:      []string{"bug"},
		Assignees:   []string{"alice"},
		IsMainIssue: false,
		TaskID:      "task-1",
		Status:      "draft",
	}, "Original body")

	ts := newIssueTestServer(t)

	// Only update title; body, labels, and assignees should be preserved
	newTitle := "Only Title Changed"
	updateReq := DraftUpdateRequest{Title: &newTitle}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-partial/issues/drafts/task-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-partial", "taskId": "task-1"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleEditDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp DraftResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Title != "Only Title Changed" {
		t.Errorf("expected title 'Only Title Changed', got %q", resp.Title)
	}
	if resp.Body != "Original body" {
		t.Errorf("expected body preserved as 'Original body', got %q", resp.Body)
	}
	if len(resp.Labels) != 1 || resp.Labels[0] != "bug" {
		t.Errorf("expected labels [bug] preserved, got %v", resp.Labels)
	}
	if len(resp.Assignees) != 1 || resp.Assignees[0] != "alice" {
		t.Errorf("expected assignees [alice] preserved, got %v", resp.Assignees)
	}
}

func TestHandleEditDraft_MainIssueByTaskIdMain(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a main issue draft
	writeDraftFile(t, projectRoot, "wf-main-edit", "00-consolidated.md", issues.DraftFrontmatter{
		Title:       "Main Issue",
		Labels:      []string{"epic"},
		IsMainIssue: true,
		TaskID:      "main",
		Status:      "draft",
	}, "Main issue body")

	ts := newIssueTestServer(t)

	newTitle := "Updated Main"
	updateReq := DraftUpdateRequest{Title: &newTitle}
	body, _ := json.Marshal(updateReq)

	// Use taskId "main" to target the main issue
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workflows/wf-main-edit/issues/drafts/main", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-main-edit", "taskId": "main"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleEditDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp DraftResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Title != "Updated Main" {
		t.Errorf("expected title 'Updated Main', got %q", resp.Title)
	}
	if !resp.IsMainIssue {
		t.Error("expected is_main_issue=true")
	}
}

// ===========================================================================
// handleGenerateIssues with mock client (happy paths)
// ===========================================================================

func TestHandleGenerateIssues_DirectModeDryRun(t *testing.T) {
	t.Parallel()
	// Set up a report directory with task files for direct mode
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-direct")
	tasksDir := filepath.Join(reportDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create a task file
	taskContent := `# Task: Setup Authentication

**Task ID**: task-1
**Assigned Agent**: claude
**Complexity**: medium
**Dependencies**: None

---

## Description

Implement OAuth2 authentication flow.
`
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-setup-auth.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	// Create consolidated analysis
	analysisDir := filepath.Join(reportDir, "analyze-phase")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(analysisDir, "consolidated.md"), []byte("# Consolidated Analysis\n\nSummary of findings."), 0o644); err != nil {
		t.Fatalf("write consolidated: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-direct", reportDir)

	reqBody, _ := json.Marshal(GenerateIssuesRequest{
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-direct/issues/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-direct"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateIssuesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if len(resp.PreviewIssues) == 0 {
		t.Error("expected at least one preview issue")
	}
	if !strings.Contains(resp.Message, "Preview:") {
		t.Errorf("expected preview message, got: %s", resp.Message)
	}
}

func TestHandleGenerateIssues_FrontendIssuesInput(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Use tmpDir as project root so WriteIssuesToDisk can write draft files
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-frontend")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-frontend", reportDir)

	reqBody, _ := json.Marshal(GenerateIssuesRequest{
		DryRun:          true,
		CreateMainIssue: true,
		CreateSubIssues: true,
		Issues: []IssueInput{
			{Title: "Main Issue", Body: "Main body", IsMainIssue: true, TaskID: "main"},
			{Title: "Sub Issue 1", Body: "Sub body 1", IsMainIssue: false, TaskID: "task-1"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-frontend/issues/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-frontend"})
	// Inject project root so WriteIssuesToDisk resolves the draft directory
	req = withProjectRoot(req, tmpDir)
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateIssuesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if len(resp.PreviewIssues) != 2 {
		t.Errorf("expected 2 preview issues, got %d", len(resp.PreviewIssues))
	}
}

func TestHandleGenerateIssues_LabelAssigneeDefaults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-defaults")
	tasksDir := filepath.Join(reportDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	taskContent := "# Task: Fix Bug\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\nFix the bug.\n"
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-fix-bug.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
		Labels:     []string{"quorum-ai", "auto-generated"},
		Assignees:  []string{"team-lead"},
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-defaults", reportDir)

	reqBody, _ := json.Marshal(GenerateIssuesRequest{
		DryRun:          true,
		CreateSubIssues: true,
		// No Labels/Assignees in request => should use config defaults
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-defaults/issues/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-defaults"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateIssuesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	// Verify that config labels were applied to the preview
	for _, preview := range resp.PreviewIssues {
		if !preview.IsMainIssue {
			foundQuorum := false
			for _, l := range preview.Labels {
				if l == "quorum-ai" {
					foundQuorum = true
				}
			}
			if !foundQuorum {
				t.Errorf("expected config label 'quorum-ai' in preview labels, got %v", preview.Labels)
			}
		}
	}
}

func TestHandleGenerateIssues_NonDryRunCreatesIssues(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-create")
	tasksDir := filepath.Join(reportDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	taskContent := "# Task: Add Feature\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: medium\n**Dependencies**: None\n\n---\n\nAdd the new feature.\n"
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-add-feature.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	analysisDir := filepath.Join(reportDir, "analyze-phase")
	if err := os.MkdirAll(analysisDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(analysisDir, "consolidated.md"), []byte("# Analysis\n\nConsolidated summary."), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-create", reportDir)

	reqBody, _ := json.Marshal(GenerateIssuesRequest{
		DryRun:          false,
		CreateMainIssue: true,
		CreateSubIssues: true,
		LinkIssues:      true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-create/issues/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-create"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateIssuesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !strings.Contains(resp.Message, "Created") {
		t.Errorf("expected 'Created' in message, got: %s", resp.Message)
	}
	// We should have a main issue and at least one sub-issue
	if resp.MainIssue == nil {
		t.Error("expected main_issue in response")
	}
	if len(resp.SubIssues) == 0 {
		t.Error("expected at least one sub-issue")
	}
}

// ===========================================================================
// handlePreviewIssues with mock client (happy paths)
// ===========================================================================

func TestHandlePreviewIssues_DirectModeSuccess(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-preview")
	tasksDir := filepath.Join(reportDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	taskContent := "# Task: Optimize DB\n\n**Task ID**: task-1\n**Assigned Agent**: gemini\n**Complexity**: high\n**Dependencies**: None\n\n---\n\nOptimize database queries.\n"
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-optimize-db.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-preview", reportDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-preview/issues/preview?fast=true", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-preview"})
	w := httptest.NewRecorder()

	ts.srv.handlePreviewIssues(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp GenerateIssuesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.AIUsed {
		t.Error("expected ai_used=false in direct mode")
	}
	if len(resp.PreviewIssues) == 0 {
		t.Error("expected at least one preview issue")
	}
	if !strings.Contains(resp.Message, "direct mode") {
		t.Errorf("expected 'direct mode' in message, got: %s", resp.Message)
	}
}

// ===========================================================================
// handleCreateSingleIssue with mock client (happy paths)
// ===========================================================================

func TestHandleCreateSingleIssue_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-single")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-single", reportDir)

	reqBody, _ := json.Marshal(CreateSingleIssueRequest{
		Title:       "Fix Login Bug",
		Body:        "Users cannot login with SSO.",
		Labels:      []string{"bug", "priority-high"},
		Assignees:   []string{"dev1"},
		IsMainIssue: false,
		TaskID:      "task-1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-single/issues/single", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-single"})
	req = withProjectRoot(req, tmpDir)
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp CreateSingleIssueResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Issue.Number == 0 {
		t.Error("expected non-zero issue number")
	}
	if resp.Issue.URL == "" {
		t.Error("expected non-empty issue URL")
	}
	if resp.Issue.State != "open" {
		t.Errorf("expected state 'open', got %q", resp.Issue.State)
	}
	if resp.Issue.Title != "Fix Login Bug" {
		t.Errorf("expected title 'Fix Login Bug', got %q", resp.Issue.Title)
	}
}

func TestHandleCreateSingleIssue_DefaultLabelsAssignees(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-single-def")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var capturedLabels []string
	var capturedAssignees []string
	mockClient := &mockIssueClient{
		createFunc: func(_ context.Context, opts core.CreateIssueOptions) (*core.Issue, error) {
			capturedLabels = opts.Labels
			capturedAssignees = opts.Assignees
			return &core.Issue{
				Number: 42,
				Title:  opts.Title,
				URL:    "https://github.com/owner/repo/issues/42",
				State:  "open",
				Labels: opts.Labels,
			}, nil
		},
	}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
		Labels:     []string{"default-label"},
		Assignees:  []string{"default-assignee"},
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-single-def", reportDir)

	// Request with NO labels/assignees => should get config defaults
	reqBody, _ := json.Marshal(CreateSingleIssueRequest{
		Title:  "Use Defaults",
		Body:   "Body",
		TaskID: "task-1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-single-def/issues/single", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-single-def"})
	req = withProjectRoot(req, tmpDir)
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	if len(capturedLabels) == 0 || capturedLabels[0] != "default-label" {
		t.Errorf("expected config default labels [default-label], got %v", capturedLabels)
	}
	if len(capturedAssignees) == 0 || capturedAssignees[0] != "default-assignee" {
		t.Errorf("expected config default assignees [default-assignee], got %v", capturedAssignees)
	}
}

// ===========================================================================
// handlePublishDrafts with mock client (happy paths)
// ===========================================================================

func TestHandlePublishDrafts_Success(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create draft files
	writeDraftFile(t, projectRoot, "wf-publish", "00-consolidated.md", issues.DraftFrontmatter{
		Title:       "Main Issue",
		Labels:      []string{"epic"},
		IsMainIssue: true,
		TaskID:      "main",
		Status:      "draft",
	}, "Main issue body")

	writeDraftFile(t, projectRoot, "wf-publish", "01-task.md", issues.DraftFrontmatter{
		Title:       "Sub Issue",
		Labels:      []string{"enhancement"},
		IsMainIssue: false,
		TaskID:      "task-1",
		Status:      "draft",
	}, "Sub issue body")

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	reqBody, _ := json.Marshal(PublishRequest{
		DryRun:     false,
		LinkIssues: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-publish/issues/publish", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(reqBody))
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-publish"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp PublishResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WorkflowID != "wf-publish" {
		t.Errorf("expected workflow_id 'wf-publish', got %q", resp.WorkflowID)
	}
	if len(resp.Published) == 0 {
		t.Error("expected at least one published record")
	}
	// Check that we have both main and sub-issue
	hasMain := false
	hasSub := false
	for _, r := range resp.Published {
		if r.IsMain {
			hasMain = true
			if r.IssueNumber == 0 {
				t.Error("expected non-zero issue number for main issue")
			}
		} else {
			hasSub = true
		}
	}
	if !hasMain {
		t.Error("expected a main issue in published records")
	}
	if !hasSub {
		t.Error("expected a sub-issue in published records")
	}
}

func TestHandlePublishDrafts_FilterByTaskIDs(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create 3 draft files
	writeDraftFile(t, projectRoot, "wf-filter", "01-task-a.md", issues.DraftFrontmatter{
		Title:  "Task A",
		TaskID: "task-a",
		Status: "draft",
	}, "Body A")

	writeDraftFile(t, projectRoot, "wf-filter", "02-task-b.md", issues.DraftFrontmatter{
		Title:  "Task B",
		TaskID: "task-b",
		Status: "draft",
	}, "Body B")

	writeDraftFile(t, projectRoot, "wf-filter", "03-task-c.md", issues.DraftFrontmatter{
		Title:  "Task C",
		TaskID: "task-c",
		Status: "draft",
	}, "Body C")

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	// Only publish task-a and task-c
	reqBody, _ := json.Marshal(PublishRequest{
		DryRun:  false,
		TaskIDs: []string{"task-a", "task-c"},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-filter/issues/publish", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(reqBody))
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-filter"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp PublishResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should have 2 published records (task-a and task-c), not 3
	if len(resp.Published) != 2 {
		t.Errorf("expected 2 published records (filtered), got %d", len(resp.Published))
	}
}

func TestHandlePublishDrafts_NoDrafts(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	// No draft files created

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-empty/issues/publish", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-empty"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	errMsg := decodeErrorResponse(t, w.Body)
	if !strings.Contains(errMsg, "no drafts") {
		t.Errorf("expected 'no drafts' error, got: %s", errMsg)
	}
}

func TestHandlePublishDrafts_DryRunMode(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	writeDraftFile(t, projectRoot, "wf-dry-pub", "01-task.md", issues.DraftFrontmatter{
		Title:  "Dry Run Task",
		TaskID: "task-1",
		Status: "draft",
	}, "Body for dry run")

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))

	reqBody, _ := json.Marshal(PublishRequest{
		DryRun: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-dry-pub/issues/publish", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(reqBody))
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-dry-pub"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handlePublishDrafts(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp PublishResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Dry run should still return records but with no issue numbers
	if len(resp.Published) == 0 {
		t.Error("expected at least one published record in dry-run mode")
	}
	for _, r := range resp.Published {
		if r.IssueNumber != 0 {
			t.Errorf("dry-run should not have issue numbers, got %d", r.IssueNumber)
		}
	}
}

// ===========================================================================
// handleIssuesStatus with draft/published data
// ===========================================================================

func TestHandleIssuesStatus_WithDrafts(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create some draft files
	writeDraftFile(t, projectRoot, "wf-status", "01-task.md", issues.DraftFrontmatter{
		Title:  "Task 1",
		TaskID: "task-1",
		Status: "draft",
	}, "Body")
	writeDraftFile(t, projectRoot, "wf-status", "02-task.md", issues.DraftFrontmatter{
		Title:  "Task 2",
		TaskID: "task-2",
		Status: "draft",
	}, "Body 2")

	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/wf-status/issues/status", nil)
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-status"})
	req = withProjectRoot(req, projectRoot)
	w := httptest.NewRecorder()

	ts.srv.handleIssuesStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp IssuesStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.HasDrafts {
		t.Error("expected has_drafts=true")
	}
	if resp.DraftCount != 2 {
		t.Errorf("expected draft_count=2, got %d", resp.DraftCount)
	}
}

// ===========================================================================
// handleGenerateIssues: timeout config applied
// ===========================================================================

func TestHandleGenerateIssues_TimeoutFromConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-timeout")
	tasksDir := filepath.Join(reportDir, "plan-phase", "tasks")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	taskContent := "# Task: Quick Fix\n\n**Task ID**: task-1\n**Assigned Agent**: claude\n**Complexity**: low\n**Dependencies**: None\n\n---\n\nFix it.\n"
	if err := os.WriteFile(filepath.Join(tasksDir, "task-1-quick-fix.md"), []byte(taskContent), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
		Mode:       "direct",
		Timeout:    "5m",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-timeout", reportDir)

	reqBody, _ := json.Marshal(GenerateIssuesRequest{
		DryRun:          true,
		CreateSubIssues: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-timeout/issues/", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-timeout"})
	w := httptest.NewRecorder()

	ts.srv.handleGenerateIssues(w, req)

	// Should succeed even with a timeout config -- the timeout applies to generation
	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

// ===========================================================================
// handleCreateSingleIssue: main issue creation
// ===========================================================================

func TestHandleCreateSingleIssue_MainIssue(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-main-single")
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mockClient := &mockIssueClient{}
	swapCreateIssueClient(t, mockClient)

	loader := configLoaderFull(t, config.IssuesConfig{
		Enabled:    true,
		Provider:   "github",
		Repository: "owner/repo",
	})
	ts := newIssueTestServer(t, WithConfigLoader(loader))
	ts.addWorkflow("wf-main-single", reportDir)

	reqBody, _ := json.Marshal(CreateSingleIssueRequest{
		Title:       "Epic: Project Setup",
		Body:        "Parent issue for all setup tasks.",
		IsMainIssue: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/wf-main-single/issues/single", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = addChiURLParams(req, map[string]string{"workflowID": "wf-main-single"})
	req = withProjectRoot(req, tmpDir)
	w := httptest.NewRecorder()

	ts.srv.handleCreateSingleIssue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp CreateSingleIssueResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Issue.Number == 0 {
		t.Error("expected non-zero issue number")
	}
}
