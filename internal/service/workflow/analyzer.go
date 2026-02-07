package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// DocsConfigURL is the URL to the configuration documentation.
const DocsConfigURL = "https://github.com/hugo-lorenzo-mato/quorum-ai/blob/main/docs/CONFIGURATION.md"

// AnalysisOutput represents output from an analysis agent.
type AnalysisOutput struct {
	AgentName       string
	Model           string
	RawOutput       string
	Claims          []string
	Risks           []string
	Recommendations []string
	TokensIn        int
	TokensOut       int
	DurationMS      int64
}

// Analyzer runs the analysis phase with semantic moderator consensus.
// The moderator evaluates semantic agreement between agent analyses and
// iteratively refines until consensus is reached or max rounds exceeded.
type Analyzer struct {
	moderator *SemanticModerator
}

// NewAnalyzer creates a new analyzer with semantic moderator.
// Returns an error if moderator configuration is invalid.
func NewAnalyzer(moderatorConfig ModeratorConfig) (*Analyzer, error) {
	moderator, err := NewSemanticModerator(moderatorConfig)
	if err != nil {
		return nil, fmt.Errorf("creating semantic moderator: %w", err)
	}
	return &Analyzer{
		moderator: moderator,
	}, nil
}

// Run executes the complete analysis phase using either single-agent or multi-agent consensus.
func (a *Analyzer) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting analyze phase", "workflow_id", wctx.State.WorkflowID)

	// Check if analyze phase is already completed by looking at checkpoints.
	// This prevents re-running analysis when resuming a workflow that has
	// already completed the analyze phase but was interrupted before saving
	// the final state with current_phase=plan.
	if isPhaseCompleted(wctx.State, core.PhaseAnalyze) {
		wctx.Logger.Info("analyze phase already completed, skipping",
			"workflow_id", wctx.State.WorkflowID)
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", "Analyze phase already completed, skipping")
		}
		return nil
	}

	wctx.State.CurrentPhase = core.PhaseAnalyze
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseAnalyze)
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Check for single-agent mode first
	if wctx.Config != nil && wctx.Config.SingleAgent.Enabled {
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", fmt.Sprintf("Starting single-agent analysis with %s", wctx.Config.SingleAgent.Agent))
		}
		return a.runSingleAgentAnalysis(ctx, wctx)
	}

	// Multi-agent consensus mode - verify moderator is configured
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "Starting multi-agent analysis phase")
	}

	if a.moderator == nil || !a.moderator.IsEnabled() {
		return fmt.Errorf("semantic moderator is required but not configured. "+
			"Enable it in your config file under 'phases.analyze.moderator.enabled: true' with agent specified. "+
			"Alternatively, enable single-agent mode with 'phases.analyze.single_agent.enabled: true'. "+
			"See: %s#phases-settings", DocsConfigURL)
	}

	// CRITICAL: Report writing is required for moderator consensus evaluation.
	// The moderator reads analysis files from disk to evaluate consensus.
	// Without report writing, analyses are never persisted and consensus always fails (0%).
	if wctx.Report == nil || !wctx.Report.IsEnabled() {
		return fmt.Errorf("FATAL: moderator requires report writing to be enabled. "+
			"The moderator reads analysis files from disk to evaluate consensus. "+
			"Without report writing, analyses cannot be persisted and consensus will always be 0%%. "+
			"Set 'report.enabled: true' in your configuration. "+
			"See: %s#report-settings", DocsConfigURL)
	}

	effectiveThreshold := a.moderator.EffectiveThreshold(wctx.State.Prompt)
	taskType := detectTaskType(wctx.State.Prompt)
	wctx.Logger.Info("using semantic moderator for consensus evaluation",
		"threshold", effectiveThreshold,
		"task_type", taskType,
		"min_rounds", a.moderator.MinRounds(),
		"max_rounds", a.moderator.MaxRounds(),
	)
	if wctx.Output != nil {
		thresholdInfo := fmt.Sprintf("%.0f%%", effectiveThreshold*100)
		if taskType != "default" {
			thresholdInfo = fmt.Sprintf("%.0f%% (adaptive: %s)", effectiveThreshold*100, taskType)
		}
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Semantic moderator enabled (threshold: %s, min rounds: %d, max rounds: %d)",
			thresholdInfo, a.moderator.MinRounds(), a.moderator.MaxRounds()))
	}

	return a.runWithModerator(ctx, wctx)
}

// runSingleAgentAnalysis executes analysis with a single specified agent, bypassing multi-agent consensus.
// This mode is useful for simpler tasks or when consensus overhead is not needed.
func (a *Analyzer) runSingleAgentAnalysis(ctx context.Context, wctx *Context) error {
	agentName := wctx.Config.SingleAgent.Agent
	if agentName == "" {
		return fmt.Errorf("single_agent.agent must be specified when single_agent.enabled is true")
	}

	wctx.Logger.Info("running single-agent analysis",
		"agent", agentName,
		"model_override", wctx.Config.SingleAgent.Model,
	)

	// Get the agent from registry
	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return fmt.Errorf("getting agent %s: %w", agentName, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return fmt.Errorf("rate limit: %w", err)
	}

	// Resolve model - use override if provided, otherwise fall back to phase model
	model := wctx.Config.SingleAgent.Model
	if model == "" {
		model = ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")
	}

	// Get output file path for LLM to write directly
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.SingleAgentAnalysisPath(agentName, model)
	}

	// Render analysis prompt
	prompt, err := wctx.Prompts.RenderAnalyzeV1(AnalyzeV1Params{
		Prompt:         GetEffectivePrompt(wctx.State),
		Context:        BuildContextString(wctx.State),
		OutputFilePath: outputFilePath,
	})
	if err != nil {
		return fmt.Errorf("rendering prompt: %w", err)
	}

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Running single-agent analysis", map[string]interface{}{
			"phase":           "analyze_single",
			"model":           model,
			"mode":            "single_agent",
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:          prompt,
			Format:          core.OutputFormatText,
			Model:           model,
			Timeout:         wctx.Config.PhaseTimeouts.Analyze,
			Sandbox:         wctx.Config.Sandbox,
			Phase:           core.PhaseAnalyze,
			ReasoningEffort: wctx.Config.SingleAgent.ReasoningEffort,
			WorkDir:         wctx.ProjectRoot,
		})
		return execErr
	})

	durationMS := time.Since(startTime).Milliseconds()

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "analyze_single",
				"model":       model,
				"mode":        "single_agent",
				"duration_ms": durationMS,
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return fmt.Errorf("single-agent analysis failed: %w", err)
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, "Single-agent analysis completed", map[string]interface{}{
			"phase":       "analyze_single",
			"model":       result.Model,
			"mode":        "single_agent",
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": durationMS,
		})
	}

	wctx.Logger.Info("single-agent analysis completed",
		"agent", agentName,
		"model", result.Model,
		"tokens_in", result.TokensIn,
		"tokens_out", result.TokensOut,
		"duration_ms", durationMS,
	)

	// Update metrics
	wctx.UpdateMetrics(func(m *core.StateMetrics) {
		m.TotalTokensIn += result.TokensIn
		m.TotalTokensOut += result.TokensOut
		// No consensus score in single-agent mode
		m.ConsensusScore = 0
	})

	// Create consolidated_analysis checkpoint directly (bypasses synthesis)
	// The checkpoint format is compatible with downstream phases (Planner, Executor)
	// which only need the "content" field
	if err := wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     result.Output,
		"agent_count": 1,
		"synthesized": false,
		"mode":        "single_agent",
		"agent":       agentName,
		"model":       result.Model,
		"tokens_in":   result.TokensIn,
		"tokens_out":  result.TokensOut,
		"duration_ms": durationMS,
	}); err != nil {
		return fmt.Errorf("creating consolidated_analysis checkpoint: %w", err)
	}

	if wctx.Output != nil {
		wctx.Output.Log("success", "analyzer", fmt.Sprintf("Single-agent analysis completed with %s", agentName))
	}

	// Create phase complete checkpoint
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// runWithModerator executes the iterative V(n) analysis with semantic moderator consensus.
// This replaces the legacy V1/V2/V3 flow with a more flexible iterative approach.
//
// CRITICAL FLOW RULE: V(n+1) works ONLY on V(n), NEVER on previous versions.
//   - V1 → Initial independent analysis (NO moderator evaluation)
//   - V2 → Ultracritical review of ONLY V1
//   - Moderator evaluates AFTER V2
//   - V3 → Reviews ONLY V2 (if no consensus)
//   - V(n+1) → Reviews ONLY V(n)
func (a *Analyzer) runWithModerator(ctx context.Context, wctx *Context) error {
	resumed, err := a.resumeFromModeratorCheckpoint(ctx, wctx)
	if err != nil {
		return err
	}
	if resumed {
		return nil
	}

	currentOutputs, round, err := a.runInitialAnalyses(ctx, wctx)
	if err != nil {
		return err
	}

	finalOutputs, finalRound, err := a.runModeratorLoop(ctx, wctx, currentOutputs, round)
	if err != nil {
		return err
	}

	return a.finalizeModeratorAnalysis(ctx, wctx, finalOutputs, finalRound)
}

