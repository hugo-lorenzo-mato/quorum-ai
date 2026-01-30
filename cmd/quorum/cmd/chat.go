package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
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
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/diagnostics"
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
	chatTrace string
)

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Default agent (claude, gemini, codex, copilot)")
	chatCmd.Flags().StringVar(&chatModel, "model", "", "Default model override")
	chatCmd.Flags().StringVar(&chatTrace, "trace", "", "Trace mode override (off, summary, full)")

	// Single-agent mode flags
	chatCmd.Flags().BoolVar(&singleAgent, "single-agent", false,
		"Run in single-agent mode (faster execution, no multi-agent consensus)")
	// Note: chat already has --agent and --model, we'll reuse them if provided
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

	// Reconcile chat-specific flags with shared single-agent variables
	if chatAgent != "" && agentName == "" {
		agentName = chatAgent
	}
	if chatModel != "" && agentModel == "" {
		agentModel = chatModel
	}

	// Validate single-agent flags
	if err := validateSingleAgentFlags(); err != nil {
		return err
	}

	// Load configuration
	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Create logger (discard in TUI mode to prevent corrupting the display)
	logger := logging.New(logging.Config{
		Level:  "error",
		Format: "text",
		Output: io.Discard,
	})

	// Create event bus with larger buffer to reduce event drops during high activity
	eventBus := events.New(500)
	defer eventBus.Close()

	// Create control plane
	controlPlane := control.New()

	// Create agent registry
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	// Initialize diagnostics if enabled
	var resourceMonitor *diagnostics.ResourceMonitor
	var crashDumpWriter *diagnostics.CrashDumpWriter
	var safeExecutor *diagnostics.SafeExecutor

	if cfg.Diagnostics.Enabled {
		// Create a silent slog logger for diagnostics (TUI mode)
		diagLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

		// Parse monitoring interval
		monitorInterval, err := time.ParseDuration(cfg.Diagnostics.ResourceMonitoring.Interval)
		if err != nil {
			monitorInterval = 30 * time.Second
		}

		// Create resource monitor
		resourceMonitor = diagnostics.NewResourceMonitor(
			monitorInterval,
			cfg.Diagnostics.ResourceMonitoring.FDThresholdPercent,
			cfg.Diagnostics.ResourceMonitoring.GoroutineThreshold,
			cfg.Diagnostics.ResourceMonitoring.MemoryThresholdMB,
			cfg.Diagnostics.ResourceMonitoring.HistorySize,
			diagLogger,
		)
		resourceMonitor.Start(ctx)
		defer resourceMonitor.Stop()

		// Create crash dump writer
		crashDumpWriter = diagnostics.NewCrashDumpWriter(
			cfg.Diagnostics.CrashDump.Dir,
			cfg.Diagnostics.CrashDump.MaxFiles,
			cfg.Diagnostics.CrashDump.IncludeStack,
			cfg.Diagnostics.CrashDump.IncludeEnv,
			diagLogger,
			resourceMonitor,
		)

		// Create safe executor
		safeExecutor = diagnostics.NewSafeExecutor(
			resourceMonitor,
			crashDumpWriter,
			diagLogger,
			cfg.Diagnostics.PreflightChecks.Enabled,
			cfg.Diagnostics.PreflightChecks.MinFreeFDPercent,
			cfg.Diagnostics.PreflightChecks.MinFreeMemoryMB,
		)

		// Inject diagnostics into all adapters
		registry.SetDiagnostics(safeExecutor, crashDumpWriter)
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

	// Determine trace mode (flag overrides config)
	traceMode := cfg.Trace.Mode
	if chatTrace != "" {
		traceMode = chatTrace
	}

	// Create workflow runner dependencies with optional tracing
	runner, traceCleanup, err := createWorkflowRunnerWithTracing(ctx, cfg, loader, registry, controlPlane, eventBus, logger, traceMode)
	if err != nil {
		return fmt.Errorf("creating workflow runner: %w", err)
	}
	defer traceCleanup()

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
			"claude-sonnet-4-5-20250929",
			"claude-haiku-4-5-20251001",
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
		}
	}
	if cfg.Agents.Gemini.Enabled {
		availableAgents = append(availableAgents, "gemini")
		agentModels["gemini"] = []string{
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			"gemini-3-pro-preview",
			"gemini-3-flash-preview",
		}
	}
	if cfg.Agents.Codex.Enabled {
		availableAgents = append(availableAgents, "codex")
		agentModels["codex"] = []string{
			"gpt-5.2-codex",
			"gpt-5.2",
			"gpt-5.1-codex-max",
			"gpt-5.1-codex",
			"gpt-5.1",
			"gpt-5",
			"gpt-5-mini",
			"gpt-4.1",
		}
	}
	if cfg.Agents.Copilot.Enabled {
		availableAgents = append(availableAgents, "copilot")
		agentModels["copilot"] = []string{
			"claude-sonnet-4.5",
			"claude-haiku-4.5",
			"claude-opus-4.5",
			"claude-sonnet-4",
			"gpt-5.2-codex",
			"gpt-5.1-codex-max",
			"gpt-5.1-codex",
			"gemini-3-pro-preview",
		}
	}
	if cfg.Agents.OpenCode.Enabled {
		availableAgents = append(availableAgents, "opencode")
		agentModels["opencode"] = []string{
			"qwen2.5-coder",
			"deepseek-coder-v2",
			"llama3.1",
			"deepseek-r1",
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
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), statePath)
	if err != nil {
		return nil, fmt.Errorf("creating state manager: %w", err)
	}

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

	// Parse phase timeouts
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

	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   3,
		DryRun:       false,
		Sandbox:      cfg.Workflow.Sandbox,
		DenyTools:    cfg.Workflow.DenyTools,
		DefaultAgent: defaultAgent,
		AgentPhaseModels: map[string]map[string]string{
			"claude":   cfg.Agents.Claude.PhaseModels,
			"gemini":   cfg.Agents.Gemini.PhaseModels,
			"codex":    cfg.Agents.Codex.PhaseModels,
			"copilot":  cfg.Agents.Copilot.PhaseModels,
			"opencode": cfg.Agents.OpenCode.PhaseModels,
		},
		WorktreeAutoClean: cfg.Git.AutoClean,
		WorktreeMode:      cfg.Git.WorktreeMode,
		Refiner: workflow.RefinerConfig{
			Enabled: cfg.Phases.Analyze.Refiner.Enabled,
			Agent:   cfg.Phases.Analyze.Refiner.Agent,
		},
		Synthesizer: workflow.SynthesizerConfig{
			Agent: cfg.Phases.Analyze.Synthesizer.Agent,
		},
		PlanSynthesizer: workflow.PlanSynthesizerConfig{
			Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
			Agent:   cfg.Phases.Plan.Synthesizer.Agent,
		},
		Report: report.Config{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
		Moderator: workflow.ModeratorConfig{
			Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
			Agent:               cfg.Phases.Analyze.Moderator.Agent,
			Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
			MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
			WarningThreshold:    cfg.Phases.Analyze.Moderator.WarningThreshold,
			StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
		},
		SingleAgent: buildSingleAgentConfig(cfg),
		PhaseTimeouts: workflow.PhaseTimeouts{
			Analyze: analyzeTimeout,
			Plan:    planTimeout,
			Execute: executeTimeout,
		},
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(3))
	rateLimiterRegistry := service.GetGlobalRateLimiter()
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
	// core.StateManager satisfies workflow.StateManager interface
	stateAdapter := stateManager

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
	})
	modeEnforcerAdapter := workflow.NewModeEnforcerAdapter(modeEnforcer)

	// Create workflow runner
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:           runnerConfig,
		State:            stateAdapter,
		Agents:           registry,
		DAG:              dagAdapter,
		Checkpoint:       checkpointAdapter,
		ResumeProvider:   resumeAdapter,
		Prompts:          promptAdapter,
		Retry:            retryAdapter,
		RateLimits:       rateLimiterAdapter,
		Worktrees:        worktreeManager,
		GitClientFactory: git.NewClientFactory(),
		Logger:           logger,
		Output:           outputNotifier,
		ModeEnforcer:     modeEnforcerAdapter,
		Control:          controlPlane,
	})

	return runner, nil
}

