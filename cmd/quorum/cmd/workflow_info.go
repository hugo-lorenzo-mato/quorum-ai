package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

var workflowInfoCmd = &cobra.Command{
	Use:   "info <workflow-id>",
	Short: "Show detailed workflow information",
	Long:  "Display comprehensive details about a specific workflow, including tasks and Git isolation status.",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkflowInfo,
}

func init() {
	workflowCmd.AddCommand(workflowInfoCmd)
}

func runWorkflowInfo(cmd *cobra.Command, args []string) error {
	workflowID := args[0]

	// Initialize state manager
	stateManager, err := initStateManager()
	if err != nil {
		return fmt.Errorf("initializing state manager: %w", err)
	}
	defer func() {
		_ = stateManager.DeactivateWorkflow(cmd.Context())
	}()

	ctx := cmd.Context()

	// Load workflow
	state, err := stateManager.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		return fmt.Errorf("loading workflow: %w", err)
	}
	if state == nil {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	printWorkflowInfo(state)
	return nil
}

func printWorkflowInfo(state *core.WorkflowState) {
	fmt.Printf("Workflow ID: %s\n", state.WorkflowID)
	if state.Title != "" {
		fmt.Printf("Title:       %s\n", state.Title)
	}
	fmt.Printf("Status:      %s\n", state.Status)
	fmt.Printf("Phase:       %s\n", state.CurrentPhase)
	fmt.Printf("Created:     %s\n", state.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", state.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Prompt:      %s\n", TruncateString(state.Prompt, 100))

	// Git isolation info
	if state.HasGitIsolation() {
		fmt.Println("\nGit Isolation:")
		fmt.Printf("  Workflow Branch: %s\n", state.WorkflowBranch)
		fmt.Printf("  Base Branch:     %s\n", state.BaseBranch)
		fmt.Printf("  Merge Strategy:  %s\n", state.MergeStrategy)
		fmt.Printf("  Worktree Root:   %s\n", state.WorktreeRoot)

		// Show pending merges
	
pendingMerges := 0
		for _, task := range state.Tasks {
			if task.MergePending {
			
pendingMerges++
			}
		}
		if pendingMerges > 0 {
			fmt.Printf("  Pending Merges:  %d\n", pendingMerges)
		}
	}

	// Task table
	fmt.Println("\nTasks:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tPHASE\tMERGE")
	fmt.Fprintln(w, "--\t----\t------\t-----\t-----")

	for _, id := range state.TaskOrder {
		task := state.Tasks[id]
		mergeStatus := "-"
		if task.MergePending {
			mergeStatus = "PENDING"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			task.ID, task.Name, task.Status, task.Phase, mergeStatus)
	}
	w.Flush()
}
