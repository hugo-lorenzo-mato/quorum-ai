package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
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
	CostUSD         float64
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

// Run executes the complete analysis phase using semantic moderator consensus.
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
		wctx.Output.Log("info", "analyzer", "Starting multi-agent analysis phase")
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// Verify moderator is configured
	if a.moderator == nil || !a.moderator.IsEnabled() {
		return fmt.Errorf("semantic moderator is required but not configured. "+
			"Enable it in your config file under 'phases.analyze.moderator.enabled: true' with agent specified. "+
			"See: %s#phases-settings", DocsConfigURL)
	}

	wctx.Logger.Info("using semantic moderator for consensus evaluation",
		"threshold", a.moderator.Threshold(),
		"min_rounds", a.moderator.MinRounds(),
		"max_rounds", a.moderator.MaxRounds(),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Semantic moderator enabled (threshold: %.0f%%, min rounds: %d, max rounds: %d)",
			a.moderator.Threshold()*100, a.moderator.MinRounds(), a.moderator.MaxRounds()))
	}

	return a.runWithModerator(ctx, wctx)
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
	// ========== PHASE 1: V1 Initial Analysis (Independent) ==========
	wctx.Logger.Info("starting V1 analysis (initial, no moderator)")
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "V1: Running initial independent analysis")
	}

	v1Outputs, err := a.runV1Analysis(ctx, wctx)
	if err != nil {
		return fmt.Errorf("V1 analysis: %w", err)
	}

	// ========== PHASE 2: V2 First Ultracritical Review of V1 ==========
	wctx.Logger.Info("starting V2 refinement (ultracritical review of V1)")
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "V2: Running ultracritical review of V1 analyses")
	}

	// V2 refines V1 with no prior arbiter evaluation (first refinement)
	v2Outputs, err := a.runVnRefinement(ctx, wctx, 2, v1Outputs, nil, nil)
	if err != nil {
		return fmt.Errorf("V2 refinement: %w", err)
	}

	// Track outputs for iteration - start with V2
	currentOutputs := v2Outputs
	round := 2

	// Track previous score for stagnation detection
	var previousScore float64

	// ========== PHASE 3: Moderator Evaluation Loop (V2+) ==========
	for round <= a.moderator.MaxRounds() {
		wctx.Logger.Info("moderator evaluation starting",
			"round", round,
			"agents", len(currentOutputs),
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", fmt.Sprintf("Round %d: Running moderator evaluation", round))
		}

		// Run moderator evaluation
		evalResult, err := a.moderator.Evaluate(ctx, wctx, round, currentOutputs)
		if err != nil {
			return fmt.Errorf("moderator evaluation round %d: %w", round, err)
		}

		// Update consensus score in metrics
		wctx.UpdateMetrics(func(m *core.StateMetrics) {
			m.ConsensusScore = evalResult.Score
		})

		wctx.Logger.Info("moderator evaluation complete",
			"round", round,
			"score", evalResult.Score,
			"threshold", a.moderator.Threshold(),
			"agreements", len(evalResult.Agreements),
			"divergences", len(evalResult.Divergences),
		)

		if wctx.Output != nil {
			statusIcon := "⚠"
			level := "warn"
			if evalResult.Score >= a.moderator.Threshold() {
				statusIcon = "✓"
				level = "success"
			}
			wctx.Output.Log(level, "analyzer", fmt.Sprintf("%s Round %d: Semantic consensus %.0f%% (threshold: %.0f%%)",
				statusIcon, round, evalResult.Score*100, a.moderator.Threshold()*100))
		}

		// Check if consensus threshold is met AND minimum rounds completed
		if evalResult.Score >= a.moderator.Threshold() {
			if round >= a.moderator.MinRounds() {
				wctx.Logger.Info("consensus threshold met",
					"score", evalResult.Score,
					"threshold", a.moderator.Threshold(),
					"round", round,
				)
				if wctx.Output != nil {
					wctx.Output.Log("success", "analyzer", fmt.Sprintf("Consensus achieved at %.0f%% after %d round(s)", evalResult.Score*100, round))
				}
				break
			}
			// Threshold met but minimum rounds not reached - continue refinement
			wctx.Logger.Info("consensus threshold met but minimum rounds not reached",
				"score", evalResult.Score,
				"threshold", a.moderator.Threshold(),
				"round", round,
				"min_rounds", a.moderator.MinRounds(),
			)
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", fmt.Sprintf("Threshold met (%.0f%%) but continuing to min rounds (%d/%d)",
					evalResult.Score*100, round, a.moderator.MinRounds()))
			}
		}

		// Check abort threshold
		if evalResult.Score < a.moderator.AbortThreshold() {
			wctx.Logger.Error("consensus score below abort threshold",
				"score", evalResult.Score,
				"abort_threshold", a.moderator.AbortThreshold(),
			)
			if wctx.Output != nil {
				wctx.Output.Log("error", "analyzer", fmt.Sprintf("Human review required: consensus %.0f%% below abort threshold %.0f%%",
					evalResult.Score*100, a.moderator.AbortThreshold()*100))
			}
			return core.ErrHumanReviewRequired(evalResult.Score, a.moderator.AbortThreshold())
		}

		// Check for stagnation (score not improving) - only after first moderator eval (round > 2)
		if round > 2 {
			improvement := evalResult.Score - previousScore
			if improvement < a.moderator.StagnationThreshold() {
				wctx.Logger.Warn("consensus stagnating, exiting refinement loop",
					"improvement", improvement,
					"stagnation_threshold", a.moderator.StagnationThreshold(),
				)
				if wctx.Output != nil {
					wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Consensus stagnating (improvement %.1f%% < %.1f%%), proceeding with current score",
						improvement*100, a.moderator.StagnationThreshold()*100))
				}
				break
			}
		}
		previousScore = evalResult.Score

		// Check if we've reached max rounds
		if round >= a.moderator.MaxRounds() {
			wctx.Logger.Warn("max rounds reached without consensus",
				"round", round,
				"max_rounds", a.moderator.MaxRounds(),
				"final_score", evalResult.Score,
			)
			if wctx.Output != nil {
				wctx.Output.Log("warn", "analyzer", fmt.Sprintf("Max rounds (%d) reached. Final consensus: %.0f%%",
					a.moderator.MaxRounds(), evalResult.Score*100))
			}
			break
		}

		// ========== Run V(n+1) refinement ==========
		// CRITICAL: V(n+1) reviews ONLY V(n), NOT previous versions
		round++
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

		// V(n+1) takes ONLY V(n) outputs - this ensures no cross-version references
		refinedOutputs, err := a.runVnRefinement(ctx, wctx, round, currentOutputs, evalResult, evalResult.Agreements)
		if err != nil {
			return fmt.Errorf("V%d refinement: %w", round, err)
		}

		// Update currentOutputs for next iteration
		currentOutputs = refinedOutputs
	}

	// ========== PHASE 4: Final Consolidation ==========
	// Consolidation uses ONLY the latest V(n) outputs, never V1 directly
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("Consolidating final analysis from V%d outputs", round))
	}
	if err := a.consolidateAnalysis(ctx, wctx, currentOutputs); err != nil {
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
	agentNames := wctx.Agents.AvailableForPhase(ctx, "analyze")
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

	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return AnalysisOutput{}, fmt.Errorf("rate limit: %w", err)
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

	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")

	// Get output file path for LLM to write directly
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.VnAnalysisPath(agentName, model, round)
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
		Threshold:            a.moderator.Threshold() * 100,
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
		wctx.Output.AgentEvent("started", agentName, fmt.Sprintf("Running V%d refinement", round), map[string]interface{}{
			"phase":           fmt.Sprintf("analyze_v%d", round),
			"model":           model,
			"round":           round,
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
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
		wctx.Output.AgentEvent("completed", agentName, fmt.Sprintf("V%d refinement completed", round), map[string]interface{}{
			"phase":       fmt.Sprintf("analyze_v%d", round),
			"model":       result.Model,
			"round":       round,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"cost_usd":    result.CostUSD,
			"duration_ms": durationMS,
		})
	}

	outputName := fmt.Sprintf("v%d-%s", round, agentName)
	output := parseAnalysisOutputWithMetrics(outputName, model, result, durationMS)

	return output, nil
}

// runV1Analysis runs initial analysis with multiple agents in parallel.
// It tolerates partial failures - continues as long as at least 2 agents succeed.
func (a *Analyzer) runV1Analysis(ctx context.Context, wctx *Context) ([]AnalysisOutput, error) {
	// Use AvailableForPhase to only get agents enabled for analyze phase
	agentNames := wctx.Agents.AvailableForPhase(ctx, "analyze")
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

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(agentName)
	if err := limiter.Acquire(); err != nil {
		return AnalysisOutput{}, fmt.Errorf("rate limit: %w", err)
	}

	// Resolve model
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")

	// Get output file path for LLM to write directly
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.V1AnalysisPath(agentName, model)
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
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
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
			"cost_usd":    result.CostUSD,
			"duration_ms": time.Since(startTime).Milliseconds(),
		})
	}

	// Calculate duration
	durationMS := time.Since(startTime).Milliseconds()

	// Parse output with metrics
	// Use v1-{agent} naming convention for V1 outputs
	outputName := fmt.Sprintf("v1-%s", agentName)
	output := parseAnalysisOutputWithMetrics(outputName, model, result, durationMS)

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
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Sandbox: wctx.Config.Sandbox,
			Phase:   core.PhaseAnalyze,
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
			"cost_usd":       result.CostUSD,
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
		"cost_usd":    result.CostUSD,
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
		CostUSD:    result.CostUSD,
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
