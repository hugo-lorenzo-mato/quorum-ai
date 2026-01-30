package workflow

import (
	"strings"
	"testing"
)

func TestNewSemanticModerator_UsesConfigValues(t *testing.T) {
	// Configuration defaults are set in internal/config/loader.go
	// The moderator should use whatever values come from the config
	// Note: Model is resolved at runtime from AgentPhaseModels[Agent][analyze]
	config := ModeratorConfig{
		Enabled:             true,
		Agent:               "gemini",
		Threshold:           0.90,
		MinRounds:           2,
		MaxRounds:           5,
		WarningThreshold:    0.30,
		StagnationThreshold: 0.02,
	}
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	if moderator.Threshold() != 0.90 {
		t.Errorf("Threshold() = %v, want 0.90", moderator.Threshold())
	}
	if moderator.MinRounds() != 2 {
		t.Errorf("MinRounds() = %v, want 2", moderator.MinRounds())
	}
	if moderator.MaxRounds() != 5 {
		t.Errorf("MaxRounds() = %v, want 5", moderator.MaxRounds())
	}
	if moderator.WarningThreshold() != 0.30 {
		t.Errorf("WarningThreshold() = %v, want 0.30", moderator.WarningThreshold())
	}
	if moderator.StagnationThreshold() != 0.02 {
		t.Errorf("StagnationThreshold() = %v, want 0.02", moderator.StagnationThreshold())
	}
	if moderator.GetConfig().Agent != "gemini" {
		t.Errorf("Agent = %v, want gemini", moderator.GetConfig().Agent)
	}
}

func TestNewSemanticModerator_CustomConfig(t *testing.T) {
	// Note: Model is resolved at runtime from AgentPhaseModels[Agent][analyze]
	config := ModeratorConfig{
		Enabled:             true,
		Agent:               "claude",
		Threshold:           0.85,
		MinRounds:           3,
		MaxRounds:           8,
		WarningThreshold:    0.25,
		StagnationThreshold: 0.05,
	}
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	if moderator.Threshold() != 0.85 {
		t.Errorf("Threshold() = %v, want 0.85", moderator.Threshold())
	}
	if moderator.MinRounds() != 3 {
		t.Errorf("MinRounds() = %v, want 3", moderator.MinRounds())
	}
	if moderator.MaxRounds() != 8 {
		t.Errorf("MaxRounds() = %v, want 8", moderator.MaxRounds())
	}
	if moderator.WarningThreshold() != 0.25 {
		t.Errorf("WarningThreshold() = %v, want 0.25", moderator.WarningThreshold())
	}
	if moderator.StagnationThreshold() != 0.05 {
		t.Errorf("StagnationThreshold() = %v, want 0.05", moderator.StagnationThreshold())
	}
	if moderator.GetConfig().Agent != "claude" {
		t.Errorf("Agent = %v, want claude", moderator.GetConfig().Agent)
	}
}

func TestNewSemanticModerator_RequiresAgentWhenEnabled(t *testing.T) {
	// Model is resolved at runtime from AgentPhaseModels, so only Agent is required
	config := ModeratorConfig{
		Enabled: true,
		// Agent is missing
	}
	_, err := NewSemanticModerator(config)
	if err == nil {
		t.Fatal("NewSemanticModerator() should return error when agent is missing")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error should mention 'agent', got: %v", err)
	}
}

func TestNewSemanticModerator_DisabledDoesNotRequireAgentOrModel(t *testing.T) {
	// When disabled, agent and model are not required
	config := ModeratorConfig{
		Enabled: false,
		// Agent and Model are missing but that's OK when disabled
	}
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v, want nil when disabled", err)
	}
	if moderator.IsEnabled() {
		t.Error("IsEnabled() = true, want false")
	}
}

func TestNewSemanticModerator_MinRoundsExceedsMaxRounds(t *testing.T) {
	// Test that min_rounds is clamped to max_rounds when it exceeds
	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "gemini",
		MinRounds: 10,
		MaxRounds: 5,
	}
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	if moderator.MinRounds() != 5 {
		t.Errorf("MinRounds() = %v, want 5 (clamped to max_rounds)", moderator.MinRounds())
	}
}

func TestNewSemanticModerator_MinRoundsExceedsCustomMaxRounds(t *testing.T) {
	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "claude",
		MinRounds: 6,
		MaxRounds: 4,
	}
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	// min_rounds should be clamped to max_rounds
	if moderator.MinRounds() != 4 {
		t.Errorf("MinRounds() = %v, want 4 (clamped to max_rounds)", moderator.MinRounds())
	}
}

