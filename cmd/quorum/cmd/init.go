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

# Trace configuration (for debugging workflows)
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
  include_phases: [refine, analyze, plan, execute]

# Workflow execution settings
workflow:
  timeout: 12h
  max_retries: 3
  dry_run: false
  sandbox: true
  deny_tools: []

# Phase configuration
phases:
  # Analyze phase settings
  analyze:
    timeout: 2h
    # Prompt refiner - enhances user prompt before analysis
    refiner:
      enabled: true
      agent: codex
    # Analysis synthesizer - consolidates multi-agent analyses
    synthesizer:
      agent: claude
    # Semantic moderator for multi-agent consensus evaluation
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.90
      min_rounds: 2
      max_rounds: 5
      abort_threshold: 0.30
      stagnation_threshold: 0.02
  # Plan phase settings
  plan:
    timeout: 1h
    # Plan synthesizer - when enabled, all agents with plan phase enabled
    # propose plans in parallel, then synthesizer consolidates them.
    # When disabled, uses single-agent planning with the default agent.
    synthesizer:
      enabled: true
      agent: claude
  # Execute phase settings
  execute:
    timeout: 2h

# Agent configuration
# Note: temperature and max_tokens are omitted - let each CLI use its optimized defaults
agents:
  default: claude

  # Claude (Anthropic) - Primary agent, synthesizer
  claude:
    enabled: true
    path: claude
    model: claude-opus-4-5-20251101
    # Per-task model overrides. Only specify phases that need a different model.
    # Unspecified phases use the default model above.
    phase_models:
      refine: claude-opus-4-5-20251101
      analyze: claude-opus-4-5-20251101
      moderate: claude-opus-4-5-20251101
      synthesize: claude-opus-4-5-20251101
      plan: claude-opus-4-5-20251101
      execute: claude-opus-4-5-20251101
    # Phases/roles this agent participates in
    # - refine: prompt refinement before analysis
    # - analyze: multi-agent analysis participation
    # - moderate: consensus evaluation between agents
    # - synthesize: consolidate multi-agent outputs
    # - plan: task planning
    # - execute: task execution
    phases:
      refine: false
      analyze: true
      moderate: false
      synthesize: true   # assigned as synthesizer
      plan: true
      execute: true

  # Gemini (Google) - Secondary agent
  gemini:
    enabled: true
    path: gemini
    model: gemini-3-pro-preview
    phase_models:
      refine: gemini-3-pro-preview
      analyze: gemini-3-pro-preview
      moderate: gemini-3-pro-preview
      synthesize: gemini-3-pro-preview
      plan: gemini-3-pro-preview
      execute: gemini-3-flash-preview
    # Phases/roles this agent participates in
    phases:
      refine: false
      analyze: true
      moderate: false
      synthesize: false
      plan: false
      execute: true

  # Codex (OpenAI) - Tertiary agent, refiner
  codex:
    enabled: true
    path: codex
    model: gpt-5.2-codex
    # Codex-specific: default reasoning effort (minimal/low/medium/high/xhigh)
    reasoning_effort: high
    phase_models:
      refine: gpt-5.2-codex
      analyze: gpt-5.2-codex
      moderate: gpt-5.2-codex
      synthesize: gpt-5.2-codex
      plan: gpt-5.2-codex
      execute: gpt-5.2-codex
    # Per-phase reasoning effort overrides (optional)
    reasoning_effort_phases:
      refine: xhigh
      analyze: xhigh
      plan: xhigh
    # Phases/roles this agent participates in
    phases:
      refine: true       # assigned as refiner
      analyze: true
      moderate: false
      synthesize: false
      plan: true
      execute: true

  # Copilot (GitHub) - Moderator only
  copilot:
    enabled: true
    path: copilot
    model: claude-sonnet-4-5
    phase_models:
      refine: claude-sonnet-4-5
      analyze: claude-sonnet-4-5
      moderate: claude-sonnet-4-5
      synthesize: claude-sonnet-4-5
      plan: claude-sonnet-4-5
      execute: claude-sonnet-4-5
    # Phases/roles this agent participates in - moderator only
    phases:
      refine: false
      analyze: false
      moderate: true     # assigned as moderator
      synthesize: false
      plan: false
      execute: false

# State persistence
state:
  # Storage backend: "json" (default) or "sqlite"
  backend: json
  path: .quorum/state/state.json
  backup_path: .quorum/state/state.json.bak
  lock_ttl: 1h

# Git configuration
git:
  # Directory for git worktrees (isolated directories for parallel task execution)
  worktree_dir: .worktrees
  # Auto-cleanup worktrees after task completion
  auto_clean: true
  # When to create worktrees: always | parallel | disabled
  #   always:   create worktree for every task
  #   parallel: only when multiple tasks can run concurrently (recommended)
  #   disabled: never create worktrees, all tasks share working directory
  worktree_mode: parallel
  # Task finalization: commit, push, and PR creation
  # Each task runs in its own branch (quorum/<task-id>). After completion:
  auto_commit: true
  auto_push: true
  auto_pr: true
  # IMPORTANT: auto_merge is disabled by default for safety.
  # Enable only if you want PRs merged automatically without review.
  auto_merge: false
  # Target branch for PRs (empty = repository default branch)
  pr_base_branch: ""
  # Merge strategy: merge, squash, rebase
  merge_strategy: squash

# GitHub integration
# Note: GitHub token should be provided via GITHUB_TOKEN or GH_TOKEN environment variable
github:
  remote: origin

# Chat/TUI settings
chat:
  timeout: 3m
  progress_interval: 15s
  editor: vim

# Report configuration (Markdown output)
report:
  enabled: true
  base_dir: .quorum/output
  use_utc: true
  include_raw: true
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
		// #nosec G304 -- config path is within user home directory
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
