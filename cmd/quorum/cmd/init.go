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

# Agent configuration
agents:
  default: claude
  claude:
    enabled: true
    path: "claude"
    model: "claude-sonnet-4-20250514"
    phase_models:
      analyze: "claude-opus-4-20250514"
      plan: "claude-sonnet-4-20250514"
      execute: "claude-3-5-haiku-20241022"
  gemini:
    enabled: true
    path: "gemini"
    model: "gemini-2.5-flash"
    phase_models:
      analyze: "gemini-2.5-pro"
      plan: "gemini-2.5-flash"
      execute: "gemini-2.5-flash"
  codex:
    enabled: true
    path: "codex"
    model: "gpt-5.1-codex"
    phase_models:
      analyze: "gpt-5.1-codex"
      plan: "gpt-5.1-codex"
      execute: "gpt-5.1-codex-mini"

# Workflow settings
workflow:
  max_retries: 3
  timeout: "30m"

# Consensus settings
consensus:
  threshold: 0.75

# State persistence
state:
  path: ".quorum/state/state.json"
  backup_path: ".quorum/state/state.json.bak"
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil { //nolint:gosec // Config file needs to be readable
		return fmt.Errorf("writing config: %w", err)
	}

	// Create directories
	dirs := []string{
		".quorum",
		".quorum/state",
		".quorum/logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(cwd, dir), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	fmt.Println("Initialized quorum project in", cwd)
	fmt.Println("Configuration file: .quorum.yaml")
	fmt.Println("Run 'quorum doctor' to verify setup")

	return nil
}
