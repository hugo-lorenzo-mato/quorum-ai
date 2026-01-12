package validation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExperimentConfig defines a validation experiment.
type ExperimentConfig struct {
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Prompts            []string  `json:"prompts"`
	Thresholds         []float64 `json:"thresholds"`
	Agents             []string  `json:"agents"`
	Iterations         int       `json:"iterations"`
	SingleAgentControl bool      `json:"single_agent_control"`
}

// ExperimentResult captures the outcome of an experiment.
type ExperimentResult struct {
	Config        ExperimentConfig `json:"config"`
	Runs          []RunResult      `json:"runs"`
	Summary       Summary          `json:"summary"`
	StartedAt     time.Time        `json:"started_at"`
	CompletedAt   time.Time        `json:"completed_at"`
	TotalDuration time.Duration    `json:"total_duration"`
}

// RunResult captures a single run outcome.
type RunResult struct {
	PromptID       string        `json:"prompt_id"`
	Threshold      float64       `json:"threshold"`
	Mode           string        `json:"mode"` // "single" or "multi"
	Agent          string        `json:"agent,omitempty"`
	ConsensusScore float64       `json:"consensus_score,omitempty"`
	V3Invoked      bool          `json:"v3_invoked"`
	TokensIn       int           `json:"tokens_in"`
	TokensOut      int           `json:"tokens_out"`
	CostUSD        float64       `json:"cost_usd"`
	Duration       time.Duration `json:"duration"`
	QualityScore   float64       `json:"quality_score"`
	Errors         []string      `json:"errors,omitempty"`
}

// Summary aggregates experiment statistics.
type Summary struct {
	TotalRuns         int              `json:"total_runs"`
	SuccessRate       float64          `json:"success_rate"`
	AvgConsensusScore float64          `json:"avg_consensus_score"`
	V3InvocationRate  float64          `json:"v3_invocation_rate"`
	AvgCostUSD        float64          `json:"avg_cost_usd"`
	TotalCostUSD      float64          `json:"total_cost_usd"`
	ThresholdAnalysis []ThresholdStats `json:"threshold_analysis"`
	ModeComparison    ModeComparison   `json:"mode_comparison"`
}

// ThresholdStats captures per-threshold statistics.
type ThresholdStats struct {
	Threshold    float64 `json:"threshold"`
	V3Rate       float64 `json:"v3_rate"`
	AvgConsensus float64 `json:"avg_consensus"`
	AvgQuality   float64 `json:"avg_quality"`
	AvgCost      float64 `json:"avg_cost"`
}

// ModeComparison compares single vs multi-agent modes.
type ModeComparison struct {
	SingleAgent        ModeStats `json:"single_agent"`
	MultiAgent         ModeStats `json:"multi_agent"`
	QualityImprovement float64   `json:"quality_improvement_percent"`
	CostIncrease       float64   `json:"cost_increase_percent"`
}

// ModeStats captures statistics for a mode.
type ModeStats struct {
	AvgQuality  float64 `json:"avg_quality"`
	AvgCost     float64 `json:"avg_cost"`
	SuccessRate float64 `json:"success_rate"`
}

// ExperimentRunner executes validation experiments.
type ExperimentRunner struct {
	evaluator *QualityEvaluator
	logger    *slog.Logger
	outputDir string
	dryRun    bool
}

// NewExperimentRunner creates a new experiment runner.
func NewExperimentRunner(outputDir string) *ExperimentRunner {
	return &ExperimentRunner{
		evaluator: NewQualityEvaluator(),
		logger:    slog.Default(),
		outputDir: outputDir,
	}
}

// SetDryRun enables dry-run mode for testing.
func (e *ExperimentRunner) SetDryRun(dryRun bool) {
	e.dryRun = dryRun
}

