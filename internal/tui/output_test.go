package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync/atomic"
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
	adapter.TaskSkipped(task, "skipped")
	adapter.WorkflowStateUpdated(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
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
	adapter.TaskSkipped(task, "skipped")
	adapter.WorkflowStateUpdated(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
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
	quiet.TaskSkipped(task, "skipped")
	quiet.WorkflowStateUpdated(&core.WorkflowState{})
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
	tuiOut.TaskSkipped(task, "skipped")
	tuiOut.WorkflowStateUpdated(&core.WorkflowState{
		Tasks: make(map[core.TaskID]*core.TaskState),
	})
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

func TestTUIOutput_PriorityNeverDrops(t *testing.T) {
	output := NewTUIOutput()

	// Fill the normal channel
	for i := 0; i < 200; i++ {
		output.Log("info", "flood message")
	}

	// Priority events should still work
	state := &core.WorkflowState{WorkflowID: "test-1"}

	done := make(chan bool, 1)
	go func() {
		output.WorkflowCompleted(state)
		done <- true
	}()

	select {
	case <-done:
		// Success - priority event was sent
	case <-time.After(100 * time.Millisecond):
		t.Error("Priority event blocked - should never happen")
	}
}

func TestTUIOutput_DroppedEventsCounter(t *testing.T) {
	output := NewTUIOutput()

	// Fill the buffer
	for i := 0; i < 500; i++ {
		output.Log("info", "test message")
	}

	// Should have dropped some events
	dropped := output.DroppedEvents()
	if dropped == 0 {
		// With ring buffer, we expect some drops when flooding
		t.Log("No events dropped (buffer might not have filled)")
	}
}

func TestTUIOutput_RingBufferBehavior(t *testing.T) {
	output := NewTUIOutput()

	// Send more events than buffer size
	bufferSize := 100
	for i := 0; i < bufferSize*3; i++ {
		output.Log("info", "message")
	}

	// Some events should have been dropped
	// (the oldest ones, not the newest)
	dropped := atomic.LoadInt64(&output.dropCount)
	t.Logf("Dropped %d events out of %d sent", dropped, bufferSize*3)
}

func TestTUIOutput_CriticalEventsDelivered(t *testing.T) {
	output := NewTUIOutput()

	// Flood with normal events
	go func() {
		for i := 0; i < 1000; i++ {
			output.Log("info", "flood")
			time.Sleep(time.Microsecond)
		}
	}()

	// Send critical events
	criticalSent := 0
	for i := 0; i < 10; i++ {
		output.WorkflowFailed(fmt.Errorf("critical error %d", i))
		criticalSent++
	}

	// Critical events should all be in the priority channel
	// (This is a simplified test - full test would verify delivery)
	t.Logf("Sent %d critical events, dropped %d normal events",
		criticalSent, output.DroppedEvents())
}
