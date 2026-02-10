package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

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

	// ── Phase 1: Analysis ──
	fmt.Println("\n[1/3] Running analysis...")
	output.PhaseStarted(core.PhaseAnalyze)

	wctx := CreateWorkflowContext(deps, state)
	analyzer, err := workflow.NewAnalyzer(deps.ModeratorConfig)
	if err != nil {
		return fmt.Errorf("creating analyzer: %w", err)
	}

	if err := analyzer.Run(ctx, wctx); err != nil {
		state.Status = core.WorkflowStatusFailed
		state.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, state)
		return fmt.Errorf("analysis failed: %w", err)
	}

	state.CurrentPhase = core.PhasePlan
	state.UpdatedAt = time.Now()
	_ = deps.StateAdapter.Save(ctx, state)

	// Show analysis summary
	analysis := workflow.GetConsolidatedAnalysis(state)
	fmt.Println("\n  Analysis complete.")
	fmt.Println("\n  === Analysis Summary ===")
	displayTruncated(analysis, 40)

	// Interactive review gate after analysis
	action, feedback := promptPhaseReview(scanner, "analysis")
	switch action {
	case "abort":
		fmt.Println("Workflow aborted.")
		return nil
	case "rerun":
		fmt.Println("\n  Re-running analysis...")
		// Reset analysis state
		state.CurrentPhase = core.PhaseAnalyze
		state.Checkpoints = nil
		state.UpdatedAt = time.Now()
		wctx = CreateWorkflowContext(deps, state)
		if err := analyzer.Run(ctx, wctx); err != nil {
			state.Status = core.WorkflowStatusFailed
			state.UpdatedAt = time.Now()
			_ = deps.StateAdapter.Save(ctx, state)
			return fmt.Errorf("analysis re-run failed: %w", err)
		}
		state.CurrentPhase = core.PhasePlan
		state.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, state)
		analysis = workflow.GetConsolidatedAnalysis(state)
		fmt.Println("\n  === Updated Analysis Summary ===")
		displayTruncated(analysis, 40)
	case "continue":
		// Apply feedback if provided
		if feedback != "" {
			if err := workflow.PrependToConsolidatedAnalysis(state, feedback); err != nil {
				deps.Logger.Warn("failed to apply analysis feedback", "error", err)
			} else {
				fmt.Println("  Feedback applied to analysis.")
			}
			_ = deps.StateAdapter.Save(ctx, state)
		}
	}

	// ── Phase 2: Planning ──
	fmt.Println("\n[2/3] Generating plan...")
	output.PhaseStarted(core.PhasePlan)

	wctx = CreateWorkflowContext(deps, state)
	if err := runPlanPhase(ctx, deps, wctx, state); err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	// Show task plan
	displayTaskPlan(state)

	// Interactive review gate after planning
	for {
		action, feedback = promptPlanReview(scanner)
		switch action {
		case "abort":
			fmt.Println("Workflow aborted.")
			return nil
		case "replan":
			fmt.Println("\n  Regenerating plan...")
			if feedback != "" {
				if err := workflow.PrependToConsolidatedAnalysis(state, "User feedback on plan: "+feedback); err != nil {
					deps.Logger.Warn("failed to apply plan feedback", "error", err)
				}
			}
			// Reset plan state
			state.CurrentPhase = core.PhasePlan
			state.Tasks = make(map[core.TaskID]*core.TaskState)
			state.TaskOrder = nil
			state.UpdatedAt = time.Now()
			wctx = CreateWorkflowContext(deps, state)
			if err := runPlanPhase(ctx, deps, wctx, state); err != nil {
				return fmt.Errorf("replanning failed: %w", err)
			}
			displayTaskPlan(state)
			continue // Loop back for review
		case "edit":
			editTasksInteractive(scanner, state)
			_ = deps.StateAdapter.Save(ctx, state)
			displayTaskPlan(state)
			continue // Loop back for review
		case "continue":
			// Continue to execution
		}
		break
	}

	// ── Phase 3: Execution ──
	fmt.Printf("\n[3/3] Executing %d tasks...\n", len(state.TaskOrder))
	output.PhaseStarted(core.PhaseExecute)

	// Create mode enforcer
	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
		DryRun:      deps.RunnerConfig.DryRun,
		DeniedTools: deps.RunnerConfig.DenyTools,
	})

	// Create a runner for execution
	runner, err := workflow.NewRunner(workflow.RunnerDeps{
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
	if err != nil {
		return fmt.Errorf("creating runner: %w", err)
	}

	// Save state before execution
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

// promptPhaseReview prompts the user to review a completed phase.
// Returns (action, feedback) where action is "continue", "rerun", or "abort".
func promptPhaseReview(scanner *bufio.Scanner, phaseName string) (string, string) {
	fmt.Printf("\n  [Enter] Continue to next phase\n")
	fmt.Printf("  [f]     Add feedback to %s\n", phaseName)
	fmt.Printf("  [r]     Re-run %s\n", phaseName)
	fmt.Printf("  [q]     Abort\n")
	fmt.Print("  > ")

	if !scanner.Scan() {
		return "continue", ""
	}

	input := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(input) {
	case "", "c":
		return "continue", ""
	case "f":
		fmt.Printf("  Enter feedback: ")
		if !scanner.Scan() {
			return "continue", ""
		}
		feedback := strings.TrimSpace(scanner.Text())
		if feedback != "" {
			fmt.Println("  Feedback saved. Continuing...")
		}
		return "continue", feedback
	case "r":
		return "rerun", ""
	case "q":
		return "abort", ""
	default:
		return "continue", ""
	}
}

// promptPlanReview prompts the user to review the task plan.
// Returns (action, feedback) where action is "continue", "edit", "replan", or "abort".
func promptPlanReview(scanner *bufio.Scanner) (string, string) {
	fmt.Println("\n  [Enter] Execute plan")
	fmt.Println("  [e]     Edit tasks")
	fmt.Println("  [r]     Regenerate plan")
	fmt.Println("  [q]     Abort")
	fmt.Print("  > ")

	if !scanner.Scan() {
		return "continue", ""
	}

	input := strings.TrimSpace(scanner.Text())
	switch strings.ToLower(input) {
	case "", "c":
		return "continue", ""
	case "e":
		return "edit", ""
	case "r":
		fmt.Print("  Feedback for replanning (optional, Enter to skip): ")
		if !scanner.Scan() {
			return "replan", ""
		}
		return "replan", strings.TrimSpace(scanner.Text())
	case "q":
		return "abort", ""
	default:
		return "continue", ""
	}
}

// editTasksInteractive allows the user to edit tasks interactively.
func editTasksInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	for {
		fmt.Printf("\n  Edit task number (1-%d), [a]dd, [d]elete, or [Enter] done: ", len(state.TaskOrder))
		if !scanner.Scan() {
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			return
		}

		switch strings.ToLower(input) {
		case "a":
			addTaskInteractive(scanner, state)
		case "d":
			deleteTaskInteractive(scanner, state)
		default:
			// Try to parse as task number
			num, err := strconv.Atoi(input)
			if err != nil || num < 1 || num > len(state.TaskOrder) {
				fmt.Println("  Invalid input.")
				continue
			}
			editSingleTask(scanner, state, num-1)
		}
	}
}

// editSingleTask edits a single task by index.
func editSingleTask(scanner *bufio.Scanner, state *core.WorkflowState, idx int) {
	taskID := state.TaskOrder[idx]
	task, ok := state.Tasks[taskID]
	if !ok {
		fmt.Println("  Task not found.")
		return
	}

	fmt.Printf("  Current: [%s] %s\n", task.CLI, task.Name)

	fmt.Print("  New name (Enter to keep): ")
	if scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name != "" {
			task.Name = name
		}
	}

	fmt.Print("  New description (Enter to keep): ")
	if scanner.Scan() {
		desc := strings.TrimSpace(scanner.Text())
		if desc != "" {
			task.Description = desc
		}
	}

	fmt.Print("  New agent (Enter to keep): ")
	if scanner.Scan() {
		agent := strings.TrimSpace(scanner.Text())
		if agent != "" {
			task.CLI = agent
		}
	}

	fmt.Println("  Task updated.")
}

