package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config configures the report writer
type Config struct {
	BaseDir    string // default: ".quorum-output"
	UseUTC     bool   // default: true
	IncludeRaw bool   // include raw JSON output in reports
	Enabled    bool   // whether to write reports
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		BaseDir:    ".quorum-output",
		UseUTC:     true,
		IncludeRaw: true,
		Enabled:    true,
	}
}

// WorkflowReportWriter manages report writing for an entire workflow execution
type WorkflowReportWriter struct {
	mu           sync.Mutex
	config       Config
	executionDir string // "20240115-143000-wf-abc123"
	workflowID   string
	startTime    time.Time
	initialized  bool
}

// NewWorkflowReportWriter creates a writer for a new workflow execution
func NewWorkflowReportWriter(cfg Config, workflowID string) *WorkflowReportWriter {
	now := time.Now()
	if cfg.UseUTC {
		now = now.UTC()
	}

	executionDir := fmt.Sprintf("%s-%s",
		now.Format("20060102-150405"),
		workflowID)

	return &WorkflowReportWriter{
		config:       cfg,
		executionDir: executionDir,
		workflowID:   workflowID,
		startTime:    now,
		initialized:  false,
	}
}

// Initialize creates the directory structure (called lazily on first write)
func (w *WorkflowReportWriter) Initialize() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initialized || !w.config.Enabled {
		return nil
	}

	// Create all necessary directories
	dirs := []string{
		w.ExecutionPath(),
		w.AnalyzePhasePath(),
		filepath.Join(w.AnalyzePhasePath(), "v1"),
		filepath.Join(w.AnalyzePhasePath(), "v2"),
		filepath.Join(w.AnalyzePhasePath(), "v3"),
		filepath.Join(w.AnalyzePhasePath(), "consensus"),
		w.PlanPhasePath(),
		filepath.Join(w.PlanPhasePath(), "v1"),
		filepath.Join(w.PlanPhasePath(), "consensus"),
		w.ExecutePhasePath(),
		filepath.Join(w.ExecutePhasePath(), "tasks"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	w.initialized = true
	return nil
}

// Path helpers

// ExecutionPath returns the root path for this execution
func (w *WorkflowReportWriter) ExecutionPath() string {
	return filepath.Join(w.config.BaseDir, w.executionDir)
}

// AnalyzePhasePath returns the analyze-phase directory path
func (w *WorkflowReportWriter) AnalyzePhasePath() string {
	return filepath.Join(w.ExecutionPath(), "analyze-phase")
}

// PlanPhasePath returns the plan-phase directory path
func (w *WorkflowReportWriter) PlanPhasePath() string {
	return filepath.Join(w.ExecutionPath(), "plan-phase")
}

// ExecutePhasePath returns the execute-phase directory path
func (w *WorkflowReportWriter) ExecutePhasePath() string {
	return filepath.Join(w.ExecutionPath(), "execute-phase")
}

// IsEnabled returns whether report writing is enabled
func (w *WorkflowReportWriter) IsEnabled() bool {
	return w.config.Enabled
}

// GetExecutionDir returns the execution directory name
func (w *WorkflowReportWriter) GetExecutionDir() string {
	return w.executionDir
}

// ========================================
// Analyze Phase Writers
// ========================================

// PromptMetrics contains metrics about prompt processing
type PromptMetrics struct {
	OriginalCharCount   int
	OptimizedCharCount  int
	ImprovementRatio    float64
	TokensUsed          int
	CostUSD             float64
	DurationMS          int64
	OptimizerAgent      string
	OptimizerModel      string
}

// WriteOriginalPrompt writes the original user prompt
func (w *WorkflowReportWriter) WriteOriginalPrompt(prompt string) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.AnalyzePhasePath(), "00-original-prompt.md")

	fm := NewFrontmatter()
	fm.Set("type", "original_prompt")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("char_count", len(prompt))
	fm.Set("word_count", countWords(prompt))

	content := "# Prompt Original\n\n## Contenido\n\n" + prompt + "\n"

	return w.writeFile(path, fm, content)
}

