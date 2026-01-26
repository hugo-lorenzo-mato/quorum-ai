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

// Task output validation constants.
// These thresholds help detect when agents complete without producing meaningful output.
const (
	// MinExpectedTokensForCodeGen is the minimum output tokens expected for code generation tasks.
	// A minimal code change (e.g., a simple function) typically requires 200+ tokens.
	// Tasks with fewer tokens are flagged as suspicious.
	MinExpectedTokensForCodeGen = 200

	// MinExpectedTokensForImplementation is the minimum for larger implementation tasks.
	// Zustand stores, API handlers, and pages typically require 500+ tokens each.
	MinExpectedTokensForImplementation = 300

	// SuspiciouslyLowTokenThreshold is the absolute minimum that indicates likely no work done.
	// 100-170 tokens is approximately 2-3 sentences of natural language, not code.
	SuspiciouslyLowTokenThreshold = 150
)

// TaskOutputValidationResult contains the outcome of task output validation.
type TaskOutputValidationResult struct {
	Valid      bool
	Warning    string
	ToolCalls  int
	TokensOut  int
	HasFileOps bool
}

// fileWriteToolNames contains tool names that indicate file write operations.
var fileWriteToolNames = map[string]bool{
	"write_file":         true,
	"write":              true,
	"create_file":        true,
	"edit_file":          true,
	"edit":               true,
	"str_replace":        true,
	"str_replace_editor": true,
	"bash":               true, // May write files via shell
	"shell":              true,
	"execute":            true,
	"run":                true,
}

// GitClientFactory creates git clients for specific paths.
type GitClientFactory interface {
	NewClient(repoPath string) (core.GitClient, error)
}

// Executor runs tasks according to the dependency graph.
type Executor struct {
	dag        TaskDAG
	stateSaver StateSaver
	denyTools  []string
	gitFactory GitClientFactory
}

// NewExecutor creates a new executor.
func NewExecutor(dag TaskDAG, stateSaver StateSaver, denyTools []string) *Executor {
	return &Executor{
		dag:        dag,
		stateSaver: stateSaver,
		denyTools:  denyTools,
	}
}

// WithGitFactory sets the git client factory for finalization.
func (e *Executor) WithGitFactory(factory GitClientFactory) *Executor {
	e.gitFactory = factory
	return e
}

