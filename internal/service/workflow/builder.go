// Package workflow provides the workflow orchestration components.
package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// RunnerBuilder provides a fluent API for constructing workflow runners.
// It unifies the construction logic previously duplicated between CLI and WebUI.
type RunnerBuilder struct {
	// Required dependencies
	config        *config.Config
	stateManager  core.StateManager
	agentRegistry core.AgentRegistry

	// Optional dependencies
	rateLimiter      *service.RateLimiterRegistry
	outputNotifier   OutputNotifier
	controlPlane     *control.ControlPlane
	heartbeat        *HeartbeatManager
	logger           *logging.Logger
	slogger          *slog.Logger
	modeEnforcer     ModeEnforcerInterface
	gitClient        core.GitClient
	githubClient     core.GitHubClient
	gitClientFactory GitClientFactory
	worktreeManager  WorktreeManager

	// Git isolation configuration
	gitIsolation *GitIsolationConfig

	// Runner configuration
	runnerConfig *RunnerConfig

	// Overrides for runner config fields
	phase      *core.Phase
	maxRetries *int
	dryRun     *bool
	sandbox    *bool

	// Disable auto-creation of git components
	skipGitAutoCreate bool

	// Error tracking
	errors []error
}

// GitIsolationConfig holds Git isolation settings for workflow execution.
type GitIsolationConfig struct {
	Enabled       bool
	BaseBranch    string
	MergeStrategy string
	AutoMerge     bool
}

// DefaultGitIsolationConfig returns sensible defaults for Git isolation.
func DefaultGitIsolationConfig() *GitIsolationConfig {
	return &GitIsolationConfig{
		Enabled:       true,
		BaseBranch:    "", // Will be auto-detected
		MergeStrategy: "sequential",
		AutoMerge:     false,
	}
}

// NewRunnerBuilder creates a new RunnerBuilder with defaults.
func NewRunnerBuilder() *RunnerBuilder {
	return &RunnerBuilder{
		runnerConfig: DefaultRunnerConfig(),
		errors:       make([]error, 0),
	}
}

// WithConfig sets the application configuration.
func (b *RunnerBuilder) WithConfig(cfg *config.Config) *RunnerBuilder {
	if cfg == nil {
		b.errors = append(b.errors, fmt.Errorf("config cannot be nil"))
		return b
	}
	b.config = cfg
	return b
}

// WithStateManager sets the state manager.
func (b *RunnerBuilder) WithStateManager(sm core.StateManager) *RunnerBuilder {
	if sm == nil {
		b.errors = append(b.errors, fmt.Errorf("state manager cannot be nil"))
		return b
	}
	b.stateManager = sm
	return b
}

// WithAgentRegistry sets the agent registry.
func (b *RunnerBuilder) WithAgentRegistry(ar core.AgentRegistry) *RunnerBuilder {
	if ar == nil {
		b.errors = append(b.errors, fmt.Errorf("agent registry cannot be nil"))
		return b
	}
	b.agentRegistry = ar
	return b
}

// WithSharedRateLimiter sets the shared rate limiter registry.
func (b *RunnerBuilder) WithSharedRateLimiter(rl *service.RateLimiterRegistry) *RunnerBuilder {
	b.rateLimiter = rl
	return b
}

// WithOutputNotifier sets the output notifier for workflow events.
func (b *RunnerBuilder) WithOutputNotifier(on OutputNotifier) *RunnerBuilder {
	b.outputNotifier = on
	return b
}

// WithControlPlane sets the control plane for pause/resume/cancel.
func (b *RunnerBuilder) WithControlPlane(cp *control.ControlPlane) *RunnerBuilder {
	b.controlPlane = cp
	return b
}

// WithHeartbeat sets the heartbeat manager for zombie detection.
func (b *RunnerBuilder) WithHeartbeat(hb *HeartbeatManager) *RunnerBuilder {
	b.heartbeat = hb
	return b
}

// WithLogger sets the logging.Logger for workflow logging.
func (b *RunnerBuilder) WithLogger(logger *logging.Logger) *RunnerBuilder {
	b.logger = logger
	return b
}

// WithSlogLogger sets an slog.Logger for workflow logging.
// This is converted to logging.Logger internally.
func (b *RunnerBuilder) WithSlogLogger(logger *slog.Logger) *RunnerBuilder {
	b.slogger = logger
	return b
}

// WithGitIsolation sets the Git isolation configuration.
func (b *RunnerBuilder) WithGitIsolation(gi *GitIsolationConfig) *RunnerBuilder {
	b.gitIsolation = gi
	return b
}

