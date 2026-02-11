package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// =============================================================================
// Mock helpers specific to this file
// =============================================================================

// mockModeratorPromptRenderer extends mockPromptRenderer to allow moderator errors.
type mockModeratorPromptRenderer struct {
	mockPromptRenderer
	moderatorErr error
}

func (m *mockModeratorPromptRenderer) RenderModeratorEvaluate(_ ModeratorEvaluateParams) (string, error) {
	if m.moderatorErr != nil {
		return "", m.moderatorErr
	}
	return "moderator evaluate prompt", nil
}

// =============================================================================
// Tests for EvaluateWithAgent edge cases
// =============================================================================

func TestSemanticModerator_EvaluateWithAgent_NotEnabled(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = mod.EvaluateWithAgent(context.Background(), &Context{}, 1, 1, nil, "claude")
	if err == nil {
		t.Fatal("expected error when moderator not enabled")
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Errorf("expected 'not enabled' in error, got: %v", err)
	}
}

func TestSemanticModerator_EvaluateWithAgent_AgentNotFound(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry := &mockAgentRegistry{}
	// Don't register any agents

	wctx := &Context{
		Agents: registry,
		Logger: logging.NewNop(),
	}

	_, err = mod.EvaluateWithAgent(context.Background(), wctx, 1, 1, nil, "nonexistent")
	if err == nil {
		t.Fatal("expected error when agent not available")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("expected 'not available' in error, got: %v", err)
	}
}

func TestSemanticModerator_EvaluateWithAgent_EmptyAgentNameUsesConfig(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry := &mockAgentRegistry{}
	// Don't register "claude" so it returns an error
	// This tests that empty agentName falls back to config agent

	wctx := &Context{
		Agents: registry,
		Logger: logging.NewNop(),
	}

	_, err = mod.EvaluateWithAgent(context.Background(), wctx, 1, 1, nil, "")
	if err == nil {
		t.Fatal("expected error (agent not found)")
	}
	// The error message should reference "claude" (the config agent)
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("expected 'claude' in error, got: %v", err)
	}
}

func TestSemanticModerator_EvaluateWithAgent_RateLimitError(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := &mockAgent{name: "claude", result: &core.ExecuteResult{Output: "test"}}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		Agents: registry,
		Logger: logging.NewNop(),
		RateLimits: &mockRateLimiterGetter{
			limiter: &mockRateLimiter{acquireErr: errors.New("rate limit exceeded")},
		},
	}

	_, err = mod.EvaluateWithAgent(context.Background(), wctx, 1, 1, nil, "claude")
	if err == nil {
		t.Fatal("expected error from rate limiter")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected 'rate limit' in error, got: %v", err)
	}
}

func TestSemanticModerator_EvaluateWithAgent_PromptRenderError(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := &mockAgent{name: "claude", result: &core.ExecuteResult{Output: "test"}}
	registry := &mockAgentRegistry{}
	registry.Register("claude", agent)

	wctx := &Context{
		Agents: registry,
		Logger: logging.NewNop(),
		RateLimits: &mockRateLimiterGetter{
			limiter: &mockRateLimiter{},
		},
		Prompts: &mockModeratorPromptRenderer{
			moderatorErr: errors.New("prompt render failed"),
		},
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{Prompt: "test prompt"},
		},
		Config: &Config{
			PhaseTimeouts: PhaseTimeouts{},
		},
	}

	_, err = mod.EvaluateWithAgent(context.Background(), wctx, 1, 1, nil, "claude")
	if err == nil {
		t.Fatal("expected error from prompt render")
	}
	if !strings.Contains(err.Error(), "prompt render failed") {
		t.Errorf("expected 'prompt render failed' in error, got: %v", err)
	}
}

// =============================================================================
// Tests for buildAnalysisSummaries
// =============================================================================

func TestSemanticModerator_buildAnalysisSummaries_NoReport(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputs := []AnalysisOutput{
		{AgentName: "claude", Model: "opus", RawOutput: "analysis 1"},
		{AgentName: "gemini", Model: "pro", RawOutput: "analysis 2"},
	}

	wctx := &Context{
		Logger: logging.NewNop(),
		Report: nil, // No report writer
	}

	analyses := mod.buildAnalysisSummaries(wctx, outputs, 1)
	if len(analyses) != 2 {
		t.Fatalf("expected 2 analyses, got %d", len(analyses))
	}

	// Without report, file paths should be empty
	for _, a := range analyses {
		if a.FilePath != "" {
			t.Errorf("expected empty file path without report, got %q", a.FilePath)
		}
	}
}

