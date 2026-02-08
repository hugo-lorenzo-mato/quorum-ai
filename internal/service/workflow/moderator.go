package workflow

import (
	"context"
	"fmt"
	"os"
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
	// ScoreFound indicates if a score was successfully extracted.
	// If false, Score will be 0 but it means "not found" rather than "zero".
	ScoreFound bool
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
		},
		nil
}

// Evaluate runs semantic consensus evaluation on the given analyses.
// Uses the configured moderator agent.
func (m *SemanticModerator) Evaluate(ctx context.Context, wctx *Context, round int, outputs []AnalysisOutput) (*ModeratorEvaluationResult, error) {
	return m.EvaluateWithAgent(ctx, wctx, round, 1, outputs, m.config.Agent)
}

// EvaluateWithAgent runs semantic consensus evaluation using a specific agent.
// This allows fallback to alternative agents if the primary moderator fails.
// The attempt parameter tracks which attempt this is (1-based) for file naming and traceability.
func (m *SemanticModerator) EvaluateWithAgent(ctx context.Context, wctx *Context, round, attempt int, outputs []AnalysisOutput, agentName string) (*ModeratorEvaluationResult, error) {
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

	// Build analysis file paths for the moderator to read
	// The moderator will use its file reading tools to access the full analysis content
	analyses := make([]ModeratorAnalysisSummary, len(outputs))
	for i, out := range outputs {
		// Extract the base agent name (remove vN- prefix if present)
		baseAgentName := out.AgentName
		if strings.HasPrefix(out.AgentName, "v") && len(out.AgentName) > 3 {
			// Handle formats like "v2-claude" -> "claude"
			if idx := strings.Index(out.AgentName, "-"); idx > 0 && idx < 4 {
				baseAgentName = out.AgentName[idx+1:]
			}
		}

		// Get the file path for this analysis
		var filePath string
		if wctx.Report != nil && wctx.Report.IsEnabled() {
			if round == 1 {
				filePath = wctx.Report.V1AnalysisPath(baseAgentName, out.Model)
			} else {
				filePath = wctx.Report.VnAnalysisPath(baseAgentName, out.Model, round)
			}
		}

		analyses[i] = ModeratorAnalysisSummary{
			AgentName: out.AgentName,
			FilePath:  filePath,
		}

		wctx.Logger.Debug("moderator will read analysis file",
			"agent", out.AgentName,
			"file_path", filePath,
		)
	}

	// Get output file path for LLM to write directly
	// Each attempt writes to its own file for traceability
	var outputFilePath string
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		outputFilePath = wctx.Report.ModeratorAttemptPath(round, attempt, moderatorAgentName)
	}
	absOutputPath := wctx.ResolveFilePath(outputFilePath)

	// Ensure output directory exists before execution (file enforcement)
	if absOutputPath != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		if err := enforcement.EnsureDirectory(absOutputPath); err != nil {
			wctx.Logger.Warn("failed to ensure moderator output directory", "path", absOutputPath, "error", err)
		}
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

	// Launch output file watchdog for recovery/reaping if agent hangs after writing
	var watchdog *OutputWatchdog
	if absOutputPath != "" {
		watchdog = NewOutputWatchdog(absOutputPath, DefaultWatchdogConfig(), wctx.Logger)
		watchdog.Start()
		defer watchdog.Stop()
	}

	// Execute moderator evaluation
	var result *core.ExecuteResult
	err = wctx.Retry.Execute(func() error {
		if ctrlErr := wctx.CheckControl(ctx); ctrlErr != nil {
			return ctrlErr
		}
		// Pre-retry: if output file already exists with substantial content, use it.
		// This recovers from the case where the agent wrote output but cmd.Wait() failed.
		if absOutputPath != "" {
			if info, statErr := os.Stat(absOutputPath); statErr == nil && info.Size() > 1024 {
				content, readErr := os.ReadFile(absOutputPath)
				if readErr == nil {
					wctx.Logger.Info("recovered moderator output from file written by previous attempt",
						"agent", moderatorAgentName, "round", round, "path", absOutputPath, "size", len(content))
					result = &core.ExecuteResult{Output: string(content), Model: model}
					return nil
				}
			}
		}

		attemptCtx, cancelAttempt := context.WithCancelCause(ctx)
		defer cancelAttempt(nil)

		stableOutputCh := make(chan string, 1)
		if watchdog != nil {
			go func() {
				select {
				case content := <-watchdog.StableCh():
					select {
					case stableOutputCh <- content:
					default:
					}
					cancelAttempt(core.ErrExecution(watchdogStableOutputCode, "output file stabilized; reaping hung agent process"))
				case <-attemptCtx.Done():
				}
			}()
		}

		var execErr error
		result, execErr = agent.Execute(attemptCtx, core.ExecuteOptions{
			Prompt:  prompt,
			Format:  core.OutputFormatText,
			Model:   model,
			Timeout: wctx.Config.PhaseTimeouts.Analyze,
			Phase:   core.PhaseAnalyze,
			WorkDir: wctx.ProjectRoot,
		})
		// If execution was cancelled after the output file stabilized, treat it as success.
		if execErr != nil {
			select {
			case content := <-stableOutputCh:
				wctx.Logger.Info("watchdog reap: using stable moderator output file",
					"agent", moderatorAgentName, "round", round, "path", absOutputPath, "size", len(content))
				result = &core.ExecuteResult{Output: content, Model: model}
				return nil
			default:
			}
		}
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

	// Some CLIs write to file but return empty stdout. Prefer the file content in that case.
	if absOutputPath != "" && (result == nil || strings.TrimSpace(result.Output) == "") {
		if content, readErr := os.ReadFile(absOutputPath); readErr == nil && strings.TrimSpace(string(content)) != "" {
			if result == nil {
				result = &core.ExecuteResult{}
			}
			result.Output = string(content)
			if result.Model == "" {
				result.Model = model
			}
			wctx.Logger.Info("file enforcement: using moderator output file content (stdout empty)",
				"agent", moderatorAgentName, "round", round, "path", absOutputPath, "size", len(content))
		}
	}

	// Ensure output file exists (file enforcement fallback)
	if absOutputPath != "" && result != nil && result.Output != "" {
		enforcement := NewFileEnforcement(wctx.Logger)
		createdByLLM, verifyErr := enforcement.VerifyOrWriteFallback(absOutputPath, result.Output)
		if verifyErr != nil {
			wctx.Logger.Warn("file enforcement failed", "path", absOutputPath, "error", verifyErr)
		} else if !createdByLLM {
			wctx.Logger.Debug("created fallback file from stdout", "path", absOutputPath)
		}
	}

	// Parse the moderator response
	evalResult := m.parseModeratorResponse(result.Output)
	evalResult.TokensIn = result.TokensIn
	evalResult.TokensOut = result.TokensOut
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
			// Use "error" instead of "warning" so frontend stops the timer for this agent
			wctx.Output.AgentEvent("error", moderatorAgentName,
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
			"duration_ms":     durationMS,
		})
	}

	// Promote successful attempt to official location
	// This ensures round-X.md only exists when validation passes
	if wctx.Report != nil && wctx.Report.IsEnabled() {
		if promoteErr := wctx.Report.PromoteModeratorAttempt(round, attempt, moderatorAgentName); promoteErr != nil {
			wctx.Logger.Warn("failed to promote moderator attempt",
				"round", round,
				"attempt", attempt,
				"agent", moderatorAgentName,
				"error", promoteErr,
			)
		}
	}

	// Write moderator report (kept for backward compatibility with metadata)
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
			DurationMS:       durationMS,
		}); reportErr != nil {
			wctx.Logger.Warn("failed to write moderator report", "round", round, "error", reportErr)
		}
	}

	return evalResult, nil
}

