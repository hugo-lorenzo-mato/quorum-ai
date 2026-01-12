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

func runInit(cmd *cobra.Command, args []string) error {
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
version: "1"

# Agent configuration
agents:
  claude:
    enabled: true
    path: "claude"
    model: "claude-sonnet-4-20250514"
  gemini:
    enabled: true
    path: "gemini"
    model: "gemini-2.0-flash"

# Workflow settings
workflow:
  consensus_threshold: 0.75
  max_retries: 3
  timeout: "30m"

# Output settings
output:
  format: "auto"
  verbose: false
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Create directories
	dirs := []string{
		".quorum",
		".quorum/state",
		".quorum/logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(cwd, dir), 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	fmt.Println("Initialized quorum project in", cwd)
	fmt.Println("Configuration file: .quorum.yaml")
	fmt.Println("Run 'quorum doctor' to verify setup")

	return nil
}
