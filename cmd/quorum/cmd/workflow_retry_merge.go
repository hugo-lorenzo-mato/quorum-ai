package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

var workflowRetryMergeCmd = &cobra.Command{
	Use:   "retry-merge <workflow-id> <task-id>",
	Short: "Retry a failed task merge",
	Long: `Retry merging a task branch that previously failed to merge.

Use this when a task completed successfully but the merge to the
workflow branch failed due to conflicts or other issues.`, 
	Args: cobra.ExactArgs(2),
	RunE: retryTaskMerge,
}

func init() {
	workflowCmd.AddCommand(workflowRetryMergeCmd)

	workflowRetryMergeCmd.Flags().String("strategy", "",
		"Override merge strategy: sequential, parallel, rebase")
	workflowRetryMergeCmd.Flags().Bool("theirs", false,
		"Prefer task changes in case of conflict")
	workflowRetryMergeCmd.Flags().Bool("ours", false,
		"Prefer workflow branch changes in case of conflict")
}

func retryTaskMerge(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	taskID := args[1]
	strategy, _ := cmd.Flags().GetString("strategy")
	theirs, _ := cmd.Flags().GetBool("theirs")
	ours, _ := cmd.Flags().GetBool("ours")

	if theirs && ours {
		return fmt.Errorf("cannot use both --theirs and --ours")
	}

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

	// Check task exists
	taskState, ok := state.Tasks[core.TaskID(taskID)]
	if !ok {
		return fmt.Errorf("task %s not found in workflow %s", taskID, workflowID)
	}

	// Check task has pending merge
	if !taskState.MergePending {
		return fmt.Errorf("task %s does not have a pending merge", taskID)
	}

	// Initialize Git client and worktree manager
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	gitClient, err := git.NewClient(cwd)
	if err != nil {
		return fmt.Errorf("initializing git client: %w", err)
	}

	// Load configuration to get worktree directory
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	wtManager := git.NewTaskWorktreeManager(gitClient, cfg.Git.WorktreeDir)

	// Determine strategy
	if strategy == "" {
		strategy = state.MergeStrategy
	}
	if strategy == "" {
		strategy = "sequential"
	}

	// Set strategy option for conflict resolution
	var strategyOption string
	if theirs {
		strategyOption = "theirs"
	} else if ours {
		strategyOption = "ours"
	}

	fmt.Printf("Retrying merge of task %s to workflow %s...\n", taskID, workflowID)

	// Attempt merge
	if err := wtManager.MergeTaskToWorkflow(ctx, workflowID, core.TaskID(taskID), strategy, strategyOption); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	// Update task state
	taskState.MergePending = false
	if err := stateManager.Save(ctx, state); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Printf("Successfully merged task %s\n", taskID)

	return nil
}
