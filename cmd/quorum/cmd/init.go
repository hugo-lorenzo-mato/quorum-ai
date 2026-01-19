package cmd

import (
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

	configPath := filepath.Join(cwd, ".quorum.yaml")

	// Check existing config
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("configuration already exists, use --force to overwrite")
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
  base_dir: ".quorum-output"
  use_utc: true
  include_raw: true

# Prompt optimizer configuration
optimizer:
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
    phase_models:
      analyze: "claude-sonnet-4.5"
      plan: "claude-sonnet-4.5"
      execute: "claude-haiku-4.5"

# Workflow settings
workflow:
  max_retries: 3
  timeout: "30m"

# Consensus settings
consensus:
  threshold: 0.75

# Consolidator settings (for analysis synthesis)
consolidator:
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
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(cwd, dir), 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	fmt.Println("Initialized quorum project in", cwd)
	fmt.Println("Configuration file: .quorum.yaml")
	fmt.Println("Run 'quorum doctor' to verify setup")

	return nil
}
