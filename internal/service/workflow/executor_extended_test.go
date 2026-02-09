package workflow

import (
	"testing"
)

func TestShouldUseWorktrees(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		mode       string
		readyCount int
		want       bool
	}{
		{"empty mode", "", 1, true},
		{"always mode", "always", 1, true},
		{"always mode uppercase", "ALWAYS", 1, true},
		{"parallel mode single task", "parallel", 1, false},
		{"parallel mode multiple tasks", "parallel", 2, true},
		{"parallel mode many tasks", "parallel", 5, true},
		{"disabled mode", "disabled", 1, false},
		{"disabled mode uppercase", "DISABLED", 5, false},
		{"off mode", "off", 1, false},
		{"false mode", "false", 1, false},
		{"unknown mode defaults to true", "unknown", 1, true},
		{"whitespace handling", "  parallel  ", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseWorktrees(tt.mode, tt.readyCount)
			if got != tt.want {
				t.Errorf("shouldUseWorktrees(%q, %d) = %v, want %v", tt.mode, tt.readyCount, got, tt.want)
			}
		})
	}
}

func TestNewExecutor(t *testing.T) {
	t.Parallel()
	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	denyTools := []string{"rm", "sudo"}

	executor := NewExecutor(dag, saver, denyTools)

	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}
	if executor.dag != dag {
		t.Error("dag not set correctly")
	}
	if executor.stateSaver != saver {
		t.Error("stateSaver not set correctly")
	}
	if len(executor.denyTools) != 2 {
		t.Errorf("denyTools length = %d, want 2", len(executor.denyTools))
	}
}