func TestSemanticModerator_buildAnalysisSummaries_VnPrefix(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outputs := []AnalysisOutput{
		{AgentName: "v2-claude", Model: "opus", RawOutput: "refined analysis"},
		{AgentName: "v3-gemini", Model: "pro", RawOutput: "refined analysis 2"},
		{AgentName: "claude", Model: "opus", RawOutput: "original analysis"},
	}

	wctx := &Context{
		Logger: logging.NewNop(),
		Report: nil,
	}

	analyses := mod.buildAnalysisSummaries(wctx, outputs, 2)
	if len(analyses) != 3 {
		t.Fatalf("expected 3 analyses, got %d", len(analyses))
	}

	// Check that the AgentName is preserved as-is in the summary
	if analyses[0].AgentName != "v2-claude" {
		t.Errorf("expected 'v2-claude', got %q", analyses[0].AgentName)
	}
	if analyses[1].AgentName != "v3-gemini" {
		t.Errorf("expected 'v3-gemini', got %q", analyses[1].AgentName)
	}
	if analyses[2].AgentName != "claude" {
		t.Errorf("expected 'claude', got %q", analyses[2].AgentName)
	}
}

// =============================================================================
// Tests for prepareOutputPaths
// =============================================================================

func TestSemanticModerator_prepareOutputPaths_NoReport(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wctx := &Context{
		Logger: logging.NewNop(),
		Report: nil,
	}

	outputFile, absPath := mod.prepareOutputPaths(wctx, 1, 1, "claude")
	if outputFile != "" {
		t.Errorf("expected empty output file path, got %q", outputFile)
	}
	if absPath != "" {
		t.Errorf("expected empty abs path, got %q", absPath)
	}
}

// =============================================================================
// Tests for emitStartedEvent
// =============================================================================

func TestSemanticModerator_emitStartedEvent_NilOutput(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wctx := &Context{
		Output: nil, // nil output notifier
		Config: &Config{PhaseTimeouts: PhaseTimeouts{}},
	}

	// Should not panic
	mod.emitStartedEvent(wctx, "claude", 1, "opus", nil)
}

func TestSemanticModerator_emitStartedEvent_WithOutput(t *testing.T) {
	t.Parallel()

	mod, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
		Config: &Config{PhaseTimeouts: PhaseTimeouts{}},
	}

	outputs := []AnalysisOutput{
		{AgentName: "agent1"},
		{AgentName: "agent2"},
	}

	mod.emitStartedEvent(wctx, "claude", 1, "opus", outputs)
	// Should have emitted an agent event (check via mock tracking)
}

// =============================================================================
// Tests for emitErrorEvent
// =============================================================================

func TestSemanticModerator_emitErrorEvent_NilOutput(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Output: nil,
	}

	// Should not panic
	mod.emitErrorEvent(wctx, "claude", 1, "opus", time.Now(), errors.New("test error"))
}

func TestSemanticModerator_emitErrorEvent_WithOutput(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
	}

	mod.emitErrorEvent(wctx, "claude", 1, "opus", time.Now(), errors.New("test error"))
}

// =============================================================================
// Tests for emitCompletedEvent
// =============================================================================

func TestSemanticModerator_emitCompletedEvent_NilOutput(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Output: nil,
	}

	result := &core.ExecuteResult{Model: "opus", TokensIn: 100, TokensOut: 200}
	evalResult := &ModeratorEvaluationResult{Score: 0.85}

	// Should not panic
	mod.emitCompletedEvent(wctx, "claude", 1, result, evalResult, 1000)
}

func TestSemanticModerator_emitCompletedEvent_WithOutput(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
	}

	result := &core.ExecuteResult{Model: "opus", TokensIn: 100, TokensOut: 200}
	evalResult := &ModeratorEvaluationResult{Score: 0.85}

	mod.emitCompletedEvent(wctx, "claude", 1, result, evalResult, 1000)
}

// =============================================================================
// Tests for handleFileEnforcement edge cases
// =============================================================================

func TestSemanticModerator_handleFileEnforcement_EmptyStdoutFallbackToFile(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "moderator-output.md")

	validContent := "---\nconsensus_score: 80\n---\n\n## Agreements\n- All agree"
	if err := os.WriteFile(outputPath, []byte(validContent), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	wctx := &Context{
		Logger: logging.NewNop(),
	}

	// result with empty stdout
	result := &core.ExecuteResult{Output: "", Model: "opus"}

	got := mod.handleFileEnforcement(wctx, result, outputPath, "claude", 1, "opus")
	if got.Output != validContent {
		t.Errorf("expected file content as output, got %q", got.Output[:min(50, len(got.Output))])
	}
}

