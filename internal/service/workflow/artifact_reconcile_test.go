package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

type memCheckpointCreator struct{}

func (m memCheckpointCreator) PhaseCheckpoint(state *core.WorkflowState, phase core.Phase, completed bool) error {
	cpType := "phase_start"
	if completed {
		cpType = "phase_complete"
	}
	data, _ := json.Marshal(map[string]any{"phase": string(phase)})
	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        "cp-" + cpType,
		Type:      cpType,
		Phase:     phase,
		Timestamp: time.Now(),
		Data:      data,
	})
	return nil
}

func (m memCheckpointCreator) TaskCheckpoint(_ *core.WorkflowState, _ *core.Task, _ bool) error {
	return nil
}

func (m memCheckpointCreator) ErrorCheckpoint(_ *core.WorkflowState, _ error) error {
	return nil
}

func (m memCheckpointCreator) ErrorCheckpointWithContext(_ *core.WorkflowState, _ error, _ service.ErrorCheckpointDetails) error {
	return nil
}

func (m memCheckpointCreator) CreateCheckpoint(state *core.WorkflowState, checkpointType string, metadata map[string]interface{}) error {
	data, _ := json.Marshal(metadata)
	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		ID:        "cp-" + checkpointType,
		Type:      checkpointType,
		Phase:     state.CurrentPhase,
		Timestamp: time.Now(),
		Data:      data,
	})
	return nil
}

func TestRunner_ReconcileAnalysisArtifacts_CreatesMissingCheckpoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()

	workflowID := core.WorkflowID("wf-reconcile-001")
	consolidatedPath := filepath.Join(tmpDir, string(workflowID), "analyze-phase", "consolidated.md")
	if err := os.MkdirAll(filepath.Dir(consolidatedPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "## Consolidated\n\n" + strings.Repeat("a", 600)
	if err := os.WriteFile(consolidatedPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write consolidated: %v", err)
	}

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: workflowID,
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Checkpoints:  nil,
			ReportPath:   "", // Force BaseDir+workflowID path resolution
			UpdatedAt:    time.Now(),
		},
	}

	r := &Runner{
		config: &RunnerConfig{Report: reportConfigForTest(tmpDir)},
		// Persist not required for this unit test; checkpoint creator mutates state in-memory.
		state:      nil,
		checkpoint: memCheckpointCreator{},
		output:     NopOutputNotifier{},
	}

	if err := r.reconcileAnalysisArtifacts(ctx, state); err != nil {
		t.Fatalf("reconcileAnalysisArtifacts() error = %v", err)
	}

	if got := strings.TrimSpace(GetConsolidatedAnalysis(state)); got != strings.TrimSpace(content) {
		t.Fatalf("GetConsolidatedAnalysis() mismatch: got len=%d want len=%d", len(got), len(content))
	}
	if !isPhaseCompleted(state, core.PhaseAnalyze) {
		t.Fatal("expected analyze phase to be marked complete after reconciliation")
	}
}

func TestRunner_ReconcileAnalysisArtifacts_DoesNotDuplicateConsolidatedCheckpoint(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()

	workflowID := core.WorkflowID("wf-reconcile-002")
	consolidatedPath := filepath.Join(tmpDir, string(workflowID), "analyze-phase", "consolidated.md")
	if err := os.MkdirAll(filepath.Dir(consolidatedPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "## Consolidated\n\n" + strings.Repeat("b", 600)
	if err := os.WriteFile(consolidatedPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write consolidated: %v", err)
	}

	data, _ := json.Marshal(map[string]any{"content": content})
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: workflowID,
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
			Checkpoints: []core.Checkpoint{
				{ID: "cp-existing", Type: "consolidated_analysis", Phase: core.PhaseAnalyze, Data: data, Timestamp: time.Now()},
			},
			ReportPath: "",
			UpdatedAt:  time.Now(),
		},
	}

	r := &Runner{
		config:      &RunnerConfig{Report: reportConfigForTest(tmpDir)},
		state:       nil,
		checkpoint:  memCheckpointCreator{},
		output:      NopOutputNotifier{},
		projectRoot: "",
	}

	if err := r.reconcileAnalysisArtifacts(ctx, state); err != nil {
		t.Fatalf("reconcileAnalysisArtifacts() error = %v", err)
	}

	// Should have added only the analyze phase_complete checkpoint.
	var consolidatedCount int
	for _, cp := range state.Checkpoints {
		if cp.Type == "consolidated_analysis" {
			consolidatedCount++
		}
	}
	if consolidatedCount != 1 {
		t.Fatalf("consolidated_analysis checkpoints = %d, want 1", consolidatedCount)
	}
	if !isPhaseCompleted(state, core.PhaseAnalyze) {
		t.Fatal("expected analyze phase to be marked complete after reconciliation")
	}
}

func reportConfigForTest(baseDir string) report.Config {
	return report.Config{
		BaseDir:    baseDir,
		Enabled:    true,
		UseUTC:     true,
		IncludeRaw: true,
	}
}

func TestRunner_ReconcileAnalysisArtifacts_SingleAgentFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()

	workflowID := core.WorkflowID("wf-reconcile-single")

	// Create single-agent analysis (NO consolidated.md)
	singleAgentDir := filepath.Join(tmpDir, string(workflowID), "analyze-phase", "single-agent")
	if err := os.MkdirAll(singleAgentDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	analysisContent := "## Single Agent Analysis\n\n" + strings.Repeat("Single-agent analysis content. ", 30) // >512 bytes
	if err := os.WriteFile(filepath.Join(singleAgentDir, "claude-opus.md"), []byte(analysisContent), 0o640); err != nil {
		t.Fatalf("write: %v", err)
	}

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: workflowID,
			Prompt:     "test",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Checkpoints:  nil,
			ReportPath:   "",
			UpdatedAt:    time.Now(),
		},
	}

	r := &Runner{
		config:      &RunnerConfig{Report: reportConfigForTest(tmpDir)},
		state:       nil,
		checkpoint:  memCheckpointCreator{},
		output:      NopOutputNotifier{},
		projectRoot: "",
	}

	if err := r.reconcileAnalysisArtifacts(ctx, state); err != nil {
		t.Fatalf("reconcileAnalysisArtifacts() error = %v", err)
	}

	got := strings.TrimSpace(GetConsolidatedAnalysis(state))
	if got == "" {
		t.Error("expected consolidated analysis from single-agent fallback")
	}
	if !strings.Contains(got, "Single-agent analysis content") {
		t.Errorf("content should contain single-agent analysis, got: %s", got[:min(len(got), 100)])
	}
	if !isPhaseCompleted(state, core.PhaseAnalyze) {
		t.Fatal("expected analyze phase to be marked complete after reconciliation")
	}
}
