package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
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

func runWorkflow(cmd *cobra.Command, args []string) error {
	// Get prompt
	prompt, err := getPrompt(args, runFile)
	if err != nil {
		return err
	}

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

	// TODO: Load config and create runner when service package is ready
	// cfg, err := loadConfig()
	// if err != nil {
	//     return err
	// }

	// runner, err := service.NewWorkflowRunner(cfg)
	// if err != nil {
	//     return fmt.Errorf("creating runner: %w", err)
	// }

	// opts := service.RunOptions{
	//     DryRun:     runDryRun,
	//     Yolo:       runYolo,
	//     MaxRetries: runMaxRetries,
	// }

	// if runResume {
	//     return runner.Resume(ctx, opts)
	// }

	// return runner.Run(ctx, prompt, opts)

	fmt.Printf("Would run workflow with prompt: %s\n", truncatePrompt(prompt, 50))
	fmt.Printf("Options: dry-run=%v, yolo=%v, resume=%v, max-retries=%d\n",
		runDryRun, runYolo, runResume, runMaxRetries)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
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

func truncatePrompt(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
