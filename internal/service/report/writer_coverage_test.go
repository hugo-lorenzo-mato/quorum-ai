package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Initialize ---

func TestWorkflowReportWriter_Initialize(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-init-test")

	if err := w.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Verify all expected directories were created
	expectedDirs := []string{
		w.ExecutionPath(),
		w.AnalyzePhasePath(),
		filepath.Join(w.AnalyzePhasePath(), "consensus"),
		w.PlanPhasePath(),
		filepath.Join(w.PlanPhasePath(), "consensus"),
		w.ExecutePhasePath(),
		filepath.Join(w.ExecutePhasePath(), "tasks"),
	}
	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// Second Initialize should be idempotent (no error)
	if err := w.Initialize(); err != nil {
		t.Fatalf("second Initialize() error = %v", err)
	}
}

func TestWorkflowReportWriter_Initialize_Disabled(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: "/nonexistent", Enabled: false}
	w := NewWorkflowReportWriter(cfg, "wf-disabled")

	if err := w.Initialize(); err != nil {
		t.Errorf("disabled writer Initialize() should not error: %v", err)
	}
}

// --- formatTime ---

func TestWorkflowReportWriter_FormatTime_UTC(t *testing.T) {
	t.Parallel()
	cfg := Config{UseUTC: true, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.FixedZone("EST", -5*3600))
	got := w.formatTime(ts)
	if !strings.Contains(got, "15:30:00Z") {
		t.Errorf("formatTime() = %q, expected UTC conversion", got)
	}
}

func TestWorkflowReportWriter_FormatTime_Local(t *testing.T) {
	t.Parallel()
	cfg := Config{UseUTC: false, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	got := w.formatTime(ts)
	if got == "" {
		t.Error("formatTime() returned empty")
	}
}

// --- ensureWithinExecutionDir ---

func TestWorkflowReportWriter_EnsureWithinExecutionDir_Valid(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: "/tmp/quorum-test", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	validPath := filepath.Join(w.ExecutionPath(), "analyze-phase", "report.md")
	if err := w.ensureWithinExecutionDir(validPath); err != nil {
		t.Errorf("valid path should not error: %v", err)
	}
}

func TestWorkflowReportWriter_EnsureWithinExecutionDir_Escape(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: "/tmp/quorum-test", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	escapePath := filepath.Join(w.ExecutionPath(), "..", "..", "etc", "passwd")
	if err := w.ensureWithinExecutionDir(escapePath); err == nil {
		t.Error("path escaping execution dir should error")
	}
}

// --- WriteOriginalPrompt ---

func TestWorkflowReportWriter_WriteOriginalPrompt(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true, UseUTC: true}
	w := NewWorkflowReportWriter(cfg, "wf-prompt-test")

	if err := w.WriteOriginalPrompt("Hello world"); err != nil {
		t.Fatalf("WriteOriginalPrompt() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "00-original-prompt.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "original_prompt") {
		t.Error("should contain type: original_prompt")
	}
	if !strings.Contains(content, "Hello world") {
		t.Error("should contain the prompt text")
	}
}

func TestWorkflowReportWriter_WriteOriginalPrompt_Disabled(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: "/nonexistent", Enabled: false}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	if err := w.WriteOriginalPrompt("test"); err != nil {
		t.Errorf("disabled writer should not error: %v", err)
	}
}

// --- WriteRefinedPrompt ---

