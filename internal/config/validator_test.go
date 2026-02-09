package config

import (
	"strings"
	"testing"
)

// validConfig returns a valid configuration for testing.
func validConfig() *Config {
	return &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "auto",
		},
		Trace: TraceConfig{
			Mode:          "off",
			Dir:           ".quorum/traces",
			SchemaVersion: 1,
			Redact:        true,
			MaxBytes:      262144,
			TotalMaxBytes: 10485760,
			MaxFiles:      500,
			IncludePhases: []string{"analyze", "plan", "execute"},
		},
		Workflow: WorkflowConfig{
			Timeout:    "12h",
			MaxRetries: 3,
		},
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: true, // Default agent must be enabled for valid config
				Path:    "claude",
				Model:   "claude-sonnet-4-20250514",
				Phases: map[string]bool{
					"refine":     true,
					"analyze":    true,
					"moderate":   true,
					"synthesize": true,
					"plan":       true,
					"execute":    true,
				},
			},
			Gemini: AgentConfig{
				Enabled: false,
			},
			Codex: AgentConfig{
				Enabled: false,
			},
			Copilot: AgentConfig{
				Enabled: false,
			},
		},
		State: StateConfig{
			Path:       ".quorum/state/state.db",
			BackupPath: ".quorum/state/state.db.bak",
			LockTTL:    "1h",
		},
		Git: GitConfig{
			Worktree: WorktreeConfig{
				Dir:       ".worktrees",
				Mode:      "parallel",
				AutoClean: true,
			},
			Task: GitTaskConfig{
				AutoCommit: true, // Required when Worktree.AutoClean is true
			},
			Finalization: GitFinalizationConfig{
				AutoPush:      false,
				AutoPR:        false,
				AutoMerge:     false,
				MergeStrategy: "squash",
			},
		},
		GitHub: GitHubConfig{
			Remote: "origin",
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Timeout: "2h",
				Refiner: RefinerConfig{
					Enabled: false,
				},
				Synthesizer: SynthesizerConfig{
					Agent: "claude",
				},
				Moderator: ModeratorConfig{
					Enabled:   false,
					Threshold: 0.90,
				},
			},
			Plan: PlanPhaseConfig{
				Timeout: "1h",
			},
			Execute: ExecutePhaseConfig{
				Timeout: "2h",
			},
		},
	}
}

func TestValidator_ValidConfig(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidator_InvalidLevel(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Log.Level = "invalid"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid log level")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}

	found := false
	for _, e := range errs {
		if e.Field == "log.level" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for log.level field")
	}
}

func TestValidator_InvalidFormat(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Log.Format = "invalid"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid log format")
	}

	if !strings.Contains(err.Error(), "log.format") {
		t.Errorf("error = %v, should mention log.format", err)
	}
}

func TestValidator_InvalidPhaseModel(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Agents.Claude.PhaseModels = map[string]string{
		"invalid": "claude-sonnet-4-20250514",
	}

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid phase model")
	}

	if !strings.Contains(err.Error(), "phase_models") {
		t.Fatalf("error = %v, should mention phase_models", err)
	}
}

func TestValidator_InvalidTimeout(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Workflow.Timeout = "invalid"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid timeout")
	}

	if !strings.Contains(err.Error(), "workflow.timeout") {
		t.Errorf("error = %v, should mention workflow.timeout", err)
	}
}

func TestValidator_MaxRetriesOutOfRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value int
	}{
		{"negative", -1},
		{"too high", 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Workflow.MaxRetries = tt.value

			v := NewValidator()
			err := v.Validate(cfg)
			if err == nil {
				t.Error("Validate() error = nil, want error for invalid max_retries")
			}
		})
	}
}

func TestValidator_ModeratorThresholdOutOfRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value float64
	}{
		{"negative", -0.1},
		{"too high", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Phases.Analyze.Moderator.Enabled = true
			cfg.Phases.Analyze.Moderator.Agent = "claude"
			cfg.Phases.Analyze.Moderator.Threshold = tt.value

			v := NewValidator()
			err := v.Validate(cfg)
			if err == nil {
				t.Error("Validate() error = nil, want error for invalid threshold")
			}
		})
	}
}

func TestValidator_IssuesInvalidProvider(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Provider = "jira"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues.provider")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}

	found := false
	for _, e := range errs {
		if e.Field == "issues.provider" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for issues.provider field")
	}
}

func TestValidator_IssuesInvalidLanguage(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Template.Language = "klingon"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues.template.language")
	}

	if !strings.Contains(err.Error(), "issues.template.language") {
		t.Errorf("error = %v, should mention issues.template.language", err)
	}
}

func TestValidator_IssuesInvalidTone(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Template.Tone = "funny"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues.template.tone")
	}

	if !strings.Contains(err.Error(), "issues.template.tone") {
		t.Errorf("error = %v, should mention issues.template.tone", err)
	}
}

func TestValidator_IssuesGitLabRequiresProjectID(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Provider = "gitlab"
	cfg.Issues.GitLab.ProjectID = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing issues.gitlab.project_id")
	}

	if !strings.Contains(err.Error(), "issues.gitlab.project_id") {
		t.Errorf("error = %v, should mention issues.gitlab.project_id", err)
	}
}

