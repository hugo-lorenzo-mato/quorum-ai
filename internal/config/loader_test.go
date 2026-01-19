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
	if cfg.Workflow.PhaseTimeouts.Analyze != "2h" {
		t.Errorf("Workflow.PhaseTimeouts.Analyze = %q, want %q", cfg.Workflow.PhaseTimeouts.Analyze, "2h")
	}
	if cfg.Workflow.PhaseTimeouts.Plan != "2h" {
		t.Errorf("Workflow.PhaseTimeouts.Plan = %q, want %q", cfg.Workflow.PhaseTimeouts.Plan, "2h")
	}
	if cfg.Workflow.PhaseTimeouts.Execute != "2h" {
		t.Errorf("Workflow.PhaseTimeouts.Execute = %q, want %q", cfg.Workflow.PhaseTimeouts.Execute, "2h")
	}
	if cfg.Workflow.MaxRetries != 3 {
		t.Errorf("Workflow.MaxRetries = %d, want %d", cfg.Workflow.MaxRetries, 3)
	}

	// Verify agent defaults
	if cfg.Agents.Default != "claude" {
		t.Errorf("Agents.Default = %q, want %q", cfg.Agents.Default, "claude")
	}
	// All agents disabled by default - config file must explicitly enable them
	if cfg.Agents.Claude.Enabled {
		t.Error("Agents.Claude.Enabled = true, want false (default)")
	}

	// Verify consensus defaults
	if cfg.Consensus.Threshold != 0.80 {
		t.Errorf("Consensus.Threshold = %f, want %f", cfg.Consensus.Threshold, 0.80)
	}
}

func TestLoader_EnvOverride(t *testing.T) {
	// Set environment variables
	os.Setenv("QUORUM_LOG_LEVEL", "debug")
	os.Setenv("QUORUM_WORKFLOW_MAX_RETRIES", "5")
	os.Setenv("QUORUM_CONSENSUS_THRESHOLD", "0.9")
	defer func() {
		os.Unsetenv("QUORUM_LOG_LEVEL")
		os.Unsetenv("QUORUM_WORKFLOW_MAX_RETRIES")
		os.Unsetenv("QUORUM_CONSENSUS_THRESHOLD")
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
	if cfg.Consensus.Threshold != 0.9 {
		t.Errorf("Consensus.Threshold = %f, want %f", cfg.Consensus.Threshold, 0.9)
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
consensus:
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
	if cfg.Consensus.Threshold != 0.85 {
		t.Errorf("Consensus.Threshold = %f, want %f", cfg.Consensus.Threshold, 0.85)
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
consensus:
  threshold: 0.80
  weights:
    claims: 0.50
    risks: 0.25
    recommendations: 0.25
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	loader := NewLoader().WithConfigFile(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Consensus.Threshold != 0.80 {
		t.Errorf("Consensus.Threshold = %f, want %f", cfg.Consensus.Threshold, 0.80)
	}
	if cfg.Consensus.Weights.Claims != 0.50 {
		t.Errorf("Consensus.Weights.Claims = %f, want %f", cfg.Consensus.Weights.Claims, 0.50)
	}
	if cfg.Consensus.Weights.Risks != 0.25 {
		t.Errorf("Consensus.Weights.Risks = %f, want %f", cfg.Consensus.Weights.Risks, 0.25)
	}
	if cfg.Consensus.Weights.Recommendations != 0.25 {
		t.Errorf("Consensus.Weights.Recommendations = %f, want %f", cfg.Consensus.Weights.Recommendations, 0.25)
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
	if cfg.Consensus.Threshold != 0.80 {
		t.Errorf("Consensus.Threshold = %f, want %f", cfg.Consensus.Threshold, 0.80)
	}
	if cfg.Costs.MaxPerWorkflow != 10.0 {
		t.Errorf("Costs.MaxPerWorkflow = %f, want %f", cfg.Costs.MaxPerWorkflow, 10.0)
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
