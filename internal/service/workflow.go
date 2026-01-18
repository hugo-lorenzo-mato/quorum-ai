package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	trace      TraceWriter
	traceID    string
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
	Trace        TraceConfig
	AppVersion   string
	AppCommit    string
	AppDate      string
	GitCommit    string
	GitDirty     bool
	// AgentPhaseModels allows per-agent, per-phase model overrides.
	AgentPhaseModels map[string]map[string]string
	// ConsolidatorAgent specifies which agent to use for analysis consolidation.
	ConsolidatorAgent string
	// ConsolidatorModel specifies the model to use for consolidation (optional).
	ConsolidatorModel string
}

// DefaultWorkflowRunnerConfig returns default configuration.
func DefaultWorkflowRunnerConfig() *WorkflowRunnerConfig {
	return &WorkflowRunnerConfig{
		Timeout:           time.Hour,
		MaxRetries:        3,
		DryRun:            false,
		Sandbox:           true,
		DefaultAgent:      "claude",
		V3Agent:           "claude",
		Trace:             TraceConfig{Mode: "off"},
		AgentPhaseModels:  map[string]map[string]string{},
		ConsolidatorAgent: "claude",
		ConsolidatorModel: "",
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
		trace:      NewTraceWriter(config.Trace, logger),
	}
}

// Run executes a complete workflow from a user prompt.
func (w *WorkflowRunner) Run(ctx context.Context, prompt string) error {
	// Validate input
	if err := w.validateRunInput(ctx, prompt); err != nil {
		return err
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := w.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = w.state.ReleaseLock(ctx) }()

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

	w.startTrace(ctx, state, prompt)

	// Save initial state
	if err := w.state.Save(ctx, state); err != nil {
		err = fmt.Errorf("saving initial state: %w", err)
		w.endTrace(ctx)
		return err
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

	if err := w.state.Save(ctx, state); err != nil {
		w.endTrace(ctx)
		return err
	}

	w.endTrace(ctx)
	return nil
}

// Resume continues a workflow from the last checkpoint.
func (w *WorkflowRunner) Resume(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()

	if err := w.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = w.state.ReleaseLock(ctx) }()

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

	w.startTrace(ctx, state, state.Prompt)

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

	if err := w.state.Save(ctx, state); err != nil {
		w.endTrace(ctx)
		return err
	}

	w.endTrace(ctx)
	return nil
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

	decision := "skip_v2"
	if consensusResult.NeedsV3 || consensusResult.NeedsHumanReview {
		decision = "v2"
	}
	w.logger.Info("consensus:v1",
		"score", consensusResult.Score,
		"threshold", w.consensus.Threshold,
		"decision", decision,
	)
	w.recordConsensus(ctx, "v1", consensusResult)

	// Track all outputs for consolidation
	allOutputs := make([]AnalysisOutput, 0, len(v1Outputs)*3)
	allOutputs = append(allOutputs, v1Outputs...)

	// V2: If low consensus (either needs V3 or needs human review), run critique
	if consensusResult.NeedsV3 || consensusResult.NeedsHumanReview {
		v2Outputs, err := w.runV2Critique(ctx, state, v1Outputs)
		if err != nil {
			return fmt.Errorf("V2 critique: %w", err)
		}
		allOutputs = append(allOutputs, v2Outputs...)

		// Re-evaluate consensus
		consensusResult = w.consensus.Evaluate(allOutputs)
		if err := w.checkpoint.ConsensusCheckpoint(ctx, state, consensusResult); err != nil {
			w.logger.Warn("failed to create V2 consensus checkpoint", "error", err)
		}

		decision = "skip_v3"
		if consensusResult.NeedsV3 || consensusResult.NeedsHumanReview {
			decision = "v3"
		}
		w.logger.Info("consensus:v2",
			"score", consensusResult.Score,
			"threshold", w.consensus.Threshold,
			"decision", decision,
		)
		w.recordConsensus(ctx, "v2", consensusResult)

		// V3: If still low, run reconciliation
		if consensusResult.NeedsV3 || consensusResult.NeedsHumanReview {
			if err := w.runV3Reconciliation(ctx, state, v1Outputs, v2Outputs, consensusResult); err != nil {
				return fmt.Errorf("V3 reconciliation: %w", err)
			}
		}
	}

	// Consolidate analysis using all outputs (V1, V2, V3)
	if err := w.consolidateAnalysis(ctx, state, allOutputs); err != nil {
		return fmt.Errorf("consolidating analysis: %w", err)
	}

	if err := w.checkpoint.PhaseCheckpoint(ctx, state, core.PhaseAnalyze, true); err != nil {
		w.logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// runV1Analysis runs initial analysis with multiple agents in parallel.
func (w *WorkflowRunner) runV1Analysis(ctx context.Context, state *core.WorkflowState) ([]AnalysisOutput, error) {
	agentNames := w.agents.Available(ctx)
	if len(agentNames) == 0 {
		return nil, core.ErrValidation("NO_AGENTS", "no agents available")
	}

	// Limit to 2 agents for V1
	if len(agentNames) > 2 {
		agentNames = agentNames[:2]
	}

	w.logger.Info("analyze:v1 start",
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

	w.logger.Info("analyze:v1 done",
		"agents", strings.Join(agentNames, ", "),
	)

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

	model := w.resolveModel(agentName, core.PhaseAnalyze, "")
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "v1",
		EventType: "prompt",
		Agent:     agentName,
		Model:     model,
		FileExt:   "txt",
		Content:   []byte(prompt),
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})

	// Execute with retry
	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   model,
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
	w.logger.Info("analyze:v1 agent done",
		"agent", agentName,
		"model", model,
	)
	w.logger.Debug("analyze:v1 agent usage",
		"agent", agentName,
		"model", model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD,
	)
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "v1",
		EventType: "response",
		Agent:     agentName,
		Model:     model,
		FileExt:   "json",
		Content:   []byte(result.Output),
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
		CostUSD:   result.CostUSD,
	})
	return output, nil
}