// Run executes the execute phase.
func (e *Executor) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting execute phase",
		"workflow_id", wctx.State.WorkflowID,
		"tasks", len(wctx.State.Tasks),
	)

	// Clean up orphaned worktrees from previous failed executions.
	// This handles cases where the process crashed/panicked and defers didn't run.
	e.cleanupOrphanedWorktrees(ctx, wctx)

	// Defensive validation: ensure tasks exist before executing
	// This catches edge cases like corrupted checkpoints or skipped planning
	if len(wctx.State.Tasks) == 0 {
		return core.ErrValidation(core.CodeMissingTasks, "no tasks to execute: the planning phase may have failed or been skipped")
	}

	wctx.State.CurrentPhase = core.PhaseExecute
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseExecute)
		wctx.Output.Log("info", "executor", fmt.Sprintf("Starting execution phase with %d tasks", len(wctx.State.Tasks)))
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
		// Check for cancellation/pause before each batch
		if wctx.Control != nil {
			if err := wctx.Control.CheckCancelled(); err != nil {
				return err
			}
			if err := wctx.Control.WaitIfPaused(ctx); err != nil {
				return err
			}
		}

		ready := e.dag.GetReadyTasks(completed)
		if len(ready) == 0 {
			return core.ErrState(core.CodeExecutionStuck, "no ready tasks but not all completed")
		}

		wctx.Logger.Info("executing task batch",
			"ready_count", len(ready),
			"completed_count", len(completed),
			"total_count", len(wctx.State.Tasks),
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "executor", fmt.Sprintf("Executing batch: %d ready, %d/%d completed", len(ready), len(completed), len(wctx.State.Tasks)))
		}

		// Execute ready tasks in parallel
		useWorktrees := shouldUseWorktrees(wctx.Config.WorktreeMode, len(ready))
		g, taskCtx := errgroup.WithContext(ctx)
		for _, task := range ready {
			task := task
			g.Go(func() error {
				return e.executeTaskSafe(taskCtx, wctx, task, useWorktrees)
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
		// Note: Per-task state save now happens in executeTask() immediately after each task completes
		// This eliminates the need for batch-level saves and enables finer-grained recovery
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseExecute, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	if wctx.Output != nil {
		wctx.Output.Log("success", "executor", fmt.Sprintf("Execution phase completed: %d tasks", len(wctx.State.Tasks)))
	}

	return nil
}

// executeTask executes a single task.
func (e *Executor) executeTask(ctx context.Context, wctx *Context, task *core.Task, useWorktrees bool) error {
	if err := e.checkControl(ctx, wctx); err != nil {
		return err
	}

	wctx.Logger.Info("executing task",
		"task_id", task.ID,
		"task_name", task.Name,
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "executor", fmt.Sprintf("Task started: %s", task.Name))
	}

	if err := e.enforceMode(ctx, wctx, task); err != nil {
		return err
	}

	taskState, startTime, err := e.startTask(wctx, task)
	if err != nil {
		return err
	}

	e.notifyTaskStarted(wctx, task)

	var taskErr error
	defer func() {
		e.notifyTaskCompletion(wctx, task, taskState, startTime, taskErr)
	}()

	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, false); err != nil {
		wctx.Logger.Warn("failed to create task checkpoint", "error", err)
	}

	if wctx.Config.DryRun {
		return e.completeDryRun(wctx, taskState)
	}

	workDir, worktreeCreated := e.setupWorktree(ctx, wctx, task, taskState, useWorktrees)
	defer e.cleanupWorktree(ctx, wctx, task, worktreeCreated)

	fail := func(err error) error {
		e.setTaskFailed(wctx, taskState, err)
		taskErr = err
		return err
	}

	agentName := task.CLI
	if agentName == "" {
		agentName = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return fail(err)
	}

	if err := e.acquireRateLimit(wctx, agentName); err != nil {
		return fail(fmt.Errorf("rate limit: %w", err))
	}

	prompt, err := wctx.Prompts.RenderTaskExecute(TaskExecuteParams{
		Task:    task,
		Context: wctx.GetContextString(),
	})
	if err != nil {
		return fail(err)
	}

	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseExecute, task.Model)
	execStartTime := time.Now()

	e.logTaskExecutionStart(wctx, task, agentName, model, workDir, prompt)
	e.notifyAgentStarted(wctx, agentName, task, model, workDir)

	result, retryCount, durationMS, execErr := e.executeWithRetry(ctx, wctx, agent, agentName, task, prompt, model, workDir, execStartTime)

	wctx.Lock()
	taskState.Retries = retryCount
	wctx.Unlock()

	if execErr != nil {
		taskErr = execErr
		return e.handleExecutionFailure(wctx, task, taskState, agentName, model, retryCount, durationMS, execErr)
	}

	if err := e.handleExecutionSuccess(ctx, wctx, task, taskState, agentName, result, workDir, durationMS); err != nil {
		taskErr = err
		return err
	}
	return nil
}

func (e *Executor) checkControl(ctx context.Context, wctx *Context) error {
	if wctx.Control == nil {
		return nil
	}
	if err := wctx.Control.CheckCancelled(); err != nil {
		return err
	}
	return wctx.Control.WaitIfPaused(ctx)
}

func (e *Executor) enforceMode(ctx context.Context, wctx *Context, task *core.Task) error {
	if wctx.ModeEnforcer == nil {
		return nil
	}

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
	return nil
}

