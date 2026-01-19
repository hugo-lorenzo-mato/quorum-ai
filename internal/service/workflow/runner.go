// Package workflow provides the workflow orchestration components.
// It implements a hexagonal architecture pattern where the Runner
// orchestrates phase-specific runners (Analyzer, Planner, Executor).
package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// idCounter provides additional uniqueness for workflow IDs.
var idCounter uint64

// StateManager manages workflow state persistence and locking.
type StateManager interface {
	Save(ctx context.Context, state *core.WorkflowState) error
	Load(ctx context.Context) (*core.WorkflowState, error)
	AcquireLock(ctx context.Context) error
	ReleaseLock(ctx context.Context) error
}

// ResumePointProvider determines where to resume a workflow.
type ResumePointProvider interface {
	GetResumePoint(state *core.WorkflowState) (*ResumePoint, error)
}

// ResumePoint indicates where to resume workflow execution.
type ResumePoint struct {
	Phase     core.Phase
	TaskID    core.TaskID
	FromStart bool
}

// RunnerConfig holds configuration for the workflow runner.
type RunnerConfig struct {
	Timeout      time.Duration
	MaxRetries   int
	DryRun       bool
	Sandbox      bool
	DenyTools    []string
	DefaultAgent string
	V3Agent      string
	// AgentPhaseModels allows per-agent, per-phase model overrides.
	AgentPhaseModels map[string]map[string]string
	// WorktreeAutoClean controls automatic worktree cleanup after task execution.
	WorktreeAutoClean bool
	// WorktreeMode controls when worktrees are created for tasks.
	WorktreeMode string
	// MaxCostPerWorkflow is the maximum total cost for the workflow in USD (0 = unlimited).
	MaxCostPerWorkflow float64
	// MaxCostPerTask is the maximum cost per task in USD (0 = unlimited).
	MaxCostPerTask float64
	// Optimizer configures the prompt optimization phase.
	Optimizer OptimizerConfig
	// Consolidator configures the analysis consolidation phase.
	Consolidator ConsolidatorConfig
	// Report configures the markdown report output.
	Report report.Config
}

// ConsolidatorConfig configures the analysis consolidation phase.
type ConsolidatorConfig struct {
	// Agent specifies which agent to use for consolidation (e.g., "claude", "gemini").
	Agent string
	// Model specifies the model to use (optional, uses agent's phase_models.analyze if empty).
	Model string
}

// DefaultRunnerConfig returns default configuration.
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		Timeout:          time.Hour,
		MaxRetries:       3,
		DryRun:           false,
		Sandbox:          true,
		DefaultAgent:     "claude",
		V3Agent:          "claude",
		AgentPhaseModels: map[string]map[string]string{},
		WorktreeMode:     "always",
		Consolidator: ConsolidatorConfig{
			Agent: "claude",
			Model: "",
		},
		Report: report.DefaultConfig(),
	}
}

// Runner orchestrates the complete workflow execution.
// It coordinates the optimization, analysis, planning, and execution phases
// but delegates the actual work to specialized phase runners.
type Runner struct {
	config         *RunnerConfig
	state          StateManager
	agents         core.AgentRegistry
	optimizer      *Optimizer
	analyzer       *Analyzer
	planner        *Planner
	executor       *Executor
	checkpoint     CheckpointCreator
	resumeProvider ResumePointProvider
	prompts        PromptRenderer
	retry          RetryExecutor
	rateLimits     RateLimiterGetter
	worktrees      WorktreeManager
	logger         *logging.Logger
	output         OutputNotifier
	modeEnforcer   ModeEnforcerInterface
	control        *control.ControlPlane
}

// RunnerDeps holds dependencies for creating a Runner.
type RunnerDeps struct {
	Config    *RunnerConfig
	State     StateManager
	Agents    core.AgentRegistry
	Consensus ConsensusEvaluator
	DAG       interface {
		DAGBuilder
		TaskDAG
	}
	Checkpoint     CheckpointCreator
	ResumeProvider ResumePointProvider
	Prompts        PromptRenderer
	Retry          RetryExecutor
	RateLimits     RateLimiterGetter
	Worktrees      WorktreeManager
	Logger         *logging.Logger
	Output         OutputNotifier
	ModeEnforcer   ModeEnforcerInterface
	Control        *control.ControlPlane
}

