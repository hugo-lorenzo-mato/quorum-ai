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
#
# Values not specified here use sensible defaults. See docs for all options.

# Phase configuration
phases:
  analyze:
    # Prompt refiner - enhances user prompt before analysis
    refiner:
      enabled: true
      agent: claude
    # Analysis synthesizer - consolidates multi-agent analyses
    synthesizer:
      agent: claude
    # Semantic moderator for multi-agent consensus evaluation
    moderator:
      enabled: true
      agent: copilot
      threshold: 0.90
  plan:
    # Plan synthesizer - when enabled, agents propose plans in parallel,
    # then synthesizer consolidates. When disabled, single-agent planning.
    synthesizer:
      enabled: true
      agent: claude

# Agent configuration
# Phases use opt-in model: only phases set to true are enabled.
# If phases is empty or omitted, agent is enabled for all phases.
# Available phases: refine, analyze, moderate, synthesize, plan, execute
agents:
  default: claude

  claude:
    enabled: true
    path: claude
    model: claude-opus-4-5-20251101
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true

  gemini:
    enabled: true
    path: gemini
    model: gemini-3-pro-preview
    # Use faster model for execution
    phase_models:
      execute: gemini-3-flash-preview
    phases:
      analyze: true
      execute: true

  codex:
    enabled: true
    path: codex
    model: gpt-5.2-codex
    reasoning_effort: high
    reasoning_effort_phases:
      refine: xhigh
      analyze: xhigh
      plan: xhigh
    phases:
      refine: true
      analyze: true
      moderate: true
      synthesize: true
      plan: true
      execute: true

  copilot:
    enabled: true
    path: copilot
    model: claude-sonnet-4-5
    phases:
      moderate: true

# Git configuration
# Tasks run in isolated worktrees on branch quorum/<task-id>.
# After completion: commit -> push -> PR (configurable).
git:
  # When to create worktrees: always | parallel | disabled
  worktree_mode: parallel
  # Post-task finalization
  auto_commit: true
  auto_push: true
  auto_pr: true
  # PR target branch (empty = repository default)
  pr_base_branch: ""
  # Auto-merge disabled by default for safety
  auto_merge: false
  merge_strategy: squash
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
