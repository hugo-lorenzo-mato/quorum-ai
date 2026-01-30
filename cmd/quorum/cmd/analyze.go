package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [prompt]",
	Short: "Run the analysis phase only",
	Long: `Execute the analysis phase of the workflow.

The analysis phase performs multi-agent analysis of the problem:
- V1: Multiple agents analyze the prompt independently
- V2: Agents critique each other's analyses (if consensus is low)
- V3: Reconciliation of conflicting analyses (if still divergent)

The result is stored in the workflow state for subsequent phases.
The prompt can be provided as an argument or via --file flag.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAnalyze,
}

var (
	analyzeFile       string
	analyzeDryRun     bool
	analyzeMaxRetries int
	analyzeOutput     string
)

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVarP(&analyzeFile, "file", "f", "", "Read prompt from file")
	analyzeCmd.Flags().BoolVar(&analyzeDryRun, "dry-run", false, "Simulate without executing")
	analyzeCmd.Flags().IntVar(&analyzeMaxRetries, "max-retries", 3, "Maximum retry attempts")
	analyzeCmd.Flags().StringVarP(&analyzeOutput, "output", "o", "", "Output mode (tui, plain, json, quiet)")

	// Single-agent mode flags
	analyzeCmd.Flags().BoolVar(&singleAgent, "single-agent", false,
		"Run in single-agent mode (faster execution, no multi-agent consensus)")
	analyzeCmd.Flags().StringVar(&agentName, "agent", "",
		"Agent to use for single-agent mode (e.g., 'claude', 'gemini', 'codex')")
	analyzeCmd.Flags().StringVar(&agentModel, "model", "",
		"Override the agent's default model (optional, requires --single-agent)")
}

func runAnalyze(_ *cobra.Command, args []string) error {
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
	if analyzeOutput != "" {
		detector.ForceMode(tui.ParseOutputMode(analyzeOutput))
	}
	outputMode := detector.Detect()
	useColor := detector.ShouldUseColor()

	// Create output handler
	output := tui.NewOutput(outputMode, useColor, false)
	defer func() { _ = output.Close() }()

	// Get prompt
	prompt, err := getPrompt(args, analyzeFile)
	if err != nil {
		return err
	}

	// Initialize phase runner dependencies
	deps, err := InitPhaseRunner(ctx, core.PhaseAnalyze, analyzeMaxRetries, analyzeDryRun, false)
	if err != nil {
		return err
	}

	phaseCtx, phaseCancel := context.WithTimeout(ctx, deps.PhaseTimeout)
	defer phaseCancel()
	ctx = phaseCtx

	deps.Logger.Info("starting analyze phase", "prompt_length", len(prompt))

	// Acquire lock
	if err := deps.StateAdapter.AcquireLock(ctx); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() { _ = deps.StateAdapter.ReleaseLock(ctx) }()

	// Build workflow config for state
	wfConfig := buildCoreWorkflowConfig(deps.RunnerConfig)

	// Initialize workflow state
	workflowState := InitializeWorkflowState(prompt, wfConfig)

	// Save initial state
	if err := deps.StateAdapter.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving initial state: %w", err)
	}

	// Create workflow context
	wctx := CreateWorkflowContext(deps, workflowState)

	// Create analyzer with moderator configuration
	analyzer, err := workflow.NewAnalyzer(deps.ModeratorConfig)
	if err != nil {
		return fmt.Errorf("creating analyzer: %w", err)
	}

	output.PhaseStarted(core.PhaseAnalyze)

	// Run analysis phase
	if err := analyzer.Run(ctx, wctx); err != nil {
		workflowState.Status = core.WorkflowStatusFailed
		workflowState.UpdatedAt = time.Now()
		_ = deps.StateAdapter.Save(ctx, workflowState)
		output.Log("error", fmt.Sprintf("analyze phase failed: %v", err))
		return err
	}

	// Update state
	workflowState.CurrentPhase = core.PhasePlan
	workflowState.UpdatedAt = time.Now()

	// Save final state
	if err := deps.StateAdapter.Save(ctx, workflowState); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	output.Log("info", "analyze phase completed")

	deps.Logger.Info("analyze phase completed",
		"workflow_id", workflowState.WorkflowID,
	)

	// Output summary in JSON mode
	if outputMode == tui.ModeJSON {
		analysis := workflow.GetConsolidatedAnalysis(workflowState)
		result := map[string]interface{}{
			"workflow_id": workflowState.WorkflowID,
			"phase":       "analyze",
			"status":      "completed",
			"analysis":    analysis,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Analysis phase completed. Workflow ID: %s\n", workflowState.WorkflowID)
	fmt.Println("Run 'quorum plan' to continue with the planning phase.")

	return nil
}
