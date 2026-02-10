package report

import (
	"strings"
	"testing"
	"time"
)

func TestCountWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty string", "", 0},
		{"single word", "hello", 1},
		{"multiple words", "hello world foo", 3},
		{"extra whitespace", "  hello   world  ", 2},
		{"tabs and newlines", "hello\tworld\nfoo", 3},
		{"only whitespace", "   \t\n  ", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := countWords(tt.input); got != tt.want {
				t.Errorf("countWords(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unicode letters", "café", "café"},
		{"digits only", "12345", "12345"},
		{"empty string", "", ""},
		{"only special chars", "@#$%", ""},
		{"mixed model name", "Claude-3 Opus/v2:latest", "claude-3-opus-v2-latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeFilename(tt.input); got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAnalysisFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agentName string
		model     string
		want      string
	}{
		{"with model", "claude", "opus-4", "claude-opus-4.md"},
		{"empty model", "claude", "", "claude.md"},
		{"complex model", "gemini", "Gemini Pro 1.5", "gemini-gemini-pro-1.5.md"},
		{"model with special chars", "codex", "gpt-4o:latest", "codex-gpt-4o-latest.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := analysisFilename(tt.agentName, tt.model); got != tt.want {
				t.Errorf("analysisFilename(%q, %q) = %q, want %q", tt.agentName, tt.model, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "0ms"},
		{"milliseconds", 500, "500ms"},
		{"one second", 1000, "1.0s"},
		{"seconds with decimal", 2500, "2.5s"},
		{"one minute", 60000, "1m 0s"},
		{"minutes and seconds", 90000, "1m 30s"},
		{"multiple minutes", 150000, "2m 30s"},
		{"sub-second", 999, "999ms"},
		{"exact boundary", 59999, "60.0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatDuration(tt.ms); got != tt.want {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestContainsPhase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		phases []string
		target string
		want   bool
	}{
		{"found exact", []string{"analyze", "plan", "execute"}, "plan", true},
		{"found case insensitive", []string{"Analyze", "Plan"}, "analyze", true},
		{"not found", []string{"analyze", "plan"}, "execute", false},
		{"empty phases", []string{}, "plan", false},
		{"empty target", []string{"analyze"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := containsPhase(tt.phases, tt.target); got != tt.want {
				t.Errorf("containsPhase(%v, %q) = %v, want %v", tt.phases, tt.target, got, tt.want)
			}
		})
	}
}

func TestRenderConsensusReport(t *testing.T) {
	t.Parallel()

	t.Run("approved", func(t *testing.T) {
		t.Parallel()
		data := ConsensusData{
			Score:                0.85,
			Threshold:            0.75,
			AgentsCount:          3,
			ClaimsScore:          0.90,
			RisksScore:           0.80,
			RecommendationsScore: 0.85,
		}
		result := renderConsensusReport(data, "analyze")
		assertContainsAll(t, result, []string{
			"Reporte de Consenso",
			"después de analyze",
			"✅ Aprobado",
			"85.00%",
			"75.00%",
			"Score Global",
		})
	})

	t.Run("needs escalation", func(t *testing.T) {
		t.Parallel()
		data := ConsensusData{
			Score:           0.50,
			NeedsEscalation: true,
		}
		result := renderConsensusReport(data, "plan")
		assertContainsAll(t, result, []string{
			"⚠️ Requiere Escalación",
		})
	})

	t.Run("needs human review", func(t *testing.T) {
		t.Parallel()
		data := ConsensusData{
			Score:            0.30,
			NeedsHumanReview: true,
			NeedsEscalation:  true, // human review takes priority
		}
		result := renderConsensusReport(data, "analyze")
		assertContainsAll(t, result, []string{
			"🚨 Requiere Revisión Humana",
		})
	})

	t.Run("with divergences", func(t *testing.T) {
		t.Parallel()
		data := ConsensusData{
			Score: 0.60,
			Divergences: []DivergenceData{
				{
					Type:        "semantic",
					Agent1:      "claude",
					Agent2:      "gemini",
					Description: "Different approach to security",
				},
			},
		}
		result := renderConsensusReport(data, "analyze")
		assertContainsAll(t, result, []string{
			"Divergencias Detectadas",
			"semantic",
			"claude",
			"gemini",
			"Different approach to security",
		})
	})
}

func TestRenderModeratorReport(t *testing.T) {
	t.Parallel()

	t.Run("high score green", func(t *testing.T) {
		t.Parallel()
		data := ModeratorData{
			Agent:            "claude",
			Model:            "opus-4",
			Round:            2,
			Score:            0.92,
			AgreementsCount:  5,
			DivergencesCount: 1,
			TokensIn:         1000,
			TokensOut:        500,
			DurationMS:       2500,
		}
		result := renderModeratorReport(data, false)
		assertContainsAll(t, result, []string{
			"🟢",
			"92%",
			"Ronda 2",
			"claude",
			"opus-4",
		})
		if strings.Contains(result, "Evaluación Completa") {
			t.Error("should not include raw output when includeRaw=false")
		}
	})

	t.Run("medium score yellow", func(t *testing.T) {
		t.Parallel()
		data := ModeratorData{Score: 0.80, Round: 1}
		result := renderModeratorReport(data, false)
		if !strings.Contains(result, "🟡") {
			t.Error("expected yellow emoji for score 0.80")
		}
	})

	t.Run("low score red", func(t *testing.T) {
		t.Parallel()
		data := ModeratorData{Score: 0.50, Round: 1}
		result := renderModeratorReport(data, false)
		if !strings.Contains(result, "🔴") {
			t.Error("expected red emoji for score 0.50")
		}
	})

	t.Run("with raw output", func(t *testing.T) {
		t.Parallel()
		data := ModeratorData{
			Score:     0.85,
			Round:     1,
			RawOutput: "Detailed moderator analysis...",
		}
		result := renderModeratorReport(data, true)
		assertContainsAll(t, result, []string{
			"Evaluación Completa del Moderador",
			"Detailed moderator analysis...",
		})
	})
}

func TestRenderPlanReport(t *testing.T) {
	t.Parallel()

	data := PlanData{
		Agent:      "claude",
		Model:      "opus-4",
		Content:    "Step 1: Do X\nStep 2: Do Y",
		TokensIn:   500,
		TokensOut:  300,
		DurationMS: 1500,
	}
	result := renderPlanReport(data)
	assertContainsAll(t, result, []string{
		"# Plan: claude (opus-4)",
		"Step 1: Do X",
		"Step 2: Do Y",
		"Tokens entrada | 500",
		"Tokens salida | 300",
		"1.5s",
	})
}

func TestRenderTaskResultReport(t *testing.T) {
	t.Parallel()

	t.Run("completed task", func(t *testing.T) {
		t.Parallel()
		data := TaskResultData{
			TaskID:     "task-1",
			TaskName:   "Implement feature",
			Agent:      "claude",
			Model:      "opus-4",
			Status:     "completed",
			Output:     "Feature implemented successfully",
			TokensIn:   1000,
			TokensOut:  800,
			DurationMS: 5000,
		}
		result := renderTaskResultReport(data)
		assertContainsAll(t, result, []string{
			"✅",
			"Implement feature",
			"task-1",
			"completed",
			"Resultado",
			"Feature implemented successfully",
		})
		if strings.Contains(result, "Error") {
			t.Error("completed task should not have Error section")
		}
	})

	t.Run("failed task", func(t *testing.T) {
		t.Parallel()
		data := TaskResultData{
			TaskID:   "task-2",
			TaskName: "Broken task",
			Agent:    "gemini",
			Model:    "pro",
			Status:   "failed",
			Error:    "timeout exceeded",
		}
		result := renderTaskResultReport(data)
		assertContainsAll(t, result, []string{
			"❌",
			"Broken task",
			"Error",
			"timeout exceeded",
		})
	})

	t.Run("skipped task", func(t *testing.T) {
		t.Parallel()
		data := TaskResultData{
			TaskID:   "task-3",
			TaskName: "Skipped task",
			Status:   "skipped",
		}
		result := renderTaskResultReport(data)
		if !strings.Contains(result, "⏭️") {
			t.Error("expected skip emoji")
		}
	})
}

func TestRenderExecutionSummaryReport(t *testing.T) {
	t.Parallel()

	data := ExecutionSummaryData{
		TotalTasks:      3,
		CompletedTasks:  2,
		FailedTasks:     1,
		SkippedTasks:    0,
		TotalTokensIn:   3000,
		TotalTokensOut:  2000,
		TotalDurationMS: 15000,
		Tasks: []TaskResultData{
			{TaskID: "t1", TaskName: "Task 1", Status: "completed", DurationMS: 5000},
			{TaskID: "t2", TaskName: "Task 2", Status: "completed", DurationMS: 5000},
			{TaskID: "t3", TaskName: "Task 3", Status: "failed", DurationMS: 5000},
		},
	}
	result := renderExecutionSummaryReport(data)
	assertContainsAll(t, result, []string{
		"Resumen de Ejecución",
		"Total de tareas | 3",
		"Completadas | 2",
		"Fallidas | 1",
		"Task 1",
		"Task 2",
		"Task 3",
	})
}

func TestRenderMetadataReport(t *testing.T) {
	t.Parallel()

	t.Run("completed workflow", func(t *testing.T) {
		t.Parallel()
		start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 1, 10, 5, 30, 0, time.UTC)
		data := WorkflowMetadata{
			WorkflowID:     "wf-123",
			StartedAt:      start,
			CompletedAt:    end,
			Status:         "completed",
			PhasesExecuted: []string{"analyze", "plan", "execute"},
			TotalTokensIn:  5000,
			TotalTokensOut: 3000,
			ConsensusScore: 0.92,
			AgentsUsed:     []string{"claude", "gemini"},
		}
		result := renderMetadataReport(data)
		assertContainsAll(t, result, []string{
			"wf-123",
			"✅",
			"completed",
			"analyze",
			"plan",
			"execute",
			"5000",
			"3000",
			"92.00%",
			"claude",
			"gemini",
			"5m30s",
		})
	})

	t.Run("failed workflow", func(t *testing.T) {
		t.Parallel()
		data := WorkflowMetadata{
			WorkflowID: "wf-456",
			Status:     "failed",
			StartedAt:  time.Now(),
		}
		result := renderMetadataReport(data)
		if !strings.Contains(result, "❌") {
			t.Error("expected failure emoji")
		}
	})

	t.Run("running workflow", func(t *testing.T) {
		t.Parallel()
		data := WorkflowMetadata{
			WorkflowID: "wf-789",
			Status:     "running",
			StartedAt:  time.Now(),
		}
		result := renderMetadataReport(data)
		if !strings.Contains(result, "🔄") {
			t.Error("expected running emoji")
		}
	})
}

func TestRenderWorkflowSummaryReport(t *testing.T) {
	t.Parallel()

	t.Run("completed with all phases", func(t *testing.T) {
		t.Parallel()
		start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
		end := time.Date(2025, 1, 1, 10, 10, 0, 0, time.UTC)
		data := WorkflowMetadata{
			WorkflowID:     "wf-123",
			Status:         "completed",
			StartedAt:      start,
			CompletedAt:    end,
			PhasesExecuted: []string{"analyze", "plan", "execute"},
			ConsensusScore: 0.88,
			TotalTokensIn:  10000,
			TotalTokensOut: 8000,
		}
		result := renderWorkflowSummaryReport(data)
		assertContainsAll(t, result, []string{
			"Resumen del Workflow",
			"✅ Completado exitosamente",
			"wf-123",
			"10m0s",
			"analyze",
			"plan",
			"execute",
			"plan-phase",
			"execute-phase",
			"88.00%",
		})
	})

	t.Run("failed workflow", func(t *testing.T) {
		t.Parallel()
		data := WorkflowMetadata{
			WorkflowID: "wf-fail",
			Status:     "failed",
			StartedAt:  time.Now(),
		}
		result := renderWorkflowSummaryReport(data)
		assertContainsAll(t, result, []string{
			"❌ Fallido",
		})
	})

	t.Run("analyze only - no plan or execute dirs", func(t *testing.T) {
		t.Parallel()
		data := WorkflowMetadata{
			WorkflowID:     "wf-analyze",
			Status:         "completed",
			PhasesExecuted: []string{"analyze"},
			StartedAt:      time.Now(),
		}
		result := renderWorkflowSummaryReport(data)
		if strings.Contains(result, "plan-phase") {
			t.Error("should not mention plan-phase when plan was not executed")
		}
		if strings.Contains(result, "execute-phase") {
			t.Error("should not mention execute-phase when execute was not executed")
		}
	})
}

func TestRenderTaskPlanReport(t *testing.T) {
	t.Parallel()

	t.Run("parallelizable task", func(t *testing.T) {
		t.Parallel()
		data := TaskPlanData{
			TaskID:         "task-1",
			Name:           "Implement auth",
			Description:    "Add OAuth2 authentication",
			CLI:            "claude",
			PlannedModel:   "opus-4",
			ExecutionBatch: 1,
			CanParallelize: true,
			ParallelWith:   []string{"task-2", "task-3"},
			Dependencies:   []string{},
		}
		result := renderTaskPlanReport(data)
		assertContainsAll(t, result, []string{
			"Implement auth",
			"OAuth2",
			"claude",
			"opus-4",
			"**Batch**: 1",
			"**Can parallelize with**: task-2, task-3",
			"None (can start immediately)",
		})
	})

	t.Run("task with dependencies", func(t *testing.T) {
		t.Parallel()
		data := TaskPlanData{
			TaskID:         "task-3",
			Name:           "Integration tests",
			CLI:            "codex",
			PlannedModel:   "gpt-4o",
			ExecutionBatch: 2,
			CanParallelize: false,
			Dependencies:   []string{"task-1", "task-2"},
		}
		result := renderTaskPlanReport(data)
		assertContainsAll(t, result, []string{
			"Not parallelizable",
			"**Dependencies**: task-1, task-2",
		})
	})

	t.Run("no description", func(t *testing.T) {
		t.Parallel()
		data := TaskPlanData{
			Name: "Simple task",
			CLI:  "claude",
		}
		result := renderTaskPlanReport(data)
		if strings.Contains(result, "## Description") {
			t.Error("should not include Description section when empty")
		}
	})
}

func TestRenderExecutionGraphReport(t *testing.T) {
	t.Parallel()

	t.Run("multi-batch graph", func(t *testing.T) {
		t.Parallel()
		data := ExecutionGraphData{
			TotalTasks:   3,
			TotalBatches: 2,
			Batches: []ExecutionBatch{
				{
					BatchNumber: 1,
					Tasks: []ExecutionTask{
						{TaskID: "t1", Name: "Analyze", CLI: "claude", PlannedModel: "opus-4"},
						{TaskID: "t2", Name: "Scan", CLI: "gemini", PlannedModel: "pro"},
					},
				},
				{
					BatchNumber: 2,
					Tasks: []ExecutionTask{
						{TaskID: "t3", Name: "Execute", CLI: "claude", PlannedModel: "opus-4", Dependencies: []string{"t1"}},
					},
				},
			},
		}
		result := renderExecutionGraphReport(data)
		assertContainsAll(t, result, []string{
			"Execution Graph",
			"Total Tasks**: 3",
			"Parallel Batches**: 2",
			"Batch 1",
			"No dependencies",
			"Batch 2",
			"Depends on Batch 1",
			"Dependency Flow",
			"Model Distribution",
		})
	})

	t.Run("single batch no dependency flow", func(t *testing.T) {
		t.Parallel()
		data := ExecutionGraphData{
			TotalTasks:   1,
			TotalBatches: 1,
			Batches: []ExecutionBatch{
				{
					BatchNumber: 1,
					Tasks:       []ExecutionTask{{TaskID: "t1", Name: "Only task", CLI: "claude"}},
				},
			},
		}
		result := renderExecutionGraphReport(data)
		if strings.Contains(result, "Dependency Flow") {
			t.Error("single batch should not include dependency flow section")
		}
	})
}

// assertContainsAll checks that s contains all the given substrings.
func assertContainsAll(t *testing.T, s string, substrings []string) {
	t.Helper()
	for _, sub := range substrings {
		if !strings.Contains(s, sub) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", sub, s)
		}
	}
}
