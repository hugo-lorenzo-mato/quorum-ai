package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new quorum project",
	Long: `Initialize a new quorum project in the current directory.
Creates configuration files and directory structure.`,
	RunE: runInit,
}

var (
	initForce bool
)

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing configuration")
}

func runInit(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	// Create .quorum directory first
	quorumDir := filepath.Join(cwd, ".quorum")
	if err := os.MkdirAll(quorumDir, 0o750); err != nil {
		return fmt.Errorf("creating .quorum directory: %w", err)
	}

	configPath := filepath.Join(quorumDir, "config.yaml")

	// Also check legacy location for migration warning
	legacyConfigPath := filepath.Join(cwd, ".quorum.yaml")
	if _, err := os.Stat(legacyConfigPath); err == nil {
		fmt.Println("Note: Found legacy config at .quorum.yaml")
		fmt.Println("      Consider moving it to .quorum/config.yaml")
	}

	// Check existing config
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("configuration already exists at .quorum/config.yaml, use --force to overwrite")
	}

	// Create default config
	defaultConfig := `# Quorum AI Configuration
# Documentation: https://github.com/hugo-lorenzo-mato/quorum-ai/blob/main/docs/CONFIGURATION.md

# Logging configuration
log:
  level: info
  format: auto

# Trace configuration
trace:
  mode: off
  dir: .quorum/traces
  schema_version: 1
  redact: true
  redact_patterns: []
  redact_allowlist: []
  max_bytes: 262144
  total_max_bytes: 10485760
  max_files: 500
  include_phases: [analyze, consensus, plan, execute]

# Report configuration (Markdown output)
report:
  enabled: true
  base_dir: ".quorum/output"
  use_utc: true
  include_raw: true

# Prompt optimizer configuration
prompt_optimizer:
  enabled: true
  agent: claude
  model: "claude-opus-4-5-20251101"

# Agent configuration
# All agents are enabled by default for multi-agent consensus
agents:
  default: claude

  # Claude (Anthropic) - Primary agent
  # Uses Opus 4.5 for deep analysis
  claude:
    enabled: true
    path: "claude"
    model: "claude-opus-4-5-20251101"
    max_tokens: 32000
    phase_models:
      analyze: "claude-opus-4-5-20251101"
      plan: "claude-opus-4-5-20251101"
      execute: "claude-opus-4-5-20251101"

  # Gemini (Google) - Secondary agent
  # Uses Gemini 3 Pro (preview) for deep analysis
  gemini:
    enabled: true
    path: "gemini"
    model: "gemini-3-pro-preview"
    max_tokens: 65536
    phase_models:
      analyze: "gemini-3-pro-preview"
      plan: "gemini-3-pro-preview"
      execute: "gemini-3-flash-preview"

  # Codex (OpenAI) - Tertiary agent
  # Uses GPT-5.2 with xhigh reasoning for analysis
  # Uses GPT-5.2-codex with xhigh reasoning for planning
  # Uses GPT-5.2-codex with high reasoning for execution
  codex:
    enabled: true
    path: "codex"
    model: "gpt-5.2"
    max_tokens: 32000
    phase_models:
      analyze: "gpt-5.2"
      plan: "gpt-5.2-codex"
      execute: "gpt-5.2-codex"

  # Copilot (GitHub) - Quaternary agent
  # Uses Claude Sonnet 4.5 for analysis and planning (high quality)
  # Uses Claude Haiku 4.5 for execution (0.33x cost, fast)
  copilot:
    enabled: true
    path: "copilot"
    model: "claude-sonnet-4.5"
    max_tokens: 32000
    phase_models:
      analyze: "claude-sonnet-4.5"
      plan: "claude-sonnet-4.5"
      execute: "claude-haiku-4.5"

# Workflow settings
workflow:
  max_retries: 3
  timeout: "4h"
  # Per-phase timeouts (each phase can run up to this duration)
  phase_timeouts:
    analyze: "2h"
    plan: "2h"
    execute: "2h"

# Consensus settings
consensus:
  threshold: 0.75
  arbiter:
    enabled: true
    agent: claude
    model: "claude-opus-4-5-20251101"
    threshold: 0.90
    min_rounds: 2
    max_rounds: 2
    abort_threshold: 0.30
    stagnation_threshold: 0.02

# Consolidator settings (for analysis synthesis)
analysis_consolidator:
  agent: claude
  model: "claude-opus-4-5-20251101"

# State persistence
state:
  path: ".quorum/state/state.json"
  backup_path: ".quorum/state/state.json.bak"
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Create directories
	dirs := []string{
		".quorum",
		".quorum/state",
		".quorum/logs",
		".quorum/output",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(cwd, dir), 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Initialize agent configurations if they don't exist
	if err := initializeAgentConfigs(); err != nil {
		fmt.Printf("Warning: Could not initialize agent configs: %v\n", err)
	}

	fmt.Println("Initialized quorum project in", cwd)
	fmt.Println("Configuration file: .quorum/config.yaml")
	fmt.Println("Run 'quorum doctor' to verify setup")

	return nil
}

// initializeAgentConfigs creates default configurations for agents to prevent common issues
func initializeAgentConfigs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	// Initialize Gemini configuration
	geminiConfigDir := filepath.Join(homeDir, ".gemini")
	geminiConfigPath := filepath.Join(geminiConfigDir, "settings.json")

	// Check if Gemini config exists
	if _, err := os.Stat(geminiConfigPath); os.IsNotExist(err) {
		// Create .gemini directory
		if err := os.MkdirAll(geminiConfigDir, 0o750); err != nil {
			return fmt.Errorf("creating .gemini directory: %w", err)
		}

		// Create minimal valid configuration
		defaultGeminiConfig := map[string]interface{}{
			"security": map[string]interface{}{
				"auth": map[string]interface{}{
					"selectedType": "oauth-personal",
				},
				"folderTrust": map[string]interface{}{
					"enabled": true,
				},
			},
			"ui": map[string]interface{}{
				"theme": "Atom One",
			},
			"general": map[string]interface{}{
				"previewFeatures": true,
				"vimMode":         false,
			},
		}

		configBytes, err := json.MarshalIndent(defaultGeminiConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling gemini config: %w", err)
		}

		if err := os.WriteFile(geminiConfigPath, configBytes, 0o600); err != nil {
			return fmt.Errorf("writing gemini config: %w", err)
		}
	} else if err == nil {
		// Config exists, check if it has the problematic "disabled": true
		configBytes, err := os.ReadFile(geminiConfigPath)
		if err != nil {
			return fmt.Errorf("reading existing gemini config: %w", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return fmt.Errorf("parsing existing gemini config: %w", err)
		}

		// Check for and remove "disabled": true at root level
		if disabled, exists := config["disabled"]; exists && disabled == true {
			delete(config, "disabled")

			// Write back the corrected config
			configBytes, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling corrected gemini config: %w", err)
			}

			if err := os.WriteFile(geminiConfigPath, configBytes, 0o600); err != nil {
				return fmt.Errorf("writing corrected gemini config: %w", err)
			}
		}
	}

	return nil
}
