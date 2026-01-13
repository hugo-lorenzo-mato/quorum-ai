package service

import (
	"sort"
	"strings"
	"unicode"
)

// ConsensusChecker evaluates agreement between agent outputs.
type ConsensusChecker struct {
	Threshold      float64
	V2Threshold    float64
	HumanThreshold float64
	Weights        CategoryWeights
}

// CategoryWeights defines the importance of each category.
type CategoryWeights struct {
	Claims          float64 // Factual statements, findings
	Risks           float64 // Potential issues, concerns
	Recommendations float64 // Suggested actions
}

// DefaultWeights returns the default category weights.
func DefaultWeights() CategoryWeights {
	return CategoryWeights{
		Claims:          0.40,
		Risks:           0.30,
		Recommendations: 0.30,
	}
}

// NewConsensusChecker creates a new consensus checker.
func NewConsensusChecker(threshold float64, weights CategoryWeights) *ConsensusChecker {
	return &ConsensusChecker{
		Threshold:      threshold,
		V2Threshold:    0.60, // Default V2 escalation threshold
		HumanThreshold: 0.50, // Default human review threshold
		Weights:        weights,
	}
}

// NewConsensusCheckerWithThresholds creates a consensus checker with explicit escalation thresholds.
func NewConsensusCheckerWithThresholds(threshold, v2Threshold, humanThreshold float64, weights CategoryWeights) *ConsensusChecker {
	return &ConsensusChecker{
		Threshold:      threshold,
		V2Threshold:    v2Threshold,
		HumanThreshold: humanThreshold,
		Weights:        weights,
	}
}

// AnalysisOutput represents structured output from an agent.
type AnalysisOutput struct {
	AgentName       string
	Claims          []string
	Risks           []string
	Recommendations []string
	RawOutput       string
}

// ConsensusResult contains the consensus evaluation results.
type ConsensusResult struct {
	Score            float64
	NeedsV3          bool
	NeedsHumanReview bool
	CategoryScores   map[string]float64
	Divergences      []Divergence
	Agreement        map[string][]string // Common items per category
}

// Divergence represents a disagreement between agents.
type Divergence struct {
	Category     string
	Agent1       string
	Agent1Items  []string
	Agent2       string
	Agent2Items  []string
	JaccardScore float64
}

// Evaluate calculates consensus between multiple outputs.
func (c *ConsensusChecker) Evaluate(outputs []AnalysisOutput) ConsensusResult {
	if len(outputs) < 2 {
		return ConsensusResult{
			Score:            1.0,
			NeedsV3:          false,
			NeedsHumanReview: false,
			CategoryScores:   make(map[string]float64),
			Agreement:        make(map[string][]string),
		}
	}

	// Calculate pairwise scores for each category
	claimsScores := c.pairwiseJaccard(outputs, func(o AnalysisOutput) []string { return o.Claims })
	risksScores := c.pairwiseJaccard(outputs, func(o AnalysisOutput) []string { return o.Risks })
	recsScores := c.pairwiseJaccard(outputs, func(o AnalysisOutput) []string { return o.Recommendations })

	// Average scores per category
	claimsAvg := average(claimsScores)
	risksAvg := average(risksScores)
	recsAvg := average(recsScores)

	// Weighted total score
	totalScore := claimsAvg*c.Weights.Claims +
		risksAvg*c.Weights.Risks +
		recsAvg*c.Weights.Recommendations

	// Find divergences
	divergences := c.findDivergences(outputs, claimsScores, risksScores, recsScores)

	// Find agreement (intersection)
	agreement := c.findAgreement(outputs)

	result := ConsensusResult{
		Score: totalScore,
		CategoryScores: map[string]float64{
			"claims":          claimsAvg,
			"risks":           risksAvg,
			"recommendations": recsAvg,
		},
		Divergences: divergences,
		Agreement:   agreement,
	}

	// Determine escalation using explicit 80/60/50 thresholds:
	// - score >= threshold (80%): approved, no escalation
	// - score < threshold but >= human_threshold: needs escalation (V2, possibly V3)
	// - score < human_threshold (50%): requires human review, abort execution
	result.NeedsV3 = totalScore < c.Threshold
	result.NeedsHumanReview = totalScore < c.HumanThreshold

	return result
}

// HasConsensus returns true if the score meets the threshold.
func (r *ConsensusResult) HasConsensus(threshold float64) bool {
	return r.Score >= threshold
}

// GetThreshold returns the consensus threshold value.
func (c *ConsensusChecker) GetThreshold() float64 {
	return c.Threshold
}

// GetV2Threshold returns the V2 escalation threshold.
func (c *ConsensusChecker) GetV2Threshold() float64 {
	return c.V2Threshold
}

