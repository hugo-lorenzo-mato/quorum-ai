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
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

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

// ConsensusEvaluator evaluates consensus between analysis outputs.
type ConsensusEvaluator interface {
	Evaluate(outputs []AnalysisOutput) ConsensusResult
	Threshold() float64
	V2Threshold() float64
	HumanThreshold() float64
}

// Analyzer runs the analysis phase with V1/V2/V3 protocol.
type Analyzer struct {
	consensus ConsensusEvaluator
}

// NewAnalyzer creates a new analyzer.
func NewAnalyzer(consensus ConsensusEvaluator) *Analyzer {
	return &Analyzer{
		consensus: consensus,
	}
}

// Run executes the complete analysis phase.
func (a *Analyzer) Run(ctx context.Context, wctx *Context) error {
	wctx.Logger.Info("starting analyze phase", "workflow_id", wctx.State.WorkflowID)

	wctx.State.CurrentPhase = core.PhaseAnalyze
	if wctx.Output != nil {
		wctx.Output.PhaseStarted(core.PhaseAnalyze)
		wctx.Output.Log("info", "analyzer", "Starting multi-agent analysis phase")
	}
	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, false); err != nil {
		wctx.Logger.Warn("failed to create phase checkpoint", "error", err)
	}

	// V1: Initial analysis from multiple agents
	v1Outputs, err := a.runV1Analysis(ctx, wctx)
	if err != nil {
		return fmt.Errorf("V1 analysis: %w", err)
	}

	// Check consensus
	consensusResult := a.consensus.Evaluate(v1Outputs)

	// Save consensus score to metrics
	wctx.UpdateMetrics(func(m *core.StateMetrics) {
		m.ConsensusScore = consensusResult.Score
	})

	if err := wctx.Checkpoint.ConsensusCheckpoint(wctx.State, consensusResult); err != nil {
		wctx.Logger.Warn("failed to create consensus checkpoint", "error", err)
	}

	wctx.Logger.Info("V1 consensus evaluated",
		"score", consensusResult.Score,
		"threshold", a.consensus.Threshold(),
		"needs_escalation", consensusResult.NeedsV3,
		"needs_human_review", consensusResult.NeedsHumanReview,
	)
	if wctx.Output != nil {
		statusIcon := "✓"
		level := "success"
		if consensusResult.NeedsV3 {
			statusIcon = "⚠"
			level = "warn"
		}
		wctx.Output.Log(level, "analyzer", fmt.Sprintf("%s V1 Consensus: %.1f%% (threshold: %.1f%%)", statusIcon, consensusResult.Score*100, a.consensus.Threshold()*100))
	}

	// Write V1 consensus report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteConsensusReport(a.buildConsensusData(consensusResult, v1Outputs), "v1"); reportErr != nil {
			wctx.Logger.Warn("failed to write V1 consensus report", "error", reportErr)
		}
	}

	// V2/V3 escalation: If consensus below approval threshold
	if consensusResult.NeedsV3 {
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", "Consensus below threshold, escalating to V2 critique")
		}
		v2Outputs, err := a.runV2Critique(ctx, wctx, v1Outputs)
		if err != nil {
			return fmt.Errorf("V2 critique: %w", err)
		}

		// Re-evaluate consensus after V2
		allOutputs := make([]AnalysisOutput, 0, len(v1Outputs)+len(v2Outputs))
		allOutputs = append(allOutputs, v1Outputs...)
		allOutputs = append(allOutputs, v2Outputs...)
		consensusResult = a.consensus.Evaluate(allOutputs)

		// Update consensus score to metrics
		wctx.UpdateMetrics(func(m *core.StateMetrics) {
			m.ConsensusScore = consensusResult.Score
		})

		if err := wctx.Checkpoint.ConsensusCheckpoint(wctx.State, consensusResult); err != nil {
			wctx.Logger.Warn("failed to create V2 consensus checkpoint", "error", err)
		}

		wctx.Logger.Info("V2 consensus evaluated",
			"score", consensusResult.Score,
			"needs_v3", consensusResult.Score < a.consensus.V2Threshold(),
			"needs_human_review", consensusResult.NeedsHumanReview,
		)
		if wctx.Output != nil {
			wctx.Output.Log("info", "analyzer", fmt.Sprintf("V2 Consensus: %.1f%% (V2 threshold: %.1f%%)", consensusResult.Score*100, a.consensus.V2Threshold()*100))
		}

		// Write V2 consensus report
		if wctx.Report != nil {
			if reportErr := wctx.Report.WriteConsensusReport(a.buildConsensusData(consensusResult, allOutputs), "v2"); reportErr != nil {
				wctx.Logger.Warn("failed to write V2 consensus report", "error", reportErr)
			}
		}

		// V3: If score below V2 threshold, run reconciliation
		if consensusResult.Score < a.consensus.V2Threshold() {
			if wctx.Output != nil {
				wctx.Output.Log("info", "analyzer", "Score still below threshold, escalating to V3 reconciliation")
			}
			if err := a.runV3Reconciliation(ctx, wctx, v1Outputs, v2Outputs, consensusResult); err != nil {
				return fmt.Errorf("V3 reconciliation: %w", err)
			}

			// Final evaluation after V3
			// Note: V3 doesn't produce new structured outputs, it synthesizes existing ones
			// The NeedsHumanReview flag from the last evaluation determines if we must abort
		}
	}

	// Check if human review is still required after all escalation attempts
	if consensusResult.NeedsHumanReview {
		wctx.Logger.Error("consensus score below human threshold, aborting workflow",
			"score", consensusResult.Score,
			"human_threshold", a.consensus.HumanThreshold(),
		)
		if wctx.Output != nil {
			wctx.Output.Log("error", "analyzer", fmt.Sprintf("Human review required: consensus %.1f%% below threshold %.1f%%", consensusResult.Score*100, a.consensus.HumanThreshold()*100))
		}
		return core.ErrHumanReviewRequired(consensusResult.Score, a.consensus.HumanThreshold())
	}

	// Consolidate analysis using LLM
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "Consolidating analysis results")
	}
	if err := a.consolidateAnalysis(ctx, wctx, v1Outputs); err != nil {
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

// runV1Analysis runs initial analysis with multiple agents in parallel.
// It tolerates partial failures - continues as long as at least 2 agents succeed.
func (a *Analyzer) runV1Analysis(ctx context.Context, wctx *Context) ([]AnalysisOutput, error) {
	// Use Available to get only agents that are actually reachable (pass Ping)
	agentNames := wctx.Agents.Available(ctx)
	if len(agentNames) == 0 {
		return nil, core.ErrValidation(core.CodeNoAgents, "no agents available")
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

	// Need at least 2 successful outputs for consensus
	minRequired := 2
	if len(agentNames) < 2 {
		minRequired = 1
	}

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

	// Render prompt (use optimized prompt if available)
	prompt, err := wctx.Prompts.RenderAnalyzeV1(AnalyzeV1Params{
		Prompt:  GetEffectivePrompt(wctx.State),
		Context: BuildContextString(wctx.State),
	})
	if err != nil {
		return AnalysisOutput{}, fmt.Errorf("rendering prompt: %w", err)
	}

	// Resolve model
	model := ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, "")

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", agentName, "Running V1 analysis", map[string]interface{}{
			"phase": "analyze_v1",
			"model": model,
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
	output := parseAnalysisOutputWithMetrics(agentName, model, result, durationMS)

	// Write V1 analysis report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteV1Analysis(report.AnalysisData{
			AgentName:       agentName,
			Model:           model,
			RawOutput:       result.Output,
			Claims:          output.Claims,
			Risks:           output.Risks,
			Recommendations: output.Recommendations,
			TokensIn:        result.TokensIn,
			TokensOut:       result.TokensOut,
			CostUSD:         result.CostUSD,
			DurationMS:      durationMS,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write V1 analysis report", "agent", agentName, "error", reportErr)
		}
	}

	return output, nil
}