// runV2Critique runs critical review of V1 outputs.
func (w *WorkflowRunner) runV2Critique(ctx context.Context, state *core.WorkflowState, v1Outputs []AnalysisOutput) ([]AnalysisOutput, error) {
	w.logger.Info("starting V2 critique phase")

	outputs := make([]AnalysisOutput, 0)

	// Each agent critiques the other's output
	for i, v1 := range v1Outputs {
		critiqueAgent := w.selectCritiqueAgent(ctx, v1.AgentName)
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

		model := w.resolveModel(critiqueAgent, core.PhaseAnalyze, "")
		w.logger.Info("analyze:v2 start",
			"agent", critiqueAgent,
			"critiquing", v1.AgentName,
			"model", model,
		)
		_ = w.trace.Record(ctx, TraceEvent{
			Phase:     "analyze",
			Step:      "v2",
			EventType: "prompt",
			Agent:     critiqueAgent,
			Model:     model,
			FileExt:   "txt",
			Content:   []byte(prompt),
			Metadata: map[string]interface{}{
				"critiquing": v1.AgentName,
				"format":     "json",
			},
		})

		var result *core.ExecuteResult
		err = w.retry.Execute(ctx, func(ctx context.Context) error {
			var execErr error
			result, execErr = agent.Execute(ctx, core.ExecuteOptions{
				Prompt:  prompt,
				Format:  core.OutputFormatJSON,
				Model:   model,
				Timeout: 5 * time.Minute,
				Sandbox: w.config.Sandbox,
			})
			return execErr
		})

		if err == nil {
			output := w.parseAnalysisOutput(fmt.Sprintf("%s-critique-%d", critiqueAgent, i), result)
			w.logger.Info("analyze:v2 agent done",
				"agent", critiqueAgent,
				"model", model,
			)
			w.logger.Debug("analyze:v2 agent usage",
				"agent", critiqueAgent,
				"model", model,
				"tokens_in", result.TokensIn,
				"tokens_out", result.TokensOut,
				"cost_usd", result.CostUSD,
			)
			_ = w.trace.Record(ctx, TraceEvent{
				Phase:     "analyze",
				Step:      "v2",
				EventType: "response",
				Agent:     critiqueAgent,
				Model:     model,
				FileExt:   "json",
				Content:   []byte(result.Output),
				TokensIn:  result.TokensIn,
				TokensOut: result.TokensOut,
				CostUSD:   result.CostUSD,
				Metadata: map[string]interface{}{
					"critiquing": v1.AgentName,
				},
			})
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

	model := w.resolveModel(v3AgentName, core.PhaseAnalyze, "")
	w.logger.Info("analyze:v3 start",
		"agent", v3AgentName,
		"model", model,
		"divergences", len(consensus.Divergences),
	)
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "v3",
		EventType: "prompt",
		Agent:     v3AgentName,
		Model:     model,
		FileExt:   "txt",
		Content:   []byte(prompt),
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})

	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   model,
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

	w.logger.Info("analyze:v3 done",
		"agent", v3AgentName,
		"model", model,
	)
	w.logger.Debug("analyze:v3 usage",
		"agent", v3AgentName,
		"model", model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD,
	)
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "v3",
		EventType: "response",
		Agent:     v3AgentName,
		Model:     model,
		FileExt:   "json",
		Content:   []byte(result.Output),
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
		CostUSD:   result.CostUSD,
	})

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
	w.logger.Info("plan start", "workflow_id", state.WorkflowID)

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

	constraints := []string{}
	agentNames := append([]string(nil), w.agents.List()...)
	if len(agentNames) > 0 {
		sort.Strings(agentNames)
		constraints = append(constraints, fmt.Sprintf("If you set agent/cli, use one of: %s", strings.Join(agentNames, ", ")))
	}

	prompt, err := w.prompts.RenderPlanGenerate(PlanParams{
		Prompt:               state.Prompt,
		ConsolidatedAnalysis: analysis,
		Constraints:          constraints,
		MaxTasks:             10,
	})
	if err != nil {
		return fmt.Errorf("rendering plan prompt: %w", err)
	}

	model := w.resolveModel(w.config.DefaultAgent, core.PhasePlan, "")
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "plan",
		Step:      "generate",
		EventType: "prompt",
		Agent:     w.config.DefaultAgent,
		Model:     model,
		FileExt:   "txt",
		Content:   []byte(prompt),
		Metadata: map[string]interface{}{
			"format": "json",
		},
	})

	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   model,
			Timeout: 5 * time.Minute,
			Sandbox: w.config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return err
	}

	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "plan",
		Step:      "generate",
		EventType: "response",
		Agent:     w.config.DefaultAgent,
		Model:     model,
		FileExt:   "json",
		Content:   []byte(result.Output),
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
		CostUSD:   result.CostUSD,
	})

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
		_ = w.dag.AddTask(task) // Errors are caught by dag.Build() below
	}

	// Build dependency graph
	for _, task := range tasks {
		for _, dep := range task.Dependencies {
			_ = w.dag.AddDependency(task.ID, dep) // Errors are caught by dag.Build() below
		}
	}

	// Validate DAG
	if _, err := w.dag.Build(); err != nil {
		return fmt.Errorf("validating task graph: %w", err)
	}

	w.logger.Info("plan done",
		"tasks", len(tasks),
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

	model := w.resolveModel(agentName, core.PhaseExecute, task.Model)
	w.logger.Info("execute task start",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", model,
	)

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

	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "execute",
		Step:      "task",
		EventType: "prompt",
		Agent:     agentName,
		Model:     model,
		TaskID:    string(task.ID),
		TaskName:  task.Name,
		FileExt:   "txt",
		Content:   []byte(prompt),
	})

	// Execute with retry
	var result *core.ExecuteResult
	err = w.retry.ExecuteWithNotify(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:      prompt,
			Format:      core.OutputFormatText,
			Model:       model,
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

	w.logger.Info("execute task done",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", model,
	)
	w.logger.Debug("execute task usage",
		"task_id", task.ID,
		"task_name", task.Name,
		"agent", agentName,
		"model", model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD,
	)
	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "execute",
		Step:      "task",
		EventType: "response",
		Agent:     agentName,
		Model:     model,
		TaskID:    string(task.ID),
		TaskName:  task.Name,
		FileExt:   "txt",
		Content:   []byte(result.Output),
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
		CostUSD:   result.CostUSD,
	})

	if err := w.checkpoint.TaskCheckpoint(ctx, state, task, true); err != nil {
		w.logger.Warn("failed to create task complete checkpoint", "error", err)
	}

	return nil
}

