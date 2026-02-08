//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// newTestServer creates an API server with real persistence but mock agents.
func newTestServer(t *testing.T) (*httptest.Server, *api.Server, core.StateManager) {
	t.Helper()

	// Unset env vars to prevent interference
	os.Unsetenv("QUORUM_AGENT")
	os.Unsetenv("QUORUM_AGENTS_DEFAULT")

	// 1. Setup StateManager (Real SQLite)
	dir := testutil.TempDir(t)
	dbPath := filepath.Join(dir, "workflow.db")
	sm, err := state.NewStateManager(dbPath)
	testutil.AssertNoError(t, err)

	// 2. Setup EventBus
	eb := events.New(100)

	// 3. Setup Mock Agent Registry
	registry := testutil.NewMockRegistry()
	registry.Add("claude", testutil.NewMockAgent("claude"))
	registry.Add("gemini", testutil.NewMockAgent("gemini"))
	registry.Add("codex", testutil.NewMockAgent("codex"))
	registry.Add("copilot", testutil.NewMockAgent("copilot"))
	registry.Add("opencode", testutil.NewMockAgent("opencode"))

	// 4. Setup Config Loader (pointing to a temp config file)
	configDir := filepath.Join(dir, ".quorum")
	testutil.AssertNoError(t, os.MkdirAll(configDir, 0o750))
	configPath := filepath.Join(configDir, "config.yaml")
	// Write default config (valid, full)
	err = os.WriteFile(configPath, []byte(config.DefaultConfigYAML), 0o600)
	testutil.AssertNoError(t, err)
	// Also write legacy config for file API tests
	legacyConfigPath := filepath.Join(dir, ".quorum.yaml")
	err = os.WriteFile(legacyConfigPath, []byte(config.DefaultConfigYAML), 0o600)
	testutil.AssertNoError(t, err)

	cfgLoader := config.NewLoader().WithConfigFile(configPath)

	// 5. Create Server
	tracker := api.NewUnifiedTracker(sm, nil, slog.Default(), api.DefaultUnifiedTrackerConfig())
	srv := api.NewServer(sm, eb,
		api.WithAgentRegistry(registry),
		api.WithConfigLoader(cfgLoader),
		api.WithUnifiedTracker(tracker),
		api.WithRoot(dir),
	)

	// 6. Create Test Server
	ts := httptest.NewServer(srv.Handler())

	t.Cleanup(func() {
		ts.Close()
		state.CloseStateManager(sm)
	})

	return ts, srv, sm
}

// Helper to make requests
func request(t *testing.T, ts *httptest.Server, method, path string, body interface{}) (int, http.Header, []byte) {
	t.Helper()

	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		testutil.AssertNoError(t, err)
	}

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(reqBody))
	testutil.AssertNoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := ts.Client()
	resp, err := client.Do(req)
	testutil.AssertNoError(t, err)

	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	testutil.AssertNoError(t, err)

	return resp.StatusCode, resp.Header.Clone(), respBytes
}

