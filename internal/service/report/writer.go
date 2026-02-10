package report

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config configures the report writer
type Config struct {
	BaseDir    string // default: ".quorum/runs"
	UseUTC     bool   // default: true
	IncludeRaw bool   // include raw JSON output in reports
	Enabled    bool   // whether to write reports
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		BaseDir:    ".quorum/runs",
		UseUTC:     true,
		IncludeRaw: true,
		Enabled:    true,
	}
}

// WorkflowReportWriter manages report writing for an entire workflow execution
type WorkflowReportWriter struct {
	mu           sync.Mutex
	config       Config
	executionDir string // e.g., "wf-20250121-153045-k7m9p"
	workflowID   string
	startTime    time.Time
	initialized  bool
}

// NewWorkflowReportWriter creates a writer for a new workflow execution.
// The execution directory is named after the workflowID directly,
// since the ID already contains a timestamp (e.g., wf-20250121-153045-k7m9p).
func NewWorkflowReportWriter(cfg Config, workflowID string) *WorkflowReportWriter {
	now := time.Now()
	if cfg.UseUTC {
		now = now.UTC()
	}

	return &WorkflowReportWriter{
		config:       cfg,
		executionDir: workflowID,
		workflowID:   workflowID,
		startTime:    now,
		initialized:  false,
	}
}

// ResumeWorkflowReportWriter creates a writer for resuming an existing workflow execution.
// It reuses the original report directory path instead of creating a new one.
func ResumeWorkflowReportWriter(cfg Config, workflowID, existingReportPath string) *WorkflowReportWriter {
	// Extract just the execution directory name from the full path
	// existingReportPath is like ".quorum/runs/wf-xxx"
	executionDir := filepath.Base(existingReportPath)

	return &WorkflowReportWriter{
		config:       cfg,
		executionDir: executionDir,
		workflowID:   workflowID,
		startTime:    time.Now(),
		// Keep initialization lazy even on resume.
		// The base execution directory may exist without phase subdirectories (e.g. created by API),
		// and MkdirAll() in Initialize() is idempotent anyway.
		initialized: false,
	}
}

// ExecutionDir returns the execution directory name (for persisting in state)
func (w *WorkflowReportWriter) ExecutionDir() string {
	return w.executionDir
}