// WithRunnerConfig sets the runner configuration directly.
// This overrides any previously set runner config.
func (b *RunnerBuilder) WithRunnerConfig(rc *RunnerConfig) *RunnerBuilder {
	if rc == nil {
		b.errors = append(b.errors, fmt.Errorf("runner config cannot be nil"))
		return b
	}
	b.runnerConfig = rc
	return b
}

// WithPhase sets the starting phase.
func (b *RunnerBuilder) WithPhase(phase core.Phase) *RunnerBuilder {
	b.phase = &phase
	return b
}

// WithDryRun enables or disables dry run mode.
func (b *RunnerBuilder) WithDryRun(dryRun bool) *RunnerBuilder {
	b.dryRun = &dryRun
	return b
}

// WithSandbox enables or disables sandbox mode.
func (b *RunnerBuilder) WithSandbox(sandbox bool) *RunnerBuilder {
	b.sandbox = &sandbox
	return b
}

// WithMaxRetries sets the maximum number of retries.
func (b *RunnerBuilder) WithMaxRetries(retries int) *RunnerBuilder {
	b.maxRetries = &retries
	return b
}

// WithModeEnforcer sets the mode enforcer for dry-run/sandbox enforcement.
func (b *RunnerBuilder) WithModeEnforcer(me ModeEnforcerInterface) *RunnerBuilder {
	b.modeEnforcer = me
	return b
}

// WithGitClient sets the Git client for repository operations.
func (b *RunnerBuilder) WithGitClient(gc core.GitClient) *RunnerBuilder {
	b.gitClient = gc
	b.skipGitAutoCreate = true
	return b
}

// WithGitHubClient sets the GitHub client for PR creation.
func (b *RunnerBuilder) WithGitHubClient(ghc core.GitHubClient) *RunnerBuilder {
	b.githubClient = ghc
	return b
}

// WithGitClientFactory sets the factory for creating Git clients.
func (b *RunnerBuilder) WithGitClientFactory(gcf GitClientFactory) *RunnerBuilder {
	b.gitClientFactory = gcf
	return b
}

// WithWorktreeManager sets the worktree manager for task isolation.
func (b *RunnerBuilder) WithWorktreeManager(wm WorktreeManager) *RunnerBuilder {
	b.worktreeManager = wm
	b.skipGitAutoCreate = true
	return b
}

// Build constructs the Runner from the builder configuration.
// It validates all required dependencies and applies defaults for optional ones.
func (b *RunnerBuilder) Build(ctx context.Context) (*Runner, error) {
	// Check for accumulated errors
	if len(b.errors) > 0 {
		return nil, fmt.Errorf("builder errors: %v", b.errors)
	}

	// Validate required dependencies
	if b.config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if b.stateManager == nil {
		return nil, fmt.Errorf("state manager is required")
	}
	if b.agentRegistry == nil {
		return nil, fmt.Errorf("agent registry is required")
	}

	// Determine logger
	logger := b.logger
	if logger == nil && b.slogger != nil {
		logger = logging.NewWithHandler(b.slogger.Handler())
	}
	if logger == nil {
		logger = logging.NewNop()
	}

	// Set defaults for optional dependencies
	rateLimiter := b.rateLimiter
	if rateLimiter == nil {
		rateLimiter = service.NewRateLimiterRegistry()
	}

	outputNotifier := b.outputNotifier
	if outputNotifier == nil {
		outputNotifier = NopOutputNotifier{}
	}

	// Build runner config from application config
	runnerConfig := b.buildRunnerConfig()

	// Apply overrides
	if b.dryRun != nil {
		runnerConfig.DryRun = *b.dryRun
	}
	if b.sandbox != nil {
		runnerConfig.Sandbox = *b.sandbox
	}
	if b.maxRetries != nil {
		runnerConfig.MaxRetries = *b.maxRetries
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(b.stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(runnerConfig.MaxRetries))
	dagBuilder := service.NewDAGBuilder()

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, fmt.Errorf("creating prompt renderer: %w", err)
	}

	// Create adapters
	checkpointAdapter := NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := NewRateLimiterRegistryAdapter(rateLimiter, ctx)
	promptAdapter := NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := NewResumePointAdapter(checkpointManager)
	dagAdapter := NewDAGAdapter(dagBuilder)

	// Use injected mode enforcer or create default
	var modeEnforcerAdapter ModeEnforcerInterface
	if b.modeEnforcer != nil {
		modeEnforcerAdapter = b.modeEnforcer
	} else {
		modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
			DryRun:      runnerConfig.DryRun,
			Sandbox:     runnerConfig.Sandbox,
			DeniedTools: runnerConfig.DenyTools,
		})
		modeEnforcerAdapter = NewModeEnforcerAdapter(modeEnforcer)
	}

	// Use injected git components or create automatically
	var worktreeManager WorktreeManager
	var gitClient core.GitClient
	var githubClient core.GitHubClient
	var gitClientFactory GitClientFactory

	if b.skipGitAutoCreate {
		// Use injected components
		worktreeManager = b.worktreeManager
		gitClient = b.gitClient
		githubClient = b.githubClient
		gitClientFactory = b.gitClientFactory
	} else {
		// Create git components automatically
		worktreeManager, gitClient, githubClient, gitClientFactory = b.createGitComponents(logger)
	}

	// Create runner dependencies
	deps := RunnerDeps{
		Config:           runnerConfig,
		State:            b.stateManager,
		Agents:           b.agentRegistry,
		DAG:              dagAdapter,
		Checkpoint:       checkpointAdapter,
		ResumeProvider:   resumeAdapter,
		Prompts:          promptAdapter,
		Retry:            retryAdapter,
		RateLimits:       rateLimiterAdapter,
		Worktrees:        worktreeManager,
		GitClientFactory: gitClientFactory,
		Git:              gitClient,
		GitHub:           githubClient,
		Logger:           logger,
		Output:           outputNotifier,
		ModeEnforcer:     modeEnforcerAdapter,
		Control:          b.controlPlane,
		Heartbeat:        b.heartbeat,
	}

	// Create the runner
	runner := NewRunner(deps)
	if runner == nil {
		return nil, fmt.Errorf("failed to create runner (check moderator config)")
	}

	return runner, nil
}

