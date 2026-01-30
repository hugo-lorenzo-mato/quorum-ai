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

var (
	// Shared single-agent mode flags
	singleAgent bool   // --single-agent
	agentName   string // --agent
	agentModel  string // --model
)

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
	WorkflowWorktrees core.WorkflowWorktreeManager
	GitIsolation      *workflow.GitIsolationConfig
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
		SingleAgent: buildSingleAgentConfig(cfg),
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
			Remote:        cfg.GitHub.Remote,
		},
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(maxRetries))
	rateLimiterRegistry := service.GetGlobalRateLimiter()
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
	var workflowWorktrees core.WorkflowWorktreeManager
	if gitClient != nil {
		worktreeManager = git.NewTaskWorktreeManager(gitClient, cfg.Git.WorktreeDir).WithLogger(logger)

		repoRoot, rootErr := gitClient.RepoRoot(ctx)
		if rootErr != nil {
			logger.Warn("failed to detect repo root, workflow git isolation disabled", "error", rootErr)
		} else {
			wtMgr, wtErr := git.NewWorkflowWorktreeManager(repoRoot, cfg.Git.WorktreeDir, gitClient, logger.Logger)
			if wtErr != nil {
				logger.Warn("failed to create workflow worktree manager, workflow git isolation disabled", "error", wtErr)
			} else {
				workflowWorktrees = wtMgr
			}
		}
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
		WorkflowWorktrees: workflowWorktrees,
		GitIsolation:      workflow.DefaultGitIsolationConfig(),
		GitClientFactory:  gitClientFactory,
		GitClient:         gitClient,
		GitHubClient:      githubClient,
		RunnerConfig:      runnerConfig,
		PhaseTimeout:      phaseTimeout,
	}, nil
}

// CreateWorkflowContext creates a workflow context from dependencies and state.
func CreateWorkflowContext(deps *PhaseRunnerDeps, state *core.WorkflowState) *workflow.Context {
	finalizationCfg := deps.RunnerConfig.Finalization
	useWorkflowIsolation := deps.GitIsolation != nil && deps.GitIsolation.Enabled &&
		deps.WorkflowWorktrees != nil && state != nil && state.WorkflowBranch != ""
	if useWorkflowIsolation {
		finalizationCfg.AutoPR = false
		finalizationCfg.AutoMerge = false
	}

	return &workflow.Context{
		State:      state,
		Agents:     deps.Registry,
		Prompts:    deps.PromptAdapter,
		Checkpoint: deps.CheckpointAdapter,
		Retry:      deps.RetryAdapter,
		RateLimits: deps.RateLimiterAdapt,
		Worktrees:  deps.WorktreeManager,
		WorkflowWorktrees: deps.WorkflowWorktrees,
		GitIsolation:      deps.GitIsolation,
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
			Finalization:      finalizationCfg,
		},
	}
}

// EnsureWorkflowGitIsolation initializes workflow-level git isolation for phase-based commands.
// It is intentionally conservative: it will not enable isolation mid-workflow if there is evidence
// that tasks already ran without a workflow branch.
func EnsureWorkflowGitIsolation(ctx context.Context, deps *PhaseRunnerDeps, state *core.WorkflowState) (bool, error) {
	if deps == nil || state == nil || state.WorkflowID == "" {
		return false, nil
	}
	if deps.RunnerConfig != nil && deps.RunnerConfig.DryRun {
		return false, nil
	}
	if deps.GitIsolation == nil || !deps.GitIsolation.Enabled {
		return false, nil
	}
	if deps.WorkflowWorktrees == nil {
		return false, nil
	}
	if state.WorkflowBranch != "" {
		return false, nil
	}

	// Safety: if tasks already executed (or have git artifacts), don't switch modes.
	for _, ts := range state.Tasks {
		if ts == nil {
			continue
		}
		if ts.Status != "" && ts.Status != core.TaskStatusPending {
			return false, nil
		}
		if ts.Branch != "" || ts.LastCommit != "" || ts.WorktreePath != "" {
			return false, nil
		}
	}

	info, err := deps.WorkflowWorktrees.InitializeWorkflow(ctx, string(state.WorkflowID), deps.GitIsolation.BaseBranch)
	if err != nil {
		return false, err
	}
	if info == nil || info.WorkflowBranch == "" {
		return false, fmt.Errorf("workflow isolation init returned empty branch")
	}

	state.WorkflowBranch = info.WorkflowBranch
	return true, nil
}

// InitializeWorkflowState creates a new workflow state for a fresh run.
func InitializeWorkflowState(prompt string, config *core.WorkflowConfig) *core.WorkflowState {
	return &core.WorkflowState{
		Version:      core.CurrentStateVersion,
		WorkflowID:   core.WorkflowID(generateCmdWorkflowID()),
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseRefine,
		Prompt:       prompt,
		Tasks:        make(map[core.TaskID]*core.TaskState),
		TaskOrder:    make([]core.TaskID, 0),
		Config:       config,
		Checkpoints:  make([]core.Checkpoint, 0),
		Metrics:      &core.StateMetrics{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// buildCoreWorkflowConfig creates a core.WorkflowConfig from RunnerConfig.
func buildCoreWorkflowConfig(runnerCfg *workflow.RunnerConfig) *core.WorkflowConfig {
	mode := "multi_agent"
	if runnerCfg.SingleAgent.Enabled {
		mode = "single_agent"
	}

	return &core.WorkflowConfig{
		ConsensusThreshold: runnerCfg.Moderator.Threshold,
		MaxRetries:         runnerCfg.MaxRetries,
		Timeout:            runnerCfg.Timeout,
		DryRun:             runnerCfg.DryRun,
		Sandbox:            runnerCfg.Sandbox,
		ExecutionMode:      mode,
		SingleAgentName:    runnerCfg.SingleAgent.Agent,
		SingleAgentModel:   runnerCfg.SingleAgent.Model,
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

// buildSingleAgentConfig creates the SingleAgentConfig with CLI flag override.
// CLI flags take precedence over config file settings.
func buildSingleAgentConfig(cfg *config.Config) workflow.SingleAgentConfig {
	// CLI flags override config file
	if singleAgent {
		return workflow.SingleAgentConfig{
			Enabled: true,
			Agent:   agentName,
			Model:   agentModel,
		}
	}

	// Fall back to config file settings
	return workflow.SingleAgentConfig{
		Enabled: cfg.Phases.Analyze.SingleAgent.Enabled,
		Agent:   cfg.Phases.Analyze.SingleAgent.Agent,
		Model:   cfg.Phases.Analyze.SingleAgent.Model,
	}
}

// validateSingleAgentFlags validates the single-agent mode flag combinations.
func validateSingleAgentFlags() error {
	if singleAgent {
		if agentName == "" {
			return fmt.Errorf("--agent is required when using --single-agent flag\n" +
				"Example: quorum run \"Fix bug\" --single-agent --agent claude")
		}
	} else {
		// If not in single-agent mode, these flags shouldn't be used
		if agentName != "" {
			return fmt.Errorf("--agent requires --single-agent flag\n" +
				"Example: quorum run \"Fix bug\" --single-agent --agent claude")
		}
		if agentModel != "" {
			return fmt.Errorf("--model requires --single-agent flag\n" +
				"Example: quorum run \"Fix bug\" --single-agent --agent claude --model claude-3-haiku")
		}
	}
	return nil
}
