package workflow

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

// ArbiterEvaluationResult contains the result of a semantic arbiter evaluation.
type ArbiterEvaluationResult struct {
	// Score is the semantic consensus score (0.0-1.0).
	Score float64
	// RawOutput is the full arbiter response.
	RawOutput string
	// Agreements are points where all analyses converge.
	Agreements []string
	// Divergences are points where analyses differ.
	Divergences []ArbiterDivergence
	// MissingPerspectives are gaps identified by the arbiter.
	MissingPerspectives []string
	// Recommendations are guidance for the next round.
	Recommendations []string
	// TokensIn is the input token count.
	TokensIn int
	// TokensOut is the output token count.
	TokensOut int
	// CostUSD is the cost of this evaluation.
	CostUSD float64
	// DurationMS is the execution time in milliseconds.
	DurationMS int64
}

// ArbiterDivergence represents a divergence identified by the arbiter.
type ArbiterDivergence struct {
	Description    string
	AgentPositions map[string]string
	Impact         string
}

// SemanticArbiter evaluates semantic consensus between agent analyses using an LLM.
type SemanticArbiter struct {
	config ArbiterConfig
}

// NewSemanticArbiter creates a new semantic arbiter.
// Configuration defaults are set in internal/config/loader.go (setDefaults).
// Returns an error if enabled but agent or model are not configured.
func NewSemanticArbiter(config ArbiterConfig) (*SemanticArbiter, error) {
	// Validate required fields when enabled
	if config.Enabled {
		if config.Agent == "" {
			return nil, fmt.Errorf("consensus.arbiter.agent is required when arbiter is enabled")
		}
		if config.Model == "" {
			return nil, fmt.Errorf("consensus.arbiter.model is required when arbiter is enabled")
		}
	}

	// Ensure min_rounds <= max_rounds (validation only, not default setting)
	if config.MinRounds > config.MaxRounds && config.MaxRounds > 0 {
		config.MinRounds = config.MaxRounds
	}

	return &SemanticArbiter{
		config: config,
	}, nil
}

// Evaluate runs semantic consensus evaluation on the given analyses.
func (a *SemanticArbiter) Evaluate(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput) (*ArbiterEvaluationResult, error) {
	if !a.config.Enabled {
		return nil, fmt.Errorf("semantic arbiter is not enabled")
	}

	// Get arbiter agent
	arbiterAgentName := a.config.Agent
	agent, err := wctx.Agents.Get(arbiterAgentName)
	if err != nil {
		return nil, fmt.Errorf("getting arbiter agent %s: %w", arbiterAgentName, err)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(arbiterAgentName)
	if err := limiter.Acquire(); err != nil {
		return nil, fmt.Errorf("rate limit for arbiter: %w", err)
	}

	// Build analysis summaries for the prompt
	analyses := make([]ArbiterAnalysisSummary, len(outputs))
	for i, out := range outputs {
		analyses[i] = ArbiterAnalysisSummary{
			AgentName: out.AgentName,
			Output:    out.RawOutput,
		}
	}

	// Render arbiter prompt
	prompt, err := wctx.Prompts.RenderArbiterEvaluate(ArbiterEvaluateParams{
		Prompt:         GetEffectivePrompt(wctx.State),
		Round:          round,
		Analyses:       analyses,
		BelowThreshold: true, // Always request recommendations
	})
	if err != nil {
		return nil, fmt.Errorf("rendering arbiter prompt: %w", err)
	}

	// Resolve model
	model := a.config.Model
	if model == "" {
		model = ResolvePhaseModel(wctx.Config, arbiterAgentName, core.PhaseAnalyze, "")
	}

	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", arbiterAgentName, fmt.Sprintf("Running semantic arbiter evaluation (round %d)", round), map[string]interface{}{
			"phase":           "arbiter",
			"round":           round,
			"model":           model,
			"analyses_count":  len(outputs),
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	// Execute arbiter evaluation
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
			wctx.Output.AgentEvent("error", arbiterAgentName, err.Error(), map[string]interface{}{
				"phase":       "arbiter",
				"round":       round,
				"model":       model,
				"duration_ms": time.Since(startTime).Milliseconds(),
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return nil, err
	}

	durationMS := time.Since(startTime).Milliseconds()

	// Parse the arbiter response
	evalResult := a.parseArbiterResponse(result.Output)
	evalResult.TokensIn = result.TokensIn
	evalResult.TokensOut = result.TokensOut
	evalResult.CostUSD = result.CostUSD
	evalResult.DurationMS = durationMS

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", arbiterAgentName, fmt.Sprintf("Arbiter evaluation completed: %.0f%% consensus", evalResult.Score*100), map[string]interface{}{
			"phase":           "arbiter",
			"round":           round,
			"model":           result.Model,
			"consensus_score": evalResult.Score,
			"tokens_in":       result.TokensIn,
			"tokens_out":      result.TokensOut,
			"cost_usd":        result.CostUSD,
			"duration_ms":     durationMS,
		})
	}

	// Write arbiter report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteArbiterReport(report.ArbiterData{
			Agent:            arbiterAgentName,
			Model:            model,
			Round:            round,
			Score:            evalResult.Score,
			RawOutput:        evalResult.RawOutput,
			AgreementsCount:  len(evalResult.Agreements),
			DivergencesCount: len(evalResult.Divergences),
			TokensIn:         result.TokensIn,
			TokensOut:        result.TokensOut,
			CostUSD:          result.CostUSD,
			DurationMS:       durationMS,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write arbiter report", "round", round, "error", reportErr)
		}
	}

	return evalResult, nil
}

