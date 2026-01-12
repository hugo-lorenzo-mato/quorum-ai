package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"golang.org/x/sync/errgroup"
)

// idCounter provides additional uniqueness for workflow IDs
var idCounter uint64

// WorkflowRunner orchestrates the complete workflow execution.
type WorkflowRunner struct {
	config     *WorkflowRunnerConfig
	state      core.StateManager
	agents     core.AgentRegistry
	dag        *DAGBuilder
	consensus  *ConsensusChecker
	prompts    *PromptRenderer
	checkpoint *CheckpointManager
	retry      *RetryPolicy
	rateLimits *RateLimiterRegistry
	logger     *logging.Logger
}

// WorkflowRunnerConfig holds workflow runner configuration.
type WorkflowRunnerConfig struct {
	Timeout      time.Duration
	MaxRetries   int
	DryRun       bool
	Sandbox      bool
	DenyTools    []string
	DefaultAgent string
	V3Agent      string
}

// DefaultWorkflowRunnerConfig returns default configuration.
func DefaultWorkflowRunnerConfig() *WorkflowRunnerConfig {
	return &WorkflowRunnerConfig{
		Timeout:      time.Hour,
		MaxRetries:   3,
		DryRun:       false,
		Sandbox:      true,
		DefaultAgent: "claude",
		V3Agent:      "claude",
	}
}

// NewWorkflowRunner creates a new workflow runner.
func NewWorkflowRunner(
	config *WorkflowRunnerConfig,
	state core.StateManager,
	agents core.AgentRegistry,
	consensus *ConsensusChecker,
	prompts *PromptRenderer,
	logger *logging.Logger,
) *WorkflowRunner {
	if config == nil {
		config = DefaultWorkflowRunnerConfig()
	}
	return &WorkflowRunner{
		config:     config,
		state:      state,
		agents:     agents,
		dag:        NewDAGBuilder(),
		consensus:  consensus,
		prompts:    prompts,
		checkpoint: NewCheckpointManager(state, logger),
		retry:      DefaultRetryPolicy(),
		rateLimits: NewRateLimiterRegistry(),
		logger:     logger,
	}
}

// Run executes a complete workflow from a user prompt.
func (w *WorkflowRunner) Run(ctx context.Context, prompt string) error {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := w.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer w.state.ReleaseLock(ctx)

	// Initialize state
	state := &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   core.WorkflowID(generateWorkflowID()),
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       prompt,
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    make([]core.TaskID, 0),
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	w.logger.Info("starting workflow",
		"workflow_id", state.WorkflowID,
		"prompt_length", len(prompt),
	)

	// Save initial state
	if err := w.state.Save(ctx, state); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Run phases
	if err := w.runAnalyzePhase(ctx, state); err != nil {
		return w.handleError(ctx, state, err)
	}

	if err := w.runPlanPhase(ctx, state); err != nil {
		return w.handleError(ctx, state, err)
	}

	if err := w.runExecutePhase(ctx, state); err != nil {
		return w.handleError(ctx, state, err)
	}

	// Mark completed
	state.Status = core.WorkflowStatusCompleted
	state.UpdatedAt = time.Now()

	w.logger.Info("workflow completed",
		"workflow_id", state.WorkflowID,
		"total_tasks", len(state.Tasks),
	)

	return w.state.Save(ctx, state)
}