// NewRunner creates a new workflow runner with all dependencies.
func NewRunner(deps RunnerDeps) *Runner {
	if deps.Config == nil {
		deps.Config = DefaultRunnerConfig()
	}
	if deps.Logger == nil {
		deps.Logger = logging.NewNop()
	}
	if deps.Output == nil {
		deps.Output = NopOutputNotifier{}
	}

	return &Runner{
		config:         deps.Config,
		state:          deps.State,
		agents:         deps.Agents,
		optimizer:      NewOptimizer(deps.Config.Optimizer),
		analyzer:       NewAnalyzer(deps.Consensus),
		planner:        NewPlanner(deps.DAG, deps.State),
		executor:       NewExecutor(deps.DAG, deps.State, deps.Config.DenyTools),
		checkpoint:     deps.Checkpoint,
		resumeProvider: deps.ResumeProvider,
		prompts:        deps.Prompts,
		retry:          deps.Retry,
		rateLimits:     deps.RateLimits,
		worktrees:      deps.Worktrees,
		logger:         deps.Logger,
		output:         deps.Output,
		modeEnforcer:   deps.ModeEnforcer,
		control:        deps.Control,
	}
}

// Run executes a complete workflow from a user prompt.
func (r *Runner) Run(ctx context.Context, prompt string) error {
	// Validate input
	if err := r.validateRunInput(prompt); err != nil {
		return err
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = r.state.ReleaseLock(ctx) }()

	// Initialize state
	workflowState := r.initializeState(prompt)

	r.logger.Info("starting workflow",
		"workflow_id", workflowState.WorkflowID,
		"prompt_length", len(prompt),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Workflow started: %s", workflowState.WorkflowID))
	}

	// Save initial state
	if err := r.state.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run phases
	if err := r.optimizer.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	if err := r.analyzer.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	if err := r.planner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	if err := r.executor.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark completed
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("workflow completed",
		"workflow_id", workflowState.WorkflowID,
		"total_tasks", len(workflowState.Tasks),
		"duration", workflowState.Metrics.Duration,
		"total_cost", workflowState.Metrics.TotalCostUSD,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Workflow completed: %d tasks, $%.4f, %s",
			len(workflowState.Tasks),
			workflowState.Metrics.TotalCostUSD,
			workflowState.Metrics.Duration.Round(time.Second)))
	}

	return r.state.Save(ctx, workflowState)
}

// Resume continues a workflow from the last checkpoint.
func (r *Runner) Resume(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = r.state.ReleaseLock(ctx) }()

	// Load existing state
	workflowState, err := r.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found to resume")
	}

	// Get resume point
	resumePoint, err := r.resumeProvider.GetResumePoint(workflowState)
	if err != nil {
		return fmt.Errorf("determining resume point: %w", err)
	}

	r.logger.Info("resuming workflow",
		"workflow_id", workflowState.WorkflowID,
		"phase", resumePoint.Phase,
		"task_id", resumePoint.TaskID,
		"from_start", resumePoint.FromStart,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Resuming workflow from %s phase", resumePoint.Phase))
	}

	wctx := r.createContext(workflowState)

	// Resume from appropriate phase
	switch resumePoint.Phase {
	case core.PhaseOptimize:
		if err := r.optimizer.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, workflowState, err)
		}
		fallthrough
	case core.PhaseAnalyze:
		if err := r.analyzer.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, workflowState, err)
		}
		fallthrough
	case core.PhasePlan:
		if err := r.planner.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, workflowState, err)
		}
		fallthrough
	case core.PhaseExecute:
		if err := r.executor.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, workflowState, err)
		}
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("workflow resumed and completed",
		"workflow_id", workflowState.WorkflowID,
		"duration", workflowState.Metrics.Duration,
		"total_cost", workflowState.Metrics.TotalCostUSD,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Resumed workflow completed: %d tasks, $%.4f",
			len(workflowState.Tasks),
			workflowState.Metrics.TotalCostUSD))
	}

	return r.state.Save(ctx, workflowState)
}

