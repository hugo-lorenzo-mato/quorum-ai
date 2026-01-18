package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestTUIOutput_SendWorkflowState(t *testing.T) {
	tuiOut := NewTUIOutput()

	state := &core.WorkflowState{
		WorkflowID:   "wf-123",
		CurrentPhase: core.PhaseExecute,
		Tasks:        make(map[core.TaskID]*core.TaskState),
	}

	// Should not panic
	tuiOut.SendWorkflowState(state)

	_ = tuiOut.Close()
}

func TestTUIOutput_TaskFailed_WithError(t *testing.T) {
	tuiOut := NewTUIOutput()

	task := &core.Task{
		ID:   "task-1",
		Name: "Failing Task",
	}
	testErr := errors.New("test error message")

	// Should not panic and should capture error message
	tuiOut.TaskFailed(task, testErr)

	_ = tuiOut.Close()
}

func TestTUIOutput_ConcurrentUpdates(t *testing.T) {
	tuiOut := NewTUIOutput()

	// Simulate concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			tuiOut.Log("info", "concurrent message")
			tuiOut.PhaseStarted(core.PhaseAnalyze)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	_ = tuiOut.Close()
}

func TestOutputNotifierAdapter(t *testing.T) {
	quiet := NewQuietOutput()
	adapter := NewOutputNotifierAdapter(quiet)

	task := &core.Task{
		ID:   "task-1",
		Name: "Test Task",
	}

	// All methods should work without panic
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, time.Second)
	adapter.TaskFailed(task, errors.New("test"))
	adapter.TaskSkipped(task, "skipped")
	adapter.WorkflowStateUpdated(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
}

func TestOutputNotifierAdapter_WithFallback(t *testing.T) {
	fallback := NewFallbackOutputAdapter(true, true)
	adapter := NewOutputNotifierAdapter(fallback)

	task := &core.Task{
		ID:          "task-1",
		Name:        "Test Task",
		Description: "Description",
	}

	adapter.PhaseStarted(core.PhasePlan)
	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, 500*time.Millisecond)
	adapter.TaskFailed(task, core.ErrValidation("TEST", "test"))
	adapter.TaskSkipped(task, "skipped")
	adapter.WorkflowStateUpdated(&core.WorkflowState{
		WorkflowID: "wf-test",
		Tasks:      make(map[core.TaskID]*core.TaskState),
	})
}

func TestJSONOutputAdapter_AllMethods(t *testing.T) {
	adapter := NewJSONOutputAdapter()
	buf := &bytes.Buffer{}
	adapter.json.writer = buf
	adapter.json.enc = json.NewEncoder(buf)

	// Test all methods
	adapter.WorkflowStarted("test prompt")
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.PhaseStarted(core.PhasePlan)
	adapter.PhaseStarted(core.PhaseExecute)

	task := &core.Task{
		ID:          "task-1",
		Name:        "Test Task",
		Description: "Test description",
	}

	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, 2*time.Second)
	adapter.TaskFailed(task, errors.New("test error"))
	adapter.TaskSkipped(task, "skipped")

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Status:       core.WorkflowStatusCompleted,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Name:   "Test Task",
				Status: core.TaskStatusCompleted,
			},
		},
	}
	adapter.WorkflowStateUpdated(state)
	adapter.WorkflowCompleted(state)
	adapter.WorkflowFailed(errors.New("workflow failed"))

	adapter.Log("debug", "debug message")
	adapter.Log("info", "info message")
	adapter.Log("warn", "warn message")
	adapter.Log("error", "error message")

	_ = adapter.Close()

	// Verify output was produced
	if buf.Len() == 0 {
		t.Error("Expected JSON output to be produced")
	}
}

func TestFallbackOutputAdapter_AllMethods(t *testing.T) {
	adapter := NewFallbackOutputAdapter(true, true)

	// Test all phases
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.PhaseStarted(core.PhasePlan)
	adapter.PhaseStarted(core.PhaseExecute)

	task := &core.Task{
		ID:          "task-1",
		Name:        "Complex Task",
		Description: "A complex task description",
	}

	adapter.TaskStarted(task)
	adapter.TaskCompleted(task, 3*time.Second)
	adapter.TaskFailed(task, errors.New("task failed"))
	adapter.TaskSkipped(task, "skipped")

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Status:       core.WorkflowStatusCompleted,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Status: core.TaskStatusCompleted},
			"task-2": {ID: "task-2", Status: core.TaskStatusFailed},
		},
	}
	adapter.WorkflowStateUpdated(state)
	adapter.WorkflowCompleted(state)
	adapter.WorkflowFailed(errors.New("workflow failed"))

	adapter.Log("info", "test log message")
}

