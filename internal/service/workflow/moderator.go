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

// ModeratorEvaluationResult contains the result of a semantic moderator evaluation.
type ModeratorEvaluationResult struct {
	// Score is the semantic consensus score (0.0-1.0).
	Score float64
	// RawOutput is the full moderator response.
	RawOutput string
	// Agreements are points where all analyses converge.
	Agreements []string
	// Divergences are points where analyses differ.
	Divergences []ModeratorDivergence
	// MissingPerspectives are gaps identified by the moderator.
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

// ModeratorDivergence represents a divergence identified by the moderator.
type ModeratorDivergence struct {
	Description    string
	AgentPositions map[string]string
	Impact         string
}

// SemanticModerator evaluates semantic consensus between agent analyses using an LLM.
type SemanticModerator struct {
	config ModeratorConfig
}

// NewSemanticModerator creates a new semantic moderator.
// Configuration defaults are set in internal/config/loader.go (setDefaults).
// Agent validation is performed by config.Validate() at startup.
func NewSemanticModerator(config ModeratorConfig) (*SemanticModerator, error) {
	// Validate agent is set when enabled (model is resolved from agent config)
	if config.Enabled && config.Agent == "" {
		return nil, fmt.Errorf("missing moderator agent: set 'phases.analyze.moderator.agent' to one of your configured agents (e.g., 'claude', 'gemini'). "+
			"See: %s#phases-settings", DocsConfigURL)
	}

	// Ensure min_rounds <= max_rounds (validation only, not default setting)
	if config.MinRounds > config.MaxRounds && config.MaxRounds > 0 {
		config.MinRounds = config.MaxRounds
	}

	return &SemanticModerator{
		config: config,
	}, nil
}

// Evaluate runs semantic consensus evaluation on the given analyses.
func (m *SemanticModerator) Evaluate(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput) (*ModeratorEvaluationResult, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("semantic moderator is not enabled. "+
			"Set 'phases.analyze.moderator.enabled: true' in your config. See: %s#phases-settings", DocsConfigURL)
	}

	// Get moderator agent
	moderatorAgentName := m.config.Agent
	agent, err := wctx.Agents.Get(moderatorAgentName)
	if err != nil {
		return nil, fmt.Errorf("moderator agent '%s' not available: %w. "+
			"Ensure this agent is configured and responding (run 'quorum doctor' to verify). See: %s#agents", moderatorAgentName, err, DocsConfigURL)
	}

	// Acquire rate limit
	limiter := wctx.RateLimits.Get(moderatorAgentName)
	if err := limiter.Acquire(); err != nil {
		return nil, fmt.Errorf("rate limit for moderator: %w", err)
	}

	// Build analysis summaries for the prompt
	// Strategy: Use structured summaries (claims, risks, recommendations) when available
	// to reduce context usage while preserving key information.
	// Fall back to full output with generous limit (80k chars) as safety net.
	const maxCharsPerAnalysis = 80000 // Generous limit for safety
	analyses := make([]ModeratorAnalysisSummary, len(outputs))
	for i, out := range outputs {
		// Try to build a structured summary from extracted data
		output := buildAnalysisSummary(out, maxCharsPerAnalysis)
		if len(output) != len(out.RawOutput) {
			wctx.Logger.Debug("using structured summary for moderator",
				"agent", out.AgentName,
				"original_len", len(out.RawOutput),
				"summary_len", len(output),
			)
		}
		analyses[i] = ModeratorAnalysisSummary{
			AgentName: out.AgentName,
			Output:    output,
		}
	}

	// Get output file path for LLM to write directly
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.ModeratorReportPath(round)
	}

	// Render moderator prompt
	prompt, err := wctx.Prompts.RenderModeratorEvaluate(ModeratorEvaluateParams{
		Prompt:         GetEffectivePrompt(wctx.State),
		Round:          round,
		NextRound:      round + 1,
		Analyses:       analyses,
		BelowThreshold: true, // Always request recommendations
		OutputFilePath: outputFilePath,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering moderator prompt: %w", err)
	}

	// Resolve model from agent's phase_models.analyze or default model
	model := ResolvePhaseModel(wctx.Config, moderatorAgentName, core.PhaseAnalyze, "")

	startTime := time.Now()

	// Emit started event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("started", moderatorAgentName, fmt.Sprintf("Running semantic moderator evaluation (round %d)", round), map[string]interface{}{
			"phase":           "moderator",
			"round":           round,
			"model":           model,
			"analyses_count":  len(outputs),
			"timeout_seconds": int(wctx.Config.PhaseTimeouts.Analyze.Seconds()),
		})
	}

	// Execute moderator evaluation
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
			wctx.Output.AgentEvent("error", moderatorAgentName, err.Error(), map[string]interface{}{
				"phase":       "moderator",
				"round":       round,
				"model":       model,
				"duration_ms": time.Since(startTime).Milliseconds(),
				"error_type":  fmt.Sprintf("%T", err),
			})
		}
		return nil, err
	}

	durationMS := time.Since(startTime).Milliseconds()

	// Parse the moderator response
	evalResult := m.parseModeratorResponse(result.Output)
	evalResult.TokensIn = result.TokensIn
	evalResult.TokensOut = result.TokensOut
	evalResult.CostUSD = result.CostUSD
	evalResult.DurationMS = durationMS

	// Emit completed event
	if wctx.Output != nil {
		wctx.Output.AgentEvent("completed", moderatorAgentName, fmt.Sprintf("Moderator evaluation completed: %.0f%% consensus", evalResult.Score*100), map[string]interface{}{
			"phase":           "moderator",
			"round":           round,
			"model":           result.Model,
			"consensus_score": evalResult.Score,
			"tokens_in":       result.TokensIn,
			"tokens_out":      result.TokensOut,
			"cost_usd":        result.CostUSD,
			"duration_ms":     durationMS,
		})
	}

	// Write moderator report
	if wctx.Report != nil {
		if reportErr := wctx.Report.WriteModeratorReport(report.ModeratorData{
			Agent:            moderatorAgentName,
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
			wctx.Logger.Warn("failed to write moderator report", "round", round, "error", reportErr)
		}
	}

	return evalResult, nil
}

