package workflow

import (
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestBuildContextString(t *testing.T) {
	tests := []struct {
		name     string
		state    *core.WorkflowState
		contains []string
	}{
		{
			name: "basic state",
			state: &core.WorkflowState{
				WorkflowID:   "wf-123",
				CurrentPhase: core.PhaseAnalyze,
				Tasks:        make(map[core.TaskID]*core.TaskState),
				TaskOrder:    []core.TaskID{},
			},
			contains: []string{"wf-123", "analyze"},
		},
		{
			name: "with completed tasks",
			state: &core.WorkflowState{
				WorkflowID:   "wf-456",
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
	cfg := &Config{
		DryRun:       false,
		Sandbox:      true,
		DefaultAgent: "claude",
		V3Agent:      "claude",
	}

	if cfg.DryRun {
		t.Error("DryRun should be false by default")
	}
	if !cfg.Sandbox {
		t.Error("Sandbox should be true by default")
	}
	if cfg.DefaultAgent != "claude" {
		t.Errorf("DefaultAgent = %q, want %q", cfg.DefaultAgent, "claude")
	}
	if cfg.V3Agent != "claude" {
		t.Errorf("V3Agent = %q, want %q", cfg.V3Agent, "claude")
	}
}
