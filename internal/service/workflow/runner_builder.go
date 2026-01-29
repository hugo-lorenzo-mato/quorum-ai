package workflow

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// RunnerBuilder provides a fluent API for creating workflow Runners.
type RunnerBuilder struct {
	config        *config.Config
	runnerConfig  *RunnerConfig
	stateManager  StateManager
	agentRegistry core.AgentRegistry
	logger        *logging.Logger
	gitIsolation  *GitIsolationConfig
	dag           interface {
		DAGBuilder
		TaskDAG
	}
	checkpoint       CheckpointCreator
	resumeProvider   ResumePointProvider
	prompts          PromptRenderer
	retry            RetryExecutor
	rateLimits       RateLimiterGetter
	worktrees        WorktreeManager
	gitClientFactory GitClientFactory
	git              core.GitClient
	github           core.GitHubClient
	output           OutputNotifier
	modeEnforcer     ModeEnforcerInterface
	control          *control.ControlPlane
	heartbeat        *HeartbeatManager
}

// NewRunnerBuilder creates a new RunnerBuilder.
func NewRunnerBuilder() *RunnerBuilder {
	return &RunnerBuilder{}
}

// WithConfig sets the application configuration.
func (b *RunnerBuilder) WithConfig(cfg *config.Config) *RunnerBuilder {
	b.config = cfg
	return b
}

// WithStateManager sets the state manager.
func (b *RunnerBuilder) WithStateManager(sm StateManager) *RunnerBuilder {
	b.stateManager = sm
	return b
}

// WithAgentRegistry sets the agent registry.
func (b *RunnerBuilder) WithAgentRegistry(ar core.AgentRegistry) *RunnerBuilder {
	b.agentRegistry = ar
	return b
}

// WithLogger sets the logger.
func (b *RunnerBuilder) WithLogger(l *logging.Logger) *RunnerBuilder {
	b.logger = l
	return b
}

// WithGitIsolation sets the Git isolation configuration.
func (b *RunnerBuilder) WithGitIsolation(gi *GitIsolationConfig) *RunnerBuilder {
	b.gitIsolation = gi
	return b
}

func (b *RunnerBuilder) WithRunnerConfig(cfg *RunnerConfig) *RunnerBuilder {
	b.runnerConfig = cfg
	return b
}

func (b *RunnerBuilder) WithDAG(dag interface {
	DAGBuilder
	TaskDAG
}) *RunnerBuilder {
	b.dag = dag
	return b
}

func (b *RunnerBuilder) WithCheckpoint(c CheckpointCreator) *RunnerBuilder {
	b.checkpoint = c
	return b
}

func (b *RunnerBuilder) WithResumeProvider(r ResumePointProvider) *RunnerBuilder {
	b.resumeProvider = r
	return b
}

func (b *RunnerBuilder) WithPrompts(p PromptRenderer) *RunnerBuilder {
	b.prompts = p
	return b
}

func (b *RunnerBuilder) WithRetry(r RetryExecutor) *RunnerBuilder {
	b.retry = r
	return b
}

func (b *RunnerBuilder) WithRateLimits(r RateLimiterGetter) *RunnerBuilder {
	b.rateLimits = r
	return b
}

func (b *RunnerBuilder) WithWorktrees(w WorktreeManager) *RunnerBuilder {
	b.worktrees = w
	return b
}

func (b *RunnerBuilder) WithGitClientFactory(f GitClientFactory) *RunnerBuilder {
	b.gitClientFactory = f
	return b
}

func (b *RunnerBuilder) WithGit(g core.GitClient) *RunnerBuilder {
	b.git = g
	return b
}

func (b *RunnerBuilder) WithGitHub(g core.GitHubClient) *RunnerBuilder {
	b.github = g
	return b
}

func (b *RunnerBuilder) WithOutput(o OutputNotifier) *RunnerBuilder {
	b.output = o
	return b
}

func (b *RunnerBuilder) WithModeEnforcer(m ModeEnforcerInterface) *RunnerBuilder {
	b.modeEnforcer = m
	return b
}

func (b *RunnerBuilder) WithControl(c *control.ControlPlane) *RunnerBuilder {
	b.control = c
	return b
}

func (b *RunnerBuilder) WithHeartbeat(h *HeartbeatManager) *RunnerBuilder {
	b.heartbeat = h
	return b
}

// Build creates a fully configured Runner.
func (b *RunnerBuilder) Build() (*Runner, error) {
	if b.runnerConfig == nil {
		b.runnerConfig = DefaultRunnerConfig()
	}
	if b.gitIsolation != nil {
		b.runnerConfig.GitIsolation = b.gitIsolation
	}

	return NewRunner(RunnerDeps{
		Config:           b.runnerConfig,
		State:            b.stateManager,
		Agents:           b.agentRegistry,
		DAG:              b.dag,
		Checkpoint:       b.checkpoint,
		ResumeProvider:   b.resumeProvider,
		Prompts:          b.prompts,
		Retry:            b.retry,
		RateLimits:       b.rateLimits,
		Worktrees:        b.worktrees,
		GitClientFactory: b.gitClientFactory,
		Git:              b.git,
		GitHub:           b.github,
		Logger:           b.logger,
		Output:           b.output,
		ModeEnforcer:     b.modeEnforcer,
		Control:          b.control,
		Heartbeat:        b.heartbeat,
	}), nil
}