// LoadConfig loads an experiment configuration from file.
func LoadConfig(path string) (*ExperimentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg ExperimentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// Run executes a complete experiment.
func (e *ExperimentRunner) Run(ctx context.Context, cfg ExperimentConfig) (*ExperimentResult, error) {
	result := &ExperimentResult{
		Config:    cfg,
		StartedAt: time.Now(),
		Runs:      make([]RunResult, 0),
	}

	e.logger.Info("starting experiment",
		slog.String("name", cfg.Name),
		slog.Int("prompts", len(cfg.Prompts)),
		slog.Int("thresholds", len(cfg.Thresholds)),
		slog.Int("iterations", cfg.Iterations))

	// Run multi-agent experiments for each threshold
	for _, threshold := range cfg.Thresholds {
		for _, prompt := range cfg.Prompts {
			for i := 0; i < cfg.Iterations; i++ {
				run := e.runMultiAgent(ctx, prompt, threshold)
				result.Runs = append(result.Runs, run)
			}
		}
	}

	// Run single-agent control experiments
	if cfg.SingleAgentControl {
		for _, agent := range cfg.Agents {
			for _, prompt := range cfg.Prompts {
				for i := 0; i < cfg.Iterations; i++ {
					run := e.runSingleAgent(ctx, prompt, agent)
					result.Runs = append(result.Runs, run)
				}
			}
		}
	}

	result.CompletedAt = time.Now()
	result.TotalDuration = result.CompletedAt.Sub(result.StartedAt)
	result.Summary = e.calculateSummary(result.Runs)

	return result, nil
}

func (e *ExperimentRunner) runMultiAgent(ctx context.Context, prompt string, threshold float64) RunResult {
	start := time.Now()

	run := RunResult{
		PromptID:  hashPrompt(prompt),
		Threshold: threshold,
		Mode:      "multi",
	}

	if e.dryRun {
		// Simulated results for dry-run
		run.ConsensusScore = 0.85
		run.V3Invoked = threshold > 0.80
		run.TokensIn = 1000
		run.TokensOut = 500
		run.CostUSD = 0.05
		run.QualityScore = 0.82
	} else {
		// Real execution would go here
		run.Errors = append(run.Errors, "not implemented: requires workflow runner integration")
	}

	run.Duration = time.Since(start)
	return run
}

func (e *ExperimentRunner) runSingleAgent(ctx context.Context, prompt, agent string) RunResult {
	start := time.Now()

	run := RunResult{
		PromptID: hashPrompt(prompt),
		Mode:     "single",
		Agent:    agent,
	}

	if e.dryRun {
		// Simulated results for dry-run
		run.TokensIn = 800
		run.TokensOut = 400
		run.CostUSD = 0.02
		run.QualityScore = 0.75
	} else {
		// Real execution would go here
		run.Errors = append(run.Errors, "not implemented: requires agent integration")
	}

	run.Duration = time.Since(start)
	return run
}

func (e *ExperimentRunner) calculateSummary(runs []RunResult) Summary {
	summary := Summary{}

	var multiRuns, singleRuns []RunResult
	thresholdMap := make(map[float64][]RunResult)
	successCount := 0

	for _, run := range runs {
		summary.TotalRuns++
		summary.TotalCostUSD += run.CostUSD

		if len(run.Errors) == 0 {
			successCount++
			if run.Mode == "multi" {
				multiRuns = append(multiRuns, run)
				thresholdMap[run.Threshold] = append(thresholdMap[run.Threshold], run)
			} else {
				singleRuns = append(singleRuns, run)
			}
		}
	}

	if summary.TotalRuns > 0 {
		summary.SuccessRate = float64(successCount) / float64(summary.TotalRuns)
		summary.AvgCostUSD = summary.TotalCostUSD / float64(summary.TotalRuns)
	}

	// Calculate threshold analysis
	for threshold, truns := range thresholdMap {
		stats := ThresholdStats{Threshold: threshold}
		var v3Count int
		for _, r := range truns {
			stats.AvgConsensus += r.ConsensusScore
			stats.AvgQuality += r.QualityScore
			stats.AvgCost += r.CostUSD
			if r.V3Invoked {
				v3Count++
			}
		}
		n := float64(len(truns))
		if n > 0 {
			stats.AvgConsensus /= n
			stats.AvgQuality /= n
			stats.AvgCost /= n
			stats.V3Rate = float64(v3Count) / n
		}
		summary.ThresholdAnalysis = append(summary.ThresholdAnalysis, stats)
	}

	// Calculate mode comparison
	if len(multiRuns) > 0 {
		for _, r := range multiRuns {
			summary.ModeComparison.MultiAgent.AvgQuality += r.QualityScore
			summary.ModeComparison.MultiAgent.AvgCost += r.CostUSD
			summary.AvgConsensusScore += r.ConsensusScore
			if r.V3Invoked {
				summary.V3InvocationRate++
			}
		}
		n := float64(len(multiRuns))
		summary.ModeComparison.MultiAgent.AvgQuality /= n
		summary.ModeComparison.MultiAgent.AvgCost /= n
		summary.AvgConsensusScore /= n
		summary.V3InvocationRate /= n
		summary.ModeComparison.MultiAgent.SuccessRate = n / n
	}

	if len(singleRuns) > 0 {
		for _, r := range singleRuns {
			summary.ModeComparison.SingleAgent.AvgQuality += r.QualityScore
			summary.ModeComparison.SingleAgent.AvgCost += r.CostUSD
		}
		n := float64(len(singleRuns))
		summary.ModeComparison.SingleAgent.AvgQuality /= n
		summary.ModeComparison.SingleAgent.AvgCost /= n
		summary.ModeComparison.SingleAgent.SuccessRate = n / n
	}

	// Calculate improvements
	if summary.ModeComparison.SingleAgent.AvgQuality > 0 {
		summary.ModeComparison.QualityImprovement =
			((summary.ModeComparison.MultiAgent.AvgQuality - summary.ModeComparison.SingleAgent.AvgQuality) /
				summary.ModeComparison.SingleAgent.AvgQuality) * 100
	}
	if summary.ModeComparison.SingleAgent.AvgCost > 0 {
		summary.ModeComparison.CostIncrease =
			((summary.ModeComparison.MultiAgent.AvgCost - summary.ModeComparison.SingleAgent.AvgCost) /
				summary.ModeComparison.SingleAgent.AvgCost) * 100
	}

	return summary
}

// SaveResult saves the experiment result to a file.
func SaveResult(result *ExperimentResult, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing result file: %w", err)
	}

	return nil
}