// runV2Critique runs critical review of V1 outputs.
func (a *Analyzer) runV2Critique(ctx context.Context, wctx *Context, v1Outputs []AnalysisOutput) ([]AnalysisOutput, error) {
	wctx.Logger.Info("starting V2 critique phase")
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", "V2 Critique: cross-reviewing agent outputs")
	}

	outputs := make([]AnalysisOutput, 0)

	// Each agent critiques the other's output
	for i, v1 := range v1Outputs {
		critiqueAgent := a.selectCritiqueAgent(ctx, wctx, v1.AgentName)
		agent, err := wctx.Agents.Get(critiqueAgent)
		if err != nil {
			wctx.Logger.Warn("critique agent not available",
				"agent", critiqueAgent,
				"error", err,
			)
			continue
		}

		// Acquire rate limit
		limiter := wctx.RateLimits.Get(critiqueAgent)
		if err := limiter.Acquire(); err != nil {
			wctx.Logger.Warn("rate limit exceeded for critique", "agent", critiqueAgent)
			continue
		}

		prompt, err := wctx.Prompts.RenderAnalyzeV2(AnalyzeV2Params{
			Prompt:     GetEffectivePrompt(wctx.State),
			V1Analysis: v1.RawOutput,
			AgentName:  v1.AgentName,
		})
		if err != nil {
			wctx.Logger.Warn("failed to render V2 prompt", "error", err)
			continue
		}

		// Resolve model and track time
		model := ResolvePhaseModel(wctx.Config, critiqueAgent, core.PhaseAnalyze, "")
		startTime := time.Now()

		// Emit started event
		if wctx.Output != nil {
			wctx.Output.AgentEvent("started", critiqueAgent, fmt.Sprintf("Running V2 critique of %s", v1.AgentName), map[string]interface{}{
				"phase":        "analyze_v2",
				"target_agent": v1.AgentName,
				"model":        model,
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
				wctx.Output.AgentEvent("error", critiqueAgent, err.Error(), map[string]interface{}{
					"phase":        "analyze_v2",
					"target_agent": v1.AgentName,
					"model":        model,
					"duration_ms":  time.Since(startTime).Milliseconds(),
					"error_type":   fmt.Sprintf("%T", err),
				})
			}
		}

		if err == nil {
			if wctx.Output != nil {
				wctx.Output.AgentEvent("completed", critiqueAgent, fmt.Sprintf("V2 critique of %s completed", v1.AgentName), map[string]interface{}{
					"phase":        "analyze_v2",
					"target_agent": v1.AgentName,
					"model":        result.Model,
					"tokens_in":    result.TokensIn,
					"tokens_out":   result.TokensOut,
					"cost_usd":     result.CostUSD,
					"duration_ms":  time.Since(startTime).Milliseconds(),
				})
			}
			durationMS := time.Since(startTime).Milliseconds()
			outputName := fmt.Sprintf("%s-critique-%d", critiqueAgent, i)
			output := parseAnalysisOutputWithMetrics(outputName, model, result, durationMS)
			outputs = append(outputs, output)

			// Write V2 critique report
			if wctx.Report != nil {
				critiqueData := report.CritiqueData{
					CriticAgent:   critiqueAgent,
					CriticModel:   model,
					TargetAgent:   v1.AgentName,
					RawOutput:     result.Output,
					TokensIn:      result.TokensIn,
					TokensOut:     result.TokensOut,
					CostUSD:       result.CostUSD,
					DurationMS:    durationMS,
				}
				// Parse critique-specific fields from JSON
				a.parseCritiqueFields(result.Output, &critiqueData)

				if reportErr := wctx.Report.WriteV2Critique(critiqueData); reportErr != nil {
					wctx.Logger.Warn("failed to write V2 critique report",
						"critic", critiqueAgent,
						"target", v1.AgentName,
						"error", reportErr,
					)
				}
			}
		} else {
			wctx.Logger.Warn("V2 critique failed",
				"agent", critiqueAgent,
				"error", err,
			)
		}
	}

	return outputs, nil
}