func TestE2E_WorkflowLifecycle(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. Create Workflow
	req := map[string]string{
		"prompt": "Build a simple calculator",
	}
	status, _, body := request(t, ts, "POST", "/api/v1/workflows", req)
	testutil.AssertEqual(t, status, http.StatusCreated)

	var createResp api.WorkflowResponse
	err := json.Unmarshal(body, &createResp)
	testutil.AssertNoError(t, err)
	workflowID := createResp.ID
	if workflowID == "" {
		t.Fatal("workflow_id is empty")
	}

	// 2. Get Workflow (verify it exists)
	status, _, body = request(t, ts, "GET", "/api/v1/workflows/"+workflowID, nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var wf api.WorkflowResponse
	err = json.Unmarshal(body, &wf)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wf.ID, workflowID)
	testutil.AssertEqual(t, wf.Prompt, "Build a simple calculator")
	testutil.AssertEqual(t, wf.Status, string(core.WorkflowStatusPending))

	// 3. List Workflows
	status, _, body = request(t, ts, "GET", "/api/v1/workflows", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var summaries []api.WorkflowResponse
	err = json.Unmarshal(body, &summaries)
	testutil.AssertNoError(t, err)

	testutil.AssertLen(t, summaries, 1)
	testutil.AssertEqual(t, summaries[0].ID, workflowID)

	// 4. Update Workflow
	updateReq := map[string]interface{}{
		"title": "Calculator App",
	}
	status, _, _ = request(t, ts, "PATCH", "/api/v1/workflows/"+workflowID, updateReq)
	testutil.AssertEqual(t, status, http.StatusOK)

	// Verify update
	status, _, body = request(t, ts, "GET", "/api/v1/workflows/"+workflowID, nil)
	testutil.AssertEqual(t, status, http.StatusOK)
	err = json.Unmarshal(body, &wf)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wf.Title, "Calculator App")

	// 5. Start Workflow (Activate/Run)
	status, _, _ = request(t, ts, "POST", "/api/v1/workflows/"+workflowID+"/run", nil)
	// Expecting 202 Accepted or 200 OK.
	if status != http.StatusOK && status != http.StatusAccepted {
		t.Errorf("expected 200 or 202, got %d", status)
	}
}

func TestE2E_Configuration(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. Get Config
	status, headers, body := request(t, ts, "GET", "/api/v1/config", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var cfg api.ConfigResponseWithMeta
	err := json.Unmarshal(body, &cfg)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, cfg.Config.Agents.Default, "claude")

	// 2. Update Config
	etag := headers.Get("ETag")

	updateReq := api.FullConfigUpdate{
		Agents: &api.AgentsConfigUpdate{
			Default: strPtr("gemini"),
		},
	}

	// Try without If-Match first (optional check, but good for E2E)
	status, _, _ = request(t, ts, "PATCH", "/api/v1/config", updateReq)

	// If it fails with PreconditionRequired, retry with header
	if status == http.StatusPreconditionRequired {
		reqBody, err := json.Marshal(updateReq)
		testutil.AssertNoError(t, err)
		req, err := http.NewRequest("PATCH", ts.URL+"/api/v1/config", bytes.NewBuffer(reqBody))
		testutil.AssertNoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("If-Match", etag)
		client := ts.Client()
		resp, err := client.Do(req)
		testutil.AssertNoError(t, err)
		_ = resp.Body.Close()
		status = resp.StatusCode
	}

	testutil.AssertEqual(t, status, http.StatusOK)

	// 3. Verify Update
	status, _, body = request(t, ts, "GET", "/api/v1/config", nil)
	testutil.AssertEqual(t, status, http.StatusOK)
	err = json.Unmarshal(body, &cfg)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, cfg.Config.Agents.Default, "gemini")
}