// Resume continues a workflow from the last checkpoint.
func (w *WorkflowRunner) Resume(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()

	if err := w.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer w.state.ReleaseLock(ctx)

	// Load existing state
	state, err := w.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if state == nil {
		return core.ErrState("NO_STATE", "no workflow state found to resume")
	}

	// Get resume point
	resumePoint, err := w.checkpoint.GetResumePoint(state)
	if err != nil {
		return fmt.Errorf("determining resume point: %w", err)
	}

	w.logger.Info("resuming workflow",
		"workflow_id", state.WorkflowID,
		"phase", resumePoint.Phase,
		"task_id", resumePoint.TaskID,
		"from_start", resumePoint.FromStart,
	)

	// Rebuild DAG from existing tasks
	if err := w.rebuildDAG(state); err != nil {
		return fmt.Errorf("rebuilding DAG: %w", err)
	}

	// Resume from appropriate phase
	switch resumePoint.Phase {
	case core.PhaseAnalyze:
		if err := w.runAnalyzePhase(ctx, state); err != nil {
			return w.handleError(ctx, state, err)
		}
		fallthrough
	case core.PhasePlan:
		if err := w.runPlanPhase(ctx, state); err != nil {
			return w.handleError(ctx, state, err)
		}
		fallthrough
	case core.PhaseExecute:
		if err := w.runExecutePhase(ctx, state); err != nil {
			return w.handleError(ctx, state, err)
		}
	}

	state.Status = core.WorkflowStatusCompleted
	state.UpdatedAt = time.Now()

	w.logger.Info("workflow resumed and completed",
		"workflow_id", state.WorkflowID,
	)

	return w.state.Save(ctx, state)
}

