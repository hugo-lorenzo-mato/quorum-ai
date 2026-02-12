package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
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
	runInteractive  bool
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
	runCmd.Flags().BoolVar(&runInteractive, "interactive", false, "Pause between phases for review and feedback")

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

func runWorkflow(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt, stopping...")
		cancel()
	}()

	if err := validateSingleAgentFlags(); err != nil {
		return err
	}
	if runInteractive {
		return runInteractiveWorkflow(ctx, args)
	}

	output, outputMode, tuiLogHandler := setupRunOutput()
	defer func() { _ = output.Close() }()

	loader := config.NewLoaderWithViper(viper.GetViper())
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	projectRoot := loader.ProjectDir()
	if err := config.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	logger := createRunLogger(cfg, outputMode, tuiLogHandler)

	tuiOutput, tuiErrCh := setupRunTUI(output, outputMode, tuiLogHandler, projectRoot)

	stateManager, err := createRunStateManager(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := state.CloseStateManager(stateManager); closeErr != nil {
			logger.Warn("closing state manager", "error", closeErr)
		}
	}()

	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	runnerConfig, err := buildRunnerConfig(cfg)
	if err != nil {
		return err
	}

	traceWriter, traceCleanup := setupRunTrace(ctx, cfg, logger)
	if traceCleanup != nil {
		defer traceCleanup()
	}

	runner, outputNotifier, err := createRunnerWithDeps(ctx, cfg, runnerConfig, stateManager, registry, logger, output, traceWriter, projectRoot)
	if err != nil {
		return err
	}

	registry.SetEventHandler(func(event core.AgentEvent) {
		outputNotifier.AgentEvent(string(event.Type), event.Agent, event.Message, event.Data)
	})

	if runResume {
		logger.Info("resuming workflow from checkpoint")
		output.WorkflowStarted("(resuming)")
		if err := runner.Resume(ctx); err != nil {
			output.WorkflowFailed(err)
			return handleTUICompletion(tuiErrCh, err)
		}
		if st, err := runner.GetState(ctx); err == nil && st != nil {
			output.WorkflowCompleted(st)
		}
		return handleTUICompletion(tuiErrCh, nil)
	}

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
	if st, err := runner.GetState(ctx); err == nil && st != nil {
		output.WorkflowCompleted(st)
	}
	_ = tuiOutput // keep reference alive for defers
	return handleTUICompletion(tuiErrCh, nil)
}

