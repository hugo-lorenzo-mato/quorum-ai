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

// WorkflowConfigOverride holds per-workflow configuration that can override
// global application settings. This is a local type to avoid circular imports
// with the api package. The API layer should convert api.WorkflowConfig to this type.
type WorkflowConfigOverride struct {
	// ExecutionMode determines whether to use multi-agent consensus or single-agent mode.
	// Valid values: "multi_agent" (default), "single_agent"
	ExecutionMode string

	// SingleAgentName is the name of the agent to use when ExecutionMode is "single_agent".
	SingleAgentName string

	// SingleAgentModel is an optional model override for the single agent.
	SingleAgentModel string

	// SingleAgentReasoningEffort is an optional reasoning effort override for the single agent.
	// If empty, agent defaults are used.
	SingleAgentReasoningEffort string

	// ConsensusThreshold is the confidence threshold required for multi-agent consensus.
	ConsensusThreshold float64

	// MaxRetries is the maximum number of times to retry failed tasks.
	MaxRetries int

	// Timeout is the maximum duration for the entire workflow.
	Timeout time.Duration

	// DryRun enables simulation mode without making external changes.
	DryRun bool

	// HasDryRun indicates if DryRun was explicitly set.
	HasDryRun bool
}

// IsSingleAgentMode returns true if the override specifies single-agent execution.
func (c *WorkflowConfigOverride) IsSingleAgentMode() bool {
	return c != nil && c.ExecutionMode == "single_agent"
}

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

	// Workflow-level configuration override (from API)
	workflowConfig *WorkflowConfigOverride

	// Runner configuration
	runnerConfig         *RunnerConfig
	runnerConfigExplicit bool // true if runnerConfig was explicitly set via WithRunnerConfig

	// Overrides for runner config fields
	phase      *core.Phase
	maxRetries *int
	dryRun     *bool

	// Disable auto-creation of git components
	skipGitAutoCreate bool

	// Project root directory (for multi-project support)
	projectRoot string

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
		errors: make([]error, 0),
		// runnerConfig is nil - will be built from application config in Build()
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
	b.runnerConfigExplicit = true
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

// WithMaxRetries sets the maximum number of retries.
func (b *RunnerBuilder) WithMaxRetries(retries int) *RunnerBuilder {
	b.maxRetries = &retries
	return b
}

// WithModeEnforcer sets the mode enforcer for execution-mode enforcement.
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

// WithProjectRoot sets the project root directory for workflow execution.
// This is used when running workflows in a different project than the server's CWD.
func (b *RunnerBuilder) WithProjectRoot(root string) *RunnerBuilder {
	b.projectRoot = root
	return b
}

// WithWorkflowConfig sets workflow-specific configuration overrides.
// When provided, these settings take precedence over global application config.
// This enables per-workflow execution mode selection (single-agent vs multi-agent).
func (b *RunnerBuilder) WithWorkflowConfig(wfConfig *WorkflowConfigOverride) *RunnerBuilder {
	b.workflowConfig = wfConfig
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

	// Resolve Git isolation config (default-enabled)
	gitIsolation := b.gitIsolation
	if gitIsolation == nil {
		gitIsolation = DefaultGitIsolationConfig()
	}

	// Create workflow-level worktree manager when isolation is enabled
	var workflowWorktrees core.WorkflowWorktreeManager
	if gitIsolation.Enabled && gitClient != nil {
		repoRoot, err := gitClient.RepoRoot(ctx)
		if err != nil {
			logger.Warn("failed to detect repo root, workflow isolation disabled", "error", err)
		} else {
			wtMgr, wtErr := createWorkflowWorktreeManager(gitClient, repoRoot, b.config.Git.Worktree.Dir, logger)
			if wtErr != nil {
				logger.Warn("failed to create workflow worktree manager, workflow isolation disabled", "error", wtErr)
			} else {
				workflowWorktrees = wtMgr
			}
		}
	}

	// Create runner dependencies
	deps := RunnerDeps{
		Config:            runnerConfig,
		State:             b.stateManager,
		Agents:            b.agentRegistry,
		DAG:               dagAdapter,
		Checkpoint:        checkpointAdapter,
		ResumeProvider:    resumeAdapter,
		Prompts:           promptAdapter,
		Retry:             retryAdapter,
		RateLimits:        rateLimiterAdapter,
		Worktrees:         worktreeManager,
		WorkflowWorktrees: workflowWorktrees,
		GitIsolation:      gitIsolation,
		GitClientFactory:  gitClientFactory,
		Git:               gitClient,
		GitHub:            githubClient,
		Logger:            logger,
		Output:            outputNotifier,
		ModeEnforcer:      modeEnforcerAdapter,
		Control:           b.controlPlane,
		Heartbeat:         b.heartbeat,
		ProjectRoot:       b.projectRoot,
	}

	// Create the runner
	runner, err := NewRunner(deps)
	if err != nil {
		return nil, err
	}

	return runner, nil
}