func (a *Analyzer) resumeFromModeratorCheckpoint(ctx context.Context, wctx *Context) (bool, error) {
	if savedRound, savedOutputs := getModeratorRoundCheckpoint(wctx.State); savedRound > 0 && len(savedOutputs) > 0 {
		wctx.Logger.Info("resuming from moderator round checkpoint",
			"saved_round", savedRound,
			"saved_agents", len(savedOutputs),
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", fmt.Sprintf("Resuming from round %d checkpoint (skipping V1-V%d)", savedRound, savedRound))
		}
		return true, a.continueFromCheckpoint(ctx, wctx, savedRound, savedOutputs)
	}
	return false, nil
}

func (a *Analyzer) runInitialAnalyses(ctx context.Context, wctx *Context) ([]AnalysisOutput, int, error) {
	wctx.Logger.Info("starting V1 analysis (initial, no moderator)")
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "V1: Running initial independent analysis")
	}

	v1Outputs, err := a.runV1Analysis(ctx, wctx)
	if err != nil {
		return nil, 0, fmt.Errorf("V1 analysis: %w", err)
	}

	wctx.Logger.Info("starting V2 refinement (ultracritical review of V1)")
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "V2: Running ultracritical review of V1 analyses")
	}

	v2Outputs, err := a.runVnRefinement(ctx, wctx, 2, v1Outputs, nil, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("V2 refinement: %w", err)
	}

	return v2Outputs, 2, nil
}

func (a *Analyzer) runModeratorLoop(ctx context.Context, wctx *Context, currentOutputs []AnalysisOutput, round int) ([]AnalysisOutput, int, error) {
	var previousScore float64
	for round <= a.moderator.MaxRounds() {
		evalResult, err := a.runModeratorRound(ctx, wctx, round, currentOutputs)
		if err != nil {
			return nil, round, err
		}

		if a.shouldStopForConsensus(wctx, round, evalResult) {
			return currentOutputs, round, nil
		}

		a.logLowConsensusWarning(wctx, round, evalResult)

		if a.shouldStopForStagnation(wctx, round, previousScore, evalResult) {
			return currentOutputs, round, nil
		}
		previousScore = evalResult.Score

		if a.shouldStopForMaxRounds(wctx, round, evalResult) {
			return currentOutputs, round, nil
		}

		round++
		refinedOutputs, err := a.runRefinementRound(ctx, wctx, round, currentOutputs, evalResult)
		if err != nil {
			return nil, round, err
		}
		currentOutputs = refinedOutputs
	}
	return currentOutputs, round, nil
}

func (a *Analyzer) runModeratorRound(ctx context.Context, wctx *Context, round int, currentOutputs []AnalysisOutput) (*ModeratorEvaluationResult, error) {
	wctx.Logger.Info("moderator evaluation starting",
		"round", round,
		"agents", len(currentOutputs),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Round %d: Running moderator evaluation", round))
	}

	evalResult, evalErr := a.runModeratorWithFallback(ctx, wctx, round, currentOutputs)
	if evalErr != nil {
		return nil, fmt.Errorf("moderator evaluation round %d: %w", round, evalErr)
	}

	wctx.UpdateMetrics(func(m *core.StateMetrics) {
		m.ConsensusScore = evalResult.Score
	})

	if cpErr := wctx.Checkpoint.CreateCheckpoint(wctx.State, string(service.CheckpointModeratorRound), map[string]interface{}{
		"round":           round,
		"consensus_score": evalResult.Score,
		"outputs":         serializeAnalysisOutputs(currentOutputs),
		"raw_output":      evalResult.RawOutput,
	}); cpErr != nil {
		wctx.Logger.Warn("failed to create moderator round checkpoint", "round", round, "error", cpErr)
	}

	effectiveThreshold := a.moderator.EffectiveThreshold(wctx.State.Prompt)
	wctx.Logger.Info("moderator evaluation complete",
		"round", round,
		"score", evalResult.Score,
		"threshold", effectiveThreshold,
		"agreements", len(evalResult.Agreements),
		"divergences", len(evalResult.Divergences),
	)

	if wctx.Output != nil {
		statusIcon := "⚠"
		level := "warn"
		if evalResult.Score >= effectiveThreshold {
			statusIcon = "✓"
			level = "success"
		}
		wctx.Output.Log(level, "analyzer", fmt.Sprintf("%s Round %d: Semantic consensus %.0f%% (threshold: %.0f%%)",
			statusIcon, round, evalResult.Score*100, effectiveThreshold*100))
	}

	return evalResult, nil
}

func (a *Analyzer) shouldStopForConsensus(wctx *Context, round int, evalResult *ModeratorEvaluationResult) bool {
	effectiveThreshold := a.moderator.EffectiveThreshold(wctx.State.Prompt)
	if evalResult.Score < effectiveThreshold {
		return false
	}
	if round >= a.moderator.MinRounds() {
		wctx.Logger.Info("consensus threshold met",
			"score", evalResult.Score,
			"threshold", effectiveThreshold,
			"round", round,
		)
		if wctx.Output != nil {
			wctx.Output.Log("success", "analyzer", fmt.Sprintf("Consensus achieved at %.0f%% after %d round(s)", evalResult.Score*100, round))
		}
		return true
	}

	wctx.Logger.Info("consensus threshold met but minimum rounds not reached",
		"score", evalResult.Score,
		"threshold", effectiveThreshold,
		"round", round,
		"min_rounds", a.moderator.MinRounds(),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Threshold met (%.0f%%) but continuing to min rounds (%d/%d)",
			evalResult.Score*100, round, a.moderator.MinRounds()))
	}
	return false
}

func (a *Analyzer) logLowConsensusWarning(wctx *Context, round int, evalResult *ModeratorEvaluationResult) {
	if evalResult.Score >= a.moderator.WarningThreshold() {
		return
	}
	wctx.Logger.Warn("consensus score is low, continuing refinement",
		"score", evalResult.Score,
		"warning_threshold", a.moderator.WarningThreshold(),
		"round", round,
		"max_rounds", a.moderator.MaxRounds(),
	)
	if wctx.Output != nil {
		wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Low consensus %.0f%% (below %.0f%%), continuing refinement to improve agreement",
			evalResult.Score*100, a.moderator.WarningThreshold()*100))
	}
}

func (a *Analyzer) shouldStopForStagnation(wctx *Context, round int, previousScore float64, evalResult *ModeratorEvaluationResult) bool {
	if round <= 2 {
		return false
	}

	// CRITICAL: Consecutive zero scores indicate a parsing or configuration failure,
	// not actual stagnation. The moderator likely can't read analysis files or
	// is failing to parse its own output. Don't treat this as convergence.
	if evalResult.Score == 0 && previousScore == 0 {
		wctx.Logger.Warn("consecutive zero scores detected - likely parsing or configuration failure, continuing refinement",
			"round", round,
			"note", "This usually means analysis files are not being written or moderator output parsing failed",
		)
		if wctx.Output != nil {
			wctx.Output.Log("warn", "analyzer", "Consecutive 0% scores detected - possible configuration issue. Continuing refinement...")
		}
		return false // Continue trying, don't consolidate with invalid data
	}

	improvement := evalResult.Score - previousScore
	if improvement >= a.moderator.StagnationThreshold() {
		return false
	}
	wctx.Logger.Warn("consensus stagnating, exiting refinement loop",
		"improvement", improvement,
		"stagnation_threshold", a.moderator.StagnationThreshold(),
	)
	if wctx.Output != nil {
		wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Consensus stagnating (improvement %.1f%% < %.1f%%), proceeding with current score",
			improvement*100, a.moderator.StagnationThreshold()*100))
	}
	return true
}

