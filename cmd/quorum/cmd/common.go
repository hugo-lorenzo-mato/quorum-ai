package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
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
	StateManager      core.StateManager
	StateAdapter      workflow.StateManager
	Registry          *cli.Registry
	ModeratorConfig   workflow.ModeratorConfig
	CheckpointAdapter *workflow.CheckpointAdapter
	RetryAdapter      *workflow.RetryAdapter
	RateLimiterAdapt  *workflow.RateLimiterRegistryAdapter
	PromptAdapter     *workflow.PromptRendererAdapter
	ResumeAdapter     *workflow.ResumePointAdapter
	DAGAdapter        *workflow.DAGAdapter
	WorktreeManager   workflow.WorktreeManager
	GitClientFactory  workflow.GitClientFactory
	GitClient         core.GitClient
	GitHubClient      core.GitHubClient
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

	// Migrate state from legacy paths if needed (only for JSON backend)
	backend := cfg.State.EffectiveBackend()
	if backend == "json" {
		if migrated, err := state.MigrateState(statePath, logger); err != nil {
			logger.Warn("state migration failed", "error", err)
		} else if migrated {
			logger.Info("migrated state from legacy path to", "path", statePath)
		}
	}

	// Parse lock TTL from config
	stateOpts := state.StateManagerOptions{}
	if cfg.State.LockTTL != "" {
		if lockTTL, err := time.ParseDuration(cfg.State.LockTTL); err == nil {
			stateOpts.LockTTL = lockTTL
		} else {
			logger.Warn("invalid state.lock_ttl, using default", "value", cfg.State.LockTTL, "error", err)
		}
	}

	stateManager, err := state.NewStateManagerWithOptions(backend, statePath, stateOpts)
	if err != nil {
		return nil, fmt.Errorf("creating state manager: %w", err)
	}

	// Create agent registry and configure from unified config
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return nil, fmt.Errorf("configuring agents: %w", err)
	}

	// Create moderator config from unified config
	moderatorConfig := workflow.ModeratorConfig{
		Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
		Agent:               cfg.Phases.Analyze.Moderator.Agent,
		Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
		MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
		MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
		AbortThreshold:      cfg.Phases.Analyze.Moderator.AbortThreshold,
		StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
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

	phaseTimeoutStr := phaseTimeoutValue(&cfg.Phases, phase)
	phaseTimeout, err := parseDurationDefault(phaseTimeoutStr, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing %s phase timeout %q: %w", strings.ToLower(string(phase)), phaseTimeoutStr, err)
	}

	// Parse all phase timeouts for passing to workflow runner
	analyzeTimeout, err := parseDurationDefault(cfg.Phases.Analyze.Timeout, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing analyze phase timeout %q: %w", cfg.Phases.Analyze.Timeout, err)
	}
	planTimeout, err := parseDurationDefault(cfg.Phases.Plan.Timeout, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing plan phase timeout %q: %w", cfg.Phases.Plan.Timeout, err)
	}
	executeTimeout, err := parseDurationDefault(cfg.Phases.Execute.Timeout, defaultPhaseTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing execute phase timeout %q: %w", cfg.Phases.Execute.Timeout, err)
	}

	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}

	// Use provided values or fall back to config, then default
	if maxRetries == 0 {
		maxRetries = cfg.Workflow.MaxRetries
		if maxRetries == 0 {
			maxRetries = 3
		}
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
		WorktreeAutoClean: cfg.Git.AutoClean,
		WorktreeMode:      cfg.Git.WorktreeMode,
		// Refiner disabled by default for independent phase runners
		// (only enabled when running full workflow via `run` command)
		Refiner: workflow.RefinerConfig{
			Enabled: false,
			Agent:   cfg.Phases.Analyze.Refiner.Agent,
		},
		Synthesizer: workflow.SynthesizerConfig{
			Agent: cfg.Phases.Analyze.Synthesizer.Agent,
		},
		Moderator: workflow.ModeratorConfig{
			Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
			Agent:               cfg.Phases.Analyze.Moderator.Agent,
			Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
			MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
			AbortThreshold:      cfg.Phases.Analyze.Moderator.AbortThreshold,
			StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
		},
		SingleAgent: workflow.SingleAgentConfig{
			Enabled: cfg.Phases.Analyze.SingleAgent.Enabled,
			Agent:   cfg.Phases.Analyze.SingleAgent.Agent,
			Model:   cfg.Phases.Analyze.SingleAgent.Model,
		},
		PhaseTimeouts: workflow.PhaseTimeouts{
			Analyze: analyzeTimeout,
			Plan:    planTimeout,
			Execute: executeTimeout,
		},
		Finalization: workflow.FinalizationConfig{
			AutoCommit:    cfg.Git.AutoCommit,
			AutoPush:      cfg.Git.AutoPush,
			AutoPR:        cfg.Git.AutoPR,
			AutoMerge:     cfg.Git.AutoMerge,
			PRBaseBranch:  cfg.Git.PRBaseBranch,
			MergeStrategy: cfg.Git.MergeStrategy,
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

	// Create git client factory for task finalization (commit, push, PR)
	gitClientFactory := git.NewClientFactory()

	// Create GitHub client for PR creation (only if auto_pr is enabled)
	var githubClient core.GitHubClient
	if cfg.Git.AutoPR {
		ghClient, ghErr := github.NewClientFromRepo()
		if ghErr != nil {
			logger.Warn("failed to create GitHub client, PR creation disabled", "error", ghErr)
		} else {
			githubClient = ghClient
			logger.Info("GitHub client initialized for PR creation")
		}
	}

	// Create adapters for modular runner interfaces
	checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx)
	promptAdapter := workflow.NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := workflow.NewResumePointAdapter(checkpointManager)
	dagAdapter := workflow.NewDAGAdapter(dagBuilder)

	// core.StateManager satisfies workflow.StateManager interface
	stateAdapter := stateManager

	return &PhaseRunnerDeps{
		Config:            cfg,
		Logger:            logger,
		StateManager:      stateManager,
		StateAdapter:      stateAdapter,
		Registry:          registry,
		ModeratorConfig:   moderatorConfig,
		CheckpointAdapter: checkpointAdapter,
		RetryAdapter:      retryAdapter,
		RateLimiterAdapt:  rateLimiterAdapter,
		PromptAdapter:     promptAdapter,
		ResumeAdapter:     resumeAdapter,
		DAGAdapter:        dagAdapter,
		WorktreeManager:   worktreeManager,
		GitClientFactory:  gitClientFactory,
		GitClient:         gitClient,
		GitHubClient:      githubClient,
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
		Git:        deps.GitClient,
		GitHub:     deps.GitHubClient,
		Logger:     deps.Logger,
		Config: &workflow.Config{
			DryRun:            deps.RunnerConfig.DryRun,
			Sandbox:           deps.RunnerConfig.Sandbox,
			DenyTools:         deps.RunnerConfig.DenyTools,
			DefaultAgent:      deps.RunnerConfig.DefaultAgent,
			AgentPhaseModels:  deps.RunnerConfig.AgentPhaseModels,
			WorktreeAutoClean: deps.RunnerConfig.WorktreeAutoClean,
			WorktreeMode:      deps.RunnerConfig.WorktreeMode,
			PhaseTimeouts:     deps.RunnerConfig.PhaseTimeouts,
			Moderator:         deps.ModeratorConfig,
			Finalization:      deps.RunnerConfig.Finalization,
		},
	}
}

// InitializeWorkflowState creates a new workflow state for a fresh run.
func InitializeWorkflowState(prompt string) *core.WorkflowState {
	return &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   core.WorkflowID(generateCmdWorkflowID()),
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseRefine,
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

// initStateManager initializes the state manager using the current configuration.
func initStateManager() (core.StateManager, error) {
	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	statePath := cfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	backend := cfg.State.EffectiveBackend()

	return state.NewStateManager(backend, statePath)
}

// OutputJSON writes the given value to stdout as indented JSON.
func OutputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// TruncateString removes newlines and truncates the string to maxLen.
func TruncateString(s string, maxLen int) string {
	// Remove newlines
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

func phaseTimeoutValue(cfg *config.PhasesConfig, phase core.Phase) string {
	switch phase {
	case core.PhaseAnalyze:
		return cfg.Analyze.Timeout
	case core.PhasePlan:
		return cfg.Plan.Timeout
	case core.PhaseExecute:
		return cfg.Execute.Timeout
	default:
		return ""
	}
}