func TestWorkflowReportWriter_WriteRefinedPrompt(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-refined-test")

	if err := w.WriteRefinedPrompt("original", "refined content", PromptMetrics{}); err != nil {
		t.Fatalf("WriteRefinedPrompt() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "01-refined-prompt.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read refined prompt: %v", err)
	}
	if !strings.Contains(string(data), "refined content") {
		t.Error("should contain refined content")
	}
}

// --- WriteV1Analysis ---

func TestWorkflowReportWriter_WriteV1Analysis(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-v1-test")

	ad := AnalysisData{
		AgentName: "claude",
		Model:     "opus-4",
		RawOutput: "analysis output here",
	}
	if err := w.WriteV1Analysis(ad); err != nil {
		t.Fatalf("WriteV1Analysis() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "v1", "claude-opus-4.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read v1 analysis: %v", err)
	}
	if !strings.Contains(string(data), "analysis output here") {
		t.Error("should contain raw output")
	}
}

func TestWorkflowReportWriter_WriteV1Analysis_Disabled(t *testing.T) {
	t.Parallel()
	cfg := Config{Enabled: false}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	if err := w.WriteV1Analysis(AnalysisData{}); err != nil {
		t.Errorf("disabled writer should not error: %v", err)
	}
}

// --- WriteVnAnalysis ---

func TestWorkflowReportWriter_WriteVnAnalysis(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-vn-test")

	vn := VnAnalysisData{
		AgentName: "gemini",
		Model:     "pro",
		Round:     3,
		RawOutput: "round 3 output",
	}
	if err := w.WriteVnAnalysis(vn); err != nil {
		t.Fatalf("WriteVnAnalysis() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "v3", "gemini-pro.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("v3 analysis file should exist: %v", err)
	}
}

// --- WriteConsolidatedAnalysis ---

func TestWorkflowReportWriter_WriteConsolidatedAnalysis(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-consol-test")

	cd := ConsolidationData{Content: "consolidated result"}
	if err := w.WriteConsolidatedAnalysis(cd); err != nil {
		t.Fatalf("WriteConsolidatedAnalysis() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "consolidated.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !strings.Contains(string(data), "consolidated result") {
		t.Error("should contain consolidated content")
	}
}

// --- WriteConsensusReport ---

func TestWorkflowReportWriter_WriteConsensusReport(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true, UseUTC: true}
	w := NewWorkflowReportWriter(cfg, "wf-consensus-test")

	cd := ConsensusData{
		Score:       0.85,
		Threshold:   0.75,
		AgentsCount: 3,
	}
	if err := w.WriteConsensusReport(cd, "analyze"); err != nil {
		t.Fatalf("WriteConsensusReport() error = %v", err)
	}

	path := filepath.Join(w.AnalyzePhasePath(), "consensus", "after-analyze.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !strings.Contains(string(data), "consensus") {
		t.Error("should contain type: consensus in frontmatter")
	}
}

// --- WriteModeratorReport ---

func TestWorkflowReportWriter_WriteModeratorReport(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true, IncludeRaw: true}
	w := NewWorkflowReportWriter(cfg, "wf-mod-test")

	md := ModeratorData{
		Agent:     "claude",
		Model:     "opus-4",
		Round:     1,
		Score:     0.90,
		RawOutput: "moderator evaluation",
	}
	if err := w.WriteModeratorReport(md); err != nil {
		t.Fatalf("WriteModeratorReport() error = %v", err)
	}

	path := w.ModeratorReportPath(1)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("moderator report should exist: %v", err)
	}
}

// --- WritePlan ---

func TestWorkflowReportWriter_WritePlan(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-plan-test")

	pd := PlanData{
		Agent:   "claude",
		Model:   "opus-4",
		Content: "Step 1: Implement\nStep 2: Test",
	}
	if err := w.WritePlan(pd); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	path := filepath.Join(w.PlanPhasePath(), "v1", "claude-plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !strings.Contains(string(data), "Step 1") {
		t.Error("should contain plan content")
	}
}

// --- WriteConsolidatedPlan ---

func TestWorkflowReportWriter_WriteConsolidatedPlan(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-cplan-test")

	if err := w.WriteConsolidatedPlan("merged plan content"); err != nil {
		t.Fatalf("WriteConsolidatedPlan() error = %v", err)
	}

	path := w.ConsolidatedPlanPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if !strings.Contains(string(data), "merged plan content") {
		t.Error("should contain consolidated plan")
	}
}

// --- WriteFinalPlan ---

func TestWorkflowReportWriter_WriteFinalPlan(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-final-test")

	if err := w.WriteFinalPlan("approved plan"); err != nil {
		t.Fatalf("WriteFinalPlan() error = %v", err)
	}

	path := filepath.Join(w.PlanPhasePath(), "final-plan.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("final plan should exist: %v", err)
	}
}

// --- WriteTaskPlan ---

func TestWorkflowReportWriter_WriteTaskPlan(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-taskplan-test")

	tp := TaskPlanData{
		TaskID:         "task-1",
		Name:           "Implement auth",
		Description:    "Add OAuth2",
		CLI:            "claude",
		PlannedModel:   "opus-4",
		ExecutionBatch: 1,
	}
	if err := w.WriteTaskPlan(tp); err != nil {
		t.Fatalf("WriteTaskPlan() error = %v", err)
	}
}

// --- WriteExecutionGraph ---

func TestWorkflowReportWriter_WriteExecutionGraph(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-graph-test")

	data := ExecutionGraphData{
		TotalTasks:   2,
		TotalBatches: 1,
		Batches: []ExecutionBatch{
			{BatchNumber: 1, Tasks: []ExecutionTask{
				{TaskID: "t1", Name: "Task1", CLI: "claude"},
				{TaskID: "t2", Name: "Task2", CLI: "gemini"},
			}},
		},
	}
	if err := w.WriteExecutionGraph(data); err != nil {
		t.Fatalf("WriteExecutionGraph() error = %v", err)
	}
}

// --- WriteTaskResult ---

func TestWorkflowReportWriter_WriteTaskResult(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-result-test")

	tr := TaskResultData{
		TaskID:   "task-1",
		TaskName: "Test task",
		Agent:    "claude",
		Model:    "opus-4",
		Status:   "completed",
		Output:   "done",
	}
	if err := w.WriteTaskResult(tr); err != nil {
		t.Fatalf("WriteTaskResult() error = %v", err)
	}
}

// --- WriteExecutionSummary ---

func TestWorkflowReportWriter_WriteExecutionSummary(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-summary-test")

	es := ExecutionSummaryData{
		TotalTasks:     2,
		CompletedTasks: 1,
		FailedTasks:    1,
	}
	if err := w.WriteExecutionSummary(es); err != nil {
		t.Fatalf("WriteExecutionSummary() error = %v", err)
	}
}

// --- WriteMetadata ---

func TestWorkflowReportWriter_WriteMetadata(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true, UseUTC: true}
	w := NewWorkflowReportWriter(cfg, "wf-meta-test")

	md := WorkflowMetadata{
		WorkflowID:     "wf-meta-test",
		StartedAt:      time.Now(),
		CompletedAt:    time.Now(),
		Status:         "completed",
		PhasesExecuted: []string{"analyze", "plan"},
		AgentsUsed:     []string{"claude"},
	}
	if err := w.WriteMetadata(md); err != nil {
		t.Fatalf("WriteMetadata() error = %v", err)
	}

	path := filepath.Join(w.ExecutionPath(), "metadata.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("metadata file should exist: %v", err)
	}
}

// --- WriteWorkflowSummary ---

func TestWorkflowReportWriter_WriteWorkflowSummary(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-wfsummary-test")

	md := WorkflowMetadata{
		WorkflowID:     "wf-wfsummary-test",
		Status:         "completed",
		StartedAt:      time.Now(),
		PhasesExecuted: []string{"analyze"},
	}
	if err := w.WriteWorkflowSummary(md); err != nil {
		t.Fatalf("WriteWorkflowSummary() error = %v", err)
	}

	path := filepath.Join(w.ExecutionPath(), "workflow-summary.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("workflow summary should exist: %v", err)
	}
}

// --- PromoteModeratorAttempt ---

func TestWorkflowReportWriter_PromoteModeratorAttempt(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-promote-test")

	if err := w.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	// Create the attempt file first
	attemptPath := w.ModeratorAttemptPath(1, 1, "claude")
	if err := os.MkdirAll(filepath.Dir(attemptPath), 0o750); err != nil {
		t.Fatalf("creating attempt dir: %v", err)
	}
	if err := os.WriteFile(attemptPath, []byte("moderator output"), 0o644); err != nil {
		t.Fatalf("writing attempt file: %v", err)
	}

	// Promote
	if err := w.PromoteModeratorAttempt(1, 1, "claude"); err != nil {
		t.Fatalf("PromoteModeratorAttempt() error = %v", err)
	}

	// Verify promoted file
	promotedPath := w.ModeratorReportPath(1)
	data, err := os.ReadFile(promotedPath)
	if err != nil {
		t.Fatalf("promoted file should exist: %v", err)
	}
	if string(data) != "moderator output" {
		t.Errorf("promoted content = %q, want %q", string(data), "moderator output")
	}
}

func TestWorkflowReportWriter_PromoteModeratorAttempt_MissingSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	cfg := Config{BaseDir: tmpDir, Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-promote-err-test")

	if err := w.PromoteModeratorAttempt(1, 1, "claude"); err == nil {
		t.Error("should error when source file doesn't exist")
	}
}

// --- WritePlanPath and TasksDir ---

func TestWorkflowReportWriter_WritePlanPath(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	got := w.WritePlanPath("claude", "opus-4")
	if !strings.HasSuffix(got, "claude-plan.md") {
		t.Errorf("WritePlanPath() = %q, want suffix claude-plan.md", got)
	}
	if !strings.Contains(got, "v1") {
		t.Errorf("WritePlanPath() should contain v1")
	}
}

func TestWorkflowReportWriter_TasksDir_Enabled(t *testing.T) {
	t.Parallel()
	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	got := w.TasksDir()
	if !strings.HasSuffix(got, "tasks") {
		t.Errorf("TasksDir() = %q, want suffix tasks", got)
	}
}

func TestWorkflowReportWriter_TasksDir_Disabled(t *testing.T) {
	t.Parallel()
	cfg := Config{Enabled: false}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	if got := w.TasksDir(); got != "" {
		t.Errorf("disabled TasksDir() = %q, want empty", got)
	}
}
