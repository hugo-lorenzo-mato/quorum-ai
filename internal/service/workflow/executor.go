package workflow

import (
	"context"
	"fmt"
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
		g, taskCtx := errgroup.WithContext(ctx)
		for _, task := range ready {
			task := task
			g.Go(func() error {
				return e.executeTask(taskCtx, wctx, task)
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
func (e *Executor) executeTask(ctx context.Context, wctx *Context, task *core.Task) error {
	wctx.Logger.Info("executing task",
		"task_id", task.ID,
		"task_name", task.Name,
	)

	// Update task state
	taskState := wctx.State.Tasks[task.ID]
	if taskState == nil {
		return fmt.Errorf("task state not found: %s", task.ID)
	}

	now := time.Now()
	taskState.Status = core.TaskStatusRunning
	taskState.StartedAt = &now

	// Create task checkpoint
	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, false); err != nil {
		wctx.Logger.Warn("failed to create task checkpoint", "error", err)
	}

	// Skip in dry-run mode
	if wctx.Config.DryRun {
		taskState.Status = core.TaskStatusCompleted
		completedAt := time.Now()
		taskState.CompletedAt = &completedAt
		return nil
	}

	// Get agent
	agentName := task.CLI
	if agentName == "" {
		agentName = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return err
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return fmt.Errorf("rate limit: %w", err)
	}

	// Render task prompt
	prompt, err := wctx.Prompts.RenderTaskExecute(TaskExecuteParams{
		Task:    task,
		Context: BuildContextString(wctx.State),
	})
	if err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
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

	taskState.Retries = retryCount

	if err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return err
	}

	// Update task metrics
	taskState.TokensIn = result.TokensIn
	taskState.TokensOut = result.TokensOut
	taskState.CostUSD = result.CostUSD
	taskState.Status = core.TaskStatusCompleted
	completedAt := time.Now()
	taskState.CompletedAt = &completedAt

	// Update aggregate metrics
	if wctx.State.Metrics != nil {
		wctx.State.Metrics.TotalTokensIn += result.TokensIn
		wctx.State.Metrics.TotalTokensOut += result.TokensOut
		wctx.State.Metrics.TotalCostUSD += result.CostUSD
	}

	if err := wctx.Checkpoint.TaskCheckpoint(wctx.State, task, true); err != nil {
		wctx.Logger.Warn("failed to create task complete checkpoint", "error", err)
	}

	return nil
}
