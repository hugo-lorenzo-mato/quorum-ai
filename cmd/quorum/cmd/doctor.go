package cmd

import (
	"fmt"
	"os/exec"

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
		{"aider", "aider", []string{"--version"}, false},
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

	if !requiredOk {
		fmt.Println("Some required dependencies are missing")
		return fmt.Errorf("dependency check failed")
	}

	if allOk {
		fmt.Println("All dependencies available")
	} else {
		fmt.Println("Required dependencies available, some optional tools missing")
	}

	return nil
}

func checkCommand(name string, args []string) bool {
	cmd := exec.Command(name, args...)
	return cmd.Run() == nil
}