// Helper methods

func (w *WorkflowRunner) resolveModel(agentName string, phase core.Phase, taskModel string) string {
	if strings.TrimSpace(taskModel) != "" {
		return taskModel
	}
	if w.config != nil && w.config.AgentPhaseModels != nil {
		if phaseModels, ok := w.config.AgentPhaseModels[agentName]; ok {
			if model, ok := phaseModels[phase.String()]; ok && strings.TrimSpace(model) != "" {
				return model
			}
		}
	}
	return ""
}

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
	_ = w.state.Save(ctx, state) // Best-effort save on error path
	w.endTrace(ctx)

	return err
}

func (w *WorkflowRunner) startTrace(ctx context.Context, state *core.WorkflowState, prompt string) {
	if w.trace == nil || !w.trace.Enabled() {
		return
	}

	traceID := fmt.Sprintf("%s-%d", state.WorkflowID, time.Now().Unix())
	info := TraceRunInfo{
		RunID:        traceID,
		WorkflowID:   string(state.WorkflowID),
		PromptLength: len(prompt),
		StartedAt:    time.Now().UTC(),
		AppVersion:   w.config.AppVersion,
		AppCommit:    w.config.AppCommit,
		AppDate:      w.config.AppDate,
		GitCommit:    w.config.GitCommit,
		GitDirty:     w.config.GitDirty,
	}

	if err := w.trace.StartRun(ctx, info); err != nil {
		w.logger.Warn("trace start failed", "error", err)
		return
	}

	w.traceID = traceID
	w.logger = w.logger.With("trace_id", traceID)
	w.checkpoint = NewCheckpointManager(w.state, w.logger)

	w.logger.Info("trace enabled", "dir", w.trace.Dir())
}