func (e *Executor) startTask(wctx *Context, task *core.Task) (*core.TaskState, time.Time, error) {
	wctx.Lock()
	defer wctx.Unlock()

	taskState := wctx.State.Tasks[task.ID]
	if taskState == nil {
		return nil, time.Time{}, fmt.Errorf("task state not found: %s", task.ID)
	}

	startTime := time.Now()
	taskState.Status = core.TaskStatusRunning
	taskState.StartedAt = &startTime
	return taskState, startTime, nil
}

func (e *Executor) notifyTaskStarted(wctx *Context, task *core.Task) {
	if wctx.Output != nil {
		wctx.Output.TaskStarted(task)
	}
}

func (e *Executor) notifyTaskCompletion(wctx *Context, task *core.Task, taskState *core.TaskState, startTime time.Time, taskErr error) {
	if wctx.Output == nil {
		return
	}
	duration := time.Since(startTime)
	if taskState.Status == core.TaskStatusCompleted {
		wctx.Output.TaskCompleted(task, duration)
		return
	}
	if taskState.Status == core.TaskStatusFailed {
		if taskErr == nil {
			taskErr = fmt.Errorf("task failed")
		}
		wctx.Output.TaskFailed(task, taskErr)
	}
}

func (e *Executor) completeDryRun(wctx *Context, taskState *core.TaskState) error {
	wctx.Lock()
	taskState.Status = core.TaskStatusCompleted
	completedAt := time.Now()
	taskState.CompletedAt = &completedAt
	wctx.Unlock()
	return nil
}

func (e *Executor) acquireRateLimit(wctx *Context, agentName string) error {
	limiter := wctx.RateLimits.Get(agentName)
	return limiter.Acquire()
}

func (e *Executor) logTaskExecutionStart(wctx *Context, task *core.Task, agentName, model, workDir, prompt string) {
	promptPreview := prompt
	if len(promptPreview) > 500 {
		promptPreview = promptPreview[:500] + "... [truncated]"
	}
	wctx.Logger.Info("executor: starting task execution",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", model,
		"workdir", workDir,
		"timeout", wctx.Config.PhaseTimeouts.Execute,
		"prompt_length", len(prompt),
		"prompt_preview", promptPreview,
	)
}

func (e *Executor) notifyAgentStarted(wctx *Context, agentName string, task *core.Task, model, workDir string) {
	if wctx.Output == nil {
		return
	}
	wctx.Output.AgentEvent("started", agentName, "Executing task: "+task.Name, map[string]interface{}{
		"task_id":         string(task.ID),
		"task_name":       task.Name,
		"phase":           "execute",
		"model":           model,
		"workdir":         workDir,
		"timeout_seconds": int(wctx.Config.PhaseTimeouts.Execute.Seconds()),
	})
}

func (e *Executor) executeWithRetry(ctx context.Context, wctx *Context, agent core.Agent, agentName string, task *core.Task, prompt, model, workDir string, execStartTime time.Time) (result *core.ExecuteResult, retryCount int, durationMS int64, err error) {
	err = wctx.Retry.ExecuteWithNotify(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:      prompt,
			Format:      core.OutputFormatText,
			Model:       model,
			Timeout:     wctx.Config.PhaseTimeouts.Execute,
			Sandbox:     wctx.Config.Sandbox,
			DeniedTools: e.denyTools,
			WorkDir:     workDir, // Execute in worktree if available
			Phase:       core.PhaseExecute,
		})
		return execErr
	}, func(attempt int, retryErr error) {
		wctx.Logger.Warn("task retry",
			"task_id", task.ID,
			"attempt", attempt,
			"error", retryErr,
		)
		if wctx.Output != nil {
			wctx.Output.AgentEvent("progress", agentName, fmt.Sprintf("Retry attempt %d: %s", attempt, retryErr.Error()), map[string]interface{}{
				"task_id":     string(task.ID),
				"attempt":     attempt,
				"error":       retryErr.Error(),
				"duration_ms": time.Since(execStartTime).Milliseconds(),
			})
		}
		retryCount = attempt
	})

	durationMS = time.Since(execStartTime).Milliseconds()
	return result, retryCount, durationMS, err
}

