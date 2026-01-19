package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Run the execution phase only",
	Long: `Execute the task execution phase of the workflow.

The execution phase runs all planned tasks:
- Executes tasks respecting dependency order
- Runs independent tasks in parallel
- Creates isolated git worktrees for task execution
- Supports resuming from failed tasks

Requires a completed plan phase. Use 'quorum plan' first.`,
	RunE: runExecute,
}

var (
	executeDryRun     bool
	executeMaxRetries int
	executeOutput     string
	executeSandbox    bool
)

func init() {
	rootCmd.AddCommand(executeCmd)

	executeCmd.Flags().BoolVar(&executeDryRun, "dry-run", false, "Simulate without executing")
	executeCmd.Flags().IntVar(&executeMaxRetries, "max-retries", 3, "Maximum retry attempts")
	executeCmd.Flags().StringVarP(&executeOutput, "output", "o", "", "Output mode (tui, plain, json, quiet)")
	executeCmd.Flags().BoolVar(&executeSandbox, "sandbox", false, "Run in sandboxed mode")
}

func runExecute(_ *cobra.Command, _ []string) error {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt, stopping...")
		cancel()
	}()

	// Detect output mode
	detector := tui.NewDetector()
	if executeOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(executeOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()

	// Create output handler
	output := tui.NewOutput(outputMode, useColor, false)
	defer func() { _ = output.Close() }()

	// Initialize phase runner dependencies
	deps, err := InitPhaseRunner(ctx, core.PhaseExecute, executeMaxRetries, executeDryRun, executeSandbox)
	if err != nil {
		return err
	}

	phaseCtx, phaseCancel := context.WithTimeout(ctx, deps.PhaseTimeout)
	defer phaseCancel()
	ctx = phaseCtx

	// Acquire lock
	if err := deps.StateAdapter.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = deps.StateAdapter.ReleaseLock(ctx) }()

	// Load existing state
	workflowState, err := deps.StateAdapter.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found; run 'quorum analyze' and 'quorum plan' first")
	}

	// Verify plan phase completed
	if len(workflowState.Tasks) == 0 {
		return core.ErrState("NO_TASKS", "no tasks found; run 'quorum plan' first")
	}

	// Verify we're at the right phase
	if workflowState.CurrentPhase != core.PhaseExecute {
		return core.ErrState("WRONG_PHASE", fmt.Sprintf("workflow is at %s phase; expected execute phase", workflowState.CurrentPhase))
	}

	deps.Logger.Info("starting execute phase",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
	)

	// Rebuild DAG from existing tasks
	for _, id := range workflowState.TaskOrder {
		taskState := workflowState.Tasks[id]
		if taskState != nil {
			task := &core.Task{
				ID:           taskState.ID,
				Name:         taskState.Name,
				Phase:        taskState.Phase,
				Status:       taskState.Status,
				CLI:          taskState.CLI,
				Model:        taskState.Model,
				Dependencies: taskState.Dependencies,
			}
			_ = deps.DAGAdapter.AddTask(task)
		}
	}

	// Build dependencies
	for _, id := range workflowState.TaskOrder {
		taskState := workflowState.Tasks[id]
		if taskState != nil {
			for _, dep := range taskState.Dependencies {
				_ = deps.DAGAdapter.AddDependency(id, dep)
			}
		}
	}

	// Validate DAG
	if _, err := deps.DAGAdapter.Build(); err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	// Create workflow context
	wctx := CreateWorkflowContext(deps, workflowState)

	// Create executor
	executor := workflow.NewExecutor(deps.DAGAdapter, deps.StateAdapter, deps.RunnerConfig.DenyTools)

	output.PhaseStarted(core.PhaseExecute)

	// Run execution phase
	if err := executor.Run(ctx, wctx); err != nil {
		workflowState.Status = core.WorkflowStatusFailed
		workflowState.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, workflowState)
		output.Log("error", fmt.Sprintf("execute phase failed: %v", err))
		return err
	}

	// Mark workflow completed
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.UpdatedAt = time.Now()

	// Save final state
	if err := deps.StateAdapter.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	output.Log("info", "execute phase completed")
	output.WorkflowCompleted(workflowState)

	deps.Logger.Info("execute phase completed",
		"workflow_id", workflowState.WorkflowID,
		"completed_tasks", countCompletedTasks(workflowState),
	)

	// Output summary in JSON mode
	if outputMode == tui.ModeJSON {
		tasks := make([]map[string]interface{}, 0, len(workflowState.TaskOrder))
		for _, id := range workflowState.TaskOrder {
			task := workflowState.Tasks[id]
			if task != nil {
				tasks = append(tasks, map[string]interface{}{
					"id":        task.ID,
					"name":      task.Name,
					"status":    task.Status,
					"tokens_in": task.TokensIn,
					"cost_usd":  task.CostUSD,
				})
			}
		}
		result := map[string]interface{}{
			"workflow_id": workflowState.WorkflowID,
			"phase":       "execute",
			"status":      "completed",
			"tasks":       tasks,
			"metrics":     workflowState.Metrics,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Execute phase completed. Workflow ID: %s\n", workflowState.WorkflowID)
	fmt.Printf("Completed %d/%d tasks.\n", countCompletedTasks(workflowState), len(workflowState.Tasks))
	if workflowState.Metrics != nil && workflowState.Metrics.TotalCostUSD > 0 {
		fmt.Printf("Total cost: $%.4f\n", workflowState.Metrics.TotalCostUSD)
	}

	return nil
}

// countCompletedTasks returns the number of completed tasks in the workflow state.
func countCompletedTasks(state *core.WorkflowState) int {
	count := 0
	for _, task := range state.Tasks {
		if task.Status == core.TaskStatusCompleted {
			count++
		}
	}
	return count
}
