package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// DAGBuilder builds and validates task dependency graphs.
type DAGBuilder interface {
	AddTask(task *core.Task) error
	AddDependency(from, to core.TaskID) error
	Build() (interface{}, error)
}

// StateSaver persists workflow state.
type StateSaver interface {
	Save(ctx context.Context, state *core.WorkflowState) error
}

// Planner generates and validates execution plans.
type Planner struct {
	dag        DAGBuilder
	stateSaver StateSaver
}

// NewPlanner creates a new planner.
func NewPlanner(dag DAGBuilder, stateSaver StateSaver) *Planner {
	return &Planner{
		dag:        dag,
		stateSaver: stateSaver,
	}
}

// Run executes the plan phase.
func (p *Planner) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting plan phase", "workflow_id", wctx.State.WorkflowID)

	wctx.State.CurrentPhase = core.PhasePlan
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhasePlan, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Get consolidated analysis
	analysis := GetConsolidatedAnalysis(wctx.State)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found")
	}

	// Generate plan
	agent, err := wctx.Agents.Get(wctx.Config.DefaultAgent)
	if err != nil {
		return fmt.Errorf("getting plan agent: %w", err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(wctx.Config.DefaultAgent)
	if err := limiter.Acquire(); err != nil {
		return fmt.Errorf("rate limit for planning: %w", err)
	}

	prompt, err := wctx.Prompts.RenderPlanGenerate(PlanParams{
		Prompt:               wctx.State.Prompt,
		ConsolidatedAnalysis: analysis,
		MaxTasks:             10,
	})
	if err != nil {
		return fmt.Errorf("rendering plan prompt: %w", err)
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Timeout: 5 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return err
	}

	// Parse plan into tasks
	tasks, err := p.parsePlan(wctx, result.Output)
	if err != nil {
		return fmt.Errorf("parsing plan: %w", err)
	}

	// Add tasks to state and DAG
	for _, task := range tasks {
		wctx.State.Tasks[task.ID] = &core.TaskState{
			ID:           task.ID,
			Phase:        task.Phase,
			Name:         task.Name,
			Status:       task.Status,
			CLI:          task.CLI,
			Model:        task.Model,
			Dependencies: task.Dependencies,
		}
		wctx.State.TaskOrder = append(wctx.State.TaskOrder, task.ID)
		_ = p.dag.AddTask(task)
	}

	// Build dependency graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			_ = p.dag.AddDependency(task.ID, dep)
		}
	}

	// Validate DAG
	if _, err := p.dag.Build(); err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	wctx.Logger.Info("plan phase completed",
		"task_count", len(tasks),
	)

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhasePlan, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return p.stateSaver.Save(ctx, wctx.State)
}

// TaskPlanItem represents a task from the plan.
type TaskPlanItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	CLI          string   `json:"cli"`
	Dependencies []string `json:"dependencies"`
}

// parsePlan parses the plan output into tasks.
func (p *Planner) parsePlan(wctx *Context, output string) ([]*core.Task, error) {
	var planItems []TaskPlanItem
	if err := json.Unmarshal([]byte(output), &planItems); err != nil {
		// Try wrapping in tasks array
		var wrapper struct {
			Tasks []TaskPlanItem `json:"tasks"`
		}
		if err := json.Unmarshal([]byte(output), &wrapper); err != nil {
			wctx.Logger.Warn("failed to parse plan, using empty task list", "error", err)
			return []*core.Task{}, nil
		}
		planItems = wrapper.Tasks
	}

	tasks := make([]*core.Task, 0, len(planItems))
	for _, item := range planItems {
		task := &core.Task{
			ID:          core.TaskID(item.ID),
			Name:        item.Name,
			Description: item.Description,
			Phase:       core.PhaseExecute,
			Status:      core.TaskStatusPending,
			CLI:         item.CLI,
		}

		for _, dep := range item.Dependencies {
			task.Dependencies = append(task.Dependencies, core.TaskID(dep))
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}
