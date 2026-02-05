package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
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
	// GitChanges indicates whether real filesystem changes were detected.
	// This is the primary validation signal - if files changed, the task produced real output.
	GitChanges bool
}

// GitChangesInfo contains information about filesystem changes detected after task execution.
// This is used to validate that agents actually produced output, regardless of their
// internal metrics (tokens, tool calls) which may not be reported accurately.
type GitChangesInfo struct {
	HasChanges    bool     // True if any files were modified, added, or deleted
	ModifiedFiles []string // Files that were modified
	AddedFiles    []string // New files (untracked)
	DeletedFiles  []string // Files that were deleted
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

		// Execute ready tasks in parallel with isolated contexts
		// Each task gets its own context to prevent cascade cancellation when one fails.
		// This allows fallback agents to work properly and other tasks to complete independently.
		useWorktrees := shouldUseWorktrees(wctx.Config.WorktreeMode, len(ready))

		// Force worktrees when workflow isolation is enabled, regardless of mode.
		// Workflow isolation requires task branches to exist for merge operations.
		if wctx.UseWorkflowIsolation() {
			useWorktrees = true
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		var failedTasks []core.TaskID

		for _, task := range ready {
			task := task // Capture for closure
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Each task gets its own context with timeout
				// Parent ctx cancellation still propagates (workflow-level cancel)
				taskCtx, taskCancel := context.WithTimeout(ctx, wctx.Config.PhaseTimeouts.Execute)
				defer taskCancel()

				err := e.executeTaskSafe(taskCtx, wctx, task, useWorktrees)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					failedTasks = append(failedTasks, task.ID)
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		if firstErr != nil {
			wctx.Logger.Error("batch execution had failures",
				"failed_count", len(failedTasks),
				"failed_tasks", failedTasks,
				"first_error", firstErr,
			)
			return firstErr
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

	// Setup worktree (workflow-scoped if isolation enabled, otherwise legacy)
	workDir, worktreeCreated := e.setupWorkflowScopedWorktree(ctx, wctx, task, taskState, useWorktrees)
	defer e.cleanupWorkflowScopedWorktree(ctx, wctx, task, worktreeCreated)

	fail := func(err error) error {
		e.setTaskFailed(wctx, taskState, err)
		taskErr = err
		return err
	}

	// Get primary agent
	primaryAgent := task.CLI
	if primaryAgent == "" {
		primaryAgent = wctx.Config.DefaultAgent
	}

	// Build list of agents to try: primary first, then other agents enabled for execute phase
	agentsToTry := []string{primaryAgent}
	enabledAgents := wctx.Agents.ListEnabledForPhase(string(core.PhaseExecute))
	for _, name := range enabledAgents {
		if name != primaryAgent {
			agentsToTry = append(agentsToTry, name)
		}
	}

	// Build context, including any workflow attachments with paths reachable from the execution directory.
	// Priority: worktree workDir > project root > current working directory
	displayWorkDir := workDir
	if strings.TrimSpace(displayWorkDir) == "" {
		if wctx.ProjectRoot != "" {
			displayWorkDir = wctx.ProjectRoot
		} else if wd, err := os.Getwd(); err == nil {
			displayWorkDir = wd
		}
	}
	execContext := wctx.GetContextString()
	if attCtx := BuildAttachmentsContext(wctx.State, displayWorkDir); attCtx != "" {
		execContext = execContext + "\n\n" + attCtx
	}

	prompt, err := wctx.Prompts.RenderTaskExecute(TaskExecuteParams{
		Task:        task,
		Context:     execContext,
		WorkDir:     displayWorkDir,
		Constraints: nil,
	})
	if err != nil {
		return fail(err)
	}

	var lastErr error
	var lastAgentName string
	var lastModel string
	var lastRetryCount int
	var lastDurationMS int64

	// Try each agent in order until one succeeds
	for agentIdx, agentName := range agentsToTry {
		agent, err := wctx.Agents.Get(agentName)
		if err != nil {
			wctx.Logger.Warn("executor: agent not available, trying next",
				"agent", agentName,
				"error", err,
				"task_id", task.ID,
			)
			lastErr = err
			lastAgentName = agentName
			continue
		}

		if err := e.acquireRateLimit(wctx, agentName); err != nil {
			wctx.Logger.Warn("executor: rate limit error, trying next agent",
				"agent", agentName,
				"error", err,
				"task_id", task.ID,
			)
			lastErr = fmt.Errorf("rate limit: %w", err)
			lastAgentName = agentName
			continue
		}

		model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseExecute, task.Model)
		execStartTime := time.Now()

		// Log fallback attempt if not the primary agent
		if agentIdx > 0 {
			wctx.Logger.Info("executor: attempting fallback agent",
				"task_id", task.ID,
				"task_name", task.Name,
				"fallback_agent", agentName,
				"attempt", agentIdx+1,
				"previous_agent", agentsToTry[agentIdx-1],
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "executor", fmt.Sprintf("Trying fallback agent %s for task %s", agentName, task.Name))
			}
		}

		e.logTaskExecutionStart(wctx, task, agentName, model, workDir, prompt)
		e.notifyAgentStarted(wctx, agentName, task, model, workDir)

		result, retryCount, durationMS, execErr := e.executeWithRetry(ctx, wctx, agent, agentName, task, prompt, model, workDir, execStartTime)

		wctx.Lock()
		taskState.Retries = retryCount
		wctx.Unlock()

		if execErr != nil {
			lastErr = execErr
			lastAgentName = agentName
			lastModel = model
			lastRetryCount = retryCount
			lastDurationMS = durationMS

			// Log failure and continue to next agent if available
			wctx.Logger.Warn("executor: agent failed, checking for fallback",
				"task_id", task.ID,
				"agent", agentName,
				"error", execErr,
				"has_more_fallbacks", agentIdx < len(agentsToTry)-1,
			)

			// If there are more agents to try, continue
			if agentIdx < len(agentsToTry)-1 {
				if wctx.Output != nil {
					// Use "error" to stop the timer for this agent, include fallback info in data
					wctx.Output.AgentEvent("error", agentName, fmt.Sprintf("Agent failed, trying next: %v", execErr), map[string]interface{}{
						"task_id":     string(task.ID),
						"next_agent":  agentsToTry[agentIdx+1],
						"is_fallback": true,
						"duration_ms": durationMS,
					})
				}
				continue
			}

			// No more agents to try, fail the task
			taskErr = execErr
			return e.handleExecutionFailure(wctx, task, taskState, lastAgentName, lastModel, lastRetryCount, lastDurationMS, execErr)
		}

		// Detect actual git changes before validation
		// This is the primary signal - if files changed, the task produced real output
		gitChanges := e.detectGitChanges(ctx, wctx, workDir)

		// Validate output before declaring success - validation failure should trigger fallback
		validation := e.validateTaskOutput(result, task, gitChanges)
		if !validation.Valid {
			validationErr := fmt.Errorf("task output validation failed: %s", validation.Warning)
			wctx.Logger.Warn("executor: task output validation failed, checking for fallback",
				"task_id", task.ID,
				"task_name", task.Name,
				"agent", agentName,
				"tokens_out", result.TokensOut,
				"git_changes", gitChanges.HasChanges,
				"warning", validation.Warning,
				"has_more_fallbacks", agentIdx < len(agentsToTry)-1,
			)

			lastErr = validationErr
			lastAgentName = agentName

			// If there are more agents to try, continue with fallback
			if agentIdx < len(agentsToTry)-1 {
				if wctx.Output != nil {
					// Use "error" to stop the timer for this agent, include fallback info in data
					wctx.Output.AgentEvent("error", agentName, fmt.Sprintf("Output validation failed, trying next agent: %v", validationErr), map[string]interface{}{
						"task_id":     string(task.ID),
						"next_agent":  agentsToTry[agentIdx+1],
						"tokens_out":  result.TokensOut,
						"is_fallback": true,
						"duration_ms": durationMS,
					})
				}
				continue
			}

			// No more agents to try, fail the task
			taskErr = validationErr
			e.setTaskFailed(wctx, taskState, validationErr)
			return validationErr
		}

		// Success! Handle the successful execution (validation already passed)
		if err := e.handleExecutionSuccessValidated(ctx, wctx, task, taskState, agentName, result, workDir, durationMS, validation); err != nil {
			taskErr = err
			return err
		}
		return nil
	}

	// All agents failed (should only reach here if all agents were unavailable)
	if lastErr != nil {
		return fail(fmt.Errorf("all agents failed for task %s: last error from %s: %w", task.Name, lastAgentName, lastErr))
	}
	return fail(fmt.Errorf("no agents available for task %s", task.Name))
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

// handleExecutionSuccessValidated handles successful execution when validation has already been performed.
func (e *Executor) handleExecutionSuccessValidated(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, agentName string, result *core.ExecuteResult, workDir string, durationMS int64, validation TaskOutputValidationResult) error {
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
		outputPath := e.saveTaskOutput(wctx, task.ID, result.Output)
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

	if finalizeErr := e.finalizeTask(ctx, wctx, task, taskState, workDir); finalizeErr != nil {
		// CR-2 FIX: Finalization errors (git commit, timeout, etc.) should mark task as failed.
		// Previously this only logged a warning, leaving status as "completed" with an error.
		wctx.Logger.Error("task finalization failed - marking task as failed",
			"task_id", task.ID,
			"error", finalizeErr,
		)
		if wctx.Output != nil {
			wctx.Output.Log("error", "executor", fmt.Sprintf("Task finalization failed: %s", finalizeErr.Error()))
		}
		e.setTaskFailed(wctx, taskState, fmt.Errorf("finalization failed: %w", finalizeErr))
		return finalizeErr
	}

	// Merge task to workflow branch if using workflow isolation
	// This happens after finalization so that the task's commits are merged
	if wctx.UseWorkflowIsolation() {
		if err := e.mergeTaskToWorkflow(ctx, wctx, task); err != nil {
			// Merge is required for correctness when using workflow isolation.
			wctx.Logger.Error("task completed but merge failed",
				"task_id", task.ID,
				"error", err,
			)
			if wctx.Output != nil {
				wctx.Output.Log("error", "executor", fmt.Sprintf("Task %s merge to workflow branch failed: %s", task.Name, err.Error()))
			}
			// MergePending is already set by mergeTaskToWorkflow
			e.setTaskFailed(wctx, taskState, err)
			return err
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

// =============================================================================
// Workflow-Scoped Worktree Methods
// =============================================================================

// setupWorkflowScopedWorktree creates a worktree for task isolation.
// If workflow isolation is enabled, it uses the WorkflowWorktreeManager to create
// a task worktree within the workflow namespace. Otherwise, falls back to legacy behavior.
func (e *Executor) setupWorkflowScopedWorktree(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState, useWorktrees bool) (workDir string, created bool) {
	if !useWorktrees {
		return "", false
	}

	// Check if we should use workflow isolation
	if wctx.UseWorkflowIsolation() {
		return e.setupWorktreeWithIsolation(ctx, wctx, task, taskState)
	}

	// Fall back to legacy worktree behavior
	return e.setupWorktree(ctx, wctx, task, taskState, true)
}

// setupWorktreeWithIsolation creates a task worktree using WorkflowWorktreeManager.
// The worktree is created within the workflow namespace, branching from the workflow branch.
func (e *Executor) setupWorktreeWithIsolation(ctx context.Context, wctx *Context, task *core.Task, taskState *core.TaskState) (workDir string, created bool) {
	wctx.RLock()
	workflowID := string(wctx.State.WorkflowID)
	wctx.RUnlock()

	wtInfo, err := wctx.WorkflowWorktrees.CreateTaskWorktree(ctx, workflowID, task)
	if err != nil {
		wctx.Logger.Warn("failed to create workflow-scoped worktree, falling back to non-isolated execution",
			"workflow_id", workflowID,
			"task_id", task.ID,
			"error", err,
		)
		// Fall back to non-isolated execution
		return "", false
	}

	wctx.Lock()
	taskState.WorktreePath = wtInfo.Path
	taskState.Branch = wtInfo.Branch
	wctx.Unlock()

	wctx.Logger.Info("created workflow-scoped worktree",
		"workflow_id", workflowID,
		"task_id", task.ID,
		"path", wtInfo.Path,
		"branch", wtInfo.Branch,
	)

	return wtInfo.Path, true
}

// cleanupWorkflowScopedWorktree cleans up the task worktree after execution.
// If workflow isolation is enabled, uses WorkflowWorktreeManager.
// The branch is NOT removed here as it may be needed for merge later.
func (e *Executor) cleanupWorkflowScopedWorktree(ctx context.Context, wctx *Context, task *core.Task, worktreeCreated bool) {
	if !worktreeCreated {
		return
	}

	if wctx.UseWorkflowIsolation() {
		wctx.RLock()
		workflowID := string(wctx.State.WorkflowID)
		wctx.RUnlock()

		// Don't remove the branch - it will be merged later
		if err := wctx.WorkflowWorktrees.RemoveTaskWorktree(ctx, workflowID, task.ID, false); err != nil {
			wctx.Logger.Warn("failed to remove workflow-scoped worktree",
				"workflow_id", workflowID,
				"task_id", task.ID,
				"error", err,
			)
		} else {
			wctx.Logger.Info("removed workflow-scoped worktree",
				"workflow_id", workflowID,
				"task_id", task.ID,
			)
		}
		return
	}

	// Fall back to legacy cleanup
	e.cleanupWorktree(ctx, wctx, task, true)
}

// mergeTaskToWorkflow merges the task branch to the workflow branch after completion.
// This integrates task changes into the workflow branch for subsequent tasks.
func (e *Executor) mergeTaskToWorkflow(ctx context.Context, wctx *Context, task *core.Task) error {
	if !wctx.UseWorkflowIsolation() {
		return nil // No merge needed without isolation
	}

	wctx.RLock()
	workflowID := string(wctx.State.WorkflowID)
	wctx.RUnlock()

	strategy := wctx.GitIsolation.MergeStrategy
	if strategy == "" {
		strategy = "sequential"
	}

	wctx.Logger.Info("merging task to workflow branch",
		"workflow_id", workflowID,
		"task_id", task.ID,
		"strategy", strategy,
	)

	if err := wctx.WorkflowWorktrees.MergeTaskToWorkflow(ctx, workflowID, task.ID, strategy); err != nil {
		// Update task state with merge failure info
		// Note: The actual status change to Failed is done by the caller (setTaskFailed)
		// Here we only set the recovery metadata (Resumable, MergePending)
		wctx.Lock()
		if taskState, ok := wctx.State.Tasks[task.ID]; ok {
			taskState.Resumable = true
			taskState.MergePending = true
			// Don't set Error here - let setTaskFailed do it consistently
		}
		wctx.Unlock()

		return fmt.Errorf("merging task %s to workflow: %w", task.ID, err)
	}

	wctx.Logger.Info("task merged to workflow branch",
		"workflow_id", workflowID,
		"task_id", task.ID,
	)

	return nil
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

	branch := strings.TrimSpace(taskState.Branch)

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

	// Resolve branch from the worktree git client when not already known.
	if branch == "" && gitClient != nil {
		if b, err := gitClient.CurrentBranch(ctx); err == nil {
			branch = strings.TrimSpace(b)
		}
	}
	if branch == "" && wctx.Git != nil {
		if b, err := wctx.Git.CurrentBranch(ctx); err == nil {
			branch = strings.TrimSpace(b)
		}
	}
	if branch == "" && (cfg.AutoPush || cfg.AutoPR) {
		return fmt.Errorf("could not determine current git branch for task finalization")
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
// Uses the report writer path when available, otherwise falls back to .quorum/outputs.
func (e *Executor) saveTaskOutput(wctx *Context, taskID core.TaskID, output string) string {
	var outputPath string

	// Use report writer path when available (outputs stored within run directory)
	if wctx != nil && wctx.Report != nil && wctx.Report.IsEnabled() {
		outputPath = wctx.Report.TaskOutputPath(string(taskID))
	} else {
		// Fallback: create outputs directory
		outputDir := ".quorum/outputs"
		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			return ""
		}
		outputPath = filepath.Join(outputDir, string(taskID)+".md")
	}

	// Write output file
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

// detectGitChanges checks for actual filesystem changes in the working directory.
// This is the primary validation signal - if files changed, the task produced real output,
// regardless of what the agent reports for tokens or tool calls.
func (e *Executor) detectGitChanges(ctx context.Context, wctx *Context, workDir string) *GitChangesInfo {
	info := &GitChangesInfo{}

	// Determine the git path
	gitPath := workDir
	if gitPath == "" {
		if wctx.Git == nil {
			return info
		}
		var err error
		gitPath, err = wctx.Git.RepoRoot(ctx)
		if err != nil || gitPath == "" {
			return info
		}
	}

	// Get a git client for the path
	var gitClient core.GitClient
	if e.gitFactory != nil {
		var err error
		gitClient, err = e.gitFactory.NewClient(gitPath)
		if err != nil {
			wctx.Logger.Debug("detectGitChanges: failed to create git client", "error", err)
			return info
		}
	} else if wctx.Git != nil {
		gitClient = wctx.Git
	} else {
		return info
	}

	// Get git status to detect changes
	status, err := gitClient.Status(ctx)
	if err != nil {
		wctx.Logger.Debug("detectGitChanges: failed to get git status", "error", err)
		return info
	}

	// Collect modified files (staged changes)
	for _, f := range status.Staged {
		info.ModifiedFiles = append(info.ModifiedFiles, f.Path)
	}

	// Collect unstaged modifications
	for _, f := range status.Unstaged {
		info.ModifiedFiles = append(info.ModifiedFiles, f.Path)
	}

	// Collect new/untracked files
	info.AddedFiles = append(info.AddedFiles, status.Untracked...)

	// Determine if there are any changes
	info.HasChanges = len(info.ModifiedFiles) > 0 || len(info.AddedFiles) > 0 || len(info.DeletedFiles) > 0

	if info.HasChanges {
		wctx.Logger.Debug("detectGitChanges: changes detected",
			"modified", len(info.ModifiedFiles),
			"added", len(info.AddedFiles),
			"deleted", len(info.DeletedFiles),
		)
	}

	return info
}

// validateTaskOutput checks if the task execution produced meaningful output.
// The primary validation signal is git changes - if files were modified, the task succeeded.
// Token counts and tool calls are used as fallback signals when git detection is unavailable.
func (e *Executor) validateTaskOutput(result *core.ExecuteResult, task *core.Task, gitChanges *GitChangesInfo) TaskOutputValidationResult {
	validation := TaskOutputValidationResult{
		Valid:     true,
		ToolCalls: len(result.ToolCalls),
		TokensOut: result.TokensOut,
	}

	// Check for file write operations in tool calls (legacy signal)
	for _, tc := range result.ToolCalls {
		toolNameLower := strings.ToLower(tc.Name)
		if fileWriteToolNames[toolNameLower] {
			validation.HasFileOps = true
			break
		}
	}

	// Set GitChanges flag if we detected real filesystem changes
	if gitChanges != nil && gitChanges.HasChanges {
		validation.GitChanges = true
	}

	// PRIMARY VALIDATION: Git changes are the source of truth.
	// If files were actually modified/created, the task produced real output,
	// regardless of what tokens or tool calls the agent reports.
	if validation.GitChanges {
		// Task produced real changes - this is a success
		// Add informational note about what changed
		totalChanges := len(gitChanges.ModifiedFiles) + len(gitChanges.AddedFiles)
		if totalChanges > 0 && result.TokensOut < MinExpectedTokensForImplementation {
			// Low tokens but real changes - just note it, don't warn
			validation.Warning = fmt.Sprintf(
				"task completed with %d file(s) changed (tokens_out=%d, possibly underreported)",
				totalChanges,
				result.TokensOut,
			)
		}
		return validation
	}

	// No git changes detected - fall back to token/tool call validation
	// This handles cases where git detection failed or task is read-only

	// Determine task type for appropriate validation
	taskNameLower := strings.ToLower(task.Name)
	isImplementationTask := strings.Contains(taskNameLower, "implement") ||
		strings.Contains(taskNameLower, "create") ||
		strings.Contains(taskNameLower, "add") ||
		strings.Contains(taskNameLower, "build") ||
		strings.Contains(taskNameLower, "write") ||
		strings.Contains(taskNameLower, "develop")

	isAnalysisTask := strings.Contains(taskNameLower, "analyze") ||
		strings.Contains(taskNameLower, "review") ||
		strings.Contains(taskNameLower, "check") ||
		strings.Contains(taskNameLower, "verify") ||
		strings.Contains(taskNameLower, "audit") ||
		strings.Contains(taskNameLower, "inspect") ||
		strings.Contains(taskNameLower, "read")

	// For analysis/read-only tasks, only require minimum token threshold
	if isAnalysisTask {
		if result.TokensOut < SuspiciouslyLowTokenThreshold {
			validation.Valid = false
			validation.Warning = fmt.Sprintf(
				"analysis task output too short: %d tokens (expected >=%d)",
				result.TokensOut,
				SuspiciouslyLowTokenThreshold,
			)
		}
		return validation
	}

	// For implementation tasks without git changes, this is likely a failure
	if isImplementationTask {
		validation.Valid = false
		validation.Warning = fmt.Sprintf(
			"implementation task completed but no files were modified. "+
				"Agent output: %d tokens, tool calls: %d. "+
				"The agent may have described what to do instead of doing it.",
			result.TokensOut,
			len(result.ToolCalls),
		)
		return validation
	}

	// For other tasks (unknown type), use token threshold as fallback
	if result.TokensOut < SuspiciouslyLowTokenThreshold {
		validation.Valid = false
		validation.Warning = fmt.Sprintf(
			"task output suspiciously short: %d tokens (expected >=%d). "+
				"No file changes detected.",
			result.TokensOut,
			SuspiciouslyLowTokenThreshold,
		)
		return validation
	}

	// Unknown task type with reasonable tokens but no git changes - warn but pass
	validation.Warning = fmt.Sprintf(
		"task completed with %d tokens but no file changes detected. Verify output is correct.",
		result.TokensOut,
	)

	return validation
}
