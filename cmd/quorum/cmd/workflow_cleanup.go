package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

var workflowCleanupCmd = &cobra.Command{
	Use:   "cleanup <workflow-id>",
	Short: "Clean up workflow Git resources",
	Long: `Remove Git resources (branches, worktrees) for a workflow.

This removes:
- Task worktrees
- Task branches
- Workflow worktree directory
- Optionally the workflow branch (--remove-branch)`, 
	Args: cobra.ExactArgs(1),
	RunE: cleanupWorkflow,
}

func init() {
	workflowCmd.AddCommand(workflowCleanupCmd)

	workflowCleanupCmd.Flags().Bool("remove-branch", false,
		"Also remove the workflow branch")
	workflowCleanupCmd.Flags().Bool("force", false,
		"Force cleanup even if workflow is running")
}

func cleanupWorkflow(cmd *cobra.Command, args []string) error {
	workflowID := args[0]
	removeBranch, _ := cmd.Flags().GetBool("remove-branch")
	force, _ := cmd.Flags().GetBool("force")

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

	// Check if running
	if state.Status == core.WorkflowStatusRunning && !force {
		return fmt.Errorf("workflow %s is still running. Use --force to cleanup anyway", workflowID)
	}

	// Check for Git isolation
	if !state.HasGitIsolation() {
		fmt.Println("Workflow does not have Git isolation. Nothing to clean up.")
		return nil
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

	// Perform cleanup
	fmt.Printf("Cleaning up workflow %s...\n", workflowID)

	// Cast to TaskWorktreeManager to access CleanupWorkflow
	// In a real implementation, this would be part of the interface.
	if err := wtManager.CleanupWorkflow(ctx, workflowID, removeBranch); err != nil {
		return fmt.Errorf("cleaning up workflow: %w", err)
	}

	if removeBranch {
		fmt.Printf("Removed workflow branch and all resources for %s\n", workflowID)
	} else {
		fmt.Printf("Removed worktrees and task branches for %s\n", workflowID)
		fmt.Printf("Workflow branch %s preserved. Use --remove-branch to delete it.\n",
			state.WorkflowBranch)
	}

	return nil
}