// GenerateReport generates a markdown report from results.
func GenerateReport(result *ExperimentResult, w io.Writer) error {
	fmt.Fprintf(w, "# POC Validation Report\n\n")
	fmt.Fprintf(w, "## Executive Summary\n\n")
	fmt.Fprintf(w, "- **Date**: %s\n", result.CompletedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "- **Duration**: %s\n", result.TotalDuration.Round(time.Second))
	fmt.Fprintf(w, "- **Total Runs**: %d\n", result.Summary.TotalRuns)
	fmt.Fprintf(w, "- **Total Cost**: $%.2f\n\n", result.Summary.TotalCostUSD)

	fmt.Fprintf(w, "## Hypothesis\n\n")
	fmt.Fprintf(w, "Multi-agent ensemble with consensus-based validation produces higher quality\n")
	fmt.Fprintf(w, "outputs than single-agent approaches, with acceptable cost overhead.\n\n")

	fmt.Fprintf(w, "## Results\n\n")

	fmt.Fprintf(w, "### Threshold Sensitivity\n\n")
	fmt.Fprintf(w, "| Threshold | V3 Rate | Avg Consensus | Avg Quality | Avg Cost |\n")
	fmt.Fprintf(w, "|-----------|---------|---------------|-------------|----------|\n")
	for _, stats := range result.Summary.ThresholdAnalysis {
		fmt.Fprintf(w, "| %.2f | %.1f%% | %.2f | %.2f | $%.3f |\n",
			stats.Threshold, stats.V3Rate*100, stats.AvgConsensus, stats.AvgQuality, stats.AvgCost)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "### Mode Comparison\n\n")
	fmt.Fprintf(w, "| Mode | Avg Quality | Avg Cost | Success Rate |\n")
	fmt.Fprintf(w, "|------|-------------|----------|------------|\n")
	fmt.Fprintf(w, "| Single Agent | %.2f | $%.3f | %.1f%% |\n",
		result.Summary.ModeComparison.SingleAgent.AvgQuality,
		result.Summary.ModeComparison.SingleAgent.AvgCost,
		result.Summary.ModeComparison.SingleAgent.SuccessRate*100)
	fmt.Fprintf(w, "| Multi Agent | %.2f | $%.3f | %.1f%% |\n\n",
		result.Summary.ModeComparison.MultiAgent.AvgQuality,
		result.Summary.ModeComparison.MultiAgent.AvgCost,
		result.Summary.ModeComparison.MultiAgent.SuccessRate*100)

	fmt.Fprintf(w, "**Quality Improvement**: %.1f%%\n", result.Summary.ModeComparison.QualityImprovement)
	fmt.Fprintf(w, "**Cost Increase**: %.1f%%\n\n", result.Summary.ModeComparison.CostIncrease)

	fmt.Fprintf(w, "## Conclusions\n\n")
	if result.Summary.ModeComparison.QualityImprovement > 10.0 {
		fmt.Fprintf(w, "The multi-agent approach shows significant quality improvement (>10%%) over\n")
		fmt.Fprintf(w, "single-agent baselines, validating the ensemble hypothesis.\n")
	} else if result.Summary.ModeComparison.QualityImprovement > 0.0 {
		fmt.Fprintf(w, "The multi-agent approach shows marginal quality improvement. Further\n")
		fmt.Fprintf(w, "optimization may be needed.\n")
	} else {
		fmt.Fprintf(w, "The multi-agent approach did not demonstrate quality improvement in this\n")
		fmt.Fprintf(w, "experiment. Review threshold settings and agent selection.\n")
	}

	return nil
}

