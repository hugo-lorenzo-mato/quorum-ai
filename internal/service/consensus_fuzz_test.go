//go:build go1.18

package service_test

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func FuzzJaccardSimilarity(f *testing.F) {
	// Seed corpus
	f.Add("hello world", "hello there")
	f.Add("the quick brown fox", "the quick brown dog")
	f.Add("", "")
	f.Add("a", "a")
	f.Add("identical text", "identical text")
	f.Add("completely different", "nothing alike")
	f.Add("one two three", "four five six")
	f.Add("repeated repeated repeated", "repeated once")

	f.Fuzz(func(t *testing.T, a, b string) {
		wordsA := extractWords(a)
		wordsB := extractWords(b)

		score := service.JaccardSimilarity(wordsA, wordsB)

		// Invariants: score must be between 0 and 1
		if score < 0 || score > 1 {
			t.Errorf("score out of range: %f", score)
		}

		// Symmetric: order shouldn't matter
		reverseScore := service.JaccardSimilarity(wordsB, wordsA)
		if score != reverseScore {
			t.Errorf("not symmetric: %f != %f", score, reverseScore)
		}

		// Identity: comparing with self should give 1.0 (if non-empty)
		if len(wordsA) > 0 {
			selfScore := service.JaccardSimilarity(wordsA, wordsA)
			if selfScore != 1.0 {
				t.Errorf("self similarity should be 1.0, got %f", selfScore)
			}
		}
	})
}

func FuzzConsensusEvaluate(f *testing.F) {
	// Seed with various section content
	f.Add("claims text", "risks text", "recommendations text")
	f.Add("", "", "")
	f.Add("long "+strings.Repeat("text ", 100), "medium text", "short")
	f.Add("special chars: @#$%", "more special: &*()", "unicode: 日本語")

	f.Fuzz(func(t *testing.T, claims, risks, recs string) {
		checker := service.NewConsensusChecker(0.75)

		outputs := []service.AnalysisOutput{
			{
				AgentName: "agent1",
				Sections: map[string]string{
					"claims":          claims,
					"risks":           risks,
					"recommendations": recs,
				},
			},
			{
				AgentName: "agent2",
				Sections: map[string]string{
					"claims":          claims,
					"risks":           risks,
					"recommendations": recs,
				},
			},
		}

		result := checker.Evaluate(outputs)

		// Should not panic and score should be valid
		if result.Score < 0 || result.Score > 1 {
			t.Errorf("invalid score: %f", result.Score)
		}
	})
}

func FuzzConsensusThreshold(f *testing.F) {
	f.Add(0.0)
	f.Add(0.5)
	f.Add(0.75)
	f.Add(1.0)
	f.Add(0.999999)

	f.Fuzz(func(t *testing.T, threshold float64) {
		// Only test valid thresholds (0-1 range)
		if threshold < 0 || threshold > 1 {
			return
		}

		checker := service.NewConsensusChecker(threshold)

		outputs := []service.AnalysisOutput{
			{
				AgentName: "agent1",
				Sections:  map[string]string{"test": "identical content"},
			},
			{
				AgentName: "agent2",
				Sections:  map[string]string{"test": "identical content"},
			},
		}

		result := checker.Evaluate(outputs)

		// Identical content should have high score
		if result.Score < 0.9 {
			t.Logf("surprisingly low score for identical content: %f", result.Score)
		}
	})
}

func extractWords(s string) []string {
	var words []string
	for _, word := range strings.Fields(s) {
		word = strings.ToLower(strings.Trim(word, ".,!?;:"))
		if word != "" {
			words = append(words, word)
		}
	}
	return words
}
