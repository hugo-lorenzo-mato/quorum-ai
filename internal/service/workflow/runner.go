// Package workflow provides the workflow orchestration components.
// It implements a hexagonal architecture pattern where the Runner
// orchestrates phase-specific runners (Analyzer, Planner, Executor).
package workflow

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// StateManager manages workflow state persistence and locking.
type StateManager interface {
	Save(ctx context.Context, state *core.WorkflowState) error
	Load(ctx context.Context) (*core.WorkflowState, error)
	LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)
	AcquireLock(ctx context.Context) error
	ReleaseLock(ctx context.Context) error
	DeactivateWorkflow(ctx context.Context) error
	ArchiveWorkflows(ctx context.Context) (int, error)
	PurgeAllWorkflows(ctx context.Context) (int, error)
	DeleteWorkflow(ctx context.Context, id core.WorkflowID) error
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
	DenyTools    []string
	DefaultAgent string
	// AgentPhaseModels allows per-agent, per-phase model overrides.
	AgentPhaseModels map[string]map[string]string
	// WorktreeAutoClean controls automatic worktree cleanup after task execution.
	WorktreeAutoClean bool
	// WorktreeMode controls when worktrees are created for tasks.
	WorktreeMode string
	// Refiner configures the prompt refinement phase.
	Refiner RefinerConfig
	// Synthesizer configures the analysis synthesis phase.
	Synthesizer SynthesizerConfig
	// PlanSynthesizer configures multi-agent plan synthesis.
	PlanSynthesizer PlanSynthesizerConfig
	// Report configures the markdown report output.
	Report report.Config
	// PhaseTimeouts holds per-phase timeout durations.
	PhaseTimeouts PhaseTimeouts
	// Moderator configures the semantic moderator for consensus evaluation.
	Moderator ModeratorConfig
	// SingleAgent configures single-agent execution mode (bypasses multi-agent consensus).
	SingleAgent SingleAgentConfig
	// Finalization configures post-task git operations (commit, push, PR).
	Finalization FinalizationConfig
	// ProjectAgentPhases maps agent name -> enabled phases for the current project.
	// This overrides the global agent phases from the server config.
	// Empty list means all phases are enabled.
	ProjectAgentPhases map[string][]string
}

// SynthesizerConfig configures the analysis synthesis phase.
type SynthesizerConfig struct {
	// Agent specifies which agent to use for synthesis (e.g., "claude", "gemini").
	// The model is resolved from AgentPhaseModels[Agent][analyze] at runtime.
	Agent string
}

// PlanSynthesizerConfig configures multi-agent plan synthesis.
type PlanSynthesizerConfig struct {
	// Enabled controls whether multi-agent planning is used.
	Enabled bool
	// Agent specifies which agent to use for plan synthesis.
	// The model is resolved from AgentPhaseModels[Agent][plan] at runtime.
	Agent string
}

// DefaultRunnerConfig returns default configuration.
// NOTE: DefaultAgent, Synthesizer.Agent, and Moderator fields have NO defaults.
// These MUST be configured explicitly in the config file (.quorum/config.yaml).
// The source of truth is always the config file, never code defaults.
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		Timeout:          time.Hour,
		MaxRetries:       3,
		DryRun:           false,
		DefaultAgent:     "", // NO default - must be configured
		AgentPhaseModels: map[string]map[string]string{},
		WorktreeMode:     "always",
		Synthesizer: SynthesizerConfig{
			Agent: "", // NO default - must be configured
		},
		PlanSynthesizer: PlanSynthesizerConfig{
			Enabled: false, // Disabled by default - opt-in feature
			Agent:   "",    // NO default - must be configured
		},
		Report: report.DefaultConfig(),
		PhaseTimeouts: PhaseTimeouts{
			Analyze: 3 * time.Hour,
			Plan:    3 * time.Hour,
			Execute: 3 * time.Hour,
		},
		Moderator: ModeratorConfig{
			Enabled:             false, // Disabled by default - must be explicitly enabled
			Agent:               "",    // NO default - must be configured
			Threshold:           0.90,
			MinSuccessfulAgents: 2,
			MinRounds:           2,
			MaxRounds:           5,
			WarningThreshold:    0.30,
			StagnationThreshold: 0.02,
		},
	}
}

// Runner orchestrates the complete workflow execution.
// It coordinates the refinement, analysis, planning, and execution phases
// but delegates the actual work to specialized phase runners.
type Runner struct {
	config            *RunnerConfig
	state             StateManager
	agents            core.AgentRegistry
	refiner           *Refiner
	analyzer          *Analyzer
	planner           *Planner
	executor          *Executor
	checkpoint        CheckpointCreator
	resumeProvider    ResumePointProvider
	prompts           PromptRenderer
	retry             RetryExecutor
	rateLimits        RateLimiterGetter
	worktrees         WorktreeManager
	workflowWorktrees core.WorkflowWorktreeManager
	gitIsolation      *GitIsolationConfig
	git               core.GitClient
	github            core.GitHubClient
	logger            *logging.Logger
	output            OutputNotifier
	modeEnforcer      ModeEnforcerInterface
	control           *control.ControlPlane
	heartbeat         *HeartbeatManager
	projectRoot       string // Project root directory for multi-project support
}

// RunnerDeps holds dependencies for creating a Runner.
type RunnerDeps struct {
	Config *RunnerConfig
	State  StateManager
	Agents core.AgentRegistry
	DAG    interface {
		DAGBuilder
		TaskDAG
	}
	Checkpoint        CheckpointCreator
	ResumeProvider    ResumePointProvider
	Prompts           PromptRenderer
	Retry             RetryExecutor
	RateLimits        RateLimiterGetter
	Worktrees         WorktreeManager
	WorkflowWorktrees core.WorkflowWorktreeManager
	GitIsolation      *GitIsolationConfig
	GitClientFactory  GitClientFactory
	Git               core.GitClient
	GitHub            core.GitHubClient
	Logger            *logging.Logger
	Output            OutputNotifier
	ModeEnforcer      ModeEnforcerInterface
	Control           *control.ControlPlane
	Heartbeat         *HeartbeatManager
	ProjectRoot       string // Project root directory for multi-project support
}