// WriteOptimizedPrompt writes the optimized prompt
func (w *WorkflowReportWriter) WriteOptimizedPrompt(original, optimized string, metrics PromptMetrics) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.AnalyzePhasePath(), "01-optimized-prompt.md")

	fm := NewFrontmatter()
	fm.Set("type", "optimized_prompt")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("optimizer_agent", metrics.OptimizerAgent)
	fm.Set("optimizer_model", metrics.OptimizerModel)
	fm.Set("original_char_count", metrics.OriginalCharCount)
	fm.Set("optimized_char_count", metrics.OptimizedCharCount)
	fm.Set("improvement_ratio", fmt.Sprintf("%.2f", metrics.ImprovementRatio))
	fm.Set("tokens_used", metrics.TokensUsed)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", metrics.CostUSD))
	fm.Set("duration_ms", metrics.DurationMS)

	content := renderOptimizedPromptTemplate(original, optimized, metrics)

	return w.writeFile(path, fm, content)
}

// AnalysisData contains data for an analysis report
type AnalysisData struct {
	AgentName       string
	Model           string
	RawOutput       string
	Claims          []string
	Risks           []string
	Recommendations []string
	TokensIn        int
	TokensOut       int
	CostUSD         float64
	DurationMS      int64
}

// WriteV1Analysis writes a V1 analysis report
func (w *WorkflowReportWriter) WriteV1Analysis(data AnalysisData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%s.md", data.AgentName, sanitizeFilename(data.Model))
	path := filepath.Join(w.AnalyzePhasePath(), "v1", filename)

	fm := NewFrontmatter()
	fm.Set("type", "analysis")
	fm.Set("version", "v1")
	fm.Set("agent", data.AgentName)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", data.CostUSD))
	fm.Set("duration_ms", data.DurationMS)

	content := renderAnalysisTemplate(data, "v1", w.config.IncludeRaw)

	return w.writeFile(path, fm, content)
}

// CritiqueData contains data for a V2 critique report
type CritiqueData struct {
	CriticAgent     string
	CriticModel     string
	TargetAgent     string
	RawOutput       string
	Agreements      []string
	Disagreements   []string
	AdditionalRisks []string
	TokensIn        int
	TokensOut       int
	CostUSD         float64
	DurationMS      int64
}

// WriteV2Critique writes a V2 critique report
func (w *WorkflowReportWriter) WriteV2Critique(data CritiqueData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-critiques-%s.md", data.CriticAgent, data.TargetAgent)
	path := filepath.Join(w.AnalyzePhasePath(), "v2", filename)

	fm := NewFrontmatter()
	fm.Set("type", "critique")
	fm.Set("version", "v2")
	fm.Set("critic_agent", data.CriticAgent)
	fm.Set("critic_model", data.CriticModel)
	fm.Set("target_agent", data.TargetAgent)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", data.CostUSD))
	fm.Set("duration_ms", data.DurationMS)

	content := renderCritiqueTemplate(data, w.config.IncludeRaw)

	return w.writeFile(path, fm, content)
}

// ReconciliationData contains data for V3 reconciliation
type ReconciliationData struct {
	Agent       string
	Model       string
	RawOutput   string
	TokensIn    int
	TokensOut   int
	CostUSD     float64
	DurationMS  int64
}

// WriteV3Reconciliation writes a V3 reconciliation report
func (w *WorkflowReportWriter) WriteV3Reconciliation(data ReconciliationData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-reconciliation.md", data.Agent)
	path := filepath.Join(w.AnalyzePhasePath(), "v3", filename)

	fm := NewFrontmatter()
	fm.Set("type", "reconciliation")
	fm.Set("version", "v3")
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", data.CostUSD))
	fm.Set("duration_ms", data.DurationMS)

	content := renderReconciliationTemplate(data, w.config.IncludeRaw)

	return w.writeFile(path, fm, content)
}