func TestE2E_ConfigSections(t *testing.T) {
	ts, _, _ := newTestServer(t)

	t.Run("Workflow Config", func(t *testing.T) {
		// Update workflow settings
		updateReq := api.FullConfigUpdate{
			Workflow: &api.WorkflowConfigUpdate{
				Timeout: strPtr("2h"),
			},
		}
		status, _, body := request(t, ts, "PATCH", "/api/v1/config", updateReq)
		testutil.AssertEqual(t, status, http.StatusOK)

		var cfg api.ConfigResponseWithMeta
		err := json.Unmarshal(body, &cfg)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, cfg.Config.Workflow.Timeout, "2h")
	})

	t.Run("Agents Config", func(t *testing.T) {
		// Update agents settings
		updateReq := api.FullConfigUpdate{
			Agents: &api.AgentsConfigUpdate{
				Default: strPtr("gemini"),
				Claude: &api.FullAgentConfigUpdate{
					Model: strPtr("claude-3-5-sonnet-latest"),
				},
			},
		}
		status, _, body := request(t, ts, "PATCH", "/api/v1/config", updateReq)
		testutil.AssertEqual(t, status, http.StatusOK)

		var cfg api.ConfigResponseWithMeta
		err := json.Unmarshal(body, &cfg)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, cfg.Config.Agents.Default, "gemini")
		testutil.AssertEqual(t, cfg.Config.Agents.Claude.Enabled, true)
		testutil.AssertEqual(t, cfg.Config.Agents.Claude.Model, "claude-3-5-sonnet-latest")
	})

	t.Run("Git Config", func(t *testing.T) {
		updateReq := api.FullConfigUpdate{
			Git: &api.GitConfigUpdate{
				Task: &api.GitTaskConfigUpdate{
					AutoCommit: boolPtr(true),
				},
				Finalization: &api.GitFinalizationConfigUpdate{
					AutoPush: boolPtr(true),
				},
				Worktree: &api.WorktreeConfigUpdate{
					Mode: strPtr("always"),
				},
			},
		}
		status, _, body := request(t, ts, "PATCH", "/api/v1/config", updateReq)
		testutil.AssertEqual(t, status, http.StatusOK)

		var cfg api.ConfigResponseWithMeta
		err := json.Unmarshal(body, &cfg)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, cfg.Config.Git.Task.AutoCommit, true)
		testutil.AssertEqual(t, cfg.Config.Git.Finalization.AutoPush, true)
		testutil.AssertEqual(t, cfg.Config.Git.Worktree.Mode, "always")
	})

	t.Run("Log Config", func(t *testing.T) {
		updateReq := api.FullConfigUpdate{
			Log: &api.LogConfigUpdate{
				Level:  strPtr("debug"),
				Format: strPtr("json"),
			},
		}
		status, _, body := request(t, ts, "PATCH", "/api/v1/config", updateReq)
		testutil.AssertEqual(t, status, http.StatusOK)

		var cfg api.ConfigResponseWithMeta
		err := json.Unmarshal(body, &cfg)
		testutil.AssertNoError(t, err)
		testutil.AssertEqual(t, cfg.Config.Log.Level, "debug")
		testutil.AssertEqual(t, cfg.Config.Log.Format, "json")
	})
}

func TestE2E_ConfigPersistence(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. Update config
	updateReq := api.FullConfigUpdate{
		Workflow: &api.WorkflowConfigUpdate{
			Timeout: strPtr("30m"),
		},
		Agents: &api.AgentsConfigUpdate{
			Default: strPtr("codex"),
		},
	}
	status, _, _ := request(t, ts, "PATCH", "/api/v1/config", updateReq)
	testutil.AssertEqual(t, status, http.StatusOK)

	// 2. Read config back (verify persistence)
	status, _, body := request(t, ts, "GET", "/api/v1/config", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var cfg api.ConfigResponseWithMeta
	err := json.Unmarshal(body, &cfg)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, cfg.Config.Workflow.Timeout, "30m")
	testutil.AssertEqual(t, cfg.Config.Agents.Default, "codex")
}

