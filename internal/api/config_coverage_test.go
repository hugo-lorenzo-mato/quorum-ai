package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// ---------------------------------------------------------------------------
// handleGetConfig additional coverage
// ---------------------------------------------------------------------------

func TestHandleGetConfig_ConditionalGet_NotModified(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	// First GET to obtain the ETag.
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec1 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first GET: expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on first GET")
	}

	// Second GET with If-None-Match matching the ETag.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	srv.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Errorf("expected 304 Not Modified, got %d", rec2.Code)
	}
}

func TestHandleGetConfig_ConditionalGet_Modified(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	// GET with a stale ETag should return 200.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	req.Header.Set("If-None-Match", `"stale-etag"`)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// determineConfigSource
// ---------------------------------------------------------------------------

func TestDetermineConfigSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// Non-existent path → default.
	if src := srv.determineConfigSource(filepath.Join(tmpDir, "missing.yaml")); src != "default" {
		t.Errorf("expected 'default', got %q", src)
	}

	// Existing path → file.
	p := filepath.Join(tmpDir, "exists.yaml")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if src := srv.determineConfigSource(p); src != "file" {
		t.Errorf("expected 'file', got %q", src)
	}
}

// ---------------------------------------------------------------------------
// getProjectConfigMode
// ---------------------------------------------------------------------------

func TestGetProjectConfigMode_NilRegistry(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb) // no projectRegistry

	mode, err := srv.getProjectConfigMode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeCustom {
		t.Errorf("expected %q, got %q", project.ConfigModeCustom, mode)
	}
}

// ---------------------------------------------------------------------------
// getProjectConfigPath
// ---------------------------------------------------------------------------

func TestGetProjectConfigPath_FallbackToRoot(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// No project context → falls back to server root.
	p := srv.getProjectConfigPath(context.Background())
	expected := filepath.Join(tmpDir, ".quorum", "config.yaml")
	if p != expected {
		t.Errorf("expected %q, got %q", expected, p)
	}
}

func TestGetProjectConfigPath_EmptyRoot(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	// Create server with empty root.
	srv := &Server{
		stateManager: sm,
		eventBus:     eb,
		root:         "",
	}

	p := srv.getProjectConfigPath(context.Background())
	expected := filepath.Join(".quorum", "config.yaml")
	if p != expected {
		t.Errorf("expected %q, got %q", expected, p)
	}
}