// NewRunner creates a new workflow runner with all dependencies.
func NewRunner(deps RunnerDeps) (*Runner, error) {
	if deps.Config == nil {
		deps.Config = DefaultRunnerConfig()
	}
	if deps.Logger == nil {
		deps.Logger = logging.NewNop()
	}
	if deps.Output == nil {
		deps.Output = NopOutputNotifier{}
	}

	// Create analyzer with moderator config
	analyzer, err := NewAnalyzer(deps.Config.Moderator)
	if err != nil {
		deps.Logger.Error("failed to create analyzer", "error", err)
		return nil, fmt.Errorf("creating analyzer: %w", err)
	}

	return &Runner{
		config:            deps.Config,
		state:             deps.State,
		agents:            deps.Agents,
		refiner:           NewRefiner(deps.Config.Refiner),
		analyzer:          analyzer,
		planner:           NewPlanner(deps.DAG, deps.State),
		executor:          NewExecutor(deps.DAG, deps.State, deps.Config.DenyTools).WithGitFactory(deps.GitClientFactory),
		checkpoint:        deps.Checkpoint,
		resumeProvider:    deps.ResumeProvider,
		prompts:           deps.Prompts,
		retry:             deps.Retry,
		rateLimits:        deps.RateLimits,
		worktrees:         deps.Worktrees,
		workflowWorktrees: deps.WorkflowWorktrees,
		gitIsolation:      deps.GitIsolation,
		git:               deps.Git,
		github:            deps.GitHub,
		logger:            deps.Logger,
		output:            deps.Output,
		modeEnforcer:      deps.ModeEnforcer,
		control:           deps.Control,
		heartbeat:         deps.Heartbeat,
		projectRoot:       deps.ProjectRoot,
	}, nil
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

	// Validate all configured agents have their CLIs installed (fail fast)
	if err := r.ValidateAgentAvailability(ctx); err != nil {
		return err
	}

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Initialize state
	workflowState := r.initializeState(prompt)

	// Ensure workflow-level Git isolation (creates workflow branch/worktree namespace).
	if _, err := r.ensureWorkflowGitIsolation(ctx, workflowState); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

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
	if err := r.refiner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Validate arbiter config before analyze phase (requires arbiter for multi-agent)
	if err := r.ValidateModeratorConfig(); err != nil {
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

	// Workflow-level finalization for Git isolation (push/PR/cleanup).
	r.finalizeWorkflowIsolation(ctx, workflowState)

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark completed - clear CurrentPhase to indicate all phases done
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = "" // Empty means fully completed
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("workflow completed",
		"workflow_id", workflowState.WorkflowID,
		"total_tasks", len(workflowState.Tasks),
		"duration", workflowState.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Workflow completed: %d tasks, %s",
			len(workflowState.Tasks),
			workflowState.Metrics.Duration.Round(time.Second)))
	}

	return r.state.Save(ctx, workflowState)
}

// RunWithState executes a workflow using an existing pre-created state.
// This is for API usage where the workflow was created and persisted before execution.
// Unlike Run(), it does NOT create a new workflow ID - it uses the provided state's ID.
func (r *Runner) RunWithState(ctx context.Context, state *core.WorkflowState) error {
	if state == nil {
		return core.ErrValidation("NIL_STATE", "workflow state cannot be nil")
	}
	if state.WorkflowID == "" {
		return core.ErrValidation("MISSING_WORKFLOW_ID", "workflow state must have a workflow ID")
	}

	// Helper to mark state as failed before returning validation errors.
	// This ensures the UI shows the correct status when validation fails early.
	markFailed := func(err error) error {
		state.Status = core.WorkflowStatusFailed
		state.Error = err.Error()
		state.UpdatedAt = time.Now()
		if saveErr := r.state.Save(ctx, state); saveErr != nil {
			r.logger.Warn("failed to save failed state", "error", saveErr)
		}
		if r.output != nil {
			r.output.Log("error", "workflow", fmt.Sprintf("Workflow failed: %s", err.Error()))
		}
		// Deactivate workflow when validation fails to prevent ghost workflows.
		if deactErr := r.state.DeactivateWorkflow(ctx); deactErr != nil {
			r.logger.Warn("failed to deactivate failed workflow",
				"workflow_id", state.WorkflowID,
				"error", deactErr,
			)
		}
		return err
	}

	if err := r.validateRunInput(state.Prompt); err != nil {
		return markFailed(err)
	}

	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := r.ValidateAgentAvailability(ctx); err != nil {
		return markFailed(err)
	}

	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Use provided state - NO initializeState()
	workflowState := state
	if workflowState.Status != core.WorkflowStatusRunning {
		workflowState.Status = core.WorkflowStatusRunning
	}
	if workflowState.CurrentPhase == "" {
		workflowState.CurrentPhase = core.PhaseRefine
	}
	if workflowState.Tasks == nil {
		workflowState.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	if workflowState.TaskOrder == nil {
		workflowState.TaskOrder = make([]core.TaskID, 0)
	}
	if workflowState.Checkpoints == nil {
		workflowState.Checkpoints = make([]core.Checkpoint, 0)
	}
	if workflowState.Metrics == nil {
		workflowState.Metrics = &core.StateMetrics{}
	}

	// Prepare new execution - increment ExecutionID and clear old events
	r.prepareExecution(workflowState, false)

	// Sync blueprint with actual runner config (API-created workflows have minimal blueprint)
	workflowState.Blueprint = r.buildBlueprint()

	// Ensure workflow-level Git isolation (API-created state may not have it persisted).
	if _, err := r.ensureWorkflowGitIsolation(ctx, workflowState); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	r.logger.Info("starting workflow with existing state",
		"workflow_id", workflowState.WorkflowID,
		"prompt_length", len(workflowState.Prompt),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Workflow started: %s", workflowState.WorkflowID))
	}

	if err := r.state.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Note: Heartbeat tracking is managed by UnifiedTracker when using the API path.
	// Direct runner usage (CLI) doesn't require heartbeat tracking.

	wctx := r.createContext(workflowState)

	// Run all phases (same as Run())
	if err := r.refiner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}
	if err := r.ValidateModeratorConfig(); err != nil {
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

	// Workflow-level finalization for Git isolation (push/PR/cleanup).
	r.finalizeWorkflowIsolation(ctx, workflowState)

	r.finalizeMetrics(workflowState)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = "" // Empty means fully completed
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("workflow completed",
		"workflow_id", workflowState.WorkflowID,
		"total_tasks", len(workflowState.Tasks),
		"duration", workflowState.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Workflow completed: %d tasks, %s",
			len(workflowState.Tasks),
			workflowState.Metrics.Duration.Round(time.Second)))
	}

	return r.state.Save(ctx, workflowState)
}

