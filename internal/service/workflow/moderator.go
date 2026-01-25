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
	"gopkg.in/yaml.v3"
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
// Uses the configured moderator agent.
func (m *SemanticModerator) Evaluate(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput) (*ModeratorEvaluationResult, error) {
	return m.EvaluateWithAgent(ctx, wctx, round, outputs, m.config.Agent)
}

// EvaluateWithAgent runs semantic consensus evaluation using a specific agent.
// This allows fallback to alternative agents if the primary moderator fails.
func (m *SemanticModerator) EvaluateWithAgent(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput, agentName string) (*ModeratorEvaluationResult, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("semantic moderator is not enabled. "+
			"Set 'phases.analyze.moderator.enabled: true' in your config. See: %s#phases-settings", DocsConfigURL)
	}

	// Use specified agent (allows fallback to alternatives)
	moderatorAgentName := agentName
	if moderatorAgentName == "" {
		moderatorAgentName = m.config.Agent
	}
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

	// Validate moderator output - detect garbage/empty responses
	validationErr := m.validateModeratorOutput(evalResult, result.Output)
	if validationErr != nil {
		wctx.Logger.Warn("moderator output validation failed",
			"round", round,
			"agent", moderatorAgentName,
			"model", model,
			"error", validationErr,
			"tokens_in", result.TokensIn,
			"tokens_out", result.TokensOut,
			"output_len", len(result.Output),
			"raw_output_sample", truncateForLog(result.Output, 500),
		)
		if wctx.Output != nil {
			wctx.Output.AgentEvent("warning", moderatorAgentName,
				fmt.Sprintf("Moderator output validation issue: %v", validationErr),
				map[string]interface{}{
					"phase":       "moderator",
					"round":       round,
					"model":       model,
					"tokens_in":   result.TokensIn,
					"tokens_out":  result.TokensOut,
					"output_len":  len(result.Output),
					"validation":  validationErr.Error(),
					"duration_ms": durationMS,
				})
		}
		// Return the validation error so caller can retry with fallback
		return nil, fmt.Errorf("moderator output validation: %w", validationErr)
	}

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

// moderatorFrontmatter represents the YAML frontmatter structure for moderator output.
type moderatorFrontmatter struct {
	ConsensusScore          int `yaml:"consensus_score"`
	HighImpactDivergences   int `yaml:"high_impact_divergences"`
	MediumImpactDivergences int `yaml:"medium_impact_divergences"`
	LowImpactDivergences    int `yaml:"low_impact_divergences"`
	AgreementsCount         int `yaml:"agreements_count"`
}