// chatOutputNotifier publishes workflow events to the event bus.
// Implements workflow.OutputNotifier interface.
type chatOutputNotifier struct {
	eventBus *events.EventBus
}

func (n *chatOutputNotifier) PhaseStarted(phase core.Phase) {
	if n.eventBus != nil {
		n.eventBus.Publish(events.NewPhaseStartedEvent("", string(phase)))
	}
}

func (n *chatOutputNotifier) TaskStarted(task *core.Task) {
	if n.eventBus != nil && task != nil {
		n.eventBus.Publish(events.NewTaskStartedEvent("", string(task.ID), ""))
	}
}

func (n *chatOutputNotifier) TaskCompleted(task *core.Task, duration time.Duration) {
	if n.eventBus != nil && task != nil {
		n.eventBus.Publish(events.NewTaskCompletedEvent("", string(task.ID), duration, task.TokensIn, task.TokensOut, task.CostUSD))
	}
}

func (n *chatOutputNotifier) TaskFailed(task *core.Task, err error) {
	if n.eventBus != nil && task != nil {
		n.eventBus.Publish(events.NewTaskFailedEvent("", string(task.ID), err, task.Retries > 0))
	}
}

func (n *chatOutputNotifier) TaskSkipped(task *core.Task, reason string) {
	if n.eventBus != nil && task != nil {
		n.eventBus.Publish(events.NewTaskSkippedEvent("", string(task.ID), reason))
	}
}