// runV3Reconciliation runs synthesis of conflicting analyses.
func (a *Analyzer) runV3Reconciliation(ctx context.Context, wctx *Context, v1, v2 []AnalysisOutput, consensus ConsensusResult) error {
	wctx.Logger.Info("starting V3 reconciliation phase",
		"divergences", len(consensus.Divergences),
	)
	if wctx.Output != nil {
		wctx.Output.Log("info", "analyzer", fmt.Sprintf("V3 Reconciliation: resolving %d divergences", len(consensus.Divergences)))
	}

	// Use V3 agent (typically Claude)
	v3AgentName := wctx.Config.V3Agent
	if v3AgentName == "" {
		v3AgentName = "claude"
	}

	agent, err := wctx.Agents.Get(v3AgentName)
	if err != nil {
		return fmt.Errorf("getting V3 agent: %w", err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(v3AgentName)
	if err := limiter.Acquire(); err != nil {
		return fmt.Errorf("rate limit for V3: %w", err)
	}

	// Combine all outputs
	var v1Combined, v2Combined strings.Builder
	for _, o := range v1 {
		v1Combined.WriteString(fmt.Sprintf("### %s\n%s\n\n", o.AgentName, o.RawOutput))
	}
	for _, o := range v2 {
		v2Combined.WriteString(fmt.Sprintf("### %s\n%s\n\n", o.AgentName, o.RawOutput))
	}

	divergenceStrings := consensus.DivergenceStrings()
	prompt, err := wctx.Prompts.RenderAnalyzeV3(AnalyzeV3Params{
		Prompt:      GetEffectivePrompt(wctx.State),
		V1Analysis:  v1Combined.String(),
		V2Analysis:  v2Combined.String(),
		Divergences: divergenceStrings,
	})
	if err != nil {
		return fmt.Errorf("rendering V3 prompt: %w", err)
	}

	// Resolve model and track time
	model := ResolvePhaseModel(wctx.Config, v3AgentName, core.PhaseAnalyze, "")
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", v3AgentName, "Running V3 reconciliation", map[string]interface{}{
			"phase":            "analyze_v3",
			"model":            model,
			"divergence_count": len(divergenceStrings),
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
			wctx.Output.AgentEvent("error", v3AgentName, err.Error(), map[string]interface{}{
				"phase":       "analyze_v3",
				"model":       model,
				"duration_ms": time.Since(startTime).Milliseconds(),
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return err
	}

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", v3AgentName, "V3 reconciliation completed", map[string]interface{}{
			"phase":       "analyze_v3",
			"model":       result.Model,
			"tokens_in":   result.TokensIn,
			"tokens_out":  result.TokensOut,
			"cost_usd":    result.CostUSD,
			"duration_ms": time.Since(startTime).Milliseconds(),
		})
	}

	durationMS := time.Since(startTime).Milliseconds()

	// Write V3 reconciliation report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteV3Reconciliation(report.ReconciliationData{
			Agent:      v3AgentName,
			Model:      model,
			RawOutput:  result.Output,
			TokensIn:   result.TokensIn,
			TokensOut:  result.TokensOut,
			CostUSD:    result.CostUSD,
			DurationMS: durationMS,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write V3 reconciliation report", "error", reportErr)
		}
	}

	// Store V3 result in state metadata
	if wctx.State.Metrics == nil {
		wctx.State.Metrics = &core.StateMetrics{}
	}

	// Create a checkpoint with the V3 reconciliation result
	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "v3_reconciliation", map[string]interface{}{
		"output":     result.Output,
		"tokens_in":  result.TokensIn,
		"tokens_out": result.TokensOut,
		"cost_usd":   result.CostUSD,
	})
}