func TestSemanticModerator_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  ModeratorConfig
		want    bool
		wantErr bool
	}{
		{
			name:    "enabled with config",
			config:  ModeratorConfig{Enabled: true, Agent: "gemini"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "disabled",
			config:  ModeratorConfig{Enabled: false},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			moderator, err := NewSemanticModerator(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSemanticModerator() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got := moderator.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseModeratorResponse(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		wantScore      float64
		wantAgreements int
		wantDivs       int
	}{
		{
			name: "valid response with 85% consensus",
			output: `CONSENSUS_SCORE: 85%

## Score Rationale
Good agreement overall.

## Agreements
- Both agents identify the main risk
- Both recommend testing

## Divergences
- Agent A focuses on security, Agent B on performance

## Missing Perspectives
- Cost analysis not addressed

## Recommendations for Next Round
- Explore security implications further`,
			wantScore:      0.85,
			wantAgreements: 2,
			wantDivs:       1,
		},
		{
			name: "response with 100% consensus",
			output: `CONSENSUS_SCORE: 100%

## Agreements
- Full agreement on all points

## Divergences
(None)`,
			wantScore:      1.0,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "response with low consensus",
			output: `CONSENSUS_SCORE: 35%

## Agreements
- Both agree on basic structure

## Divergences
- Major disagreement on approach
- Different risk assessments
- Conflicting recommendations`,
			wantScore:      0.35,
			wantAgreements: 1,
			wantDivs:       3,
		},
		{
			name:           "malformed response - no score",
			output:         "Some analysis without proper formatting",
			wantScore:      0,
			wantAgreements: 0,
			wantDivs:       0,
		},
		{
			name: "fallback pattern with 75% consensus",
			output: `After deep analysis of all perspectives...

The consensus evaluation reveals significant agreement.

=-=-=-=75%=-=-=-=-=

## Agreements
- All agents agree on architecture approach
- Security concerns are consistent

## Divergences
- Minor difference in implementation timeline`,
			wantScore:      0.75,
			wantAgreements: 2,
			wantDivs:       1,
		},
		{
			name: "fallback pattern with 92% consensus (no primary pattern)",
			output: `Extended reasoning complete.

Based on thorough ultracritical analysis:

=-=-=-=92%=-=-=-=-=

## Agreements
- Complete alignment on risk assessment
- Unanimous on priority recommendations
- Shared understanding of constraints

## Divergences
(None identified)`,
			wantScore:      0.92,
			wantAgreements: 3,
			wantDivs:       0,
		},
		{
			name: "primary pattern takes precedence over fallback",
			output: `CONSENSUS_SCORE: 80%

Some text with fallback pattern embedded: =-=-=-=50%=-=-=-=-=

## Agreements
- Agreement point

## Divergences
- Divergence point`,
			wantScore:      0.80,
			wantAgreements: 1,
			wantDivs:       1,
		},
		{
			name: "markdown formatted score (bold)",
			output: `**CONSENSUS_SCORE:** 78%

## Score Rationale
Good agreement.

## Agreements
- Both agree on approach`,
			wantScore:      0.78,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "score without space after colon",
			output: `CONSENSUS_SCORE:65%

## Agreements
- Agreement found`,
			wantScore:      0.65,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "generic 'consensus score' phrase in text",
			output: `After thorough analysis, the consensus score: 72%

## Agreements
- Multiple agreements`,
			wantScore:      0.72,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "overall score with 'is' keyword",
			output: `The overall score is 88%

## Agreements
- Full agreement`,
			wantScore:      0.88,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "decimal format score",
			output: `The semantic score is 0.67

## Agreements
- Some agreement`,
			wantScore:      0.67,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "score with underscore (CONSENSUS_SCORE)",
			output: `CONSENSUS SCORE: 90%

## Agreements
- High agreement`,
			wantScore:      0.90,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "YAML frontmatter format (primary)",
			output: `---
consensus_score: 85
high_impact_divergences: 1
medium_impact_divergences: 2
low_impact_divergences: 0
agreements_count: 3
---

## Score Rationale
Good overall consensus with minor implementation differences.

## Agreements
- Architecture approach agreed
- Security model aligned
- API design consistent

## Divergences
- Minor timing differences`,
			wantScore:      0.85,
			wantAgreements: 3,
			wantDivs:       1,
		},
		{
			name: "YAML frontmatter with 100% consensus",
			output: `---
consensus_score: 100
high_impact_divergences: 0
medium_impact_divergences: 0
low_impact_divergences: 0
agreements_count: 5
---

## Agreements
- Full alignment on all points

## Divergences
(None)`,
			wantScore:      1.0,
			wantAgreements: 1,
			wantDivs:       0,
		},
		{
			name: "YAML frontmatter with low consensus",
			output: `---
consensus_score: 25
high_impact_divergences: 3
medium_impact_divergences: 2
low_impact_divergences: 1
agreements_count: 1
---

## Agreements
- Basic structure agreed

## Divergences
- Major architectural disagreement
- Different security approaches
- Conflicting API designs`,
			wantScore:      0.25,
			wantAgreements: 1,
			wantDivs:       3,
		},
	}

	moderator, err := NewSemanticModerator(ModeratorConfig{
		Enabled: true,
		Agent:   "gemini",
	})
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := moderator.parseModeratorResponse(tt.output)

			if result.Score != tt.wantScore {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}
			if len(result.Agreements) != tt.wantAgreements {
				t.Errorf("len(Agreements) = %v, want %v", len(result.Agreements), tt.wantAgreements)
			}
			if len(result.Divergences) != tt.wantDivs {
				t.Errorf("len(Divergences) = %v, want %v", len(result.Divergences), tt.wantDivs)
			}
			if result.RawOutput != tt.output {
				t.Errorf("RawOutput not preserved")
			}
		})
	}
}