func (n *chatOutputNotifier) WorkflowStateUpdated(state *core.WorkflowState) {
	if n.eventBus != nil && state != nil {
		// Calculate task counts
		var completed, failed, skipped int
		for _, task := range state.Tasks {
			switch task.Status {
			case core.TaskStatusCompleted:
				completed++
			case core.TaskStatusFailed:
				failed++
			case core.TaskStatusSkipped:
				skipped++
			}
		}
		n.eventBus.Publish(events.NewWorkflowStateUpdatedEvent(
			string(state.WorkflowID),
			string(state.CurrentPhase),
			len(state.Tasks),
			completed,
			failed,
			skipped,
		))
	}
}
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

// createWorkflowRunnerWithTracing creates a workflow runner with optional tracing support.
// Returns a cleanup function that should be called when the TUI exits.
func createWorkflowRunnerWithTracing(
	ctx context.Context,
	cfg *config.Config,
	loader *config.Loader,
	registry *cli.Registry,
	controlPlane *control.ControlPlane,
	eventBus *events.EventBus,
	logger *logging.Logger,
	traceMode string,
) (*workflow.Runner, func(), error) {
	// If tracing is disabled or off, create runner without tracing
	if traceMode == "" || traceMode == "off" {
		runner, err := createWorkflowRunner(ctx, cfg, loader, registry, controlPlane, eventBus, logger)
		return runner, func() {}, err
	}

	// Create trace config from app config with mode override
	traceCfg := service.TraceConfig{
		Mode:            traceMode,
		Dir:             cfg.Trace.Dir,
		SchemaVersion:   cfg.Trace.SchemaVersion,
		Redact:          cfg.Trace.Redact,
		RedactPatterns:  cfg.Trace.RedactPatterns,
		RedactAllowlist: cfg.Trace.RedactAllowlist,
		MaxBytes:        cfg.Trace.MaxBytes,
		TotalMaxBytes:   cfg.Trace.TotalMaxBytes,
		MaxFiles:        cfg.Trace.MaxFiles,
		IncludePhases:   cfg.Trace.IncludePhases,
	}

	// Create trace writer
	traceWriter := service.NewTraceWriter(traceCfg, logger)
	if !traceWriter.Enabled() {
		runner, err := createWorkflowRunner(ctx, cfg, loader, registry, controlPlane, eventBus, logger)
		return runner, func() {}, err
	}

	// Start trace run
	traceRunID := fmt.Sprintf("chat-%d", time.Now().UnixNano())
	if err := traceWriter.StartRun(ctx, service.TraceRunInfo{
		RunID:      traceRunID,
		StartedAt:  time.Now(),
		AppVersion: GetVersion(),
		Config:     traceCfg,
	}); err != nil {
		logger.Warn("failed to start trace run", "error", err)
		runner, err := createWorkflowRunner(ctx, cfg, loader, registry, controlPlane, eventBus, logger)
		return runner, func() {}, err
	}

	logger.Info("trace enabled for chat session",
		"mode", traceMode,
		"run_id", traceRunID,
		"dir", traceWriter.Dir())

	// Create runner with trace-enabled output notifier
	runner, err := createWorkflowRunnerWithTrace(ctx, cfg, loader, registry, controlPlane, eventBus, logger, traceWriter)
	if err != nil {
		return nil, func() {}, err
	}

	// Create cleanup function
	cleanup := func() {
		summary := traceWriter.EndRun(ctx)
		if summary.TotalEvents > 0 {
			logger.Info("trace session ended",
				"events", summary.TotalEvents,
				"tokens_in", summary.TotalTokensIn,
				"tokens_out", summary.TotalTokensOut,
				"cost_usd", summary.TotalCostUSD,
				"dir", summary.Dir)
		}
	}

	return runner, cleanup, nil
}

