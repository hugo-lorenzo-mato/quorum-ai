package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
			Model:   ResolvePhaseModel(wctx.Config, wctx.Config.DefaultAgent, core.PhasePlan, ""),
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
	Agent        string   `json:"agent"`
	Dependencies []string `json:"dependencies"`
}

// parsePlan parses the plan output into tasks.
func (p *Planner) parsePlan(wctx *Context, output string) ([]*core.Task, error) {
	planItems, err := parsePlanItems(output)
	if err != nil {
		return nil, err
	}
	if len(planItems) == 0 {
		return nil, fmt.Errorf("plan produced no tasks")
	}

	tasks := make([]*core.Task, 0, len(planItems))
	for _, item := range planItems {
		cli := item.CLI
		if cli == "" {
			cli = item.Agent
		}
		task := &core.Task{
			ID:          core.TaskID(item.ID),
			Name:        item.Name,
			Description: item.Description,
			Phase:       core.PhaseExecute,
			Status:      core.TaskStatusPending,
			CLI:         cli,
		}

		for _, dep := range item.Dependencies {
			task.Dependencies = append(task.Dependencies, core.TaskID(dep))
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func parsePlanItems(output string) ([]TaskPlanItem, error) {
	cleaned := strings.TrimSpace(output)
	if cleaned == "" {
		return nil, fmt.Errorf("plan output empty")
	}

	var planItems []TaskPlanItem
	if err := json.Unmarshal([]byte(cleaned), &planItems); err == nil {
		return planItems, nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cleaned), &wrapper); err != nil {
		if extracted := extractJSON(cleaned); extracted != "" && extracted != cleaned {
			return parsePlanItems(extracted)
		}
		return nil, fmt.Errorf("failed to parse plan output as JSON: %w", err)
	}

	if rawTasks, ok := wrapper["tasks"]; ok {
		if err := json.Unmarshal(rawTasks, &planItems); err != nil {
			return nil, fmt.Errorf("failed to parse tasks field: %w", err)
		}
		return planItems, nil
	}

	for _, key := range []string{"result", "content", "text", "output"} {
		raw, ok := wrapper[key]
		if !ok {
			continue
		}
		var candidate string
		if err := json.Unmarshal(raw, &candidate); err == nil {
			candidate = strings.TrimSpace(candidate)
			if candidate != "" && candidate != cleaned {
				return parsePlanItems(candidate)
			}
		}
	}

	var gemini struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(cleaned), &gemini); err == nil && len(gemini.Candidates) > 0 {
		var parts []string
		for _, part := range gemini.Candidates[0].Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
		if len(parts) > 0 {
			return parsePlanItems(strings.Join(parts, "\n"))
		}
	}

	return nil, fmt.Errorf("plan output missing tasks field")
}

func extractJSON(output string) string {
	start := strings.IndexAny(output, "{[")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	openChar := output[start]
	closeChar := byte('}')
	if openChar == '[' {
		closeChar = ']'
	}

	for i := start; i < len(output); i++ {
		c := output[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == openChar {
			depth++
		} else if c == closeChar {
			depth--
			if depth == 0 {
				return output[start : i+1]
			}
		}
	}

	return ""
}