func (w *WorkflowRunner) endTrace(ctx context.Context) {
	if w.trace == nil || !w.trace.Enabled() {
		return
	}

	summary := w.trace.EndRun(ctx)
	if summary.RunID == "" {
		return
	}

	w.logger.Info("trace summary",
		"prompts", summary.TotalPrompts,
		"tokens_in", summary.TotalTokensIn,
		"tokens_out", summary.TotalTokensOut,
		"cost_usd", summary.TotalCostUSD,
		"dir", summary.Dir,
	)
}

func (w *WorkflowRunner) recordConsensus(ctx context.Context, step string, result ConsensusResult) {
	if w.trace == nil || !w.trace.Enabled() {
		return
	}

	data, err := json.Marshal(result)
	if err != nil {
		return
	}

	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "consensus",
		Step:      step,
		EventType: "consensus",
		FileExt:   "json",
		Content:   data,
		Metadata: map[string]interface{}{
			"score":              result.Score,
			"threshold":          w.consensus.Threshold,
			"needs_v3":           result.NeedsV3,
			"needs_human_review": result.NeedsHumanReview,
		},
	})
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

func (w *WorkflowRunner) selectCritiqueAgent(ctx context.Context, original string) string {
	agents := w.agents.Available(ctx)
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
	// Get consolidator agent
	consolidatorAgent := w.config.ConsolidatorAgent
	if consolidatorAgent == "" {
		consolidatorAgent = w.config.DefaultAgent
	}

	agent, err := w.agents.Get(consolidatorAgent)
	if err != nil {
		w.logger.Warn("consolidator agent not available, using concatenation fallback",
			"agent", consolidatorAgent,
			"error", err,
		)
		return w.consolidateAnalysisFallback(ctx, state, outputs)
	}

	// Acquire rate limit
	limiter := w.rateLimits.Get(consolidatorAgent)
	if err := limiter.Acquire(ctx); err != nil {
		w.logger.Warn("rate limit exceeded for consolidator, using fallback",
			"agent", consolidatorAgent,
		)
		return w.consolidateAnalysisFallback(ctx, state, outputs)
	}

	// Render consolidation prompt
	prompt, err := w.prompts.RenderConsolidateAnalysis(ConsolidateAnalysisParams{
		Prompt:   state.Prompt,
		Analyses: outputs,
	})
	if err != nil {
		w.logger.Warn("failed to render consolidation prompt, using fallback",
			"error", err,
		)
		return w.consolidateAnalysisFallback(ctx, state, outputs)
	}

	// Resolve model
	model := w.config.ConsolidatorModel
	if model == "" {
		model = w.resolveModel(consolidatorAgent, core.PhaseAnalyze, "")
	}

	w.logger.Info("consolidate start",
		"agent", consolidatorAgent,
		"model", model,
		"analyses_count", len(outputs),
	)

	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "consolidate",
		EventType: "prompt",
		Agent:     consolidatorAgent,
		Model:     model,
		FileExt:   "txt",
		Content:   []byte(prompt),
		Metadata: map[string]interface{}{
			"format":         "json",
			"analyses_count": len(outputs),
		},
	})

	// Execute with retry
	var result *core.ExecuteResult
	err = w.retry.Execute(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   model,
			Timeout: 5 * time.Minute,
			Sandbox: w.config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		w.logger.Warn("consolidation LLM call failed, using fallback",
			"error", err,
		)
		return w.consolidateAnalysisFallback(ctx, state, outputs)
	}

	w.logger.Info("consolidate done",
		"agent", consolidatorAgent,
		"model", model,
	)
	w.logger.Debug("consolidate usage",
		"agent", consolidatorAgent,
		"model", model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"cost_usd", result.CostUSD,
	)

	_ = w.trace.Record(ctx, TraceEvent{
		Phase:     "analyze",
		Step:      "consolidate",
		EventType: "response",
		Agent:     consolidatorAgent,
		Model:     model,
		FileExt:   "json",
		Content:   []byte(result.Output),
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
		CostUSD:   result.CostUSD,
	})

	// Store the LLM-synthesized consolidation as checkpoint
	return w.checkpoint.CreateCheckpoint(ctx, state, CheckpointType("consolidated_analysis"), map[string]interface{}{
		"content":     result.Output,
		"agent_count": len(outputs),
		"synthesized": true,
		"agent":       consolidatorAgent,
		"model":       model,
		"tokens_in":   result.TokensIn,
		"tokens_out":  result.TokensOut,
		"cost_usd":    result.CostUSD,
	})
}

