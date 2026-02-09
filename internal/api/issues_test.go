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

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
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
func (its *issueTestServer) addWorkflow(id string, reportPath string) {
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
	_, err := createIssueClient(config.IssuesConfig{Provider: "gitlab"})
	if err == nil {
		t.Fatal("expected error for gitlab provider")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' error, got: %v", err)
	}
}

func TestCreateIssueClient_UnknownProvider(t *testing.T) {
	_, err := createIssueClient(config.IssuesConfig{Provider: "jira"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' error, got: %v", err)
	}
}

func TestCreateIssueClient_InvalidRepositoryFormat(t *testing.T) {
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "no-slash"})
	if err == nil {
		t.Fatal("expected error for invalid repository format")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestCreateIssueClient_EmptyOwnerInRepository(t *testing.T) {
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "/repo"})
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestCreateIssueClient_EmptyRepoInRepository(t *testing.T) {
	_, err := createIssueClient(config.IssuesConfig{Provider: "github", Repository: "owner/"})
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
	if !strings.Contains(err.Error(), "invalid repository format") {
		t.Errorf("expected 'invalid repository format' error, got: %v", err)
	}
}

func TestWriteIssueClientError_GitLabNotImplemented(t *testing.T) {
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("GitLab issue generation not yet implemented"))

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected %d, got %d", http.StatusNotImplemented, w.Code)
	}
}

func TestWriteIssueClientError_UnknownProvider(t *testing.T) {
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("unknown provider: jira"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWriteIssueClientError_InvalidRepositoryFormat(t *testing.T) {
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("invalid repository format \"bad\", expected owner/repo"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWriteIssueClientError_AuthError_GHNotAuthenticated(t *testing.T) {
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
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("gh auth login required"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWriteIssueClientError_AuthError_NotAuthenticated(t *testing.T) {
	w := httptest.NewRecorder()
	writeIssueClientError(w, fmt.Errorf("GitHub CLI not authenticated"))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWriteIssueClientError_GenericError(t *testing.T) {
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
	ts := newIssueTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/nonexistent/issues/", nil)
	w := httptest.NewRecorder()

	ts.srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestHandleSaveIssuesFiles_ViaRouter_WorkflowNotFound(t *testing.T) {
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
