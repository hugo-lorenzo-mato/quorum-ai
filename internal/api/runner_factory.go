// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	webadapters "github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/web"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// RunnerFactory creates workflow.Runner instances for web execution context.
// It handles all the dependency wiring that would otherwise be duplicated
// in each handler that needs to run workflows.
type RunnerFactory struct {
	stateManager  core.StateManager
	agentRegistry core.AgentRegistry
	eventBus      *events.EventBus
	configLoader  *config.Loader
	logger        *logging.Logger
}

// NewRunnerFactory creates a new runner factory.
func NewRunnerFactory(
	stateManager core.StateManager,
	agentRegistry core.AgentRegistry,
	eventBus *events.EventBus,
	configLoader *config.Loader,
	logger *logging.Logger,
) *RunnerFactory {
	return &RunnerFactory{
		stateManager:  stateManager,
		agentRegistry: agentRegistry,
		eventBus:      eventBus,
		configLoader:  configLoader,
		logger:        logger,
	}
}

// CreateRunner creates a new workflow.Runner for executing a workflow.
// It creates all necessary dependencies and adapters for the web context.
//
// Parameters:
//   - ctx: Context for the runner (should have appropriate timeout)
//   - workflowID: The ID of the workflow being executed
//   - cp: Optional ControlPlane for pause/resume/cancel (may be nil)
//
// Returns:
//   - *workflow.Runner: Fully configured runner
//   - *webadapters.WebOutputNotifier: The notifier (for lifecycle events)
//   - error: Any error during setup
func (f *RunnerFactory) CreateRunner(ctx context.Context, workflowID string, cp *control.ControlPlane) (*workflow.Runner, *webadapters.WebOutputNotifier, error) {
	// Validate prerequisites
	if f.stateManager == nil {
		return nil, nil, fmt.Errorf("state manager not configured")
	}
	if f.agentRegistry == nil {
		return nil, nil, fmt.Errorf("agent registry not configured")
	}
	if f.eventBus == nil {
		return nil, nil, fmt.Errorf("event bus not configured")
	}
	if f.configLoader == nil {
		return nil, nil, fmt.Errorf("config loader not configured")
	}

	// Load configuration
	cfg, err := f.configLoader.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	// Build runner configuration from loaded config
	runnerConfig := buildRunnerConfig(cfg)

	// Create service components
	checkpointManager := service.NewCheckpointManager(f.stateManager, f.logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(runnerConfig.MaxRetries))
	rateLimiterRegistry := service.NewRateLimiterRegistry()
	dagBuilder := service.NewDAGBuilder()

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, nil, fmt.Errorf("creating prompt renderer: %w", err)
	}

	// Create git client and worktree manager (optional, may fail)
	var worktreeManager workflow.WorktreeManager
	var gitClient core.GitClient
	cwd, err := os.Getwd()
	if err == nil {
		gc, gitErr := git.NewClient(cwd)
		if gitErr == nil && gc != nil {
			gitClient = gc
			worktreeManager = git.NewTaskWorktreeManager(gc, cfg.Git.WorktreeDir).WithLogger(f.logger)
		} else if f.logger != nil {
			f.logger.Warn("git client unavailable, worktree isolation disabled", "error", gitErr)
		}
	}

	// Create GitHub client for PR creation (only if auto_pr is enabled)
	var githubClient core.GitHubClient
	if cfg.Git.AutoPR {
		ghClient, ghErr := github.NewClientFromRepo()
		if ghErr != nil {
			if f.logger != nil {
				f.logger.Warn("failed to create GitHub client, PR creation disabled", "error", ghErr)
			}
		} else {
			githubClient = ghClient
			if f.logger != nil {
				f.logger.Info("GitHub client initialized for PR creation")
			}
		}
	}

	// Create adapters
	checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx)
	promptAdapter := workflow.NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := workflow.NewResumePointAdapter(checkpointManager)
	dagAdapter := workflow.NewDAGAdapter(dagBuilder)

	// Create mode enforcer
	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
		DryRun:      runnerConfig.DryRun,
		Sandbox:     runnerConfig.Sandbox,
		DeniedTools: runnerConfig.DenyTools,
	})
	modeEnforcerAdapter := workflow.NewModeEnforcerAdapter(modeEnforcer)

	// Create web output notifier (bridges to EventBus)
	outputNotifier := webadapters.NewWebOutputNotifier(f.eventBus, workflowID)

	// Create runner with all dependencies
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:           runnerConfig,
		State:            f.stateManager,
		Agents:           f.agentRegistry,
		DAG:              dagAdapter,
		Checkpoint:       checkpointAdapter,
		ResumeProvider:   resumeAdapter,
		Prompts:          promptAdapter,
		Retry:            retryAdapter,
		RateLimits:       rateLimiterAdapter,
		Worktrees:        worktreeManager,
		GitClientFactory: git.NewClientFactory(),
		Git:              gitClient,
		GitHub:           githubClient,
		Logger:           f.logger,
		Output:           outputNotifier,
		ModeEnforcer:     modeEnforcerAdapter,
		Control:          cp, // For pause/resume/cancel support
	})

	if runner == nil {
		return nil, nil, fmt.Errorf("failed to create runner (check moderator config)")
	}

	return runner, outputNotifier, nil
}

