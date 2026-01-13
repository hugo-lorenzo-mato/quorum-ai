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
			IncludePhases: []string{"analyze", "consensus", "plan", "execute"},
		},
		Workflow: WorkflowConfig{
			Timeout:    "2h",
			MaxRetries: 3,
		},
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled:     true,
				Path:        "claude",
				Model:       "claude-sonnet-4-20250514",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			Gemini: AgentConfig{
				Enabled:     true,
				Path:        "gemini",
				Model:       "gemini-2.5-flash",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			Codex: AgentConfig{
				Enabled: false,
			},
			Copilot: AgentConfig{
				Enabled: false,
			},
			Aider: AgentConfig{
				Enabled: false,
			},
		},
		State: StateConfig{
			Path:       ".quorum/state/state.json",
			BackupPath: ".quorum/state/state.json.bak",
			LockTTL:    "1h",
		},
		Git: GitConfig{
			WorktreeDir: ".worktrees",
			AutoClean:   true,
		},
		GitHub: GitHubConfig{
			Remote: "origin",
		},
		Consensus: ConsensusConfig{
			Threshold: 0.75,
			Weights: ConsensusWeight{
				Claims:          0.40,
				Risks:           0.30,
				Recommendations: 0.30,
			},
		},
		Costs: CostsConfig{
			MaxPerWorkflow: 10.0,
			MaxPerTask:     2.0,
			AlertThreshold: 0.80,
		},
	}
}

func TestValidator_ValidConfig(t *testing.T) {
	cfg := validConfig()
	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidator_InvalidLevel(t *testing.T) {
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

func TestValidator_WeightSum(t *testing.T) {
	cfg := validConfig()
	cfg.Consensus.Weights.Claims = 0.5
	cfg.Consensus.Weights.Risks = 0.5
	cfg.Consensus.Weights.Recommendations = 0.5 // Total = 1.5, should fail

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for weights not summing to 1.0")
	}

	if !strings.Contains(err.Error(), "consensus.weights") {
		t.Errorf("error = %v, should mention consensus.weights", err)
	}
}

func TestValidator_WeightOutOfRange(t *testing.T) {
	cfg := validConfig()
	cfg.Consensus.Weights.Claims = -0.1

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for negative weight")
	}
}

func TestValidator_ThresholdOutOfRange(t *testing.T) {
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
			cfg.Consensus.Threshold = tt.value

			v := NewValidator()
			err := v.Validate(cfg)
			if err == nil {
				t.Error("Validate() error = nil, want error for invalid threshold")
			}
		})
	}
}

func TestValidator_MultipleErrors(t *testing.T) {
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

func TestValidator_AgentTemperatureOutOfRange(t *testing.T) {
	cfg := validConfig()
	cfg.Agents.Claude.Temperature = 3.0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for temperature > 2")
	}
}

func TestValidator_AgentMaxTokensOutOfRange(t *testing.T) {
	cfg := validConfig()
	cfg.Agents.Claude.MaxTokens = -1

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for negative max_tokens")
	}
}

func TestValidator_EmptyStatePath(t *testing.T) {
	cfg := validConfig()
	cfg.State.Path = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty state path")
	}
}

func TestValidator_InvalidLockTTL(t *testing.T) {
	cfg := validConfig()
	cfg.State.LockTTL = "not-a-duration"

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for invalid lock_ttl")
	}
}

func TestValidator_EmptyWorktreeDir(t *testing.T) {
	cfg := validConfig()
	cfg.Git.WorktreeDir = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty worktree_dir")
	}
}

func TestValidator_EmptyGitHubRemote(t *testing.T) {
	cfg := validConfig()
	cfg.GitHub.Remote = ""

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for empty remote")
	}
}

func TestValidationError_Error(t *testing.T) {
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
	cfg := validConfig()
	err := ValidateConfig(cfg)
	if err != nil {
		t.Errorf("ValidateConfig() error = %v, want nil", err)
	}
}

func TestValidator_NegativeMaxPerWorkflow(t *testing.T) {
	cfg := validConfig()
	cfg.Costs.MaxPerWorkflow = -1.0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for negative max_per_workflow")
	}

	if !strings.Contains(err.Error(), "costs.max_per_workflow") {
		t.Errorf("error = %v, should mention costs.max_per_workflow", err)
	}
}

func TestValidator_NegativeMaxPerTask(t *testing.T) {
	cfg := validConfig()
	cfg.Costs.MaxPerTask = -1.0

	v := NewValidator()
	err := v.Validate(cfg)
	if err == nil {
		t.Error("Validate() error = nil, want error for negative max_per_task")
	}

	if !strings.Contains(err.Error(), "costs.max_per_task") {
		t.Errorf("error = %v, should mention costs.max_per_task", err)
	}
}

func TestValidator_AlertThresholdOutOfRange(t *testing.T) {
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
			cfg.Costs.AlertThreshold = tt.value

			v := NewValidator()
			err := v.Validate(cfg)
			if err == nil {
				t.Error("Validate() error = nil, want error for invalid alert_threshold")
			}
		})
	}
}

func TestValidator_ZeroCostsAreValid(t *testing.T) {
	cfg := validConfig()
	cfg.Costs.MaxPerWorkflow = 0
	cfg.Costs.MaxPerTask = 0

	v := NewValidator()
	err := v.Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil (zero costs should be valid for unlimited)", err)
	}
}
