package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui/chat"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start interactive chat mode",
	Long: `Start an interactive chat session with AI agents.

Chat mode provides a conversational interface with slash commands
for workflow control:

  /plan <prompt>   Generate plan (Optimize → Analyze → Plan)
  /run <prompt>    Run complete workflow (Optimize → Analyze → Plan → Execute)
  /status          Show workflow status
  /cancel          Cancel current workflow
  /model <name>    Set current model
  /agent <name>    Set current agent
  /help            Show all commands

The workflow phases use the configured agents and consensus mechanism
from your .quorum/config.yaml configuration.

Example:
  quorum chat
  quorum chat --agent gemini
  quorum chat --model opus`,
	RunE: runChat,
}

var (
	chatAgent string
	chatModel string
)

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Default agent (claude, gemini, codex, copilot)")
	chatCmd.Flags().StringVar(&chatModel, "model", "", "Default model override")
}

func runChat(_ *cobra.Command, _ []string) error {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Load configuration
	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create logger (quiet for chat mode)
	logger := logging.New(logging.Config{
		Level:  "warn",
		Format: "text",
		Output: os.Stderr,
	})

	// Create event bus
	eventBus := events.New(100)
	defer eventBus.Close()

	// Create control plane
	controlPlane := control.New()

	// Create agent registry
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	// Determine default agent
	defaultAgent := chatAgent
	if defaultAgent == "" {
		defaultAgent = cfg.Agents.Default
	}
	if defaultAgent == "" {
		defaultAgent = "claude"
	}

	// Determine default model
	defaultModel := chatModel

	// Create workflow runner dependencies
	runner, err := createWorkflowRunner(ctx, cfg, loader, registry, controlPlane, eventBus, logger)
	if err != nil {
		return fmt.Errorf("creating workflow runner: %w", err)
	}

	// Parse chat configuration
	chatTimeout, _ := time.ParseDuration(cfg.Chat.Timeout)
	chatProgressInterval, _ := time.ParseDuration(cfg.Chat.ProgressInterval)

	// Build list of available agents and their models
	availableAgents := []string{}
	agentModels := make(map[string][]string)

	if cfg.Agents.Claude.Enabled {
		availableAgents = append(availableAgents, "claude")
		agentModels["claude"] = []string{
			"claude-opus-4-5-20251101",
			"claude-sonnet-4-20250514",
			"claude-sonnet-4-5-20251101",
			"claude-haiku-4-5-20251101",
		}
	}
	if cfg.Agents.Gemini.Enabled {
		availableAgents = append(availableAgents, "gemini")
		agentModels["gemini"] = []string{
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-3-pro-preview",
			"gemini-3-flash-preview",
		}
	}
	if cfg.Agents.Codex.Enabled {
		availableAgents = append(availableAgents, "codex")
		agentModels["codex"] = []string{
			"gpt-5.1",
			"gpt-5.1-codex",
			"gpt-5.2",
			"gpt-5.2-codex",
			"o3",
			"o3-mini",
		}
	}
	if cfg.Agents.Copilot.Enabled {
		availableAgents = append(availableAgents, "copilot")
		agentModels["copilot"] = []string{
			"claude-sonnet-4-5",
			"claude-haiku-4-5",
			"gpt-4o",
			"o3-mini",
		}
	}

	// Create chat model with workflow runner, config, and version
	model := chat.NewModel(controlPlane, registry, defaultAgent, defaultModel)
	model = model.WithWorkflowRunner(runner, eventBus, logger)
	model = model.WithChatConfig(chatTimeout, chatProgressInterval)
	model = model.WithAgentModels(availableAgents, agentModels)
	model = model.WithEditor(cfg.Chat.Editor)
	model = model.WithVersion(GetVersion())

	// Run the TUI
	// Note: Mouse capture disabled to allow native terminal text selection
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("running chat: %w", err)
	}

	return nil
}