// addTaskInteractive adds a new task interactively.
func addTaskInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	fmt.Print("  Task name: ")
	if !scanner.Scan() {
		return
	}
	name := strings.TrimSpace(scanner.Text())
	if name == "" {
		fmt.Println("  Name required. Cancelled.")
		return
	}

	fmt.Print("  Agent (e.g., claude, gemini, codex): ")
	if !scanner.Scan() {
		return
	}
	agent := strings.TrimSpace(scanner.Text())
	if agent == "" {
		fmt.Println("  Agent required. Cancelled.")
		return
	}

	fmt.Print("  Description (optional): ")
	var desc string
	if scanner.Scan() {
		desc = strings.TrimSpace(scanner.Text())
	}

	taskID := core.TaskID(fmt.Sprintf("task_interactive_%d", time.Now().UnixNano()))
	newTask := &core.TaskState{
		ID:          taskID,
		Phase:       core.PhaseExecute,
		Name:        name,
		Description: desc,
		Status:      core.TaskStatusPending,
		CLI:         agent,
	}

	if state.Tasks == nil {
		state.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	state.Tasks[taskID] = newTask
	state.TaskOrder = append(state.TaskOrder, taskID)
	fmt.Println("  Task added.")
}

// deleteTaskInteractive deletes a task interactively.
func deleteTaskInteractive(scanner *bufio.Scanner, state *core.WorkflowState) {
	fmt.Printf("  Delete task number (1-%d): ", len(state.TaskOrder))
	if !scanner.Scan() {
		return
	}

	num, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || num < 1 || num > len(state.TaskOrder) {
		fmt.Println("  Invalid task number.")
		return
	}

	idx := num - 1
	taskID := state.TaskOrder[idx]

	// Check dependencies
	for _, otherTask := range state.Tasks {
		for _, dep := range otherTask.Dependencies {
			if dep == taskID {
				fmt.Printf("  Cannot delete: task %s depends on it.\n", otherTask.Name)
				return
			}
		}
	}

	delete(state.Tasks, taskID)
	state.TaskOrder = append(state.TaskOrder[:idx], state.TaskOrder[idx+1:]...)
	fmt.Println("  Task deleted.")
}

// displayTaskPlan shows the current task plan.
func displayTaskPlan(state *core.WorkflowState) {
	fmt.Printf("\n  === Task Plan (%d tasks) ===\n", len(state.TaskOrder))
	for i, taskID := range state.TaskOrder {
		task, ok := state.Tasks[taskID]
		if !ok {
			continue
		}
		deps := ""
		if len(task.Dependencies) > 0 {
			depNums := make([]string, 0, len(task.Dependencies))
			for _, dep := range task.Dependencies {
				// Find the 1-based index of the dependency
				for j, tid := range state.TaskOrder {
					if tid == dep {
						depNums = append(depNums, strconv.Itoa(j+1))
						break
					}
				}
			}
			deps = fmt.Sprintf(" (depends: %s)", strings.Join(depNums, ", "))
		}
		fmt.Printf("  %d. [%s] %s%s\n", i+1, task.CLI, task.Name, deps)
	}
}

// displayTruncated displays text, truncated to maxLines.
func displayTruncated(text string, maxLines int) {
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		return
	}
	for _, line := range lines[:maxLines] {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("  ... (%d more lines)\n", len(lines)-maxLines)
}
