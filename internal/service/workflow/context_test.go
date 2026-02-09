package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestBuildContextString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		state    *core.WorkflowState
		contains []string
	}{
		{
			name: "basic state",
			state: &core.WorkflowState{
				WorkflowDefinition: core.WorkflowDefinition{
					WorkflowID: "wf-123",
				},
				WorkflowRun: core.WorkflowRun{
					CurrentPhase: core.PhaseAnalyze,
					Tasks:        make(map[core.TaskID]*core.TaskState),
					TaskOrder:    []core.TaskID{},
				},
			},
			contains: []string{"wf-123", "analyze"},
		},
		{
			name: "with completed tasks",
			state: &core.WorkflowState{
				WorkflowDefinition: core.WorkflowDefinition{
					WorkflowID: "wf-456",
				},
				WorkflowRun: core.WorkflowRun{
					CurrentPhase: core.PhaseExecute,
					Tasks: map[core.TaskID]*core.TaskState{
						"task-1": {
							ID:     "task-1",
							Name:   "Setup",
							Status: core.TaskStatusCompleted,
						},
						"task-2": {
							ID:     "task-2",
							Name:   "Build",
							Status: core.TaskStatusRunning,
						},
					},
					TaskOrder: []core.TaskID{"task-1", "task-2"},
				},
			},
			contains: []string{"wf-456", "execute", "Setup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildContextString(tt.state)
			for _, want := range tt.contains {
				if !containsString(result, want) {
					t.Errorf("BuildContextString() = %q, want to contain %q", result, want)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		DryRun:       false,
		DefaultAgent: "claude",
	}

	if cfg.DryRun {
		t.Error("DryRun should be false by default")
	}
	if cfg.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want %q", cfg.DefaultAgent, "claude")
	}
}

func TestResolvePhaseModel(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		AgentPhaseModels: map[string]map[string]string{
			"claude": {
				"analyze": "claude-opus-4-20250514",
			},
		},
	}

	if got := ResolvePhaseModel(cfg, "claude", core.PhaseAnalyze, "task-model"); got != "task-model" {
		t.Fatalf("ResolvePhaseModel() = %q, want %q (task override)", got, "task-model")
	}

	if got := ResolvePhaseModel(cfg, "claude", core.PhaseAnalyze, ""); got != "claude-opus-4-20250514" {
		t.Fatalf("ResolvePhaseModel() = %q, want %q (phase override)", got, "claude-opus-4-20250514")
	}

	if got := ResolvePhaseModel(cfg, "claude", core.PhasePlan, ""); got != "" {
		t.Fatalf("ResolvePhaseModel() = %q, want empty (no override)", got)
	}
}