// AnalyzeWithState executes refine (if enabled) and analyze phases using an existing workflow state.
// This is for API usage where the workflow was created and persisted before phase execution.
//
// After completion, Status will be "completed" and CurrentPhase will be "plan" (ready for planning).
func (r *Runner) AnalyzeWithState(ctx context.Context, state *core.WorkflowState) error {
	if state == nil {
		return core.ErrValidation("NIL_STATE", "workflow state cannot be nil")
	}
	if state.WorkflowID == "" {
		return core.ErrValidation("MISSING_WORKFLOW_ID", "workflow state must have a workflow ID")
	}

	// Helper to mark state as failed before returning validation errors.
	// This ensures the UI shows the correct status when validation fails early.
	markFailed := func(err error) error {
		state.Status = core.WorkflowStatusFailed
		state.Error = err.Error()
		state.UpdatedAt = time.Now()
		if saveErr := r.state.Save(ctx, state); saveErr != nil {
			r.logger.Warn("failed to save failed state", "error", saveErr)
		}
		if r.output != nil {
			r.output.Log("error", "workflow", fmt.Sprintf("Workflow failed: %s", err.Error()))
		}
		// Deactivate workflow when validation fails to prevent ghost workflows.
		if deactErr := r.state.DeactivateWorkflow(ctx); deactErr != nil {
			r.logger.Warn("failed to deactivate failed workflow",
				"workflow_id", state.WorkflowID,
				"error", deactErr,
			)
		}
		return err
	}

	if err := r.validateRunInput(state.Prompt); err != nil {
		return markFailed(err)
	}

	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := r.ValidateAgentAvailability(ctx); err != nil {
		return markFailed(err)
	}

	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	workflowState := state
	if workflowState.Status != core.WorkflowStatusRunning {
		workflowState.Status = core.WorkflowStatusRunning
	}
	if workflowState.CurrentPhase == "" {
		workflowState.CurrentPhase = core.PhaseRefine
	}
	if workflowState.Tasks == nil {
		workflowState.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	if workflowState.TaskOrder == nil {
		workflowState.TaskOrder = make([]core.TaskID, 0)
	}
	if workflowState.Checkpoints == nil {
		workflowState.Checkpoints = make([]core.Checkpoint, 0)
	}
	if workflowState.Metrics == nil {
		workflowState.Metrics = &core.StateMetrics{}
	}

	// New execution: bump execution id and clear any previous agent events.
	r.prepareExecution(workflowState, false)

	// Sync blueprint with actual runner config (API-created workflows have minimal blueprint)
	workflowState.Blueprint = r.buildBlueprint()

	// Ensure workflow-level Git isolation (API-created state may not have it persisted).
	if _, err := r.ensureWorkflowGitIsolation(ctx, workflowState); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	r.logger.Info("starting analyze phase with existing state",
		"workflow_id", workflowState.WorkflowID,
		"prompt_length", len(workflowState.Prompt),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Analyze phase started: %s", workflowState.WorkflowID))
	}

	if err := r.state.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	wctx := r.createContext(workflowState)

	// Run refine (if enabled) and analyze only.
	if err := r.refiner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}
	if err := r.ValidateModeratorConfig(); err != nil {
		return r.handleError(ctx, workflowState, err)
	}
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
		"consensus_score", workflowState.Metrics.ConsensusScore,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Analysis completed: consensus %.1f%%",
			workflowState.Metrics.ConsensusScore*100))
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
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Load existing state
	workflowState, err := r.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found to resume")
	}

	// Ensure workflow-level Git isolation branch exists (older workflows may not have it persisted).
	if changed, err := r.ensureWorkflowGitIsolation(ctx, workflowState); err != nil {
		return r.handleError(ctx, workflowState, err)
	} else if changed {
		if saveErr := r.state.Save(ctx, workflowState); saveErr != nil {
			return fmt.Errorf("saving state after git isolation init: %w", saveErr)
		}
	}

	// Prepare resume execution - increment ExecutionID but keep events for history
	r.prepareExecution(workflowState, true)

	// Reconcile checkpoints from on-disk artifacts before computing the resume point.
	// This prevents re-running analysis when the markdown output exists but checkpoints are missing.
	if recErr := r.reconcileAnalysisArtifacts(ctx, workflowState); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
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
	case core.PhaseRefine:
		if err := r.refiner.Run(ctx, wctx); err != nil {
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
		// When resuming directly to execute, rebuild DAG from existing tasks
		if len(workflowState.Tasks) > 0 {
			r.logger.Info("rebuilding DAG from existing tasks",
				"task_count", len(workflowState.Tasks))
			if r.output != nil {
				r.output.Log("info", "workflow", fmt.Sprintf("Rebuilding task DAG from %d existing tasks", len(workflowState.Tasks)))
			}
			if err := r.planner.RebuildDAGFromState(workflowState); err != nil {
				return r.handleError(ctx, workflowState, fmt.Errorf("rebuilding DAG: %w", err))
			}
		}
		if err := r.executor.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, workflowState, err)
		}
	}

	// Workflow-level finalization for Git isolation (push/PR/cleanup).
	r.finalizeWorkflowIsolation(ctx, workflowState)

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = "" // Empty means fully completed
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("workflow resumed and completed",
		"workflow_id", workflowState.WorkflowID,
		"duration", workflowState.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Resumed workflow completed: %d tasks, %s",
			len(workflowState.Tasks),
			workflowState.Metrics.Duration.Round(time.Second)))
	}

	return r.state.Save(ctx, workflowState)
}

// ResumeWithState continues execution using pre-loaded state.
// This is for API usage where the state was loaded before calling resume.
func (r *Runner) ResumeWithState(ctx context.Context, state *core.WorkflowState) error {
	if state == nil {
		return core.ErrState("NIL_STATE", "workflow state cannot be nil")
	}

	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Ensure workflow-level Git isolation branch exists when resuming.
	if changed, err := r.ensureWorkflowGitIsolation(ctx, state); err != nil {
		return r.handleError(ctx, state, err)
	} else if changed {
		if saveErr := r.state.Save(ctx, state); saveErr != nil {
			return fmt.Errorf("saving state after git isolation init: %w", saveErr)
		}
	}

	// Prepare resume execution - increment ExecutionID but keep events for history
	r.prepareExecution(state, true)

	// Reconcile checkpoints from on-disk artifacts before computing the resume point.
	if recErr := r.reconcileAnalysisArtifacts(ctx, state); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
	}

	resumePoint, err := r.resumeProvider.GetResumePoint(state)
	if err != nil {
		return fmt.Errorf("determining resume point: %w", err)
	}

	r.logger.Info("resuming workflow with existing state",
		"workflow_id", state.WorkflowID,
		"phase", resumePoint.Phase,
		"task_id", resumePoint.TaskID,
		"from_start", resumePoint.FromStart,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Resuming workflow from %s phase", resumePoint.Phase))
	}

	// Note: Heartbeat tracking is managed by UnifiedTracker when using the API path.
	// Direct runner usage (CLI) doesn't require heartbeat tracking.

	wctx := r.createContext(state)

	// Resume from appropriate phase (same logic as Resume())
	switch resumePoint.Phase {
	case core.PhaseRefine:
		if err := r.refiner.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, state, err)
		}
		fallthrough
	case core.PhaseAnalyze:
		if err := r.analyzer.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, state, err)
		}
		fallthrough
	case core.PhasePlan:
		if err := r.planner.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, state, err)
		}
		fallthrough
	case core.PhaseExecute:
		if len(state.Tasks) > 0 {
			r.logger.Info("rebuilding DAG from existing tasks",
				"task_count", len(state.Tasks))
			if r.output != nil {
				r.output.Log("info", "workflow", fmt.Sprintf("Rebuilding task DAG from %d existing tasks", len(state.Tasks)))
			}
			if err := r.planner.RebuildDAGFromState(state); err != nil {
				return r.handleError(ctx, state, fmt.Errorf("rebuilding DAG: %w", err))
			}
		}
		if err := r.executor.Run(ctx, wctx); err != nil {
			return r.handleError(ctx, state, err)
		}
	}

	// Workflow-level finalization for Git isolation (push/PR/cleanup).
	r.finalizeWorkflowIsolation(ctx, state)

	r.finalizeMetrics(state)
	state.Status = core.WorkflowStatusCompleted
	state.CurrentPhase = "" // Empty means fully completed
	state.UpdatedAt = time.Now()

	r.logger.Info("workflow resumed and completed",
		"workflow_id", state.WorkflowID,
		"duration", state.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Resumed workflow completed: %d tasks, %s",
			len(state.Tasks),
			state.Metrics.Duration.Round(time.Second)))
	}

	return r.state.Save(ctx, state)
}