func (e *Executor) handleExecutionFailure(wctx *Context, task *core.Task, taskState *core.TaskState, agentName, model string, retryCount int, durationMS int64, err error) error {
	wctx.Logger.Error("executor: task execution failed",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", model,
		"retries", retryCount,
		"duration_ms", durationMS,
		"error", err,
		"error_type", fmt.Sprintf("%T", err),
	)

	if wctx.Output != nil {
		wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
			"task_id":     string(task.ID),
			"task_name":   task.Name,
			"model":       model,
			"retries":     retryCount,
			"duration_ms": durationMS,
			"error_type":  fmt.Sprintf("%T", err),
		})
	}
	e.setTaskFailed(wctx, taskState, err)
	return err
}

func (e *Executor) handleExecutionSuccess(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, agentName string, result *core.ExecuteResult, workDir string, durationMS int64) error {
	validation := e.validateTaskOutput(result, task)
	if !validation.Valid {
		validationErr := fmt.Errorf("task output validation failed: %s", validation.Warning)
		wctx.Logger.Error("executor: task output validation failed",
			"task_id", task.ID,
			"task_name", task.Name,
			"agent", agentName,
			"tokens_out", result.TokensOut,
			"tool_calls", len(result.ToolCalls),
			"has_file_ops", validation.HasFileOps,
			"warning", validation.Warning,
		)
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, validation.Warning, map[string]interface{}{
				"task_id":      string(task.ID),
				"task_name":    task.Name,
				"tokens_out":   result.TokensOut,
				"tool_calls":   len(result.ToolCalls),
				"has_file_ops": validation.HasFileOps,
				"duration_ms":  durationMS,
			})
		}
		e.setTaskFailed(wctx, taskState, validationErr)
		return validationErr
	}

	if validation.Warning != "" {
		wctx.Logger.Warn("executor: task output validation warning",
			"task_id", task.ID,
			"task_name", task.Name,
			"warning", validation.Warning,
			"tokens_out", result.TokensOut,
			"tool_calls", len(result.ToolCalls),
		)
		if wctx.Output != nil {
			wctx.Output.Log("warn", "executor", fmt.Sprintf("Task %s: %s", task.Name, validation.Warning))
		}
	}

	// Log successful completion
	wctx.Logger.Info("executor: task completed successfully",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", result.Model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD,
		"duration_ms", durationMS,
		"finish_reason", result.FinishReason,
		"tool_calls", len(result.ToolCalls),
		"has_file_ops", validation.HasFileOps,
	)

	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, task.Name, map[string]interface{}{
			"task_id":       string(task.ID),
			"task_name":     task.Name,
			"model":         result.Model,
			"tokens_in":     result.TokensIn,
			"tokens_out":    result.TokensOut,
			"cost_usd":      result.CostUSD,
			"duration_ms":   durationMS,
			"tool_calls":    len(result.ToolCalls),
			"has_file_ops":  validation.HasFileOps,
			"finish_reason": result.FinishReason,
		})
	}

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

	if len(result.Output) <= core.MaxInlineOutputSize {
		taskState.Output = result.Output
	} else {
		taskState.Output = result.Output[:core.MaxInlineOutputSize] + "\n... [truncated, see output_file]"
		outputPath := e.saveTaskOutput(task.ID, result.Output)
		if outputPath != "" {
			taskState.OutputFile = outputPath
		}
	}

	if wctx.State.Metrics != nil {
		wctx.State.Metrics.TotalTokensIn += result.TokensIn
		wctx.State.Metrics.TotalTokensOut += result.TokensOut
		wctx.State.Metrics.TotalCostUSD += result.CostUSD
	}
	wctx.Unlock()

	if wctx.ModeEnforcer != nil {
		wctx.ModeEnforcer.RecordCost(result.CostUSD)
	}

	if costErr := e.checkCostLimits(wctx, task, taskState, result.CostUSD); costErr != nil {
		return costErr
	}

	if finalizeErr := e.finalizeTask(ctx, wctx, task, taskState, workDir); finalizeErr != nil {
		wctx.Logger.Warn("task finalization failed",
			"task_id", task.ID,
			"error", finalizeErr,
		)
		if wctx.Output != nil {
			wctx.Output.Log("warn", "executor", fmt.Sprintf("Task finalization failed: %s", finalizeErr.Error()))
		}
	}

	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, true); err != nil {
		wctx.Logger.Warn("failed to create task complete checkpoint", "error", err)
	}

	if e.stateSaver != nil {
		if saveErr := e.stateSaver.Save(ctx, wctx.State); saveErr != nil {
			wctx.Logger.Error("failed to save state after task completion",
				"task_id", task.ID,
				"error", saveErr,
			)
			return fmt.Errorf("failed to save state after task %s: %w", task.ID, saveErr)
		}
		wctx.Logger.Debug("state saved after task completion", "task_id", task.ID)
	}

	if wctx.Output != nil {
		wctx.Output.WorkflowStateUpdated(wctx.State)
		wctx.Output.Log("success", "executor", fmt.Sprintf("Task completed: %s ($%.4f)", task.Name, result.CostUSD))
	}

	return nil
}

