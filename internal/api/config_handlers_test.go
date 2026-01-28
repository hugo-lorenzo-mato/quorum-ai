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
			body:      `{"log": {"level": "info"}, "agents": {"default": "claude", "claude": {"enabled": true}}}`,
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