// initializeState creates initial workflow state.
func (r *Runner) initializeState(prompt string) *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Version:    core.CurrentStateVersion,
			WorkflowID: core.WorkflowID(generateWorkflowID()),
			Prompt:     prompt,
			Blueprint:  r.buildBlueprint(),
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			ExecutionID:  1, // First execution
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseRefine,
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    make([]core.TaskID, 0),
			Checkpoints:  make([]core.Checkpoint, 0),
			Metrics:      &core.StateMetrics{},
			UpdatedAt:    time.Now(),
		},
	}
}

// buildBlueprint constructs a Blueprint from the runner configuration.
func (r *Runner) buildBlueprint() *core.Blueprint {
	mode := "multi_agent"
	if r.config.SingleAgent.Enabled {
		mode = "single_agent"
	}

	return &core.Blueprint{
		ExecutionMode: mode,
		SingleAgent: core.BlueprintSingleAgent{
			Agent:           r.config.SingleAgent.Agent,
			Model:           r.config.SingleAgent.Model,
			ReasoningEffort: r.config.SingleAgent.ReasoningEffort,
		},
		Consensus: core.BlueprintConsensus{
			Enabled:             r.config.Moderator.Enabled,
			Agent:               r.config.Moderator.Agent,
			Threshold:           r.config.Moderator.Threshold,
			Thresholds:          r.config.Moderator.Thresholds,
			MinRounds:           r.config.Moderator.MinRounds,
			MaxRounds:           r.config.Moderator.MaxRounds,
			WarningThreshold:    r.config.Moderator.WarningThreshold,
			StagnationThreshold: r.config.Moderator.StagnationThreshold,
		},
		Refiner: core.BlueprintRefiner{
			Enabled: r.config.Refiner.Enabled,
			Agent:   r.config.Refiner.Agent,
		},
		Synthesizer: core.BlueprintSynthesizer{
			Agent: r.config.Synthesizer.Agent,
		},
		PlanSynthesizer: core.BlueprintPlanSynthesizer{
			Enabled: r.config.PlanSynthesizer.Enabled,
			Agent:   r.config.PlanSynthesizer.Agent,
		},
		Phases: core.BlueprintPhases{
			Analyze: core.BlueprintPhaseTimeout{Timeout: r.config.PhaseTimeouts.Analyze},
			Plan:    core.BlueprintPhaseTimeout{Timeout: r.config.PhaseTimeouts.Plan},
			Execute: core.BlueprintPhaseTimeout{Timeout: r.config.PhaseTimeouts.Execute},
		},
		MaxRetries: r.config.MaxRetries,
		Timeout:    r.config.Timeout,
		DryRun:     r.config.DryRun,
	}
}

// prepareExecution increments ExecutionID and optionally clears old events.
// Call this at the start of RunWithState or ResumeWithState to distinguish event sets.
func (r *Runner) prepareExecution(state *core.WorkflowState, isResume bool) {
	state.ExecutionID++
	if !isResume {
		// New execution: clear previous events
		state.AgentEvents = nil
	}
	// Resume: keep events but new execution is distinguished by ExecutionID
	state.UpdatedAt = time.Now()
}

// ensureWorkflowGitIsolation initializes workflow-level Git isolation if configured.
// It creates a workflow branch and worktree namespace and stores the workflow branch in state.
//
// Returns changed=true if the workflow branch was set on state.
func (r *Runner) ensureWorkflowGitIsolation(ctx context.Context, state *core.WorkflowState) (changed bool, _ error) {
	// Avoid side effects in dry-run mode.
	if r.config != nil && r.config.DryRun {
		return false, nil
	}

	if state == nil || state.WorkflowID == "" {
		return false, nil
	}

	if r.gitIsolation == nil || !r.gitIsolation.Enabled {
		return false, nil
	}
	if r.workflowWorktrees == nil {
		return false, nil
	}
	if state.WorkflowBranch != "" {
		return false, nil
	}

	// Safety: don't enable workflow isolation mid-workflow if tasks already executed or have
	// git artifacts from legacy execution (that work wouldn't be present in the new workflow branch).
	for _, ts := range state.Tasks {
		if ts == nil {
			continue
		}
		if ts.Status != "" && ts.Status != core.TaskStatusPending {
			r.logger.Info("skipping workflow git isolation init: workflow already has executed tasks",
				"workflow_id", state.WorkflowID,
				"task_id", ts.ID,
				"task_status", ts.Status,
			)
			return false, nil
		}
		if ts.Branch != "" || ts.LastCommit != "" || ts.WorktreePath != "" {
			r.logger.Info("skipping workflow git isolation init: workflow already has git artifacts",
				"workflow_id", state.WorkflowID,
				"task_id", ts.ID,
				"branch", ts.Branch,
				"worktree_path", ts.WorktreePath,
				"last_commit", ts.LastCommit,
			)
			return false, nil
		}
	}

	info, err := r.workflowWorktrees.InitializeWorkflow(ctx, string(state.WorkflowID), r.gitIsolation.BaseBranch)
	if err != nil {
		return false, err
	}
	if info == nil || info.WorkflowBranch == "" {
		return false, fmt.Errorf("workflow isolation init returned empty branch")
	}

	state.WorkflowBranch = info.WorkflowBranch
	return true, nil
}

// createContext creates a workflow context for phase runners.
func (r *Runner) createContext(state *core.WorkflowState) *Context {
	var reportWriter *report.WorkflowReportWriter

	// Check if we're resuming with an existing report path
	if state.ReportPath != "" {
		// Resume: reuse the original report directory
		if r.logger != nil {
			r.logger.Debug("resuming with existing report path", "path", state.ReportPath)
		}
		reportWriter = report.ResumeWorkflowReportWriter(r.config.Report, string(state.WorkflowID), state.ReportPath)
	} else {
		// New workflow: create a new report directory
		reportWriter = report.NewWorkflowReportWriter(r.config.Report, string(state.WorkflowID))
		// Save the report path to state for future resumes
		state.ReportPath = reportWriter.ExecutionPath()
		if r.logger != nil {
			r.logger.Debug("created new report path", "path", state.ReportPath)
		}

		// Initialize directory structure immediately (eager initialization).
		// This ensures the directory exists even if the workflow fails during validation,
		// preventing "ghost" workflows without trace in the filesystem.
		if err := reportWriter.Initialize(); err != nil {
			if r.logger != nil {
				r.logger.Warn("failed to initialize report directory",
					"workflow_id", state.WorkflowID,
					"error", err,
				)
			}
			// Don't fail the workflow for this - it's not critical for execution
		}
	}

	// In workflow isolation mode, task branches are merged into the workflow branch locally.
	// We create a single workflow-level PR to the base branch on completion (optional),
	// so per-task PRs are disabled to avoid noisy/incorrect targets.
	finalizationCfg := r.config.Finalization
	useWorkflowIsolation := r.gitIsolation != nil && r.gitIsolation.Enabled &&
		r.workflowWorktrees != nil && state != nil && state.WorkflowBranch != ""
	if useWorkflowIsolation {
		finalizationCfg.AutoPR = false
		finalizationCfg.AutoMerge = false
	}

	return &Context{
		State:             state,
		Agents:            r.agents,
		Prompts:           r.prompts,
		Checkpoint:        r.checkpoint,
		Retry:             r.retry,
		RateLimits:        r.rateLimits,
		Worktrees:         r.worktrees,
		WorkflowWorktrees: r.workflowWorktrees,
		GitIsolation:      r.gitIsolation,
		Git:               r.git,
		GitHub:            r.github,
		Logger:            r.logger,
		Output:            r.output,
		ModeEnforcer:      r.modeEnforcer,
		Control:           r.control,
		Report:            reportWriter,
		Config: &Config{
			DryRun:                 r.config.DryRun,
			DenyTools:              r.config.DenyTools,
			DefaultAgent:           r.config.DefaultAgent,
			AgentPhaseModels:       r.config.AgentPhaseModels,
			WorktreeAutoClean:      r.config.WorktreeAutoClean,
			WorktreeMode:           r.config.WorktreeMode,
			SynthesizerAgent:       r.config.Synthesizer.Agent,
			PlanSynthesizerEnabled: r.config.PlanSynthesizer.Enabled,
			PlanSynthesizerAgent:   r.config.PlanSynthesizer.Agent,
			PhaseTimeouts:          r.config.PhaseTimeouts,
			Moderator:              r.config.Moderator,
			SingleAgent:            r.config.SingleAgent,
			Finalization:           finalizationCfg,
			ProjectAgentPhases:     r.config.ProjectAgentPhases,
		},
		ProjectRoot: r.projectRoot,
	}
}

