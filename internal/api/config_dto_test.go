package api

import (
	"encoding/json"
	"testing"
)

func TestFullConfigResponse_Marshaling(t *testing.T) {
	t.Parallel()
	response := FullConfigResponse{
		Workflow: WorkflowConfigResponse{
			Timeout:    "1h",
			MaxRetries: 3,
			DenyTools:  []string{},
		},
		Agents: AgentsConfigResponse{
			Default: "claude",
			Claude: FullAgentConfigResponse{
				Enabled: true,
				Model:   "claude-opus-4-6",
				Path:    "claude",
			},
			Copilot: FullAgentConfigResponse{
				Enabled: false,
				Model:   "gpt-5.2-codex",
				Path:    "copilot",
			},
		},
		Git: GitConfigResponse{
			Worktree: WorktreeConfigResponse{
				Dir:       ".worktrees",
				Mode:      "parallel",
				AutoClean: false,
			},
			Task: GitTaskConfigResponse{
				AutoCommit: false,
			},
			Finalization: GitFinalizationConfigResponse{
				AutoPush:      false,
				AutoPR:        false,
				AutoMerge:     false,
				MergeStrategy: "squash",
			},
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
	t.Parallel()
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
			jsonBody:     `{"git": {"worktree": {"mode": ""}}}`,
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
				if req.Git == nil || req.Git.Worktree == nil || req.Git.Worktree.Mode == nil {
					t.Error("expected Git.Worktree.Mode to be set")
				}
			} else if req.Git != nil {
				t.Error("expected Git to be nil")
			}
		})
	}
}

func TestAgentsConfigResponse_IncludesBuiltins(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestIssuesConfigResponse_Marshaling(t *testing.T) {
	t.Parallel()
	response := IssuesConfigResponse{
		Enabled:        true,
		Provider:       "github",
		AutoGenerate:   true,
		Timeout:        "5m",
		Mode:           "agent",
		DraftDirectory: "custom/issues",
		Repository:     "owner/repo",
		ParentPrompt: "epic",
		Prompt: IssuePromptConfigResponse{
			Language:           "english",
			Tone:               "technical",
			IncludeDiagrams:    true,
			TitleFormat:        "[quorum] {task_name}",
			BodyPromptFile:     "prompt.md",
			Convention:         "conventional",
			CustomInstructions: "Be concise",
		},
		Labels:    []string{"quorum", "automated"},
		Assignees: []string{"dev1"},
		GitLab: GitLabIssueConfigResponse{
			UseEpics:  true,
			ProjectID: "12345",
		},
		Generator: IssueGeneratorConfigResponse{
			Enabled:           true,
			Agent:             "claude",
			Model:             "opus",
			Summarize:         true,
			MaxBodyLength:     50000,
			ReasoningEffort:   "high",
			Instructions:      "Generate detailed issues",
			TitleInstructions: "Use conventional format",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal issues response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal issues response: %v", err)
	}

	// Verify key JSON field names
	for _, key := range []string{
		"enabled", "provider", "auto_generate", "timeout",
		"mode", "draft_directory", "repository", "parent_prompt",
		"prompt", "default_labels", "default_assignees",
		"gitlab", "generator",
	} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in issues response", key)
		}
	}

	// Verify nested prompt fields
	tmpl, ok := result["prompt"].(map[string]interface{})
	if !ok {
		t.Fatal("expected prompt to be an object")
	}
	for _, key := range []string{"language", "tone", "include_diagrams", "title_format", "body_prompt_file", "convention", "custom_instructions"} {
		if _, ok := tmpl[key]; !ok {
			t.Errorf("expected key %q in prompt", key)
		}
	}

	// Verify generator fields
	gen, ok := result["generator"].(map[string]interface{})
	if !ok {
		t.Fatal("expected generator to be an object")
	}
	for _, key := range []string{"enabled", "agent", "model", "summarize", "max_body_length", "reasoning_effort", "instructions", "title_instructions"} {
		if _, ok := gen[key]; !ok {
			t.Errorf("expected key %q in generator", key)
		}
	}
}

func TestIssuesConfigUpdate_PointerFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		jsonBody   string
		wantIssues bool
	}{
		{
			name:       "issues enabled",
			jsonBody:   `{"issues": {"enabled": true}}`,
			wantIssues: true,
		},
		{
			name:       "issues not set",
			jsonBody:   `{"log": {"level": "debug"}}`,
			wantIssues: false,
		},
		{
			name:       "issues with prompt",
			jsonBody:   `{"issues": {"prompt": {"language": "spanish"}}}`,
			wantIssues: true,
		},
		{
			name:       "issues with generator",
			jsonBody:   `{"issues": {"generator": {"enabled": true, "agent": "claude"}}}`,
			wantIssues: true,
		},
		{
			name:       "issues with gitlab",
			jsonBody:   `{"issues": {"gitlab": {"use_epics": true}}}`,
			wantIssues: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req FullConfigUpdate
			if err := json.Unmarshal([]byte(tt.jsonBody), &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}

			if tt.wantIssues {
				if req.Issues == nil {
					t.Fatal("expected Issues to be set")
				}
			} else if req.Issues != nil {
				t.Error("expected Issues to be nil")
			}
		})
	}
}

func TestFullConfigResponse_IssuesSection(t *testing.T) {
	t.Parallel()
	response := FullConfigResponse{
		Issues: IssuesConfigResponse{
			Enabled:  true,
			Provider: "github",
			Labels:   []string{"quorum"},
			Assignees: []string{},
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

	if _, ok := result["issues"]; !ok {
		t.Error("expected 'issues' key in full config response")
	}
}