func (a *Analyzer) shouldStopForMaxRounds(wctx *Context, round int, evalResult *ModeratorEvaluationResult) bool {
	if round < a.moderator.MaxRounds() {
		return false
	}
	wctx.Logger.Warn("max rounds reached without consensus",
		"round", round,
		"max_rounds", a.moderator.MaxRounds(),
		"final_score", evalResult.Score,
	)
	if wctx.Output != nil {
		wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Max rounds (%d) reached. Final consensus: %.0f%%",
			a.moderator.MaxRounds(), evalResult.Score*100))
	}
	return true
}

func (a *Analyzer) runRefinementRound(ctx context.Context, wctx *Context, round int, currentOutputs []AnalysisOutput, evalResult *ModeratorEvaluationResult) ([]AnalysisOutput, error) {
	wctx.Logger.Info("starting refinement round",
		"round", round,
		"previous_round", round-1,
		"agreements_to_preserve", len(evalResult.Agreements),
		"divergences_to_resolve", len(evalResult.Divergences),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Round %d: Refining V%d analyses (preserving %d agreements, resolving %d divergences)",
			round, round-1, len(evalResult.Agreements), len(evalResult.Divergences)))
	}

	refinedOutputs, err := a.runVnRefinement(ctx, wctx, round, currentOutputs, evalResult, evalResult.Agreements)
	if err != nil {
		return nil, fmt.Errorf("V%d refinement: %w", round, err)
	}
	return refinedOutputs, nil
}

func (a *Analyzer) finalizeModeratorAnalysis(ctx context.Context, wctx *Context, outputs []AnalysisOutput, round int) error {
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Consolidating final analysis from V%d outputs", round))
	}
	if err := a.consolidateAnalysis(ctx, wctx, outputs); err != nil {
		return fmt.Errorf("consolidating analysis: %w", err)
	}
	if wctx.Output != nil {
		wctx.Output.Log("success", "analyzer", "Analysis phase completed successfully")
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}
	return nil
}

// runVnRefinement runs a V(n) refinement round with all agents.
func (a *Analyzer) runVnRefinement(ctx context.Context, wctx *Context, round int, previousOutputs []AnalysisOutput, evalResult *ModeratorEvaluationResult, agreements []string) ([]AnalysisOutput, error) {
	// Use AvailableForPhase to only get agents enabled for analyze phase
	// If project has specific phase config, use that; otherwise fallback to global config
	var agentNames []string
	if len(wctx.Config.ProjectAgentPhases) > 0 {
		agentNames = wctx.Agents.AvailableForPhaseWithConfig(ctx, "analyze", wctx.Config.ProjectAgentPhases)
	} else {
		agentNames = wctx.Agents.AvailableForPhase(ctx, "analyze")
	}
	if len(agentNames) == 0 {
		return nil, core.ErrValidation(core.CodeNoAgents, "no agents available for analyze phase")
	}

	// Build map of previous outputs by agent
	previousByAgent := make(map[string]AnalysisOutput)
	for _, out := range previousOutputs {
		// Extract agent name from output name (e.g., "v1-claude" -> "claude")
		agentName := out.AgentName
		if strings.HasPrefix(agentName, "v") && strings.Contains(agentName, "-") {
			parts := strings.SplitN(agentName, "-", 2)
			if len(parts) == 2 {
				agentName = parts[1]
			}
		}
		previousByAgent[agentName] = out
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	outputs := make([]AnalysisOutput, 0, len(agentNames))
	errors := make(map[string]error)

	for _, name := range agentNames {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Get this agent's previous analysis
			prevOutput, hasPrevious := previousByAgent[name]
			if !hasPrevious {
				// If this agent didn't participate before, treat as new V1
				output, err := a.runAnalysisWithAgent(ctx, wctx, name)
				if err != nil {
					mu.Lock()
					errors[name] = err
					mu.Unlock()
					return
				}
				output.AgentName = fmt.Sprintf("v%d-%s", round, name)
				mu.Lock()
				outputs = append(outputs, output)
				mu.Unlock()
				return
			}

			// Run refinement
			output, err := a.runVnRefinementWithAgent(ctx, wctx, name, round, prevOutput, evalResult, agreements)
			mu.Lock()
			if err != nil {
				errors[name] = err
			} else {
				outputs = append(outputs, output)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Need at least 2 successful outputs
	const minRequired = 2
	if len(outputs) < minRequired {
		var errMsgs []string
		for agent, err := range errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", agent, err))
		}
		return nil, fmt.Errorf("insufficient agents succeeded in V%d (%d/%d required): %s",
			round, len(outputs), minRequired, strings.Join(errMsgs, "; "))
	}

	return outputs, nil
}

// runVnRefinementWithAgent runs V(n) refinement with a specific agent.
func (a *Analyzer) runVnRefinementWithAgent(ctx context.Context, wctx *Context, agentName string, round int, prevOutput AnalysisOutput, evalResult *ModeratorEvaluationResult, agreements []string) (AnalysisOutput, error) {
	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("getting agent %s: %w", agentName, err)
	}

	// Resolve model FIRST (needed for cache lookup)
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")

	// Get output file path FIRST (needed for cache lookup)
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.VnAnalysisPath(agentName, model, round)
	}

	// Compute prompt hash for cache validation
	promptHash := computePromptHash(wctx.State)

	// === CACHE CHECK: BEFORE rate limiter ===

	// 1. Check checkpoint (with metrics restoration)
	if meta := getAnalysisCheckpoint(wctx.State, agentName, round, promptHash); meta != nil {
		if output, err := restoreAnalysisFromCheckpoint(meta); err == nil {
			wctx.Logger.Info("restored Vn analysis from checkpoint",
				"agent", agentName,
				"round", round,
				"tokens_in", output.TokensIn,
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", fmt.Sprintf("Restored V%d analysis for %s from checkpoint (cached)", round, agentName))
			}
			return *output, nil
		}
		// Checkpoint exists but file invalid - continue to re-execute
		wctx.Logger.Debug("checkpoint found but file invalid, re-executing", "agent", agentName, "round", round)
	}

	// 2. Backward compatibility: file exists but no checkpoint
	if outputFilePath != "" {
		if output, err := loadExistingAnalysis(outputFilePath, agentName, model); err == nil {
			wctx.Logger.Info("using existing Vn analysis (no checkpoint, metrics unavailable)",
				"agent", agentName,
				"round", round,
				"path", outputFilePath,
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", fmt.Sprintf("Using existing V%d analysis for %s (legacy cache)", round, agentName))
			}
			return *output, nil
		}
	}

	// === NO CACHE: Acquire rate limit and execute ===

	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return AnalysisOutput{}, fmt.Errorf("rate limit: %w", err)
	}

	// Ensure output directory exists before execution (file enforcement)
	if outputFilePath != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		if err := enforcement.EnsureDirectory(outputFilePath); err != nil {
			wctx.Logger.Warn("failed to ensure output directory", "path", outputFilePath, "error", err)
		}
	}

	// Build divergence info for this agent (evalResult may be nil for V2 first refinement)
	divergences := make([]VnDivergenceInfo, 0)
	var consensusScore float64
	var missingPerspectives []string
	if evalResult != nil {
		for _, div := range evalResult.Divergences {
			divergences = append(divergences, VnDivergenceInfo{
				Category:       div.Description,
				YourPosition:   "See your previous analysis",
				OtherPositions: "See moderator evaluation",
				Guidance:       "Refine based on evidence",
			})
		}
		consensusScore = evalResult.Score * 100
		missingPerspectives = evalResult.MissingPerspectives
	}

	// Build summary of previous analysis to avoid context overflow
	// Single agent refinement can use more context than multi-agent moderator
	const maxPrevAnalysisChars = 100000 // Generous limit
	previousAnalysis := buildAnalysisSummary(prevOutput, maxPrevAnalysisChars)
	if len(previousAnalysis) != len(prevOutput.RawOutput) {
		wctx.Logger.Debug("using summary of previous analysis for Vn refinement",
			"agent", agentName,
			"round", round,
			"original_len", len(prevOutput.RawOutput),
			"summary_len", len(previousAnalysis),
		)
	}

	prompt, err := wctx.Prompts.RenderVnRefine(VnRefineParams{
		Prompt:               GetEffectivePrompt(wctx.State),
		Context:              BuildContextString(wctx.State),
		Round:                round,
		PreviousRound:        round - 1,
		PreviousAnalysis:     previousAnalysis,
		HasArbiterEvaluation: evalResult != nil,
		ConsensusScore:       consensusScore,
		Threshold:            a.moderator.EffectiveThreshold(wctx.State.Prompt) * 100,
		Agreements:           agreements,
		Divergences:          divergences,
		MissingPerspectives:  missingPerspectives,
		OutputFilePath:       outputFilePath,
	})
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("rendering V%d prompt: %w", round, err)
	}
	startTime := time.Now()

	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, fmt.Sprintf("Running V%d analysis refinement", round), map[string]interface{}{
			"phase":           fmt.Sprintf("analyze_v%d", round),
			"model":           model,
			"round":           round,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       fmt.Sprintf("analyze_v%d", round),
				"model":       model,
				"round":       round,
				"duration_ms": time.Since(startTime).Milliseconds(),
			})
		}
		return AnalysisOutput{}, err
	}

	durationMS := time.Since(startTime).Milliseconds()

	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, fmt.Sprintf("V%d analysis refinement completed", round), map[string]interface{}{
			"phase":       fmt.Sprintf("analyze_v%d", round),
			"model":       result.Model,
			"round":       round,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": durationMS,
		})
	}

	// Ensure output file exists (file enforcement fallback)
	if outputFilePath != "" && result != nil && result.Output != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		createdByLLM, verifyErr := enforcement.VerifyOrWriteFallback(outputFilePath, result.Output)
		if verifyErr != nil {
			wctx.Logger.Warn("file enforcement failed", "path", outputFilePath, "error", verifyErr)
		} else if !createdByLLM {
			wctx.Logger.Debug("created fallback file from stdout", "path", outputFilePath)
		}
	}

	outputName := fmt.Sprintf("v%d-%s", round, agentName)
	output := parseAnalysisOutputWithMetrics(outputName, model, result, durationMS)

	// Create checkpoint for future resume with full metrics
	if outputFilePath != "" {
		if cpErr := createAnalysisCheckpoint(wctx, agentName, model, round, outputFilePath, output, promptHash); cpErr != nil {
			wctx.Logger.Warn("failed to create Vn analysis checkpoint", "agent", agentName, "round", round, "error", cpErr)
		}
	}

	return output, nil
}