func TestValidator_MultipleErrors(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Log.Level = "invalid"
	cfg.Log.Format = "invalid"
	cfg.Workflow.Timeout = "invalid"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want multiple errors")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("error type = %T, want ValidationErrors", err)
	}

	if len(errs) < 3 {
		t.Errorf("got %d errors, want at least 3", len(errs))
	}
}

func TestValidator_UnknownAgent(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Agents.Default = "unknown-agent"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for unknown agent")
	}

	if !strings.Contains(err.Error(), "agents.default") {
		t.Errorf("error = %v, should mention agents.default", err)
	}
}

func TestValidator_AgentPathRequired(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Agents.Claude.Enabled = true
	cfg.Agents.Claude.Path = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for missing agent path")
	}

	if !strings.Contains(err.Error(), "agents.claude.path") {
		t.Errorf("error = %v, should mention agents.claude.path", err)
	}
}

func TestValidator_DisabledAgentSkipsValidation(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Keep claude enabled as default, but disable gemini
	cfg.Agents.Gemini.Enabled = false
	cfg.Agents.Gemini.Path = "" // Would normally fail if enabled

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil (disabled agents should skip validation)", err)
	}
}

// Note: TestValidator_AgentTemperatureOutOfRange and TestValidator_AgentMaxTokensOutOfRange
// were removed because temperature and max_tokens are no longer part of AgentConfig.
// Each CLI tool should use its own optimized defaults.

func TestValidator_EmptyStatePath(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.State.Path = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty state path")
	}
}

func TestValidator_InvalidLockTTL(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.State.LockTTL = "not-a-duration"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid lock_ttl")
	}
}

func TestValidator_EmptyWorktreeDir(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Git.Worktree.Dir = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty worktree.dir")
	}
}

func TestValidator_EmptyGitHubRemote(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.GitHub.Remote = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty remote")
	}
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()
	err := ValidationError{
		Field:   "test.field",
		Value:   "test-value",
		Message: "test message",
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "test.field") {
		t.Error("error string should contain field name")
	}
	if !strings.Contains(errStr, "test message") {
		t.Error("error string should contain message")
	}
	if !strings.Contains(errStr, "test-value") {
		t.Error("error string should contain value")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()
	errs := ValidationErrors{
		{Field: "field1", Value: "v1", Message: "msg1"},
		{Field: "field2", Value: "v2", Message: "msg2"},
	}

	errStr := errs.Error()
	if !strings.Contains(errStr, "field1") {
		t.Error("error string should contain field1")
	}
	if !strings.Contains(errStr, "field2") {
		t.Error("error string should contain field2")
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Parallel()
	empty := ValidationErrors{}
	if empty.HasErrors() {
		t.Error("empty ValidationErrors should not have errors")
	}

	withErrors := ValidationErrors{
		{Field: "f", Value: "v", Message: "m"},
	}
	if !withErrors.HasErrors() {
		t.Error("non-empty ValidationErrors should have errors")
	}
}

func TestValidateConfig_Convenience(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	err := ValidateConfig(cfg)
	if err != nil {
		t.Errorf("ValidateConfig() error = %v, want nil", err)
	}
}

func TestValidator_SingleAgentValidConfig(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Enable single-agent mode with valid agent
	cfg.Phases.Analyze.SingleAgent.Enabled = true
	cfg.Phases.Analyze.SingleAgent.Agent = "claude"
	// Disable moderator to avoid mutual exclusivity error
	cfg.Phases.Analyze.Moderator.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidator_SingleAgentMissingAgent(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Enable single-agent mode without specifying agent
	cfg.Phases.Analyze.SingleAgent.Enabled = true
	cfg.Phases.Analyze.SingleAgent.Agent = "" // Missing agent
	cfg.Phases.Analyze.Moderator.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for missing single_agent.agent")
	}

	if !strings.Contains(err.Error(), "single_agent.agent") {
		t.Errorf("error = %v, should mention single_agent.agent", err)
	}
}

func TestValidator_SingleAgentUnknownAgent(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Enable single-agent mode with unknown agent
	cfg.Phases.Analyze.SingleAgent.Enabled = true
	cfg.Phases.Analyze.SingleAgent.Agent = "nonexistent"
	cfg.Phases.Analyze.Moderator.Enabled = false

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for unknown agent")
	}

	if !strings.Contains(err.Error(), "single_agent.agent") {
		t.Errorf("error = %v, should mention single_agent.agent", err)
	}
}

func TestValidator_SingleAgentWithDisabledAgent(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Enable single-agent mode with a disabled agent
	cfg.Phases.Analyze.SingleAgent.Enabled = true
	cfg.Phases.Analyze.SingleAgent.Agent = "gemini"
	cfg.Phases.Analyze.Moderator.Enabled = false
	cfg.Agents.Gemini.Enabled = false // Agent is disabled

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for disabled agent in single_agent mode")
	}

	if !strings.Contains(err.Error(), "single_agent.agent") {
		t.Errorf("error = %v, should mention single_agent.agent", err)
	}
}

