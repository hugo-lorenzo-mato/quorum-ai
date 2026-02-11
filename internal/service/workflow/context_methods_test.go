package workflow

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Quick coverage tests for NopOutputNotifier


// Quick coverage tests for Context methods

func TestContext_ResolveFilePath(t *testing.T) {
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

func TestContext_CheckControl(t *testing.T) {
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

func TestContext_UpdateMetrics(t *testing.T) {
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




func TestContext_GetContextString(t *testing.T) {
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