// setTaskFailed marks a task as failed with the given error.
func (e *Executor) setTaskFailed(wctx *Context, taskState *core.TaskState, err error) {
	wctx.Lock()
	taskState.Status = core.TaskStatusFailed
	taskState.Error = err.Error()
	wctx.Unlock()
	if wctx.Output != nil {
		wctx.Output.Log("error", "executor", fmt.Sprintf("Task failed: %s", err.Error()))
	}
}

// checkCostLimits verifies task and workflow cost limits.
func (e *Executor) checkCostLimits(wctx *Context, task *core.Task, taskState *core.TaskState, cost float64) error {
	if wctx.Config == nil {
		return nil
	}

	// Check task cost limit
	if wctx.Config.MaxCostPerTask > 0 && cost > wctx.Config.MaxCostPerTask {
		e.setTaskFailed(wctx, taskState, fmt.Errorf("task budget exceeded"))
		wctx.Logger.Error("task budget exceeded",
			"task_id", task.ID,
			"cost", cost,
			"limit", wctx.Config.MaxCostPerTask,
		)
		return core.ErrTaskBudgetExceeded(string(task.ID), cost, wctx.Config.MaxCostPerTask)
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

	return nil
}

// setupWorktree creates a worktree for task isolation if enabled.
// If the task has dependencies, the worktree is created from the most recent
// completed dependency's branch to inherit its changes.
func (e *Executor) setupWorktree(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, useWorktrees bool) (workDir string, created bool) {
	if !useWorktrees || wctx.Worktrees == nil {
		return "", false
	}

	// Find base branch from dependencies (if any)
	baseBranch := e.findDependencyBranch(wctx, task)

	var wtInfo *core.WorktreeInfo
	var err error

	if baseBranch != "" {
		// Create worktree from dependency's branch
		wctx.Logger.Info("creating worktree from dependency branch",
			"task_id", task.ID,
			"base_branch", baseBranch,
		)
		wtInfo, err = wctx.Worktrees.CreateFromBranch(ctx, task, "", baseBranch)
	} else {
		// Create worktree from HEAD
		wtInfo, err = wctx.Worktrees.Create(ctx, task, "")
	}

	if err != nil {
		wctx.Logger.Warn("failed to create worktree, executing in main repo",
			"task_id", task.ID,
			"error", err,
		)
		return "", false
	}

	wctx.Lock()
	taskState.WorktreePath = wtInfo.Path
	wctx.Unlock()

	wctx.Logger.Info("created worktree for task",
		"task_id", task.ID,
		"worktree_path", wtInfo.Path,
		"branch", wtInfo.Branch,
	)

	return wtInfo.Path, true
}

// findDependencyBranch returns the branch of the most recently completed dependency.
// This allows dependent tasks to start with the changes from their dependencies.
func (e *Executor) findDependencyBranch(wctx *Context, task *core.Task) string {
	if len(task.Dependencies) == 0 {
		return ""
	}

	wctx.RLock()
	defer wctx.RUnlock()

	// Find the most recently completed dependency with a worktree
	var latestBranch string
	for _, depID := range task.Dependencies {
		depState := wctx.State.Tasks[depID]
		if depState == nil || depState.Status != core.TaskStatusCompleted {
			continue
		}
		if depState.WorktreePath == "" {
			continue
		}

		// Get the branch from the dependency's worktree
		depTask := &core.Task{
			ID:          depID,
			Name:        depState.Name,
			Description: depState.Name, // Use name as description for worktree naming
		}
		wtInfo, err := wctx.Worktrees.Get(context.Background(), depTask)
		if err != nil || wtInfo == nil {
			continue
		}

		// Use the most recent dependency's branch
		// (In practice, we'd want to merge multiple branches, but for simplicity
		// we use the last one found - typically tasks have single dependencies)
		latestBranch = wtInfo.Branch
	}

	return latestBranch
}

// cleanupWorktree removes the worktree if auto_clean is enabled.
func (e *Executor) cleanupWorktree(ctx context.Context, wctx *Context, task *core.Task, created bool) {
	if !created || !wctx.Config.WorktreeAutoClean || wctx.Worktrees == nil {
		return
	}

	if rmErr := wctx.Worktrees.Remove(ctx, task); rmErr != nil {
		wctx.Logger.Warn("failed to cleanup worktree",
			"task_id", task.ID,
			"error", rmErr,
		)
	} else {
		wctx.Logger.Info("cleaned up worktree", "task_id", task.ID)
	}
}

// executeTaskSafe wraps executeTask with panic recovery.
// This ensures that if a task panics, the error is captured and the worktree
// cleanup in the defer still has a chance to run properly.
func (e *Executor) executeTaskSafe(ctx context.Context, wctx *Context, task *core.Task, useWorktrees bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error
			err = fmt.Errorf("task %s panicked: %v", task.ID, r)
			wctx.Logger.Error("task execution panicked",
				"task_id", task.ID,
				"panic", r,
			)

			// Mark task as failed
			wctx.Lock()
			if taskState := wctx.State.Tasks[task.ID]; taskState != nil {
				taskState.Status = core.TaskStatusFailed
				taskState.Error = err.Error()
			}
			wctx.Unlock()

			if wctx.Output != nil {
				wctx.Output.Log("error", "executor", fmt.Sprintf("Task panicked: %s - %v", task.ID, r))
			}
		}
	}()

	return e.executeTask(ctx, wctx, task, useWorktrees)
}