func TestValidator_SingleAgentAndModeratorMutualExclusion(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// Enable both single-agent and moderator - should be mutually exclusive
	cfg.Phases.Analyze.SingleAgent.Enabled = true
	cfg.Phases.Analyze.SingleAgent.Agent = "claude"
	cfg.Phases.Analyze.Moderator.Enabled = true
	cfg.Phases.Analyze.Moderator.Agent = "claude"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for mutual exclusion violation")
	}

	if !strings.Contains(err.Error(), "single_agent.enabled") {
		t.Errorf("error = %v, should mention single_agent.enabled", err)
	}
}

func TestValidator_SingleAgentDisabledIsValid(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	// When single-agent is disabled, missing agent is OK
	cfg.Phases.Analyze.SingleAgent.Enabled = false
	cfg.Phases.Analyze.SingleAgent.Agent = "" // Empty is fine when disabled

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		// Error might be from other fields, check it's not from single_agent
		if strings.Contains(err.Error(), "single_agent") {
			t.Errorf("Validate() error = %v, should not error on single_agent when disabled", err)
		}
	}
}

func TestValidator_IssuesInvalidMode(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Mode = "batch"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues.mode")
	}

	if !strings.Contains(err.Error(), "issues.mode") {
		t.Errorf("error = %v, should mention issues.mode", err)
	}
}

func TestValidator_IssuesValidModes(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"direct", "agent"} {
		t.Run(mode, func(t *testing.T) {
			cfg := validConfig()
			cfg.Issues.Enabled = true
			cfg.Issues.Mode = mode

			v := NewValidator()
			err := v.Validate(cfg)
			if err != nil {
				t.Errorf("Validate() error = %v, want nil for valid mode %q", err, mode)
			}
		})
	}
}

func TestValidator_IssuesInvalidRepository(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		repo string
	}{
		{"no slash", "myrepo"},
		{"too many slashes", "owner/repo/extra"},
		{"empty owner", "/repo"},
		{"empty repo", "owner/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Issues.Enabled = true
			cfg.Issues.Repository = tt.repo

			v := NewValidator()
			err := v.Validate(cfg)
			if err == nil {
				t.Fatalf("Validate() error = nil, want error for invalid repository %q", tt.repo)
			}

			if !strings.Contains(err.Error(), "issues.repository") {
				t.Errorf("error = %v, should mention issues.repository", err)
			}
		})
	}
}

func TestValidator_IssuesValidRepository(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Repository = "owner/repo"

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil for valid repository", err)
	}
}

func TestValidator_IssuesInvalidTimeout(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Timeout = "not-a-duration"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid issues.timeout")
	}

	if !strings.Contains(err.Error(), "issues.timeout") {
		t.Errorf("error = %v, should mention issues.timeout", err)
	}
}

func TestValidator_IssuesGeneratorInvalidReasoningEffort(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Generator.Enabled = true
	cfg.Issues.Generator.Agent = "claude"
	cfg.Issues.Generator.ReasoningEffort = "ultra"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid reasoning_effort")
	}

	if !strings.Contains(err.Error(), "issues.generator.reasoning_effort") {
		t.Errorf("error = %v, should mention issues.generator.reasoning_effort", err)
	}
}

func TestValidator_IssuesGeneratorValidReasoningEffort(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Generator.Enabled = true
	cfg.Issues.Generator.Agent = "claude"
	cfg.Issues.Generator.ReasoningEffort = "high"

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil for valid reasoning_effort", err)
	}
}

func TestValidator_IssuesGeneratorInvalidAgent(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Generator.Enabled = true
	cfg.Issues.Generator.Agent = "nonexistent"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid generator agent")
	}

	if !strings.Contains(err.Error(), "issues.generator.agent") {
		t.Errorf("error = %v, should mention issues.generator.agent", err)
	}
}

func TestValidator_IssuesGeneratorNegativeMaxBodyLength(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Issues.Enabled = true
	cfg.Issues.Generator.Enabled = true
	cfg.Issues.Generator.Agent = "claude"
	cfg.Issues.Generator.MaxBodyLength = -1

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative max_body_length")
	}

	if !strings.Contains(err.Error(), "issues.generator.max_body_length") {
		t.Errorf("error = %v, should mention issues.generator.max_body_length", err)
	}
}

func TestSingleAgentConfig_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		config SingleAgentConfig
		want   bool
	}{
		{"disabled with no agent is valid", SingleAgentConfig{Enabled: false, Agent: ""}, true},
		{"disabled with agent is valid", SingleAgentConfig{Enabled: false, Agent: "claude"}, true},
		{"enabled with agent is valid", SingleAgentConfig{Enabled: true, Agent: "claude"}, true},
		{"enabled with model override is valid", SingleAgentConfig{Enabled: true, Agent: "claude", Model: "opus"}, true},
		{"enabled with no agent is invalid", SingleAgentConfig{Enabled: true, Agent: ""}, false},
		{"enabled with whitespace agent is invalid", SingleAgentConfig{Enabled: true, Agent: "   "}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
