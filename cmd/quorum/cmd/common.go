package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// cmdIDCounter provides additional uniqueness for workflow IDs generated from cmd.
var cmdIDCounter uint64

const (
	defaultWorkflowTimeout = 12 * time.Hour
	defaultPhaseTimeout    = 2 * time.Hour
)

// PhaseRunnerDeps holds all dependencies needed for running workflow phases.
type PhaseRunnerDeps struct {
	Config            *config.Config
	Logger            *logging.Logger
	StateManager      *state.JSONStateManager
	StateAdapter      workflow.StateManager
	Registry          *cli.Registry
	ArbiterConfig     workflow.ArbiterConfig
	CheckpointAdapter *workflow.CheckpointAdapter
	RetryAdapter      *workflow.RetryAdapter
	RateLimiterAdapt  *workflow.RateLimiterRegistryAdapter
	PromptAdapter     *workflow.PromptRendererAdapter
	ResumeAdapter     *workflow.ResumePointAdapter
	DAGAdapter        *workflow.DAGAdapter
	WorktreeManager   workflow.WorktreeManager
	RunnerConfig      *workflow.RunnerConfig
	PhaseTimeout      time.Duration
}

// InitPhaseRunner initializes all dependencies needed for running individual phases.
// This extracts the common initialization logic from run.go to be reused by
// analyze, plan, and execute commands.
func InitPhaseRunner(ctx context.Context, phase core.Phase, maxRetries int, dryRun, sandbox bool) (*PhaseRunnerDeps, error) {
	// Load unified configuration using global viper (includes flag bindings)
	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	// Create logger from unified config
	logger := logging.New(logging.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
	})

	// Create state manager from unified config
	statePath := cfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}

	// Migrate state from legacy paths if needed
	if migrated, err := state.MigrateState(statePath, logger); err != nil {
		logger.Warn("state migration failed", "error", err)
	} else if migrated {
		logger.Info("migrated state from legacy path to", "path", statePath)
	}

	stateManager := state.NewJSONStateManager(statePath)

	// Create agent registry and configure from unified config
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return nil, fmt.Errorf("configuring agents: %w", err)
	}

	// Create arbiter config from unified config
	arbiterConfig := workflow.ArbiterConfig{
		Enabled:             cfg.Consensus.Arbiter.Enabled,
		Agent:               cfg.Consensus.Arbiter.Agent,
		Model:               cfg.Consensus.Arbiter.Model,
		Threshold:           cfg.Consensus.Arbiter.Threshold,
		MinRounds:           cfg.Consensus.Arbiter.MinRounds,
		MaxRounds:           cfg.Consensus.Arbiter.MaxRounds,
		AbortThreshold:      cfg.Consensus.Arbiter.AbortThreshold,
		StagnationThreshold: cfg.Consensus.Arbiter.StagnationThreshold,
	}

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, fmt.Errorf("creating prompt renderer: %w", err)
	}

	// Determine workflow timeout
	timeout, err := parseDurationDefault(cfg.Workflow.Timeout, defaultWorkflowTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow timeout %q: %w", cfg.Workflow.Timeout, err)
	}

	phaseTimeoutStr := phaseTimeoutValue(cfg.Workflow.PhaseTimeouts, phase)
	phaseTimeout, err := parseDurationDefault(phaseTimeoutStr, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing %s phase timeout %q: %w", strings.ToLower(string(phase)), phaseTimeoutStr, err)
	}

	// Parse all phase timeouts for passing to workflow runner
	analyzeTimeout, err := parseDurationDefault(cfg.Workflow.PhaseTimeouts.Analyze, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing analyze phase timeout %q: %w", cfg.Workflow.PhaseTimeouts.Analyze, err)
	}
	planTimeout, err := parseDurationDefault(cfg.Workflow.PhaseTimeouts.Plan, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing plan phase timeout %q: %w", cfg.Workflow.PhaseTimeouts.Plan, err)
	}
	executeTimeout, err := parseDurationDefault(cfg.Workflow.PhaseTimeouts.Execute, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing execute phase timeout %q: %w", cfg.Workflow.PhaseTimeouts.Execute, err)
	}

	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}

	// Use provided values or fall back to config
	if maxRetries == 0 {
		maxRetries = 3
	}

	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   maxRetries,
		DryRun:       dryRun,
		Sandbox:      sandbox || cfg.Workflow.Sandbox,
		DenyTools:    cfg.Workflow.DenyTools,
		DefaultAgent: defaultAgent,
		AgentPhaseModels: map[string]map[string]string{
			"claude":  cfg.Agents.Claude.PhaseModels,
			"gemini":  cfg.Agents.Gemini.PhaseModels,
			"codex":   cfg.Agents.Codex.PhaseModels,
			"copilot": cfg.Agents.Copilot.PhaseModels,
		},
		WorktreeAutoClean:  cfg.Git.AutoClean,
		WorktreeMode:       cfg.Git.WorktreeMode,
		MaxCostPerWorkflow: cfg.Costs.MaxPerWorkflow,
		MaxCostPerTask:     cfg.Costs.MaxPerTask,
		// Optimizer disabled by default for independent phase runners
		// (only enabled when running full workflow via `run` command)
		Optimizer: workflow.OptimizerConfig{
			Enabled: false,
			Agent:   cfg.PromptOptimizer.Agent,
			Model:   cfg.PromptOptimizer.Model,
		},
		PhaseTimeouts: workflow.PhaseTimeouts{
			Analyze: analyzeTimeout,
			Plan:    planTimeout,
			Execute: executeTimeout,
		},
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(maxRetries))
	rateLimiterRegistry := service.NewRateLimiterRegistry()
	dagBuilder := service.NewDAGBuilder()

	// Create worktree manager for task isolation
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	gitClient, err := git.NewClient(cwd)
	if err != nil {
		logger.Warn("failed to create git client, worktree isolation disabled", "error", err)
	}

	var worktreeManager workflow.WorktreeManager
	if gitClient != nil {
		worktreeManager = git.NewTaskWorktreeManager(gitClient, cfg.Git.WorktreeDir).WithLogger(logger)
	}

	// Create adapters for modular runner interfaces
	checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx)
	promptAdapter := workflow.NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := workflow.NewResumePointAdapter(checkpointManager)
	dagAdapter := workflow.NewDAGAdapter(dagBuilder)

	// Create state manager adapter
	stateAdapter := &stateManagerAdapter{sm: stateManager}

	return &PhaseRunnerDeps{
		Config:            cfg,
		Logger:            logger,
		StateManager:      stateManager,
		StateAdapter:      stateAdapter,
		Registry:          registry,
		ArbiterConfig:     arbiterConfig,
		CheckpointAdapter: checkpointAdapter,
		RetryAdapter:      retryAdapter,
		RateLimiterAdapt:  rateLimiterAdapter,
		PromptAdapter:     promptAdapter,
		ResumeAdapter:     resumeAdapter,
		DAGAdapter:        dagAdapter,
		WorktreeManager:   worktreeManager,
		RunnerConfig:      runnerConfig,
		PhaseTimeout:      phaseTimeout,
	}, nil
}