func TestExtractModeratorSection(t *testing.T) {
	text := `# Report

## Agreements
- Point one
- Point two
- Point three

## Divergences
- Difference A
- Difference B

## Other Section
Some content here.`

	tests := []struct {
		section   string
		wantCount int
	}{
		{"Agreements", 3},
		{"Divergences", 2},
		{"Other Section", 0}, // No bullet points
		{"NonExistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.section, func(t *testing.T) {
			items := extractModeratorSection(text, tt.section)
			if len(items) != tt.wantCount {
				t.Errorf("extractModeratorSection(%q) returned %d items, want %d", tt.section, len(items), tt.wantCount)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxChars  int
		wantTrunc bool
		wantNote  bool
	}{
		{
			name:      "short text no truncation",
			input:     "Short analysis text",
			maxChars:  1000,
			wantTrunc: false,
			wantNote:  false,
		},
		{
			name:      "long text gets truncated",
			input:     strings.Repeat("This is a long analysis. ", 3000),
			maxChars:  5000,
			wantTrunc: true,
			wantNote:  true,
		},
		{
			name:      "truncation preserves section boundaries",
			input:     "## Section 1\n\nContent here.\n\n## Section 2\n\nMore content.\n\n" + strings.Repeat("x", 10000),
			maxChars:  500,
			wantTrunc: true,
			wantNote:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.input, tt.maxChars)

			if tt.wantTrunc {
				if len(result) >= len(tt.input) {
					t.Errorf("expected truncation, but output length (%d) >= input length (%d)", len(result), len(tt.input))
				}
			} else {
				if result != tt.input {
					t.Errorf("expected no truncation, but result differs from input")
				}
			}

			if tt.wantNote {
				if !strings.Contains(result, "[see full report") {
					t.Errorf("expected truncation notice in output")
				}
			}
		})
	}
}

func TestBuildAnalysisSummary(t *testing.T) {
	tests := []struct {
		name     string
		input    AnalysisOutput
		maxChars int
		wantSumm bool // Expect summary (not full output)
	}{
		{
			name: "short output no summary needed",
			input: AnalysisOutput{
				RawOutput: "Short analysis",
			},
			maxChars: 1000,
			wantSumm: false,
		},
		{
			name: "long output with structured data uses summary",
			input: AnalysisOutput{
				RawOutput: strings.Repeat("Long content. ", 5000),
				Claims:    []string{"Claim 1", "Claim 2", "Claim 3"},
				Risks:     []string{"Risk 1", "Risk 2"},
			},
			maxChars: 5000,
			wantSumm: true,
		},
		{
			name: "long output without structured data uses truncation",
			input: AnalysisOutput{
				RawOutput: strings.Repeat("Long content. ", 5000),
			},
			maxChars: 5000,
			wantSumm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAnalysisSummary(tt.input, tt.maxChars)

			if tt.wantSumm {
				if len(result) >= len(tt.input.RawOutput) {
					t.Errorf("expected summary/truncation, but output length (%d) >= input length (%d)", len(result), len(tt.input.RawOutput))
				}
				if len(result) > tt.maxChars {
					t.Errorf("result length (%d) exceeds maxChars (%d)", len(result), tt.maxChars)
				}
			} else {
				if result != tt.input.RawOutput {
					t.Errorf("expected full output, but got different result")
				}
			}
		})
	}
}
