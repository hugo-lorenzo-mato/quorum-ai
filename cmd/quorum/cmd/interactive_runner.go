package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

type analyzerRunner interface {
	Run(ctx context.Context, wctx *workflow.Context) error
}

type resumableRunner interface {
	ResumeWithState(ctx context.Context, state *core.WorkflowState) error
}

var newAnalyzerFn = func(moderatorConfig workflow.ModeratorConfig) (analyzerRunner, error) {
	return workflow.NewAnalyzer(moderatorConfig)
}

var runPlanPhaseFn = runPlanPhase

var newInteractiveRunnerFn = func(deps *PhaseRunnerDeps, output tui.Output) (resumableRunner, error) {
	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
		DryRun:      deps.RunnerConfig.DryRun,
		DeniedTools: deps.RunnerConfig.DenyTools,
	})

	return workflow.NewRunner(workflow.RunnerDeps{
		Config:            deps.RunnerConfig,
		State:             deps.StateAdapter,
		Agents:            deps.Registry,
		DAG:               deps.DAGAdapter,
		Checkpoint:        deps.CheckpointAdapter,
		ResumeProvider:    deps.ResumeAdapter,
		Prompts:           deps.PromptAdapter,
		Retry:             deps.RetryAdapter,
		RateLimits:        deps.RateLimiterAdapt,
		Worktrees:         deps.WorktreeManager,
		WorkflowWorktrees: deps.WorkflowWorktrees,
		GitIsolation:      deps.GitIsolation,
		GitClientFactory:  deps.GitClientFactory,
		Git:               deps.GitClient,
		GitHub:            deps.GitHubClient,
		Logger:            deps.Logger,
		Output:            tui.NewOutputNotifierAdapter(output),
		ModeEnforcer:      workflow.NewModeEnforcerAdapter(modeEnforcer),
	})
}

// runInteractiveWorkflow executes a workflow interactively, pausing between phases
// for user review and feedback. The CLI controls the loop directly using
// AnalyzeWithState/PlanWithState and the executor, without using ControlPlane.
func runInteractiveWorkflow(ctx context.Context, args []string) error {
	// Get prompt
	prompt, err := getPrompt(args, runFile)
	if err != nil {
		return err
	}

	// Initialize phase runner dependencies
	deps, err := InitPhaseRunner(ctx, core.PhaseAnalyze, runMaxRetries, runDryRun)
	if err != nil {
		return err
	}

	// Create output handler
	detector := tui.NewDetector()
	if runOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(runOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()
	output := tui.NewOutput(outputMode, useColor, false)
	defer func() { _ = output.Close() }()

	deps.Logger.Info("starting interactive workflow", "prompt_length", len(prompt))

	// Acquire lock
	if err := deps.StateAdapter.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = deps.StateAdapter.ReleaseLock(ctx) }()

	// Build blueprint with interactive mode
	bp := buildBlueprint(deps.RunnerConfig)
	bp.ExecutionMode = core.ExecutionModeInteractive

	// Initialize workflow state
	state := InitializeWorkflowState(prompt, bp)

	// Initialize workflow-level git isolation
	if changed, isoErr := EnsureWorkflowGitIsolation(ctx, deps, state); isoErr != nil {
		deps.Logger.Warn("failed to initialize workflow git isolation", "error", isoErr)
	} else if changed {
		deps.Logger.Info("workflow git isolation initialized", "workflow_branch", state.WorkflowBranch)
	}

	// Save initial state
	if err := deps.StateAdapter.Save(ctx, state); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)

	abort, err := runInteractiveAnalysisPhase(ctx, deps, output, scanner, state)
	if err != nil {
		return err
	}
	if abort {
		fmt.Println("Workflow aborted.")
		return nil
	}

	abort, err = runInteractivePlanningPhase(ctx, deps, output, scanner, state)
	if err != nil {
		return err
	}
	if abort {
		fmt.Println("Workflow aborted.")
		return nil
	}

	return runInteractiveExecutionPhase(ctx, deps, output, state)
}