// runV1Analysis runs initial analysis with multiple agents in parallel.
// It tolerates partial failures - continues as long as at least 2 agents succeed.
func (a *Analyzer) runV1Analysis(ctx context.Context, wctx *Context) ([]AnalysisOutput, error) {
	// Use AvailableForPhase to only get agents enabled for analyze phase
	// If project has specific phase config, use that; otherwise fallback to global config
	var agentNames []string
	if len(wctx.Config.ProjectAgentPhases) > 0 {
		agentNames = wctx.Agents.AvailableForPhaseWithConfig(ctx, "analyze", wctx.Config.ProjectAgentPhases)
	} else {
		agentNames = wctx.Agents.AvailableForPhase(ctx, "analyze")
	}
	if len(agentNames) == 0 {
		return nil, core.ErrValidation(core.CodeNoAgents, "no agents available for analyze phase")
	}

	wctx.Logger.Info("running V1 analysis",
		"agents", strings.Join(agentNames, ", "),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("V1 Analysis: querying %d agents (%s)", len(agentNames), strings.Join(agentNames, ", ")))
	}

	// Use sync.WaitGroup instead of errgroup to avoid cancelling all goroutines on first error
	var wg sync.WaitGroup
	var mu sync.Mutex
	outputs := make([]AnalysisOutput, 0, len(agentNames))
	errors := make(map[string]error)

	for _, name := range agentNames {
		name := name // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := a.runAnalysisWithAgent(ctx, wctx, name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				wctx.Logger.Error("agent analysis failed",
					"agent", name,
					"error", err,
				)
				errors[name] = err
			} else {
				outputs = append(outputs, output)
			}
		}()
	}

	wg.Wait()

	// Log summary
	wctx.Logger.Info("V1 analysis complete",
		"succeeded", len(outputs),
		"failed", len(errors),
		"total", len(agentNames),
	)

	// Need at least 2 successful outputs for meaningful consensus
	// Without at least 2 agents, there's no cross-validation benefit
	const minRequired = 2

	if len(outputs) < minRequired {
		// Collect error messages
		var errMsgs []string
		for agent, err := range errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", agent, err))
		}
		return nil, fmt.Errorf("insufficient agents succeeded (%d/%d required): %s",
			len(outputs), minRequired, strings.Join(errMsgs, "; "))
	}

	// Log which agents failed but we're continuing
	if len(errors) > 0 {
		var failedAgents []string
		for agent := range errors {
			failedAgents = append(failedAgents, agent)
		}
		if wctx.Output != nil {
			wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Continuing with %d/%d agents (failed: %s)",
				len(outputs), len(agentNames), strings.Join(failedAgents, ", ")))
		}
	}

	return outputs, nil
}

// runAnalysisWithAgent runs analysis with a specific agent.
func (a *Analyzer) runAnalysisWithAgent(ctx context.Context, wctx *Context, agentName string) (AnalysisOutput, error) {
	agent, err := wctx.Agents.Get(agentName)
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("getting agent %s: %w", agentName, err)
	}

	// Resolve model FIRST (needed for cache lookup)
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")

	// Get output file path FIRST (needed for cache lookup)
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.V1AnalysisPath(agentName, model)
	}

	// Compute prompt hash for cache validation
	promptHash := computePromptHash(wctx.State)

	// === CACHE CHECK: BEFORE rate limiter ===

	// 1. Check checkpoint (with metrics restoration)
	if meta := getAnalysisCheckpoint(wctx.State, agentName, 1, promptHash); meta != nil {
		if output, err := restoreAnalysisFromCheckpoint(meta); err == nil {
			wctx.Logger.Info("restored V1 analysis from checkpoint",
				"agent", agentName,
				"tokens_in", output.TokensIn,
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", fmt.Sprintf("Restored V1 analysis for %s from checkpoint (cached)", agentName))
			}
			return *output, nil
		}
		// Checkpoint exists but file invalid - continue to re-execute
		wctx.Logger.Debug("checkpoint found but file invalid, re-executing", "agent", agentName)
	}

	// 2. Backward compatibility: file exists but no checkpoint
	if outputFilePath != "" {
		if output, err := loadExistingAnalysis(outputFilePath, agentName, model); err == nil {
			wctx.Logger.Info("using existing V1 analysis (no checkpoint, metrics unavailable)",
				"agent", agentName,
				"path", outputFilePath,
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", fmt.Sprintf("Using existing V1 analysis for %s (legacy cache)", agentName))
			}
			return *output, nil
		}
	}

	// === NO CACHE: Acquire rate limit and execute ===

	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return AnalysisOutput{}, fmt.Errorf("rate limit: %w", err)
	}

	// Ensure output directory exists before execution (file enforcement)
	if outputFilePath != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		if err := enforcement.EnsureDirectory(outputFilePath); err != nil {
			wctx.Logger.Warn("failed to ensure output directory", "path", outputFilePath, "error", err)
		}
	}

	// Render prompt (use optimized prompt if available)
	prompt, err := wctx.Prompts.RenderAnalyzeV1(AnalyzeV1Params{
		Prompt:         GetEffectivePrompt(wctx.State),
		Context:        BuildContextString(wctx.State),
		OutputFilePath: outputFilePath,
	})
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("rendering prompt: %w", err)
	}

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Running V1 analysis", map[string]interface{}{
			"phase":           "analyze_v1",
			"model":           model,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", agentName, err.Error(), map[string]interface{}{
				"phase":       "analyze_v1",
				"model":       model,
				"duration_ms": time.Since(startTime).Milliseconds(),
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return AnalysisOutput{}, err
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", agentName, "V1 analysis completed", map[string]interface{}{
			"phase":       "analyze_v1",
			"model":       result.Model,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"duration_ms": time.Since(startTime).Milliseconds(),
		})
	}

	// Calculate duration
	durationMS := time.Since(startTime).Milliseconds()

	// Ensure output file exists (file enforcement fallback)
	// If LLM didn't write to the file, write stdout as fallback
	if outputFilePath != "" && result != nil && result.Output != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		createdByLLM, verifyErr := enforcement.VerifyOrWriteFallback(outputFilePath, result.Output)
		if verifyErr != nil {
			wctx.Logger.Warn("file enforcement failed", "path", outputFilePath, "error", verifyErr)
		} else if !createdByLLM {
			wctx.Logger.Debug("created fallback file from stdout", "path", outputFilePath)
		}
	}

	// Parse output with metrics
	// Use v1-{agent} naming convention for V1 outputs
	outputName := fmt.Sprintf("v1-%s", agentName)
	output := parseAnalysisOutputWithMetrics(outputName, model, result, durationMS)

	// Create checkpoint for future resume with full metrics
	if outputFilePath != "" {
		if cpErr := createAnalysisCheckpoint(wctx, agentName, model, 1, outputFilePath, output, promptHash); cpErr != nil {
			wctx.Logger.Warn("failed to create analysis checkpoint", "agent", agentName, "error", cpErr)
		}
	}

	return output, nil
}