// parseModeratorResponse parses the moderator's response to extract the consensus score and details.
func (m *SemanticModerator) parseModeratorResponse(output string) *ModeratorEvaluationResult {
	result := &ModeratorEvaluationResult{
		RawOutput: output,
		Score:     0,
	}

	// Pattern 1: CONSENSUS_SCORE: XX% (primary, with flexible whitespace and optional markdown)
	// Handles: "CONSENSUS_SCORE: 78%", "**CONSENSUS_SCORE:** 78%", "CONSENSUS_SCORE:78%"
	// The pattern handles markdown bold (**), underscores/spaces between words, and colons in any position
	scorePattern := regexp.MustCompile(`(?im)^\**CONSENSUS[_\s]?SCORE[*:]*\s*(\d+)\s*%`)
	match := scorePattern.FindStringSubmatch(output)
	if match != nil && len(match) > 1 {
		if score, err := strconv.Atoi(match[1]); err == nil {
			result.Score = float64(score) / 100.0
		}
	}

	// Pattern 2: Look for score in markdown code blocks (model might wrap in ```)
	if result.Score == 0 {
		codeBlockPattern := regexp.MustCompile("(?m)`?CONSENSUS[_\\s]?SCORE:?\\s*(\\d+)\\s*%`?")
		match := codeBlockPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.Atoi(match[1]); err == nil {
				result.Score = float64(score) / 100.0
			}
		}
	}

	// Pattern 3: =-=-=-=XX%=-=-=-=-= (fallback for deep reasoning models)
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

	// Pattern 4: "consensus score" or "overall score" followed by percentage
	// Handles: "consensus score: 78%", "overall consensus score of 78%", "score is 78%"
	if result.Score == 0 {
		genericScorePattern := regexp.MustCompile(`(?i)(?:consensus|overall|semantic)\s+score[:\s]+(?:is\s+)?(?:of\s+)?(\d+)\s*%`)
		match := genericScorePattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.Atoi(match[1]); err == nil {
				result.Score = float64(score) / 100.0
			}
		}
	}

	// Pattern 5: Score as decimal (0.XX) - some models output 0.78 instead of 78%
	if result.Score == 0 {
		decimalPattern := regexp.MustCompile(`(?i)(?:consensus|overall|semantic)\s+score[:\s]+(?:is\s+)?(?:of\s+)?(0\.\d+)`)
		match := decimalPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil && score <= 1.0 {
				result.Score = score
			}
		}
	}

	// Extract agreements
	result.Agreements = extractModeratorSection(output, "Agreements")

	// Extract divergences (simplified)
	divergenceTexts := extractModeratorSection(output, "Divergences")
	for _, text := range divergenceTexts {
		result.Divergences = append(result.Divergences, ModeratorDivergence{
			Description: text,
		})
	}

	// Extract missing perspectives
	result.MissingPerspectives = extractModeratorSection(output, "Missing Perspectives")

	// Extract recommendations
	result.Recommendations = extractModeratorSection(output, "Recommendations for Next Round")

	return result
}