// moderatorFrontmatter represents the YAML frontmatter structure for moderator output.
type moderatorFrontmatter struct {
	// ConsensusScore uses interface{} to handle int, float, or string (robustness)
	ConsensusScore          interface{} `yaml:"consensus_score"`
	HighImpactDivergences   int         `yaml:"high_impact_divergences"`
	MediumImpactDivergences int         `yaml:"medium_impact_divergences"`
	LowImpactDivergences    int         `yaml:"low_impact_divergences"`
	AgreementsCount         int         `yaml:"agreements_count"`
}

// parseModeratorResponse parses the moderator's response to extract the consensus score and details.
//
//nolint:gocyclo // Parsing logic is intentionally explicit for robustness.
func (m *SemanticModerator) parseModeratorResponse(output string) *ModeratorEvaluationResult {
	result := &ModeratorEvaluationResult{
		RawOutput:  output,
		Score:      0,
		ScoreFound: false,
	}

	// 1. Sanitization: Detect and remove markdown code blocks wrapping YAML
	cleanOutput := sanitizeRawOutput(output)

	// 2. Primary method: Robust YAML Frontmatter Extraction
	// Looks for the first valid-looking frontmatter block anywhere in text
	if frontmatter, body, ok := parseYAMLFrontmatterRobust(cleanOutput); ok {
		var fm moderatorFrontmatter
		if err := yaml.Unmarshal([]byte(frontmatter), &fm); err == nil {
			// Extract robust score from interface{}
			score, found := extractScoreFromInterface(fm.ConsensusScore)
			if found {
				result.Score = score
				result.ScoreFound = true
				// Use body (content after frontmatter) for section extraction
				output = body
			}
		}
	}

	// 3. Backup Method: Double Anchoring (End of file)
	// Looks for ">> FINAL SCORE: XX <<" pattern mandated in prompt
	if !result.ScoreFound {
		anchorPattern := regexp.MustCompile(`>>\s*FINAL\s*SCORE\s*:\s*(\d+)\s*<<`)
		match := anchorPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				result.Score = score / 100.0
				result.ScoreFound = true
			}
		}
	}

	// 4. Fallback patterns (relaxed regexes)
	if !result.ScoreFound {
		// Pattern 1: Flexible key-value with optional percent
		// Matches: "Consensus Score: 80", "Semantic Score = 85%", "Score: 90/100"
		scorePattern := regexp.MustCompile(`(?im)(?:consensus|semantic|overall)\s*_?\s*score\s*[:=]\s*(\d+(?:\.\d+)?)(?:\s*%|\s*/\s*100)?`)
		match := scorePattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				// Normalize: if > 1.0, assume percentage (0-100), otherwise assume ratio (0.0-1.0)
				if score > 1.0 {
					result.Score = score / 100.0
				} else {
					result.Score = score
				}
				result.ScoreFound = true
			}
		}
	}

	// Pattern 2: Look for score in markdown code blocks or bold format
	// Matches: `CONSENSUS_SCORE: 78%`, **CONSENSUS_SCORE:** 78%
	if !result.ScoreFound {
		codeBlockPattern := regexp.MustCompile(`(?im)(?:\*\*)?CONSENSUS[_\s]?SCORE(?:\*\*)?[:\s*]+(\d+)(?:\s*%)?`)
		match := codeBlockPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				result.Score = score / 100.0
				result.ScoreFound = true
			}
		}
	}

	// Pattern 3: Deep reasoning pattern (for extended thinking models)
	// Matches: =-=-=-=75%=-=-=-=-=
	if !result.ScoreFound {
		deepPattern := regexp.MustCompile(`=-=-=-=(\d+)%?=-=-=-=-=`)
		match := deepPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				result.Score = score / 100.0
				result.ScoreFound = true
			}
		}
	}

	// Pattern 4: "overall/semantic/consensus score is XX%" with 'is' keyword
	if !result.ScoreFound {
		isPattern := regexp.MustCompile(`(?i)(?:overall|semantic|consensus)\s+score\s+is\s+(\d+(?:\.\d+)?)\s*%?`)
		match := isPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				if score > 1.0 {
					result.Score = score / 100.0
				} else {
					result.Score = score
				}
				result.ScoreFound = true
			}
		}
	}

	// Pattern 5: Decimal format "semantic score is 0.XX"
	if !result.ScoreFound {
		decimalPattern := regexp.MustCompile(`(?i)(?:semantic|consensus|overall)\s+score\s+(?:is\s+)?(0\.\d+)`)
		match := decimalPattern.FindStringSubmatch(output)
		if match != nil && len(match) > 1 {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil && score <= 1.0 {
				result.Score = score
				result.ScoreFound = true
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

// sanitizeRawOutput removes markdown code blocks that might wrap YAML.
func sanitizeRawOutput(text string) string {
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip opening/closing code blocks
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	return strings.Join(cleanLines, "\n")
}

// parseYAMLFrontmatterRobust searches for YAML-like blocks anywhere in text.
// It is more aggressive than the standard parser.
func parseYAMLFrontmatterRobust(text string) (frontmatter, body string, ok bool) {
	// Look for block delimited by ---
	// Use regex to find the first block that looks like frontmatter
	// (?s) enables dot matching newlines
	re := regexp.MustCompile(`(?s)(?:^|\n)---\s*\n(.*?)\n---\s*(?:\n|$)`)
	match := re.FindStringSubmatchIndex(text)

	if match == nil {
		return "", text, false
	}

	// Extract content between delimiters
	start, end := match[2], match[3]
	frontmatter = text[start:end]

	// Body is everything after the closing ---
	fullMatchEnd := match[1]
	if fullMatchEnd < len(text) {
		body = text[fullMatchEnd:]
	} else {
		body = ""
	}

	return frontmatter, body, true
}

// extractScoreFromInterface handles robust type conversion for score.
func extractScoreFromInterface(val interface{}) (float64, bool) {
	if val == nil {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return float64(v) / 100.0, true
	case float64:
		// If small (<=1), assume ratio. If large (>1), assume percentage.
		if v <= 1.0 {
			return v, true
		}
		return v / 100.0, true
	case string:
		// Clean string: remove %, spaces, /100
		cleaned := strings.ReplaceAll(v, "%", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		cleaned = strings.TrimSuffix(cleaned, "/100")

		if s, err := strconv.ParseFloat(cleaned, 64); err == nil {
			if s <= 1.0 {
				return s, true
			}
			return s / 100.0, true
		}
	}
	return 0, false
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

// taskTypeKeywords maps task types to their keyword patterns for adaptive threshold detection.
// Order matters for deterministic tie-breaking (analysis > design > bugfix > refactor).
var taskTypeKeywords = []struct {
	name     string
	keywords []string
}{
	{name: "analysis", keywords: []string{"analizar", "analyze", "investigar", "investigate", "evaluar", "evaluate", "viabilidad", "feasibility", "assessment", "review"}},
	{name: "design", keywords: []string{"diseñar", "design", "arquitectura", "architecture", "implementar", "implement", "crear", "create", "build"}},
	{name: "bugfix", keywords: []string{"fix", "bug", "error", "corregir", "repair", "solve", "debug", "issue", "problem"}},
	{name: "refactor", keywords: []string{"refactorizar", "refactor", "mejorar", "improve", "optimizar", "optimize", "clean", "reorganize", "restructure"}},
}

// detectTaskType analyzes the prompt to determine the task type for adaptive thresholds.
func detectTaskType(prompt string) string {
	promptLower := strings.ToLower(prompt)

	// Count keyword matches for each type
	maxMatches := 0
	detectedType := "default"

	for _, entry := range taskTypeKeywords {
		matches := 0
		for _, keyword := range entry.keywords {
			if strings.Contains(promptLower, keyword) {
				matches++
			}
		}
		if matches > maxMatches {
			maxMatches = matches
			detectedType = entry.name
		}
	}

	return detectedType
}

// EffectiveThreshold returns the appropriate threshold for the given prompt.
// If adaptive thresholds are configured and the prompt matches a task type,
// the type-specific threshold is returned. Otherwise, the default threshold is used.
func (m *SemanticModerator) EffectiveThreshold(prompt string) float64 {
	// Check if adaptive thresholds are configured
	if m.config.Thresholds == nil || len(m.config.Thresholds) == 0 {
		return m.config.Threshold // Default threshold
	}

	taskType := detectTaskType(prompt)
	if threshold, ok := m.config.Thresholds[taskType]; ok {
		return threshold
	}

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

// WarningThreshold returns the configured warning threshold.
func (m *SemanticModerator) WarningThreshold() float64 {
	return m.config.WarningThreshold
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

	// Check 2: Score must be found
	if !result.ScoreFound {
		// Detect refusal keywords to provide better error context
		refusalKeywords := []string{"cannot evaluate", "refuse to score", "unable to assess", "insufficient information"}
		rawLower := strings.ToLower(rawOutput)
		for _, kw := range refusalKeywords {
			if strings.Contains(rawLower, kw) {
				return fmt.Errorf("moderator refused to score: detected phrase %q", kw)
			}
		}
		return fmt.Errorf("no numeric consensus score found in output (checked YAML, anchors, and patterns)")
	}

	// Check 3: Output should contain expected sections for a valid evaluation
	hasContent := strings.Contains(rawOutput, "##") || // Markdown headers
		strings.Contains(rawOutput, "Agreement") ||
		strings.Contains(rawOutput, "Divergen")

	if !hasContent {
		return fmt.Errorf("output lacks expected structure (no content sections found)")
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