// consolidateAnalysisFallback concatenates analyses when LLM consolidation fails.
func (w *WorkflowRunner) consolidateAnalysisFallback(ctx context.Context, state *core.WorkflowState, outputs []AnalysisOutput) error {
	var consolidated strings.Builder

	for _, output := range outputs {
		consolidated.WriteString(fmt.Sprintf("## Analysis from %s\n", output.AgentName))
		consolidated.WriteString(output.RawOutput)
		consolidated.WriteString("\n\n")
	}

	return w.checkpoint.CreateCheckpoint(ctx, state, CheckpointType("consolidated_analysis"), map[string]interface{}{
		"content":     consolidated.String(),
		"agent_count": len(outputs),
		"synthesized": false,
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
	Agent        string   `json:"agent"`
	Dependencies []string `json:"dependencies"`
}

func (w *WorkflowRunner) parsePlan(output string) ([]*core.Task, error) {
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
		cli = w.resolveTaskAgent(cli)
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

func (w *WorkflowRunner) resolveTaskAgent(candidate string) string {
	cleaned := strings.TrimSpace(candidate)
	if cleaned == "" {
		return w.config.DefaultAgent
	}

	if isShellLikeAgent(cleaned) {
		w.logger.Warn("plan used shell name as agent, defaulting",
			"agent", cleaned,
			"default", w.config.DefaultAgent,
		)
		return w.config.DefaultAgent
	}

	for _, name := range w.agents.List() {
		if strings.EqualFold(name, cleaned) {
			return name
		}
	}

	w.logger.Warn("plan requested unknown agent, defaulting",
		"agent", cleaned,
		"default", w.config.DefaultAgent,
	)
	return w.config.DefaultAgent
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
		_ = w.dag.AddTask(task) // Errors are non-fatal during restoration
	}

	// Add dependencies
	for _, taskState := range state.Tasks {
		for _, dep := range taskState.Dependencies {
			_ = w.dag.AddDependency(taskState.ID, dep) // Errors are non-fatal during restoration
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

// Validation methods

// validateRunInput validates the input for Run.
func (w *WorkflowRunner) validateRunInput(ctx context.Context, prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return core.ErrValidation(core.CodeEmptyPrompt, "prompt cannot be empty")
	}
	if len(prompt) > core.MaxPromptLength {
		return core.ErrValidation(core.CodePromptTooLong,
			fmt.Sprintf("prompt exceeds maximum length of %d characters", core.MaxPromptLength))
	}
	if w.config.Timeout <= 0 {
		return core.ErrValidation(core.CodeInvalidTimeout, "timeout must be positive")
	}
	if len(w.agents.Available(ctx)) == 0 {
		return core.ErrValidation(core.CodeNoAgents, "no agents configured")
	}
	return nil
}
