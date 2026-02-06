package cmd

import (
	"bufio"
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

	// Create state manager using factory
	stateManager, err := state.NewStateManager(cfg.State.Path)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing state manager: %v\n", closeErr)
		}
	}()

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
	case core.PhaseRefine:
		return "refine"
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

// Delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <workflow-id>",
	Short: "Delete a workflow",
	Long: `Delete a specific workflow by its ID.

This permanently removes the workflow and all its associated data including
tasks, checkpoints, and analysis results.

Running workflows cannot be deleted. Cancel them first if needed.

Examples:
  quorum workflow delete abc123
  quorum workflow delete abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

var deleteForce bool

func init() {
	workflowCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
}

// workflowCmd is a parent command for workflow subcommands
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Manage workflows",
	Long:  `Commands for managing quorum workflows.`,
}

func init() {
	rootCmd.AddCommand(workflowCmd)
}

func runDelete(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	workflowID := core.WorkflowID(args[0])

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create state manager
	stateManager, err := state.NewStateManager(cfg.State.Path)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing state manager: %v\n", closeErr)
		}
	}()

	// Load workflow to verify it exists and check status
	wf, err := stateManager.LoadByID(ctx, workflowID)
	if err != nil || wf == nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Prevent deletion of running workflows
	if wf.Status == core.WorkflowStatusRunning {
		return fmt.Errorf("cannot delete running workflow (use 'quorum cancel' first)")
	}

	// Confirmation prompt unless --force
	if !deleteForce {
		fmt.Printf("Delete workflow %s?\n", workflowID)
		fmt.Printf("  Status: %s\n", formatStatus(wf.Status))
		fmt.Printf("  Prompt: %s\n", truncateString(wf.Prompt, 60))
		fmt.Print("\nThis action cannot be undone. Continue? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Delete the workflow
	if err := stateManager.DeleteWorkflow(ctx, workflowID); err != nil {
		return fmt.Errorf("deleting workflow: %w", err)
	}

	fmt.Printf("Workflow %s deleted.\n", workflowID)
	return nil
}