// createWorkflowRunner creates a workflow runner with all dependencies.
func createWorkflowRunner(
	ctx context.Context,
	cfg *config.Config,
	_ *config.Loader,
	registry *cli.Registry,
	controlPlane *control.ControlPlane,
	eventBus *events.EventBus,
	logger *logging.Logger,
) (*workflow.Runner, error) {
	// Create state manager
	statePath := cfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	stateManager := state.NewJSONStateManager(statePath)

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, fmt.Errorf("creating prompt renderer: %w", err)
	}

	// Create runner config
	timeout, err := parseDurationDefault(cfg.Workflow.Timeout, defaultWorkflowTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow timeout %q: %w", cfg.Workflow.Timeout, err)
	}

	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}

	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   3,
		DryRun:       false,
		Sandbox:      cfg.Workflow.Sandbox,
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
		Optimizer: workflow.OptimizerConfig{
			Enabled: cfg.PromptOptimizer.Enabled,
			Agent:   cfg.PromptOptimizer.Agent,
			Model:   cfg.PromptOptimizer.Model,
		},
		Consolidator: workflow.ConsolidatorConfig{
			Agent: cfg.AnalysisConsolidator.Agent,
			Model: cfg.AnalysisConsolidator.Model,
		},
		Report: report.Config{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
		Arbiter: workflow.ArbiterConfig{
			Enabled:             cfg.Consensus.Arbiter.Enabled,
			Agent:               cfg.Consensus.Arbiter.Agent,
			Model:               cfg.Consensus.Arbiter.Model,
			Threshold:           cfg.Consensus.Arbiter.Threshold,
			MinRounds:           cfg.Consensus.Arbiter.MinRounds,
			MaxRounds:           cfg.Consensus.Arbiter.MaxRounds,
			AbortThreshold:      cfg.Consensus.Arbiter.AbortThreshold,
			StagnationThreshold: cfg.Consensus.Arbiter.StagnationThreshold,
		},
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(3))
	rateLimiterRegistry := service.NewRateLimiterRegistry()
	dagBuilder := service.NewDAGBuilder()

	// Create worktree manager
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

	// Create adapters
	checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx)
	promptAdapter := workflow.NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := workflow.NewResumePointAdapter(checkpointManager)
	dagAdapter := workflow.NewDAGAdapter(dagBuilder)
	stateAdapter := &stateManagerAdapter{sm: stateManager}

	// Create output notifier for chat (minimal output)
	outputNotifier := &chatOutputNotifier{eventBus: eventBus}

	// Connect registry to event bus for real-time streaming events from CLI adapters
	registry.SetEventHandler(func(event core.AgentEvent) {
		// Convert core.AgentEvent to chatOutputNotifier event format
		data := make(map[string]interface{})
		for k, v := range event.Data {
			data[k] = v
		}
		outputNotifier.AgentEvent(string(event.Type), event.Agent, event.Message, data)
	})

	// Create mode enforcer
	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
		DryRun:      runnerConfig.DryRun,
		Sandbox:     runnerConfig.Sandbox,
		DeniedTools: runnerConfig.DenyTools,
		MaxCost:     runnerConfig.MaxCostPerWorkflow,
	})
	modeEnforcerAdapter := workflow.NewModeEnforcerAdapter(modeEnforcer)

	// Create workflow runner
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:         runnerConfig,
		State:          stateAdapter,
		Agents:         registry,
		DAG:            dagAdapter,
		Checkpoint:     checkpointAdapter,
		ResumeProvider: resumeAdapter,
		Prompts:        promptAdapter,
		Retry:          retryAdapter,
		RateLimits:     rateLimiterAdapter,
		Worktrees:      worktreeManager,
		Logger:         logger,
		Output:         outputNotifier,
		ModeEnforcer:   modeEnforcerAdapter,
		Control:        controlPlane,
	})

	return runner, nil
}

// chatOutputNotifier publishes workflow events to the event bus.
// Implements workflow.OutputNotifier interface.
type chatOutputNotifier struct {
	eventBus *events.EventBus
}

func (n *chatOutputNotifier) PhaseStarted(_ core.Phase)                   {}
func (n *chatOutputNotifier) TaskStarted(_ *core.Task)                    {}
func (n *chatOutputNotifier) TaskCompleted(_ *core.Task, _ time.Duration) {}
func (n *chatOutputNotifier) TaskFailed(_ *core.Task, _ error)            {}
func (n *chatOutputNotifier) TaskSkipped(_ *core.Task, _ string)          {}
func (n *chatOutputNotifier) WorkflowStateUpdated(_ *core.WorkflowState)  {}
func (n *chatOutputNotifier) Log(level, source, message string) {
	if n.eventBus != nil {
		fullMessage := "[" + source + "] " + message
		n.eventBus.Publish(events.NewLogEvent("", level, fullMessage, nil))
	}
}

func (n *chatOutputNotifier) AgentEvent(kind, agent, message string, data map[string]interface{}) {
	if n.eventBus != nil {
		n.eventBus.Publish(events.NewAgentStreamEvent("", events.AgentEventType(kind), agent, message).WithData(data))
	}
}