// runAnalyzePhase executes the analysis phase with V1/V2/V3 protocol.
func (w *WorkflowRunner) runAnalyzePhase(ctx context.Context, state *core.WorkflowState) error {
	w.logger.Info("starting analyze phase", "workflow_id", state.WorkflowID)

	state.CurrentPhase = core.PhaseAnalyze
	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, false); err != nil {
		w.logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// V1: Initial analysis from multiple agents
	v1Outputs, err := w.runV1Analysis(ctx, state)
	if err != nil {
		return fmt.Errorf("V1 analysis: %w", err)
	}

	// Check consensus
	consensusResult := w.consensus.Evaluate(v1Outputs)
	if err := w.checkpoint.ConsensusCheckpoint(ctx, state, consensusResult); err != nil {
		w.logger.Warn("failed to create consensus checkpoint", "error", err)
	}

	w.logger.Info("V1 consensus evaluated",
		"score", consensusResult.Score,
		"threshold", w.consensus.Threshold,
		"needs_v3", consensusResult.NeedsV3,
	)

	// V2: If low consensus, run critique
	if consensusResult.NeedsV3 {
		v2Outputs, err := w.runV2Critique(ctx, state, v1Outputs)
		if err != nil {
			return fmt.Errorf("V2 critique: %w", err)
		}

		// Re-evaluate consensus
		allOutputs := append(v1Outputs, v2Outputs...)
		consensusResult = w.consensus.Evaluate(allOutputs)
		if err := w.checkpoint.ConsensusCheckpoint(ctx, state, consensusResult); err != nil {
			w.logger.Warn("failed to create V2 consensus checkpoint", "error", err)
		}

		// V3: If still low, run reconciliation
		if consensusResult.NeedsV3 || consensusResult.NeedsHumanReview {
			if err := w.runV3Reconciliation(ctx, state, v1Outputs, v2Outputs, consensusResult); err != nil {
				return fmt.Errorf("V3 reconciliation: %w", err)
			}
		}
	}

	// Consolidate analysis
	if err := w.consolidateAnalysis(ctx, state, v1Outputs); err != nil {
		return fmt.Errorf("consolidating analysis: %w", err)
	}

	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, true); err != nil {
		w.logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// runV1Analysis runs initial analysis with multiple agents in parallel.
func (w *WorkflowRunner) runV1Analysis(ctx context.Context, state *core.WorkflowState) ([]AnalysisOutput, error) {
	agentNames := w.agents.List()
	if len(agentNames) == 0 {
		return nil, core.ErrValidation("NO_AGENTS", "no agents available")
	}

	// Limit to 2 agents for V1
	if len(agentNames) > 2 {
		agentNames = agentNames[:2]
	}

	w.logger.Info("running V1 analysis",
		"agents", strings.Join(agentNames, ", "),
	)

	var mu sync.Mutex
	outputs := make([]AnalysisOutput, 0, len(agentNames))

	g, ctx := errgroup.WithContext(ctx)
	for _, name := range agentNames {
		name := name // capture
		g.Go(func() error {
			output, err := w.runAnalysisWithAgent(ctx, state, name)
			if err != nil {
				w.logger.Error("agent analysis failed",
					"agent", name,
					"error", err,
				)
				return err
			}
			mu.Lock()
			outputs = append(outputs, output)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return outputs, nil
}

// runAnalysisWithAgent runs analysis with a specific agent.
func (w *WorkflowRunner) runAnalysisWithAgent(ctx context.Context, state *core.WorkflowState, agentName string) (AnalysisOutput, error) {
	agent, err := w.agents.Get(agentName)
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("getting agent %s: %w", agentName, err)
	}

	// Acquire rate limit
	limiter := w.rateLimits.Get(agentName)
	if err := limiter.Acquire(ctx); err != nil {
		return AnalysisOutput{}, fmt.Errorf("rate limit: %w", err)
	}

	// Render prompt
	prompt, err := w.prompts.RenderAnalyzeV1(AnalyzeV1Params{
		Prompt:  state.Prompt,
		Context: w.buildContext(state),
	})
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("rendering prompt: %w", err)
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Timeout: 5 * time.Minute,
			Sandbox: w.config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return AnalysisOutput{}, err
	}

	// Parse output
	output := w.parseAnalysisOutput(agentName, result)
	return output, nil
}

// runV2Critique runs critical review of V1 outputs.
func (w *WorkflowRunner) runV2Critique(ctx context.Context, state *core.WorkflowState, v1Outputs []AnalysisOutput) ([]AnalysisOutput, error) {
	w.logger.Info("starting V2 critique phase")

	outputs := make([]AnalysisOutput, 0)

	// Each agent critiques the other's output
	for i, v1 := range v1Outputs {
		critiqueAgent := w.selectCritiqueAgent(v1.AgentName)
		agent, err := w.agents.Get(critiqueAgent)
		if err != nil {
			w.logger.Warn("critique agent not available",
				"agent", critiqueAgent,
				"error", err,
			)
			continue
		}

		// Acquire rate limit
		limiter := w.rateLimits.Get(critiqueAgent)
		if err := limiter.Acquire(ctx); err != nil {
			w.logger.Warn("rate limit exceeded for critique", "agent", critiqueAgent)
			continue
		}

		prompt, err := w.prompts.RenderAnalyzeV2(AnalyzeV2Params{
			Prompt:     state.Prompt,
			V1Analysis: v1.RawOutput,
			AgentName:  v1.AgentName,
		})
		if err != nil {
			w.logger.Warn("failed to render V2 prompt", "error", err)
			continue
		}

		var result *core.ExecuteResult
		err = w.retry.Execute(ctx, func(ctx context.Context) error {
			var execErr error
			result, execErr = agent.Execute(ctx, core.ExecuteOptions{
				Prompt:  prompt,
				Format:  core.OutputFormatJSON,
				Timeout: 5 * time.Minute,
				Sandbox: w.config.Sandbox,
			})
			return execErr
		})

		if err == nil {
			output := w.parseAnalysisOutput(fmt.Sprintf("%s-critique-%d", critiqueAgent, i), result)
			outputs = append(outputs, output)
		} else {
			w.logger.Warn("V2 critique failed",
				"agent", critiqueAgent,
				"error", err,
			)
		}
	}

	return outputs, nil
}

// runV3Reconciliation runs synthesis of conflicting analyses.
func (w *WorkflowRunner) runV3Reconciliation(ctx context.Context, state *core.WorkflowState, v1, v2 []AnalysisOutput, consensus ConsensusResult) error {
	w.logger.Info("starting V3 reconciliation phase",
		"divergences", len(consensus.Divergences),
	)

	// Use V3 agent (typically Claude)
	v3AgentName := w.config.V3Agent
	if v3AgentName == "" {
		v3AgentName = "claude"
	}

	agent, err := w.agents.Get(v3AgentName)
	if err != nil {
		return fmt.Errorf("getting V3 agent: %w", err)
	}

	// Acquire rate limit
	limiter := w.rateLimits.Get(v3AgentName)
	if err := limiter.Acquire(ctx); err != nil {
		return fmt.Errorf("rate limit for V3: %w", err)
	}

	// Combine all outputs
	var v1Combined, v2Combined strings.Builder
	for _, o := range v1 {
		v1Combined.WriteString(fmt.Sprintf("### %s\n%s\n\n", o.AgentName, o.RawOutput))
	}
	for _, o := range v2 {
		v2Combined.WriteString(fmt.Sprintf("### %s\n%s\n\n", o.AgentName, o.RawOutput))
	}

	prompt, err := w.prompts.RenderAnalyzeV3(AnalyzeV3Params{
		Prompt:      state.Prompt,
		V1Analysis:  v1Combined.String(),
		V2Analysis:  v2Combined.String(),
		Divergences: consensus.Divergences,
	})
	if err != nil {
		return fmt.Errorf("rendering V3 prompt: %w", err)
	}

	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Timeout: 10 * time.Minute,
			Sandbox: w.config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return err
	}

	// Store V3 result in state metadata
	if state.Metrics == nil {
		state.Metrics = &core.StateMetrics{}
	}

	// Create a checkpoint with the V3 reconciliation result
	return w.checkpoint.CreateCheckpoint(ctx, state, CheckpointType("v3_reconciliation"), map[string]interface{}{
		"output":     result.Output,
		"tokens_in":  result.TokensIn,
		"tokens_out": result.TokensOut,
		"cost_usd":   result.CostUSD,
	})
}

// runPlanPhase generates execution plan from analysis.
func (w *WorkflowRunner) runPlanPhase(ctx context.Context, state *core.WorkflowState) error {
	w.logger.Info("starting plan phase", "workflow_id", state.WorkflowID)

	state.CurrentPhase = core.PhasePlan
	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhasePlan, false); err != nil {
		w.logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Get consolidated analysis from the most recent checkpoint
	analysis := w.getConsolidatedAnalysis(state)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found")
	}

	// Generate plan
	agent, err := w.agents.Get(w.config.DefaultAgent)
	if err != nil {
		return fmt.Errorf("getting plan agent: %w", err)
	}

	// Acquire rate limit
	limiter := w.rateLimits.Get(w.config.DefaultAgent)
	if err := limiter.Acquire(ctx); err != nil {
		return fmt.Errorf("rate limit for planning: %w", err)
	}

	prompt, err := w.prompts.RenderPlanGenerate(PlanParams{
		Prompt:               state.Prompt,
		ConsolidatedAnalysis: analysis,
		MaxTasks:             10,
	})
	if err != nil {
		return fmt.Errorf("rendering plan prompt: %w", err)
	}

	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Timeout: 5 * time.Minute,
			Sandbox: w.config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return err
	}

	// Parse plan into tasks
	tasks, err := w.parsePlan(result.Output)
	if err != nil {
		return fmt.Errorf("parsing plan: %w", err)
	}

	// Add tasks to state and DAG
	for _, task := range tasks {
		state.Tasks[task.ID] = &core.TaskState{
			ID:           task.ID,
			Phase:        task.Phase,
			Name:         task.Name,
			Status:       task.Status,
			CLI:          task.CLI,
			Model:        task.Model,
			Dependencies: task.Dependencies,
		}
		state.TaskOrder = append(state.TaskOrder, task.ID)
		w.dag.AddTask(task)
	}

	// Build dependency graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			w.dag.AddDependency(task.ID, dep)
		}
	}

	// Validate DAG
	if _, err := w.dag.Build(); err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	w.logger.Info("plan phase completed",
		"task_count", len(tasks),
	)

	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhasePlan, true); err != nil {
		w.logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return w.state.Save(ctx, state)
}

