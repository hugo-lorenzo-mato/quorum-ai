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
			body:      `{"log": {"level": "info"}, "agents": {"default": "claude", "claude": {"enabled": true, "phases": {"plan": true, "execute": true, "synthesize": true}}}, "phases": {"analyze": {"refiner": {"enabled": false}, "moderator": {"enabled": false}, "synthesizer": {"agent": "claude"}}}, "git": {"worktree": {"dir": ".worktrees"}}}`,
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

func TestHandleGetAgents(t *testing.T) {
	srv := setupConfigTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config/agents", nil)
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var agents []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &agents); err != nil {
		t.Fatalf("unmarshal agents: %v", err)
	}

	// Verify all 5 agents are returned
	if len(agents) != 5 {
		t.Fatalf("expected 5 agents, got %d", len(agents))
	}

	// Verify agent names
	names := make(map[string]bool)
	for _, agent := range agents {
		name, _ := agent["name"].(string)
		names[name] = true
	}
	for _, expected := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		if !names[expected] {
			t.Errorf("expected agent %q in response", expected)
		}
	}

	// Verify claude has reasoning efforts
	for _, agent := range agents {
		if agent["name"] == "claude" {
			if hasReasoning, ok := agent["hasReasoningEffort"].(bool); !ok || !hasReasoning {
				t.Error("expected claude to have hasReasoningEffort=true")
			}
			efforts, ok := agent["reasoningEfforts"].([]interface{})
			if !ok || len(efforts) == 0 {
				t.Error("expected claude to have reasoningEfforts")
			}
			models, ok := agent["models"].([]interface{})
			if !ok || len(models) == 0 {
				t.Error("expected claude to have models list")
			}
		}
		if agent["name"] == "codex" {
			if hasReasoning, ok := agent["hasReasoningEffort"].(bool); !ok || !hasReasoning {
				t.Error("expected codex to have hasReasoningEffort=true")
			}
		}
		if agent["name"] == "copilot" {
			// Copilot should NOT have reasoning effort
			if _, ok := agent["hasReasoningEffort"]; ok {
				t.Error("expected copilot to NOT have hasReasoningEffort field")
			}
		}
	}
}