// extractModeratorSection extracts bullet points from a named section in the moderator output.
func extractModeratorSection(text, sectionName string) []string {
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
func (m *SemanticModerator) Threshold() float64 {
	return m.config.Threshold
}

// MinRounds returns the configured minimum rounds before accepting consensus.
func (m *SemanticModerator) MinRounds() int {
	return m.config.MinRounds
}

// MaxRounds returns the configured maximum rounds.
func (m *SemanticModerator) MaxRounds() int {
	return m.config.MaxRounds
}

// AbortThreshold returns the configured abort threshold.
func (m *SemanticModerator) AbortThreshold() float64 {
	return m.config.AbortThreshold
}

// StagnationThreshold returns the configured stagnation threshold.
func (m *SemanticModerator) StagnationThreshold() float64 {
	return m.config.StagnationThreshold
}

// IsEnabled returns whether the moderator is enabled.
func (m *SemanticModerator) IsEnabled() bool {
	return m.config.Enabled
}

// GetConfig returns the moderator configuration.
func (m *SemanticModerator) GetConfig() ModeratorConfig {
	return m.config
}

// buildAnalysisSummary creates a context-efficient summary from an analysis output.
// Strategy:
// 1. If output is under maxChars, return it unchanged
// 2. If structured data (claims/risks/recommendations) is available, build a focused summary
// 3. Otherwise, use the beginning of the output up to maxChars
func buildAnalysisSummary(out AnalysisOutput, maxChars int) string {
	// If under limit, use full output
	if len(out.RawOutput) <= maxChars {
		return out.RawOutput
	}

	// Check if we have structured data to summarize
	hasStructuredData := len(out.Claims) > 0 || len(out.Risks) > 0 || len(out.Recommendations) > 0
	if !hasStructuredData {
		// No structured data - use intelligent truncation of raw output
		return truncateText(out.RawOutput, maxChars)
	}

	// Build structured summary
	var sb strings.Builder
	sb.WriteString("## Summary of Key Points\n\n")

	// Include claims
	if len(out.Claims) > 0 {
		sb.WriteString("### Claims\n")
		for i, claim := range out.Claims {
			if i >= 20 { // Limit to top 20 claims
				sb.WriteString(fmt.Sprintf("- ... and %d more claims\n", len(out.Claims)-20))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", claim))
		}
		sb.WriteString("\n")
	}

	// Include risks
	if len(out.Risks) > 0 {
		sb.WriteString("### Risks\n")
		for i, risk := range out.Risks {
			if i >= 15 { // Limit to top 15 risks
				sb.WriteString(fmt.Sprintf("- ... and %d more risks\n", len(out.Risks)-15))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", risk))
		}
		sb.WriteString("\n")
	}

	// Include recommendations
	if len(out.Recommendations) > 0 {
		sb.WriteString("### Recommendations\n")
		for i, rec := range out.Recommendations {
			if i >= 15 { // Limit to top 15 recommendations
				sb.WriteString(fmt.Sprintf("- ... and %d more recommendations\n", len(out.Recommendations)-15))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		sb.WriteString("\n")
	}

	summary := sb.String()

	// If summary is still too large, use intelligent truncation
	if len(summary) > maxChars {
		return truncateText(summary, maxChars)
	}

	// Calculate remaining space for context from raw output
	remainingChars := maxChars - len(summary) - 200 // Reserve space for separator
	if remainingChars > 5000 {
		// Add beginning of raw output for additional context
		sb.WriteString("---\n\n## Additional Context (from full analysis)\n\n")
		context := out.RawOutput
		if len(context) > remainingChars {
			context = context[:remainingChars]
			// Try to cut at a paragraph boundary
			lastPara := strings.LastIndex(context, "\n\n")
			if lastPara > remainingChars/2 {
				context = context[:lastPara]
			}
			context += "\n\n...[see full report for complete analysis]..."
		}
		sb.WriteString(context)
	}

	return sb.String()
}

// truncateText intelligently truncates text to fit within maxChars.
// It tries to preserve complete sections rather than cutting mid-sentence.
func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	// Reserve space for notice
	const notice = "\n\n...[see full report for complete analysis]..."
	targetLen := maxChars - len(notice)
	if targetLen < 1000 {
		targetLen = 1000
	}

	text = text[:targetLen]

	// Find the last complete section (look for ## headers)
	lastSection := strings.LastIndex(text, "\n## ")
	if lastSection > targetLen/2 {
		text = text[:lastSection]
	} else {
		// Try to cut at a paragraph boundary
		lastPara := strings.LastIndex(text, "\n\n")
		if lastPara > targetLen*3/4 {
			text = text[:lastPara]
		}
	}

	return text + notice
}