// buildAgentPhaseModels extracts phase model overrides from agent configurations.
// Returns a map of agent name -> phase name -> model name for use in ResolvePhaseModel.
func buildAgentPhaseModels(agents config.AgentsConfig) map[string]map[string]string {
	result := make(map[string]map[string]string)

	agentMap := map[string]config.AgentConfig{
		"claude":   agents.Claude,
		"gemini":   agents.Gemini,
		"codex":    agents.Codex,
		"copilot":  agents.Copilot,
		"opencode": agents.OpenCode,
	}

	for name, cfg := range agentMap {
		if cfg.Enabled && len(cfg.PhaseModels) > 0 {
			result[name] = cfg.PhaseModels
		}
	}

	return result
}

// buildRunnerConfig creates a RunnerConfig from the application configuration.
func buildRunnerConfig(cfg *config.Config) *workflow.RunnerConfig {
	// Parse workflow timeout (defaults to 12h if not set or invalid)
	timeout := 12 * time.Hour
	if cfg.Workflow.Timeout != "" {
		if parsed, err := time.ParseDuration(cfg.Workflow.Timeout); err == nil {
			timeout = parsed
		}
	}

	return &workflow.RunnerConfig{
		Timeout:           timeout,
		MaxRetries:        cfg.Workflow.MaxRetries,
		DryRun:            cfg.Workflow.DryRun,
		Sandbox:           cfg.Workflow.Sandbox,
		DenyTools:         cfg.Workflow.DenyTools,
		DefaultAgent:      cfg.Agents.Default,
		AgentPhaseModels:  buildAgentPhaseModels(cfg.Agents),
		WorktreeAutoClean: cfg.Git.AutoClean,
		WorktreeMode:      cfg.Git.WorktreeMode,
		Refiner: workflow.RefinerConfig{
			Enabled: cfg.Phases.Analyze.Refiner.Enabled,
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
		PlanSynthesizer: workflow.PlanSynthesizerConfig{
			Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
			Agent:   cfg.Phases.Plan.Synthesizer.Agent,
		},
	}
}

// RunnerFactory returns a factory for creating workflow runners.
// Returns nil if required dependencies are not configured.
func (s *Server) RunnerFactory() *RunnerFactory {
	if s.stateManager == nil || s.agentRegistry == nil || s.eventBus == nil || s.configLoader == nil {
		return nil
	}

	// Create a logging.Logger from the slog.Logger
	var logger *logging.Logger
	if s.logger != nil {
		logger = logging.NewWithHandler(s.logger.Handler())
	}

	return NewRunnerFactory(
		s.stateManager,
		s.agentRegistry,
		s.eventBus,
		s.configLoader,
		logger,
	)
}
