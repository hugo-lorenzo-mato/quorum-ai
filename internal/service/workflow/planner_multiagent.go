package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// Compile-time assertions for methods reserved for future multi-agent planning.
// These are intentionally unused now but will be wired in when multi-agent mode is enabled.
var (
	_ = (*Planner).runMultiAgentPlanning
	_ = (*Planner).runV1Planning
	_ = (*Planner).runPlanningWithAgent
	_ = (*Planner).consolidatePlans
)

// PlanOutput represents output from a planning agent.
type PlanOutput struct {
	AgentName  string
	Model      string
	RawOutput  string
	TokensIn   int
	TokensOut  int
	DurationMS int64
}

// runMultiAgentPlanning executes the multi-agent planning flow.
// This is called when phases.plan.synthesizer.enabled is true in config.
func (p *Planner) runMultiAgentPlanning(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting multi-agent planning",
		"synthesizer", wctx.Config.PlanSynthesizerAgent,
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", "Multi-agent planning enabled")
	}

	// ========== PHASE 1: V1 Parallel Planning ==========
	wctx.Logger.Info("starting V1 parallel planning")
	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", "V1: Running parallel plan proposals")
	}

	v1Plans, err := p.runV1Planning(ctx, wctx)
	if err != nil {
		return fmt.Errorf("V1 planning: %w", err)
	}

	// ========== PHASE 2: Plan Consolidation ==========
	wctx.Logger.Info("consolidating plans", "count", len(v1Plans))
	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", fmt.Sprintf("Consolidating %d plan proposals", len(v1Plans)))
	}

	consolidatedPlan, err := p.consolidatePlans(ctx, wctx, v1Plans)
	if err != nil {
		return fmt.Errorf("consolidating plans: %w", err)
	}

	// ========== PHASE 3: Parse and Validate ==========
	// Parse consolidated plan into tasks
	tasks, err := p.parsePlan(ctx, wctx, consolidatedPlan.Output)
	if err != nil {
		if wctx.Output != nil {
			wctx.Output.Log("error", "planner", fmt.Sprintf("Failed to parse consolidated plan: %s", err.Error()))
		}
		return fmt.Errorf("parsing consolidated plan: %w", err)
	}

	// Add tasks to state and DAG (same as single-agent flow)
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

	// Write reports
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		// Type assert to concrete DAGState
		if ds, ok := dagState.(*service.DAGState); ok {
			p.writeTaskReports(wctx, tasks, ds)
			p.writeExecutionGraph(wctx, tasks, ds)
		}
		// Write final consolidated plan
		if err := wctx.Report.WriteFinalPlan(consolidatedPlan.Output); err != nil {
			wctx.Logger.Warn("failed to write final plan", "error", err)
		}
	}

	wctx.Logger.Info("multi-agent planning completed",
		"task_count", len(tasks),
		"proposals", len(v1Plans),
	)
	if wctx.Output != nil {
		wctx.Output.Log("success", "planner", fmt.Sprintf("Multi-agent planning completed: %d tasks from %d proposals", len(tasks), len(v1Plans)))
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhasePlan, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return p.stateSaver.Save(ctx, wctx.State)
}

// runV1Planning runs parallel planning with multiple agents.
// Returns at least 2 successful plan proposals.
func (p *Planner) runV1Planning(ctx context.Context, wctx *Context) ([]PlanOutput, error) {
	// Use AvailableForPhase to only get agents enabled for plan phase
	agentNames := wctx.Agents.AvailableForPhaseWithConfig(ctx, "plan", wctx.Config.ProjectAgentPhases)
	if len(agentNames) == 0 {
		return nil, core.ErrValidation(core.CodeNoAgents, "no agents available for plan phase")
	}

	wctx.Logger.Info("running V1 planning",
		"agents", strings.Join(agentNames, ", "),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "planner", fmt.Sprintf("V1 Planning: querying %d agents (%s)", len(agentNames), strings.Join(agentNames, ", ")))
	}

	// Use sync.WaitGroup for parallel execution
	var wg sync.WaitGroup
	var mu sync.Mutex
	plans := make([]PlanOutput, 0, len(agentNames))
	errors := make(map[string]error)

	for _, name := range agentNames {
		name := name // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			plan, err := p.runPlanningWithAgent(ctx, wctx, name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				wctx.Logger.Error("agent planning failed",
					"agent", name,
					"error", err,
				)
				errors[name] = err
			} else {
				plans = append(plans, plan)
			}
		}()
	}

	wg.Wait()

	// Log summary
	wctx.Logger.Info("V1 planning complete",
		"succeeded", len(plans),
		"failed", len(errors),
		"total", len(agentNames),
	)

	// Need at least 2 successful plans for meaningful consolidation
	const minRequired = 2

	if len(plans) < minRequired {
		// Collect error messages
		var errMsgs []string
		for agent, err := range errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", agent, err))
		}
		return nil, fmt.Errorf("insufficient agents succeeded (%d/%d required): %s",
			len(plans), minRequired, strings.Join(errMsgs, "; "))
	}

	// Log which agents failed but we're continuing
	if len(errors) > 0 {
		var failedAgents []string
		for agent := range errors {
			failedAgents = append(failedAgents, agent)
		}
		if wctx.Output != nil {
			wctx.Output.Log("warn", "planner", fmt.Sprintf("Continuing with %d/%d agents (failed: %s)",
				len(plans), len(agentNames), strings.Join(failedAgents, ", ")))
		}
	}

	return plans, nil
}

