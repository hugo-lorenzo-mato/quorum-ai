package cmd

import (
	"bufio"
	"context"
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

var (
	workflowsOutput string
)

var workflowsCmd = &cobra.Command{
	Use:    "workflows",
	Short:  "List available workflows (alias for 'workflow list')",
	Hidden: true,
	RunE:   runWorkflows,
}

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
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), cfg.State.Path)
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
			return OutputJSON([]interface{}{})
		}
		fmt.Println("No workflows found.")
		fmt.Println("Run 'quorum analyze <prompt>' to start a new workflow.")
		return nil
	}

	// JSON output
	if outputMode == tui.ModeJSON {
		return OutputJSON(workflows)
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
		prompt := TruncateString(wf.Prompt, 50)

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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available workflows",
	Long: `List all available workflows with their status and details.

Displays workflow ID, status, current phase, creation time, and prompt summary.
The active workflow is marked with an asterisk (*).`,
	RunE: runWorkflows,
}

var switchCmd = &cobra.Command{
	Use:   "switch <workflow-id>",
	Short: "Switch the active workflow",
	Long: `Set a specific workflow as the active one.

Future 'plan' or 'execute' commands will use this workflow by default if no
ID is specified via flags.`,
	Args: cobra.ExactArgs(1),
	RunE: runSwitch,
}

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed and failed workflows",
	Long: `Archive all workflows with 'completed' or 'failed' status.

Archived workflows are moved to the .quorum/state/archive directory as JSON
files and removed from the active database. This helps keep the workflow list
clean and improves performance.`,
	RunE: runArchive,
}

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Permanently delete all workflows",
	Long: `Permanently remove ALL workflows and their associated data.

This action is destructive and cannot be undone. It clears the entire
workflow database.`,
	RunE: runPurge,
}

var purgeForce bool

func init() {
	rootCmd.AddCommand(workflowCmd)
	workflowCmd.AddCommand(listCmd)
	workflowCmd.AddCommand(switchCmd)
	workflowCmd.AddCommand(archiveCmd)
	workflowCmd.AddCommand(purgeCmd)

	listCmd.Flags().StringVarP(&workflowsOutput, "output", "o", "", "Output mode (plain, json)")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "Skip confirmation prompt")
}

func runSwitch(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	workflowID := core.WorkflowID(args[0])

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create state manager
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), cfg.State.Path)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		_ = state.CloseStateManager(stateManager)
	}()

	// Verify workflow exists
	wf, err := stateManager.LoadByID(ctx, workflowID)
	if err != nil || wf == nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Set as active
	if err := stateManager.SetActiveWorkflowID(ctx, workflowID); err != nil {
		return fmt.Errorf("switching workflow: %w", err)
	}

	fmt.Printf("Switched to workflow %s\n", workflowID)
	return nil
}

func runArchive(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create state manager
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), cfg.State.Path)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		_ = state.CloseStateManager(stateManager)
	}()

	// Archive workflows
	count, err := stateManager.ArchiveWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("archiving workflows: %w", err)
	}

	if count == 0 {
		fmt.Println("No workflows to archive.")
	} else {
		fmt.Printf("Archived %d workflow(s).\n", count)
	}

	return nil
}

func runPurge(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Confirmation prompt unless --force
	if !purgeForce {
		fmt.Println("WARNING: This will permanently delete ALL workflows and their associated data.")
		fmt.Print("Are you sure you want to continue? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create state manager
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), cfg.State.Path)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		_ = state.CloseStateManager(stateManager)
	}()

	// Purge all workflows
	count, err := stateManager.PurgeAllWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("purging workflows: %w", err)
	}

	fmt.Printf("Purged %d workflow(s). All data cleared.\n", count)
	return nil
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
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), cfg.State.Path)
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
		fmt.Printf("  Prompt: %s\n", TruncateString(wf.Prompt, 60))
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