// initializeState creates initial workflow state.
func (r *Runner) initializeState(prompt string) *core.WorkflowState {
	return &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   core.WorkflowID(generateWorkflowID()),
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseOptimize,
		Prompt:       prompt,
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    make([]core.TaskID, 0),
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// createContext creates a workflow context for phase runners.
func (r *Runner) createContext(state *core.WorkflowState) *Context {
	// Create report writer for this workflow execution
	reportWriter := report.NewWorkflowReportWriter(r.config.Report, string(state.WorkflowID))

	return &Context{
		State:        state,
		Agents:       r.agents,
		Prompts:      r.prompts,
		Checkpoint:   r.checkpoint,
		Retry:        r.retry,
		RateLimits:   r.rateLimits,
		Worktrees:    r.worktrees,
		Logger:       r.logger,
		Output:       r.output,
		ModeEnforcer: r.modeEnforcer,
		Control:      r.control,
		Report:       reportWriter,
		Config: &Config{
			DryRun:             r.config.DryRun,
			Sandbox:            r.config.Sandbox,
			DenyTools:          r.config.DenyTools,
			DefaultAgent:       r.config.DefaultAgent,
			V3Agent:            r.config.V3Agent,
			AgentPhaseModels:   r.config.AgentPhaseModels,
			WorktreeAutoClean:  r.config.WorktreeAutoClean,
			WorktreeMode:       r.config.WorktreeMode,
			MaxCostPerWorkflow: r.config.MaxCostPerWorkflow,
			MaxCostPerTask:     r.config.MaxCostPerTask,
			ConsolidatorAgent:  r.config.Consolidator.Agent,
			ConsolidatorModel:  r.config.Consolidator.Model,
		},
	}
}

// handleError handles workflow errors.
func (r *Runner) handleError(ctx context.Context, state *core.WorkflowState, err error) error {
	r.logger.Error("workflow error",
		"workflow_id", state.WorkflowID,
		"phase", state.CurrentPhase,
		"error", err,
	)
	if r.output != nil {
		r.output.Log("error", "workflow", fmt.Sprintf("Workflow failed in %s: %s", state.CurrentPhase, err.Error()))
	}

	state.Status = core.WorkflowStatusFailed
	state.UpdatedAt = time.Now()
	if checkpointErr := r.checkpoint.ErrorCheckpoint(state, err); checkpointErr != nil {
		r.logger.Warn("failed to create error checkpoint", "checkpoint_error", checkpointErr)
	}
	_ = r.state.Save(ctx, state)

	return err
}

// validateRunInput validates the input for Run.
func (r *Runner) validateRunInput(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return core.ErrValidation(core.CodeEmptyPrompt, "prompt cannot be empty")
	}
	if len(prompt) > core.MaxPromptLength {
		return core.ErrValidation(core.CodePromptTooLong,
			fmt.Sprintf("prompt exceeds maximum length of %d characters", core.MaxPromptLength))
	}
	if r.config.Timeout <= 0 {
		return core.ErrValidation(core.CodeInvalidTimeout, "timeout must be positive")
	}
	if len(r.agents.List()) == 0 {
		return core.ErrValidation(core.CodeNoAgents, "no agents configured")
	}
	return nil
}

// Analyze executes only the analyze phase (with optional optimization).
// This is useful when you want to get multi-agent analysis without planning or execution.
func (r *Runner) Analyze(ctx context.Context, prompt string) error {
	// Validate input
	if err := r.validateRunInput(prompt); err != nil {
		return err
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = r.state.ReleaseLock(ctx) }()

	// Initialize state
	workflowState := r.initializeState(prompt)

	r.logger.Info("starting analyze-only workflow",
		"workflow_id", workflowState.WorkflowID,
		"prompt_length", len(prompt),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Analyze-only workflow started: %s", workflowState.WorkflowID))
	}

	// Save initial state
	if err := r.state.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run optimization phase (if enabled)
	if err := r.optimizer.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Run analyze phase
	if err := r.analyzer.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark as completed (analyze phase done, ready for plan)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhasePlan // Ready for next phase
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("analyze phase completed",
		"workflow_id", workflowState.WorkflowID,
		"duration", workflowState.Metrics.Duration,
		"total_cost", workflowState.Metrics.TotalCostUSD,
		"consensus_score", workflowState.Metrics.ConsensusScore,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Analysis completed: consensus %.1f%%, $%.4f",
			workflowState.Metrics.ConsensusScore*100,
			workflowState.Metrics.TotalCostUSD))
	}

	return r.state.Save(ctx, workflowState)
}

// GetState returns the current workflow state.
func (r *Runner) GetState(ctx context.Context) (*core.WorkflowState, error) {
	return r.state.Load(ctx)
}

// SetDryRun enables or disables dry-run mode.
func (r *Runner) SetDryRun(enabled bool) {
	r.config.DryRun = enabled
}

// generateWorkflowID generates a unique workflow ID.
func generateWorkflowID() string {
	counter := atomic.AddUint64(&idCounter, 1)
	return fmt.Sprintf("wf-%d-%d", time.Now().UnixNano(), counter)
}

// finalizeMetrics calculates final aggregate metrics.
func (r *Runner) finalizeMetrics(state *core.WorkflowState) {
	if state.Metrics == nil {
		state.Metrics = &core.StateMetrics{}
	}

	// Calculate workflow duration
	state.Metrics.Duration = time.Since(state.CreatedAt)

	// Note: ConsensusScore is set by analyzer during analyze phase
	// See analyzer.go for where this is updated
}
