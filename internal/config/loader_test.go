package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_Defaults(t *testing.T) {
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "auto" {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "auto")
	}

	// Verify workflow defaults
	if cfg.Workflow.Timeout != "12h" {
		t.Errorf("Workflow.Timeout = %q, want %q", cfg.Workflow.Timeout, "12h")
	}
	if cfg.Phases.Analyze.Timeout != "2h" {
		t.Errorf("Phases.Analyze.Timeout = %q, want %q", cfg.Phases.Analyze.Timeout, "2h")
	}
	if cfg.Phases.Plan.Timeout != "1h" {
		t.Errorf("Phases.Plan.Timeout = %q, want %q", cfg.Phases.Plan.Timeout, "1h")
	}
	if cfg.Phases.Execute.Timeout != "2h" {
		t.Errorf("Phases.Execute.Timeout = %q, want %q", cfg.Phases.Execute.Timeout, "2h")
	}
	if cfg.Workflow.MaxRetries != 3 {
		t.Errorf("Workflow.MaxRetries = %d, want %d", cfg.Workflow.MaxRetries, 3)
	}

	// Verify agent defaults
	// agents.default has NO default - user must configure explicitly
	if cfg.Agents.Default != "" {
		t.Errorf("Agents.Default = %q, want empty (no default)", cfg.Agents.Default)
	}
	// All agents disabled by default - config file must explicitly enable them
	if cfg.Agents.Claude.Enabled {
		t.Error("Agents.Claude.Enabled = true, want false (default)")
	}
	// Agent models have NO default - user must configure explicitly
	if cfg.Agents.Claude.Model != "" {
		t.Errorf("Agents.Claude.Model = %q, want empty (no default)", cfg.Agents.Claude.Model)
	}

	// Verify moderator defaults
	if cfg.Phases.Analyze.Moderator.Threshold != 0.80 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.80)
	}
}

func TestLoader_EnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("QUORUM_LOG_LEVEL", "debug")
	os.Setenv("QUORUM_WORKFLOW_MAX_RETRIES", "5")
	os.Setenv("QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD", "0.95")
	defer func() {
		os.Unsetenv("QUORUM_LOG_LEVEL")
		os.Unsetenv("QUORUM_WORKFLOW_MAX_RETRIES")
		os.Unsetenv("QUORUM_PHASES_ANALYZE_MODERATOR_THRESHOLD")
	}()

	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify environment overrides
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
	if cfg.Workflow.MaxRetries != 5 {
		t.Errorf("Workflow.MaxRetries = %d, want %d", cfg.Workflow.MaxRetries, 5)
	}
	if cfg.Phases.Analyze.Moderator.Threshold != 0.95 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.95)
	}
}

func TestLoader_MissingConfig(t *testing.T) {
	// Create a loader without any config file
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (should use defaults)", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Should have loaded defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q (default)", cfg.Log.Level, "info")
	}
}

func TestLoader_ConfigFileOverride(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
log:
  level: warn
  format: json
workflow:
  timeout: "4h"
  max_retries: 10
agents:
  default: gemini
phases:
  analyze:
    moderator:
      threshold: 0.85
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify file overrides
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "warn")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want %q", cfg.Log.Format, "json")
	}
	if cfg.Workflow.Timeout != "4h" {
		t.Errorf("Workflow.Timeout = %q, want %q", cfg.Workflow.Timeout, "4h")
	}
	if cfg.Workflow.MaxRetries != 10 {
		t.Errorf("Workflow.MaxRetries = %d, want %d", cfg.Workflow.MaxRetries, 10)
	}
	if cfg.Agents.Default != "gemini" {
		t.Errorf("Agents.Default = %q, want %q", cfg.Agents.Default, "gemini")
	}
	// Config file explicitly sets threshold to 0.85
	if cfg.Phases.Analyze.Moderator.Threshold != 0.85 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.85)
	}
}

func TestLoader_Precedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	// Config file sets level to "warn"
	configContent := `
log:
  level: warn
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Environment sets level to "debug" (should override file)
	os.Setenv("QUORUM_LOG_LEVEL", "debug")
	defer os.Unsetenv("QUORUM_LOG_LEVEL")

	loader := NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Environment should take precedence over config file
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q (env should override file)", cfg.Log.Level, "debug")
	}
}

func TestLoader_InvalidConfigFile(t *testing.T) {
	// Create a temporary invalid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid-config.yaml")

	invalidContent := `
log:
  level: [invalid yaml
`
	if err := os.WriteFile(configPath, []byte(invalidContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	_, err := loader.Load()
	if err == nil {
		t.Error("Load() with invalid config should return error")
	}
}

func TestLoader_ConfigFileUsed(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `log:
  level: info
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	_, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	usedFile := loader.ConfigFile()
	if usedFile != configPath {
		t.Errorf("ConfigFile() = %q, want %q", usedFile, configPath)
	}
}

func TestLoader_NestedConfig(t *testing.T) {
	// Create a temporary config file with nested values
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
phases:
  analyze:
    timeout: "3h"
    moderator:
      enabled: true
      agent: claude
      threshold: 0.85
      min_rounds: 2
      max_rounds: 4
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Phases.Analyze.Timeout != "3h" {
		t.Errorf("Phases.Analyze.Timeout = %q, want %q", cfg.Phases.Analyze.Timeout, "3h")
	}
	// Config file explicitly sets threshold to 0.85
	if cfg.Phases.Analyze.Moderator.Threshold != 0.85 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.85)
	}
	if cfg.Phases.Analyze.Moderator.MinRounds != 2 {
		t.Errorf("Phases.Analyze.Moderator.MinRounds = %d, want %d", cfg.Phases.Analyze.Moderator.MinRounds, 2)
	}
	if cfg.Phases.Analyze.Moderator.MaxRounds != 4 {
		t.Errorf("Phases.Analyze.Moderator.MaxRounds = %d, want %d", cfg.Phases.Analyze.Moderator.MaxRounds, 4)
	}
}

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
	if loader.v == nil {
		t.Error("NewLoader() viper instance is nil")
	}
	if loader.envPrefix != "QUORUM" {
		t.Errorf("NewLoader() envPrefix = %q, want %q", loader.envPrefix, "QUORUM")
	}
}