// handleAbort maps a cancellation to an aborted workflow and persists it using a
// non-cancelled context. This prevents workflows from getting stuck in "running"
// if the execution context is cancelled mid-save.
func (r *Runner) handleAbort(ctx context.Context, state *core.WorkflowState, _ error) error {
	cancelMsg := "workflow cancelled by user"
	now := time.Now()

	r.logger.Info("workflow cancelled",
		"workflow_id", state.WorkflowID,
		"phase", state.CurrentPhase,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", "Workflow cancelled.")
	}

	state.Status = core.WorkflowStatusAborted
	state.Error = cancelMsg
	state.UpdatedAt = now

	// Ensure tasks aren't left in "running" when the workflow is aborted.
	for _, ts := range state.Tasks {
		if ts == nil {
			continue
		}
		if ts.Status == core.TaskStatusRunning {
			ts.Status = core.TaskStatusFailed
			ts.Error = cancelMsg
			if ts.CompletedAt == nil {
				completedAt := now
				ts.CompletedAt = &completedAt
			}
		}
	}

	// Add an explicit checkpoint for cancellation (best-effort, persisted via Save below).
	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        fmt.Sprintf("cancel-%d", now.UnixNano()),
		Type:      "cancelled",
		Phase:     state.CurrentPhase,
		Timestamp: now,
		Message:   cancelMsg,
	})

	// Persist terminal state using a context that ignores execution cancellation.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if saveErr := r.state.Save(saveCtx, state); saveErr != nil {
		r.logger.Warn("failed to save aborted workflow state",
			"workflow_id", state.WorkflowID,
			"error", saveErr,
		)
	}

	deactCtx, deactCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer deactCancel()
	if deactErr := r.state.DeactivateWorkflow(deactCtx); deactErr != nil {
		r.logger.Warn("failed to deactivate aborted workflow",
			"workflow_id", state.WorkflowID,
			"error", deactErr,
		)
	} else {
		r.logger.Info("deactivated aborted workflow", "workflow_id", state.WorkflowID)
	}

	return workflowCancelledError()
}

// handleError handles workflow errors.
func (r *Runner) handleError(ctx context.Context, state *core.WorkflowState, err error) error {
	if isWorkflowCancelled(err) {
		return r.handleAbort(ctx, state, err)
	}

	r.logger.Error("workflow error",
		"workflow_id", state.WorkflowID,
		"phase", state.CurrentPhase,
		"error", err,
	)
	if r.output != nil {
		r.output.Log("error", "workflow", fmt.Sprintf("Workflow failed in %s: %s", state.CurrentPhase, err.Error()))
	}

	state.Status = core.WorkflowStatusFailed
	state.Error = err.Error()
	state.UpdatedAt = time.Now()
	if checkpointErr := r.checkpoint.ErrorCheckpoint(state, err); checkpointErr != nil {
		r.logger.Warn("failed to create error checkpoint", "checkpoint_error", checkpointErr)
	}

	// Persist terminal state using a context that ignores execution cancellation.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = r.state.Save(saveCtx, state)

	// Write error details to the report directory for debugging and traceability.
	r.writeErrorToReportDir(state, err)

	// Deactivate workflow when it fails to prevent ghost workflows.
	// A failed workflow should not remain as the active workflow.
	deactCtx, deactCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer deactCancel()
	if deactErr := r.state.DeactivateWorkflow(deactCtx); deactErr != nil {
		r.logger.Warn("failed to deactivate failed workflow",
			"workflow_id", state.WorkflowID,
			"error", deactErr,
		)
	} else {
		r.logger.Info("deactivated failed workflow", "workflow_id", state.WorkflowID)
	}

	return err
}

// writeErrorToReportDir writes error details to error.md in the workflow's report directory.
// This provides traceability for failed workflows, especially those that fail early.
func (r *Runner) writeErrorToReportDir(state *core.WorkflowState, err error) {
	if state.ReportPath == "" {
		return
	}

	reportDir := state.ReportPath
	if !filepath.IsAbs(reportDir) && strings.TrimSpace(r.projectRoot) != "" {
		reportDir = filepath.Join(r.projectRoot, reportDir)
	}
	if mkErr := os.MkdirAll(reportDir, 0o750); mkErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to ensure report directory for error file",
				"workflow_id", state.WorkflowID,
				"path", reportDir,
				"error", mkErr,
			)
		}
		return
	}

	errorFile := filepath.Join(reportDir, "error.md")
	content := fmt.Sprintf(`# Workflow Error

## Workflow ID
%s

## Timestamp
%s

## Phase
%s

## Error
%s

## Prompt
%s
`, state.WorkflowID, time.Now().Format(time.RFC3339), state.CurrentPhase, err.Error(), state.Prompt)

	if writeErr := os.WriteFile(errorFile, []byte(content), 0o600); writeErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to write error file",
				"workflow_id", state.WorkflowID,
				"error", writeErr,
			)
		}
	}
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
	// Validate DefaultAgent is configured (required for planning/execution)
	if r.config.DefaultAgent == "" {
		return core.ErrValidation(core.CodeInvalidConfig,
			"agents.default is not configured. "+
				"Please set 'agents.default' in your .quorum/config.yaml file. "+
				"Run 'quorum init' to generate a complete configuration.")
	}
	return nil
}

