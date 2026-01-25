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
	// Check for cancellation
	if wctx.Control != nil {
		if err := wctx.Control.CheckCancelled(); err != nil {
			return err
		}
		// Wait if paused
		if err := wctx.Control.WaitIfPaused(ctx); err != nil {
			return err
		}
	}

	wctx.Logger.Info("executing task",
		"task_id", task.ID,
		"task_name", task.Name,
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "executor", fmt.Sprintf("Task started: %s", task.Name))
	}

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
	workDir, worktreeCreated := e.setupWorktree(ctx, wctx, task, taskState, useWorktrees)

	// Cleanup worktree when done (if auto_clean is enabled)
	defer e.cleanupWorktree(ctx, wctx, task, worktreeCreated)

	// Get agent
	agentName := task.CLI
	if agentName == "" {
		agentName = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		e.setTaskFailed(wctx, taskState, err)
		taskErr = err
		return err
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		e.setTaskFailed(wctx, taskState, err)
		taskErr = err
		return fmt.Errorf("rate limit: %w", err)
	}

	// Render task prompt
	prompt, err := wctx.Prompts.RenderTaskExecute(TaskExecuteParams{
		Task:    task,
		Context: wctx.GetContextString(),
	})
	if err != nil {
		e.setTaskFailed(wctx, taskState, err)
		taskErr = err
		return err
	}

	// Execute with retry
	var result *core.ExecuteResult
	var retryCount int
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseExecute, task.Model)
	execStartTime := time.Now()

	// Log task execution start with detailed context
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

	// Emit agent started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Executing task: "+task.Name, map[string]interface{}{
			"task_id":         string(task.ID),
			"task_name":       task.Name,
			"phase":           "execute",
			"model":           model,
			"workdir":         workDir,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Execute.Seconds()),
		})
	}

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

	durationMS := time.Since(execStartTime).Milliseconds()

	wctx.Lock()
	taskState.Retries = retryCount
	wctx.Unlock()

	if err != nil {
		// Log detailed error information
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
		taskErr = err
		return err
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
	)

	// Emit completed event
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
			"finish_reason": result.FinishReason,
		})
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
	if costErr := e.checkCostLimits(wctx, task, taskState, result.CostUSD); costErr != nil {
		taskErr = costErr
		return costErr
	}

	// Finalize task (commit, push, PR) if configured
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

	// Save state immediately after each task completion (not waiting for batch)
	// This enables fine-grained recovery if the workflow is interrupted
	if e.stateSaver != nil {
		if saveErr := e.stateSaver.Save(ctx, wctx.State); saveErr != nil {
			wctx.Logger.Warn("failed to save state after task completion",
				"task_id", task.ID,
				"error", saveErr,
			)
		} else {
			wctx.Logger.Debug("state saved after task completion", "task_id", task.ID)
		}
	}

	// Notify UI of state update so task panel refreshes
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