// consolidateAnalysis uses an LLM to synthesize all analysis outputs into a unified report.
func (a *Analyzer) consolidateAnalysis(ctx context.Context, wctx *Context, outputs []AnalysisOutput) error {
	// Get synthesizer agent
	synthesizerAgent := wctx.Config.SynthesizerAgent
	if synthesizerAgent == "" {
		// No fallback - synthesizer must be explicitly configured for multi-agent workflows
		return fmt.Errorf("phases.analyze.synthesizer.agent is not configured. " +
			"Multi-agent analysis requires a synthesizer to combine results. " +
			"Please set 'phases.analyze.synthesizer.agent' in your .quorum/config.yaml file")
	}

	agent, err := wctx.Agents.Get(synthesizerAgent)
	if err != nil {
		wctx.Logger.Warn("synthesizer agent not available, using concatenation fallback",
			"agent", synthesizerAgent,
			"error", err,
		)
		return a.synthesizeAnalysisFallback(wctx, outputs)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(synthesizerAgent)
	if err := limiter.Acquire(); err != nil {
		wctx.Logger.Warn("rate limit exceeded for synthesizer, using fallback",
			"agent", synthesizerAgent,
		)
		return a.synthesizeAnalysisFallback(wctx, outputs)
	}

	// Resolve model from agent's phase_models.analyze or default model
	model := ResolvePhaseModel(wctx.Config, synthesizerAgent, core.PhaseAnalyze, "")

	// Get output file path for LLM to write directly
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.ConsolidatedAnalysisPath()
	}

	// Build summaries of analyses to avoid context overflow in synthesis
	// With 4 agents, ~80k chars each is generous while leaving room for prompt and output
	const maxCharsPerAnalysisSynthesize = 80000
	summarizedOutputs := make([]AnalysisOutput, len(outputs))
	for i, out := range outputs {
		summarizedOutputs[i] = out
		summary := buildAnalysisSummary(out, maxCharsPerAnalysisSynthesize)
		if len(summary) != len(out.RawOutput) {
			summarizedOutputs[i].RawOutput = summary
			wctx.Logger.Debug("using summary for synthesis",
				"agent", out.AgentName,
				"original_len", len(out.RawOutput),
				"summary_len", len(summary),
			)
		}
	}

	// Render synthesis prompt
	prompt, err := wctx.Prompts.RenderSynthesizeAnalysis(SynthesizeAnalysisParams{
		Prompt:         GetEffectivePrompt(wctx.State),
		Analyses:       summarizedOutputs,
		OutputFilePath: outputFilePath,
	})
	if err != nil {
		wctx.Logger.Warn("failed to render synthesis prompt, using fallback",
			"error", err,
		)
		return a.synthesizeAnalysisFallback(wctx, outputs)
	}

	wctx.Logger.Info("synthesis start",
		"agent", synthesizerAgent,
		"model", model,
		"analyses_count", len(outputs),
	)

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", synthesizerAgent, "Synthesizing analyses", map[string]interface{}{
			"phase":           "synthesize",
			"model":           model,
			"analyses_count":  len(outputs),
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
			WorkDir: wctx.ProjectRoot,
		})
		return execErr
	})

	if err != nil {
		if wctx.Output != nil {
			wctx.Output.AgentEvent("error", synthesizerAgent, err.Error(), map[string]interface{}{
				"phase":          "synthesize",
				"model":          model,
				"analyses_count": len(outputs),
				"duration_ms":    time.Since(startTime).Milliseconds(),
				"error_type":     fmt.Sprintf("%T", err),
				"fallback":       true,
			})
		}
		wctx.Logger.Warn("synthesis LLM call failed, using fallback",
			"error", err,
		)
		return a.synthesizeAnalysisFallback(wctx, outputs)
	}

	durationMS := time.Since(startTime).Milliseconds()

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", synthesizerAgent, "Synthesis completed", map[string]interface{}{
			"phase":          "synthesize",
			"model":          result.Model,
			"analyses_count": len(outputs),
			"tokens_in":      result.TokensIn,
			"tokens_out":     result.TokensOut,
			"duration_ms":    durationMS,
		})
	}

	wctx.Logger.Info("synthesis done",
		"agent", synthesizerAgent,
		"model", model,
	)

	// Store the LLM-synthesized analysis as checkpoint
	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     result.Output,
		"agent_count": len(outputs),
		"synthesized": true,
		"agent":       synthesizerAgent,
		"model":       model,
		"tokens_in":   result.TokensIn,
		"tokens_out":  result.TokensOut,
	})
}

// synthesizeAnalysisFallback concatenates analyses when LLM synthesis fails.
func (a *Analyzer) synthesizeAnalysisFallback(wctx *Context, outputs []AnalysisOutput) error {
	var consolidated strings.Builder

	for _, output := range outputs {
		consolidated.WriteString(fmt.Sprintf("## Analysis from %s\n", output.AgentName))
		consolidated.WriteString(output.RawOutput)
		consolidated.WriteString("\n\n")
	}

	content := consolidated.String()

	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     content,
		"agent_count": len(outputs),
		"synthesized": false,
	})
}

// parseAnalysisOutput parses agent output into AnalysisOutput (legacy, no metrics).
func parseAnalysisOutput(agentName string, result *core.ExecuteResult) AnalysisOutput {
	return parseAnalysisOutputWithMetrics(agentName, "", result, 0)
}

