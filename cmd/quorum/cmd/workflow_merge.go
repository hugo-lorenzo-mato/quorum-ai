package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
)

var workflowMergeCmd = &cobra.Command{
	Use:   "merge <workflow-id>",
	Short: "Merge workflow branch to base branch",
	Long: `Merge a completed workflow's Git branch into the base branch.

The workflow must be completed and have Git isolation enabled.
By default, this creates a merge commit. Use --squash for a squash merge.`, 
	Args: cobra.ExactArgs(1),
	RunE: mergeWorkflow,
}

func init() {
	workflowCmd.AddCommand(workflowMergeCmd)

	workflowMergeCmd.Flags().Bool("squash", false, "Squash all commits into one")
	workflowMergeCmd.Flags().Bool("dry-run", false, "Show what would be merged without doing it")
	workflowMergeCmd.Flags().String("message", "", "Custom merge commit message")
}

func mergeWorkflow(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	squash, _ := cmd.Flags().GetBool("squash")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	message, _ := cmd.Flags().GetString("message")

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

	// Check workflow has Git isolation
	if !state.HasGitIsolation() {
		return fmt.Errorf("workflow %s does not have Git isolation enabled", workflowID)
	}

	// Check workflow is completed
	if state.Status != core.WorkflowStatusCompleted {
		return fmt.Errorf("workflow %s is not completed (status: %s)", workflowID, state.Status)
	}

	// Initialize Git client
	gitClient, err := git.NewClient(".")
	if err != nil {
		return fmt.Errorf("initializing git client: %w", err)
	}

	if dryRun {
		fmt.Printf("Would merge branch %s into %s\n", state.WorkflowBranch, state.BaseBranch)

		// Show diff stats
		files, err := gitClient.DiffFiles(ctx, state.BaseBranch, state.WorkflowBranch)
		if err == nil {
			fmt.Printf("Files changed: %d\n", len(files))
			for _, f := range files {
				fmt.Printf("  %s\n", f)
			}
		}
		return nil
	}

	// Perform merge
	fmt.Printf("Merging %s into %s...\n", state.WorkflowBranch, state.BaseBranch)

	// Checkout base branch
	if err := gitClient.CheckoutBranch(ctx, state.BaseBranch); err != nil {
		return fmt.Errorf("checking out base branch: %w", err)
	}

	// Merge workflow branch
	mergeOpts := core.MergeOptions{
		Squash:        squash,
		NoFastForward: true,
	}
	if message != "" {
		mergeOpts.Message = message
	} else {
		mergeOpts.Message = fmt.Sprintf("Merge workflow %s", workflowID)
	}

	if err := gitClient.Merge(ctx, state.WorkflowBranch, mergeOpts); err != nil {
		return fmt.Errorf("merging workflow branch: %w", err)
	}

	fmt.Printf("Successfully merged workflow %s\n", workflowID)

	// Optionally delete the workflow branch
	fmt.Printf("Workflow branch %s preserved. Use 'quorum workflow cleanup %s' to remove it.\n",
		state.WorkflowBranch, workflowID)

	return nil
}
