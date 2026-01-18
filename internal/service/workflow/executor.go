package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"golang.org/x/sync/errgroup"
)

// TaskDAG provides task scheduling based on dependencies.
type TaskDAG interface {
	GetReadyTasks(completed map[core.TaskID]bool) []*core.Task
}

// Executor runs tasks according to the dependency graph.
type Executor struct {
	dag        TaskDAG
	stateSaver StateSaver
	denyTools  []string
}

// NewExecutor creates a new executor.
func NewExecutor(dag TaskDAG, stateSaver StateSaver, denyTools []string) *Executor {
	return &Executor{
		dag:        dag,
		stateSaver: stateSaver,
		denyTools:  denyTools,
	}
}

// Run executes the execute phase.
func (e *Executor) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting execute phase",
		"workflow_id", wctx.State.WorkflowID,
		"tasks", len(wctx.State.Tasks),
	)

	wctx.State.CurrentPhase = core.PhaseExecute
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseExecute)
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseExecute, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	completed := make(map[core.TaskID]bool)

	// Find already completed tasks
	for id, task := range wctx.State.Tasks {
		if task.Status == core.TaskStatusCompleted {
			completed[id] = true
		}
	}

	// Execute remaining tasks
	for len(completed) < len(wctx.State.Tasks) {
		ready := e.dag.GetReadyTasks(completed)
		if len(ready) == 0 {
			return core.ErrState(core.CodeExecutionStuck, "no ready tasks but not all completed")
		}

		wctx.Logger.Info("executing task batch",
			"ready_count", len(ready),
			"completed_count", len(completed),
			"total_count", len(wctx.State.Tasks),
		)

		// Execute ready tasks in parallel
		useWorktrees := shouldUseWorktrees(wctx.Config.WorktreeMode, len(ready))
		g, taskCtx := errgroup.WithContext(ctx)
		for _, task := range ready {
			task := task
			g.Go(func() error {
				return e.executeTask(taskCtx, wctx, task, useWorktrees)
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// Update completed set
		for _, task := range ready {
			if wctx.State.Tasks[task.ID].Status == core.TaskStatusCompleted {
				completed[task.ID] = true
			}
		}

		// Save state after each batch
		if err := e.stateSaver.Save(ctx, wctx.State); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseExecute, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// executeTask executes a single task.
func (e *Executor) executeTask(ctx context.Context, wctx *Context, task *core.Task, useWorktrees bool) error {
	wctx.Logger.Info("executing task",
		"task_id", task.ID,
		"task_name", task.Name,
	)

	// Enforce mode restrictions before execution
	if wctx.ModeEnforcer != nil {
		op := ModeOperation{
			Name:           task.Name,
			Type:           "llm",
			Tool:           task.CLI,
			HasSideEffects: !wctx.Config.DryRun,
			InWorkspace:    true,
			// In sandbox mode, LLM operations are allowed but shell commands aren't
			AllowedInSandbox: true,
		}

		if err := wctx.ModeEnforcer.CanExecute(ctx, op); err != nil {
			wctx.Logger.Warn("task blocked by mode enforcer",
				"task_id", task.ID,
				"error", err,
			)
			return fmt.Errorf("mode enforcer blocked task: %w", err)
		}
	}

	// Update task state (with lock for concurrent access)
	wctx.Lock()
	taskState := wctx.State.Tasks[task.ID]
	if taskState == nil {
		wctx.Unlock()
		return fmt.Errorf("task state not found: %s", task.ID)
	}

	startTime := time.Now()
	taskState.Status = core.TaskStatusRunning
	taskState.StartedAt = &startTime
	wctx.Unlock()

	// Notify output that task has started
	if wctx.Output != nil {
		wctx.Output.TaskStarted(task)
	}

	// Notify output when task completes (success or failure)
	var taskErr error
	defer func() {
		if wctx.Output != nil {
			duration := time.Since(startTime)
			if taskState.Status == core.TaskStatusCompleted {
				wctx.Output.TaskCompleted(task, duration)
			} else if taskState.Status == core.TaskStatusFailed {
				wctx.Output.TaskFailed(task, taskErr)
			}
		}
	}()

	// Create task checkpoint
	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, false); err != nil {
		wctx.Logger.Warn("failed to create task checkpoint", "error", err)
	}

	// Skip in dry-run mode
	if wctx.Config.DryRun {
		wctx.Lock()
		taskState.Status = core.TaskStatusCompleted
		completedAt := time.Now()
		taskState.CompletedAt = &completedAt
		wctx.Unlock()
		return nil
	}

	// Create worktree for task isolation
	var workDir string
	var worktreeCreated bool
	if useWorktrees && wctx.Worktrees != nil {
		wtInfo, err := wctx.Worktrees.Create(ctx, task, "")
		if err != nil {
			wctx.Logger.Warn("failed to create worktree, executing in main repo",
				"task_id", task.ID,
				"error", err,
			)
		} else {
			workDir = wtInfo.Path
			wctx.Lock()
			taskState.WorktreePath = workDir
			wctx.Unlock()
			worktreeCreated = true
			wctx.Logger.Info("created worktree for task",
				"task_id", task.ID,
				"worktree_path", workDir,
			)
		}
	}

	// Cleanup worktree when done (if auto_clean is enabled)
	defer func() {
		if worktreeCreated && wctx.Config.WorktreeAutoClean && wctx.Worktrees != nil {
			if rmErr := wctx.Worktrees.Remove(ctx, task); rmErr != nil {
				wctx.Logger.Warn("failed to cleanup worktree",
					"task_id", task.ID,
					"error", rmErr,
				)
			} else {
				wctx.Logger.Info("cleaned up worktree",
					"task_id", task.ID,
				)
			}
		}
	}()

	// Get agent
	agentName := task.CLI
	if agentName == "" {
		agentName = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		wctx.Lock()
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		wctx.Unlock()
		taskErr = err
		return err
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		wctx.Lock()
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		wctx.Unlock()
		taskErr = err
		return fmt.Errorf("rate limit: %w", err)
	}

	// Render task prompt
	prompt, err := wctx.Prompts.RenderTaskExecute(TaskExecuteParams{
		Task:    task,
		Context: wctx.GetContextString(),
	})
	if err != nil {
		wctx.Lock()
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		wctx.Unlock()
		taskErr = err
		return err
	}

	// Execute with retry
	var result *core.ExecuteResult
	var retryCount int
	err = wctx.Retry.ExecuteWithNotify(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:      prompt,
			Format:      core.OutputFormatText,
			Model:       ResolvePhaseModel(wctx.Config, agentName, core.PhaseExecute, task.Model),
			Timeout:     10 * time.Minute,
			Sandbox:     wctx.Config.Sandbox,
			DeniedTools: e.denyTools,
			WorkDir:     workDir, // Execute in worktree if available
			Phase:       core.PhaseExecute,
		})
		return execErr
	}, func(attempt int, err error) {
		wctx.Logger.Warn("task retry",
			"task_id", task.ID,
			"attempt", attempt,
			"error", err,
		)
		retryCount = attempt
	})

	wctx.Lock()
	taskState.Retries = retryCount
	wctx.Unlock()

	if err != nil {
		wctx.Lock()
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		wctx.Unlock()
		taskErr = err
		return err
	}

	// Update task metrics
	wctx.Lock()
	taskState.TokensIn = result.TokensIn
	taskState.TokensOut = result.TokensOut
	taskState.CostUSD = result.CostUSD
	taskState.Status = core.TaskStatusCompleted
	completedAt := time.Now()
	taskState.CompletedAt = &completedAt
	taskState.ModelUsed = result.Model
	taskState.FinishReason = result.FinishReason
	taskState.ToolCalls = result.ToolCalls

	// Save output (truncate if too large)
	if len(result.Output) <= core.MaxInlineOutputSize {
		taskState.Output = result.Output
	} else {
		// Store truncated version inline
		taskState.Output = result.Output[:core.MaxInlineOutputSize] + "\n... [truncated, see output_file]"
		// Save full output to file
		outputPath := e.saveTaskOutput(task.ID, result.Output)
		if outputPath != "" {
			taskState.OutputFile = outputPath
		}
	}

	// Update aggregate metrics
	if wctx.State.Metrics != nil {
		wctx.State.Metrics.TotalTokensIn += result.TokensIn
		wctx.State.Metrics.TotalTokensOut += result.TokensOut
		wctx.State.Metrics.TotalCostUSD += result.CostUSD
	}
	wctx.Unlock()

	// Record cost with ModeEnforcer
	if wctx.ModeEnforcer != nil {
		wctx.ModeEnforcer.RecordCost(result.CostUSD)
	}

	// Check cost limits
	if wctx.Config != nil {
		// Check task cost limit
		if wctx.Config.MaxCostPerTask > 0 && result.CostUSD > wctx.Config.MaxCostPerTask {
			wctx.Lock()
			taskState.Status = core.TaskStatusFailed
			taskState.Error = "task budget exceeded"
			wctx.Unlock()
			taskErr = core.ErrTaskBudgetExceeded(string(task.ID), result.CostUSD, wctx.Config.MaxCostPerTask)
			wctx.Logger.Error("task budget exceeded",
				"task_id", task.ID,
				"cost", result.CostUSD,
				"limit", wctx.Config.MaxCostPerTask,
			)
			return taskErr
		}

		// Check workflow cost limit
		wctx.RLock()
		totalCost := float64(0)
		if wctx.State.Metrics != nil {
			totalCost = wctx.State.Metrics.TotalCostUSD
		}
		wctx.RUnlock()
		if wctx.Config.MaxCostPerWorkflow > 0 && totalCost > wctx.Config.MaxCostPerWorkflow {
			wctx.Logger.Error("workflow budget exceeded",
				"total_cost", totalCost,
				"limit", wctx.Config.MaxCostPerWorkflow,
			)
			return core.ErrWorkflowBudgetExceeded(totalCost, wctx.Config.MaxCostPerWorkflow)
		}
	}

	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, true); err != nil {
		wctx.Logger.Warn("failed to create task complete checkpoint", "error", err)
	}

	return nil
}

func shouldUseWorktrees(mode string, readyCount int) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "always":
		return true
	case "parallel":
		return readyCount > 1
	case "disabled", "off", "false":
		return false
	default:
		return true
	}
}

// saveTaskOutput saves large task output to a file.
func (e *Executor) saveTaskOutput(taskID core.TaskID, output string) string {
	// Create outputs directory
	outputDir := ".quorum/outputs"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return ""
	}

	// Write output file
	outputPath := filepath.Join(outputDir, string(taskID)+".txt")
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		return ""
	}

	return outputPath
}

// GetFullOutput retrieves the full output for a task.
// If output was truncated, reads from the output file.
func GetFullOutput(state *core.TaskState) (string, error) {
	if state.OutputFile == "" {
		return state.Output, nil
	}

	data, err := os.ReadFile(state.OutputFile)
	if err != nil {
		return state.Output, fmt.Errorf("reading output file: %w", err)
	}

	return string(data), nil
}
