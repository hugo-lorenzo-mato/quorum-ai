package workflow

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Quick coverage tests for NopOutputNotifier

func TestNopOutputNotifier_Coverage(t *testing.T) {
	t.Parallel()

	n := NopOutputNotifier{}

	// Test all methods to improve coverage
	n.PhaseStarted(core.PhaseAnalyze)
	n.TaskStarted(&core.Task{})
	n.TaskCompleted(&core.Task{}, 0)
	n.TaskFailed(&core.Task{}, nil)
	n.TaskSkipped(&core.Task{}, "reason")
	n.WorkflowStateUpdated(&core.WorkflowState{})
	n.Log("info", "source", "message")
	n.AgentEvent("started", "claude", "message", nil)
}

// Quick coverage tests for Context methods

func TestContext_ResolveFilePath_Coverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		ctx         *Context
		path        string
		expectedSub string
	}{
		{
			name:        "empty path",
			ctx:         &Context{ProjectRoot: "/project"},
			path:        "",
			expectedSub: "",
		},
		{
			name:        "absolute path",
			ctx:         &Context{ProjectRoot: "/project"},
			path:        "/abs/path",
			expectedSub: "/abs/path",
		},
		{
			name:        "relative path with project root",
			ctx:         &Context{ProjectRoot: "/project"},
			path:        "relative/path",
			expectedSub: "/project/relative/path",
		},
		{
			name:        "relative path without project root",
			ctx:         &Context{},
			path:        "relative/path",
			expectedSub: "relative/path",
		},
		{
			name:        "nil context",
			ctx:         nil,
			path:        "test",
			expectedSub: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.ctx != nil {
				result = tt.ctx.ResolveFilePath(tt.path)
			} else {
				ctx := (*Context)(nil)
				result = ctx.ResolveFilePath(tt.path)
			}
			// Normalize paths for cross-platform comparison
			normalizedResult := filepath.ToSlash(result)
			normalizedExpected := filepath.ToSlash(tt.expectedSub)
			if normalizedResult != normalizedExpected {
				t.Errorf("ResolveFilePath() = %q, want %q", result, tt.expectedSub)
			}
		})
	}
}

func TestContext_CheckControl_Coverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ctx  *Context
	}{
		{
			name: "nil context",
			ctx:  nil,
		},
		{
			name: "nil control plane",
			ctx:  &Context{Control: nil},
		},
		{
			name: "with control plane",
			ctx:  &Context{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.CheckControl(context.Background())
			// Just verify it doesn't panic
			_ = err
		})
	}
}

func TestContext_UpdateMetrics_Coverage(t *testing.T) {
	t.Parallel()

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "test"},
	}

	ctx := &Context{
		State: state,
	}

	ctx.UpdateMetrics(func(m *core.StateMetrics) {
		m.TotalTokensIn = 100
		m.TotalTokensOut = 200
		m.ConsensusScore = 0.95
	})

	if state.Metrics == nil {
		t.Error("Metrics should be initialized")
	}
	if state.Metrics.TotalTokensIn != 100 {
		t.Errorf("TotalTokensIn = %d, want 100", state.Metrics.TotalTokensIn)
	}
}

func TestContext_UseWorkflowIsolation_Coverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      *Context
		expected bool
	}{
		{
			name:     "nil git isolation",
			ctx:      &Context{GitIsolation: nil},
			expected: false,
		},
		{
			name:     "disabled git isolation",
			ctx:      &Context{GitIsolation: &GitIsolationConfig{Enabled: false}},
			expected: false,
		},
		{
			name: "enabled but no worktree manager",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: true},
				WorkflowWorktrees: nil,
			},
			expected: false,
		},
		{
			name: "enabled with worktree but no workflow branch",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: true},
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             &core.WorkflowState{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.UseWorkflowIsolation()
			if got != tt.expected {
				t.Errorf("UseWorkflowIsolation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestResolvePhaseModel_Coverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       *Config
		agentName string
		phase     core.Phase
		taskModel string
		expected  string
	}{
		{
			name:      "task model takes precedence",
			cfg:       nil,
			agentName: "claude",
			phase:     core.PhaseAnalyze,
			taskModel: "task-model",
			expected:  "task-model",
		},
		{
			name: "phase model from config",
			cfg: &Config{
				AgentPhaseModels: map[string]map[string]string{
					"claude": {
						"analyze": "phase-model",
					},
				},
			},
			agentName: "claude",
			phase:     core.PhaseAnalyze,
			taskModel: "",
			expected:  "phase-model",
		},
		{
			name:      "no model configured",
			cfg:       nil,
			agentName: "claude",
			phase:     core.PhaseAnalyze,
			taskModel: "",
			expected:  "",
		},
		{
			name: "whitespace task model ignored",
			cfg: &Config{
				AgentPhaseModels: map[string]map[string]string{
					"claude": {
						"analyze": "phase-model",
					},
				},
			},
			agentName: "claude",
			phase:     core.PhaseAnalyze,
			taskModel: "  \t  ",
			expected:  "phase-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePhaseModel(tt.cfg, tt.agentName, tt.phase, tt.taskModel)
			if got != tt.expected {
				t.Errorf("ResolvePhaseModel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildContextString_Coverage(t *testing.T) {
	t.Parallel()

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test",
		},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {
					Name:   "Task 1",
					Status: core.TaskStatusCompleted,
				},
				"task-2": {
					Name:   "Task 2",
					Status: core.TaskStatusRunning,
				},
			},
			TaskOrder: []core.TaskID{"task-1", "task-2"},
		},
	}

	result := BuildContextString(state)

	if result == "" {
		t.Error("BuildContextString() returned empty string")
	}
	if !builderContains(result, "wf-test") {
		t.Error("BuildContextString() should contain workflow ID")
	}
	if !builderContains(result, "execute") {
		t.Error("BuildContextString() should contain phase")
	}
}

func TestContext_GetContextString_Coverage(t *testing.T) {
	t.Parallel()

	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-test"},
		WorkflowRun:        core.WorkflowRun{CurrentPhase: core.PhasePlan},
	}

	ctx := &Context{
		State: state,
	}

	result := ctx.GetContextString()

	if result == "" {
		t.Error("GetContextString() returned empty string")
	}
}

// Helper function from builder tests
func builderContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