// parseAnalysisOutputWithMetrics parses agent output into AnalysisOutput with full metrics.
// Supports both JSON and Markdown formats for flexibility across different CLI agents.
func parseAnalysisOutputWithMetrics(agentName, model string, result *core.ExecuteResult, durationMS int64) AnalysisOutput {
	output := AnalysisOutput{
		AgentName:  agentName,
		Model:      model,
		RawOutput:  result.Output,
		TokensIn:   result.TokensIn,
		TokensOut:  result.TokensOut,
		DurationMS: durationMS,
	}

	// Try JSON first (for backwards compatibility)
	var parsed struct {
		Claims          []string `json:"claims"`
		Risks           []string `json:"risks"`
		Recommendations []string `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(result.Output), &parsed); err == nil {
		output.Claims = parsed.Claims
		output.Risks = parsed.Risks
		output.Recommendations = parsed.Recommendations
		return output
	}

	// Fall back to Markdown extraction
	output.Claims = extractMarkdownSection(result.Output, "claims")
	output.Risks = extractMarkdownSection(result.Output, "risks")
	output.Recommendations = extractMarkdownSection(result.Output, "recommendations")

	return output
}

// extractMarkdownSection extracts items from a Markdown section.
// Looks for sections like "## Claims", "### Claims", "**Claims**", or "Claims:" and extracts:
// - Bullet points (-, *, •)
// - Numbered lists (1., 1))
// - Table rows (| content |)
// - Bold items at start of lines (**item**: description)
// - Subsection headers (### Subsection)
func extractMarkdownSection(text, sectionName string) []string {
	// Pattern to find section header (case-insensitive)
	// Matches: ## Claims, ### Claims, **Claims**, Claims:, # Claims
	headerPattern := regexp.MustCompile(`(?im)^(?:#{1,4}\s*` + sectionName + `|[\*_]{2}` + sectionName + `[\*_]{2}|` + sectionName + `\s*:)\s*$`)

	loc := headerPattern.FindStringIndex(text)
	if loc == nil {
		return nil
	}

	// Get text after the header
	afterHeader := text[loc[1]:]

	// Find the next section header to limit our search
	// Match any markdown header (# to ####) that starts a major section
	// This includes: ## Risks, ### Recommendations, ## Summary, etc.
	// Excludes subsections within our section (those are extracted as items)
	nextSectionPattern := regexp.MustCompile(`(?m)^#{1,4}\s+(?:Claims|Risks|Recommendations|Summary|Agreements|Disagreements|Missing|Enhanced|Divergences|Conclusions?)(?:\s|$)`)
	nextLoc := nextSectionPattern.FindStringIndex(afterHeader)

	sectionText := afterHeader
	if nextLoc != nil {
		sectionText = afterHeader[:nextLoc[0]]
	}

	var items []string

	// 1. Extract bullet points (-, *, •, or numbered lists)
	bulletPattern := regexp.MustCompile(`(?m)^\s*[-*•]\s*(.+)$|^\s*\d+[.)]\s*(.+)$`)
	bulletMatches := bulletPattern.FindAllStringSubmatch(sectionText, -1)
	for _, match := range bulletMatches {
		item := match[1]
		if item == "" {
			item = match[2]
		}
		item = strings.TrimSpace(item)
		if item != "" && !strings.HasPrefix(item, "|") { // Skip table-related bullets
			items = append(items, cleanMarkdownItem(item))
		}
	}

	// 2. Extract items from tables (| Content | ... |)
	tablePattern := regexp.MustCompile(`(?m)^\|([^|]+)\|`)
	tableMatches := tablePattern.FindAllStringSubmatch(sectionText, -1)
	for _, match := range tableMatches {
		cell := strings.TrimSpace(match[1])
		// Skip header rows and separator rows
		if cell != "" && !strings.HasPrefix(cell, "-") && !strings.HasPrefix(cell, "---") &&
			cell != "Claim" && cell != "Risk" && cell != "Recommendation" &&
			cell != "Agent" && cell != "Agent(s)" && cell != "Assessment" &&
			cell != "Severity" && cell != "Notes" && cell != "Raised By" {
			items = append(items, cleanMarkdownItem(cell))
		}
	}

	// 3. If no items found, try extracting bold items at line start (**Item**: description)
	if len(items) == 0 {
		boldPattern := regexp.MustCompile(`(?m)^\s*\*\*([^*]+)\*\*:?\s*(.*)$`)
		boldMatches := boldPattern.FindAllStringSubmatch(sectionText, -1)
		for _, match := range boldMatches {
			title := strings.TrimSpace(match[1])
			desc := strings.TrimSpace(match[2])
			if title != "" {
				if desc != "" {
					items = append(items, title+": "+desc)
				} else {
					items = append(items, title)
				}
			}
		}
	}

	// 4. If still no items, try extracting subsection headers (### Subsection)
	if len(items) == 0 {
		subHeaderPattern := regexp.MustCompile(`(?m)^###\s+(.+)$`)
		subMatches := subHeaderPattern.FindAllStringSubmatch(sectionText, -1)
		for _, match := range subMatches {
			item := strings.TrimSpace(match[1])
			if item != "" {
				items = append(items, cleanMarkdownItem(item))
			}
		}
	}

	return items
}

// cleanMarkdownItem removes common markdown formatting from an item.
func cleanMarkdownItem(s string) string {
	// Remove bold markers
	s = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(s, "$1")
	// Remove italic markers
	s = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(s, "$1")
	// Remove code ticks
	s = regexp.MustCompile("`([^`]+)`").ReplaceAllString(s, "$1")
	// Remove links but keep text
	s = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// GetConsolidatedAnalysis retrieves the consolidated analysis from state.
func GetConsolidatedAnalysis(state *core.WorkflowState) string {
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := state.Checkpoints[i]
		if cp.Type == "consolidated_analysis" && len(cp.Data) > 0 {
			var metadata map[string]interface{}
			if err := json.Unmarshal(cp.Data, &metadata); err == nil {
				if content, ok := metadata["content"].(string); ok {
					return content
				}
			}
		}
	}
	return ""
}

// isPhaseCompleted checks if a phase has already been completed by looking
// at the checkpoints. This is used to prevent re-running phases when resuming
// a workflow that was interrupted after phase completion but before saving
// the final state.
func isPhaseCompleted(state *core.WorkflowState, phase core.Phase) bool {
	// Look for a phase_complete checkpoint for the given phase.
	// We scan from the end to find the most recent checkpoint.
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := state.Checkpoints[i]
		if cp.Type == "phase_complete" && cp.Phase == phase {
			return true
		}
	}
	return false
}

// continueFromCheckpoint resumes analysis from a saved moderator round checkpoint.
// This is called when a previous run failed after completing some moderator rounds,
// allowing the analysis to continue without re-running V1-V(n) analyses.
func (a *Analyzer) continueFromCheckpoint(ctx context.Context, wctx *Context, savedRound int, savedOutputs []AnalysisOutput) error {
	// Start from the saved round - the checkpoint was saved AFTER that round's
	// moderator evaluation completed successfully, so we need to continue with
	// the next refinement round
	currentOutputs := savedOutputs
	round := savedRound

	// Get the previous score from checkpoint metadata for stagnation detection
	var previousScore float64
	for i := len(wctx.State.Checkpoints) - 1; i >= 0; i-- {
		cp := wctx.State.Checkpoints[i]
		if cp.Type == string(service.CheckpointModeratorRound) {
			var metadata map[string]interface{}
			if err := json.Unmarshal(cp.Data, &metadata); err == nil {
				if score, ok := metadata["consensus_score"].(float64); ok {
					previousScore = score
					break
				}
			}
		}
	}

	wctx.Logger.Info("continuing moderator loop from checkpoint",
		"round", round,
		"previous_score", previousScore,
		"agents", len(currentOutputs),
	)

	// Check if we've already reached max rounds (shouldn't normally happen)
	if round >= a.moderator.MaxRounds() {
		wctx.Logger.Info("resuming at max rounds, proceeding to consolidation")
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", "Resuming at max rounds, proceeding to consolidation")
		}
		goto consolidation
	}

	// Need to run the next V(n+1) refinement before the next moderator evaluation
	// because the checkpoint is saved AFTER successful moderator evaluation
	round++
	wctx.Logger.Info("starting refinement round after resume",
		"round", round,
		"previous_round", round-1,
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Round %d: Refining V%d analyses (post-resume)", round, round-1))
	}

	// Run V(n+1) refinement with no specific moderator feedback (since we don't have it)
	{
		refinedOutputs, err := a.runVnRefinement(ctx, wctx, round, currentOutputs, nil, nil)
		if err != nil {
			return fmt.Errorf("V%d refinement (post-resume): %w", round, err)
		}
		currentOutputs = refinedOutputs
	}

	// Continue the moderator evaluation loop
	for round <= a.moderator.MaxRounds() {
		wctx.Logger.Info("moderator evaluation starting (resumed)",
			"round", round,
			"agents", len(currentOutputs),
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", fmt.Sprintf("Round %d: Running moderator evaluation (resumed)", round))
		}

		// Run moderator evaluation with retry and fallback support
		evalResult, evalErr := a.runModeratorWithFallback(ctx, wctx, round, currentOutputs)
		if evalErr != nil {
			return fmt.Errorf("moderator evaluation round %d: %w", round, evalErr)
		}

		// Update consensus score in metrics
		wctx.UpdateMetrics(func(m *core.StateMetrics) {
			m.ConsensusScore = evalResult.Score
		})

		// Save checkpoint after successful moderator evaluation
		if cpErr := wctx.Checkpoint.CreateCheckpoint(wctx.State, string(service.CheckpointModeratorRound), map[string]interface{}{
			"round":           round,
			"consensus_score": evalResult.Score,
			"outputs":         serializeAnalysisOutputs(currentOutputs),
			"raw_output":      evalResult.RawOutput,
		}); cpErr != nil {
			wctx.Logger.Warn("failed to create moderator round checkpoint", "round", round, "error", cpErr)
		}

		effectiveThreshold := a.moderator.EffectiveThreshold(wctx.State.Prompt)
		wctx.Logger.Info("moderator evaluation complete",
			"round", round,
			"score", evalResult.Score,
			"threshold", effectiveThreshold,
		)

		if wctx.Output != nil {
			statusIcon := "⚠"
			level := "warn"
			if evalResult.Score >= effectiveThreshold {
				statusIcon = "✓"
				level = "success"
			}
			wctx.Output.Log(level, "analyzer", fmt.Sprintf("%s Round %d: Semantic consensus %.0f%% (threshold: %.0f%%)",
				statusIcon, round, evalResult.Score*100, effectiveThreshold*100))
		}

		// Check if consensus threshold is met AND minimum rounds completed
		if evalResult.Score >= effectiveThreshold && round >= a.moderator.MinRounds() {
			wctx.Logger.Info("consensus threshold met",
				"score", evalResult.Score,
				"threshold", effectiveThreshold,
				"round", round,
			)
			if wctx.Output != nil {
				wctx.Output.Log("success", "analyzer", fmt.Sprintf("Consensus achieved at %.0f%% after %d round(s)", evalResult.Score*100, round))
			}
			break
		}

		// Log warning if consensus is very low, but continue iterating
		// Low consensus indicates divergence - we should keep refining rather than abort
		if evalResult.Score < a.moderator.WarningThreshold() {
			wctx.Logger.Warn("consensus score is low (resumed), continuing refinement",
				"score", evalResult.Score,
				"warning_threshold", a.moderator.WarningThreshold(),
				"round", round,
				"max_rounds", a.moderator.MaxRounds(),
			)
			if wctx.Output != nil {
				wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Low consensus %.0f%% (below %.0f%%), continuing refinement to improve agreement",
					evalResult.Score*100, a.moderator.WarningThreshold()*100))
			}
			// Continue to next round instead of aborting
		}

		// Check for stagnation
		if round > savedRound+1 {
			improvement := evalResult.Score - previousScore
			if improvement < a.moderator.StagnationThreshold() {
				wctx.Logger.Warn("consensus stagnating, proceeding to consolidation")
				if wctx.Output != nil {
					wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Consensus stagnating (improvement %.1f%%), proceeding",
						improvement*100))
				}
				break
			}
		}
		previousScore = evalResult.Score

		// Check if we've reached max rounds
		if round >= a.moderator.MaxRounds() {
			wctx.Logger.Warn("max rounds reached without consensus", "final_score", evalResult.Score)
			break
		}

		// Run V(n+1) refinement
		round++
		refinedOutputs, err := a.runVnRefinement(ctx, wctx, round, currentOutputs, evalResult, evalResult.Agreements)
		if err != nil {
			return fmt.Errorf("V%d refinement: %w", round, err)
		}
		currentOutputs = refinedOutputs
	}

