package tui

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestNewOutput(t *testing.T) {
	tests := []struct {
		name     string
		mode     OutputMode
		wantType string
	}{
		{"TUI mode", ModeTUI, "*tui.TUIOutput"},
		{"Plain mode", ModePlain, "*tui.FallbackOutputAdapter"},
		{"JSON mode", ModeJSON, "*tui.JSONOutputAdapter"},
		{"Quiet mode", ModeQuiet, "*tui.QuietOutput"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := NewOutput(tt.mode, false, false)
			if output == nil {
				t.Error("NewOutput returned nil")
			}
			_ = output.Close()
		})
	}
}

func TestFallbackOutputAdapter_Interface(t *testing.T) {
	adapter := NewFallbackOutputAdapter(false, false)

	// Test that it implements Output interface
	var _ Output = adapter

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	// These should not panic
	adapter.WorkflowStarted("test prompt")
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, 100*time.Millisecond)
	adapter.TaskFailed(task, nil)
	adapter.WorkflowCompleted(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
	adapter.WorkflowFailed(nil)
	adapter.Log("info", "test message")
	_ = adapter.Close()
}

func TestJSONOutputAdapter_Interface(t *testing.T) {
	adapter := NewJSONOutputAdapter()

	// Test that it implements Output interface
	var _ Output = adapter

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	// Redirect output for testing
	buf := &bytes.Buffer{}
	adapter.json.writer = buf
	adapter.json.enc = json.NewEncoder(buf)

	testErr := core.ErrValidation("TEST", "test error")

	adapter.WorkflowStarted("test prompt")
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, 100*time.Millisecond)
	adapter.TaskFailed(task, testErr)
	adapter.WorkflowCompleted(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
	adapter.WorkflowFailed(testErr)
	adapter.Log("info", "test message")
	_ = adapter.Close()

	// Verify JSON output was produced
	if buf.Len() == 0 {
		t.Error("Expected JSON output")
	}
}

func TestQuietOutput_Interface(t *testing.T) {
	quiet := NewQuietOutput()

	// Test that it implements Output interface
	var _ Output = quiet

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	// All methods should be no-ops
	quiet.WorkflowStarted("test prompt")
	quiet.PhaseStarted(core.PhaseAnalyze)
	quiet.TaskStarted(task)
	quiet.TaskCompleted(task, 100*time.Millisecond)
	quiet.TaskFailed(task, nil)
	quiet.WorkflowCompleted(&core.WorkflowState{})
	quiet.WorkflowFailed(nil)
	quiet.Log("info", "test message")
	_ = quiet.Close()
}

func TestTUIOutput_Interface(t *testing.T) {
	tuiOut := NewTUIOutput()

	// Test that it implements Output interface
	var _ Output = tuiOut

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	// These should not panic (they queue messages)
	tuiOut.WorkflowStarted("test prompt")
	tuiOut.PhaseStarted(core.PhaseAnalyze)
	tuiOut.TaskStarted(task)
	tuiOut.TaskCompleted(task, 100*time.Millisecond)
	tuiOut.TaskFailed(task, nil)
	tuiOut.WorkflowCompleted(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
	tuiOut.WorkflowFailed(nil)
	tuiOut.Log("info", "test message")

	// Close should clean up
	_ = tuiOut.Close()

	// After close, methods should not panic
	tuiOut.Log("info", "after close")
}
