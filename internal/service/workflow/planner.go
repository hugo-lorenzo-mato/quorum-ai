package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
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
// Chooses between single-agent and multi-agent planning based on configuration.
func (p *Planner) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting plan phase", "workflow_id", wctx.State.WorkflowID)

	// Check if plan phase is already completed to prevent re-running
	// when resuming a workflow that was interrupted.
	if isPhaseCompleted(wctx.State, core.PhasePlan) {
		wctx.Logger.Info("plan phase already completed, skipping",
			"workflow_id", wctx.State.WorkflowID)
		if wctx.Output != nil {
			wctx.Output.Log("info", "planner", "Plan phase already completed, skipping")
		}
		return nil
	}

	wctx.State.CurrentPhase = core.PhasePlan

	// Choose between single-agent and multi-agent planning
	if wctx.Config.PlanSynthesizerEnabled {
		wctx.Logger.Info("using multi-agent planning")
		return p.runMultiAgentPlanning(ctx, wctx)
	}

	wctx.Logger.Info("using single-agent planning")
	return p.runSingleAgentPlanning(ctx, wctx)
}

// runSingleAgentPlanning executes the traditional single-agent planning flow.
func (p *Planner) runSingleAgentPlanning(ctx context.Context, wctx *Context) error {
	wctx.State.CurrentPhase = core.PhasePlan
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhasePlan)
		wctx.Output.Log("info", "planner", "Starting task planning phase")
	}
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
		Prompt:               GetEffectivePrompt(wctx.State),
		ConsolidatedAnalysis: analysis,
		MaxTasks:             10,
	})
	if err != nil {
		return fmt.Errorf("rendering plan prompt: %w", err)
	}

	// Emit started event
	agentName := wctx.Config.DefaultAgent
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhasePlan, "")
	startTime := time.Now()

	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Generating execution plan", map[string]interface{}{
			"phase":           "plan",
			"model":           model,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Plan.Seconds()),
		})
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Plan,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhasePlan,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "plan",
				"model":       model,
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return err
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, "Plan generated", map[string]interface{}{
			"phase":       "plan",
			"model":       result.Model,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"cost_usd":    result.CostUSD,
			"duration_ms": durationMS,
		})
	}

	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", fmt.Sprintf("Generating execution plan with %s", agentName))
	}

	// Write plan report
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if err := wctx.Report.WritePlan(report.PlanData{
			Agent:      agentName,
			Model:      model,
			Content:    result.Output,
			TokensIn:   result.TokensIn,
			TokensOut:  result.TokensOut,
			CostUSD:    result.CostUSD,
			DurationMS: durationMS,
		}); err != nil {
			wctx.Logger.Warn("failed to write plan report", "error", err)
		}
	}

	// Parse plan into tasks
	tasks, err := p.parsePlan(ctx, wctx, result.Output)
	if err != nil {
		if wctx.Output != nil {
			wctx.Output.Log("error", "planner", fmt.Sprintf("Failed to parse plan: %s", err.Error()))
		}
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

	// Notify output that tasks have been created
	if wctx.Output != nil {
		wctx.Output.WorkflowStateUpdated(wctx.State)
	}

	// Build dependency graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			_ = p.dag.AddDependency(task.ID, dep)
		}
	}

	// Validate DAG and get execution levels
	dagState, err := p.dag.Build()
	if err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	// Write individual task plans and execution graph
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		// Type assert to concrete DAGState
		if ds, ok := dagState.(*service.DAGState); ok {
			p.writeTaskReports(wctx, tasks, ds)
			p.writeExecutionGraph(wctx, tasks, ds)
		}
		// Write final consolidated plan
		if err := wctx.Report.WriteFinalPlan(result.Output); err != nil {
			wctx.Logger.Warn("failed to write final plan", "error", err)
		}
	}

	wctx.Logger.Info("plan phase completed",
		"task_count", len(tasks),
	)
	if wctx.Output != nil {
		wctx.Output.Log("success", "planner", fmt.Sprintf("Planning completed: %d tasks created", len(tasks)))
	}

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
func (p *Planner) parsePlan(ctx context.Context, wctx *Context, output string) ([]*core.Task, error) {
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
		cli = resolveTaskAgent(ctx, wctx, cli)
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

func resolveTaskAgent(ctx context.Context, wctx *Context, candidate string) string {
	cleaned := strings.TrimSpace(candidate)
	if cleaned == "" {
		return wctx.Config.DefaultAgent
	}

	if isShellLikeAgent(cleaned) {
		wctx.Logger.Warn("plan used shell name as agent, defaulting",
			"agent", cleaned,
			"default", wctx.Config.DefaultAgent,
		)
		return wctx.Config.DefaultAgent
	}

	// Use AvailableForPhase to only accept agents that are:
	// 1. enabled: true in config
	// 2. Pass Ping (CLI is installed)
	// 3. Have phases.execute != false (or no phases restriction)
	availableForExecute := wctx.Agents.AvailableForPhase(ctx, "execute")
	for _, name := range availableForExecute {
		if strings.EqualFold(name, cleaned) {
			return name
		}
	}

	// Agent not available for execute phase - check why for better logging
	allAgents := wctx.Agents.List()
	for _, name := range allAgents {
		if strings.EqualFold(name, cleaned) {
			// Agent exists but not available for execute
			wctx.Logger.Warn("plan requested agent not available for execute phase, defaulting",
				"agent", cleaned,
				"default", wctx.Config.DefaultAgent,
				"hint", "check if agent has enabled: true and phases.execute is not false",
			)
			return wctx.Config.DefaultAgent
		}
	}

	wctx.Logger.Warn("plan requested unknown agent, defaulting",
		"agent", cleaned,
		"default", wctx.Config.DefaultAgent,
	)
	return wctx.Config.DefaultAgent
}

func isShellLikeAgent(candidate string) bool {
	switch strings.ToLower(strings.TrimSpace(candidate)) {
	case "bash", "sh", "zsh", "fish", "powershell", "pwsh", "terminal", "shell", "command", "cli", "default", "auto":
		return true
	default:
		return false
	}
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
		if candidate := rawToText(raw); candidate != "" && candidate != cleaned {
			return parsePlanItems(candidate)
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

func rawToText(raw json.RawMessage) string {
	var direct string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return strings.TrimSpace(direct)
	}

	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var collected []string
		for _, part := range parts {
			if strings.TrimSpace(part.Text) != "" {
				collected = append(collected, part.Text)
			}
		}
		if len(collected) > 0 {
			return strings.TrimSpace(strings.Join(collected, "\n"))
		}
	}

	var obj struct {
		Text    string `json:"text"`
		Content string `json:"content"`
		Parts   []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if strings.TrimSpace(obj.Text) != "" {
			return strings.TrimSpace(obj.Text)
		}
		if strings.TrimSpace(obj.Content) != "" {
			return strings.TrimSpace(obj.Content)
		}
		if len(obj.Parts) > 0 {
			var collected []string
			for _, part := range obj.Parts {
				if strings.TrimSpace(part.Text) != "" {
					collected = append(collected, part.Text)
				}
			}
			if len(collected) > 0 {
				return strings.TrimSpace(strings.Join(collected, "\n"))
			}
		}
	}

	return ""
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

// writeTaskReports writes individual task plan files
func (p *Planner) writeTaskReports(wctx *Context, tasks []*core.Task, dagState *service.DAGState) {
	// Build a map of task to batch number and parallel tasks
	taskToBatch := make(map[core.TaskID]int)
	batchTasks := make(map[int][]core.TaskID)

	// Assign batch based on DAGState levels
	if dagState != nil && dagState.Levels != nil {
		for batchNum, level := range dagState.Levels {
			for _, taskID := range level {
				taskToBatch[taskID] = batchNum + 1 // 1-indexed for humans
				batchTasks[batchNum+1] = append(batchTasks[batchNum+1], taskID)
			}
		}
	} else {
		// Fallback: assign sequential batches
		for i, task := range tasks {
			taskToBatch[task.ID] = i + 1
			batchTasks[i+1] = []core.TaskID{task.ID}
		}
	}

	// Write each task plan
	for _, task := range tasks {
		batchNum := taskToBatch[task.ID]

		// Find parallel tasks (same batch, different task)
		var parallelWith []string
		for _, tid := range batchTasks[batchNum] {
			if tid != task.ID {
				parallelWith = append(parallelWith, string(tid))
			}
		}

		// Resolve model for this task
		model := ResolvePhaseModel(wctx.Config, task.CLI, core.PhaseExecute, "")

		// Convert dependencies to strings
		deps := make([]string, len(task.Dependencies))
		for i, dep := range task.Dependencies {
			deps[i] = string(dep)
		}

		err := wctx.Report.WriteTaskPlan(report.TaskPlanData{
			TaskID:         string(task.ID),
			Name:           task.Name,
			Description:    task.Description,
			CLI:            task.CLI,
			PlannedModel:   model,
			ExecutionBatch: batchNum,
			ParallelWith:   parallelWith,
			Dependencies:   deps,
			CanParallelize: len(task.Dependencies) == 0 || len(parallelWith) > 0,
		})

		if err != nil {
			wctx.Logger.Warn("failed to write task plan",
				"task_id", task.ID,
				"error", err,
			)
		}
	}

	wctx.Logger.Info("task plans written",
		"count", len(tasks),
	)
}

// writeExecutionGraph writes the execution graph visualization
func (p *Planner) writeExecutionGraph(wctx *Context, tasks []*core.Task, dagState *service.DAGState) {
	var batches []report.ExecutionBatch

	// Get levels from DAG if available
	if dagState != nil && dagState.Levels != nil {
		// Build task map for quick lookup
		taskMap := make(map[core.TaskID]*core.Task)
		for _, task := range tasks {
			taskMap[task.ID] = task
		}

		// Convert levels to batches
		for batchNum, level := range dagState.Levels {
			batch := report.ExecutionBatch{
				BatchNumber: batchNum + 1, // 1-indexed for humans
				Tasks:       make([]report.ExecutionTask, 0, len(level)),
			}

			for _, taskID := range level {
				task, ok := taskMap[taskID]
				if !ok {
					continue
				}

				// Resolve model for this task
				model := ResolvePhaseModel(wctx.Config, task.CLI, core.PhaseExecute, "")

				// Convert dependencies to strings
				deps := make([]string, len(task.Dependencies))
				for i, dep := range task.Dependencies {
					deps[i] = string(dep)
				}

				batch.Tasks = append(batch.Tasks, report.ExecutionTask{
					TaskID:       string(task.ID),
					Name:         task.Name,
					CLI:          task.CLI,
					PlannedModel: model,
					Dependencies: deps,
				})
			}

			batches = append(batches, batch)
		}
	} else {
		// Fallback: create sequential batches
		for i, task := range tasks {
			model := ResolvePhaseModel(wctx.Config, task.CLI, core.PhaseExecute, "")

			deps := make([]string, len(task.Dependencies))
			for j, dep := range task.Dependencies {
				deps[j] = string(dep)
			}

			batches = append(batches, report.ExecutionBatch{
				BatchNumber: i + 1,
				Tasks: []report.ExecutionTask{{
					TaskID:       string(task.ID),
					Name:         task.Name,
					CLI:          task.CLI,
					PlannedModel: model,
					Dependencies: deps,
				}},
			})
		}
	}

	err := wctx.Report.WriteExecutionGraph(report.ExecutionGraphData{
		Batches:      batches,
		TotalTasks:   len(tasks),
		TotalBatches: len(batches),
	})

	if err != nil {
		wctx.Logger.Warn("failed to write execution graph", "error", err)
	} else {
		wctx.Logger.Info("execution graph written",
			"batches", len(batches),
			"tasks", len(tasks),
		)
	}
}
