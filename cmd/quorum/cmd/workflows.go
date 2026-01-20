package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

var workflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "List available workflows",
	Long: `List all available workflows with their status and details.

Displays workflow ID, status, current phase, creation time, and prompt summary.
The active workflow is marked with an asterisk (*).

Use 'quorum plan --workflow <id>' or 'quorum execute --workflow <id>' to resume
a specific workflow.`,
	RunE: runWorkflows,
}

var (
	workflowsOutput string
)

func init() {
	rootCmd.AddCommand(workflowsCmd)
	workflowsCmd.Flags().StringVarP(&workflowsOutput, "output", "o", "", "Output mode (plain, json)")
}

func runWorkflows(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Detect output mode
	detector := tui.NewDetector()
	if workflowsOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(workflowsOutput))
	}
	outputMode := detector.Detect()

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create state manager
	stateManager := state.NewJSONStateManager(
		cfg.State.Path,
		state.WithBackupPath(cfg.State.BackupPath),
	)

	// List workflows
	workflows, err := stateManager.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("listing workflows: %w", err)
	}

	if len(workflows) == 0 {
		if outputMode == tui.ModeJSON {
			return json.NewEncoder(os.Stdout).Encode([]interface{}{})
		}
		fmt.Println("No workflows found.")
		fmt.Println("Run 'quorum analyze <prompt>' to start a new workflow.")
		return nil
	}

	// JSON output
	if outputMode == tui.ModeJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(workflows)
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPHASE\tCREATED\tPROMPT")
	fmt.Fprintln(w, "--\t------\t-----\t-------\t------")

	for _, wf := range workflows {
		id := string(wf.WorkflowID)
		if wf.IsActive {
			id = "* " + id
		} else {
			id = "  " + id
		}

		status := formatStatus(wf.Status)
		phase := formatPhase(wf.CurrentPhase)
		created := formatWorkflowTime(wf.CreatedAt)
		prompt := truncateString(wf.Prompt, 50)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", id, status, phase, created, prompt)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("* = active workflow")
	fmt.Println("Use 'quorum plan --workflow <id>' to continue a specific workflow")

	return nil
}

func formatStatus(s core.WorkflowStatus) string {
	switch s {
	case core.WorkflowStatusPending:
		return "pending"
	case core.WorkflowStatusRunning:
		return "running"
	case core.WorkflowStatusCompleted:
		return "completed"
	case core.WorkflowStatusFailed:
		return "failed"
	default:
		return string(s)
	}
}

func formatPhase(p core.Phase) string {
	switch p {
	case core.PhaseOptimize:
		return "optimize"
	case core.PhaseAnalyze:
		return "analyze"
	case core.PhasePlan:
		return "plan"
	case core.PhaseExecute:
		return "execute"
	default:
		return string(p)
	}
}

func formatWorkflowTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func truncateString(s string, maxLen int) string {
	// Remove newlines
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
