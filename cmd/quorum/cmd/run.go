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
	configureAgents(registry)

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

func configureAgents(registry *cli.Registry) {
	// Configure Claude
	if viper.GetBool("agents.claude.enabled") {
		registry.Configure("claude", cli.AgentConfig{
			Name:        "claude",
			Path:        viper.GetString("agents.claude.path"),
			Model:       viper.GetString("agents.claude.model"),
			MaxTokens:   viper.GetInt("agents.claude.max_tokens"),
			Temperature: viper.GetFloat64("agents.claude.temperature"),
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Gemini
	if viper.GetBool("agents.gemini.enabled") {
		registry.Configure("gemini", cli.AgentConfig{
			Name:        "gemini",
			Path:        viper.GetString("agents.gemini.path"),
			Model:       viper.GetString("agents.gemini.model"),
			MaxTokens:   viper.GetInt("agents.gemini.max_tokens"),
			Temperature: viper.GetFloat64("agents.gemini.temperature"),
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Codex
	if viper.GetBool("agents.codex.enabled") {
		registry.Configure("codex", cli.AgentConfig{
			Name:        "codex",
			Path:        viper.GetString("agents.codex.path"),
			Model:       viper.GetString("agents.codex.model"),
			MaxTokens:   viper.GetInt("agents.codex.max_tokens"),
			Temperature: viper.GetFloat64("agents.codex.temperature"),
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Copilot
	if viper.GetBool("agents.copilot.enabled") {
		registry.Configure("copilot", cli.AgentConfig{
			Name:        "copilot",
			Path:        viper.GetString("agents.copilot.path"),
			MaxTokens:   viper.GetInt("agents.copilot.max_tokens"),
			Temperature: viper.GetFloat64("agents.copilot.temperature"),
			Timeout:     5 * time.Minute,
		})
	}

	// Configure Aider
	if viper.GetBool("agents.aider.enabled") {
		registry.Configure("aider", cli.AgentConfig{
			Name:        "aider",
			Path:        viper.GetString("agents.aider.path"),
			Model:       viper.GetString("agents.aider.model"),
			MaxTokens:   viper.GetInt("agents.aider.max_tokens"),
			Temperature: viper.GetFloat64("agents.aider.temperature"),
			Timeout:     5 * time.Minute,
		})
	}
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