// cleanupOrphanedWorktrees removes worktrees from previous failed executions.
// This handles cases where the process crashed/panicked and defers didn't run,
// or when a task was interrupted and left in a running state.
func (e *Executor) cleanupOrphanedWorktrees(ctx context.Context, wctx *Context) {
	if wctx.Worktrees == nil {
		return
	}

	managed, err := wctx.Worktrees.List(ctx)
	if err != nil {
		wctx.Logger.Warn("failed to list worktrees for cleanup", "error", err)
		return
	}

	if len(managed) == 0 {
		return
	}

	wctx.Logger.Info("checking for orphaned worktrees", "count", len(managed))

	cleaned := 0
	for _, wt := range managed {
		// Check if this worktree belongs to a task that is NOT currently running
		taskState := wctx.State.Tasks[wt.TaskID]

		// Remove worktree if:
		// - Task doesn't exist in current workflow state
		// - Task is not in "running" status (pending, completed, failed are all orphaned)
		shouldRemove := taskState == nil || taskState.Status != core.TaskStatusRunning

		if shouldRemove {
			wctx.Logger.Info("removing orphaned worktree",
				"task_id", wt.TaskID,
				"path", wt.Path,
				"task_exists", taskState != nil,
			)

			// Create a minimal task struct for the Remove call
			orphanTask := &core.Task{
				ID:          wt.TaskID,
				Name:        string(wt.TaskID), // Use ID as name for worktree naming
				Description: string(wt.TaskID),
			}

			if rmErr := wctx.Worktrees.Remove(ctx, orphanTask); rmErr != nil {
				wctx.Logger.Warn("failed to remove orphaned worktree",
					"task_id", wt.TaskID,
					"error", rmErr,
				)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		wctx.Logger.Info("cleaned up orphaned worktrees", "count", cleaned)
		if wctx.Output != nil {
			wctx.Output.Log("info", "executor", fmt.Sprintf("Cleaned up %d orphaned worktrees from previous runs", cleaned))
		}
	}
}

// finalizeTask handles post-task git operations (commit, push, PR).
func (e *Executor) finalizeTask(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, workDir string) error {
	cfg := wctx.Config.Finalization
	if !cfg.AutoCommit {
		return nil
	}

	// Determine the git repo path and branch
	gitPath := workDir
	if gitPath == "" {
		// No worktree, use main repo
		if wctx.Git == nil {
			return nil
		}
		gitPath, _ = wctx.Git.RepoRoot(ctx)
	}
	if gitPath == "" {
		return nil
	}

	// Get the branch name from the worktree path (stored in taskState)
	branch := ""
	if taskState.WorktreePath != "" {
		// Extract branch from worktree info
		if wctx.Worktrees != nil {
			wtInfo, err := wctx.Worktrees.Get(ctx, task)
			if err == nil && wtInfo != nil {
				branch = wtInfo.Branch
			}
		}
	}
	if branch == "" {
		// Fall back to current branch in the working directory
		if wctx.Git != nil {
			branch, _ = wctx.Git.CurrentBranch(ctx)
		}
	}

	// Create a git client for the specific path
	var gitClient core.GitClient
	if e.gitFactory != nil {
		var err error
		gitClient, err = e.gitFactory.NewClient(gitPath)
		if err != nil {
			return fmt.Errorf("creating git client for worktree: %w", err)
		}
	} else if wctx.Git != nil {
		gitClient = wctx.Git
	} else {
		return nil
	}

	// Get modified files before commit for recovery metadata
	var modifiedFiles []string
	if status, statusErr := gitClient.Status(ctx); statusErr == nil {
		for _, f := range status.Staged {
			modifiedFiles = append(modifiedFiles, f.Path)
		}
		for _, f := range status.Unstaged {
			modifiedFiles = append(modifiedFiles, f.Path)
		}
		modifiedFiles = append(modifiedFiles, status.Untracked...)
	}

	// Create and run finalizer
	finalizer := NewTaskFinalizer(gitClient, wctx.GitHub, cfg)
	result, err := finalizer.Finalize(ctx, task, gitPath, branch)
	if err != nil {
		return err
	}

	// Store recovery metadata in taskState for resume capability
	wctx.Lock()
	if result.CommitSHA != "" {
		taskState.LastCommit = result.CommitSHA
	}
	if branch != "" {
		taskState.Branch = branch
	}
	if len(modifiedFiles) > 0 {
		taskState.FilesModified = modifiedFiles
	}
	// Mark task as resumable if it has a commit
	taskState.Resumable = result.CommitSHA != ""
	wctx.Unlock()

	// Log finalization results
	if result.CommitSHA != "" {
		wctx.Logger.Info("task committed",
			"task_id", task.ID,
			"commit", result.CommitSHA,
			"files_modified", len(modifiedFiles),
		)
	}
	if result.Pushed {
		wctx.Logger.Info("task pushed to remote",
			"task_id", task.ID,
			"branch", branch,
		)
	}
	if result.PRNumber > 0 {
		wctx.Logger.Info("PR created for task",
			"task_id", task.ID,
			"pr_number", result.PRNumber,
			"pr_url", result.PRURL,
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "executor", fmt.Sprintf("PR created: %s", result.PRURL))
		}
	}
	if result.Merged {
		wctx.Logger.Info("PR merged for task",
			"task_id", task.ID,
			"pr_number", result.PRNumber,
		)
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
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ""
	}

	// Write output file
	outputPath := filepath.Join(outputDir, string(taskID)+".txt")
	if err := os.WriteFile(outputPath, []byte(output), 0o600); err != nil {
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

// validateTaskOutput checks if the task execution produced meaningful output.
// This addresses the issue where agents may complete without error but produce no files.
// Returns validation result with details about tool calls and token counts.
func (e *Executor) validateTaskOutput(result *core.ExecuteResult, task *core.Task) TaskOutputValidationResult {
	validation := TaskOutputValidationResult{
		Valid:     true,
		ToolCalls: len(result.ToolCalls),
		TokensOut: result.TokensOut,
	}

	// Check for file write operations in tool calls
	for _, tc := range result.ToolCalls {
		toolNameLower := strings.ToLower(tc.Name)
		if fileWriteToolNames[toolNameLower] {
			validation.HasFileOps = true
			break
		}
	}

	// Validation 1: Check for suspiciously low token output
	// Tasks with very low output tokens likely didn't produce code
	if result.TokensOut < SuspiciouslyLowTokenThreshold {
		validation.Valid = false
		validation.Warning = fmt.Sprintf(
			"task output suspiciously short: %d tokens (expected >=%d for code generation). "+
				"Agent may have responded with intent description instead of implementation.",
			result.TokensOut,
			SuspiciouslyLowTokenThreshold,
		)
		return validation
	}

	// Validation 2: Check for missing tool calls in implementation tasks
	// If task name suggests implementation but no tools were called, it's suspicious
	taskNameLower := strings.ToLower(task.Name)
	isImplementationTask := strings.Contains(taskNameLower, "implement") ||
		strings.Contains(taskNameLower, "create") ||
		strings.Contains(taskNameLower, "add") ||
		strings.Contains(taskNameLower, "build") ||
		strings.Contains(taskNameLower, "write") ||
		strings.Contains(taskNameLower, "develop")

	if isImplementationTask && len(result.ToolCalls) == 0 {
		// Implementation task with no tool calls and low-ish tokens is suspicious
		if result.TokensOut < MinExpectedTokensForImplementation {
			validation.Valid = false
			validation.Warning = fmt.Sprintf(
				"implementation task completed without tool calls and with low output (%d tokens). "+
					"Agent may not have executed file writes. Tool calls: %d",
				result.TokensOut,
				len(result.ToolCalls),
			)
			return validation
		}
		// Has some tokens but no tool calls - warn but don't fail
		validation.Warning = fmt.Sprintf(
			"implementation task completed without tool calls (tokens_out=%d). "+
				"Verify that expected files were created.",
			result.TokensOut,
		)
	}

	// Validation 3: Check token count for tasks that require substantial code
	// Frontend components, stores, handlers typically need 300+ tokens
	requiresSubstantialCode := strings.Contains(taskNameLower, "component") ||
		strings.Contains(taskNameLower, "page") ||
		strings.Contains(taskNameLower, "store") ||
		strings.Contains(taskNameLower, "handler") ||
		strings.Contains(taskNameLower, "frontend") ||
		strings.Contains(taskNameLower, "backend") ||
		strings.Contains(taskNameLower, "api")

	if requiresSubstantialCode && result.TokensOut < MinExpectedTokensForImplementation {
		if !validation.HasFileOps {
			validation.Valid = false
			validation.Warning = fmt.Sprintf(
				"task requiring substantial code completed with only %d tokens and no file operations. "+
					"Expected >=%d tokens for this type of task.",
				result.TokensOut,
				MinExpectedTokensForImplementation,
			)
			return validation
		}
		// Has file ops but low tokens - just warn
		if validation.Warning == "" {
			validation.Warning = fmt.Sprintf(
				"task output lower than expected: %d tokens (typically need %d+ for this task type)",
				result.TokensOut,
				MinExpectedTokensForImplementation,
			)
		}
	}

	return validation
}