// Initialize creates the base directory structure (called lazily on first write)
// Versioned directories (v1, v2, v3, etc.) are created on demand when writing files
func (w *WorkflowReportWriter) Initialize() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.initialized || !w.config.Enabled {
		return nil
	}

	// Create only base directories - version directories (v1, v2, etc.) are created on demand
	dirs := []string{
		w.ExecutionPath(),
		w.AnalyzePhasePath(),
		filepath.Join(w.AnalyzePhasePath(), "consensus"),
		w.PlanPhasePath(),
		filepath.Join(w.PlanPhasePath(), "consensus"),
		w.ExecutePhasePath(),
		filepath.Join(w.ExecutePhasePath(), "tasks"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
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

// TaskOutputPath returns the path for saving large task outputs.
// Creates the outputs directory if it doesn't exist.
func (w *WorkflowReportWriter) TaskOutputPath(taskID string) string {
	outputsDir := filepath.Join(w.ExecutePhasePath(), "outputs")
	_ = os.MkdirAll(outputsDir, 0o750)
	return filepath.Join(outputsDir, taskID+".md")
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
// Path Getters (for LLM-directed file writing)
// ========================================

// V1AnalysisPath returns the path where the LLM should write a V1 analysis
func (w *WorkflowReportWriter) V1AnalysisPath(agentName, model string) string {
	return filepath.Join(w.AnalyzePhasePath(), "v1", analysisFilename(agentName, model))
}

// VnAnalysisPath returns the path where the LLM should write a V(n) analysis
func (w *WorkflowReportWriter) VnAnalysisPath(agentName, model string, round int) string {
	return filepath.Join(w.AnalyzePhasePath(), fmt.Sprintf("v%d", round), analysisFilename(agentName, model))
}

// SingleAgentAnalysisPath returns the path where the LLM should write a single-agent analysis
func (w *WorkflowReportWriter) SingleAgentAnalysisPath(agentName, model string) string {
	return filepath.Join(w.AnalyzePhasePath(), "single-agent", analysisFilename(agentName, model))
}

// ConsolidatedAnalysisPath returns the path where the LLM should write the consolidated analysis
func (w *WorkflowReportWriter) ConsolidatedAnalysisPath() string {
	return filepath.Join(w.AnalyzePhasePath(), "consolidated.md")
}

// OptimizedPromptPath returns the path where the LLM should write the optimized prompt
func (w *WorkflowReportWriter) OptimizedPromptPath() string {
	return filepath.Join(w.AnalyzePhasePath(), "01-optimized-prompt.md")
}

// ModeratorReportPath returns the path where the LLM should write the moderator evaluation report
func (w *WorkflowReportWriter) ModeratorReportPath(round int) string {
	return filepath.Join(w.AnalyzePhasePath(), "consensus", fmt.Sprintf("round-%d.md", round))
}

// ModeratorAttemptPath returns path for a specific moderator attempt.
// Each attempt by a moderator (primary or fallback) writes to its own file for traceability.
func (w *WorkflowReportWriter) ModeratorAttemptPath(round, attempt int, agentName string) string {
	return filepath.Join(w.AnalyzePhasePath(), "consensus", "attempts",
		fmt.Sprintf("round-%d", round),
		fmt.Sprintf("attempt-%d-%s.md", attempt, agentName))
}

// PromoteModeratorAttempt copies a successful attempt to the official location.
// This ensures the official round-X.md file only exists when validation passes.
func (w *WorkflowReportWriter) PromoteModeratorAttempt(round, attempt int, agentName string) error {
	src := w.ModeratorAttemptPath(round, attempt, agentName)
	dst := w.ModeratorReportPath(round)

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("creating consensus directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening attempt file %s: %w", src, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating promoted file %s: %w", dst, err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying attempt to promoted location: %w", err)
	}

	return nil
}

// ========================================
// Analyze Phase Writers
// ========================================

// PromptMetrics contains metrics about prompt processing
type PromptMetrics struct {
	OriginalCharCount  int
	OptimizedCharCount int
	ImprovementRatio   float64
	TokensUsed         int
	DurationMS         int64
	OptimizerAgent     string
	OptimizerModel     string
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

// WriteRefinedPrompt writes the refined prompt (raw content only, no metadata)
func (w *WorkflowReportWriter) WriteRefinedPrompt(_, refined string, _ PromptMetrics) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.AnalyzePhasePath(), "01-refined-prompt.md")

	// Write only the refined prompt content, no frontmatter or metadata
	return w.writeFileRaw(path, refined+"\n")
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
	DurationMS      int64
}

// WriteV1Analysis writes a V1 analysis report (raw LLM output only, no metadata)
func (w *WorkflowReportWriter) WriteV1Analysis(data AnalysisData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	// Create v1 directory if needed (lazy creation)
	v1Dir := filepath.Join(w.AnalyzePhasePath(), "v1")
	if err := os.MkdirAll(v1Dir, 0o750); err != nil {
		return fmt.Errorf("creating v1 directory: %w", err)
	}

	path := filepath.Join(v1Dir, analysisFilename(data.AgentName, data.Model))

	// Write only the raw LLM output, no frontmatter or metadata
	return w.writeFileRaw(path, data.RawOutput+"\n")
}

// ConsensusData contains consensus evaluation data
type ConsensusData struct {
	Score                float64
	Threshold            float64
	NeedsEscalation      bool
	NeedsHumanReview     bool
	AgentsCount          int
	ClaimsScore          float64
	RisksScore           float64
	RecommendationsScore float64
	Divergences          []DivergenceData
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

	content := renderConsensusReport(data, afterPhase)

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
	TotalDurationMS int64
}

// ModeratorData contains data for a semantic moderator evaluation report
type ModeratorData struct {
	Agent            string
	Model            string
	Round            int
	Score            float64
	RawOutput        string
	AgreementsCount  int
	DivergencesCount int
	TokensIn         int
	TokensOut        int
	DurationMS       int64
}

// WriteModeratorReport writes a semantic moderator evaluation report with metadata.
// This writes to the same location as ModeratorReportPath (consensus/round-X.md)
// but includes structured metadata in the frontmatter.
func (w *WorkflowReportWriter) WriteModeratorReport(data ModeratorData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	// Use the same path as ModeratorReportPath for consistency
	path := w.ModeratorReportPath(data.Round)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating consensus directory: %w", err)
	}

	fm := NewFrontmatter()
	fm.Set("type", "moderator_evaluation")
	fm.Set("round", data.Round)
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("consensus_score", fmt.Sprintf("%.4f", data.Score))
	fm.Set("agreements_count", data.AgreementsCount)
	fm.Set("divergences_count", data.DivergencesCount)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("duration_ms", data.DurationMS)

	content := renderModeratorReport(data, w.config.IncludeRaw)

	return w.writeFile(path, fm, content)
}