// ConsensusData contains consensus evaluation data
type ConsensusData struct {
	Score             float64
	Threshold         float64
	NeedsEscalation   bool
	NeedsHumanReview  bool
	AgentsCount       int
	ClaimsScore       float64
	RisksScore        float64
	RecommendationsScore float64
	Divergences       []DivergenceData
}

// DivergenceData represents a divergence between agents
type DivergenceData struct {
	Type        string
	Agent1      string
	Agent2      string
	Description string
}

// WriteConsensusReport writes a consensus report
func (w *WorkflowReportWriter) WriteConsensusReport(data ConsensusData, afterPhase string) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("after-%s.md", afterPhase)
	path := filepath.Join(w.AnalyzePhasePath(), "consensus", filename)

	fm := NewFrontmatter()
	fm.Set("type", "consensus")
	fm.Set("after_phase", afterPhase)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("score", fmt.Sprintf("%.4f", data.Score))
	fm.Set("threshold", fmt.Sprintf("%.4f", data.Threshold))
	fm.Set("needs_escalation", data.NeedsEscalation)
	fm.Set("needs_human_review", data.NeedsHumanReview)
	fm.Set("agents_count", data.AgentsCount)

	content := renderConsensusTemplate(data, afterPhase)

	return w.writeFile(path, fm, content)
}

// ConsolidationData contains data for consolidated analysis
type ConsolidationData struct {
	Agent           string
	Model           string
	Content         string
	AnalysesCount   int
	Synthesized     bool
	ConsensusScore  float64
	TotalTokensIn   int
	TotalTokensOut  int
	TotalCostUSD    float64
	TotalDurationMS int64
}

// WriteConsolidatedAnalysis writes the consolidated analysis
func (w *WorkflowReportWriter) WriteConsolidatedAnalysis(data ConsolidationData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.AnalyzePhasePath(), "consolidated.md")

	fm := NewFrontmatter()
	fm.Set("type", "consolidated")
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("analyses_count", data.AnalysesCount)
	fm.Set("synthesized", data.Synthesized)
	fm.Set("consensus_score", fmt.Sprintf("%.4f", data.ConsensusScore))
	fm.Set("total_tokens_in", data.TotalTokensIn)
	fm.Set("total_tokens_out", data.TotalTokensOut)
	fm.Set("total_cost_usd", fmt.Sprintf("%.4f", data.TotalCostUSD))
	fm.Set("total_duration_ms", data.TotalDurationMS)

	content := renderConsolidationTemplate(data)

	return w.writeFile(path, fm, content)
}

// ========================================
// Plan Phase Writers
// ========================================

// PlanData contains data for a plan report
type PlanData struct {
	Agent      string
	Model      string
	Content    string
	TokensIn   int
	TokensOut  int
	CostUSD    float64
	DurationMS int64
}

// WritePlan writes a plan report
func (w *WorkflowReportWriter) WritePlan(data PlanData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-plan.md", data.Agent)
	path := filepath.Join(w.PlanPhasePath(), "v1", filename)

	fm := NewFrontmatter()
	fm.Set("type", "plan")
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", data.CostUSD))
	fm.Set("duration_ms", data.DurationMS)

	content := renderPlanTemplate(data)

	return w.writeFile(path, fm, content)
}

// WriteFinalPlan writes the final approved plan
func (w *WorkflowReportWriter) WriteFinalPlan(content string) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.PlanPhasePath(), "final-plan.md")

	fm := NewFrontmatter()
	fm.Set("type", "final_plan")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)

	mdContent := "# Plan Final Aprobado\n\n" + content + "\n"

	return w.writeFile(path, fm, mdContent)
}

// ========================================
// Execute Phase Writers
// ========================================

// TaskResultData contains data for a task execution result
type TaskResultData struct {
	TaskID      string
	TaskName    string
	Agent       string
	Model       string
	Status      string // "completed", "failed", "skipped"
	Output      string
	Error       string
	TokensIn    int
	TokensOut   int
	CostUSD     float64
	DurationMS  int64
}