consolidation:
	// Final consolidation
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Consolidating final analysis from V%d outputs", round))
	}
	if err := a.consolidateAnalysis(ctx, wctx, currentOutputs); err != nil {
		return fmt.Errorf("consolidating analysis: %w", err)
	}
	if wctx.Output != nil {
		wctx.Output.Log("success", "analyzer", "Analysis phase completed successfully (resumed)")
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// isTransientModeratorError checks if an error is likely transient and worth retrying.
// This includes streaming errors, timeouts, and network issues.
func isTransientModeratorError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Stream completed without") ||
		strings.Contains(errStr, "stream") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "Timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "Connection") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "reset by peer") ||
		core.IsRetryable(err)
}

// runModeratorWithFallback runs the moderator evaluation with retry and fallback support.
// If the primary moderator agent fails, it tries fallback agents configured with moderate phase.
func (a *Analyzer) runModeratorWithFallback(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput) (*ModeratorEvaluationResult, error) {
	// Build list of fallback agents for moderator role
	fallbackAgents := a.buildModeratorFallbackChain(wctx)

	var lastErr error
	var triedAgents []string
	var lastAgent string
	var lastAttempt int
	globalAttempt := 0 // Global attempt counter for unique file naming

	for agentIdx, agentName := range fallbackAgents {
		isPrimary := agentIdx == 0
		lastAgent = agentName

		// Try up to 2 attempts with each agent
		for attempt := 1; attempt <= 2; attempt++ {
			globalAttempt++ // Increment global counter for each attempt
			lastAttempt = globalAttempt

			if !isPrimary || attempt > 1 {
				wctx.Logger.Info("trying moderator evaluation",
					"round", round,
					"agent", agentName,
					"is_fallback", !isPrimary,
					"attempt", attempt,
					"global_attempt", globalAttempt,
				)
				if wctx.Output != nil {
					if isPrimary {
						wctx.Output.Log("info", "analyzer",
							fmt.Sprintf("Round %d: Retrying moderator with %s (attempt %d)", round, agentName, attempt))
					} else {
						wctx.Output.Log("info", "analyzer",
							fmt.Sprintf("Round %d: Trying fallback moderator %s (attempt %d)", round, agentName, globalAttempt))
					}
				}
			}

			// Use EvaluateWithAgent to allow fallback to alternative agents
			// Pass globalAttempt for unique file naming per attempt
			evalResult, evalErr := a.moderator.EvaluateWithAgent(ctx, wctx, round, globalAttempt, outputs, agentName)
			if evalErr == nil {
				// Success - log if we used a fallback
				if !isPrimary {
					wctx.Logger.Info("fallback moderator succeeded",
						"round", round,
						"agent", agentName,
						"previous_failures", triedAgents,
					)
					if wctx.Output != nil {
						wctx.Output.Log("success", "analyzer",
							fmt.Sprintf("Round %d: Fallback moderator %s succeeded (after trying: %v)", round, agentName, triedAgents))
					}
				}
				return evalResult, nil
			}

			lastErr = evalErr

			// Log the error with detailed context
			wctx.Logger.Warn("moderator evaluation failed",
				"round", round,
				"agent", agentName,
				"attempt", attempt,
				"is_fallback", !isPrimary,
				"error", evalErr,
				"is_transient", isTransientModeratorError(evalErr),
				"is_validation", IsModeratorValidationError(evalErr),
			)

			// Decide whether to retry same agent or move to fallback
			if attempt < 2 && isTransientModeratorError(evalErr) {
				if wctx.Output != nil {
					wctx.Output.Log("warn", "analyzer",
						fmt.Sprintf("Round %d: %s failed (%v), retrying in 30s...", round, agentName, evalErr))
				}
				time.Sleep(30 * time.Second)
				continue
			}

			// If validation error or non-transient, move to next agent
			break
		}

		// Track that we tried this agent
		triedAgents = append(triedAgents, agentName)
	}

	// All agents failed - create detailed error checkpoint for debugging
	if wctx.Checkpoint != nil {
		_ = wctx.Checkpoint.ErrorCheckpointWithContext(wctx.State, lastErr, service.ErrorCheckpointDetails{
			Agent:             lastAgent,
			Round:             round,
			Attempt:           lastAttempt,
			IsTransient:       isTransientModeratorError(lastErr),
			IsValidationError: IsModeratorValidationError(lastErr),
			FallbacksTried:    triedAgents,
			Extra: map[string]string{
				"total_agents":  fmt.Sprintf("%d", len(fallbackAgents)),
				"outputs_count": fmt.Sprintf("%d", len(outputs)),
			},
		})
	}

	// All agents failed
	return nil, fmt.Errorf("all moderator agents failed (tried %d agents: %v, last error: %w)", len(fallbackAgents), triedAgents, lastErr)
}

