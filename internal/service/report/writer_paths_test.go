package report

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowReportWriter_PathHelpers(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-20250121-153045-k7m9p")

	t.Run("ExecutionPath", func(t *testing.T) {
		t.Parallel()
		got := w.ExecutionPath()
		want := filepath.Join(".quorum/runs", "wf-20250121-153045-k7m9p")
		if got != want {
			t.Errorf("ExecutionPath() = %q, want %q", got, want)
		}
	})

	t.Run("AnalyzePhasePath", func(t *testing.T) {
		t.Parallel()
		got := w.AnalyzePhasePath()
		if !strings.HasSuffix(got, "analyze-phase") {
			t.Errorf("AnalyzePhasePath() = %q, should end with analyze-phase", got)
		}
	})

	t.Run("PlanPhasePath", func(t *testing.T) {
		t.Parallel()
		got := w.PlanPhasePath()
		if !strings.HasSuffix(got, "plan-phase") {
			t.Errorf("PlanPhasePath() = %q, should end with plan-phase", got)
		}
	})

	t.Run("ExecutePhasePath", func(t *testing.T) {
		t.Parallel()
		got := w.ExecutePhasePath()
		if !strings.HasSuffix(got, "execute-phase") {
			t.Errorf("ExecutePhasePath() = %q, should end with execute-phase", got)
		}
	})

	t.Run("ExecutionDir", func(t *testing.T) {
		t.Parallel()
		if w.ExecutionDir() != "wf-20250121-153045-k7m9p" {
			t.Errorf("ExecutionDir() = %q, want %q", w.ExecutionDir(), "wf-20250121-153045-k7m9p")
		}
	})

	t.Run("GetExecutionDir", func(t *testing.T) {
		t.Parallel()
		if w.GetExecutionDir() != "wf-20250121-153045-k7m9p" {
			t.Errorf("GetExecutionDir() = %q", w.GetExecutionDir())
		}
	})

	t.Run("IsEnabled", func(t *testing.T) {
		t.Parallel()
		if !w.IsEnabled() {
			t.Error("IsEnabled() = false, want true")
		}
	})
}

func TestWorkflowReportWriter_AnalysisPaths(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	t.Run("V1AnalysisPath", func(t *testing.T) {
		t.Parallel()
		got := w.V1AnalysisPath("claude", "opus-4")
		if !strings.Contains(got, "v1") {
			t.Errorf("V1AnalysisPath() = %q, should contain v1", got)
		}
		if !strings.Contains(got, "claude") {
			t.Errorf("V1AnalysisPath() = %q, should contain agent name", got)
		}
	})

	t.Run("VnAnalysisPath", func(t *testing.T) {
		t.Parallel()
		got := w.VnAnalysisPath("gemini", "pro-2.5", 3)
		if !strings.Contains(got, "v3") {
			t.Errorf("VnAnalysisPath() = %q, should contain v3", got)
		}
		if !strings.Contains(got, "gemini") {
			t.Errorf("VnAnalysisPath() = %q, should contain agent name", got)
		}
	})

	t.Run("SingleAgentAnalysisPath", func(t *testing.T) {
		t.Parallel()
		got := w.SingleAgentAnalysisPath("codex", "o3-pro")
		if !strings.Contains(got, "single-agent") {
			t.Errorf("SingleAgentAnalysisPath() = %q, should contain single-agent", got)
		}
	})

	t.Run("ConsolidatedAnalysisPath", func(t *testing.T) {
		t.Parallel()
		got := w.ConsolidatedAnalysisPath()
		if !strings.HasSuffix(got, "consolidated.md") {
			t.Errorf("ConsolidatedAnalysisPath() = %q, should end with consolidated.md", got)
		}
	})

	t.Run("OptimizedPromptPath", func(t *testing.T) {
		t.Parallel()
		got := w.OptimizedPromptPath()
		if !strings.HasSuffix(got, "01-optimized-prompt.md") {
			t.Errorf("OptimizedPromptPath() = %q, should end with 01-optimized-prompt.md", got)
		}
	})
}

func TestWorkflowReportWriter_ModeratorPaths(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	t.Run("ModeratorReportPath", func(t *testing.T) {
		t.Parallel()
		got := w.ModeratorReportPath(2)
		if !strings.Contains(got, "consensus") {
			t.Errorf("ModeratorReportPath() = %q, should contain consensus", got)
		}
		if !strings.HasSuffix(got, "round-2.md") {
			t.Errorf("ModeratorReportPath() = %q, should end with round-2.md", got)
		}
	})

	t.Run("ModeratorAttemptPath", func(t *testing.T) {
		t.Parallel()
		got := w.ModeratorAttemptPath(1, 2, "claude")
		if !strings.Contains(got, "attempts") {
			t.Errorf("ModeratorAttemptPath() = %q, should contain attempts", got)
		}
		if !strings.Contains(got, "round-1") {
			t.Errorf("ModeratorAttemptPath() = %q, should contain round-1", got)
		}
		if !strings.Contains(got, "attempt-2-claude") {
			t.Errorf("ModeratorAttemptPath() = %q, should contain attempt-2-claude", got)
		}
	})
}

func TestWorkflowReportWriter_TaskOutputPath(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: t.TempDir(), Enabled: true}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	got := w.TaskOutputPath("task-1")
	if !strings.Contains(got, "outputs") {
		t.Errorf("TaskOutputPath() = %q, should contain outputs", got)
	}
	if !strings.HasSuffix(got, "task-1.md") {
		t.Errorf("TaskOutputPath() = %q, should end with task-1.md", got)
	}
}

func TestWorkflowReportWriter_Disabled(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: false}
	w := NewWorkflowReportWriter(cfg, "wf-test")

	if w.IsEnabled() {
		t.Error("IsEnabled() = true, want false for disabled config")
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.BaseDir != ".quorum/runs" {
		t.Errorf("BaseDir = %q, want %q", cfg.BaseDir, ".quorum/runs")
	}
	if !cfg.UseUTC {
		t.Error("UseUTC should default to true")
	}
	if !cfg.IncludeRaw {
		t.Error("IncludeRaw should default to true")
	}
	if !cfg.Enabled {
		t.Error("Enabled should default to true")
	}
}

func TestResumeWorkflowReportWriter(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: true}
	w := ResumeWorkflowReportWriter(cfg, "wf-test", ".quorum/runs/wf-test")

	if w.ExecutionDir() != "wf-test" {
		t.Errorf("ExecutionDir() = %q, want %q", w.ExecutionDir(), "wf-test")
	}
	if w.workflowID != "wf-test" {
		t.Errorf("workflowID = %q, want %q", w.workflowID, "wf-test")
	}
}

func TestNewWorkflowReportWriter(t *testing.T) {
	t.Parallel()

	cfg := Config{BaseDir: ".quorum/runs", Enabled: true, UseUTC: true}
	w := NewWorkflowReportWriter(cfg, "wf-20250121-test")

	if w.executionDir != "wf-20250121-test" {
		t.Errorf("executionDir = %q, want %q", w.executionDir, "wf-20250121-test")
	}
	if w.workflowID != "wf-20250121-test" {
		t.Errorf("workflowID = %q", w.workflowID)
	}
	if w.initialized {
		t.Error("initialized should be false initially")
	}
}
