package validation

import (
	"bytes"
	"context"
	"testing"
)

func TestExperimentRunner_DryRun(t *testing.T) {
	runner := NewExperimentRunner("testdata/output")
	runner.SetDryRun(true)

	cfg := ExperimentConfig{
		Name:               "test-experiment",
		Description:        "Test experiment for validation",
		Prompts:            []string{"test prompt 1", "test prompt 2"},
		Thresholds:         []float64{0.70, 0.75, 0.80},
		Agents:             []string{"claude", "gemini"},
		Iterations:         1,
		SingleAgentControl: true,
	}

	result, err := runner.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.TotalRuns == 0 {
		t.Error("expected some runs")
	}

	// 3 thresholds * 2 prompts * 1 iteration = 6 multi-agent runs
	// 2 agents * 2 prompts * 1 iteration = 4 single-agent runs
	expectedRuns := 6 + 4
	if result.Summary.TotalRuns != expectedRuns {
		t.Errorf("expected %d runs, got %d", expectedRuns, result.Summary.TotalRuns)
	}
}

func TestQualityEvaluator_Completeness(t *testing.T) {
	eval := NewQualityEvaluator()

	// Complete output with all sections
	complete := `
	Summary: This code is well structured.
	Analysis: The architecture follows best practices.
	Claims: The code is secure.
	Risks: No major risks identified.
	Recommendations: Consider adding tests.
	`

	score := eval.evaluateCompleteness(complete)
	if score != 1.0 {
		t.Errorf("expected completeness 1.0, got %f", score)
	}

	// Partial output
	partial := "This is some analysis without proper sections."
	partialScore := eval.evaluateCompleteness(partial)
	if partialScore >= score {
		t.Error("partial output should have lower completeness")
	}
}

func TestQualityEvaluator_Specificity(t *testing.T) {
	eval := NewQualityEvaluator()

	// Specific output
	specific := "Specifically in file main.go at line 42, the function processData has a bug."
	specificScore := eval.evaluateSpecificity(specific)

	// Vague output
	vague := "There might be some issues. Various things could possibly be improved."
	vagueScore := eval.evaluateSpecificity(vague)

	if vagueScore >= specificScore {
		t.Errorf("vague output (%f) should score lower than specific (%f)", vagueScore, specificScore)
	}
}

func TestQualityEvaluator_Actionable(t *testing.T) {
	eval := NewQualityEvaluator()

	// Actionable output
	actionable := "You should refactor this function. Add error handling. Remove the unused variable."
	actionableScore := eval.evaluateActionable(actionable)

	if actionableScore == 0 {
		t.Error("expected non-zero actionable score")
	}

	// Non-actionable
	passive := "The code exists. Data is processed. Results are returned."
	passiveScore := eval.evaluateActionable(passive)

	if passiveScore >= actionableScore {
		t.Errorf("passive output (%f) should score lower than actionable (%f)", passiveScore, actionableScore)
	}
}

func TestQualityEvaluator_Consistency(t *testing.T) {
	eval := NewQualityEvaluator()

	// Consistent output
	consistent := "The code is safe and follows good practices."
	consistentScore := eval.evaluateConsistency(consistent)

	if consistentScore != 1.0 {
		t.Errorf("expected consistency 1.0, got %f", consistentScore)
	}

	// Contradictory output
	contradictory := "The code is good but also bad. It's safe yet unsafe."
	contradictoryScore := eval.evaluateConsistency(contradictory)

	if contradictoryScore >= consistentScore {
		t.Errorf("contradictory output (%f) should score lower than consistent (%f)",
			contradictoryScore, consistentScore)
	}
}

func TestQualityEvaluator_Evaluate(t *testing.T) {
	eval := NewQualityEvaluator()

	output := `
	Summary: The codebase is well organized.
	Analysis: File structure follows standard patterns.
	Claims: The code is secure and maintainable.
	Risks: No critical vulnerabilities identified.
	Recommendations: Should add more unit tests. Consider refactoring the main function.
	`

	score := eval.Evaluate(output)

	if score < 0 || score > 1 {
		t.Errorf("score out of range: %f", score)
	}
}

func TestHashPrompt(t *testing.T) {
	hash1 := hashPrompt("test prompt 1")
	hash2 := hashPrompt("test prompt 2")
	hash1Again := hashPrompt("test prompt 1")

	if hash1 == hash2 {
		t.Error("different prompts should have different hashes")
	}

	if hash1 != hash1Again {
		t.Error("same prompt should have same hash")
	}

	if len(hash1) != 9 { // "p" + 8 hex chars
		t.Errorf("expected hash length 9, got %d", len(hash1))
	}
}

func TestCalculateSummary(t *testing.T) {
	runner := NewExperimentRunner("testdata/output")

	runs := []RunResult{
		{Mode: "multi", Threshold: 0.75, ConsensusScore: 0.8, QualityScore: 0.7, CostUSD: 0.05, V3Invoked: false},
		{Mode: "multi", Threshold: 0.75, ConsensusScore: 0.85, QualityScore: 0.75, CostUSD: 0.06, V3Invoked: true},
		{Mode: "single", Agent: "claude", QualityScore: 0.65, CostUSD: 0.02},
	}

	summary := runner.calculateSummary(runs)

	if summary.TotalRuns != 3 {
		t.Errorf("expected 3 total runs, got %d", summary.TotalRuns)
	}

	if summary.TotalCostUSD != 0.13 {
		t.Errorf("expected total cost 0.13, got %f", summary.TotalCostUSD)
	}
}

func TestGenerateReport(t *testing.T) {
	result := &ExperimentResult{
		Config: ExperimentConfig{Name: "Test"},
		Summary: Summary{
			TotalRuns:    10,
			TotalCostUSD: 1.50,
			ThresholdAnalysis: []ThresholdStats{
				{Threshold: 0.75, V3Rate: 0.2, AvgConsensus: 0.8, AvgQuality: 0.7, AvgCost: 0.05},
			},
			ModeComparison: ModeComparison{
				SingleAgent:        ModeStats{AvgQuality: 0.65, AvgCost: 0.02, SuccessRate: 1.0},
				MultiAgent:         ModeStats{AvgQuality: 0.75, AvgCost: 0.05, SuccessRate: 1.0},
				QualityImprovement: 15.4,
				CostIncrease:       150.0,
			},
		},
	}

	var buf bytes.Buffer
	if err := GenerateReport(result, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report := buf.String()

	if len(report) == 0 {
		t.Error("expected non-empty report")
	}

	// Check for key sections
	expectedSections := []string{
		"POC Validation Report",
		"Executive Summary",
		"Threshold Sensitivity",
		"Mode Comparison",
		"Conclusions",
	}

	for _, section := range expectedSections {
		if !bytes.Contains([]byte(report), []byte(section)) {
			t.Errorf("report missing section: %s", section)
		}
	}
}

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	total := weights.Completeness + weights.Specificity + weights.Actionable + weights.Consistency
	if total != 1.0 {
		t.Errorf("weights should sum to 1.0, got %f", total)
	}
}
