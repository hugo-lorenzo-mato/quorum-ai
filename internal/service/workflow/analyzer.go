package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"golang.org/x/sync/errgroup"
)

// AnalysisOutput represents output from an analysis agent.
type AnalysisOutput struct {
	AgentName       string
	RawOutput       string
	Claims          []string
	Risks           []string
	Recommendations []string
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
	if err := wctx.Checkpoint.ConsensusCheckpoint(wctx.State, consensusResult); err != nil {
		wctx.Logger.Warn("failed to create consensus checkpoint", "error", err)
	}

	wctx.Logger.Info("V1 consensus evaluated",
		"score", consensusResult.Score,
		"threshold", a.consensus.Threshold(),
		"needs_escalation", consensusResult.NeedsV3,
		"needs_human_review", consensusResult.NeedsHumanReview,
	)

	// V2/V3 escalation: If consensus below approval threshold
	if consensusResult.NeedsV3 {
		v2Outputs, err := a.runV2Critique(ctx, wctx, v1Outputs)
		if err != nil {
			return fmt.Errorf("V2 critique: %w", err)
		}

		// Re-evaluate consensus after V2
		allOutputs := make([]AnalysisOutput, 0, len(v1Outputs)+len(v2Outputs))
		allOutputs = append(allOutputs, v1Outputs...)
		allOutputs = append(allOutputs, v2Outputs...)
		consensusResult = a.consensus.Evaluate(allOutputs)
		if err := wctx.Checkpoint.ConsensusCheckpoint(wctx.State, consensusResult); err != nil {
			wctx.Logger.Warn("failed to create V2 consensus checkpoint", "error", err)
		}

		wctx.Logger.Info("V2 consensus evaluated",
			"score", consensusResult.Score,
			"needs_v3", consensusResult.Score < a.consensus.V2Threshold(),
			"needs_human_review", consensusResult.NeedsHumanReview,
		)

		// V3: If score below V2 threshold, run reconciliation
		if consensusResult.Score < a.consensus.V2Threshold() {
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
		return core.ErrHumanReviewRequired(consensusResult.Score, a.consensus.HumanThreshold())
	}

	// Consolidate analysis
	if err := a.consolidateAnalysis(wctx, v1Outputs); err != nil {
		return fmt.Errorf("consolidating analysis: %w", err)
	}

	if err := wctx.Checkpoint.PhaseCheckpoint(wctx.State, core.PhaseAnalyze, true); err != nil {
		wctx.Logger.Warn("failed to create phase complete checkpoint", "error", err)
	}

	return nil
}

// runV1Analysis runs initial analysis with multiple agents in parallel.
func (a *Analyzer) runV1Analysis(ctx context.Context, wctx *Context) ([]AnalysisOutput, error) {
	// Use Available to get only agents that are actually reachable (pass Ping)
	agentNames := wctx.Agents.Available(ctx)
	if len(agentNames) == 0 {
		return nil, core.ErrValidation(core.CodeNoAgents, "no agents available")
	}

	wctx.Logger.Info("running V1 analysis",
		"agents", strings.Join(agentNames, ", "),
	)

	var mu sync.Mutex
	outputs := make([]AnalysisOutput, 0, len(agentNames))

	g, gctx := errgroup.WithContext(ctx)
	for _, name := range agentNames {
		name := name // capture
		g.Go(func() error {
			output, err := a.runAnalysisWithAgent(gctx, wctx, name)
			if err != nil {
				wctx.Logger.Error("agent analysis failed",
					"agent", name,
					"error", err,
				)
				return err
			}
			mu.Lock()
			outputs = append(outputs, output)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
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

	// Execute with retry
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   ResolvePhaseModel(wctx.Config, agentName, core.PhaseAnalyze, ""),
			Timeout: 5 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return AnalysisOutput{}, err
	}

	// Parse output
	output := parseAnalysisOutput(agentName, result)
	return output, nil
}

// runV2Critique runs critical review of V1 outputs.
func (a *Analyzer) runV2Critique(ctx context.Context, wctx *Context, v1Outputs []AnalysisOutput) ([]AnalysisOutput, error) {
	wctx.Logger.Info("starting V2 critique phase")

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

		var result *core.ExecuteResult
		err = wctx.Retry.Execute(func() error {
			var execErr error
			result, execErr = agent.Execute(ctx, core.ExecuteOptions{
				Prompt:  prompt,
				Format:  core.OutputFormatJSON,
				Model:   ResolvePhaseModel(wctx.Config, critiqueAgent, core.PhaseAnalyze, ""),
				Timeout: 5 * time.Minute,
				Sandbox: wctx.Config.Sandbox,
			})
			return execErr
		})

		if err == nil {
			output := parseAnalysisOutput(fmt.Sprintf("%s-critique-%d", critiqueAgent, i), result)
			outputs = append(outputs, output)
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

	prompt, err := wctx.Prompts.RenderAnalyzeV3(AnalyzeV3Params{
		Prompt:      GetEffectivePrompt(wctx.State),
		V1Analysis:  v1Combined.String(),
		V2Analysis:  v2Combined.String(),
		Divergences: consensus.Divergences,
	})
	if err != nil {
		return fmt.Errorf("rendering V3 prompt: %w", err)
	}

	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		var execErr error
		result, execErr = agent.Execute(ctx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatJSON,
			Model:   ResolvePhaseModel(wctx.Config, v3AgentName, core.PhaseAnalyze, ""),
			Timeout: 10 * time.Minute,
			Sandbox: wctx.Config.Sandbox,
		})
		return execErr
	})

	if err != nil {
		return err
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

// consolidateAnalysis combines all analysis outputs.
func (a *Analyzer) consolidateAnalysis(wctx *Context, outputs []AnalysisOutput) error {
	var consolidated strings.Builder

	for _, output := range outputs {
		consolidated.WriteString(fmt.Sprintf("## Analysis from %s\n", output.AgentName))
		consolidated.WriteString(output.RawOutput)
		consolidated.WriteString("\n\n")
	}

	return wctx.Checkpoint.CreateCheckpoint(wctx.State, "consolidated_analysis", map[string]interface{}{
		"content":     consolidated.String(),
		"agent_count": len(outputs),
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

// parseAnalysisOutput parses agent output into AnalysisOutput.
func parseAnalysisOutput(agentName string, result *core.ExecuteResult) AnalysisOutput {
	output := AnalysisOutput{
		AgentName: agentName,
		RawOutput: result.Output,
	}

	var parsed struct {
		Claims          []string `json:"claims"`
		Risks           []string `json:"risks"`
		Recommendations []string `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(result.Output), &parsed); err == nil {
		output.Claims = parsed.Claims
		output.Risks = parsed.Risks
		output.Recommendations = parsed.Recommendations
	}

	return output
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
