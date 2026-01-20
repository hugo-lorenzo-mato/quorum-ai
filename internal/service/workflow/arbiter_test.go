package workflow

import (
	"strings"
	"testing"
)

func TestNewSemanticArbiter_UsesConfigValues(t *testing.T) {
	// Configuration defaults are set in internal/config/loader.go
	// The arbiter should use whatever values come from the config
	config := ArbiterConfig{
		Enabled:             true,
		Agent:               "gemini",
		Model:               "gemini-2.5-pro",
		Threshold:           0.90,
		MinRounds:           2,
		MaxRounds:           5,
		AbortThreshold:      0.30,
		StagnationThreshold: 0.02,
	}
	arbiter, err := NewSemanticArbiter(config)
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v", err)
	}

	if arbiter.Threshold() != 0.90 {
		t.Errorf("Threshold() = %v, want 0.90", arbiter.Threshold())
	}
	if arbiter.MinRounds() != 2 {
		t.Errorf("MinRounds() = %v, want 2", arbiter.MinRounds())
	}
	if arbiter.MaxRounds() != 5 {
		t.Errorf("MaxRounds() = %v, want 5", arbiter.MaxRounds())
	}
	if arbiter.AbortThreshold() != 0.30 {
		t.Errorf("AbortThreshold() = %v, want 0.30", arbiter.AbortThreshold())
	}
	if arbiter.StagnationThreshold() != 0.02 {
		t.Errorf("StagnationThreshold() = %v, want 0.02", arbiter.StagnationThreshold())
	}
	if arbiter.GetConfig().Agent != "gemini" {
		t.Errorf("Agent = %v, want gemini", arbiter.GetConfig().Agent)
	}
	if arbiter.GetConfig().Model != "gemini-2.5-pro" {
		t.Errorf("Model = %v, want gemini-2.5-pro", arbiter.GetConfig().Model)
	}
}

func TestNewSemanticArbiter_CustomConfig(t *testing.T) {
	config := ArbiterConfig{
		Enabled:             true,
		Agent:               "claude",
		Model:               "claude-sonnet-4-20250514",
		Threshold:           0.85,
		MinRounds:           3,
		MaxRounds:           8,
		AbortThreshold:      0.25,
		StagnationThreshold: 0.05,
	}
	arbiter, err := NewSemanticArbiter(config)
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v", err)
	}

	if arbiter.Threshold() != 0.85 {
		t.Errorf("Threshold() = %v, want 0.85", arbiter.Threshold())
	}
	if arbiter.MinRounds() != 3 {
		t.Errorf("MinRounds() = %v, want 3", arbiter.MinRounds())
	}
	if arbiter.MaxRounds() != 8 {
		t.Errorf("MaxRounds() = %v, want 8", arbiter.MaxRounds())
	}
	if arbiter.AbortThreshold() != 0.25 {
		t.Errorf("AbortThreshold() = %v, want 0.25", arbiter.AbortThreshold())
	}
	if arbiter.StagnationThreshold() != 0.05 {
		t.Errorf("StagnationThreshold() = %v, want 0.05", arbiter.StagnationThreshold())
	}
	if arbiter.GetConfig().Agent != "claude" {
		t.Errorf("Agent = %v, want claude", arbiter.GetConfig().Agent)
	}
	if arbiter.GetConfig().Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %v, want claude-sonnet-4-20250514", arbiter.GetConfig().Model)
	}
}

func TestNewSemanticArbiter_RequiresAgentWhenEnabled(t *testing.T) {
	config := ArbiterConfig{
		Enabled: true,
		Model:   "gemini-2.5-pro",
		// Agent is missing
	}
	_, err := NewSemanticArbiter(config)
	if err == nil {
		t.Fatal("NewSemanticArbiter() should return error when agent is missing")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("error should mention 'agent', got: %v", err)
	}
}

func TestNewSemanticArbiter_RequiresModelWhenEnabled(t *testing.T) {
	config := ArbiterConfig{
		Enabled: true,
		Agent:   "gemini",
		// Model is missing
	}
	_, err := NewSemanticArbiter(config)
	if err == nil {
		t.Fatal("NewSemanticArbiter() should return error when model is missing")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}

func TestNewSemanticArbiter_DisabledDoesNotRequireAgentOrModel(t *testing.T) {
	// When disabled, agent and model are not required
	config := ArbiterConfig{
		Enabled: false,
		// Agent and Model are missing but that's OK when disabled
	}
	arbiter, err := NewSemanticArbiter(config)
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v, want nil when disabled", err)
	}
	if arbiter.IsEnabled() {
		t.Error("IsEnabled() = true, want false")
	}
}

func TestNewSemanticArbiter_MinRoundsExceedsMaxRounds(t *testing.T) {
	// Test that min_rounds is clamped to max_rounds when it exceeds
	config := ArbiterConfig{
		Enabled:   true,
		Agent:     "gemini",
		Model:     "gemini-2.5-pro",
		MinRounds: 10,
		MaxRounds: 5,
	}
	arbiter, err := NewSemanticArbiter(config)
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v", err)
	}

	if arbiter.MinRounds() != 5 {
		t.Errorf("MinRounds() = %v, want 5 (clamped to max_rounds)", arbiter.MinRounds())
	}
}

func TestNewSemanticArbiter_MinRoundsExceedsCustomMaxRounds(t *testing.T) {
	config := ArbiterConfig{
		Enabled:   true,
		Agent:     "claude",
		Model:     "claude-sonnet-4-20250514",
		MinRounds: 6,
		MaxRounds: 4,
	}
	arbiter, err := NewSemanticArbiter(config)
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v", err)
	}

	// min_rounds should be clamped to max_rounds
	if arbiter.MinRounds() != 4 {
		t.Errorf("MinRounds() = %v, want 4 (clamped to max_rounds)", arbiter.MinRounds())
	}
}

func TestSemanticArbiter_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		config  ArbiterConfig
		want    bool
		wantErr bool
	}{
		{
			name:    "enabled with config",
			config:  ArbiterConfig{Enabled: true, Agent: "gemini", Model: "gemini-2.5-pro"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "disabled",
			config:  ArbiterConfig{Enabled: false},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arbiter, err := NewSemanticArbiter(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSemanticArbiter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got := arbiter.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseArbiterResponse(t *testing.T) {
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
	}

	arbiter, err := NewSemanticArbiter(ArbiterConfig{
		Enabled: true,
		Agent:   "gemini",
		Model:   "gemini-2.5-pro",
	})
	if err != nil {
		t.Fatalf("NewSemanticArbiter() error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := arbiter.parseArbiterResponse(tt.output)

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

func TestExtractArbiterSection(t *testing.T) {
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
			items := extractArbiterSection(text, tt.section)
			if len(items) != tt.wantCount {
				t.Errorf("extractArbiterSection(%q) returned %d items, want %d", tt.section, len(items), tt.wantCount)
			}
		})
	}
}
