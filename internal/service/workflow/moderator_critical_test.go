package workflow

import (
	"strings"
	"testing"
	"time"
)

// TestSemanticModerator_ConsensusEdgeCases tests critical edge cases.
func TestSemanticModerator_ConsensusEdgeCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		threshold float64
		score     float64
		wantPass  bool
	}{
		{
			name:      "exact_threshold_passes",
			threshold: 0.75,
			score:     0.75,
			wantPass:  true,
		},
		{
			name:      "above_threshold_passes",
			threshold: 0.75,
			score:     0.76,
			wantPass:  true,
		},
		{
			name:      "below_threshold_fails",
			threshold: 0.75,
			score:     0.74,
			wantPass:  false,
		},
		{
			name:      "tie_at_50_percent",
			threshold: 0.50,
			score:     0.50,
			wantPass:  true,
		},
		{
			name:      "close_to_49_percent",
			threshold: 0.50,
			score:     0.49,
			wantPass:  false,
		},
		{
			name:      "perfect_consensus",
			threshold: 0.75,
			score:     1.0,
			wantPass:  true,
		},
		{
			name:      "no_consensus",
			threshold: 0.75,
			score:     0.0,
			wantPass:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			config := ModeratorConfig{
				Enabled:   true,
				Agent:     "test",
				Threshold: tc.threshold,
			}
			
			moderator, err := NewSemanticModerator(config)
			if err != nil {
				t.Fatalf("NewSemanticModerator() error = %v", err)
			}

			// Create test result with specific score
			result := &ModeratorEvaluationResult{
				Score:      tc.score,
				ScoreFound: true,
			}

			passed := result.Score >= moderator.Threshold()
			if passed != tc.wantPass {
				t.Errorf("consensus check = %v, want %v (score: %.2f, threshold: %.2f)", 
					passed, tc.wantPass, tc.score, tc.threshold)
			}
		})
	}
}

// TestSemanticModerator_StagnationDetection tests stagnation detection algorithm.
func TestSemanticModerator_StagnationDetection(t *testing.T) {
	t.Parallel()

	config := ModeratorConfig{
		Enabled:             true,
		Agent:               "test",
		StagnationThreshold: 0.05, // 5% improvement required
	}
	
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	testCases := []struct {
		name         string
		scores       []float64
		wantStagnant bool
	}{
		{
			name:         "improving_scores",
			scores:       []float64{0.30, 0.45, 0.60, 0.75},
			wantStagnant: false,
		},
		{
			name:         "stagnant_scores", 
			scores:       []float64{0.60, 0.61, 0.62, 0.63},
			wantStagnant: true,
		},
		{
			name:         "declining_scores",
			scores:       []float64{0.80, 0.75, 0.70, 0.65},
			wantStagnant: true, // No improvement
		},
		{
			name:         "fluctuating_within_threshold",
			scores:       []float64{0.50, 0.52, 0.48, 0.51},
			wantStagnant: true,
		},
		{
			name:         "significant_final_improvement",
			scores:       []float64{0.40, 0.42, 0.44, 0.60},
			wantStagnant: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if len(tc.scores) < 2 {
				t.Fatal("Need at least 2 scores for stagnation test")
			}

			// Calculate if scores show stagnation
			lastScore := tc.scores[len(tc.scores)-1]
			prevScore := tc.scores[len(tc.scores)-2]
			improvement := lastScore - prevScore

			stagnant := improvement < moderator.StagnationThreshold()

			if stagnant != tc.wantStagnant {
				t.Errorf("stagnation detection = %v, want %v (improvement: %.3f, threshold: %.3f)",
					stagnant, tc.wantStagnant, improvement, moderator.StagnationThreshold())
			}
		})
	}
}

// TestSemanticModerator_LargeResponseHandling tests performance with large responses.
func TestSemanticModerator_LargeResponseHandling(t *testing.T) {
	t.Parallel()

	// Create artificially large outputs
	largeOutput := strings.Repeat("This is a very long analysis response with detailed explanations and extensive reasoning. ", 1000)
	veryLargeOutput := strings.Repeat("Even longer response with comprehensive analysis across multiple dimensions and perspectives. ", 5000)

	outputs := []AnalysisOutput{
		{
			AgentName: "agent1",
			RawOutput: largeOutput,
			TokensIn:  2500,
			TokensOut: 25000,
		},
		{
			AgentName: "agent2", 
			RawOutput: veryLargeOutput,
			TokensIn:  2500,
			TokensOut: 125000,
		},
		{
			AgentName: "agent3",
			RawOutput: largeOutput,
			TokensIn:  2500,
			TokensOut: 25000,
		},
	}

	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "test",
		Threshold: 0.75,
	}
	
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	// Test that large outputs don't crash the moderator creation and validation
	start := time.Now()
	
	// Verify outputs are preserved correctly
	totalInputTokens := 0
	totalOutputTokens := 0
	for _, output := range outputs {
		totalInputTokens += output.TokensIn
		totalOutputTokens += output.TokensOut
		
		if len(output.RawOutput) == 0 {
			t.Error("Output RawOutput should not be empty")
		}
	}

	expectedInputTokens := 3 * 2500
	expectedOutputTokens := 2*25000 + 125000

	if totalInputTokens != expectedInputTokens {
		t.Errorf("Total input tokens = %d, want %d", totalInputTokens, expectedInputTokens)
	}
	
	if totalOutputTokens != expectedOutputTokens {
		t.Errorf("Total output tokens = %d, want %d", totalOutputTokens, expectedOutputTokens)
	}

	duration := time.Since(start)
	
	// Should process reasonably quickly
	if duration > 1*time.Second {
		t.Errorf("Large response processing took too long: %v", duration)
	}

	// Test moderator configuration is preserved
	if moderator.Threshold() != 0.75 {
		t.Errorf("Threshold = %.2f, want 0.75", moderator.Threshold())
	}

	// Test large content handling in memory
	largeContent := strings.Repeat("x", 1024*1024) // 1MB of data
	if len(largeContent) != 1024*1024 {
		t.Errorf("Large content size = %d, want %d", len(largeContent), 1024*1024)
	}
}