// buildModeratorFallbackChain builds a list of agents to try for moderator evaluation.
// Primary is the configured moderator agent, fallbacks are other agents with moderate phase enabled.
func (a *Analyzer) buildModeratorFallbackChain(wctx *Context) []string {
	primaryAgent := a.moderator.GetConfig().Agent
	agents := []string{primaryAgent}

	// Get all available agents that have moderate phase enabled
	// These can serve as fallbacks if the primary fails
	// Use ListEnabledForPhase to ensure we only get agents that are explicitly enabled
	// in the configuration and have the "moderate" phase active.
	fallbackCandidates := wctx.Agents.ListEnabledForPhase("moderate")
	for _, agentName := range fallbackCandidates {
		if agentName == primaryAgent {
			continue // Skip primary, already first
		}
		agents = append(agents, agentName)
	}

	// Limit to max 3 fallback agents to avoid infinite retries
	if len(agents) > 4 {
		agents = agents[:4]
	}

	wctx.Logger.Debug("moderator fallback chain built",
		"primary", primaryAgent,
		"total_agents", len(agents),
		"chain", agents,
	)

	return agents
}

// compactAnalysisOutput is a compact representation of AnalysisOutput for checkpoint storage.
type compactAnalysisOutput struct {
	AgentName string `json:"agent"`
	Model     string `json:"model"`
	RawOutput string `json:"output"`
}

// serializeAnalysisOutputs converts analysis outputs to JSON for checkpoint storage.
// Only essential fields are stored to reduce checkpoint size.
func serializeAnalysisOutputs(outputs []AnalysisOutput) string {
	compact := make([]compactAnalysisOutput, len(outputs))
	for i, o := range outputs {
		compact[i] = compactAnalysisOutput{
			AgentName: o.AgentName,
			Model:     o.Model,
			RawOutput: o.RawOutput,
		}
	}
	data, _ := json.Marshal(compact)
	return string(data)
}

// deserializeAnalysisOutputs reconstructs analysis outputs from checkpoint JSON.
func deserializeAnalysisOutputs(data string) ([]AnalysisOutput, error) {
	var compact []compactAnalysisOutput
	if err := json.Unmarshal([]byte(data), &compact); err != nil {
		return nil, fmt.Errorf("unmarshaling analysis outputs: %w", err)
	}

	outputs := make([]AnalysisOutput, len(compact))
	for i, c := range compact {
		outputs[i] = AnalysisOutput{
			AgentName: c.AgentName,
			Model:     c.Model,
			RawOutput: c.RawOutput,
		}
	}
	return outputs, nil
}

// getModeratorRoundCheckpoint looks for a moderator_round checkpoint to resume from.
// Returns the round number and outputs if found, or 0 and nil if not found.
func getModeratorRoundCheckpoint(state *core.WorkflowState) (int, []AnalysisOutput) {
	// Scan from the end to find the most recent moderator_round checkpoint
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := state.Checkpoints[i]
		if cp.Type == string(service.CheckpointModeratorRound) && cp.Phase == core.PhaseAnalyze {
			var metadata map[string]interface{}
			if err := json.Unmarshal(cp.Data, &metadata); err != nil {
				continue
			}

			round := 0
			if r, ok := metadata["round"].(float64); ok {
				round = int(r)
			}

			var outputs []AnalysisOutput
			if outputsStr, ok := metadata["outputs"].(string); ok {
				if parsed, err := deserializeAnalysisOutputs(outputsStr); err == nil {
					outputs = parsed
				}
			}

			if round > 0 && len(outputs) > 0 {
				return round, outputs
			}
		}
	}
	return 0, nil
}

// loadExistingAnalysis attempts to load a previously generated analysis from disk.
// This supports resuming partial runs without re-executing LLM calls.
func loadExistingAnalysis(path, agentName, model string) (*AnalysisOutput, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	if info.Size() == 0 {
		return nil, fmt.Errorf("file is empty")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Construct result wrapper to reuse parsing logic
	result := &core.ExecuteResult{
		Output: string(content),
		Model:  model,
		// Note: We lose exact token/cost metrics when reloading from raw markdown,
		// but preserving the content is more important for resume.
	}

	output := parseAnalysisOutputWithMetrics(agentName, model, result, 0)
	return &output, nil
}

// AnalysisCheckpointMetadata stores metadata for analysis_complete checkpoints.
// This enables cache validation and metrics restoration on resume.
type AnalysisCheckpointMetadata struct {
	AgentName   string `json:"agent_name"`
	Model       string `json:"model"`
	Round       int    `json:"round"`       // 1 for V1, 2 for V2, etc.
	FilePath    string `json:"file_path"`   // Path to analysis file on disk
	PromptHash  string `json:"prompt_hash"` // SHA256 of effective prompt for cache invalidation
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
	DurationMS  int64  `json:"duration_ms"`
	ContentHash string `json:"content_hash"` // SHA256 of file content for integrity
}

// computePromptHash returns SHA256 hash of effective prompt for cache invalidation.
func computePromptHash(state *core.WorkflowState) string {
	prompt := GetEffectivePrompt(state)
	h := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(h[:])
}

// computeContentHash returns SHA256 hash of content for integrity validation.
func computeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// getAnalysisCheckpoint looks for a valid analysis_complete checkpoint for agent/round.
// Returns nil if no valid checkpoint exists or if prompt hash doesn't match.
func getAnalysisCheckpoint(state *core.WorkflowState, agentName string, round int, expectedPromptHash string) *AnalysisCheckpointMetadata {
	for i := len(state.Checkpoints) - 1; i >= 0; i-- {
		cp := state.Checkpoints[i]
		if cp.Type != string(service.CheckpointAnalysisComplete) || cp.Phase != core.PhaseAnalyze {
			continue
		}

		var meta AnalysisCheckpointMetadata
		if err := json.Unmarshal(cp.Data, &meta); err != nil {
			continue
		}

		// Match agent and round
		if meta.AgentName != agentName || meta.Round != round {
			continue
		}

		// Validate prompt hash (cache invalidation if prompt changed)
		if meta.PromptHash != expectedPromptHash {
			continue
		}

		return &meta
	}
	return nil
}

// restoreAnalysisFromCheckpoint reconstructs AnalysisOutput from checkpoint + file.
// Returns error if file doesn't exist or content hash doesn't match.
func restoreAnalysisFromCheckpoint(meta *AnalysisCheckpointMetadata) (*AnalysisOutput, error) {
	content, err := os.ReadFile(meta.FilePath)
	if err != nil {
		return nil, fmt.Errorf("reading cached file: %w", err)
	}

	// Validate content integrity
	if computeContentHash(string(content)) != meta.ContentHash {
		return nil, fmt.Errorf("content hash mismatch - file was modified")
	}

	// Construct result wrapper with restored metrics
	result := &core.ExecuteResult{
		Output:    string(content),
		Model:     meta.Model,
		TokensIn:  meta.TokensIn,
		TokensOut: meta.TokensOut,
	}

	output := parseAnalysisOutputWithMetrics(meta.AgentName, meta.Model, result, meta.DurationMS)
	return &output, nil
}

// createAnalysisCheckpoint creates a checkpoint after successful analysis execution.
// It validates that the file exists before creating the checkpoint to prevent
// downstream failures (e.g., moderator failing with "Missing analysis files").
func createAnalysisCheckpoint(wctx *Context, agentName, model string, round int, filePath string, output AnalysisOutput, promptHash string) error {
	// Validate file exists before creating checkpoint
	if filePath != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		if err := enforcement.ValidateBeforeCheckpoint(filePath); err != nil {
			wctx.Logger.Warn("skipping checkpoint due to missing file",
				"agent", agentName,
				"round", round,
				"file_path", filePath,
				"error", err)
			return err
		}
	}

	meta := map[string]interface{}{
		"agent_name":   agentName,
		"model":        model,
		"round":        round,
		"file_path":    filePath,
		"prompt_hash":  promptHash,
		"tokens_in":    output.TokensIn,
		"tokens_out":   output.TokensOut,
		"duration_ms":  output.DurationMS,
		"content_hash": computeContentHash(output.RawOutput),
	}

	return wctx.Checkpoint.CreateCheckpoint(wctx.State, string(service.CheckpointAnalysisComplete), meta)
}