// buildRunnerConfig creates a RunnerConfig from the application configuration.
func (b *RunnerBuilder) buildRunnerConfig() *RunnerConfig {
	cfg := b.config

	// If a runner config was explicitly set, use it as base
	if b.runnerConfig != nil && b.runnerConfig != DefaultRunnerConfig() {
		return b.runnerConfig
	}

	return BuildRunnerConfigFromConfig(cfg)
}

// BuildRunnerConfigFromConfig creates a RunnerConfig from application config.
// This is exported for use by CLI and WebUI when they need to create RunnerConfig
// outside of the builder pattern.
func BuildRunnerConfigFromConfig(cfg *config.Config) *RunnerConfig {
	// Parse workflow timeout (defaults to 12h if not set or invalid)
	timeout := DefaultRunnerConfig().Timeout
	if cfg.Workflow.Timeout != "" {
		if parsed, err := time.ParseDuration(cfg.Workflow.Timeout); err == nil {
			timeout = parsed
		}
	}

	// Parse phase timeouts
	defaultPhaseTimeout := DefaultRunnerConfig().PhaseTimeouts.Analyze

	analyzeTimeout := defaultPhaseTimeout
	if cfg.Phases.Analyze.Timeout != "" {
		if parsed, err := time.ParseDuration(cfg.Phases.Analyze.Timeout); err == nil {
			analyzeTimeout = parsed
		}
	}

	planTimeout := defaultPhaseTimeout
	if cfg.Phases.Plan.Timeout != "" {
		if parsed, err := time.ParseDuration(cfg.Phases.Plan.Timeout); err == nil {
			planTimeout = parsed
		}
	}

	executeTimeout := defaultPhaseTimeout
	if cfg.Phases.Execute.Timeout != "" {
		if parsed, err := time.ParseDuration(cfg.Phases.Execute.Timeout); err == nil {
			executeTimeout = parsed
		}
	}

	return &RunnerConfig{
		Timeout:           timeout,
		MaxRetries:        cfg.Workflow.MaxRetries,
		DryRun:            cfg.Workflow.DryRun,
		Sandbox:           cfg.Workflow.Sandbox,
		DenyTools:         cfg.Workflow.DenyTools,
		DefaultAgent:      cfg.Agents.Default,
		AgentPhaseModels:  buildAgentPhaseModels(cfg.Agents),
		WorktreeAutoClean: cfg.Git.AutoClean,
		WorktreeMode:      cfg.Git.WorktreeMode,
		Refiner: RefinerConfig{
			Enabled: cfg.Phases.Analyze.Refiner.Enabled,
			Agent:   cfg.Phases.Analyze.Refiner.Agent,
		},
		Synthesizer: SynthesizerConfig{
			Agent: cfg.Phases.Analyze.Synthesizer.Agent,
		},
		Moderator: ModeratorConfig{
			Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
			Agent:               cfg.Phases.Analyze.Moderator.Agent,
			Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
			MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
			AbortThreshold:      cfg.Phases.Analyze.Moderator.AbortThreshold,
			StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
		},
		SingleAgent: SingleAgentConfig{
			Enabled: cfg.Phases.Analyze.SingleAgent.Enabled,
			Agent:   cfg.Phases.Analyze.SingleAgent.Agent,
			Model:   cfg.Phases.Analyze.SingleAgent.Model,
		},
		PlanSynthesizer: PlanSynthesizerConfig{
			Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
			Agent:   cfg.Phases.Plan.Synthesizer.Agent,
		},
		PhaseTimeouts: PhaseTimeouts{
			Analyze: analyzeTimeout,
			Plan:    planTimeout,
			Execute: executeTimeout,
		},
		Finalization: FinalizationConfig{
			AutoCommit:    cfg.Git.AutoCommit,
			AutoPush:      cfg.Git.AutoPush,
			AutoPR:        cfg.Git.AutoPR,
			AutoMerge:     cfg.Git.AutoMerge,
			PRBaseBranch:  cfg.Git.PRBaseBranch,
			MergeStrategy: cfg.Git.MergeStrategy,
		},
		Report: report.Config{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
	}
}

// buildAgentPhaseModels extracts phase model overrides from agent configurations.
func buildAgentPhaseModels(agents config.AgentsConfig) map[string]map[string]string {
	result := make(map[string]map[string]string)

	agentMap := map[string]config.AgentConfig{
		"claude":   agents.Claude,
		"gemini":   agents.Gemini,
		"codex":    agents.Codex,
		"copilot":  agents.Copilot,
		"opencode": agents.OpenCode,
	}

	for name, agentCfg := range agentMap {
		if agentCfg.Enabled && len(agentCfg.PhaseModels) > 0 {
			result[name] = agentCfg.PhaseModels
		}
	}

	return result
}

// createGitComponents creates Git-related components for the runner.
func (b *RunnerBuilder) createGitComponents(logger *logging.Logger) (WorktreeManager, core.GitClient, core.GitHubClient, GitClientFactory) {
	cfg := b.config

	// Create git client and worktree manager (optional, may fail)
	var worktreeManager WorktreeManager
	var gitClient core.GitClient
	cwd, err := os.Getwd()
	if err == nil {
		// Import git package dynamically to avoid import cycle
		gc, gitErr := createGitClient(cwd)
		if gitErr == nil && gc != nil {
			gitClient = gc
			worktreeManager = createWorktreeManager(gc, cfg.Git.WorktreeDir, logger)
		} else if logger != nil {
			logger.Warn("git client unavailable, worktree isolation disabled", "error", gitErr)
		}
	}

	// Create GitHub client for PR creation (only if auto_pr is enabled)
	var githubClient core.GitHubClient
	if cfg.Git.AutoPR {
		ghClient, ghErr := createGitHubClient()
		if ghErr != nil {
			if logger != nil {
				logger.Warn("failed to create GitHub client, PR creation disabled", "error", ghErr)
			}
		} else {
			githubClient = ghClient
			if logger != nil {
				logger.Info("GitHub client initialized for PR creation")
			}
		}
	}

	// Create git client factory
	gitClientFactory := createGitClientFactory()

	return worktreeManager, gitClient, githubClient, gitClientFactory
}

// These functions are defined to avoid import cycles.
// They will be implemented using the adapters/git and adapters/github packages.
var (
	createGitClient        = defaultCreateGitClient
	createWorktreeManager  = defaultCreateWorktreeManager
	createGitHubClient     = defaultCreateGitHubClient
	createGitClientFactory = defaultCreateGitClientFactory
)

func defaultCreateGitClient(cwd string) (core.GitClient, error) {
	return nil, fmt.Errorf("git client factory not configured")
}

func defaultCreateWorktreeManager(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager {
	return nil
}

func defaultCreateGitHubClient() (core.GitHubClient, error) {
	return nil, fmt.Errorf("github client factory not configured")
}

func defaultCreateGitClientFactory() GitClientFactory {
	return nil
}

// SetGitFactories sets the factory functions for creating Git components.
// This should be called during application initialization to wire up the git adapters.
func SetGitFactories(
	gitClientFn func(cwd string) (core.GitClient, error),
	worktreeMgrFn func(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager,
	githubClientFn func() (core.GitHubClient, error),
	gitClientFactoryFn func() GitClientFactory,
) {
	if gitClientFn != nil {
		createGitClient = gitClientFn
	}
	if worktreeMgrFn != nil {
		createWorktreeManager = worktreeMgrFn
	}
	if githubClientFn != nil {
		createGitHubClient = githubClientFn
	}
	if gitClientFactoryFn != nil {
		createGitClientFactory = gitClientFactoryFn
	}
}