// ValidateAgentAvailability checks that all configured agents have their CLIs installed.
// This runs early to fail fast with a clear error message before any phase starts.
func (r *Runner) ValidateAgentAvailability(ctx context.Context) error {
	configured := r.agents.ListEnabled()
	available := r.agents.Available(ctx)

	// Build set of available agents
	availableSet := make(map[string]bool)
	for _, name := range available {
		availableSet[strings.ToLower(name)] = true
	}

	// Check which configured agents are not available
	var missing []string
	for _, name := range configured {
		if !availableSet[strings.ToLower(name)] {
			missing = append(missing, name)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	// Build helpful error message
	if len(missing) == 1 {
		return fmt.Errorf("agent %q is configured but CLI is not installed or not responding.\n\n"+
			"To fix this:\n"+
			"  1. Install the %s CLI (see documentation)\n"+
			"  2. Or disable this agent in .quorum/config.yaml:\n"+
			"     agents:\n"+
			"       %s:\n"+
			"         enabled: false\n\n"+
			"Run 'quorum doctor' for detailed diagnostics.",
			missing[0], missing[0], missing[0])
	}

	return fmt.Errorf("multiple agents are configured but their CLIs are not installed:\n"+
		"  Missing: %v\n\n"+
		"To fix this:\n"+
		"  1. Install the missing CLIs (see documentation)\n"+
		"  2. Or disable them in .quorum/config.yaml:\n"+
		"     agents:\n"+
		"       <agent_name>:\n"+
		"         enabled: false\n\n"+
		"Run 'quorum doctor' for detailed diagnostics.",
		missing)
}

// ValidateModeratorConfig checks if moderator is properly configured for multi-agent analysis.
// This should be called BEFORE starting the analyze phase to fail fast with a clear error.
// Returns nil if validation passes or if moderator is not needed (single agent mode or only one enabled agent).
func (r *Runner) ValidateModeratorConfig() error {
	agents := r.agents.ListEnabled()
	enabledAgents := len(agents)

	// Single agent doesn't need moderator
	if enabledAgents <= 1 {
		return nil
	}

	moderatorCfg := r.config.Moderator

	// Check if moderator is enabled
	if !moderatorCfg.Enabled {
		return fmt.Errorf("multi-agent analysis requires semantic moderator. "+
			"You have %d agents enabled but moderator is disabled.\n\n"+
			"Add this to your .quorum/config.yaml:\n\n"+
			"phases:\n"+
			"  analyze:\n"+
			"    moderator:\n"+
			"      enabled: true\n"+
			"      agent: claude\n\n"+
			"Or run 'quorum init --force' to regenerate config with defaults.\n"+
			"See: %s#phases-settings", enabledAgents, DocsConfigURL)
	}

	// Check if agent is specified
	if moderatorCfg.Agent == "" {
		return fmt.Errorf("moderator is enabled but no agent specified.\n\n"+
			"Add 'agent' to your moderator config in .quorum/config.yaml:\n\n"+
			"phases:\n"+
			"  analyze:\n"+
			"    moderator:\n"+
			"      enabled: true\n"+
			"      agent: claude  # or gemini, codex, copilot\n\n"+
			"See: %s#phases-settings", DocsConfigURL)
	}

	// Verify the specified moderator agent exists
	found := false
	for _, agent := range agents {
		if strings.EqualFold(agent, moderatorCfg.Agent) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("moderator agent %q is not available.\n\n"+
			"Available agents: %v\n\n"+
			"Either enable the %q agent or change phases.analyze.moderator.agent in .quorum/config.yaml.\n"+
			"See: %s#phases-settings", moderatorCfg.Agent, agents, moderatorCfg.Agent, DocsConfigURL)
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

	// Validate all configured agents have their CLIs installed (fail fast)
	if err := r.ValidateAgentAvailability(ctx); err != nil {
		return err
	}

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

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
	if err := r.refiner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Validate arbiter config before analyze phase (requires arbiter for multi-agent)
	if err := r.ValidateModeratorConfig(); err != nil {
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
		"consensus_score", workflowState.Metrics.ConsensusScore,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Analysis completed: consensus %.1f%%",
			workflowState.Metrics.ConsensusScore*100))
	}

	return r.state.Save(ctx, workflowState)
}

// Plan executes only the planning phase.
// This is useful when you want to generate an execution plan without executing tasks.
// Requires a completed analyze phase with consolidated analysis.
func (r *Runner) Plan(ctx context.Context) error {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Load existing state
	workflowState, err := r.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found to plan")
	}

	// Reconcile analysis artifacts: allow planning from a consolidated.md even if checkpoints are missing.
	if recErr := r.reconcileAnalysisArtifacts(ctx, workflowState); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
	}

	// Verify analyze phase completed
	analysis := GetConsolidatedAnalysis(workflowState)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found; run analyze phase first")
	}

	// Check if plan phase already completed
	if isPhaseCompleted(workflowState, core.PhasePlan) {
		r.logger.Info("plan phase already completed",
			"workflow_id", workflowState.WorkflowID)
		if r.output != nil {
			r.output.Log("info", "workflow", "Plan phase already completed, skipping")
		}
		return nil
	}

	// New phase execution attempt: bump execution id but keep prior events for history.
	r.prepareExecution(workflowState, true)

	r.logger.Info("starting plan-only workflow",
		"workflow_id", workflowState.WorkflowID,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Plan-only workflow started: %s", workflowState.WorkflowID))
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run ONLY the planner - no fallthrough to execute
	if err := r.planner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark as completed (plan phase done, ready for execute)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhaseExecute // Ready for next phase
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("plan phase completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
		"duration", workflowState.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Planning completed: %d tasks created",
			len(workflowState.Tasks)))
	}

	return r.state.Save(ctx, workflowState)
}

// PlanWithState executes only the planning phase using an existing workflow state.
// This is for API usage where the workflow state was loaded by ID before calling into the runner.
//
// After completion, Status will be "completed" and CurrentPhase will be "execute" (ready for execution).
func (r *Runner) PlanWithState(ctx context.Context, state *core.WorkflowState) error {
	if state == nil {
		return core.ErrState("NIL_STATE", "workflow state cannot be nil")
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	workflowState := state

	// Reconcile analysis artifacts: allow planning from a consolidated.md even if checkpoints are missing.
	if recErr := r.reconcileAnalysisArtifacts(ctx, workflowState); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
	}

	// Verify analyze phase completed
	analysis := GetConsolidatedAnalysis(workflowState)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found; run analyze phase first")
	}

	// Check if plan phase already completed
	if isPhaseCompleted(workflowState, core.PhasePlan) {
		r.logger.Info("plan phase already completed",
			"workflow_id", workflowState.WorkflowID)
		if r.output != nil {
			r.output.Log("info", "workflow", "Plan phase already completed, skipping")
		}
		return nil
	}

	// Ensure state maps exist
	if workflowState.Tasks == nil {
		workflowState.Tasks = make(map[core.TaskID]*core.TaskState)
	}
	if workflowState.TaskOrder == nil {
		workflowState.TaskOrder = make([]core.TaskID, 0)
	}
	if workflowState.Checkpoints == nil {
		workflowState.Checkpoints = make([]core.Checkpoint, 0)
	}
	if workflowState.Metrics == nil {
		workflowState.Metrics = &core.StateMetrics{}
	}

	// New phase execution attempt: bump execution id but keep prior events for history.
	r.prepareExecution(workflowState, true)

	r.logger.Info("starting plan phase with existing state",
		"workflow_id", workflowState.WorkflowID,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Plan phase started: %s", workflowState.WorkflowID))
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run ONLY the planner - no fallthrough to execute
	if err := r.planner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark as completed (plan phase done, ready for execute)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhaseExecute // Ready for next phase
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("plan phase completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
		"duration", workflowState.Metrics.Duration,
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Planning completed: %d tasks created",
			len(workflowState.Tasks)))
	}

	return r.state.Save(ctx, workflowState)
}