func TestGetProjectConfigPath_WithProjectContext(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	pc := &project.ProjectContext{
		ID:   "proj1",
		Root: "/custom/project",
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	p := srv.getProjectConfigPath(ctx)
	expected := filepath.Join("/custom/project", ".quorum", "config.yaml")
	if p != expected {
		t.Errorf("expected %q, got %q", expected, p)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateConfig coverage
// ---------------------------------------------------------------------------

func TestHandleUpdateConfig_InvalidJSON(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateConfig_ForceBypassesETag(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	body := `{
		"log": {"level": "debug"},
		"agents": {"default": "claude", "claude": {"enabled": true, "phases": {"plan": true, "execute": true, "synthesize": true}}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}, "synthesizer": {"agent": "claude"}}},
		"git": {"worktree": {"dir": ".worktrees"}}
	}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config?force=true", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"wrong-etag"`)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with force=true, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleGetGlobalConfig
// ---------------------------------------------------------------------------

func TestHandleGetGlobalConfig(t *testing.T) {
	// Set HOME to temp dir so EnsureGlobalConfigFile creates files there.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/global", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Meta.Scope != "global" {
		t.Errorf("expected scope 'global', got %q", resp.Meta.Scope)
	}
}

func TestHandleGetGlobalConfig_ConditionalGet(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// First request to get the ETag.
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/config/global", nil)
	rec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first GET: expected 200, got %d", rec1.Code)
	}
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	// Second request with matching ETag.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/config/global", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", rec2.Code)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateGlobalConfig
// ---------------------------------------------------------------------------

func TestHandleUpdateGlobalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	body := `{
		"log": {"level": "warn"},
		"agents": {"default": "claude", "claude": {"enabled": true, "phases": {"plan": true, "execute": true, "synthesize": true}}},
		"phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}, "synthesizer": {"agent": "claude"}}},
		"git": {"worktree": {"dir": ".worktrees"}}
	}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config/global?force=true", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

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
	if resp.Meta.Scope != "global" {
		t.Errorf("expected scope 'global', got %q", resp.Meta.Scope)
	}
}

func TestHandleUpdateGlobalConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config/global", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateGlobalConfig_ETagConflict(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	body := `{"log": {"level": "debug"}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config/global", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"wrong-etag"`)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// handleResetGlobalConfig
// ---------------------------------------------------------------------------

func TestHandleResetGlobalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/global/reset", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Meta.Scope != "global" {
		t.Errorf("expected scope 'global', got %q", resp.Meta.Scope)
	}
	if resp.Meta.Source != "file" {
		t.Errorf("expected source 'file', got %q", resp.Meta.Source)
	}
}

// ---------------------------------------------------------------------------
// handleGetAgents additional coverage
// ---------------------------------------------------------------------------

func TestHandleGetAgents_ModelLists(t *testing.T) {
	t.Parallel()
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/agents", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var agents []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify each agent has a models list.
	for _, agent := range agents {
		name := agent["name"].(string)
		models, ok := agent["models"].([]interface{})
		if !ok || len(models) == 0 {
			t.Errorf("agent %q should have a non-empty models list", name)
		}
		// All agents should be available.
		if avail, ok := agent["available"].(bool); !ok || !avail {
			t.Errorf("agent %q should be available", name)
		}
	}

	// Verify codex has reasoning efforts.
	for _, agent := range agents {
		if agent["name"] == "codex" {
			efforts, ok := agent["reasoningEfforts"].([]interface{})
			if !ok || len(efforts) == 0 {
				t.Error("codex should have reasoningEfforts")
			}
		}
	}

	// Verify gemini does NOT have reasoning effort.
	for _, agent := range agents {
		if agent["name"] == "gemini" {
			if _, ok := agent["hasReasoningEffort"]; ok {
				t.Error("gemini should NOT have hasReasoningEffort field")
			}
		}
	}

	// Verify opencode does NOT have reasoning effort.
	for _, agent := range agents {
		if agent["name"] == "opencode" {
			if _, ok := agent["hasReasoningEffort"]; ok {
				t.Error("opencode should NOT have hasReasoningEffort field")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// configToFullResponse coverage
// ---------------------------------------------------------------------------

func TestConfigToFullResponse_NilSlicesBecomEmptySlices(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		// Leave all slices nil in Workflow, Trace, etc.
		Log: config.LogConfig{Level: "info", Format: "auto"},
	}

	resp := configToFullResponse(cfg)

	// Nil slices should become empty slices.
	if resp.Workflow.DenyTools == nil {
		t.Error("DenyTools should be non-nil empty slice")
	}
	if resp.Trace.RedactPatterns == nil {
		t.Error("RedactPatterns should be non-nil empty slice")
	}
	if resp.Trace.RedactAllowlist == nil {
		t.Error("RedactAllowlist should be non-nil empty slice")
	}
	if resp.Trace.IncludePhases == nil {
		t.Error("IncludePhases should be non-nil empty slice")
	}
}

func TestConfigToFullResponse_PreservesValues(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Log:   config.LogConfig{Level: "debug", Format: "json"},
		Trace: config.TraceConfig{Mode: "file", Dir: "/tmp/traces", SchemaVersion: 2, Redact: true, RedactPatterns: []string{"pw"}, RedactAllowlist: []string{"ok"}, MaxBytes: 1000, TotalMaxBytes: 5000, MaxFiles: 10, IncludePhases: []string{"analyze"}},
		Workflow: config.WorkflowConfig{
			Timeout:    "5m",
			MaxRetries: 5,
			DryRun:     true,
			DenyTools:  []string{"rm"},
			Heartbeat:  config.HeartbeatConfig{Interval: "30s", StaleThreshold: "2m", CheckInterval: "1m", AutoResume: true, MaxResumes: 3},
		},
		State: config.StateConfig{Path: "/state", BackupPath: "/backup", LockTTL: "1h"},
		Git: config.GitConfig{
			Worktree:     config.WorktreeConfig{Dir: ".wt", Mode: "parallel", AutoClean: true},
			Task:         config.GitTaskConfig{AutoCommit: true},
			Finalization: config.GitFinalizationConfig{AutoPush: true, AutoPR: true, AutoMerge: true, PRBaseBranch: "main", MergeStrategy: "squash"},
		},
		GitHub: config.GitHubConfig{Remote: "origin"},
		Chat:   config.ChatConfig{Timeout: "10m", ProgressInterval: "5s", Editor: "vim"},
		Report: config.ReportConfig{Enabled: true, BaseDir: "/reports", UseUTC: true, IncludeRaw: true},
	}

	resp := configToFullResponse(cfg)

	if resp.Log.Level != "debug" {
		t.Errorf("expected 'debug', got %q", resp.Log.Level)
	}
	if resp.Workflow.Timeout != "5m" {
		t.Errorf("expected '5m', got %q", resp.Workflow.Timeout)
	}
	if !resp.Workflow.DryRun {
		t.Error("expected DryRun true")
	}
	if resp.Workflow.Heartbeat.Interval != "30s" {
		t.Errorf("expected '30s', got %q", resp.Workflow.Heartbeat.Interval)
	}
	if !resp.Workflow.Heartbeat.AutoResume {
		t.Error("expected AutoResume true")
	}
	if resp.Git.Finalization.MergeStrategy != "squash" {
		t.Errorf("expected 'squash', got %q", resp.Git.Finalization.MergeStrategy)
	}
	if resp.State.LockTTL != "1h" {
		t.Errorf("expected '1h', got %q", resp.State.LockTTL)
	}
	if resp.GitHub.Remote != "origin" {
		t.Errorf("expected 'origin', got %q", resp.GitHub.Remote)
	}
	if resp.Chat.Editor != "vim" {
		t.Errorf("expected 'vim', got %q", resp.Chat.Editor)
	}
	if !resp.Report.UseUTC {
		t.Error("expected UseUTC true")
	}
}

// ---------------------------------------------------------------------------
// agentConfigToResponse coverage
// ---------------------------------------------------------------------------

func TestAgentConfigToResponse_NilMaps(t *testing.T) {
	t.Parallel()
	cfg := &config.AgentConfig{
		Enabled: true,
		Path:    "claude",
		Model:   "opus",
		// Leave maps nil.
	}

	resp := agentConfigToResponse(cfg)

	if resp.PhaseModels == nil {
		t.Error("PhaseModels should be non-nil")
	}
	if resp.Phases == nil {
		t.Error("Phases should be non-nil")
	}
	if resp.ReasoningEffortPhases == nil {
		t.Error("ReasoningEffortPhases should be non-nil")
	}
}

func TestAgentConfigToResponse_WithValues(t *testing.T) {
	t.Parallel()
	cfg := &config.AgentConfig{
		Enabled:                   true,
		Path:                      "claude",
		Model:                     "opus",
		PhaseModels:               map[string]string{"plan": "sonnet"},
		Phases:                    map[string]bool{"plan": true},
		ReasoningEffort:           "high",
		ReasoningEffortPhases:     map[string]string{"plan": "max"},
		TokenDiscrepancyThreshold: 0.5,
	}

	resp := agentConfigToResponse(cfg)

	if resp.Model != "opus" {
		t.Errorf("expected 'opus', got %q", resp.Model)
	}
	if resp.PhaseModels["plan"] != "sonnet" {
		t.Errorf("expected 'sonnet', got %q", resp.PhaseModels["plan"])
	}
	if resp.TokenDiscrepancyThreshold != 0.5 {
		t.Errorf("expected 0.5, got %f", resp.TokenDiscrepancyThreshold)
	}
}

// ---------------------------------------------------------------------------
// applyFullConfigUpdates coverage (all section branches)
// ---------------------------------------------------------------------------

func TestApplyFullConfigUpdates_AllSections(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}

	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }
	intPtr := func(i int) *int { return &i }
	int64Ptr := func(i int64) *int64 { return &i }
	float64Ptr := func(f float64) *float64 { return &f }

	req := &FullConfigUpdate{
		Log:   &LogConfigUpdate{Level: strPtr("warn"), Format: strPtr("json")},
		Trace: &TraceConfigUpdate{Mode: strPtr("file"), Dir: strPtr("/traces"), SchemaVersion: intPtr(2), Redact: boolPtr(true), RedactPatterns: &[]string{"pw"}, RedactAllowlist: &[]string{"ok"}, MaxBytes: int64Ptr(1000), TotalMaxBytes: int64Ptr(5000), MaxFiles: intPtr(10), IncludePhases: &[]string{"analyze"}},
		Workflow: &WorkflowConfigUpdate{
			Timeout:    strPtr("10m"),
			MaxRetries: intPtr(5),
			DryRun:     boolPtr(true),
			DenyTools:  &[]string{"rm"},
			Heartbeat: &HeartbeatConfigUpdate{
				Interval:       strPtr("60s"),
				StaleThreshold: strPtr("5m"),
				CheckInterval:  strPtr("2m"),
				AutoResume:     boolPtr(true),
				MaxResumes:     intPtr(5),
			},
		},
		Phases: &PhasesConfigUpdate{
			Analyze: &AnalyzePhaseConfigUpdate{
				Timeout:   strPtr("20m"),
				Refiner:   &RefinerConfigUpdate{Enabled: boolPtr(true), Agent: strPtr("claude")},
				Moderator: &ModeratorConfigUpdate{Enabled: boolPtr(true), Agent: strPtr("gemini"), Threshold: float64Ptr(0.9), MinSuccessfulAgents: intPtr(2), MinRounds: intPtr(1), MaxRounds: intPtr(5), WarningThreshold: float64Ptr(0.7), StagnationThreshold: float64Ptr(0.1)},
				Synthesizer: &SynthesizerConfigUpdate{Agent: strPtr("claude")},
				SingleAgent: &SingleAgentConfigUpdate{Enabled: boolPtr(true), Agent: strPtr("claude"), Model: strPtr("opus")},
			},
			Plan: &PlanPhaseConfigUpdate{
				Timeout:     strPtr("15m"),
				Synthesizer: &PlanSynthesizerConfigUpdate{Enabled: boolPtr(true), Agent: strPtr("gemini")},
			},
			Execute: &ExecutePhaseConfigUpdate{Timeout: strPtr("30m")},
		},
		Agents: &AgentsConfigUpdate{
			Default:  strPtr("claude"),
			Claude:   &FullAgentConfigUpdate{Enabled: boolPtr(true), Path: strPtr("claude"), Model: strPtr("opus"), ReasoningEffort: strPtr("high"), TokenDiscrepancyThreshold: float64Ptr(0.3)},
			Gemini:   &FullAgentConfigUpdate{Enabled: boolPtr(true)},
			Codex:    &FullAgentConfigUpdate{Enabled: boolPtr(false)},
			Copilot:  &FullAgentConfigUpdate{Enabled: boolPtr(false)},
			OpenCode: &FullAgentConfigUpdate{Enabled: boolPtr(false)},
		},
		State: &StateConfigUpdate{Path: strPtr("/state"), BackupPath: strPtr("/backup"), LockTTL: strPtr("2h")},
		Git: &GitConfigUpdate{
			Worktree:     &WorktreeConfigUpdate{Dir: strPtr(".wt"), Mode: strPtr("parallel"), AutoClean: boolPtr(true)},
			Task:         &GitTaskConfigUpdate{AutoCommit: boolPtr(true)},
			Finalization: &GitFinalizationConfigUpdate{AutoPush: boolPtr(true), AutoPR: boolPtr(true), AutoMerge: boolPtr(true), PRBaseBranch: strPtr("main"), MergeStrategy: strPtr("squash")},
		},
		GitHub: &GitHubConfigUpdate{Remote: strPtr("upstream")},
		Chat:   &ChatConfigUpdate{Timeout: strPtr("10m"), ProgressInterval: strPtr("5s"), Editor: strPtr("vim")},
		Report: &ReportConfigUpdate{Enabled: boolPtr(true), BaseDir: strPtr("/reports"), UseUTC: boolPtr(true), IncludeRaw: boolPtr(true)},
		Diagnostics: &DiagnosticsConfigUpdate{
			Enabled: boolPtr(true),
			ResourceMonitoring: &ResourceMonitoringConfigUpdate{
				Interval: strPtr("10s"), FDThresholdPercent: intPtr(80), GoroutineThreshold: intPtr(1000), MemoryThresholdMB: intPtr(512), HistorySize: intPtr(100),
			},
			CrashDump: &CrashDumpConfigUpdate{
				Dir: strPtr("/crashes"), MaxFiles: intPtr(5), IncludeStack: boolPtr(true), IncludeEnv: boolPtr(false),
			},
			PreflightChecks: &PreflightConfigUpdate{
				Enabled: boolPtr(true), MinFreeFDPercent: intPtr(20), MinFreeMemoryMB: intPtr(256),
			},
		},
		Issues: &IssuesConfigUpdate{
			Enabled:        boolPtr(true),
			Provider:       strPtr("github"),
			AutoGenerate:   boolPtr(true),
			Timeout:        strPtr("5m"),
			Mode:           strPtr("agent"),
			DraftDirectory: strPtr("custom"),
			Repository:     strPtr("org/repo"),
			ParentPrompt:   strPtr("epic"),
			Labels:         &[]string{"bug"},
			Assignees:      &[]string{"dev"},
			Prompt:         &IssuePromptConfigUpdate{Language: strPtr("en"), Tone: strPtr("formal"), IncludeDiagrams: boolPtr(true), TitleFormat: strPtr("[Q] {name}"), BodyPromptFile: strPtr("p.md"), Convention: strPtr("conventional"), CustomInstructions: strPtr("be brief")},
			GitLab:         &GitLabIssueConfigUpdate{UseEpics: boolPtr(true), ProjectID: strPtr("42")},
			Generator:      &IssueGeneratorConfigUpdate{Enabled: boolPtr(true), Agent: strPtr("claude"), Model: strPtr("opus"), Summarize: boolPtr(true), MaxBodyLength: intPtr(50000), ReasoningEffort: strPtr("high"), Instructions: strPtr("detail"), TitleInstructions: strPtr("short")},
		},
	}

	applyFullConfigUpdates(cfg, req)

	// Spot-check applied values.
	if cfg.Log.Level != "warn" {
		t.Errorf("expected 'warn', got %q", cfg.Log.Level)
	}
	if cfg.Trace.Mode != "file" {
		t.Errorf("expected 'file', got %q", cfg.Trace.Mode)
	}
	if cfg.Workflow.MaxRetries != 5 {
		t.Errorf("expected 5, got %d", cfg.Workflow.MaxRetries)
	}
	if cfg.Phases.Analyze.Moderator.Threshold != 0.9 {
		t.Errorf("expected 0.9, got %f", cfg.Phases.Analyze.Moderator.Threshold)
	}
	if cfg.Phases.Analyze.SingleAgent.Model != "opus" {
		t.Errorf("expected 'opus', got %q", cfg.Phases.Analyze.SingleAgent.Model)
	}
	if cfg.Agents.Default != "claude" {
		t.Errorf("expected 'claude', got %q", cfg.Agents.Default)
	}
	if cfg.State.LockTTL != "2h" {
		t.Errorf("expected '2h', got %q", cfg.State.LockTTL)
	}
	if cfg.Git.Finalization.MergeStrategy != "squash" {
		t.Errorf("expected 'squash', got %q", cfg.Git.Finalization.MergeStrategy)
	}
	if cfg.GitHub.Remote != "upstream" {
		t.Errorf("expected 'upstream', got %q", cfg.GitHub.Remote)
	}
	if cfg.Chat.Editor != "vim" {
		t.Errorf("expected 'vim', got %q", cfg.Chat.Editor)
	}
	if !cfg.Report.UseUTC {
		t.Error("expected UseUTC true")
	}
	if cfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB != 512 {
		t.Errorf("expected 512, got %d", cfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB)
	}
	if cfg.Diagnostics.CrashDump.Dir != "/crashes" {
		t.Errorf("expected '/crashes', got %q", cfg.Diagnostics.CrashDump.Dir)
	}
	if cfg.Diagnostics.PreflightChecks.MinFreeMemoryMB != 256 {
		t.Errorf("expected 256, got %d", cfg.Diagnostics.PreflightChecks.MinFreeMemoryMB)
	}
	if cfg.Issues.Provider != "github" {
		t.Errorf("expected 'github', got %q", cfg.Issues.Provider)
	}
	if cfg.Issues.Prompt.Convention != "conventional" {
		t.Errorf("expected 'conventional', got %q", cfg.Issues.Prompt.Convention)
	}
	if cfg.Issues.GitLab.ProjectID != "42" {
		t.Errorf("expected '42', got %q", cfg.Issues.GitLab.ProjectID)
	}
	if cfg.Issues.Generator.Instructions != "detail" {
		t.Errorf("expected 'detail', got %q", cfg.Issues.Generator.Instructions)
	}
}

func TestApplyFullConfigUpdates_NilSections(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Log: config.LogConfig{Level: "info"},
	}
	// All nil sections → nothing should change.
	applyFullConfigUpdates(cfg, &FullConfigUpdate{})
	if cfg.Log.Level != "info" {
		t.Errorf("expected 'info', got %q", cfg.Log.Level)
	}
}

// ---------------------------------------------------------------------------
// issuesToResponse coverage
// ---------------------------------------------------------------------------

func TestIssuesToResponse_NilSlices(t *testing.T) {
	t.Parallel()
	cfg := &config.IssuesConfig{}

	resp := issuesToResponse(cfg)

	if resp.Labels == nil {
		t.Error("Labels should be non-nil empty slice")
	}
	if resp.Assignees == nil {
		t.Error("Assignees should be non-nil empty slice")
	}
}

// ---------------------------------------------------------------------------
// loadConfigForContext
// ---------------------------------------------------------------------------

func TestLoadConfigForContext_FileExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))

	// Create a config file.
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config.DefaultConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := srv.loadConfigForContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoadConfigForContext_FileNotExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(tmpDir))
	// No config file created → should still return default config.

	cfg, err := srv.loadConfigForContext(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

// ---------------------------------------------------------------------------
// applyAgentUpdates – PhaseModels and Phases
// ---------------------------------------------------------------------------

func TestApplyAgentUpdates_PhaseModelsAndPhases(t *testing.T) {
	t.Parallel()
	cfg := &config.AgentConfig{
		Enabled: true,
		Model:   "opus",
	}

	pm := map[string]string{"plan": "sonnet", "execute": "haiku"}
	ph := map[string]bool{"plan": true, "execute": true}
	rp := map[string]string{"plan": "max"}

	update := &FullAgentConfigUpdate{
		PhaseModels:           &pm,
		Phases:                &ph,
		ReasoningEffortPhases: &rp,
	}

	applyAgentUpdates(cfg, update)

	if cfg.PhaseModels["plan"] != "sonnet" {
		t.Errorf("expected 'sonnet', got %q", cfg.PhaseModels["plan"])
	}
	if !cfg.Phases["execute"] {
		t.Error("expected phases.execute true")
	}
	if cfg.ReasoningEffortPhases["plan"] != "max" {
		t.Errorf("expected 'max', got %q", cfg.ReasoningEffortPhases["plan"])
	}
}