// runExecutePhase executes tasks according to the DAG.
func (w *WorkflowRunner) runExecutePhase(ctx context.Context, state *core.WorkflowState) error {
	w.logger.Info("starting execute phase",
		"workflow_id", state.WorkflowID,
		"tasks", len(state.Tasks),
	)

	state.CurrentPhase = core.PhaseExecute
	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhaseExecute, false); err != nil {
		w.logger.Warn("failed to create phase checkpoint", "error", err)
	}

	completed := make(map[core.TaskID]bool)

	// Find already completed tasks
	for id, task := range state.Tasks {
		if task.Status == core.TaskStatusCompleted {
			completed[id] = true
		}
	}

	// Execute remaining tasks
	for len(completed) < len(state.Tasks) {
		ready := w.dag.GetReadyTasks(completed)
		if len(ready) == 0 {
			// Check for stuck state
			return core.ErrState("EXECUTION_STUCK", "no ready tasks but not all completed")
		}

		w.logger.Info("executing task batch",
			"ready_count", len(ready),
			"completed_count", len(completed),
			"total_count", len(state.Tasks),
		)

		// Execute ready tasks in parallel
		g, taskCtx := errgroup.WithContext(ctx)
		for _, task := range ready {
			task := task
			g.Go(func() error {
				return w.executeTask(taskCtx, state, task)
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}

		// Update completed set
		for _, task := range ready {
			if state.Tasks[task.ID].Status == core.TaskStatusCompleted {
				completed[task.ID] = true
			}
		}

		// Save state after each batch
		if err := w.state.Save(ctx, state); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}

	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhaseExecute, true); err != nil {
		w.logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// executeTask executes a single task.
func (w *WorkflowRunner) executeTask(ctx context.Context, state *core.WorkflowState, task *core.Task) error {
	w.logger.Info("executing task",
		"task_id", task.ID,
		"task_name", task.Name,
	)

	// Update task state
	taskState := state.Tasks[task.ID]
	if taskState == nil {
		return fmt.Errorf("task state not found: %s", task.ID)
	}

	now := time.Now()
	taskState.Status = core.TaskStatusRunning
	taskState.StartedAt = &now

	// Create task checkpoint
	if err := w.checkpoint.TaskCheckpoint(ctx, state, task, false); err != nil {
		w.logger.Warn("failed to create task checkpoint", "error", err)
	}

	// Skip in dry-run mode
	if w.config.DryRun {
		taskState.Status = core.TaskStatusCompleted
		completedAt := time.Now()
		taskState.CompletedAt = &completedAt
		return nil
	}

	// Get agent
	agentName := task.CLI
	if agentName == "" {
		agentName = w.config.DefaultAgent
	}

	agent, err := w.agents.Get(agentName)
	if err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return err
	}

	// Acquire rate limit
	limiter := w.rateLimits.Get(agentName)
	if err := limiter.Acquire(ctx); err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return fmt.Errorf("rate limit: %w", err)
	}

	// Render task prompt
	prompt, err := w.prompts.RenderTaskExecute(TaskExecuteParams{
		Task:    task,
		Context: w.buildContext(state),
	})
	if err != nil {
		taskState.Status = core.TaskStatusFailed
		taskState.Error = err.Error()
		return err
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = w.retry.ExecuteWithNotify(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:      prompt,
			Format:      core.OutputFormatText,
			Timeout:     10 * time.Minute,
			Sandbox:     w.config.Sandbox,
			DeniedTools: w.config.DenyTools,
		})
		return execErr
	}, func(attempt int, err error, delay time.Duration) {
		w.logger.Warn("task retry",
			"task_id", task.ID,
			"attempt", attempt,
			"error", err,
			"delay", delay,
		)
		taskState.Retries = attempt
	})

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
	if state.Metrics != nil {
		state.Metrics.TotalTokensIn += result.TokensIn
		state.Metrics.TotalTokensOut += result.TokensOut
		state.Metrics.TotalCostUSD += result.CostUSD
	}

	if err := w.checkpoint.TaskCheckpoint(ctx, state, task, true); err != nil {
		w.logger.Warn("failed to create task complete checkpoint", "error", err)
	}

	return nil
}