// parseArbiterResponse parses the arbiter's response to extract the consensus score and details.
func (a *SemanticArbiter) parseArbiterResponse(output string) *ArbiterEvaluationResult {
	result := &ArbiterEvaluationResult{
		RawOutput: output,
		Score:     0,
	}

	// Pattern 1: CONSENSUS_SCORE: XX% (primary)
	// This is the critical line that the orchestrator parses
	scorePattern := regexp.MustCompile(`(?im)^CONSENSUS_SCORE:\s*(\d+)%`)
	match := scorePattern.FindStringSubmatch(output)
	if match != nil && len(match) > 1 {
		if score, err := strconv.Atoi(match[1]); err == nil {
			result.Score = float64(score) / 100.0
		}
	}

	// Pattern 2: =-=-=-=XX%=-=-=-=-= (fallback for deep reasoning models)
	// Some models with extended thinking may embed the score in this distinctive pattern
	if result.Score == 0 {
		fallbackPattern := regexp.MustCompile(`=-=-=-=(\d+)%=-=-=-=-=`)
		match := fallbackPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.Atoi(match[1]); err == nil {
				result.Score = float64(score) / 100.0
			}
		}
	}

	// Extract agreements
	result.Agreements = extractArbiterSection(output, "Agreements")

	// Extract divergences (simplified)
	divergenceTexts := extractArbiterSection(output, "Divergences")
	for _, text := range divergenceTexts {
		result.Divergences = append(result.Divergences, ArbiterDivergence{
			Description: text,
		})
	}

	// Extract missing perspectives
	result.MissingPerspectives = extractArbiterSection(output, "Missing Perspectives")

	// Extract recommendations
	result.Recommendations = extractArbiterSection(output, "Recommendations for Next Round")

	return result
}

// extractArbiterSection extracts bullet points from a named section in the arbiter output.
func extractArbiterSection(text, sectionName string) []string {
	// Find section header
	headerPattern := regexp.MustCompile(`(?im)^#+\s*` + regexp.QuoteMeta(sectionName) + `.*$`)
	loc := headerPattern.FindStringIndex(text)
	if loc == nil {
		return nil
	}

	// Get text after header
	afterHeader := text[loc[1]:]

	// Find next section (any header)
	nextSectionPattern := regexp.MustCompile(`(?m)^#+\s+`)
	nextLoc := nextSectionPattern.FindStringIndex(afterHeader)

	sectionText := afterHeader
	if nextLoc != nil {
		sectionText = afterHeader[:nextLoc[0]]
	}

	// Extract bullet points
	var items []string
	bulletPattern := regexp.MustCompile(`(?m)^\s*[-*•]\s*(.+)$`)
	matches := bulletPattern.FindAllStringSubmatch(sectionText, -1)
	for _, match := range matches {
		item := strings.TrimSpace(match[1])
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

// Threshold returns the configured consensus threshold.
func (a *SemanticArbiter) Threshold() float64 {
	return a.config.Threshold
}

// MinRounds returns the configured minimum rounds before accepting consensus.
func (a *SemanticArbiter) MinRounds() int {
	return a.config.MinRounds
}

// MaxRounds returns the configured maximum rounds.
func (a *SemanticArbiter) MaxRounds() int {
	return a.config.MaxRounds
}

// AbortThreshold returns the configured abort threshold.
func (a *SemanticArbiter) AbortThreshold() float64 {
	return a.config.AbortThreshold
}

// StagnationThreshold returns the configured stagnation threshold.
func (a *SemanticArbiter) StagnationThreshold() float64 {
	return a.config.StagnationThreshold
}

// IsEnabled returns whether the arbiter is enabled.
func (a *SemanticArbiter) IsEnabled() bool {
	return a.config.Enabled
}

// GetConfig returns the arbiter configuration.
func (a *SemanticArbiter) GetConfig() ArbiterConfig {
	return a.config
}