func runInteractiveAnalysisPhase(ctx context.Context, deps *PhaseRunnerDeps, output tui.Output, scanner *bufio.Scanner, state *core.WorkflowState) (bool, error) {
	fmt.Println("\n[1/3] Running analysis...")
	output.PhaseStarted(core.PhaseAnalyze)

	analyzer, err := newAnalyzerFn(deps.ModeratorConfig)
	if err != nil {
		return false, fmt.Errorf("creating analyzer: %w", err)
	}

	if err := analyzer.Run(ctx, CreateWorkflowContext(deps, state)); err != nil {
		state.Status = core.WorkflowStatusFailed
		state.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, state)
		return false, fmt.Errorf("analysis failed: %w", err)
	}

	state.CurrentPhase = core.PhasePlan
	state.UpdatedAt = time.Now()
	_ = deps.StateAdapter.Save(ctx, state)

	analysis := workflow.GetConsolidatedAnalysis(state)
	fmt.Println("\n  Analysis complete.")
	fmt.Println("\n  === Analysis Summary ===")
	displayTruncated(analysis, 40)

	action, feedback := promptPhaseReview(scanner, "analysis")
	switch action {
	case "abort":
		return true, nil
	case "rerun":
		fmt.Println("\n  Re-running analysis...")
		state.CurrentPhase = core.PhaseAnalyze
		state.Checkpoints = nil
		state.UpdatedAt = time.Now()

		if err := analyzer.Run(ctx, CreateWorkflowContext(deps, state)); err != nil {
			state.Status = core.WorkflowStatusFailed
			state.UpdatedAt = time.Now()
			_ = deps.StateAdapter.Save(ctx, state)
			return false, fmt.Errorf("analysis re-run failed: %w", err)
		}

		state.CurrentPhase = core.PhasePlan
		state.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, state)

		analysis = workflow.GetConsolidatedAnalysis(state)
		fmt.Println("\n  === Updated Analysis Summary ===")
		displayTruncated(analysis, 40)
	case "continue":
		if feedback != "" {
			if err := workflow.PrependToConsolidatedAnalysis(state, feedback); err != nil {
				deps.Logger.Warn("failed to apply analysis feedback", "error", err)
			} else {
				fmt.Println("  Feedback applied to analysis.")
			}
			_ = deps.StateAdapter.Save(ctx, state)
		}
	}

	return false, nil
}

func runInteractivePlanningPhase(ctx context.Context, deps *PhaseRunnerDeps, output tui.Output, scanner *bufio.Scanner, state *core.WorkflowState) (bool, error) {
	fmt.Println("\n[2/3] Generating plan...")
	output.PhaseStarted(core.PhasePlan)

	if err := runPlanPhaseFn(ctx, deps, CreateWorkflowContext(deps, state), state); err != nil {
		return false, fmt.Errorf("planning failed: %w", err)
	}

	displayTaskPlan(state)

	for {
		action, feedback := promptPlanReview(scanner)
		switch action {
		case "abort":
			return true, nil
		case "replan":
			if err := replanInteractive(ctx, deps, state, feedback); err != nil {
				return false, err
			}
			displayTaskPlan(state)
			continue
		case "edit":
			editTasksInteractive(scanner, state)
			_ = deps.StateAdapter.Save(ctx, state)
			displayTaskPlan(state)
			continue
		case "continue":
			return false, nil
		}
	}
}

func replanInteractive(ctx context.Context, deps *PhaseRunnerDeps, state *core.WorkflowState, feedback string) error {
	fmt.Println("\n  Regenerating plan...")
	if feedback != "" {
		if err := workflow.PrependToConsolidatedAnalysis(state, "User feedback on plan: "+feedback); err != nil {
			deps.Logger.Warn("failed to apply plan feedback", "error", err)
		}
	}

	state.CurrentPhase = core.PhasePlan
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	state.TaskOrder = nil
	state.UpdatedAt = time.Now()

	if err := runPlanPhaseFn(ctx, deps, CreateWorkflowContext(deps, state), state); err != nil {
		return fmt.Errorf("replanning failed: %w", err)
	}
	return nil
}

func runInteractiveExecutionPhase(ctx context.Context, deps *PhaseRunnerDeps, output tui.Output, state *core.WorkflowState) error {
	fmt.Printf("\n[3/3] Executing %d tasks...\n", len(state.TaskOrder))
	output.PhaseStarted(core.PhaseExecute)

	runner, err := newInteractiveRunnerFn(deps, output)
	if err != nil {
		return fmt.Errorf("creating runner: %w", err)
	}

	state.CurrentPhase = core.PhaseExecute
	state.Status = core.WorkflowStatusRunning
	state.UpdatedAt = time.Now()
	if err := deps.StateAdapter.Save(ctx, state); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	startTime := time.Now()
	if err := runner.ResumeWithState(ctx, state); err != nil {
		state.Status = core.WorkflowStatusFailed
		state.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, state)
		return fmt.Errorf("execution failed: %w", err)
	}
	duration := time.Since(startTime)

	state.Status = core.WorkflowStatusCompleted
	state.CurrentPhase = core.PhaseDone
	state.UpdatedAt = time.Now()
	_ = deps.StateAdapter.Save(ctx, state)

	fmt.Printf("\nWorkflow completed: %d tasks, %s\n", len(state.TaskOrder), duration.Round(time.Second))
	return nil
}

// runPlanPhase runs the planning phase using the Planner.
func runPlanPhase(ctx context.Context, deps *PhaseRunnerDeps, wctx *workflow.Context, _ *core.WorkflowState) error {
	planner := workflow.NewPlanner(deps.DAGAdapter, deps.StateAdapter)
	return planner.Run(ctx, wctx)
}
