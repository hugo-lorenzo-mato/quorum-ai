package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system dependencies",
	Long:  "Verify that all required dependencies are installed and configured.",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(_ *cobra.Command, _ []string) error {
	checks := []struct {
		name     string
		command  string
		args     []string
		required bool
	}{
		{"git", "git", []string{"--version"}, true},
		{"gh", "gh", []string{"--version"}, false},
		{"claude", "claude", []string{"--version"}, false},
		{"gemini", "gemini", []string{"--version"}, false},
		{"codex", "codex", []string{"--version"}, false},
		{"copilot", "copilot", []string{"--version"}, false},
	}

	fmt.Println("Checking dependencies...")
	fmt.Println()

	allOk := true
	requiredOk := true

	for _, check := range checks {
		status := checkCommand(check.command, check.args)
		icon := "✓"
		suffix := ""

		if !status {
			if check.required {
				icon = "✗"
				requiredOk = false
			} else {
				icon = "○"
				suffix = " (optional)"
			}
		}

		if !status && check.required {
			allOk = false
		}

		fmt.Printf("  %s %s%s\n", icon, check.name, suffix)
	}

	fmt.Println()

	// Check agent configurations (external config files like ~/.gemini/settings.json)
	fmt.Println("Checking agent configurations...")
	fmt.Println()

	configIssues := checkAgentConfigs()
	if len(configIssues) > 0 {
		for _, issue := range configIssues {
			fmt.Printf("  ⚠ %s\n", issue)
		}
		fmt.Println()
		fmt.Println("Recommendation: Run 'quorum init --force' to fix configuration issues")
		fmt.Println()
	} else {
		fmt.Println("  ✓ All agent configurations valid")
		fmt.Println()
	}

	// Validate quorum configuration (fail-fast check for phase consistency)
	fmt.Println("Validating quorum configuration...")
	fmt.Println()

	validationIssues := validateQuorumConfig()
	if len(validationIssues) > 0 {
		for _, issue := range validationIssues {
			fmt.Printf("  ✗ %s\n", issue)
		}
		fmt.Println()
		fmt.Println("Configuration errors must be fixed before running workflows.")
		fmt.Println("Edit .quorum/config.yaml to fix the issues above.")
		fmt.Println()
		allOk = false
	} else {
		fmt.Println("  ✓ Quorum configuration valid")
		fmt.Println()
	}

	if !requiredOk {
		fmt.Println("Some required dependencies are missing")
		return fmt.Errorf("dependency check failed")
	}

	if allOk && len(configIssues) == 0 {
		fmt.Println("All dependencies available and configurations valid")
	} else if allOk {
		fmt.Println("Required dependencies available, but some configuration issues found")
	} else {
		fmt.Println("Required dependencies available, some optional tools missing")
	}

	return nil
}

func checkCommand(name string, args []string) bool {
	cmd := exec.Command(name, args...)
	return cmd.Run() == nil
}

// validateQuorumConfig loads and validates the quorum configuration
func validateQuorumConfig() []string {
	var issues []string

	// Try to load config using the loader
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot load config: %v", err))
		return issues
	}

	// Run full validation
	if err := config.ValidateConfig(cfg); err != nil {
		// Parse validation errors
		if verrs, ok := err.(config.ValidationErrors); ok {
			for _, verr := range verrs {
				issues = append(issues, verr.Error())
			}
		} else {
			issues = append(issues, err.Error())
		}
	}

	return issues
}

// checkAgentConfigs validates agent configurations and returns a list of issues
func checkAgentConfigs() []string {
	var issues []string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		issues = append(issues, "Cannot access home directory")
		return issues
	}

	// Check Gemini configuration
	geminiConfigPath := filepath.Join(homeDir, ".gemini", "settings.json")
	if _, err := os.Stat(geminiConfigPath); err == nil {
		// #nosec G304 -- config path is within user home directory
		configBytes, err := os.ReadFile(geminiConfigPath)
		if err != nil {
			issues = append(issues, "Gemini config exists but cannot be read")
		} else {
			var config map[string]interface{}
			if err := json.Unmarshal(configBytes, &config); err != nil {
				issues = append(issues, "Gemini config contains invalid JSON")
			} else {
				// Check for problematic "disabled": true
				if disabled, exists := config["disabled"]; exists && disabled == true {
					issues = append(issues, "Gemini config contains 'disabled: true' which causes 'NO_AGENTS' error")
				}
			}
		}
	}

	return issues
}