func TestNewOutput_AllModes(t *testing.T) {
	modes := []OutputMode{
		ModeTUI,
		ModePlain,
		ModeJSON,
		ModeQuiet,
		OutputMode(99), // Unknown mode should default to plain
	}

	for _, mode := range modes {
		output := NewOutput(mode, true, true)
		if output == nil {
			t.Errorf("NewOutput(%d) returned nil", mode)
		}
		_ = output.Close()
	}
}

func TestTUIOutput_AfterClose(t *testing.T) {
	tuiOut := NewTUIOutput()
	_ = tuiOut.Close()

	// Operations after close should not panic
	tuiOut.WorkflowStarted("test")
	tuiOut.PhaseStarted(core.PhaseAnalyze)
	tuiOut.TaskStarted(&core.Task{ID: "task-1"})
	tuiOut.TaskCompleted(&core.Task{ID: "task-1"}, time.Second)
	tuiOut.TaskFailed(&core.Task{ID: "task-1"}, nil)
	tuiOut.TaskSkipped(&core.Task{ID: "task-1"}, "skipped")
	tuiOut.WorkflowStateUpdated(&core.WorkflowState{Tasks: make(map[core.TaskID]*core.TaskState)})
	tuiOut.WorkflowCompleted(&core.WorkflowState{Tasks: make(map[core.TaskID]*core.TaskState)})
	tuiOut.WorkflowFailed(nil)
	tuiOut.Log("info", "test")
	tuiOut.SendWorkflowState(&core.WorkflowState{Tasks: make(map[core.TaskID]*core.TaskState)})

	// Second close should also not panic
	_ = tuiOut.Close()
}

func TestTUIOutput_NilError(t *testing.T) {
	tuiOut := NewTUIOutput()

	task := &core.Task{
		ID:   "task-1",
		Name: "Task",
	}

	// Nil error should not panic
	tuiOut.TaskFailed(task, nil)
	tuiOut.WorkflowFailed(nil)

	_ = tuiOut.Close()
}

func TestOutputNotifierAdapter_NilOutput(t *testing.T) {
	// Test with a quiet output that does nothing
	quiet := NewQuietOutput()
	adapter := NewOutputNotifierAdapter(quiet)

	// All methods should work
	adapter.PhaseStarted(core.PhaseAnalyze)
	adapter.TaskStarted(&core.Task{ID: "1"})
	adapter.TaskCompleted(&core.Task{ID: "1"}, time.Millisecond)
	adapter.TaskFailed(&core.Task{ID: "1"}, nil)
	adapter.TaskSkipped(&core.Task{ID: "1"}, "skipped")
	adapter.WorkflowStateUpdated(&core.WorkflowState{Tasks: make(map[core.TaskID]*core.TaskState)})
}

func TestQuietOutput_AllMethods(t *testing.T) {
	quiet := NewQuietOutput()

	// All methods should not panic and do nothing
	quiet.WorkflowStarted("test")
	quiet.PhaseStarted(core.PhaseAnalyze)
	quiet.PhaseStarted(core.PhasePlan)
	quiet.PhaseStarted(core.PhaseExecute)

	task := &core.Task{
		ID:          "task-1",
		Name:        "Test Task",
		Description: "Description",
		TokensIn:    100,
		TokensOut:   50,
		CostUSD:     0.01,
	}

	quiet.TaskStarted(task)
	quiet.TaskCompleted(task, time.Second)
	quiet.TaskFailed(task, errors.New("test error"))
	quiet.TaskSkipped(task, "skipped")

	state := &core.WorkflowState{
		WorkflowID:   "wf-test",
		CurrentPhase: core.PhaseExecute,
		Status:       core.WorkflowStatusCompleted,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {ID: "task-1", Status: core.TaskStatusCompleted},
		},
	}
	quiet.WorkflowStateUpdated(state)
	quiet.WorkflowCompleted(state)
	quiet.WorkflowFailed(errors.New("workflow failed"))
	quiet.Log("info", "test message")
	quiet.Log("error", "error message")

	// Close should return nil
	err := quiet.Close()
	if err != nil {
		t.Errorf("QuietOutput.Close() error = %v", err)
	}
}