// parseModeratorResponse parses the moderator's response to extract the consensus score and details.
func (m *SemanticModerator) parseModeratorResponse(output string) *ModeratorEvaluationResult {
	result := &ModeratorEvaluationResult{
		RawOutput: output,
		Score:     0,
	}

	// Primary method: Parse YAML frontmatter
	// Format: ---\nconsensus_score: XX\n...\n---
	if frontmatter, body, ok := parseYAMLFrontmatter(output); ok {
		var fm moderatorFrontmatter
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err == nil && fm.ConsensusScore > 0 {
			result.Score = float64(fm.ConsensusScore) / 100.0
			// Use body (content after frontmatter) for section extraction
			output = body
		}
	}

	// Fallback patterns for backwards compatibility with older format
	if result.Score == 0 {
		// Pattern 1: CONSENSUS_SCORE: XX% (primary, with flexible whitespace and optional markdown)
		// Handles: "CONSENSUS_SCORE: 78%", "**CONSENSUS_SCORE:** 78%", "CONSENSUS_SCORE:78%"
		scorePattern := regexp.MustCompile(`(?im)^\**CONSENSUS[_\s]?SCORE[*:]*\s*(\d+)\s*%`)
		match := scorePattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.Atoi(match[1]); err == nil {
				result.Score = float64(score) / 100.0
			}
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

// parseYAMLFrontmatter extracts YAML frontmatter from a markdown document.
// Returns the frontmatter content, the body (content after frontmatter), and whether parsing succeeded.
// Frontmatter must start with "---" on the first line and end with "---" on its own line.
func parseYAMLFrontmatter(text string) (frontmatter, body string, ok bool) {
	text = strings.TrimSpace(text)

	// Must start with ---
	if !strings.HasPrefix(text, "---") {
		return "", text, false
	}

	// Find the closing ---
	// Skip the first line (opening ---)
	afterOpen := text[3:]
	if afterOpen != "" && afterOpen[0] == '\n' {
		afterOpen = afterOpen[1:]
	} else if len(afterOpen) > 1 && afterOpen[0] == '\r' && afterOpen[1] == '\n' {
		afterOpen = afterOpen[2:]
	} else {
		return "", text, false
	}

	// Find closing ---
	closeIdx := strings.Index(afterOpen, "\n---")
	if closeIdx == -1 {
		// Try with \r\n
		closeIdx = strings.Index(afterOpen, "\r\n---")
		if closeIdx == -1 {
			return "", text, false
		}
	}

	frontmatter = strings.TrimSpace(afterOpen[:closeIdx])

	// Body starts after the closing ---
	remaining := afterOpen[closeIdx+4:] // skip \n---
	if remaining != "" && remaining[0] == '\n' {
		remaining = remaining[1:]
	} else if len(remaining) > 1 && remaining[0] == '\r' && remaining[1] == '\n' {
		remaining = remaining[2:]
	}
	body = remaining

	return frontmatter, body, true
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

// validateModeratorOutput checks if the moderator output is valid and usable.
// Returns an error describing the validation failure if the output is invalid.
func (m *SemanticModerator) validateModeratorOutput(result *ModeratorEvaluationResult, rawOutput string) error {
	// Check 1: Output must not be empty
	if len(rawOutput) < 100 {
		return fmt.Errorf("output too short (%d chars, minimum 100)", len(rawOutput))
	}

	// Check 2: Score must be parseable (non-zero unless explicitly set)
	// A score of exactly 0.0 with no agreements/divergences is suspicious
	if result.Score == 0.0 && len(result.Agreements) == 0 && len(result.Divergences) == 0 {
		// Check if the output actually contains a score pattern
		if !strings.Contains(strings.ToUpper(rawOutput), "CONSENSUS") {
			return fmt.Errorf("no consensus score found in output")
		}
		// If it contains CONSENSUS but score is 0, the parsing might have failed
		if !strings.Contains(rawOutput, "0%") && !strings.Contains(rawOutput, ": 0") {
			return fmt.Errorf("consensus score parsing failed (score=0 but no '0%%' in output)")
		}
	}

	// Check 3: Token count sanity check
	// If reported tokens are suspiciously low but output is long, it's suspicious but not a failure
	// (the LLM might have written to file, or token counting may be inaccurate)
	// Note: This is informational only - token discrepancy detection handles correction

	// Check 4: Output should contain expected sections for a valid evaluation
	hasScore := strings.Contains(strings.ToUpper(rawOutput), "CONSENSUS") ||
		strings.Contains(strings.ToUpper(rawOutput), "SCORE")
	hasContent := strings.Contains(rawOutput, "##") || // Markdown headers
		strings.Contains(rawOutput, "Agreement") ||
		strings.Contains(rawOutput, "Divergen")

	if !hasScore && !hasContent {
		return fmt.Errorf("output lacks expected structure (no score or content sections)")
	}

	return nil
}

// NOTE: truncateForLog is defined in refiner.go (same package)

// ModeratorValidationError represents a validation failure for moderator output.
type ModeratorValidationError struct {
	Reason    string
	TokensIn  int
	TokensOut int
	OutputLen int
}

func (e *ModeratorValidationError) Error() string {
	return fmt.Sprintf("moderator validation failed: %s (tokens_in=%d, tokens_out=%d, output_len=%d)",
		e.Reason, e.TokensIn, e.TokensOut, e.OutputLen)
}

// IsModeratorValidationError checks if an error is a moderator validation error.
func IsModeratorValidationError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "moderator output validation") ||
		strings.Contains(err.Error(), "moderator validation failed")
}