// GetHumanThreshold returns the human review threshold.
func (c *ConsensusChecker) GetHumanThreshold() float64 {
	return c.HumanThreshold
}

// pairwiseJaccard calculates Jaccard similarity for all pairs.
func (c *ConsensusChecker) pairwiseJaccard(outputs []AnalysisOutput, extract func(AnalysisOutput) []string) []float64 {
	scores := make([]float64, 0)

	for i := 0; i < len(outputs); i++ {
		for j := i + 1; j < len(outputs); j++ {
			set1 := normalizeSet(extract(outputs[i]))
			set2 := normalizeSet(extract(outputs[j]))
			score := JaccardSimilarity(set1, set2)
			scores = append(scores, score)
		}
	}

	return scores
}

// JaccardSimilarity calculates Jaccard index: |A ∩ B| / |A ∪ B|
func JaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0 // Both empty = perfect agreement
	}

	setA := toSet(a)
	setB := toSet(b)

	intersection := 0
	for item := range setA {
		if setB[item] {
			intersection++
		}
	}

	union := len(setA)
	for item := range setB {
		if !setA[item] {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// normalizeSet normalizes items for comparison.
func normalizeSet(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		normalized := NormalizeText(item)
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

// NormalizeText normalizes text for comparison.
func NormalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Remove punctuation and extra whitespace
	var builder strings.Builder
	prevSpace := true
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			builder.WriteRune(' ')
			prevSpace = true
		}
	}

	return strings.TrimSpace(builder.String())
}

// toSet converts a slice to a set (map).
func toSet(items []string) map[string]bool {
	result := make(map[string]bool)
	for _, item := range items {
		result[item] = true
	}
	return result
}

// average calculates the average of a slice.
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// findDivergences identifies significant disagreements.
func (c *ConsensusChecker) findDivergences(outputs []AnalysisOutput, claims, risks, recs []float64) []Divergence {
	divergences := make([]Divergence, 0)
	divergenceThreshold := 0.5 // Below this is a significant divergence

	idx := 0
	for i := 0; i < len(outputs); i++ {
		for j := i + 1; j < len(outputs); j++ {
			if idx < len(claims) && claims[idx] < divergenceThreshold {
				divergences = append(divergences, Divergence{
					Category:     "claims",
					Agent1:       outputs[i].AgentName,
					Agent1Items:  outputs[i].Claims,
					Agent2:       outputs[j].AgentName,
					Agent2Items:  outputs[j].Claims,
					JaccardScore: claims[idx],
				})
			}
			if idx < len(risks) && risks[idx] < divergenceThreshold {
				divergences = append(divergences, Divergence{
					Category:     "risks",
					Agent1:       outputs[i].AgentName,
					Agent1Items:  outputs[i].Risks,
					Agent2:       outputs[j].AgentName,
					Agent2Items:  outputs[j].Risks,
					JaccardScore: risks[idx],
				})
			}
			if idx < len(recs) && recs[idx] < divergenceThreshold {
				divergences = append(divergences, Divergence{
					Category:     "recommendations",
					Agent1:       outputs[i].AgentName,
					Agent1Items:  outputs[i].Recommendations,
					Agent2:       outputs[j].AgentName,
					Agent2Items:  outputs[j].Recommendations,
					JaccardScore: recs[idx],
				})
			}
			idx++
		}
	}

	return divergences
}

// findAgreement finds items that all agents agree on.
func (c *ConsensusChecker) findAgreement(outputs []AnalysisOutput) map[string][]string {
	agreement := make(map[string][]string)

	if len(outputs) == 0 {
		return agreement
	}

	// Find intersection for each category
	agreement["claims"] = intersectAll(extractAll(outputs, func(o AnalysisOutput) []string { return o.Claims }))
	agreement["risks"] = intersectAll(extractAll(outputs, func(o AnalysisOutput) []string { return o.Risks }))
	agreement["recommendations"] = intersectAll(extractAll(outputs, func(o AnalysisOutput) []string { return o.Recommendations }))

	return agreement
}

func extractAll(outputs []AnalysisOutput, extract func(AnalysisOutput) []string) [][]string {
	result := make([][]string, len(outputs))
	for i, o := range outputs {
		result[i] = normalizeSet(extract(o))
	}
	return result
}

func intersectAll(sets [][]string) []string {
	if len(sets) == 0 {
		return nil
	}

	// Start with first set
	result := toSet(sets[0])

	// Intersect with remaining sets
	for i := 1; i < len(sets); i++ {
		nextSet := toSet(sets[i])
		for item := range result {
			if !nextSet[item] {
				delete(result, item)
			}
		}
	}

	// Convert back to slice
	items := make([]string, 0, len(result))
	for item := range result {
		items = append(items, item)
	}
	sort.Strings(items)
	return items
}