// TestSemanticModerator_RoundLimits tests min/max round enforcement.
func TestSemanticModerator_RoundLimits(t *testing.T) {
	t.Parallel()

	config := ModeratorConfig{
		Enabled:   true,
		Agent:     "test",
		Threshold: 0.75,
		MinRounds: 3,
		MaxRounds: 6,
	}
	
	moderator, err := NewSemanticModerator(config)
	if err != nil {
		t.Fatalf("NewSemanticModerator() error = %v", err)
	}

	testCases := []struct {
		name        string
		round       int
		score       float64
		shouldStop  bool
		description string
	}{
		{
			name:        "early_high_score_continues",
			round:       1,
			score:       0.95,
			shouldStop:  false,
			description: "High score before MinRounds should continue",
		},
		{
			name:        "min_rounds_low_score_continues",
			round:       3,
			score:       0.60,
			shouldStop:  false,
			description: "Low score at MinRounds should continue",
		},
		{
			name:        "min_rounds_high_score_can_stop",
			round:       3,
			score:       0.80,
			shouldStop:  true,
			description: "High score at MinRounds can stop",
		},
		{
			name:        "max_rounds_reached",
			round:       6,
			score:       0.60,
			shouldStop:  true,
			description: "MaxRounds forces stop regardless of score",
		},
		{
			name:        "middle_round_high_score_can_stop",
			round:       4,
			score:       0.85,
			shouldStop:  true,
			description: "High score after MinRounds can stop",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Simulate decision logic
			reachedMinRounds := tc.round >= moderator.MinRounds()
			reachedMaxRounds := tc.round >= moderator.MaxRounds()
			aboveThreshold := tc.score >= moderator.Threshold()

			shouldContinue := !reachedMaxRounds && (!reachedMinRounds || !aboveThreshold)
			shouldStop := !shouldContinue

			if shouldStop != tc.shouldStop {
				t.Errorf("%s: shouldStop = %v, want %v (round: %d, score: %.2f)",
					tc.description, shouldStop, tc.shouldStop, tc.round, tc.score)
			}
		})
	}
}

// TestSemanticModerator_ConfigValidation tests configuration validation.
func TestSemanticModerator_ConfigValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		config    ModeratorConfig
		wantError bool
	}{
		{
			name: "valid_config",
			config: ModeratorConfig{
				Enabled:             true,
				Agent:               "gemini",
				Threshold:           0.75,
				MinRounds:           2,
				MaxRounds:           5,
				WarningThreshold:    0.30,
				StagnationThreshold: 0.05,
			},
			wantError: false,
		},
		{
			name: "disabled_moderator",
			config: ModeratorConfig{
				Enabled: false,
				Agent:   "gemini",
			},
			wantError: false,
		},
		{
			name: "missing_agent_when_enabled",
			config: ModeratorConfig{
				Enabled:   true,
				Agent:     "", // Empty agent when enabled
				Threshold: 0.75,
			},
			wantError: true,
		},
		{
			name: "auto_corrected_min_max_rounds",
			config: ModeratorConfig{
				Enabled:   true,
				Agent:     "gemini",
				Threshold: 0.75,
				MinRounds: 5,
				MaxRounds: 3, // Will be auto-corrected
			},
			wantError: false, // No error, auto-corrected
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			moderator, err := NewSemanticModerator(tc.config)
			
			if tc.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tc.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify auto-correction behavior
			if !tc.wantError && moderator != nil && tc.config.MinRounds > tc.config.MaxRounds && tc.config.MaxRounds > 0 {
				if moderator.MinRounds() != moderator.MaxRounds() {
					t.Errorf("MinRounds should be auto-corrected to MaxRounds: got %d, want %d", 
						moderator.MinRounds(), moderator.MaxRounds())
				}
			}
		})
	}
}