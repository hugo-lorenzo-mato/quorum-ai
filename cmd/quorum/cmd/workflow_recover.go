package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

var workflowRecoverCmd = &cobra.Command{
	Use:   "recover [workflow-id]",
	Short: "Recover crashed workflows",
	Long: `Detect and recover workflows that crashed during execution.

If no workflow-id is provided, all zombie workflows will be recovered.

A workflow is considered a zombie if:
- It has status "running" but heartbeat is stale (>5 minutes)
- The process that started it is no longer running

Recovery includes:
- Saving any uncommitted changes to a recovery branch
- Aborting incomplete merge/rebase operations
- Resetting running tasks to pending
- Marking workflow as paused for resume`,
	Args: cobra.MaximumNArgs(1),
	RunE: recoverWorkflows,
}

var (
	recoverStaleThreshold time.Duration
	recoverListOnly       bool
	recoverForce          bool
)

func init() {
	workflowCmd.AddCommand(workflowRecoverCmd)

	workflowRecoverCmd.Flags().DurationVar(&recoverStaleThreshold, "stale-threshold", 5*time.Minute,
		"Duration after which a workflow is considered stale")
	workflowRecoverCmd.Flags().BoolVar(&recoverListOnly, "list-only", false,
		"Only list zombie workflows without recovering")
	workflowRecoverCmd.Flags().BoolVar(&recoverForce, "force", false,
		"Force recovery even if workflow seems active")
}

func recoverWorkflows(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
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
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: closing state manager: %v\n", closeErr)
		}
	}()

	// Create logger
	logger := logging.New(logging.Config{
		Level:  "info",
		Format: "auto",
		Output: os.Stderr,
	})

	// Get repo path
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Create recovery manager with a wrapper that implements RecoveryStateManager
	rm := workflow.NewRecoveryManager(
		&recoveryStateManagerAdapter{stateManager},
		repoPath,
		logger,
	)
	rm.SetStaleThreshold(recoverStaleThreshold)

	// If specific workflow ID provided
	if len(args) == 1 {
		workflowID := core.WorkflowID(args[0])

		if recoverListOnly {
			wfState, err := stateManager.LoadByID(ctx, workflowID)
			if err != nil {
				return fmt.Errorf("loading workflow: %w", err)
			}
			if wfState == nil {
				return fmt.Errorf("workflow not found: %s", workflowID)
			}
			printWorkflowRecoveryInfo(wfState)
			return nil
		}

		result, err := rm.RecoverWorkflow(ctx, workflowID)
		if err != nil {
			return fmt.Errorf("recovering workflow: %w", err)
		}

		printRecoveryResult(result)
		return nil
	}

	// Find all zombie workflows
	zombies, err := rm.FindZombieWorkflows(ctx)
	if err != nil {
		return fmt.Errorf("finding zombie workflows: %w", err)
	}

	if len(zombies) == 0 {
		fmt.Println("No zombie workflows found")
		return nil
	}

	fmt.Printf("Found %d zombie workflow(s):\n\n", len(zombies))

	for _, zombie := range zombies {
		printWorkflowRecoveryInfo(zombie)
		fmt.Println()
	}

	if recoverListOnly {
		return nil
	}

	// Confirm before proceeding
	if !recoverForce {
		fmt.Print("\nProceed with recovery? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Recovery cancelled")
			return nil
		}
	}

	// Recover all zombies
	fmt.Println("\nRecovering workflows...")
	for _, zombie := range zombies {
		result, err := rm.RecoverWorkflow(ctx, zombie.WorkflowID)
		if err != nil {
			fmt.Printf("  Failed to recover %s: %v\n", zombie.WorkflowID, err)
			continue
		}
		printRecoveryResultBrief(result)
	}

	fmt.Println("\nRecovery complete. Use 'quorum resume <id>' to continue workflows.")

	return nil
}

func printWorkflowRecoveryInfo(wfState *core.WorkflowState) {
	fmt.Printf("Workflow: %s\n", wfState.WorkflowID)
	fmt.Printf("  Status: %s\n", wfState.Status)
	fmt.Printf("  Phase: %s\n", wfState.CurrentPhase)
	if wfState.HeartbeatAt != nil {
		fmt.Printf("  Last Heartbeat: %s\n", wfState.HeartbeatAt.Format(time.RFC3339))
	} else {
		fmt.Printf("  Last Heartbeat: never\n")
	}

	runningTasks := 0
	for _, task := range wfState.Tasks {
		if task.Status == core.TaskStatusRunning {
			runningTasks++
		}
	}
	fmt.Printf("  Running Tasks: %d\n", runningTasks)
}

func printRecoveryResult(result *workflow.RecoveryResult) {
	fmt.Printf("\nRecovery Result for %s:\n", result.WorkflowID)

	if result.RecoveredChanges {
		fmt.Printf("  Recovered uncommitted changes to: %s\n", result.RecoveryBranch)
	}
	if result.AbortedMerge {
		fmt.Println("  Aborted incomplete merge operation")
	}
	if result.AbortedRebase {
		fmt.Println("  Aborted incomplete rebase operation")
	}
	if len(result.ResetTasks) > 0 {
		fmt.Printf("  Reset %d running task(s) to pending\n", len(result.ResetTasks))
	}

	if result.Error != nil {
		fmt.Printf("  Error: %v\n", result.Error)
	}
}

func printRecoveryResultBrief(result *workflow.RecoveryResult) {
	status := "OK"
	if result.Error != nil {
		status = "ERROR"
	}

	details := make([]string, 0)
	if result.RecoveredChanges {
		details = append(details, fmt.Sprintf("recovered:%s", result.RecoveryBranch))
	}
	if result.AbortedMerge || result.AbortedRebase {
		details = append(details, "git-cleanup")
	}
	if len(result.ResetTasks) > 0 {
		details = append(details, fmt.Sprintf("tasks-reset:%d", len(result.ResetTasks)))
	}

	if len(details) > 0 {
		fmt.Printf("  %s: %s (%s)\n", result.WorkflowID, status, strings.Join(details, ", "))
	} else {
		fmt.Printf("  %s: %s\n", result.WorkflowID, status)
	}
}

// recoveryStateManagerAdapter wraps core.StateManager to implement workflow.RecoveryStateManager.
type recoveryStateManagerAdapter struct {
	sm core.StateManager
}

func (a *recoveryStateManagerAdapter) LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	return a.sm.LoadByID(ctx, id)
}

func (a *recoveryStateManagerAdapter) Save(ctx context.Context, state *core.WorkflowState) error {
	return a.sm.Save(ctx, state)
}

func (a *recoveryStateManagerAdapter) FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	return a.sm.FindZombieWorkflows(ctx, staleThreshold)
}
