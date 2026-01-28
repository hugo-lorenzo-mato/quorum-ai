package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
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

	// Create default config using shared constant
	if err := os.WriteFile(configPath, []byte(config.DefaultConfigYAML), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Create directories
	dirs := []string{
		".quorum",
		".quorum/state",
		".quorum/logs",
		".quorum/runs",
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