// VnAnalysisData contains data for a V(n) iterative analysis report
type VnAnalysisData struct {
	AgentName  string
	Model      string
	Round      int
	RawOutput  string
	TokensIn   int
	TokensOut  int
	DurationMS int64
}

// WriteVnAnalysis writes a V(n) iterative analysis report (raw LLM output only, no metadata)
func (w *WorkflowReportWriter) WriteVnAnalysis(data VnAnalysisData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	// Create vn directory for this round if needed
	vnDir := filepath.Join(w.AnalyzePhasePath(), fmt.Sprintf("v%d", data.Round))
	if err := os.MkdirAll(vnDir, 0o750); err != nil {
		return fmt.Errorf("creating v%d directory: %w", data.Round, err)
	}

	path := filepath.Join(vnDir, analysisFilename(data.AgentName, data.Model))

	// Write only the raw LLM output, no frontmatter or metadata
	return w.writeFileRaw(path, data.RawOutput+"\n")
}

// WriteConsolidatedAnalysis writes the consolidated analysis (raw LLM output only, no metadata)
func (w *WorkflowReportWriter) WriteConsolidatedAnalysis(data ConsolidationData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.AnalyzePhasePath(), "consolidated.md")

	// Write only the raw LLM output, no frontmatter or metadata
	return w.writeFileRaw(path, data.Content+"\n")
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

	// Create v1 directory if needed (lazy creation)
	v1Dir := filepath.Join(w.PlanPhasePath(), "v1")
	if err := os.MkdirAll(v1Dir, 0o750); err != nil {
		return fmt.Errorf("creating plan v1 directory: %w", err)
	}

	filename := fmt.Sprintf("%s-plan.md", data.Agent)
	path := filepath.Join(v1Dir, filename)

	fm := NewFrontmatter()
	fm.Set("type", "plan")
	fm.Set("agent", data.Agent)
	fm.Set("model", data.Model)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("tokens_in", data.TokensIn)
	fm.Set("tokens_out", data.TokensOut)
	fm.Set("duration_ms", data.DurationMS)

	content := renderPlanReport(data)

	return w.writeFile(path, fm, content)
}

// WritePlanPath returns the path where a V1 plan should be written
func (w *WorkflowReportWriter) WritePlanPath(agentName, _ string) string {
	filename := fmt.Sprintf("%s-plan.md", agentName)
	return filepath.Join(w.PlanPhasePath(), "v1", filename)
}

// WriteConsolidatedPlan writes the consolidated plan (multi-agent)
func (w *WorkflowReportWriter) WriteConsolidatedPlan(content string) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.PlanPhasePath(), "consolidated-plan.md")

	fm := NewFrontmatter()
	fm.Set("type", "consolidated_plan")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)

	mdContent := "# Consolidated Plan (Multi-Agent)\n\n" + content + "\n"

	return w.writeFile(path, fm, mdContent)
}

// ConsolidatedPlanPath returns the path for the consolidated plan
func (w *WorkflowReportWriter) ConsolidatedPlanPath() string {
	return filepath.Join(w.PlanPhasePath(), "consolidated-plan.md")
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

// TaskPlanData contains data for a single task plan
type TaskPlanData struct {
	TaskID         string
	Name           string
	Description    string
	CLI            string
	PlannedModel   string
	ExecutionBatch int
	ParallelWith   []string
	Dependencies   []string
	CanParallelize bool
}

// WriteTaskPlan writes an individual task plan file
func (w *WorkflowReportWriter) WriteTaskPlan(data TaskPlanData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	// Create tasks directory
	tasksDir := filepath.Join(w.PlanPhasePath(), "tasks")
	if err := os.MkdirAll(tasksDir, 0o750); err != nil {
		return fmt.Errorf("creating tasks directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.md", data.TaskID, sanitizeFilename(data.Name))
	path := filepath.Join(tasksDir, filename)

	fm := NewFrontmatter()
	fm.Set("type", "task_plan")
	fm.Set("task_id", data.TaskID)
	fm.Set("name", data.Name)
	fm.Set("cli", data.CLI)
	fm.Set("planned_model", data.PlannedModel)
	fm.Set("execution_batch", data.ExecutionBatch)
	fm.Set("can_parallelize", data.CanParallelize)
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)

	content := renderTaskPlanReport(data)

	return w.writeFile(path, fm, content)
}

// TaskPlanPath returns the path where a task plan should be written.
// This is used when CLIs generate task documentation directly.
// The actual writing is delegated to the assigned CLI, not to this writer.
func (w *WorkflowReportWriter) TaskPlanPath(taskID, taskName string) string {
	filename := fmt.Sprintf("%s-%s.md", taskID, sanitizeFilename(taskName))
	return filepath.Join(w.PlanPhasePath(), "tasks", filename)
}

// EnsureTasksDir creates the tasks directory if it doesn't exist.
// This should be called before CLIs attempt to write task files.
func (w *WorkflowReportWriter) EnsureTasksDir() error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	tasksDir := filepath.Join(w.PlanPhasePath(), "tasks")
	return os.MkdirAll(tasksDir, 0o750)
}