func TestE2E_Agents(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. Get Agents
	status, _, body := request(t, ts, "GET", "/api/v1/config/agents", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var agents []map[string]interface{}
	err := json.Unmarshal(body, &agents)
	testutil.AssertNoError(t, err)

	// Should have claude, gemini, codex
	if len(agents) < 3 {
		t.Errorf("expected at least 3 agents, got %d", len(agents))
	}

	// Verify agent structure
	for _, agent := range agents {
		if _, ok := agent["name"]; !ok {
			t.Error("agent missing 'name' field")
		}
		if _, ok := agent["displayName"]; !ok {
			t.Error("agent missing 'displayName' field")
		}
		if _, ok := agent["models"]; !ok {
			t.Error("agent missing 'models' field")
		}
		if _, ok := agent["available"]; !ok {
			t.Error("agent missing 'available' field")
		}
	}
}

func TestE2E_Files(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. List Files (root)
	status, _, body := request(t, ts, "GET", "/api/v1/files?path=.", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var files []interface{}
	err := json.Unmarshal(body, &files)
	testutil.AssertNoError(t, err)

	// 2. Get File Content (read .quorum.yaml created in newTestServer)
	status, _, body = request(t, ts, "GET", "/api/v1/files/content?path=.quorum.yaml", nil)
	testutil.AssertEqual(t, status, http.StatusOK)
	if len(body) == 0 {
		t.Error("expected file content, got empty")
	}

	// 3. Get File Tree
	status, _, _ = request(t, ts, "GET", "/api/v1/files/tree", nil)
	testutil.AssertEqual(t, status, http.StatusOK)
}

func TestE2E_Chat(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. Create Session
	createReq := map[string]string{
		"agent": "claude",
	}
	status, _, body := request(t, ts, "POST", "/api/v1/chat/sessions", createReq)
	testutil.AssertEqual(t, status, http.StatusCreated)

	var session map[string]interface{}
	err := json.Unmarshal(body, &session)
	testutil.AssertNoError(t, err)
	sessionID, ok := session["id"].(string)
	if !ok || sessionID == "" {
		t.Fatal("session_id is empty or invalid")
	}

	// 2. Send Message
	msgReq := map[string]string{
		"role":    "user",
		"content": "Hello",
	}
	status, _, _ = request(t, ts, "POST", "/api/v1/chat/sessions/"+sessionID+"/messages", msgReq)
	testutil.AssertEqual(t, status, http.StatusOK)

	// 3. Get Messages
	status, _, body = request(t, ts, "GET", "/api/v1/chat/sessions/"+sessionID+"/messages", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var messages []interface{}
	err = json.Unmarshal(body, &messages)
	testutil.AssertNoError(t, err)
	if len(messages) == 0 {
		t.Error("expected messages, got empty list")
	}
}

func TestE2E_Health(t *testing.T) {
	ts, _, _ := newTestServer(t)

	status, _, body := request(t, ts, "GET", "/health", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var health map[string]string
	err := json.Unmarshal(body, &health)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, health["status"], "healthy")
}

func TestE2E_ConfigPartialUpdate(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// Get initial config
	status, _, body := request(t, ts, "GET", "/api/v1/config", nil)
	testutil.AssertEqual(t, status, http.StatusOK)

	var initialCfg api.ConfigResponseWithMeta
	err := json.Unmarshal(body, &initialCfg)
	testutil.AssertNoError(t, err)

	// Update only log level (other fields should remain unchanged)
	updateReq := api.FullConfigUpdate{
		Log: &api.LogConfigUpdate{
			Level: strPtr("warn"),
		},
	}
	status, _, body = request(t, ts, "PATCH", "/api/v1/config", updateReq)
	testutil.AssertEqual(t, status, http.StatusOK)

	var updatedCfg api.ConfigResponseWithMeta
	err = json.Unmarshal(body, &updatedCfg)
	testutil.AssertNoError(t, err)

	// Verify log level changed
	testutil.AssertEqual(t, updatedCfg.Config.Log.Level, "warn")

	// Verify other sections unchanged
	testutil.AssertEqual(t, updatedCfg.Config.Agents.Default, initialCfg.Config.Agents.Default)
	testutil.AssertEqual(t, updatedCfg.Config.Git.Task.AutoCommit, initialCfg.Config.Git.Task.AutoCommit)
}

func TestE2E_ConfigInvalidRequest(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// Send invalid JSON
	reqBody := []byte(`{"invalid": }`)
	req, err := http.NewRequest("PATCH", ts.URL+"/api/v1/config", bytes.NewBuffer(reqBody))
	testutil.AssertNoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	client := ts.Client()
	resp, err := client.Do(req)
	testutil.AssertNoError(t, err)
	_ = resp.Body.Close()

	testutil.AssertEqual(t, resp.StatusCode, http.StatusBadRequest)
}

// Helper functions for creating pointers
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
