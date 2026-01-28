package api

import (
	"encoding/json"
	"testing"
)

func TestFullConfigResponse_Marshaling(t *testing.T) {
	response := FullConfigResponse{
		Workflow: WorkflowConfigResponse{
			Timeout:    "1h",
			MaxRetries: 3,
			Sandbox:    false,
			DenyTools:  []string{},
		},
		Agents: AgentsConfigResponse{
			Default: "claude",
			Claude: FullAgentConfigResponse{
				Enabled: true,
				Model:   "claude-opus-4-5-20251101",
				Path:    "claude",
			},
			Copilot: FullAgentConfigResponse{
				Enabled: false,
				Model:   "gpt-5.2-codex",
				Path:    "copilot",
			},
		},
		Git: GitConfigResponse{
			AutoCommit:   false,
			AutoPush:     false,
			WorktreeMode: "parallel",
		},
		Log: LogConfigResponse{
			Level:  "info",
			Format: "auto",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	for _, key := range []string{"workflow", "agents", "git", "log"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in response", key)
		}
	}
}

func TestFullConfigUpdate_PointerFields(t *testing.T) {
	tests := []struct {
		name         string
		jsonBody     string
		wantLog      bool
		wantLevelSet bool
		wantFormat   bool
		wantWorkflow bool
		wantGit      bool
	}{
		{
			name:         "omitted fields are nil",
			jsonBody:     `{"log": {"level": "debug"}}`,
			wantLog:      true,
			wantLevelSet: true,
			wantFormat:   false,
			wantWorkflow: false,
			wantGit:      false,
		},
		{
			name:         "explicit null remains nil",
			jsonBody:     `{"log": {"level": null}}`,
			wantLog:      true,
			wantLevelSet: false,
			wantFormat:   false,
			wantWorkflow: false,
			wantGit:      false,
		},
		{
			name:         "empty string is set",
			jsonBody:     `{"git": {"worktree_mode": ""}}`,
			wantLog:      false,
			wantLevelSet: false,
			wantFormat:   false,
			wantWorkflow: false,
			wantGit:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req FullConfigUpdate
			if err := json.Unmarshal([]byte(tt.jsonBody), &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}

			if tt.wantLog {
				if req.Log == nil {
					t.Fatal("expected Log to be set")
				}
				if tt.wantLevelSet && req.Log.Level == nil {
					t.Error("expected Log.Level to be set")
				}
				if !tt.wantLevelSet && req.Log.Level != nil {
					t.Error("expected Log.Level to be nil")
				}
				if tt.wantFormat && req.Log.Format == nil {
					t.Error("expected Log.Format to be set")
				}
				if !tt.wantFormat && req.Log.Format != nil {
					t.Error("expected Log.Format to be nil")
				}
			} else if req.Log != nil {
				t.Error("expected Log to be nil")
			}

			if tt.wantWorkflow {
				if req.Workflow == nil {
					t.Error("expected Workflow to be set")
				}
			} else if req.Workflow != nil {
				t.Error("expected Workflow to be nil")
			}

			if tt.wantGit {
				if req.Git == nil || req.Git.WorktreeMode == nil {
					t.Error("expected Git.WorktreeMode to be set")
				}
			} else if req.Git != nil {
				t.Error("expected Git to be nil")
			}
		})
	}
}

func TestAgentsConfigResponse_IncludesBuiltins(t *testing.T) {
	response := AgentsConfigResponse{
		Default:  "claude",
		Claude:   FullAgentConfigResponse{Enabled: true},
		Gemini:   FullAgentConfigResponse{Enabled: false},
		Codex:    FullAgentConfigResponse{Enabled: false},
		Copilot:  FullAgentConfigResponse{Enabled: false},
		OpenCode: FullAgentConfigResponse{Enabled: false},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal agents response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal agents response: %v", err)
	}

	for _, key := range []string{"claude", "gemini", "codex", "copilot", "opencode"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected %s key in agents response", key)
		}
	}
}

func TestValidationErrorResponse_Format(t *testing.T) {
	response := ValidationErrorResponse{
		Message: "Validation failed",
		Errors: []ValidationFieldError{
			{
				Field:   "log.level",
				Message: "must be one of: debug, info, warn, error",
				Code:    "INVALID",
			},
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}

	if result["message"] != "Validation failed" {
		t.Errorf("expected message to be %q", "Validation failed")
	}

	errs, ok := result["errors"].([]interface{})
	if !ok || len(errs) != 1 {
		t.Fatalf("expected one validation error")
	}

	first, ok := errs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error entry to be object")
	}

	if first["field"] != "log.level" {
		t.Errorf("expected field to be log.level")
	}
	if first["code"] != "INVALID" {
		t.Errorf("expected code to be INVALID")
	}
}
