package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

var runCmd = &cobra.Command{
	Use:   "run [prompt]",
	Short: "Run a complete workflow",
	Long: `Execute a complete workflow including analyze, plan, and execute phases.
The prompt can be provided as an argument or via --file flag.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorkflow,
}

var (
	runFile       string
	runDryRun     bool
	runYolo       bool
	runResume     bool
	runMaxRetries int
	runTrace      string
	runOutput     string
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
}

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

	// Detect output mode
	detector := tui.NewDetector()
	if runOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(runOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()

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

	// Migrate state from legacy paths if needed
	if migrated, err := state.MigrateState(statePath, logger); err != nil {
		logger.Warn("state migration failed", "error", err)
	} else if migrated {
		logger.Info("migrated state from legacy path to", "path", statePath)
	}

	stateManager := state.NewJSONStateManager(statePath)

	// Create agent registry and configure from unified config
	registry := cli.NewRegistry()
	if err := configureAgentsFromConfig(registry, cfg, loader); err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	// Create consensus checker from unified config (80/60/50 escalation policy)
	consensusChecker := service.NewConsensusCheckerWithThresholds(
		cfg.Consensus.Threshold,
		cfg.Consensus.V2Threshold,
		cfg.Consensus.HumanThreshold,
		service.CategoryWeights{
			Claims:          cfg.Consensus.Weights.Claims,
			Risks:           cfg.Consensus.Weights.Risks,
			Recommendations: cfg.Consensus.Weights.Recommendations,
		},
	)

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
	timeout := time.Hour
	if cfg.Workflow.Timeout != "" {
		parsed, parseErr := time.ParseDuration(cfg.Workflow.Timeout)
		if parseErr != nil {
			return fmt.Errorf("parsing workflow timeout %q: %w", cfg.Workflow.Timeout, parseErr)
		}
		timeout = parsed
	}
	defaultAgent := cfg.Agents.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
	runnerConfig := &workflow.RunnerConfig{
		Timeout:      timeout,
		MaxRetries:   runMaxRetries,
		DryRun:       runDryRun,
		Sandbox:      cfg.Workflow.Sandbox,
		DenyTools:    cfg.Workflow.DenyTools,
		DefaultAgent: defaultAgent,
		V3Agent:      "claude",
		AgentPhaseModels: map[string]map[string]string{
			"claude":  cfg.Agents.Claude.PhaseModels,
			"gemini":  cfg.Agents.Gemini.PhaseModels,
			"codex":   cfg.Agents.Codex.PhaseModels,
			"copilot": cfg.Agents.Copilot.PhaseModels,
			"aider":   cfg.Agents.Aider.PhaseModels,
		},
		WorktreeAutoClean:  cfg.Git.AutoClean,
		MaxCostPerWorkflow: cfg.Costs.MaxPerWorkflow,
		MaxCostPerTask:     cfg.Costs.MaxPerTask,
	}

	// Store trace config for potential later use
	_ = traceCfg
	_ = gitCommit
	_ = gitDirty

	// Create service components needed by the modular runner
	checkpointManager := service.NewCheckpointManager(stateManager, logger)
	retryPolicy := service.NewRetryPolicy(service.WithMaxAttempts(runMaxRetries))
	rateLimiterRegistry := service.NewRateLimiterRegistry()
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
	if gitClient != nil {
		worktreeManager = git.NewTaskWorktreeManager(gitClient, cfg.Git.WorktreeDir)
	}

	// Create adapters for modular runner interfaces
	consensusAdapter := workflow.NewConsensusAdapter(consensusChecker)
	checkpointAdapter := workflow.NewCheckpointAdapter(checkpointManager, ctx)
	retryAdapter := workflow.NewRetryAdapter(retryPolicy, ctx)
	rateLimiterAdapter := workflow.NewRateLimiterRegistryAdapter(rateLimiterRegistry, ctx)
	promptAdapter := workflow.NewPromptRendererAdapter(promptRenderer)
	resumeAdapter := workflow.NewResumePointAdapter(checkpointManager)
	dagAdapter := workflow.NewDAGAdapter(dagBuilder)

	// Create state manager adapter that implements workflow.StateManager
	stateAdapter := &stateManagerAdapter{sm: stateManager}

	// Create workflow runner using modular architecture (ADR-0005)
	runner := workflow.NewRunner(workflow.RunnerDeps{
		Config:         runnerConfig,
		State:          stateAdapter,
		Agents:         registry,
		Consensus:      consensusAdapter,
		DAG:            dagAdapter,
		Checkpoint:     checkpointAdapter,
		ResumeProvider: resumeAdapter,
		Prompts:        promptAdapter,
		Retry:          retryAdapter,
		RateLimits:     rateLimiterAdapter,
		Worktrees:      worktreeManager,
		Logger:         logger,
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
// The loader is used to check if values are explicitly set in config vs defaults.
func configureAgentsFromConfig(registry *cli.Registry, cfg *config.Config, loader *config.Loader) error {
	isEnabled := func(key, envKey string, enabled bool) bool {
		if !enabled {
			return false
		}
		if loader.Viper().InConfig(key) {
			return true
		}
		_, ok := os.LookupEnv(envKey)
		return ok
	}

	// Configure Claude
	if isEnabled("agents.claude.enabled", "QUORUM_AGENTS_CLAUDE_ENABLED", cfg.Agents.Claude.Enabled) {
		registry.Configure("claude", cli.AgentConfig{
			Name:        "claude",
			Path:        cfg.Agents.Claude.Path,
			Model:       cfg.Agents.Claude.Model,
			MaxTokens:   cfg.Agents.Claude.MaxTokens,
			Temperature: cfg.Agents.Claude.Temperature,
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Gemini
	if isEnabled("agents.gemini.enabled", "QUORUM_AGENTS_GEMINI_ENABLED", cfg.Agents.Gemini.Enabled) {
		registry.Configure("gemini", cli.AgentConfig{
			Name:        "gemini",
			Path:        cfg.Agents.Gemini.Path,
			Model:       cfg.Agents.Gemini.Model,
			MaxTokens:   cfg.Agents.Gemini.MaxTokens,
			Temperature: cfg.Agents.Gemini.Temperature,
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Codex
	if isEnabled("agents.codex.enabled", "QUORUM_AGENTS_CODEX_ENABLED", cfg.Agents.Codex.Enabled) {
		registry.Configure("codex", cli.AgentConfig{
			Name:        "codex",
			Path:        cfg.Agents.Codex.Path,
			Model:       cfg.Agents.Codex.Model,
			MaxTokens:   cfg.Agents.Codex.MaxTokens,
			Temperature: cfg.Agents.Codex.Temperature,
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Copilot
	if isEnabled("agents.copilot.enabled", "QUORUM_AGENTS_COPILOT_ENABLED", cfg.Agents.Copilot.Enabled) {
		registry.Configure("copilot", cli.AgentConfig{
			Name:        "copilot",
			Path:        cfg.Agents.Copilot.Path,
			MaxTokens:   cfg.Agents.Copilot.MaxTokens,
			Temperature: cfg.Agents.Copilot.Temperature,
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Aider
	if isEnabled("agents.aider.enabled", "QUORUM_AGENTS_AIDER_ENABLED", cfg.Agents.Aider.Enabled) {
		registry.Configure("aider", cli.AgentConfig{
			Name:        "aider",
			Path:        cfg.Agents.Aider.Path,
			Model:       cfg.Agents.Aider.Model,
			MaxTokens:   cfg.Agents.Aider.MaxTokens,
			Temperature: cfg.Agents.Aider.Temperature,
			Timeout:     5 * time.Minute,
		})
	}

	return nil
}

func getPrompt(args []string, file string) (string, error) {
	if file != "" {
		data, err := os.ReadFile(file)
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

// stateManagerAdapter adapts state.JSONStateManager to workflow.StateManager interface.
type stateManagerAdapter struct {
	sm *state.JSONStateManager
}

func (a *stateManagerAdapter) Save(ctx context.Context, st *core.WorkflowState) error {
	return a.sm.Save(ctx, st)
}

func (a *stateManagerAdapter) Load(ctx context.Context) (*core.WorkflowState, error) {
	return a.sm.Load(ctx)
}

func (a *stateManagerAdapter) AcquireLock(ctx context.Context) error {
	return a.sm.AcquireLock(ctx)
}

func (a *stateManagerAdapter) ReleaseLock(ctx context.Context) error {
	return a.sm.ReleaseLock(ctx)
}