// TasksDir returns the path to the tasks directory.
// This is where CLI-generated task specification files are written.
func (w *WorkflowReportWriter) TasksDir() string {
	if !w.config.Enabled {
		return ""
	}
	return filepath.Join(w.PlanPhasePath(), "tasks")
}

// ExecutionGraphData contains data for the execution graph visualization
type ExecutionGraphData struct {
	Batches      []ExecutionBatch
	TotalTasks   int
	TotalBatches int
}

// ExecutionBatch represents a group of tasks that can execute in parallel
type ExecutionBatch struct {
	BatchNumber int
	Tasks       []ExecutionTask
}

// ExecutionTask represents a task in the execution graph
type ExecutionTask struct {
	TaskID       string
	Name         string
	CLI          string
	PlannedModel string
	Dependencies []string
}

// WriteExecutionGraph writes the execution graph visualization
func (w *WorkflowReportWriter) WriteExecutionGraph(data ExecutionGraphData) error {
	if !w.config.Enabled {
		return nil
	}
	if err := w.Initialize(); err != nil {
		return err
	}

	path := filepath.Join(w.PlanPhasePath(), "execution-graph.md")

	fm := NewFrontmatter()
	fm.Set("type", "execution_graph")
	fm.Set("timestamp", w.formatTime(time.Now()))
	fm.Set("workflow_id", w.workflowID)
	fm.Set("total_tasks", data.TotalTasks)
	fm.Set("total_batches", data.TotalBatches)

	content := renderExecutionGraphReport(data)

	return w.writeFile(path, fm, content)
}

// ========================================
// Execute Phase Writers
// ========================================

// TaskResultData contains data for a task execution result
type TaskResultData struct {
	TaskID     string
	TaskName   string
	Agent      string
	Model      string
	Status     string // "completed", "failed", "skipped"
	Output     string
	Error      string
	TokensIn   int
	TokensOut  int
	DurationMS int64
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
	fm.Set("duration_ms", data.DurationMS)

	content := renderTaskResultReport(data)

	return w.writeFile(path, fm, content)
}

// ExecutionSummaryData contains summary of task execution
type ExecutionSummaryData struct {
	TotalTasks      int
	CompletedTasks  int
	FailedTasks     int
	SkippedTasks    int
	TotalTokensIn   int
	TotalTokensOut  int
	TotalDurationMS int64
	Tasks           []TaskResultData
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
	fm.Set("total_duration_ms", data.TotalDurationMS)

	content := renderExecutionSummaryReport(data)

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
	fm.Set("consensus_score", fmt.Sprintf("%.4f", data.ConsensusScore))

	content := renderMetadataReport(data)

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

	content := renderWorkflowSummaryReport(data)

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

	if err := w.ensureWithinExecutionDir(path); err != nil {
		return err
	}

	// #nosec G304 -- path validated to be within execution directory
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

// writeFileRaw writes content directly without frontmatter
func (w *WorkflowReportWriter) writeFileRaw(path, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureWithinExecutionDir(path); err != nil {
		return err
	}

	// #nosec G304 -- path validated to be within execution directory
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("writing content: %w", err)
	}

	return nil
}

func (w *WorkflowReportWriter) ensureWithinExecutionDir(path string) error {
	baseAbs, err := filepath.Abs(w.ExecutionPath())
	if err != nil {
		return fmt.Errorf("resolving execution path: %w", err)
	}
	targetAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving report path: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("report path escapes execution directory")
	}
	return nil
}
