package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

var workflowListRunningCmd = &cobra.Command{
	Use:   "list-running",
	Short: "List currently running workflows",
	Long:  "Display all workflows that are currently being executed",
	RunE:  listRunningWorkflows,
}

func init() {
	workflowCmd.AddCommand(workflowListRunningCmd)
	workflowListRunningCmd.Flags().Bool("json", false, "Output in JSON format")
}

func listRunningWorkflows(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Initialize state manager
	stateManager, err := initStateManager()
	if err != nil {
		return fmt.Errorf("initializing state manager: %w", err)
	}
	defer func() {
		_ = stateManager.DeactivateWorkflow(cmd.Context()) // Placeholder for Close() if needed
	}()

	// Get all workflows and filter for running ones
	// Note: The spec mentioned stateManager.ListRunningWorkflows, but it's not in the interface.
	// I'll use ListWorkflows and filter.
	ctx := cmd.Context()
	allWorkflows, err := stateManager.ListWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("listing workflows: %w", err)
	}

	var running []core.WorkflowSummary
	for _, wf := range allWorkflows {
		if wf.Status == core.WorkflowStatusRunning {
			running = append(running, wf)
		}
	}

	if len(running) == 0 {
		fmt.Println("No workflows are currently running")
		return nil
	}

	if jsonOutput {
		return OutputJSON(running)
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WORKFLOW ID\tSTATUS\tPHASE\tBRANCH\tSTARTED")

	for _, summary := range running {
		state, err := stateManager.LoadByID(ctx, summary.WorkflowID)
		if err != nil {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				summary.WorkflowID, "unknown", "-", "-", "-")
			continue
		}

		branch := state.WorkflowBranch
		if branch == "" {
			branch = "(no isolation)"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			state.WorkflowID,
			state.Status,
			state.CurrentPhase,
			branch,
			state.CreatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	return w.Flush()
}
