package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

type configTestServer struct {
	router http.Handler
}

func setupConfigTestServer(t *testing.T) *configTestServer {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	// Restore working dir for other tests
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() {
		eb.Close()
	})

	srv := NewServer(sm, eb)
	return &configTestServer{
		router: srv.Handler(),
	}
}

func TestHandleGetConfig(t *testing.T) {
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Meta.ETag == "" {
		t.Error("expected ETag in response meta")
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("expected ETag header")
	}
	if response.Config.Log.Level == "" {
		t.Error("expected log level to be set")
	}
	// NOTE: agents.default has no default value - it must be explicitly configured
	// so we don't check for it when loading defaults without a config file
}

func TestHandleUpdateConfig_ETagConflict(t *testing.T) {
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config", bytes.NewBufferString(`{"log": {"level": "debug"}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", "\"wrong-etag\"")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected status %d, got %d: %s", http.StatusPreconditionFailed, rec.Code, rec.Body.String())
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if _, ok := errResp["current_etag"]; !ok {
		t.Error("expected current_etag in response")
	}
}

func TestHandleGetSchema(t *testing.T) {
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/schema", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var schema ConfigSchema
	if err := json.Unmarshal(rec.Body.Bytes(), &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	// Find log.level field in sections
	var foundLogLevel bool
	for _, section := range schema.Sections {
		for _, field := range section.Fields {
			if field.Path == "log.level" {
				foundLogLevel = true
				if field.Type != "string" {
					t.Errorf("expected log.level type string, got %s", field.Type)
				}
				if len(field.ValidValues) == 0 {
					t.Error("expected log.level valid_values to be populated")
				}
				break
			}
		}
	}
	if !foundLogLevel {
		t.Fatal("expected schema to include log.level field")
	}
}

func TestHandleGetEnums(t *testing.T) {
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/enums", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var enums EnumsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &enums); err != nil {
		t.Fatalf("unmarshal enums: %v", err)
	}

	if !containsString(enums.LogLevels, "debug") || !containsString(enums.LogLevels, "info") {
		t.Error("expected log levels to include debug and info")
	}
	if !containsString(enums.StateBackends, "sqlite") {
		t.Error("expected state backends to include sqlite")
	}
	if !containsString(enums.Agents, "copilot") {
		t.Error("expected agents to include copilot")
	}
	if !containsString(enums.Agents, "opencode") {
		t.Error("expected agents to include opencode")
	}
}

func TestHandleValidateConfig(t *testing.T) {
	srv := setupConfigTestServer(t)

	tests := []struct {
		name       string
		body       string
		wantValid  bool
		wantErrors int
	}{
		{
			name:      "valid config",
			body:      `{"log": {"level": "info"}, "agents": {"default": "claude", "claude": {"enabled": true}}, "phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}}}, "git": {"worktree": {"dir": ".worktrees"}}}`,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/config/validate", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			srv.router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
			}

			var result ValidationResult
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}

			if result.Valid != tt.wantValid {
				t.Errorf("expected valid=%v, got %v", tt.wantValid, result.Valid)
			}
			if !tt.wantValid && len(result.Errors) != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, len(result.Errors))
			}
		})
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func TestHandleResetConfig(t *testing.T) {
	srv := setupConfigTestServer(t)

	// First create a config file by updating it with a valid configuration
	validConfig := `{
		"log": {"level": "debug"},
		"agents": {
			"default": "claude",
			"claude": {
				"enabled": true,
				"phases": {"plan": true, "execute": true, "synthesize": true}
			}
		},
		"phases": {
			"analyze": {
				"refiner": {"enabled": false},
				"moderator": {"enabled": false},
				"synthesizer": {"agent": "claude"}
			}
		},
		"git": {
			"worktree": {"dir": ".worktrees", "mode": "parallel"},
			"task": {"auto_commit": true}
		}
	}`
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/config?force=true", bytes.NewBufferString(validConfig))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	srv.router.ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusOK {
		t.Fatalf("failed to create config: %s", updateRec.Body.String())
	}

	// Verify the config was set to debug
	var updateResp ConfigResponseWithMeta
	if err := json.Unmarshal(updateRec.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("unmarshal update response: %v", err)
	}
	if updateResp.Config.Log.Level != "debug" {
		t.Fatalf("expected log level 'debug' after update, got %q", updateResp.Config.Log.Level)
	}

	// Now reset it
	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reset", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// After reset, source should be "file" (we write the default config to file)
	if response.Meta.Source != "file" {
		t.Errorf("expected source 'file', got %q", response.Meta.Source)
	}

	// Should have an ETag now
	if response.Meta.ETag == "" {
		t.Error("expected ETag after reset")
	}

	// Log level should be back to default "info"
	if response.Config.Log.Level != "info" {
		t.Errorf("expected log.level 'info' after reset, got %q", response.Config.Log.Level)
	}

	// Verify default config values from DefaultConfigYAML
	if response.Config.Agents.Default != "claude" {
		t.Errorf("expected agents.default 'claude', got %q", response.Config.Agents.Default)
	}
	if !response.Config.Agents.Claude.Enabled {
		t.Error("expected agents.claude.enabled to be true")
	}
	if response.Config.Agents.Claude.Model != "claude-opus-4-6" {
		t.Errorf("expected agents.claude.model 'claude-opus-4-6', got %q", response.Config.Agents.Claude.Model)
	}
	if !response.Config.Phases.Analyze.Refiner.Enabled {
		t.Error("expected phases.analyze.refiner.enabled to be true")
	}
	if response.Config.Phases.Analyze.Moderator.Threshold != 0.80 {
		t.Errorf("expected phases.analyze.moderator.threshold 0.80, got %v", response.Config.Phases.Analyze.Moderator.Threshold)
	}
}
