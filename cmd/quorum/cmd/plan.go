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

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Run the planning phase only",
	Long: `Execute the planning phase of the workflow.

The planning phase generates an execution plan based on the analysis:
- Uses the consolidated analysis from the previous analyze phase
- Generates a list of tasks with dependencies
- Builds a dependency graph (DAG) for task execution

Requires a completed analyze phase. Use 'quorum analyze' first.`,
	RunE: runPlan,
}

var (
	planDryRun     bool
	planMaxRetries int
	planOutput     string
	planWorkflowID string
)

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().BoolVar(&planDryRun, "dry-run", false, "Simulate without executing")
	planCmd.Flags().IntVar(&planMaxRetries, "max-retries", 3, "Maximum retry attempts")
	planCmd.Flags().StringVarP(&planOutput, "output", "o", "", "Output mode (tui, plain, json, quiet)")
	planCmd.Flags().StringVarP(&planWorkflowID, "workflow", "w", "", "Resume specific workflow by ID")

	// Single-agent mode flags
	planCmd.Flags().BoolVar(&singleAgent, "single-agent", false,
		"Run in single-agent mode (faster execution, no multi-agent consensus)")
	planCmd.Flags().StringVar(&agentName, "agent", "",
		"Agent to use for single-agent mode (e.g., 'claude', 'gemini', 'codex')")
	planCmd.Flags().StringVar(&agentModel, "model", "",
		"Override the agent's default model (optional, requires --single-agent)")
}

func runPlan(_ *cobra.Command, _ []string) error {
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

	// Validate single-agent flags
	if err := validateSingleAgentFlags(); err != nil {
		return err
	}

	// Detect output mode
	detector := tui.NewDetector()
	if planOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(planOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()

	// Create output handler
	output := tui.NewOutput(outputMode, useColor, false)
	defer func() { _ = output.Close() }()

	// Initialize phase runner dependencies
	deps, err := InitPhaseRunner(ctx, core.PhasePlan, planMaxRetries, planDryRun, false)
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
	var workflowState *core.WorkflowState

	if planWorkflowID != "" {
		// Load specific workflow by ID
		workflowState, err = deps.StateAdapter.LoadByID(ctx, core.WorkflowID(planWorkflowID))
		if err != nil {
			return fmt.Errorf("loading workflow %s: %w", planWorkflowID, err)
		}
		if workflowState == nil {
			return core.ErrState("NO_STATE", fmt.Sprintf("workflow %s not found", planWorkflowID))
		}
	} else {
		// Load active workflow
		workflowState, err = deps.StateAdapter.Load(ctx)
		if err != nil {
			return fmt.Errorf("loading state: %w", err)
		}
		if workflowState == nil {
			return core.ErrState("NO_STATE", "no workflow state found; run 'quorum analyze' first or use --workflow <id>")
		}
	}

	// Verify analyze phase completed
	analysis := workflow.GetConsolidatedAnalysis(workflowState)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found; run 'quorum analyze' first")
	}

	// Verify we're at the right phase
	if workflowState.CurrentPhase == core.PhaseExecute {
		return core.ErrState("PHASE_COMPLETED", "plan phase already completed; use 'quorum execute' to continue")
	}

	deps.Logger.Info("starting plan phase",
		"workflow_id", workflowState.WorkflowID,
	)

	// Create workflow context
	wctx := CreateWorkflowContext(deps, workflowState)

	// Create planner
	planner := workflow.NewPlanner(deps.DAGAdapter, deps.StateAdapter)

	output.PhaseStarted(core.PhasePlan)

	// Run planning phase
	if err := planner.Run(ctx, wctx); err != nil {
		workflowState.Status = core.WorkflowStatusFailed
		workflowState.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, workflowState)
		output.Log("error", fmt.Sprintf("plan phase failed: %v", err))
		return err
	}

	// Update state
	workflowState.CurrentPhase = core.PhaseExecute
	workflowState.UpdatedAt = time.Now()

	// Save final state
	if err := deps.StateAdapter.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	output.Log("info", "plan phase completed")

	deps.Logger.Info("plan phase completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
	)

	// Output summary in JSON mode
	if outputMode == tui.ModeJSON {
		tasks := make([]map[string]interface{}, 0, len(workflowState.TaskOrder))
		for _, id := range workflowState.TaskOrder {
			task := workflowState.Tasks[id]
			if task != nil {
				tasks = append(tasks, map[string]interface{}{
					"id":           task.ID,
					"name":         task.Name,
					"cli":          task.CLI,
					"dependencies": task.Dependencies,
				})
			}
		}
		result := map[string]interface{}{
			"workflow_id": workflowState.WorkflowID,
			"phase":       "plan",
			"status":      "completed",
			"task_count":  len(workflowState.Tasks),
			"tasks":       tasks,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Plan phase completed. Workflow ID: %s\n", workflowState.WorkflowID)
	fmt.Printf("Generated %d tasks.\n", len(workflowState.Tasks))
	fmt.Println("Run 'quorum execute' to continue with task execution.")

	return nil
}