// createWorkflowRunnerWithTrace is a variant of createWorkflowRunner that includes tracing.
func createWorkflowRunnerWithTrace(
	ctx context.Context,
	cfg *config.Config,
	_ *config.Loader,
	registry *cli.Registry,
	controlPlane *control.ControlPlane,
	eventBus *events.EventBus,
	logger *logging.Logger,
	traceWriter service.TraceWriter,
) (*workflow.Runner, error) {
	// Create state manager
	statePath := cfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	stateManager, err := state.NewStateManager(cfg.State.EffectiveBackend(), statePath)
	if err != nil {
		return nil, fmt.Errorf("creating state manager: %w", err)
	}

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

	// Parse phase timeouts
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

	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   3,
		DryRun:       false,
		Sandbox:      cfg.Workflow.Sandbox,
		DenyTools:    cfg.Workflow.DenyTools,
		DefaultAgent: defaultAgent,
		AgentPhaseModels: map[string]map[string]string{
			"claude":   cfg.Agents.Claude.PhaseModels,
			"gemini":   cfg.Agents.Gemini.PhaseModels,
			"codex":    cfg.Agents.Codex.PhaseModels,
			"copilot":  cfg.Agents.Copilot.PhaseModels,
			"opencode": cfg.Agents.OpenCode.PhaseModels,
		},
		WorktreeAutoClean: cfg.Git.AutoClean,
		WorktreeMode:      cfg.Git.WorktreeMode,
		Refiner: workflow.RefinerConfig{
			Enabled: cfg.Phases.Analyze.Refiner.Enabled,
			Agent:   cfg.Phases.Analyze.Refiner.Agent,
		},
		Synthesizer: workflow.SynthesizerConfig{
			Agent: cfg.Phases.Analyze.Synthesizer.Agent,
		},
		PlanSynthesizer: workflow.PlanSynthesizerConfig{
			Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
			Agent:   cfg.Phases.Plan.Synthesizer.Agent,
		},
		Report: report.Config{
			Enabled:    cfg.Report.Enabled,
			BaseDir:    cfg.Report.BaseDir,
			UseUTC:     cfg.Report.UseUTC,
			IncludeRaw: cfg.Report.IncludeRaw,
		},
		Moderator: workflow.ModeratorConfig{
			Enabled:             cfg.Phases.Analyze.Moderator.Enabled,
			Agent:               cfg.Phases.Analyze.Moderator.Agent,
			Threshold:           cfg.Phases.Analyze.Moderator.Threshold,
			MinRounds:           cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds:           cfg.Phases.Analyze.Moderator.MaxRounds,
			WarningThreshold:    cfg.Phases.Analyze.Moderator.WarningThreshold,
			StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
		},
		SingleAgent: buildSingleAgentConfig(cfg),
		PhaseTimeouts: workflow.PhaseTimeouts{
			Analyze: analyzeTimeout,
			Plan:    planTimeout,
			Execute: executeTimeout,
		},
	}

	// Create service components
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(3))
	rateLimiterRegistry := service.GetGlobalRateLimiter()
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
	// core.StateManager satisfies workflow.StateManager interface
	stateAdapter := stateManager

	// Create trace-enabled output notifier
	traceNotifier := service.NewTraceOutputNotifier(traceWriter)
	outputNotifier := &tracingChatOutputNotifier{
		eventBus: eventBus,
		tracer:   traceNotifier,
	}

	// Connect registry to event bus for real-time streaming events from CLI adapters
	registry.SetEventHandler(func(event core.AgentEvent) {
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
	})
	modeEnforcerAdapter := workflow.NewModeEnforcerAdapter(modeEnforcer)

	// Create workflow runner
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:           runnerConfig,
		State:            stateAdapter,
		Agents:           registry,
		DAG:              dagAdapter,
		Checkpoint:       checkpointAdapter,
		ResumeProvider:   resumeAdapter,
		Prompts:          promptAdapter,
		Retry:            retryAdapter,
		RateLimits:       rateLimiterAdapter,
		Worktrees:        worktreeManager,
		GitClientFactory: git.NewClientFactory(),
		Logger:           logger,
		Output:           outputNotifier,
		ModeEnforcer:     modeEnforcerAdapter,
		Control:          controlPlane,
	})

	return runner, nil
}