// WriteTaskResult writes a task execution result
func (w *WorkflowReportWriter) WriteTaskResult(data TaskResultData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%s.md", data.TaskID, sanitizeFilename(data.TaskName))
	path := filepath.Join(w.ExecutePhasePath(), "tasks", filename)

	fm := NewFrontmatter()
	fm.Set("type", "task_result")
	fm.Set("task_id", data.TaskID)
	fm.Set("task_name", data.TaskName)
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("status", data.Status)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("cost_usd", fmt.Sprintf("%.4f", data.CostUSD))
	fm.Set("duration_ms", data.DurationMS)

	content := renderTaskResultTemplate(data)

	return w.writeFile(path, fm, content)
}

// ExecutionSummaryData contains summary of task execution
type ExecutionSummaryData struct {
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	SkippedTasks   int
	TotalTokensIn  int
	TotalTokensOut int
	TotalCostUSD   float64
	TotalDurationMS int64
	Tasks          []TaskResultData
}

// WriteExecutionSummary writes the execution summary
func (w *WorkflowReportWriter) WriteExecutionSummary(data ExecutionSummaryData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.ExecutePhasePath(), "execution-summary.md")

	fm := NewFrontmatter()
	fm.Set("type", "execution_summary")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("total_tasks", data.TotalTasks)
	fm.Set("completed_tasks", data.CompletedTasks)
	fm.Set("failed_tasks", data.FailedTasks)
	fm.Set("skipped_tasks", data.SkippedTasks)
	fm.Set("total_cost_usd", fmt.Sprintf("%.4f", data.TotalCostUSD))
	fm.Set("total_duration_ms", data.TotalDurationMS)

	content := renderExecutionSummaryTemplate(data)

	return w.writeFile(path, fm, content)
}

// ========================================
// Metadata & Summary Writers
// ========================================

// WorkflowMetadata contains workflow-level metadata
type WorkflowMetadata struct {
	WorkflowID     string
	StartedAt      time.Time
	CompletedAt    time.Time
	Status         string
	PhasesExecuted []string
	TotalCostUSD   float64
	TotalTokensIn  int
	TotalTokensOut int
	ConsensusScore float64
	AgentsUsed     []string
}

// WriteMetadata writes the workflow metadata file
func (w *WorkflowReportWriter) WriteMetadata(data WorkflowMetadata) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.ExecutionPath(), "metadata.md")

	fm := NewFrontmatter()
	fm.Set("workflow_id", data.WorkflowID)
	fm.Set("started_at", w.formatTime(data.StartedAt))
	fm.Set("completed_at", w.formatTime(data.CompletedAt))
	fm.Set("status", data.Status)
	fm.Set("total_cost_usd", fmt.Sprintf("%.4f", data.TotalCostUSD))
	fm.Set("consensus_score", fmt.Sprintf("%.4f", data.ConsensusScore))

	content := renderMetadataTemplate(data)

	return w.writeFile(path, fm, content)
}

// WriteWorkflowSummary writes the final workflow summary
func (w *WorkflowReportWriter) WriteWorkflowSummary(data WorkflowMetadata) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.ExecutionPath(), "workflow-summary.md")

	fm := NewFrontmatter()
	fm.Set("type", "workflow_summary")
	fm.Set("workflow_id", data.WorkflowID)
	fm.Set("timestamp", w.formatTime(time.Now()))

	content := renderWorkflowSummaryTemplate(data)

	return w.writeFile(path, fm, content)
}

// ========================================
// Internal helpers
// ========================================

func (w *WorkflowReportWriter) formatTime(t time.Time) string {
	if w.config.UseUTC {
		t = t.UTC()
	}
	return t.Format(time.RFC3339)
}

func (w *WorkflowReportWriter) writeFile(path string, fm *Frontmatter, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer file.Close()

	// Write frontmatter
	if _, err := file.WriteString(fm.Render()); err != nil {
		return fmt.Errorf("writing frontmatter: %w", err)
	}

	// Write content
	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("writing content: %w", err)
	}

	return nil
}