// consolidateAnalysis uses an LLM to synthesize all analysis outputs into a unified report.
func (a *Analyzer) consolidateAnalysis(ctx context.Context, wctx *Context, outputs []AnalysisOutput) error {
	// Get consolidator agent
	consolidatorAgent := wctx.Config.ConsolidatorAgent
	if consolidatorAgent == "" {
		consolidatorAgent = wctx.Config.DefaultAgent
	}

	agent, err := wctx.Agents.Get(consolidatorAgent)
	if err != nil {
		wctx.Logger.Warn("consolidator agent not available, using concatenation fallback",
			"agent", consolidatorAgent,
			"error", err,
		)
		return a.consolidateAnalysisFallback(wctx, outputs)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(consolidatorAgent)
	if err := limiter.Acquire(); err != nil {
		wctx.Logger.Warn("rate limit exceeded for consolidator, using fallback",
			"agent", consolidatorAgent,
		)
		return a.consolidateAnalysisFallback(wctx, outputs)
	}

	// Render consolidation prompt
	prompt, err := wctx.Prompts.RenderConsolidateAnalysis(ConsolidateAnalysisParams{
		Prompt:   GetEffectivePrompt(wctx.State),
		Analyses: outputs,
	})
	if err != nil {
		wctx.Logger.Warn("failed to render consolidation prompt, using fallback",
			"error", err,
		)
		return a.consolidateAnalysisFallback(wctx, outputs)
	}

	// Resolve model
	model := wctx.Config.ConsolidatorModel
	if model == "" {
		model = ResolvePhaseModel(wctx.Config, consolidatorAgent, core.PhaseAnalyze, "")
	}

	wctx.Logger.Info("consolidate start",
		"agent", consolidatorAgent,
		"model", model,
		"analyses_count", len(outputs),
	)

	// Track start time
	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", consolidatorAgent, "Consolidating analyses", map[string]interface{}{
			"phase":          "consolidate",
			"model":          model,
			"analyses_count": len(outputs),
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
			wctx.Output.AgentEvent("error", consolidatorAgent, err.Error(), map[string]interface{}{
				"phase":          "consolidate",
				"model":          model,
				"analyses_count": len(outputs),
				"duration_ms":    time.Since(startTime).Milliseconds(),
				"error_type":     fmt.Sprintf("%T", err),
				"fallback":       true,
			})
		}
		wctx.Logger.Warn("consolidation LLM call failed, using fallback",
			"error", err,
		)
		return a.consolidateAnalysisFallback(wctx, outputs)
	}

	durationMS := time.Since(startTime).Milliseconds()

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", consolidatorAgent, "Consolidation completed", map[string]interface{}{
			"phase":          "consolidate",
			"model":          result.Model,
			"analyses_count": len(outputs),
			"tokens_in":      result.TokensIn,
			"tokens_out":     result.TokensOut,
			"cost_usd":       result.CostUSD,
			"duration_ms":    durationMS,
		})
	}

	wctx.Logger.Info("consolidate done",
		"agent", consolidatorAgent,
		"model", model,
	)

	// Calculate totals from all outputs
	totalTokensIn, totalTokensOut, totalCost := a.calculateOutputTotals(outputs)

	// Write consolidated analysis report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteConsolidatedAnalysis(report.ConsolidationData{
			Agent:           consolidatorAgent,
			Model:           model,
			Content:         result.Output,
			AnalysesCount:   len(outputs),
			Synthesized:     true,
			ConsensusScore:  wctx.State.Metrics.ConsensusScore,
			TotalTokensIn:   totalTokensIn + result.TokensIn,
			TotalTokensOut:  totalTokensOut + result.TokensOut,
			TotalCostUSD:    totalCost + result.CostUSD,
			TotalDurationMS: durationMS,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write consolidated analysis report", "error", reportErr)
		}
	}

	// Store the LLM-synthesized consolidation as checkpoint
	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     result.Output,
		"agent_count": len(outputs),
		"synthesized": true,
		"agent":       consolidatorAgent,
		"model":       model,
		"tokens_in":   result.TokensIn,
		"tokens_out":  result.TokensOut,
		"cost_usd":    result.CostUSD,
	})
}