// CreateWorkflowContext creates a workflow context from dependencies and state.
func CreateWorkflowContext(deps *PhaseRunnerDeps, state *core.WorkflowState) *workflow.Context {
	return &workflow.Context{
		State:      state,
		Agents:     deps.Registry,
		Prompts:    deps.PromptAdapter,
		Checkpoint: deps.CheckpointAdapter,
		Retry:      deps.RetryAdapter,
		RateLimits: deps.RateLimiterAdapt,
		Worktrees:  deps.WorktreeManager,
		Logger:     deps.Logger,
		Config: &workflow.Config{
			DryRun:             deps.RunnerConfig.DryRun,
			Sandbox:            deps.RunnerConfig.Sandbox,
			DenyTools:          deps.RunnerConfig.DenyTools,
			DefaultAgent:       deps.RunnerConfig.DefaultAgent,
			AgentPhaseModels:   deps.RunnerConfig.AgentPhaseModels,
			WorktreeAutoClean:  deps.RunnerConfig.WorktreeAutoClean,
			WorktreeMode:       deps.RunnerConfig.WorktreeMode,
			MaxCostPerWorkflow: deps.RunnerConfig.MaxCostPerWorkflow,
			MaxCostPerTask:     deps.RunnerConfig.MaxCostPerTask,
			PhaseTimeouts:      deps.RunnerConfig.PhaseTimeouts,
			Arbiter:            deps.ArbiterConfig,
		},
	}
}

// InitializeWorkflowState creates a new workflow state for a fresh run.
func InitializeWorkflowState(prompt string) *core.WorkflowState {
	return &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   core.WorkflowID(generateCmdWorkflowID()),
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

// generateCmdWorkflowID generates a unique workflow ID.
func generateCmdWorkflowID() string {
	counter := atomic.AddUint64(&cmdIDCounter, 1)
	return fmt.Sprintf("wf-%d-%d", time.Now().UnixNano(), counter)
}

func parseDurationDefault(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	return d, nil
}

func phaseTimeoutValue(cfg config.PhaseTimeoutConfig, phase core.Phase) string {
	switch phase {
	case core.PhaseAnalyze:
		return cfg.Analyze
	case core.PhasePlan:
		return cfg.Plan
	case core.PhaseExecute:
		return cfg.Execute
	default:
		return ""
	}
}