// Replan clears existing plan data and re-executes the planning phase.
// This is useful when you want to regenerate tasks with the same consolidated analysis.
// Optionally prepends additional context to the consolidated analysis.
func (r *Runner) Replan(ctx context.Context, additionalContext string) error {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Load existing state
	workflowState, err := r.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found to replan")
	}

	// Reconcile analysis artifacts: allow replan from a consolidated.md even if checkpoints are missing.
	if recErr := r.reconcileAnalysisArtifacts(ctx, workflowState); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
	}

	// Verify analyze phase completed
	analysis := GetConsolidatedAnalysis(workflowState)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found; run analyze phase first")
	}

	// If additional context provided, prepend it to the analysis
	if additionalContext != "" {
		if err := PrependToConsolidatedAnalysis(workflowState, additionalContext); err != nil {
			return fmt.Errorf("updating analysis context: %w", err)
		}
		r.logger.Info("prepended additional context to analysis",
			"context_length", len(additionalContext))
	}

	// Clear plan phase data
	r.clearPlanPhaseData(workflowState)

	// New phase execution attempt: bump execution id but keep prior events for history.
	r.prepareExecution(workflowState, true)

	r.logger.Info("starting replan workflow",
		"workflow_id", workflowState.WorkflowID,
		"previous_tasks", len(workflowState.Tasks),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", "Replanning: clearing previous plan and regenerating tasks...")
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run planner
	if err := r.planner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark as completed (plan phase done, ready for execute)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhaseExecute
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("replan completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Replanning completed: %d tasks created",
			len(workflowState.Tasks)))
	}

	return r.state.Save(ctx, workflowState)
}

// ReplanWithState clears existing plan data and re-executes the planning phase using an existing workflow state.
// This is for API usage where the workflow state was loaded by ID before calling into the runner.
func (r *Runner) ReplanWithState(ctx context.Context, state *core.WorkflowState, additionalContext string) error {
	if state == nil {
		return core.ErrState("NIL_STATE", "workflow state cannot be nil")
	}

	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	workflowState := state

	// Reconcile analysis artifacts: allow replan from a consolidated.md even if checkpoints are missing.
	if recErr := r.reconcileAnalysisArtifacts(ctx, workflowState); recErr != nil {
		if r.logger != nil {
			r.logger.Warn("failed to reconcile analysis artifacts", "error", recErr)
		}
	}

	// Verify analyze phase completed
	analysis := GetConsolidatedAnalysis(workflowState)
	if analysis == "" {
		return core.ErrState("MISSING_ANALYSIS", "no consolidated analysis found; run analyze phase first")
	}

	// If additional context provided, prepend it to the analysis
	if additionalContext != "" {
		if err := PrependToConsolidatedAnalysis(workflowState, additionalContext); err != nil {
			return fmt.Errorf("updating analysis context: %w", err)
		}
		r.logger.Info("prepended additional context to analysis",
			"context_length", len(additionalContext))
	}

	// Clear plan phase data
	r.clearPlanPhaseData(workflowState)

	// New phase execution attempt: bump execution id but keep prior events for history.
	r.prepareExecution(workflowState, true)

	r.logger.Info("starting replan phase with existing state",
		"workflow_id", workflowState.WorkflowID,
		"previous_tasks", len(workflowState.Tasks),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", "Replanning: clearing previous plan and regenerating tasks...")
	}

	// Create workflow context for phase runners
	wctx := r.createContext(workflowState)

	// Run planner
	if err := r.planner.Run(ctx, wctx); err != nil {
		return r.handleError(ctx, workflowState, err)
	}

	// Finalize metrics
	r.finalizeMetrics(workflowState)

	// Mark as completed (plan phase done, ready for execute)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhaseExecute
	workflowState.UpdatedAt = time.Now()

	r.logger.Info("replan completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
	)
	if r.output != nil {
		r.output.Log("success", "workflow", fmt.Sprintf("Replanning completed: %d tasks created",
			len(workflowState.Tasks)))
	}

	return r.state.Save(ctx, workflowState)
}

// clearPlanPhaseData removes all plan-related checkpoints and tasks from state.
func (r *Runner) clearPlanPhaseData(state *core.WorkflowState) {
	// Remove plan phase checkpoints
	var filteredCheckpoints []core.Checkpoint
	for _, cp := range state.Checkpoints {
		// Keep checkpoints that are NOT plan-related
		if cp.Phase != core.PhasePlan && cp.Type != "phase_complete" {
			filteredCheckpoints = append(filteredCheckpoints, cp)
		} else if cp.Type == "phase_complete" && cp.Phase != core.PhasePlan {
			filteredCheckpoints = append(filteredCheckpoints, cp)
		}
		// Discard plan checkpoints and plan phase_complete checkpoints
	}
	state.Checkpoints = filteredCheckpoints

	// Clear tasks
	state.Tasks = make(map[core.TaskID]*core.TaskState)
	state.TaskOrder = nil

	// Reset phase to plan
	state.CurrentPhase = core.PhasePlan
	state.Status = core.WorkflowStatusRunning

	r.logger.Info("cleared plan phase data",
		"remaining_checkpoints", len(state.Checkpoints))
}

// PrependToConsolidatedAnalysis adds context to the beginning of the consolidated analysis.
func PrependToConsolidatedAnalysis(state *core.WorkflowState, context string) error {
	// Find the consolidated analysis checkpoint
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := &state.Checkpoints[i]
		if cp.Type == "consolidated_analysis" && len(cp.Data) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(cp.Data, &metadata); err != nil {
				return fmt.Errorf("parsing analysis checkpoint: %w", err)
			}

			if content, ok := metadata["content"].(string); ok {
				// Prepend context
				newContent := context + "\n\n---\n\n" + content
				metadata["content"] = newContent

				// Re-serialize
				newData, err := json.Marshal(metadata)
				if err != nil {
					return fmt.Errorf("serializing updated analysis: %w", err)
				}
				cp.Data = newData
				return nil
			}
		}
	}
	return fmt.Errorf("no consolidated analysis checkpoint found")
}