// Helper methods

func (w *WorkflowRunner) handleError(ctx context.Context, state *core.WorkflowState, err error) error {
	w.logger.Error("workflow error",
		"workflow_id", state.WorkflowID,
		"phase", state.CurrentPhase,
		"error", err,
	)

	state.Status = core.WorkflowStatusFailed
	state.UpdatedAt = time.Now()
	if err := w.checkpoint.ErrorCheckpoint(ctx, state, err); err != nil {
		w.logger.Warn("failed to create error checkpoint", "checkpoint_error", err)
	}
	w.state.Save(ctx, state)

	return err
}

func (w *WorkflowRunner) buildContext(state *core.WorkflowState) string {
	// Build context from state
	var ctx strings.Builder
	ctx.WriteString(fmt.Sprintf("Workflow: %s\n", state.WorkflowID))
	ctx.WriteString(fmt.Sprintf("Phase: %s\n", state.CurrentPhase))

	// Add completed task summaries
	for _, id := range state.TaskOrder {
		task := state.Tasks[id]
		if task != nil && task.Status == core.TaskStatusCompleted {
			ctx.WriteString(fmt.Sprintf("- Completed: %s\n", task.Name))
		}
	}

	return ctx.String()
}

func (w *WorkflowRunner) selectCritiqueAgent(original string) string {
	agents := w.agents.List()
	for _, a := range agents {
		if a != original {
			return a
		}
	}
	return original
}