// consolidateAnalysisFallback concatenates analyses when LLM consolidation fails.
func (a *Analyzer) consolidateAnalysisFallback(wctx *Context, outputs []AnalysisOutput) error {
	var consolidated strings.Builder

	for _, output := range outputs {
		consolidated.WriteString(fmt.Sprintf("## Analysis from %s\n", output.AgentName))
		consolidated.WriteString(output.RawOutput)
		consolidated.WriteString("\n\n")
	}

	content := consolidated.String()

	// Calculate totals from all outputs
	totalTokensIn, totalTokensOut, totalCost := a.calculateOutputTotals(outputs)

	// Write consolidated analysis report (fallback mode)
	if wctx.Report != nil {
		var consensusScore float64
		if wctx.State.Metrics != nil {
			consensusScore = wctx.State.Metrics.ConsensusScore
		}
		if reportErr := wctx.Report.WriteConsolidatedAnalysis(report.ConsolidationData{
			Agent:           "fallback",
			Model:           "",
			Content:         content,
			AnalysesCount:   len(outputs),
			Synthesized:     false,
			ConsensusScore:  consensusScore,
			TotalTokensIn:   totalTokensIn,
			TotalTokensOut:  totalTokensOut,
			TotalCostUSD:    totalCost,
			TotalDurationMS: 0,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write consolidated analysis report (fallback)", "error", reportErr)
		}
	}

	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     content,
		"agent_count": len(outputs),
		"synthesized": false,
	})
}

