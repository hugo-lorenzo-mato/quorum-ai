package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	// Set workflow status to running to simulate already running state
	sm.workflows["wf-double"].Status = core.WorkflowStatusRunning

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
	if resp["error"] != "workflow is already running" {
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

func TestWorkflowConfig_IsSingleAgentMode(t *testing.T) {
	tests := []struct {
		name     string
		config   *WorkflowConfig
		expected bool
	}{
		{
			name:     "nil config returns false",
			config:   nil,
			expected: false,
		},
		{
			name:     "empty execution_mode returns false",
			config:   &WorkflowConfig{},
			expected: false,
		},
		{
			name:     "multi_agent returns false",
			config:   &WorkflowConfig{ExecutionMode: "multi_agent"},
			expected: false,
		},
		{
			name:     "single_agent returns true",
			config:   &WorkflowConfig{ExecutionMode: "single_agent"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsSingleAgentMode(); got != tt.expected {
				t.Errorf("IsSingleAgentMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkflowConfig_GetExecutionMode(t *testing.T) {
	tests := []struct {
		name     string
		config   *WorkflowConfig
		expected string
	}{
		{
			name:     "nil config returns multi_agent",
			config:   nil,
			expected: "multi_agent",
		},
		{
			name:     "empty returns multi_agent",
			config:   &WorkflowConfig{},
			expected: "multi_agent",
		},
		{
			name:     "single_agent returns single_agent",
			config:   &WorkflowConfig{ExecutionMode: "single_agent"},
			expected: "single_agent",
		},
		{
			name:     "multi_agent returns multi_agent",
			config:   &WorkflowConfig{ExecutionMode: "multi_agent"},
			expected: "multi_agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.GetExecutionMode(); got != tt.expected {
				t.Errorf("GetExecutionMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkflowConfig_JSON_Roundtrip(t *testing.T) {
	original := WorkflowConfig{
		ConsensusThreshold:         0.8,
		DryRun:                     true,
		ExecutionMode:              "single_agent",
		SingleAgentName:            "claude",
		SingleAgentModel:           "claude-3-sonnet",
		SingleAgentReasoningEffort: "high",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded WorkflowConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ExecutionMode != original.ExecutionMode {
		t.Errorf("ExecutionMode = %q, want %q", decoded.ExecutionMode, original.ExecutionMode)
	}
	if decoded.SingleAgentName != original.SingleAgentName {
		t.Errorf("SingleAgentName = %q, want %q", decoded.SingleAgentName, original.SingleAgentName)
	}
	if decoded.SingleAgentModel != original.SingleAgentModel {
		t.Errorf("SingleAgentModel = %q, want %q", decoded.SingleAgentModel, original.SingleAgentModel)
	}
	if decoded.SingleAgentReasoningEffort != original.SingleAgentReasoningEffort {
		t.Errorf("SingleAgentReasoningEffort = %q, want %q", decoded.SingleAgentReasoningEffort, original.SingleAgentReasoningEffort)
	}
	if decoded.ConsensusThreshold != original.ConsensusThreshold {
		t.Errorf("ConsensusThreshold = %v, want %v", decoded.ConsensusThreshold, original.ConsensusThreshold)
	}
	if decoded.DryRun != original.DryRun {
		t.Errorf("DryRun = %v, want %v", decoded.DryRun, original.DryRun)
	}
}

func TestWorkflowConfig_Backward_Compatibility(t *testing.T) {
	// JSON without new fields should deserialize correctly
	oldJSON := `{"consensus_threshold": 0.75, "dry_run": true}`

	var cfg WorkflowConfig
	if err := json.Unmarshal([]byte(oldJSON), &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.ConsensusThreshold != 0.75 {
		t.Errorf("ConsensusThreshold = %v, want %v", cfg.ConsensusThreshold, 0.75)
	}
	if !cfg.DryRun {
		t.Errorf("DryRun = %v, want %v", cfg.DryRun, true)
	}
	if cfg.ExecutionMode != "" {
		t.Errorf("ExecutionMode = %q, want empty", cfg.ExecutionMode)
	}
	if cfg.SingleAgentName != "" {
		t.Errorf("SingleAgentName = %q, want empty", cfg.SingleAgentName)
	}
	if cfg.SingleAgentModel != "" {
		t.Errorf("SingleAgentModel = %q, want empty", cfg.SingleAgentModel)
	}
	if cfg.SingleAgentReasoningEffort != "" {
		t.Errorf("SingleAgentReasoningEffort = %q, want empty", cfg.SingleAgentReasoningEffort)
	}
	if cfg.IsSingleAgentMode() {
		t.Errorf("IsSingleAgentMode() = true, want false")
	}
}

func TestCreateWorkflow_WithSingleAgentMode(t *testing.T) {
	// Create a temp directory with config file that has claude enabled
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configContent := `
agents:
  default: claude
  claude:
    enabled: true
    path: claude
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to temp directory for test
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(originalDir) })

	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	reqBody := CreateWorkflowRequest{
		Prompt: "Test single agent workflow",
		Title:  "Single Agent Test",
		Config: &WorkflowConfig{
			ExecutionMode:    "single_agent",
			SingleAgentName:  "claude",
			SingleAgentModel: "claude-3-haiku",
		},
	}
	body, _ := json.Marshal(reqBody)

	// Use chi router to handle the request properly
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workflows/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Config == nil {
		t.Fatal("expected config in response, got nil")
	}

	if resp.Config.ExecutionMode != "single_agent" {
		t.Errorf("expected execution_mode 'single_agent', got '%s'", resp.Config.ExecutionMode)
	}

	if resp.Config.SingleAgentName != "claude" {
		t.Errorf("expected single_agent_name 'claude', got '%s'", resp.Config.SingleAgentName)
	}

	if resp.Config.SingleAgentModel != "claude-3-haiku" {
		t.Errorf("expected single_agent_model 'claude-3-haiku', got '%s'", resp.Config.SingleAgentModel)
	}

	// Verify it was saved in state manager correctly
	wfID := core.WorkflowID(resp.ID)
	state, _ := sm.LoadByID(context.Background(), wfID)
	if state.Config.ExecutionMode != "single_agent" {
		t.Errorf("expected saved execution_mode 'single_agent', got '%s'", state.Config.ExecutionMode)
	}
}

func TestGetWorkflow_IncludesConfig(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	wfID := core.WorkflowID("wf-config-test")
	state := &core.WorkflowState{
		WorkflowID: wfID,
		Status:     core.WorkflowStatusPending,
		Prompt:     "Test prompt",
		Config: &core.WorkflowConfig{
			ExecutionMode:   "single_agent",
			SingleAgentName: "gemini",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	sm.workflows[wfID] = state

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/"+string(wfID)+"/", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Config == nil {
		t.Fatal("expected config in response, got nil")
	}

	if resp.Config.ExecutionMode != "single_agent" {
		t.Errorf("expected execution_mode 'single_agent', got '%s'", resp.Config.ExecutionMode)
	}

	if resp.Config.SingleAgentName != "gemini" {
		t.Errorf("expected single_agent_name 'gemini', got '%s'", resp.Config.SingleAgentName)
	}
}

func TestUpdateWorkflow_AllowsConfigEditWhenPending(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configContent := `
agents:
  default: codex
  codex:
    enabled: true
    path: codex
    model: gpt-5.2-codex
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(originalDir) })

	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	wfID := core.WorkflowID("wf-edit-config")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowID: wfID,
		Status:     core.WorkflowStatusPending,
		Prompt:     "Test prompt",
		Config: &core.WorkflowConfig{
			ExecutionMode: "multi_agent",
		},
		Tasks:     make(map[core.TaskID]*core.TaskState),
		TaskOrder: []core.TaskID{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	reqBody := map[string]interface{}{
		"config": map[string]interface{}{
			"execution_mode":                "single_agent",
			"single_agent_name":             "codex",
			"single_agent_model":            "gpt-5.2-codex",
			"single_agent_reasoning_effort": "high",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/"+string(wfID)+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp WorkflowResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Config == nil {
		t.Fatal("expected config in response, got nil")
	}
	if resp.Config.ExecutionMode != "single_agent" {
		t.Errorf("expected execution_mode 'single_agent', got '%s'", resp.Config.ExecutionMode)
	}
	if resp.Config.SingleAgentName != "codex" {
		t.Errorf("expected single_agent_name 'codex', got '%s'", resp.Config.SingleAgentName)
	}
	if resp.Config.SingleAgentModel != "gpt-5.2-codex" {
		t.Errorf("expected single_agent_model 'gpt-5.2-codex', got '%s'", resp.Config.SingleAgentModel)
	}
	if resp.Config.SingleAgentReasoningEffort != "high" {
		t.Errorf("expected single_agent_reasoning_effort 'high', got '%s'", resp.Config.SingleAgentReasoningEffort)
	}
}

func TestUpdateWorkflow_RejectsConfigEditWhenNotPending(t *testing.T) {
	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	wfID := core.WorkflowID("wf-edit-config-running")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowID: wfID,
		Status:     core.WorkflowStatusRunning,
		Prompt:     "Test prompt",
		Config: &core.WorkflowConfig{
			ExecutionMode: "multi_agent",
		},
		Tasks:     make(map[core.TaskID]*core.TaskState),
		TaskOrder: []core.TaskID{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	reqBody := map[string]interface{}{
		"config": map[string]interface{}{
			"execution_mode":    "single_agent",
			"single_agent_name": "claude",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/"+string(wfID)+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

func TestUpdateWorkflow_ValidatesConfigEdits(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configContent := `
agents:
  default: claude
  claude:
    enabled: true
    path: claude
    model: claude-3-sonnet
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(originalDir) })

	sm := newMockStateManager()
	eb := events.New(100)
	srv := NewServer(sm, eb)

	wfID := core.WorkflowID("wf-edit-config-invalid")
	sm.workflows[wfID] = &core.WorkflowState{
		WorkflowID: wfID,
		Status:     core.WorkflowStatusPending,
		Prompt:     "Test prompt",
		Config: &core.WorkflowConfig{
			ExecutionMode: "multi_agent",
		},
		Tasks:     make(map[core.TaskID]*core.TaskState),
		TaskOrder: []core.TaskID{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	reqBody := map[string]interface{}{
		"config": map[string]interface{}{
			"execution_mode": "single_agent",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workflows/"+string(wfID)+"/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}