func (w *WorkflowRunner) parseAnalysisOutput(agentName string, result *core.ExecuteResult) AnalysisOutput {
	output := AnalysisOutput{
		AgentName: agentName,
		RawOutput: result.Output,
	}

	// Try to parse JSON output
	var parsed struct {
		Claims          []string `json:"claims"`
		Risks           []string `json:"risks"`
		Recommendations []string `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(result.Output), &parsed); err == nil {
		output.Claims = parsed.Claims
		output.Risks = parsed.Risks
		output.Recommendations = parsed.Recommendations
	}

	return output
}

func (w *WorkflowRunner) consolidateAnalysis(ctx context.Context, state *core.WorkflowState, outputs []AnalysisOutput) error {
	var consolidated strings.Builder

	for _, output := range outputs {
		consolidated.WriteString(fmt.Sprintf("## Analysis from %s\n", output.AgentName))
		consolidated.WriteString(output.RawOutput)
		consolidated.WriteString("\n\n")
	}

	// Store as checkpoint metadata
	return w.checkpoint.CreateCheckpoint(ctx, state, CheckpointType("consolidated_analysis"), map[string]interface{}{
		"content":     consolidated.String(),
		"agent_count": len(outputs),
	})
}

func (w *WorkflowRunner) getConsolidatedAnalysis(state *core.WorkflowState) string {
	// Look for consolidated analysis checkpoint
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := state.Checkpoints[i]
		if cp.Type == "consolidated_analysis" && len(cp.Data) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(cp.Data, &metadata); err == nil {
				if content, ok := metadata["content"].(string); ok {
					return content
				}
			}
		}
	}
	return ""
}

// TaskPlanItem represents a task from the plan.
type TaskPlanItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	CLI          string   `json:"cli"`
	Dependencies []string `json:"dependencies"`
}

func (w *WorkflowRunner) parsePlan(output string) ([]*core.Task, error) {
	// Try to parse as JSON array
	var planItems []TaskPlanItem
	if err := json.Unmarshal([]byte(output), &planItems); err != nil {
		// Try wrapping in tasks array
		var wrapper struct {
			Tasks []TaskPlanItem `json:"tasks"`
		}
		if err := json.Unmarshal([]byte(output), &wrapper); err != nil {
			// Return empty plan if parsing fails
			w.logger.Warn("failed to parse plan, using empty task list", "error", err)
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

func (w *WorkflowRunner) rebuildDAG(state *core.WorkflowState) error {
	w.dag = NewDAGBuilder()

	for id, taskState := range state.Tasks {
		task := &core.Task{
			ID:           id,
			Name:         taskState.Name,
			Phase:        taskState.Phase,
			Status:       taskState.Status,
			CLI:          taskState.CLI,
			Model:        taskState.Model,
			Dependencies: taskState.Dependencies,
		}
		w.dag.AddTask(task)
	}

	// Add dependencies
	for _, taskState := range state.Tasks {
		for _, dep := range taskState.Dependencies {
			w.dag.AddDependency(taskState.ID, dep)
		}
	}

	return nil
}

func generateWorkflowID() string {
	counter := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("wf-%d-%d", time.Now().UnixNano(), counter)
}

// GetState returns the current workflow state (for testing).
func (w *WorkflowRunner) GetState(ctx context.Context) (*core.WorkflowState, error) {
	return w.state.Load(ctx)
}

// SetDryRun enables or disables dry-run mode.
func (w *WorkflowRunner) SetDryRun(enabled bool) {
	w.config.DryRun = enabled
}
