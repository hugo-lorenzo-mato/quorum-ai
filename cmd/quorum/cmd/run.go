package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/fsutil"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

var runCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Run a complete workflow",
	Long: `Execute a complete workflow including analyze, plan, and execute phases.
The prompt can be provided as an argument or via --file flag.

By default, workflows use multi-agent consensus mode where multiple agents
analyze the task and reach agreement through moderated discussion.

For simpler tasks, use --single-agent mode for faster execution with a single agent.`,
	Example: `  # Multi-agent mode (default) - best for complex tasks
  quorum run "Implement user authentication with JWT"

  # Single-agent mode - faster for simple tasks
  quorum run "Fix the null pointer in auth.go" --single-agent --agent claude

  # Single-agent with specific model
  quorum run "Add docstrings" --single-agent --agent claude --model claude-3-haiku

  # Available agents: claude, gemini, codex (if enabled in config)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorkflow,
}

var (
	runFile         string
	runDryRun       bool
	runYolo         bool
	runResume       bool
	runMaxRetries   int
	runTrace        string
	runOutput       string
	runSkipOptimize bool
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "Read prompt from file")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Simulate without executing")
	runCmd.Flags().BoolVar(&runYolo, "yolo", false, "Skip confirmations")
	runCmd.Flags().BoolVar(&runResume, "resume", false, "Resume from last checkpoint")
	runCmd.Flags().IntVar(&runMaxRetries, "max-retries", 3, "Maximum retry attempts")
	runCmd.Flags().StringVar(&runTrace, "trace", "", "Trace mode (off, summary, full)")
	if flag := runCmd.Flags().Lookup("trace"); flag != nil {
		flag.NoOptDefVal = "summary"
	}
	runCmd.Flags().StringVarP(&runOutput, "output", "o", "", "Output mode (tui, plain, json, quiet)")
	runCmd.Flags().BoolVar(&runSkipOptimize, "skip-refine", false, "Skip prompt refinement phase")

	// Single-agent mode flags (using shared variables from common.go)
	runCmd.Flags().BoolVar(&singleAgent, "single-agent", false,
		"Run in single-agent mode (faster execution, no multi-agent consensus)")
	runCmd.Flags().StringVar(&agentName, "agent", "",
		"Agent to use for single-agent mode (e.g., 'claude', 'gemini', 'codex')")
	runCmd.Flags().StringVar(&agentModel, "model", "",
		"Override the agent's default model (optional, requires --single-agent)")
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

//nolint:gocyclo // Complexity is acceptable for CLI orchestration function
func runWorkflow(_ *cobra.Command, args []string) error {
	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt, stopping...")
		cancel()
	}()

	// Validate single-agent flags
	if err := validateSingleAgentFlags(); err != nil {
		return err
	}

	// Detect output mode
	detector := tui.NewDetector()
	if runOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(runOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()

	// Load unified configuration using global viper (includes flag bindings)
	// Precedence: flags > env > project config > user config > defaults
	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate configuration (catches invalid weights, thresholds, etc.)
	if err := config.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	// Create logger - for TUI mode we'll set up a custom handler later
	var logger *logging.Logger
	var tuiLogHandler *tui.TUILogHandler

	if outputMode == tui.ModeTUI {
		// Create a placeholder handler - we'll connect it to TUIOutput later
		tuiLogHandler = tui.NewTUILogHandler(nil, parseLogLevel(cfg.Log.Level))
		logger = logging.NewWithHandler(tuiLogHandler)
	} else {
		// Normal logging for non-TUI modes
		logger = logging.New(logging.Config{
			Level:  cfg.Log.Level,
			Format: cfg.Log.Format,
			Output: os.Stdout,
		})
	}

	// Create output handler based on detected mode
	// Verbose output is enabled when trace mode is set or log level is debug
	verboseOutput := runTrace != ""
	output := tui.NewOutput(outputMode, useColor, verboseOutput)
	defer func() { _ = output.Close() }()

	// For TUI mode, start the TUI in a goroutine and run workflow in background
	var tuiOutput *tui.TUIOutput
	var tuiErrCh chan error
	if outputMode == tui.ModeTUI {
		var ok bool
		tuiOutput, ok = output.(*tui.TUIOutput)
		if ok {
			baseDir := ".quorum"
			tuiOutput.SetModel(tui.NewWithStateManager(baseDir))

			// Connect TUILogHandler to TUIOutput so logs are routed directly
			if tuiLogHandler != nil {
				tuiLogHandler.SetOutput(tuiOutput)
			}

			tuiErrCh = make(chan error, 1)
			go func() {
				tuiErrCh <- tuiOutput.Start()
			}()
			// Give TUI time to initialize
			defer func() {
				if tuiOutput != nil {
					_ = tuiOutput.Close()
				}
			}()
		}
	}

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

	stateManager, err := state.NewStateManager(backend, statePath)
	if err != nil {
		return fmt.Errorf("creating state manager: %w", err)
	}
	defer func() {
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			logger.Warn("closing state manager", "error", closeErr)
		}
	}()

	// Create agent registry and configure from unified config
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return fmt.Errorf("creating prompt renderer: %w", err)
	}

	traceCfg, err := parseTraceConfig(cfg, runTrace)
	if err != nil {
		return err
	}

	gitCommit, gitDirty := loadGitInfo()

	// Create workflow runner config from unified config
	timeout, err := parseDurationDefault(cfg.Workflow.Timeout, defaultWorkflowTimeout)
	if err != nil {
		return fmt.Errorf("parsing workflow timeout %q: %w", cfg.Workflow.Timeout, err)
	}
	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
	// Parse phase timeouts
	analyzeTimeout, err := parseDurationDefault(cfg.Phases.Analyze.Timeout, defaultPhaseTimeout)
	if err != nil {
		return fmt.Errorf("parsing analyze phase timeout %q: %w", cfg.Phases.Analyze.Timeout, err)
	}
	planTimeout, err := parseDurationDefault(cfg.Phases.Plan.Timeout, defaultPhaseTimeout)
	if err != nil {
		return fmt.Errorf("parsing plan phase timeout %q: %w", cfg.Phases.Plan.Timeout, err)
	}
	executeTimeout, err := parseDurationDefault(cfg.Phases.Execute.Timeout, defaultPhaseTimeout)
	if err != nil {
		return fmt.Errorf("parsing execute phase timeout %q: %w", cfg.Phases.Execute.Timeout, err)
	}

	// Refiner config: disabled if --skip-refine flag is set
	refinerEnabled := cfg.Phases.Analyze.Refiner.Enabled && !runSkipOptimize
	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   runMaxRetries,
		DryRun:       runDryRun,
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
		WorktreeAutoClean: cfg.Git.Worktree.AutoClean,
		WorktreeMode:      cfg.Git.Worktree.Mode,
		Refiner: workflow.RefinerConfig{
			Enabled: refinerEnabled,
			Agent:   cfg.Phases.Analyze.Refiner.Agent,
		},
		Synthesizer: workflow.SynthesizerConfig{
			Agent: cfg.Phases.Analyze.Synthesizer.Agent,
		},
		PlanSynthesizer: workflow.PlanSynthesizerConfig{
			Enabled: cfg.Phases.Plan.Synthesizer.Enabled,
			Agent:   cfg.Phases.Plan.Synthesizer.Agent,
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
		Finalization: workflow.FinalizationConfig{
			AutoCommit:    cfg.Git.Task.AutoCommit,
			AutoPush:      cfg.Git.Finalization.AutoPush,
			AutoPR:        cfg.Git.Finalization.AutoPR,
			AutoMerge:     cfg.Git.Finalization.AutoMerge,
			PRBaseBranch:  cfg.Git.Finalization.PRBaseBranch,
			MergeStrategy: cfg.Git.Finalization.MergeStrategy,
			Remote:        cfg.GitHub.Remote,
		},
	}

	// Create trace writer if tracing is enabled
	traceWriter := service.NewTraceWriter(traceCfg, logger)
	if traceWriter.Enabled() {
		// Generate workflow ID early for trace directory
		traceRunID := fmt.Sprintf("run-%d", time.Now().UnixNano())
		if err := traceWriter.StartRun(ctx, service.TraceRunInfo{
			RunID:      traceRunID,
			WorkflowID: traceRunID,
			StartedAt:  time.Now(),
			GitCommit:  gitCommit,
			GitDirty:   gitDirty,
		}); err != nil {
			logger.Warn("failed to start trace run", "error", err)
		} else {
			logger.Info("trace enabled", "mode", traceCfg.Mode, "dir", traceWriter.Dir())
			defer func() {
				summary := traceWriter.EndRun(ctx)
				if summary.TotalEvents > 0 {
					logger.Info("trace completed",
						"events", summary.TotalEvents,
						"dir", summary.Dir,
					)
				}
			}()
		}
	}

	// Create service components needed by the modular runner
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(runMaxRetries))
	rateLimiterRegistry := service.GetGlobalRateLimiter()
	dagBuilder := service.NewDAGBuilder()

	// Create worktree manager for task isolation
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	gitClient, err := git.NewClient(cwd)
	if err != nil {
		logger.Warn("failed to create git client, worktree isolation disabled", "error", err)
	}
	var worktreeManager workflow.WorktreeManager
	var workflowWorktrees core.WorkflowWorktreeManager
	if gitClient != nil {
		worktreeManager = git.NewTaskWorktreeManager(gitClient, cfg.Git.Worktree.Dir).WithLogger(logger)

		repoRoot, rootErr := gitClient.RepoRoot(ctx)
		if rootErr != nil {
			logger.Warn("failed to detect repo root, workflow git isolation disabled", "error", rootErr)
		} else {
			wtMgr, wtErr := git.NewWorkflowWorktreeManager(repoRoot, cfg.Git.Worktree.Dir, gitClient, logger.Logger)
			if wtErr != nil {
				logger.Warn("failed to create workflow worktree manager, workflow git isolation disabled", "error", wtErr)
			} else {
				workflowWorktrees = wtMgr
			}
		}
	}

	// Create GitHub client for PR creation (only if auto_pr is enabled)
	var githubClient core.GitHubClient
	if cfg.Git.Finalization.AutoPR {
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

	// Create output notifier adapter for real-time TUI updates
	baseNotifier := tui.NewOutputNotifierAdapter(output)

	// Wrap with trace notifier if tracing is enabled
	var outputNotifier workflow.OutputNotifier
	if traceWriter.Enabled() {
		traceNotifier := service.NewTraceOutputNotifier(traceWriter)
		outputNotifier = tui.NewTracingOutputNotifierAdapter(baseNotifier, traceNotifier)
	} else {
		outputNotifier = baseNotifier
	}

	// Create mode enforcer from config
	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{
		DryRun:      runnerConfig.DryRun,
		Sandbox:     runnerConfig.Sandbox,
		DeniedTools: runnerConfig.DenyTools,
	})
	modeEnforcerAdapter := workflow.NewModeEnforcerAdapter(modeEnforcer)

	// Create workflow runner using modular architecture (ADR-0005)
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:            runnerConfig,
		State:             stateAdapter,
		Agents:            registry,
		DAG:               dagAdapter,
		Checkpoint:        checkpointAdapter,
		ResumeProvider:    resumeAdapter,
		Prompts:           promptAdapter,
		Retry:             retryAdapter,
		RateLimits:        rateLimiterAdapter,
		Worktrees:         worktreeManager,
		WorkflowWorktrees: workflowWorktrees,
		GitIsolation:      workflow.DefaultGitIsolationConfig(),
		GitClientFactory:  git.NewClientFactory(),
		Git:               gitClient,
		GitHub:            githubClient,
		Logger:            logger,
		Output:            outputNotifier,
		ModeEnforcer:      modeEnforcerAdapter,
	})

	// Connect agent streaming events to the output notifier for real-time progress
	registry.SetEventHandler(func(event core.AgentEvent) {
		outputNotifier.AgentEvent(string(event.Type), event.Agent, event.Message, event.Data)
	})

	// Resume or run new workflow
	if runResume {
		logger.Info("resuming workflow from checkpoint")
		output.WorkflowStarted("(resuming)")
		if err := runner.Resume(ctx); err != nil {
			output.WorkflowFailed(err)
			return handleTUICompletion(tuiErrCh, err)
		}
		// Get state for completion notification
		if state, err := runner.GetState(ctx); err == nil && state != nil {
			output.WorkflowCompleted(state)
		}
		return handleTUICompletion(tuiErrCh, nil)
	}

	// Get prompt for new workflow
	prompt, err := getPrompt(args, runFile)
	if err != nil {
		return err
	}

	logger.Info("starting new workflow", "prompt_length", len(prompt))
	output.WorkflowStarted(prompt)
	if err := runner.Run(ctx, prompt); err != nil {
		output.WorkflowFailed(err)
		return handleTUICompletion(tuiErrCh, err)
	}

	// Get state for completion notification
	if state, err := runner.GetState(ctx); err == nil && state != nil {
		output.WorkflowCompleted(state)
	}
	return handleTUICompletion(tuiErrCh, nil)
}

// handleTUICompletion waits for TUI to finish if running.
func handleTUICompletion(tuiErrCh chan error, workflowErr error) error {
	if tuiErrCh != nil {
		// Wait a bit for TUI to render final state, then signal quit
		select {
		case tuiErr := <-tuiErrCh:
			// TUI exited first (user pressed q)
			if tuiErr != nil && workflowErr == nil {
				return tuiErr
			}
		default:
			// TUI still running, give it time to display final state
		}
	}
	return workflowErr
}

func loadGitInfo() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	client, err := git.NewClient(cwd)
	if err != nil {
		return "", false
	}

	commit, err := client.CurrentCommit(context.Background())
	if err != nil {
		return "", false
	}

	status, err := client.StatusLocal(context.Background())
	if err != nil {
		return commit, false
	}

	return commit, !status.IsClean()
}

func parseTraceConfig(cfg *config.Config, override string) (service.TraceConfig, error) {
	trace := service.TraceConfig{
		Mode:            cfg.Trace.Mode,
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

	if override != "" {
		trace.Mode = override
	}

	switch trace.Mode {
	case "", "off", "summary", "full":
		if trace.Mode == "" {
			trace.Mode = "off"
		}
		return trace, nil
	default:
		return service.TraceConfig{}, fmt.Errorf("invalid trace mode: %s", trace.Mode)
	}
}

// configureAgentsFromConfig configures agents in the registry using unified config.
// Agents are configured if their enabled flag is true in the config.
func configureAgentsFromConfig(registry *cli.Registry, cfg *config.Config, _ *config.Loader) error {
	// Configure Claude
	if cfg.Agents.Claude.Enabled {
		registry.Configure("claude", cli.AgentConfig{
			Name:                      "claude",
			Path:                      cfg.Agents.Claude.Path,
			Model:                     cfg.Agents.Claude.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Claude.Phases,
			ReasoningEffort:           cfg.Agents.Claude.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Claude.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Claude.TokenDiscrepancyThreshold),
		})
	}

	// Configure Gemini
	if cfg.Agents.Gemini.Enabled {
		registry.Configure("gemini", cli.AgentConfig{
			Name:                      "gemini",
			Path:                      cfg.Agents.Gemini.Path,
			Model:                     cfg.Agents.Gemini.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Gemini.Phases,
			ReasoningEffort:           cfg.Agents.Gemini.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Gemini.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Gemini.TokenDiscrepancyThreshold),
		})
	}

	// Configure Codex
	if cfg.Agents.Codex.Enabled {
		registry.Configure("codex", cli.AgentConfig{
			Name:                      "codex",
			Path:                      cfg.Agents.Codex.Path,
			Model:                     cfg.Agents.Codex.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Codex.Phases,
			ReasoningEffort:           cfg.Agents.Codex.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Codex.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Codex.TokenDiscrepancyThreshold),
		})
	}

	// Configure Copilot
	if cfg.Agents.Copilot.Enabled {
		registry.Configure("copilot", cli.AgentConfig{
			Name:                      "copilot",
			Path:                      cfg.Agents.Copilot.Path,
			Model:                     cfg.Agents.Copilot.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Copilot.Phases,
			ReasoningEffort:           cfg.Agents.Copilot.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Copilot.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Copilot.TokenDiscrepancyThreshold),
		})
	}

	// Configure OpenCode
	if cfg.Agents.OpenCode.Enabled {
		registry.Configure("opencode", cli.AgentConfig{
			Name:                      "opencode",
			Path:                      cfg.Agents.OpenCode.Path,
			Model:                     cfg.Agents.OpenCode.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.OpenCode.Phases,
			ReasoningEffort:           cfg.Agents.OpenCode.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.OpenCode.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.OpenCode.TokenDiscrepancyThreshold),
		})
	}

	return nil
}

// getTokenDiscrepancyThreshold returns the configured threshold or the default.
func getTokenDiscrepancyThreshold(configured float64) float64 {
	if configured > 0 {
		return configured
	}
	return cli.DefaultTokenDiscrepancyThreshold
}

func getPrompt(args []string, file string) (string, error) {
	if file != "" {
		data, err := fsutil.ReadFileScoped(file)
		if err != nil {
			return "", fmt.Errorf("reading prompt file: %w", err)
		}
		return string(data), nil
	}

	if len(args) > 0 {
		return args[0], nil
	}

	return "", fmt.Errorf("prompt required: provide as argument or use --file")
}