// tracingChatOutputNotifier extends chatOutputNotifier with tracing support.
type tracingChatOutputNotifier struct {
	eventBus *events.EventBus
	tracer   *service.TraceOutputNotifier
}

func (n *tracingChatOutputNotifier) PhaseStarted(phase core.Phase) {
	if n.tracer != nil {
		n.tracer.PhaseStarted(string(phase))
	}
}

func (n *tracingChatOutputNotifier) TaskStarted(task *core.Task) {
	if n.tracer != nil {
		n.tracer.TaskStarted(string(task.ID), task.Name, task.CLI)
	}
}

func (n *tracingChatOutputNotifier) TaskCompleted(task *core.Task, duration time.Duration) {
	if n.tracer != nil {
		n.tracer.TaskCompleted(string(task.ID), task.Name, duration, task.TokensIn, task.TokensOut, task.CostUSD)
	}
}

func (n *tracingChatOutputNotifier) TaskFailed(task *core.Task, err error) {
	if n.tracer != nil {
		n.tracer.TaskFailed(string(task.ID), task.Name, err)
	}
}

func (n *tracingChatOutputNotifier) TaskSkipped(_ *core.Task, _ string) {}

func (n *tracingChatOutputNotifier) WorkflowStateUpdated(state *core.WorkflowState) {
	if n.tracer != nil {
		n.tracer.WorkflowStateUpdated(string(state.Status), len(state.Tasks))
	}
}

func (n *tracingChatOutputNotifier) Log(level, source, message string) {
	if n.eventBus != nil {
		fullMessage := "[" + source + "] " + message
		n.eventBus.Publish(events.NewLogEvent("", level, fullMessage, nil))
	}
}

func (n *tracingChatOutputNotifier) AgentEvent(kind, agent, message string, data map[string]interface{}) {
	if n.eventBus != nil {
		n.eventBus.Publish(events.NewAgentStreamEvent("", events.AgentEventType(kind), agent, message).WithData(data))
	}
}