// buildRunnerConfig creates a RunnerConfig from the application configuration.
func (b *RunnerBuilder) buildRunnerConfig() *RunnerConfig {
	cfg := b.config

	// If a runner config was explicitly set via WithRunnerConfig, use it
	if b.runnerConfigExplicit && b.runnerConfig != nil {
		return b.runnerConfig
	}

	// Build from application config
	runnerCfg := BuildRunnerConfigFromConfig(cfg)

	// Apply workflow-level overrides (higher priority than app config)
	if b.workflowConfig != nil {
		if b.workflowConfig.ConsensusThreshold > 0 {
			runnerCfg.Moderator.Threshold = b.workflowConfig.ConsensusThreshold
		}
		if b.workflowConfig.MaxRetries > 0 {
			runnerCfg.MaxRetries = b.workflowConfig.MaxRetries
		}
		if b.workflowConfig.Timeout > 0 {
			runnerCfg.Timeout = b.workflowConfig.Timeout
		}
		if b.workflowConfig.HasDryRun {
			runnerCfg.DryRun = b.workflowConfig.DryRun
		}
	}

	runnerCfg.SingleAgent = b.buildSingleAgentConfig(cfg)

	return runnerCfg
}

// buildSingleAgentConfig constructs SingleAgentConfig with proper precedence:
//  1. Workflow-specific override (highest priority)
//  2. Application config (fallback)
//  3. Disabled (default if nothing configured)
//
// This enables per-workflow execution mode selection while maintaining
// backward compatibility with global configuration.
func (b *RunnerBuilder) buildSingleAgentConfig(cfg *config.Config) SingleAgentConfig {
	// Check for workflow-level override first (highest priority)
	if b.workflowConfig != nil {
		// Explicit single-agent mode request
		if b.workflowConfig.IsSingleAgentMode() {
			return SingleAgentConfig{
				Enabled:         true,
				Agent:           b.workflowConfig.SingleAgentName,
				Model:           b.workflowConfig.SingleAgentModel,
				ReasoningEffort: b.workflowConfig.SingleAgentReasoningEffort,
			}
		}

		// Explicit multi-agent mode request (can override global single-agent)
		if b.workflowConfig.ExecutionMode == "multi_agent" {
			return SingleAgentConfig{
				Enabled: false,
			}
		}
	}

	// Fall back to application config (second priority)
	if cfg != nil && cfg.Phases.Analyze.SingleAgent.Enabled {
		return SingleAgentConfig{
			Enabled: cfg.Phases.Analyze.SingleAgent.Enabled,
			Agent:   cfg.Phases.Analyze.SingleAgent.Agent,
			Model:   cfg.Phases.Analyze.SingleAgent.Model,
		}
	}

	// Default: disabled (multi-agent mode)
	return SingleAgentConfig{
		Enabled: false,
	}
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

	// Parse process grace period (default: 30s)
	processGracePeriod := 30 * time.Second
	if cfg.Phases.Analyze.ProcessGracePeriod != "" {
		if parsed, err := time.ParseDuration(cfg.Phases.Analyze.ProcessGracePeriod); err == nil {
			processGracePeriod = parsed
		}
	}

	return &RunnerConfig{
		Timeout:           timeout,
		MaxRetries:        cfg.Workflow.MaxRetries,
		DryRun:            cfg.Workflow.DryRun,
		DenyTools:         cfg.Workflow.DenyTools,
		DefaultAgent:      cfg.Agents.Default,
		AgentPhaseModels:  buildAgentPhaseModels(cfg.Agents),
		WorktreeAutoClean: cfg.Git.Worktree.AutoClean,
		WorktreeMode:      cfg.Git.Worktree.Mode,
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
			Thresholds:          cfg.Phases.Analyze.Moderator.Thresholds,
			MinSuccessfulAgents: cfg.Phases.Analyze.Moderator.MinSuccessfulAgents,
			MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
			WarningThreshold:    cfg.Phases.Analyze.Moderator.WarningThreshold,
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
			Analyze:            analyzeTimeout,
			Plan:               planTimeout,
			Execute:            executeTimeout,
			ProcessGracePeriod: processGracePeriod,
		},
		Finalization: FinalizationConfig{
			AutoCommit:    cfg.Git.Task.AutoCommit,
			AutoPush:      cfg.Git.Finalization.AutoPush,
			AutoPR:        cfg.Git.Finalization.AutoPR,
			AutoMerge:     cfg.Git.Finalization.AutoMerge,
			PRBaseBranch:  cfg.Git.Finalization.PRBaseBranch,
			MergeStrategy: cfg.Git.Finalization.MergeStrategy,
		},
		Report: report.Config{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
		ProjectAgentPhases: cfg.ExtractAgentPhases(),
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

	// Determine the root directory for git operations:
	// 1. Use projectRoot if explicitly set (multi-project mode)
	// 2. Fall back to current working directory (single-project/CLI mode)
	var rootDir string
	if b.projectRoot != "" {
		rootDir = b.projectRoot
		if logger != nil {
			logger.Debug("using project root for git operations", "root", rootDir)
		}
	} else {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			if logger != nil {
				logger.Warn("failed to get working directory", "error", err)
			}
			return nil, nil, nil, createGitClientFactory()
		}
	}

	// Create git client and worktree manager (optional, may fail)
	var worktreeManager WorktreeManager
	var gitClient core.GitClient
	gc, gitErr := createGitClient(rootDir)
	if gitErr == nil && gc != nil {
		gitClient = gc
		worktreeManager = createWorktreeManager(gc, cfg.Git.Worktree.Dir, logger)
	} else if logger != nil {
		logger.Warn("git client unavailable, worktree isolation disabled", "error", gitErr)
	}

	// Create GitHub client for PR creation (only if auto_pr is enabled)
	var githubClient core.GitHubClient
	if cfg.Git.Finalization.AutoPR {
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
	createGitClient               = defaultCreateGitClient
	createWorktreeManager         = defaultCreateWorktreeManager
	createGitHubClient            = defaultCreateGitHubClient
	createGitClientFactory        = defaultCreateGitClientFactory
	createWorkflowWorktreeManager = defaultCreateWorkflowWorktreeManager
)

func defaultCreateGitClient(_ string) (core.GitClient, error) {
	return nil, fmt.Errorf("git client factory not configured")
}

func defaultCreateWorktreeManager(_ core.GitClient, _ string, _ *logging.Logger) WorktreeManager {
	return nil
}

func defaultCreateGitHubClient() (core.GitHubClient, error) {
	return nil, fmt.Errorf("github client factory not configured")
}

func defaultCreateGitClientFactory() GitClientFactory {
	return nil
}

func defaultCreateWorkflowWorktreeManager(_ core.GitClient, _, _ string, _ *logging.Logger) (core.WorkflowWorktreeManager, error) {
	return nil, fmt.Errorf("workflow worktree manager factory not configured")
}

// SetGitFactories sets the factory functions for creating Git components.
// This should be called during application initialization to wire up the git adapters.
func SetGitFactories(
	gitClientFn func(cwd string) (core.GitClient, error),
	worktreeMgrFn func(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager,
	githubClientFn func() (core.GitHubClient, error),
	gitClientFactoryFn func() GitClientFactory,
	workflowWorktreeMgrFn func(gc core.GitClient, repoRoot, worktreeDir string, logger *logging.Logger) (core.WorkflowWorktreeManager, error),
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
	if workflowWorktreeMgrFn != nil {
		createWorkflowWorktreeManager = workflowWorktreeMgrFn
	}
}