func TestHandleGetConfig_IssuesSection(t *testing.T) {
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

	// Verify issues section is present in the response
	// Labels and assignees should be empty arrays, not nil
	if response.Config.Issues.Labels == nil {
		t.Error("expected issues.default_labels to be non-nil (empty array)")
	}
	if response.Config.Issues.Assignees == nil {
		t.Error("expected issues.default_assignees to be non-nil (empty array)")
	}

	// Verify JSON serialization has issues key
	data, err := json.Marshal(response.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["issues"]; !ok {
		t.Error("expected 'issues' key in config response")
	}
}

func TestHandleUpdateConfig_IssuesFields(t *testing.T) {
	srv := setupConfigTestServer(t)

	// First set up a valid base config (agents, phases) to pass validation
	baseConfig := `{
		"agents": {
			"default": "claude",
			"claude": {
				"enabled": true,
				"phases": {"analyze": true, "plan": true, "execute": true, "synthesize": true}
			},
			"gemini": {
				"enabled": true,
				"phases": {"analyze": true}
			}
		},
		"phases": {
			"analyze": {
				"refiner": {"enabled": false},
				"moderator": {"enabled": false},
				"synthesizer": {"agent": "claude"}
			}
		},
		"git": {"worktree": {"dir": ".worktrees"}}
	}`
	baseReq := httptest.NewRequest(http.MethodPatch, "/api/v1/config?force=true", bytes.NewBufferString(baseConfig))
	baseReq.Header.Set("Content-Type", "application/json")
	baseRec := httptest.NewRecorder()
	srv.router.ServeHTTP(baseRec, baseReq)
	if baseRec.Code != http.StatusOK {
		t.Fatalf("failed to set base config: %s", baseRec.Body.String())
	}

	// Now update just the issues section
	updateBody := `{
		"issues": {
			"enabled": true,
			"provider": "github",
			"auto_generate": true,
			"mode": "agent",
			"draft_directory": "custom/issues",
			"repository": "owner/repo",
			"parent_template": "epic",
			"default_labels": ["quorum", "automated"],
			"default_assignees": ["dev1"],
			"template": {
				"language": "english",
				"tone": "technical",
				"include_diagrams": true,
				"title_format": "[quorum] {task_name}",
				"convention": "conventional-commits",
				"custom_instructions": "Be concise"
			},
			"gitlab": {
				"use_epics": true,
				"project_id": "12345"
			},
			"generator": {
				"enabled": true,
				"agent": "claude",
				"model": "opus",
				"summarize": true,
				"max_body_length": 50000,
				"reasoning_effort": "high",
				"instructions": "Generate detailed issues",
				"title_instructions": "Use conventional format"
			}
		}
	}`

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config?force=true", bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response ConfigResponseWithMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	issues := response.Config.Issues
	if !issues.Enabled {
		t.Error("expected issues.enabled to be true")
	}
	if issues.Provider != "github" {
		t.Errorf("expected issues.provider 'github', got %q", issues.Provider)
	}
	if !issues.AutoGenerate {
		t.Error("expected issues.auto_generate to be true")
	}
	if issues.Mode != "agent" {
		t.Errorf("expected issues.mode 'agent', got %q", issues.Mode)
	}
	if issues.DraftDirectory != "custom/issues" {
		t.Errorf("expected issues.draft_directory 'custom/issues', got %q", issues.DraftDirectory)
	}
	if issues.Repository != "owner/repo" {
		t.Errorf("expected issues.repository 'owner/repo', got %q", issues.Repository)
	}
	if issues.ParentTemplate != "epic" {
		t.Errorf("expected issues.parent_template 'epic', got %q", issues.ParentTemplate)
	}
	if len(issues.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(issues.Labels))
	}
	if len(issues.Assignees) != 1 || issues.Assignees[0] != "dev1" {
		t.Errorf("expected assignees [dev1], got %v", issues.Assignees)
	}

	// Verify template fields
	if issues.Template.Language != "english" {
		t.Errorf("expected template.language 'english', got %q", issues.Template.Language)
	}
	if issues.Template.Tone != "technical" {
		t.Errorf("expected template.tone 'technical', got %q", issues.Template.Tone)
	}
	if !issues.Template.IncludeDiagrams {
		t.Error("expected template.include_diagrams to be true")
	}
	if issues.Template.TitleFormat != "[quorum] {task_name}" {
		t.Errorf("expected template.title_format '[quorum] {task_name}', got %q", issues.Template.TitleFormat)
	}
	if issues.Template.Convention != "conventional-commits" {
		t.Errorf("expected template.convention 'conventional-commits', got %q", issues.Template.Convention)
	}
	if issues.Template.CustomInstructions != "Be concise" {
		t.Errorf("expected template.custom_instructions 'Be concise', got %q", issues.Template.CustomInstructions)
	}

	// Verify GitLab fields
	if !issues.GitLab.UseEpics {
		t.Error("expected gitlab.use_epics to be true")
	}
	if issues.GitLab.ProjectID != "12345" {
		t.Errorf("expected gitlab.project_id '12345', got %q", issues.GitLab.ProjectID)
	}

	// Verify generator fields
	if !issues.Generator.Enabled {
		t.Error("expected generator.enabled to be true")
	}
	if issues.Generator.Agent != "claude" {
		t.Errorf("expected generator.agent 'claude', got %q", issues.Generator.Agent)
	}
	if issues.Generator.Model != "opus" {
		t.Errorf("expected generator.model 'opus', got %q", issues.Generator.Model)
	}
	if !issues.Generator.Summarize {
		t.Error("expected generator.summarize to be true")
	}
	if issues.Generator.MaxBodyLength != 50000 {
		t.Errorf("expected generator.max_body_length 50000, got %d", issues.Generator.MaxBodyLength)
	}
	if issues.Generator.ReasoningEffort != "high" {
		t.Errorf("expected generator.reasoning_effort 'high', got %q", issues.Generator.ReasoningEffort)
	}
	if issues.Generator.Instructions != "Generate detailed issues" {
		t.Errorf("expected generator.instructions value, got %q", issues.Generator.Instructions)
	}
	if issues.Generator.TitleInstructions != "Use conventional format" {
		t.Errorf("expected generator.title_instructions value, got %q", issues.Generator.TitleInstructions)
	}
}

func TestHandleGetEnums_IssueFields(t *testing.T) {
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

	// Verify issue-specific enums
	if len(enums.IssueProviders) == 0 {
		t.Error("expected issue_providers to be populated")
	}
	if !containsString(enums.IssueProviders, "github") {
		t.Error("expected issue_providers to include 'github'")
	}

	if len(enums.TemplateLanguages) == 0 {
		t.Error("expected template_languages to be populated")
	}

	if len(enums.TemplateTones) == 0 {
		t.Error("expected template_tones to be populated")
	}

	if len(enums.IssueModes) == 0 {
		t.Error("expected issue_modes to be populated")
	}
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