func TestSemanticModerator_handleFileEnforcement_NilResult(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "moderator-output.md")

	validContent := "---\nconsensus_score: 75\n---\n\n## Agreements\n- Agreement point"
	if err := os.WriteFile(outputPath, []byte(validContent), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	wctx := &Context{
		Logger: logging.NewNop(),
	}

	// nil result - should create a new ExecuteResult from file
	got := mod.handleFileEnforcement(wctx, nil, outputPath, "claude", 1, "opus")
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Output != validContent {
		t.Errorf("expected file content, got %q", got.Output[:min(50, len(got.Output))])
	}
	if got.Model != "opus" {
		t.Errorf("expected model 'opus', got %q", got.Model)
	}
}

func TestSemanticModerator_handleFileEnforcement_QualityGateReplacesStdout(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "moderator-output.md")

	// Valid structured content in file
	validContent := "---\nconsensus_score: 82\n---\n\n## Score Rationale\nGood consensus.\n\n## Agreements\n- Architecture agreed\n\n>> FINAL SCORE: 82 <<"
	if err := os.WriteFile(outputPath, []byte(validContent), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	wctx := &Context{
		Logger: logging.NewNop(),
	}

	// Conversational stdout that fails quality gate
	result := &core.ExecuteResult{
		Output: "I'll read the analysis files now and provide my evaluation.",
		Model:  "opus",
	}

	got := mod.handleFileEnforcement(wctx, result, outputPath, "claude", 1, "opus")
	if got.Output != validContent {
		t.Errorf("expected file content to replace conversational stdout")
	}
}

func TestSemanticModerator_handleFileEnforcement_NoFile(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Logger: logging.NewNop(),
	}

	// No output file path
	result := &core.ExecuteResult{
		Output: "some output",
		Model:  "opus",
	}

	got := mod.handleFileEnforcement(wctx, result, "", "claude", 1, "opus")
	if got.Output != "some output" {
		t.Errorf("expected original output when no file path, got %q", got.Output)
	}
}

func TestSemanticModerator_handleFileEnforcement_FileEnforcementFallback(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "moderator-output.md")
	// File does NOT exist initially

	wctx := &Context{
		Logger: logging.NewNop(),
	}

	validOutput := "---\nconsensus_score: 90\n---\n\n## Agreements\n- All agree"
	result := &core.ExecuteResult{
		Output: validOutput,
		Model:  "opus",
	}

	got := mod.handleFileEnforcement(wctx, result, outputPath, "claude", 1, "opus")
	if got.Output != validOutput {
		t.Errorf("expected original valid output preserved")
	}

	// The file enforcement should have created the file as fallback
	if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
		t.Error("expected file enforcement to create fallback file")
	}
}

// =============================================================================
// Tests for processModeratorResponse
// =============================================================================

func TestSemanticModerator_processModeratorResponse_ValidationFails(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Logger: logging.NewNop(),
		Output: &mockOutputNotifier{},
	}

	// Short output that fails validation
	result := &core.ExecuteResult{
		Output:    "Short",
		TokensIn:  100,
		TokensOut: 50,
		Model:     "opus",
	}

	_, err := mod.processModeratorResponse(wctx, result, "claude", 1, "opus", 500)
	if err == nil {
		t.Fatal("expected validation error for short output")
	}
	if !strings.Contains(err.Error(), "moderator output validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestSemanticModerator_processModeratorResponse_Success(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Logger: logging.NewNop(),
		Output: nil,
	}

	validOutput := "---\nconsensus_score: 85\nhigh_impact_divergences: 1\n---\n\n## Score Rationale\nGood overall agreement between all agents.\n\n## Agreements\n- Architecture aligned\n- Testing approach agreed\n\n## Divergences\n- Minor timing differences"
	result := &core.ExecuteResult{
		Output:    validOutput,
		TokensIn:  500,
		TokensOut: 800,
		Model:     "opus",
	}

	evalResult, err := mod.processModeratorResponse(wctx, result, "claude", 1, "opus", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evalResult == nil {
		t.Fatal("expected non-nil eval result")
	}
	if evalResult.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", evalResult.Score)
	}
	if evalResult.TokensIn != 500 {
		t.Errorf("expected TokensIn 500, got %d", evalResult.TokensIn)
	}
	if evalResult.DurationMS != 1000 {
		t.Errorf("expected DurationMS 1000, got %d", evalResult.DurationMS)
	}
}