// selectCritiqueAgent selects a different available agent for critique.
func (a *Analyzer) selectCritiqueAgent(ctx context.Context, wctx *Context, original string) string {
	agents := wctx.Agents.Available(ctx)
	for _, ag := range agents {
		if ag != original {
			return ag
		}
	}
	return original
}

// buildConsensusData converts ConsensusResult to report.ConsensusData.
func (a *Analyzer) buildConsensusData(cr ConsensusResult, outputs []AnalysisOutput) report.ConsensusData {
	divergences := make([]report.DivergenceData, len(cr.Divergences))
	for i, d := range cr.Divergences {
		divergences[i] = report.DivergenceData{
			Type:        d.Category,
			Agent1:      d.Agent1,
			Agent2:      d.Agent2,
			Description: fmt.Sprintf("Jaccard score: %.2f", d.JaccardScore),
		}
	}

	return report.ConsensusData{
		Score:            cr.Score,
		Threshold:        a.consensus.Threshold(),
		NeedsEscalation:  cr.NeedsV3,
		NeedsHumanReview: cr.NeedsHumanReview,
		AgentsCount:      len(outputs),
		ClaimsScore:      cr.CategoryScores["claims"],
		RisksScore:       cr.CategoryScores["risks"],
		RecommendationsScore: cr.CategoryScores["recommendations"],
		Divergences:      divergences,
	}
}

// parseCritiqueFields parses critique-specific fields from JSON output.
func (a *Analyzer) parseCritiqueFields(output string, data *report.CritiqueData) {
	var parsed struct {
		Agreements      []string `json:"agreements"`
		Disagreements   []string `json:"disagreements"`
		AdditionalRisks []string `json:"additional_risks"`
	}
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		data.Agreements = parsed.Agreements
		data.Disagreements = parsed.Disagreements
		data.AdditionalRisks = parsed.AdditionalRisks
	}
}

// calculateOutputTotals sums up tokens and cost from all analysis outputs.
func (a *Analyzer) calculateOutputTotals(outputs []AnalysisOutput) (tokensIn, tokensOut int, costUSD float64) {
	for _, o := range outputs {
		tokensIn += o.TokensIn
		tokensOut += o.TokensOut
		costUSD += o.CostUSD
	}
	return
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

// extractMarkdownSection extracts bullet points from a Markdown section.
// Looks for sections like "## Claims", "### Claims", "**Claims**", or "Claims:" followed by bullet points.
func extractMarkdownSection(text, sectionName string) []string {
	// Pattern to find section header (case-insensitive)
	// Matches: ## Claims, ### Claims, **Claims**, Claims:
	headerPattern := regexp.MustCompile(`(?im)^(?:#{1,4}\s*` + sectionName + `|[\*_]{2}` + sectionName + `[\*_]{2}|` + sectionName + `\s*:)\s*$`)

	loc := headerPattern.FindStringIndex(text)
	if loc == nil {
		return nil
	}

	// Get text after the header
	afterHeader := text[loc[1]:]

	// Find the next section header to limit our search
	nextSectionPattern := regexp.MustCompile(`(?m)^(?:#{1,4}\s|\*\*[A-Z])`)
	nextLoc := nextSectionPattern.FindStringIndex(afterHeader)

	sectionText := afterHeader
	if nextLoc != nil {
		sectionText = afterHeader[:nextLoc[0]]
	}

	// Extract bullet points (-, *, or numbered lists)
	bulletPattern := regexp.MustCompile(`(?m)^[\s]*[-*•]\s*(.+)$|^[\s]*\d+[.)]\s*(.+)$`)
	matches := bulletPattern.FindAllStringSubmatch(sectionText, -1)

	var items []string
	for _, match := range matches {
		// match[1] is for - or * bullets, match[2] is for numbered lists
		item := match[1]
		if item == "" {
			item = match[2]
		}
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
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