// UsePlan generates a manifest from existing task files in the filesystem
// without re-running the planning agent. This is useful when task files
// were generated but the manifest parsing failed.
func (r *Runner) UsePlan(ctx context.Context) error {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Acquire lock
	if err := r.state.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = r.state.ReleaseLock(releaseCtx)
	}()

	// Load existing state
	workflowState, err := r.state.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if workflowState == nil {
		return core.ErrState("NO_STATE", "no workflow state found")
	}

	// Find the tasks directory
	tasksDir, err := r.findTasksDirectory(workflowState.WorkflowID)
	if err != nil {
		return fmt.Errorf("finding tasks directory: %w", err)
	}

	r.logger.Info("using existing task files",
		"workflow_id", workflowState.WorkflowID,
		"tasks_dir", tasksDir,
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Using existing task files from: %s", tasksDir))
	}

	// Generate manifest from filesystem
	manifest, err := generateManifestFromFilesystem(tasksDir)
	if err != nil {
		return fmt.Errorf("generating manifest from filesystem: %w", err)
	}

	r.logger.Info("manifest generated from filesystem",
		"tasks_count", len(manifest.Tasks),
		"levels_count", len(manifest.ExecutionLevels),
	)
	if r.output != nil {
		r.output.Log("info", "workflow", fmt.Sprintf("Found %d task files, %d execution levels",
			len(manifest.Tasks), len(manifest.ExecutionLevels)))
	}

	// Clear existing plan data
	r.clearPlanPhaseData(workflowState)

	// Create tasks from manifest and add to state
	for _, item := range manifest.Tasks {
		cli := item.CLI
		if cli == "" {
			cli = r.config.DefaultAgent
		}

		taskID := core.TaskID(item.ID)
		workflowState.Tasks[taskID] = &core.TaskState{
			ID:           taskID,
			Phase:        core.PhaseExecute,
			Name:         item.Name,
			Status:       core.TaskStatusPending,
			CLI:          cli,
			Dependencies: make([]core.TaskID, 0, len(item.Dependencies)),
		}

		for _, dep := range item.Dependencies {
			workflowState.Tasks[taskID].Dependencies = append(
				workflowState.Tasks[taskID].Dependencies,
				core.TaskID(dep),
			)
		}

		workflowState.TaskOrder = append(workflowState.TaskOrder, taskID)
	}

	// Mark as completed (plan phase done, ready for execute)
	workflowState.Status = core.WorkflowStatusCompleted
	workflowState.CurrentPhase = core.PhaseExecute
	workflowState.UpdatedAt = time.Now()

	// Create phase_complete checkpoint for plan phase
	// This is critical: without it, Resume() will think plan hasn't completed
	// and will try to run the planner again, causing duplicate task errors
	if err := r.checkpoint.PhaseCheckpoint(workflowState, core.PhasePlan, true); err != nil {
		r.logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	// Notify output
	if r.output != nil {
		r.output.WorkflowStateUpdated(workflowState)
		r.output.Log("success", "workflow", fmt.Sprintf("Plan loaded: %d tasks ready for execution",
			len(workflowState.Tasks)))
	}

	r.logger.Info("useplan completed",
		"workflow_id", workflowState.WorkflowID,
		"task_count", len(workflowState.Tasks),
	)

	return r.state.Save(ctx, workflowState)
}

// findTasksDirectory searches for the tasks directory for a given workflow.
// It looks in .quorum/runs/ for directories matching the workflow ID pattern.
func (r *Runner) findTasksDirectory(workflowID core.WorkflowID) (string, error) {
	// Use project root for resolving relative paths
	baseDir := r.projectRoot
	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}

	outputDir := filepath.Join(baseDir, ".quorum/runs")

	// List directories in output
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return "", fmt.Errorf("reading output directory: %w", err)
	}

	// Find directories that contain the workflow ID
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.Contains(entry.Name(), string(workflowID)) {
			tasksDir := filepath.Join(outputDir, entry.Name(), "plan-phase", "tasks")
			if _, err := os.Stat(tasksDir); err == nil {
				candidates = append(candidates, tasksDir)
			}
		}
	}

	if len(candidates) == 0 {
		// Try fallback: .quorum/tasks
		fallback := filepath.Join(baseDir, ".quorum", "tasks")
		if _, err := os.Stat(fallback); err == nil {
			return fallback, nil
		}
		return "", fmt.Errorf("no tasks directory found for workflow %s", workflowID)
	}

	// Sort by name (which includes timestamp) and return the most recent
	sort.Strings(candidates)
	return candidates[len(candidates)-1], nil
}

// GetState returns the current workflow state.
func (r *Runner) GetState(ctx context.Context) (*core.WorkflowState, error) {
	return r.state.Load(ctx)
}

// SaveState saves the workflow state.
func (r *Runner) SaveState(ctx context.Context, state *core.WorkflowState) error {
	return r.state.Save(ctx, state)
}

// ListWorkflows returns summaries of all available workflows.
func (r *Runner) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	// Check if state manager supports listing
	type workflowLister interface {
		ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error)
	}
	if lister, ok := r.state.(workflowLister); ok {
		return lister.ListWorkflows(ctx)
	}
	// Fallback: return single workflow if available
	state, err := r.state.Load(ctx)
	if err != nil || state == nil {
		return nil, err
	}
	return []core.WorkflowSummary{{
		WorkflowID:   state.WorkflowID,
		Status:       state.Status,
		CurrentPhase: state.CurrentPhase,
		Prompt:       state.Prompt,
		CreatedAt:    state.CreatedAt,
		UpdatedAt:    state.UpdatedAt,
		IsActive:     true,
	}}, nil
}

// LoadWorkflow loads a specific workflow by ID and sets it as active.
// Returns the loaded workflow state.
func (r *Runner) LoadWorkflow(ctx context.Context, workflowID string) (*core.WorkflowState, error) {
	// Load the workflow by ID
	state, err := r.state.LoadByID(ctx, core.WorkflowID(workflowID))
	if err != nil {
		return nil, fmt.Errorf("loading workflow %s: %w", workflowID, err)
	}
	if state == nil {
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	// Set it as active
	type activeWorkflowSetter interface {
		SetActiveWorkflowID(ctx context.Context, id core.WorkflowID) error
	}
	if setter, ok := r.state.(activeWorkflowSetter); ok {
		if err := setter.SetActiveWorkflowID(ctx, core.WorkflowID(workflowID)); err != nil {
			return nil, fmt.Errorf("setting active workflow: %w", err)
		}
	}

	return state, nil
}

// SetDryRun enables or disables dry-run mode.
func (r *Runner) SetDryRun(enabled bool) {
	r.config.DryRun = enabled
}

// generateWorkflowID generates a unique workflow ID.
// Format: wf-YYYYMMDD-HHMMSS-xxxxx (e.g., wf-20250121-153045-k7m9p)
// Uses UTC for consistency and a random suffix for uniqueness.
func generateWorkflowID() string {
	now := time.Now().UTC()
	return fmt.Sprintf("wf-%s-%s", now.Format("20060102-150405"), randomSuffix(5))
}

// randomSuffix generates a random alphanumeric suffix of the given length.
// Uses base36 (0-9, a-z) for URL-safe, human-readable identifiers.
func randomSuffix(length int) string {
	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp nanoseconds if crypto/rand fails
		return fmt.Sprintf("%05d", time.Now().UnixNano()%100000)
	}
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
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

// DeactivateWorkflow clears the active workflow without deleting any data.
func (r *Runner) DeactivateWorkflow(ctx context.Context) error {
	return r.state.DeactivateWorkflow(ctx)
}

// ArchiveWorkflows moves completed workflows to an archive location.
// Returns the number of workflows archived.
func (r *Runner) ArchiveWorkflows(ctx context.Context) (int, error) {
	return r.state.ArchiveWorkflows(ctx)
}

// PurgeAllWorkflows deletes all workflow data permanently.
// Returns the number of workflows deleted.
func (r *Runner) PurgeAllWorkflows(ctx context.Context) (int, error) {
	return r.state.PurgeAllWorkflows(ctx)
}

// DeleteWorkflow deletes a single workflow by ID.
// Returns error if workflow does not exist.
func (r *Runner) DeleteWorkflow(ctx context.Context, workflowID string) error {
	return r.state.DeleteWorkflow(ctx, core.WorkflowID(workflowID))
}