func TestSemanticModerator_processModeratorResponse_NoScoreWithRefusal(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Logger: logging.NewNop(),
		Output: &mockOutputNotifier{},
	}

	// Output that looks like a refusal
	refusalOutput := "I cannot evaluate these analyses because there is insufficient information to make a meaningful assessment. " +
		strings.Repeat("The analyses lack critical details needed for proper evaluation. ", 10)
	result := &core.ExecuteResult{
		Output:    refusalOutput,
		TokensIn:  200,
		TokensOut: 300,
	}

	_, err := mod.processModeratorResponse(wctx, result, "claude", 1, "opus", 500)
	if err == nil {
		t.Fatal("expected error for refusal output")
	}
	if !strings.Contains(err.Error(), "moderator output validation") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

// =============================================================================
// Tests for finalizeModeratorResult
// =============================================================================

func TestSemanticModerator_finalizeModeratorResult_NilReport(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	wctx := &Context{
		Logger: logging.NewNop(),
		Report: nil, // No report writer
	}

	evalResult := &ModeratorEvaluationResult{Score: 0.85}
	result := &core.ExecuteResult{TokensIn: 100, TokensOut: 200}

	// Should not panic
	mod.finalizeModeratorResult(wctx, 1, 1, "claude", "opus", evalResult, result, 500)
}

// =============================================================================
// Tests for validateModeratorOutput edge cases
// =============================================================================