// runPlanningWithAgent runs planning with a single agent.
func (p *Planner) runPlanningWithAgent(ctx context.Context, wctx *Context, agentName string) (PlanOutput, error) {
	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return PlanOutput{}, fmt.Errorf("getting agent %s: %w", agentName, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return PlanOutput{}, fmt.Errorf("rate limit: %w", err)
	}

	// Resolve model
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhasePlan, "")

	// Get consolidated analysis
	analysis := GetConsolidatedAnalysis(wctx.State)
	if analysis == "" {
		return PlanOutput{}, core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found")
	}

	// Render prompt
	prompt, err := wctx.Prompts.RenderPlanGenerate(PlanParams{
		Prompt:               GetEffectivePrompt(wctx.State),
		ConsolidatedAnalysis: analysis,
		MaxTasks:             10,
	})
	if err != nil {
		return PlanOutput{}, fmt.Errorf("rendering prompt: %w", err)
	}

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Proposing execution plan", map[string]interface{}{
			"phase":           "plan_v1",
			"model":           model,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Plan.Seconds()),
		})
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Plan,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhasePlan,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "plan_v1",
				"model":       model,
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return PlanOutput{}, err
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, "Plan proposal completed", map[string]interface{}{
			"phase":       "plan_v1",
			"model":       result.Model,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": durationMS,
		})
	}

	// Write plan report
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if err := wctx.Report.WritePlan(report.PlanData{
			Agent:      agentName,
			Model:      model,
			Content:    result.Output,
			TokensIn:   result.TokensIn,
			TokensOut:  result.TokensOut,
			DurationMS: durationMS,
		}); err != nil {
			wctx.Logger.Warn("failed to write plan report", "error", err)
		}
	}

	return PlanOutput{
		AgentName:  agentName,
		Model:      model,
		RawOutput:  result.Output,
		TokensIn:   result.TokensIn,
		TokensOut:  result.TokensOut,
		DurationMS: durationMS,
	}, nil
}

// ConsolidatedPlan represents the synthesized plan from multiple proposals.
type ConsolidatedPlan struct {
	Agent      string
	Model      string
	Output     string
	TokensIn   int
	TokensOut  int
	DurationMS int64
}

// consolidatePlans synthesizes multiple plan proposals into a single optimal plan.
func (p *Planner) consolidatePlans(ctx context.Context, wctx *Context, plans []PlanOutput) (*ConsolidatedPlan, error) {
	// Get synthesizer agent
	synthesizerAgent := wctx.Config.PlanSynthesizerAgent
	if synthesizerAgent == "" {
		return nil, fmt.Errorf("phases.plan.synthesizer.agent is not configured. " +
			"Multi-agent planning requires a synthesizer to combine proposals. " +
			"Please set 'phases.plan.synthesizer.agent' in your .quorum/config.yaml file")
	}

	agent, err := wctx.Agents.Get(synthesizerAgent)
	if err != nil {
		wctx.Logger.Error("synthesizer agent not available",
			"agent", synthesizerAgent,
			"error", err,
		)
		return nil, fmt.Errorf("synthesizer agent '%s' not available: %w", synthesizerAgent, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(synthesizerAgent)
	if err := limiter.Acquire(); err != nil {
		return nil, fmt.Errorf("rate limit for synthesizer: %w", err)
	}

	// Resolve model from agent's phase_models.plan or default model
	model := ResolvePhaseModel(wctx.Config, synthesizerAgent, core.PhasePlan, "")

	// Build summaries to avoid context overflow
	const maxCharsPerPlan = 80000
	summarizedPlans := make([]PlanOutput, len(plans))
	for i, plan := range plans {
		summarizedPlans[i] = plan
		if len(plan.RawOutput) > maxCharsPerPlan {
			// Truncate if too large
			summarizedPlans[i].RawOutput = plan.RawOutput[:maxCharsPerPlan] + "\n... [truncated]"
			wctx.Logger.Debug("truncating plan for synthesis",
				"agent", plan.AgentName,
				"original_len", len(plan.RawOutput),
				"truncated_len", maxCharsPerPlan,
			)
		}
	}

	// Render synthesis prompt
	prompt, err := wctx.Prompts.RenderSynthesizePlans(SynthesizePlansParams{
		Prompt:   GetEffectivePrompt(wctx.State),
		Analysis: GetConsolidatedAnalysis(wctx.State),
		Plans:    summarizedPlans,
		MaxTasks: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering synthesis prompt: %w", err)
	}

	wctx.Logger.Info("synthesize plans start",
		"agent", synthesizerAgent,
		"model", model,
		"plans_count", len(plans),
	)

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", synthesizerAgent, "Synthesizing plan proposals", map[string]interface{}{
			"phase":           "synthesize_plans",
			"model":           model,
			"plans_count":     len(plans),
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Plan.Seconds()),
		})
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Plan,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhasePlan,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", synthesizerAgent, err.Error(), map[string]interface{}{
				"phase":       "synthesize_plans",
				"model":       model,
				"plans_count": len(plans),
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return nil, fmt.Errorf("synthesis LLM call failed: %w", err)
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", synthesizerAgent, "Plan synthesis completed", map[string]interface{}{
			"phase":       "synthesize_plans",
			"model":       result.Model,
			"plans_count": len(plans),
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": durationMS,
		})
	}

	wctx.Logger.Info("synthesize plans done",
		"agent", synthesizerAgent,
		"model", model,
	)

	// Write synthesized plan to report
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if err := wctx.Report.WriteConsolidatedPlan(result.Output); err != nil {
			wctx.Logger.Warn("failed to write synthesized plan", "error", err)
		}
	}

	return &ConsolidatedPlan{
		Agent:      synthesizerAgent,
		Model:      result.Model,
		Output:     result.Output,
		TokensIn:   result.TokensIn,
		TokensOut:  result.TokensOut,
		DurationMS: durationMS,
	}, nil
}