func TestLoader_WithEnvPrefix(t *testing.T) {
	// Set environment variable with custom prefix
	os.Setenv("CUSTOM_LOG_LEVEL", "error")
	defer os.Unsetenv("CUSTOM_LOG_LEVEL")

	loader := NewLoader().WithEnvPrefix("CUSTOM")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Log.Level != "error" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "error")
	}
}

func TestLoader_DefaultConfigFile(t *testing.T) {
	// Test loading the default config file
	loader := NewLoader().WithConfigFile("../../configs/default.yaml")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify it can be validated
	v := NewValidator()
	if err := v.Validate(cfg); err != nil {
		t.Errorf("Validate() error = %v, default config should be valid", err)
	}

	// Verify key values match defaults
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Agents.Default != "claude" {
		t.Errorf("Agents.Default = %q, want %q", cfg.Agents.Default, "claude")
	}
	if cfg.Phases.Analyze.Moderator.Threshold != 0.80 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.80)
	}
}

func TestDefaultConfig_SandboxEnabled(t *testing.T) {
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.Workflow.Sandbox {
		t.Error("Expected workflow.sandbox to default to true")
	}
}

func TestLoader_ModeratorDefaults(t *testing.T) {
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify moderator defaults
	// Moderator is disabled by default - user must enable and configure in config file
	if cfg.Phases.Analyze.Moderator.Enabled {
		t.Error("Phases.Analyze.Moderator.Enabled = true, want false (default)")
	}
	// Agent has NO default - user must configure it explicitly in config file
	// Model is resolved from agent's phase_models.analyze at runtime
	if cfg.Phases.Analyze.Moderator.Agent != "" {
		t.Errorf("Phases.Analyze.Moderator.Agent = %q, want empty (no default)", cfg.Phases.Analyze.Moderator.Agent)
	}
	// Numeric thresholds have sensible defaults
	if cfg.Phases.Analyze.Moderator.Threshold != 0.80 {
		t.Errorf("Phases.Analyze.Moderator.Threshold = %f, want %f", cfg.Phases.Analyze.Moderator.Threshold, 0.80)
	}
	if cfg.Phases.Analyze.Moderator.MinRounds != 2 {
		t.Errorf("Phases.Analyze.Moderator.MinRounds = %d, want %d", cfg.Phases.Analyze.Moderator.MinRounds, 2)
	}
	if cfg.Phases.Analyze.Moderator.MaxRounds != 5 {
		t.Errorf("Phases.Analyze.Moderator.MaxRounds = %d, want %d", cfg.Phases.Analyze.Moderator.MaxRounds, 5)
	}
	if cfg.Phases.Analyze.Moderator.AbortThreshold != 0.30 {
		t.Errorf("Phases.Analyze.Moderator.AbortThreshold = %f, want %f", cfg.Phases.Analyze.Moderator.AbortThreshold, 0.30)
	}
	if cfg.Phases.Analyze.Moderator.StagnationThreshold != 0.02 {
		t.Errorf("Phases.Analyze.Moderator.StagnationThreshold = %f, want %f", cfg.Phases.Analyze.Moderator.StagnationThreshold, 0.02)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: true,
				Model:   "claude-opus-4-5",
				PhaseModels: map[string]string{
					"refine":  "claude-opus-4-5",
					"analyze": "claude-opus-4-5",
					"plan":    "claude-opus-4-5",
				},
			},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner: RefinerConfig{
					Enabled: true,
					Agent:   "claude",
				},
				Synthesizer: SynthesizerConfig{
					Agent: "claude",
				},
				Moderator: ModeratorConfig{
					Enabled: false,
				},
			},
			Plan: PlanPhaseConfig{
				Synthesizer: PlanSynthesizerConfig{
					Enabled: false,
				},
			},
		},
	}

	err := Validate(cfg)
	if err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestValidate_MissingDefaultAgent(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "",
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error when agents.default is missing")
	}
}

func TestValidate_DefaultAgentNotEnabled(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: false,
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error when default agent is disabled")
	}
}

func TestValidate_RefinerAgentMissing(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: true,
				Model:   "claude-opus-4-5",
			},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner: RefinerConfig{
					Enabled: true,
					Agent:   "", // Missing agent
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error when phases.analyze.refiner.agent is missing")
	}
}

func TestValidate_RefinerAgentNoModel(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: true,
				Model:   "", // No model
			},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner: RefinerConfig{
					Enabled: true,
					Agent:   "claude",
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error when refiner agent has no model for refine phase")
	}
}

func TestValidate_ModeratorAgentMissing(t *testing.T) {
	cfg := &Config{
		Agents: AgentsConfig{
			Default: "claude",
			Claude: AgentConfig{
				Enabled: true,
				Model:   "claude-opus-4-5",
			},
		},
		Phases: PhasesConfig{
			Analyze: AnalyzePhaseConfig{
				Refiner: RefinerConfig{
					Enabled: false,
				},
				Synthesizer: SynthesizerConfig{
					Agent: "claude",
				},
				Moderator: ModeratorConfig{
					Enabled: true,
					Agent:   "", // Missing agent
				},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("Validate() should return error when phases.analyze.moderator.agent is missing")
	}
}