func hashPrompt(prompt string) string {
	h := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("p%x", h[:4])
}

// QualityEvaluator scores output quality.
type QualityEvaluator struct {
	weights QualityWeights
}

// QualityWeights defines scoring weights.
type QualityWeights struct {
	Completeness float64
	Specificity  float64
	Actionable   float64
	Consistency  float64
}

// DefaultWeights returns standard quality weights.
func DefaultWeights() QualityWeights {
	return QualityWeights{
		Completeness: 0.25,
		Specificity:  0.30,
		Actionable:   0.30,
		Consistency:  0.15,
	}
}

// NewQualityEvaluator creates a new evaluator.
func NewQualityEvaluator() *QualityEvaluator {
	return &QualityEvaluator{
		weights: DefaultWeights(),
	}
}

// Evaluate scores output quality from 0.0 to 1.0.
func (e *QualityEvaluator) Evaluate(output string) float64 {
	completeness := e.evaluateCompleteness(output)
	specificity := e.evaluateSpecificity(output)
	actionable := e.evaluateActionable(output)
	consistency := e.evaluateConsistency(output)

	return completeness*e.weights.Completeness +
		specificity*e.weights.Specificity +
		actionable*e.weights.Actionable +
		consistency*e.weights.Consistency
}

func (e *QualityEvaluator) evaluateCompleteness(output string) float64 {
	requiredSections := []string{
		"claims", "risks", "recommendations",
		"summary", "analysis",
	}

	lower := strings.ToLower(output)
	found := 0
	for _, section := range requiredSections {
		if strings.Contains(lower, section) {
			found++
		}
	}

	return float64(found) / float64(len(requiredSections))
}

func (e *QualityEvaluator) evaluateSpecificity(output string) float64 {
	vagueTerms := []string{
		"might", "could", "possibly", "maybe",
		"some", "various", "general", "etc",
	}
	specificIndicators := []string{
		"specifically", "exactly", "precisely",
		"line", "file", "function", "class",
	}

	lower := strings.ToLower(output)
	vagueCount := 0
	specificCount := 0

	for _, term := range vagueTerms {
		vagueCount += strings.Count(lower, term)
	}
	for _, term := range specificIndicators {
		specificCount += strings.Count(lower, term)
	}

	if vagueCount+specificCount == 0 {
		return 0.5
	}

	return float64(specificCount) / float64(vagueCount+specificCount)
}

func (e *QualityEvaluator) evaluateActionable(output string) float64 {
	actionableIndicators := []string{
		"should", "must", "recommend", "suggest",
		"add", "remove", "change", "update",
		"refactor", "implement", "fix", "replace",
	}

	lower := strings.ToLower(output)
	count := 0
	for _, indicator := range actionableIndicators {
		count += strings.Count(lower, indicator)
	}

	normalized := float64(count) / 10.0
	if normalized > 1.0 {
		normalized = 1.0
	}

	return normalized
}

func (e *QualityEvaluator) evaluateConsistency(output string) float64 {
	contradictionPairs := [][]string{
		{"good", "bad"},
		{"safe", "unsafe"},
		{"recommended", "not recommended"},
	}

	lower := strings.ToLower(output)
	contradictions := 0

	for _, pair := range contradictionPairs {
		if strings.Contains(lower, pair[0]) && strings.Contains(lower, pair[1]) {
			contradictions++
		}
	}

	if contradictions == 0 {
		return 1.0
	}

	return 1.0 - (float64(contradictions) * 0.2)
}