func setupRunOutput() (tui.Output, tui.OutputMode, *tui.TUILogHandler) {
	detector := tui.NewDetector()
	if runOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(runOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()
	verboseOutput := runTrace != ""
	output := tui.NewOutput(outputMode, useColor, verboseOutput)

	var tuiLogHandler *tui.TUILogHandler
	if outputMode == tui.ModeTUI {
		tuiLogHandler = tui.NewTUILogHandler(nil, slog.LevelInfo) // replaced in createRunLogger
	}
	return output, outputMode, tuiLogHandler
}

func createRunLogger(cfg *config.Config, outputMode tui.OutputMode, tuiLogHandler *tui.TUILogHandler) *logging.Logger {
	if outputMode == tui.ModeTUI {
		handler := tui.NewTUILogHandler(nil, parseLogLevel(cfg.Log.Level))
		*tuiLogHandler = *handler
		return logging.NewWithHandler(tuiLogHandler)
	}
	return logging.New(logging.Config{Level: cfg.Log.Level, Format: cfg.Log.Format, Output: os.Stdout})
}

func setupRunTUI(output tui.Output, outputMode tui.OutputMode, tuiLogHandler *tui.TUILogHandler, projectRoot string) (*tui.TUIOutput, chan error) {
	if outputMode != tui.ModeTUI {
		return nil, nil
	}
	tuiOutput, ok := output.(*tui.TUIOutput)
	if !ok {
		return nil, nil
	}
	baseDir := filepath.Join(projectRoot, ".quorum")
	tuiOutput.SetModel(tui.NewWithStateManager(baseDir))
	if tuiLogHandler != nil {
		tuiLogHandler.SetOutput(tuiOutput)
	}
	tuiErrCh := make(chan error, 1)
	go func() { tuiErrCh <- tuiOutput.Start() }()
	return tuiOutput, tuiErrCh
}

func createRunStateManager(cfg *config.Config, logger *logging.Logger) (core.StateManager, error) {
	statePath := cfg.State.Path
	if statePath == "" {
		statePath = ".quorum/state/state.db"
	}
	stateOpts := state.StateManagerOptions{BackupPath: cfg.State.BackupPath}
	if cfg.State.LockTTL != "" {
		if lockTTL, err := time.ParseDuration(cfg.State.LockTTL); err == nil {
			stateOpts.LockTTL = lockTTL
		} else {
			logger.Warn("invalid state.lock_ttl, using default", "value", cfg.State.LockTTL, "error", err)
		}
	}
	sm, err := state.NewStateManagerWithOptions(statePath, stateOpts)
	if err != nil {
		return nil, fmt.Errorf("creating state manager: %w", err)
	}
	return sm, nil
}

func buildRunnerConfig(cfg *config.Config) (*workflow.RunnerConfig, error) {
	timeout, err := parseDurationDefault(cfg.Workflow.Timeout, defaultWorkflowTimeout)
	if err != nil {
		return nil, fmt.Errorf("parsing workflow timeout %q: %w", cfg.Workflow.Timeout, err)
	}
	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
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

	refinerEnabled := cfg.Phases.Analyze.Refiner.Enabled && !runSkipOptimize
	return &workflow.RunnerConfig{
		Timeout: timeout, MaxRetries: runMaxRetries, DryRun: runDryRun,
		DenyTools: cfg.Workflow.DenyTools, DefaultAgent: defaultAgent,
		AgentPhaseModels: map[string]map[string]string{
			"claude": cfg.Agents.Claude.PhaseModels, "gemini": cfg.Agents.Gemini.PhaseModels,
			"codex": cfg.Agents.Codex.PhaseModels, "copilot": cfg.Agents.Copilot.PhaseModels,
			"opencode": cfg.Agents.OpenCode.PhaseModels,
		},
		WorktreeAutoClean: cfg.Git.Worktree.AutoClean, WorktreeMode: cfg.Git.Worktree.Mode,
		Refiner: workflow.RefinerConfig{
			Enabled: refinerEnabled, Agent: cfg.Phases.Analyze.Refiner.Agent, Template: cfg.Phases.Analyze.Refiner.Template,
		},
		Synthesizer:     workflow.SynthesizerConfig{Agent: cfg.Phases.Analyze.Synthesizer.Agent},
		PlanSynthesizer: workflow.PlanSynthesizerConfig{Enabled: cfg.Phases.Plan.Synthesizer.Enabled, Agent: cfg.Phases.Plan.Synthesizer.Agent},
		Moderator: workflow.ModeratorConfig{
			Enabled: cfg.Phases.Analyze.Moderator.Enabled, Agent: cfg.Phases.Analyze.Moderator.Agent,
			Threshold: cfg.Phases.Analyze.Moderator.Threshold, MinRounds: cfg.Phases.Analyze.Moderator.MinRounds,
			MaxRounds: cfg.Phases.Analyze.Moderator.MaxRounds, WarningThreshold: cfg.Phases.Analyze.Moderator.WarningThreshold,
			StagnationThreshold: cfg.Phases.Analyze.Moderator.StagnationThreshold,
		},
		SingleAgent:   buildSingleAgentConfig(cfg),
		PhaseTimeouts: workflow.PhaseTimeouts{Analyze: analyzeTimeout, Plan: planTimeout, Execute: executeTimeout},
		Finalization: workflow.FinalizationConfig{
			AutoCommit: cfg.Git.Task.AutoCommit, AutoPush: cfg.Git.Finalization.AutoPush,
			AutoPR: cfg.Git.Finalization.AutoPR, AutoMerge: cfg.Git.Finalization.AutoMerge,
			PRBaseBranch: cfg.Git.Finalization.PRBaseBranch, MergeStrategy: cfg.Git.Finalization.MergeStrategy,
			Remote: cfg.GitHub.Remote,
		},
		Report: report.Config{Enabled: cfg.Report.Enabled, BaseDir: cfg.Report.BaseDir, UseUTC: cfg.Report.UseUTC, IncludeRaw: cfg.Report.IncludeRaw},
	}, nil
}

func setupRunTrace(ctx context.Context, cfg *config.Config, logger *logging.Logger) (service.TraceWriter, func()) {
	traceCfg, err := parseTraceConfig(cfg, runTrace)
	if err != nil {
		return service.NewTraceWriter(service.TraceConfig{Mode: "off"}, logger), nil
	}
	gitCommit, gitDirty := loadGitInfo()
	traceWriter := service.NewTraceWriter(traceCfg, logger)
	if !traceWriter.Enabled() {
		return traceWriter, nil
	}
	traceRunID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	if err := traceWriter.StartRun(ctx, service.TraceRunInfo{
		RunID: traceRunID, WorkflowID: traceRunID, StartedAt: time.Now(),
		GitCommit: gitCommit, GitDirty: gitDirty,
	}); err != nil {
		logger.Warn("failed to start trace run", "error", err)
		return traceWriter, nil
	}
	logger.Info("trace enabled", "mode", traceCfg.Mode, "dir", traceWriter.Dir())
	return traceWriter, func() {
		summary := traceWriter.EndRun(ctx)
		if summary.TotalEvents > 0 {
			logger.Info("trace completed", "events", summary.TotalEvents, "dir", summary.Dir)
		}
	}
}

func createRunnerWithDeps(
	ctx context.Context, cfg *config.Config, runnerConfig *workflow.RunnerConfig,
	stateManager core.StateManager, registry *cli.Registry,
	logger *logging.Logger, output tui.Output, traceWriter service.TraceWriter,
	projectRoot string,
) (*workflow.Runner, workflow.OutputNotifier, error) {
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return nil, nil, fmt.Errorf("creating prompt renderer: %w", err)
	}

	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(runMaxRetries))
	rateLimiterRegistry := service.GetGlobalRateLimiter()
	dagBuilder := service.NewDAGBuilder()

	gitClient, err := git.NewClient(projectRoot)
	if err != nil {
		logger.Warn("failed to create git client, worktree isolation disabled", "error", err)
	}
	var worktreeManager workflow.WorktreeManager
	var workflowWorktrees core.WorkflowWorktreeManager
	if gitClient != nil {
		worktreeManager = git.NewTaskWorktreeManager(gitClient, cfg.Git.Worktree.Dir).WithLogger(logger)
		if repoRoot, rootErr := gitClient.RepoRoot(ctx); rootErr != nil {
			logger.Warn("failed to detect repo root, workflow git isolation disabled", "error", rootErr)
		} else if wtMgr, wtErr := git.NewWorkflowWorktreeManager(repoRoot, cfg.Git.Worktree.Dir, gitClient, logger.Logger); wtErr != nil {
			logger.Warn("failed to create workflow worktree manager, workflow git isolation disabled", "error", wtErr)
		} else {
			workflowWorktrees = wtMgr
		}
	}

	var githubClient core.GitHubClient
	if cfg.Git.Finalization.AutoPR {
		if ghClient, ghErr := github.NewClientFromRepo(); ghErr != nil {
			logger.Warn("failed to create GitHub client, PR creation disabled", "error", ghErr)
		} else {
			githubClient = ghClient
			logger.Info("GitHub client initialized for PR creation")
		}
	}

	baseNotifier := tui.NewOutputNotifierAdapter(output)
	var outputNotifier workflow.OutputNotifier
	if traceWriter.Enabled() {
		traceNotifier := service.NewTraceOutputNotifier(traceWriter)
		outputNotifier = tui.NewTracingOutputNotifierAdapter(baseNotifier, traceNotifier)
	} else {
		outputNotifier = baseNotifier
	}

	modeEnforcer := service.NewModeEnforcer(service.ExecutionMode{DryRun: runnerConfig.DryRun, DeniedTools: runnerConfig.DenyTools})

	runner, err := workflow.NewRunner(workflow.RunnerDeps{
		Config: runnerConfig, State: stateManager, Agents: registry,
		DAG: workflow.NewDAGAdapter(dagBuilder), Checkpoint: workflow.NewCheckpointAdapter(checkpointManager, ctx),
		ResumeProvider: workflow.NewResumePointAdapter(checkpointManager),
		Prompts: workflow.NewPromptRendererAdapter(promptRenderer),
		Retry: workflow.NewRetryAdapter(retryPolicy, ctx),
		RateLimits: workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx),
		Worktrees: worktreeManager, WorkflowWorktrees: workflowWorktrees,
		GitIsolation: workflow.DefaultGitIsolationConfig(), GitClientFactory: git.NewClientFactory(),
		Git: gitClient, GitHub: githubClient, Logger: logger, Output: outputNotifier,
		ModeEnforcer: workflow.NewModeEnforcerAdapter(modeEnforcer), ProjectRoot: projectRoot,
	})
	if err != nil {
		return nil, nil, err
	}
	return runner, outputNotifier, nil
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
	return cli.ConfigureRegistryFromConfig(registry, cfg)
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