func TestSemanticModerator_validateModeratorOutput_RefusalKeywords(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	tests := []struct {
		name    string
		raw     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "cannot evaluate refusal",
			raw:     "I cannot evaluate this request because the analyses are incomplete. " + strings.Repeat("More text. ", 20),
			wantErr: true,
			errMsg:  "refused to score",
		},
		{
			name:    "refuse to score refusal",
			raw:     "I refuse to score these analyses because they lack substance. " + strings.Repeat("Additional context. ", 20),
			wantErr: true,
			errMsg:  "refused to score",
		},
		{
			name:    "unable to assess refusal",
			raw:     "I am unable to assess the quality of these analyses. " + strings.Repeat("Explanation text. ", 20),
			wantErr: true,
			errMsg:  "refused to score",
		},
		{
			name:    "no structure with score",
			raw:     strings.Repeat("This is plain text without any markdown or structure. ", 10),
			wantErr: true,
			errMsg:  "no numeric consensus score",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ModeratorEvaluationResult{
				Score:      0,
				ScoreFound: false,
				RawOutput:  tt.raw,
			}
			err := mod.validateModeratorOutput(result, tt.raw)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestSemanticModerator_validateModeratorOutput_MissingStructure(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	// Has a score but no structure
	raw := "The overall numeric rating is 80 percent and agents generally agree on the approach taken for the implementation." + strings.Repeat(" More text.", 5)
	result := &ModeratorEvaluationResult{
		Score:      0.80,
		ScoreFound: true,
		RawOutput:  raw,
	}

	err := mod.validateModeratorOutput(result, raw)
	if err == nil {
		t.Error("expected error for output lacking structure")
	}
	if !strings.Contains(err.Error(), "lacks expected structure") {
		t.Errorf("expected 'lacks expected structure' error, got: %v", err)
	}
}

// =============================================================================
// Tests for buildAnalysisSummary edge cases
// =============================================================================

func TestBuildAnalysisSummary_StructuredDataExceedsLimit(t *testing.T) {
	t.Parallel()

	// Build an output with many claims that exceed the max
	claims := make([]string, 25)
	for i := range claims {
		claims[i] = fmt.Sprintf("Claim %d: %s", i+1, strings.Repeat("detailed claim text ", 10))
	}
	risks := make([]string, 20)
	for i := range risks {
		risks[i] = fmt.Sprintf("Risk %d: %s", i+1, strings.Repeat("risk description ", 10))
	}
	recommendations := make([]string, 20)
	for i := range recommendations {
		recommendations[i] = fmt.Sprintf("Recommendation %d: %s", i+1, strings.Repeat("rec text ", 10))
	}

	out := AnalysisOutput{
		RawOutput:       strings.Repeat("Long raw output content. ", 5000),
		Claims:          claims,
		Risks:           risks,
		Recommendations: recommendations,
	}

	result := buildAnalysisSummary(out, 2000)

	if len(result) > 2000 {
		// The function should truncate to maxChars
		t.Errorf("result length %d exceeds maxChars 2000", len(result))
	}
}

func TestBuildAnalysisSummary_AdditionalContextAppended(t *testing.T) {
	t.Parallel()

	// RawOutput must exceed maxChars so the function takes the structured summary path.
	// The structured summary (Claims/Risks/Recs) is small, so remainingChars > 5000,
	// which triggers the "Additional Context" section from the raw output.
	rawOutput := strings.Repeat("Additional raw context from full analysis. ", 2000) // ~88000 chars
	out := AnalysisOutput{
		RawOutput:       rawOutput,
		Claims:          []string{"Claim 1"},
		Risks:           []string{"Risk 1"},
		Recommendations: []string{"Rec 1"},
	}

	// maxChars must be > len(RawOutput) is FALSE (so we enter structured path),
	// but large enough that remainingChars > 5000 after building the short summary.
	result := buildAnalysisSummary(out, 50000)

	if !strings.Contains(result, "Additional Context") {
		t.Error("expected 'Additional Context' section when space permits")
	}
	if !strings.Contains(result, "Claim 1") {
		t.Error("expected claims in result")
	}
}

func TestBuildAnalysisSummary_NoStructuredData(t *testing.T) {
	t.Parallel()

	longOutput := strings.Repeat("This is unstructured analysis text. ", 500)
	out := AnalysisOutput{
		RawOutput: longOutput,
		// No Claims, Risks, or Recommendations
	}

	result := buildAnalysisSummary(out, 5000)

	if len(result) > 5000 {
		t.Errorf("result length %d exceeds maxChars 5000", len(result))
	}
	// Should contain truncation notice
	if !strings.Contains(result, "[see full report") {
		t.Error("expected truncation notice in result")
	}
}

// =============================================================================
// Tests for truncateText edge cases
// =============================================================================

func TestTruncateText_VerySmallMaxChars(t *testing.T) {
	t.Parallel()

	// maxChars so small that targetLen would be < 1000, gets clamped to 1000
	text := strings.Repeat("x", 5000)
	result := truncateText(text, 500)

	// Should still produce output (clamped to 1000)
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "[see full report") {
		t.Error("expected truncation notice")
	}
}

func TestTruncateText_WithSectionBoundary(t *testing.T) {
	t.Parallel()

	text := "## Section 1\n\nContent for section 1.\n\n## Section 2\n\nContent for section 2.\n\n" + strings.Repeat("x", 3000)
	result := truncateText(text, 200)

	// Should truncate
	if len(result) >= len(text) {
		t.Error("expected truncation")
	}
}

func TestTruncateText_WithParagraphBoundary(t *testing.T) {
	t.Parallel()

	text := "First paragraph content.\n\nSecond paragraph content.\n\n" + strings.Repeat("x", 3000)
	result := truncateText(text, 200)

	if len(result) >= len(text) {
		t.Error("expected truncation")
	}
}

func TestTruncateText_ShortText(t *testing.T) {
	t.Parallel()

	text := "Short"
	result := truncateText(text, 1000)
	if result != text {
		t.Errorf("expected no truncation, got %q", result)
	}
}

// =============================================================================
// Tests for parseModeratorResponse - additional patterns
// =============================================================================

func TestParseModeratorResponse_AnchorPattern(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	output := "Long analysis text without YAML frontmatter.\n\n" +
		"## Agreements\n- Point 1\n\n" +
		"## Divergences\n- Diff 1\n\n" +
		">> FINAL SCORE: 78 <<"

	result := mod.parseModeratorResponse(output)
	if !result.ScoreFound {
		t.Error("expected score to be found via anchor pattern")
	}
	if result.Score != 0.78 {
		t.Errorf("expected score 0.78, got %f", result.Score)
	}
}

func TestParseModeratorResponse_EmptyOutput(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	result := mod.parseModeratorResponse("")
	if result.ScoreFound {
		t.Error("expected no score found for empty output")
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %f", result.Score)
	}
}

func TestParseModeratorResponse_YAMLWrappedInCodeBlock(t *testing.T) {
	t.Parallel()

	mod, _ := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "claude",
	})

	output := "```yaml\n---\nconsensus_score: 92\n---\n```\n\n## Agreements\n- All agree\n\n## Divergences\n- None"

	result := mod.parseModeratorResponse(output)
	if !result.ScoreFound {
		t.Error("expected score to be found after sanitizing code blocks")
	}
	if result.Score != 0.92 {
		t.Errorf("expected score 0.92, got %f", result.Score)
	}
}

