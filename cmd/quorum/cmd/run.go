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
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/state"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
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
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "Read prompt from file")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Simulate without executing")
	runCmd.Flags().BoolVar(&runYolo, "yolo", false, "Skip confirmations")
	runCmd.Flags().BoolVar(&runResume, "resume", false, "Resume from last checkpoint")
	runCmd.Flags().IntVar(&runMaxRetries, "max-retries", 3, "Maximum retry attempts")
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

	// Create logger
	logger := logging.New(logging.Config{
		Level:  viper.GetString("log.level"),
		Format: viper.GetString("log.format"),
	})

	// Create state manager
	statePath := viper.GetString("state.path")
	if statePath == "" {
		statePath = ".quorum/state/state.json"
	}
	stateManager := state.NewJSONStateManager(statePath)

	// Create agent registry
	registry := cli.NewRegistry()

	// Configure agents from config
	cfg, err := configureAgents(registry)
	if err != nil {
		return fmt.Errorf("configuring agents: %w", err)
	}

	// Create consensus checker
	threshold := viper.GetFloat64("consensus.threshold")
	if threshold == 0 {
		threshold = 0.75
	}
	consensusChecker := service.NewConsensusChecker(threshold, service.DefaultWeights())

	// Create prompt renderer
	promptRenderer, err := service.NewPromptRenderer()
	if err != nil {
		return fmt.Errorf("creating prompt renderer: %w", err)
	}

	// Create workflow runner config
	timeout := viper.GetDuration("workflow.timeout")
	if timeout == 0 {
		timeout = time.Hour
	}
	runnerConfig := &service.WorkflowRunnerConfig{
		Timeout:      timeout,
		MaxRetries:   runMaxRetries,
		DryRun:       runDryRun,
		Sandbox:      viper.GetBool("workflow.sandbox"),
		DenyTools:    viper.GetStringSlice("workflow.deny_tools"),
		DefaultAgent: viper.GetString("agents.default"),
		V3Agent:      "claude",
		AgentPhaseModels: map[string]map[string]string{
			"claude":  cfg.Agents.Claude.PhaseModels,
			"gemini":  cfg.Agents.Gemini.PhaseModels,
			"codex":   cfg.Agents.Codex.PhaseModels,
			"copilot": cfg.Agents.Copilot.PhaseModels,
			"aider":   cfg.Agents.Aider.PhaseModels,
		},
	}
	if runnerConfig.DefaultAgent == "" {
		runnerConfig.DefaultAgent = "claude"
	}

	// Create workflow runner
	runner := service.NewWorkflowRunner(
		runnerConfig,
		stateManager,
		registry,
		consensusChecker,
		promptRenderer,
		logger,
	)

	// Resume or run new workflow
	if runResume {
		logger.Info("resuming workflow from checkpoint")
		return runner.Resume(ctx)
	}

	// Get prompt for new workflow
	prompt, err := getPrompt(args, runFile)
	if err != nil {
		return err
	}

	logger.Info("starting new workflow", "prompt_length", len(prompt))
	return runner.Run(ctx, prompt)
}

func configureAgents(registry *cli.Registry) (*config.Config, error) {
	loader := config.NewLoader()
	if cfgFile != "" {
		loader.WithConfigFile(cfgFile)
	}
	cfg, err := loader.Load()
	if err != nil {
		return nil, err
	}
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

	return cfg, nil
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
