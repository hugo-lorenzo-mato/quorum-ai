package web

import (
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestWebOutputNotifier_PhaseStarted(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	notifier.PhaseStarted(core.PhaseAnalyze)

	select {
	case event := <-ch:
		if event.EventType() != events.TypePhaseStarted {
			t.Errorf("expected %s, got %s", events.TypePhaseStarted, event.EventType())
		}
		if event.WorkflowID() != "wf-test-123" {
			t.Errorf("expected workflow ID wf-test-123, got %s", event.WorkflowID())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_TaskStarted(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	task := &core.Task{ID: "task-1", Name: "Test Task"}
	notifier.TaskStarted(task)

	select {
	case event := <-ch:
		if event.EventType() != events.TypeTaskStarted {
			t.Errorf("expected %s, got %s", events.TypeTaskStarted, event.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_TaskCompleted(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	task := &core.Task{
		ID:        "task-1",
		Name:      "Test Task",
		TokensIn:  100,
		TokensOut: 50,
		CostUSD:   0.005,
	}
	notifier.TaskCompleted(task, 5*time.Second)

	select {
	case event := <-ch:
		if event.EventType() != events.TypeTaskCompleted {
			t.Errorf("expected %s, got %s", events.TypeTaskCompleted, event.EventType())
		}
		e, ok := event.(events.TaskCompletedEvent)
		if !ok {
			t.Fatal("event is not TaskCompletedEvent")
		}
		if e.TaskID != "task-1" {
			t.Errorf("expected task ID task-1, got %s", e.TaskID)
		}
		if e.TokensIn != 100 {
			t.Errorf("expected tokens in 100, got %d", e.TokensIn)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_TaskCompleted_ZeroMetrics(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	task := &core.Task{ID: "task-1", Name: "Test Task"}
	notifier.TaskCompleted(task, 5*time.Second)

	select {
	case event := <-ch:
		e, ok := event.(events.TaskCompletedEvent)
		if !ok {
			t.Fatal("event is not TaskCompletedEvent")
		}
		if e.TokensIn != 0 || e.TokensOut != 0 || e.CostUSD != 0 {
			t.Errorf("expected zero metrics for zero-valued task fields")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_TaskFailed(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	task := &core.Task{ID: "task-1", Name: "Test Task"}
	notifier.TaskFailed(task, errors.New("test error"))

	select {
	case event := <-ch:
		if event.EventType() != events.TypeTaskFailed {
			t.Errorf("expected %s, got %s", events.TypeTaskFailed, event.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_TaskSkipped(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	task := &core.Task{ID: "task-1", Name: "Test Task"}
	notifier.TaskSkipped(task, "dependency failed")

	select {
	case event := <-ch:
		if event.EventType() != events.TypeTaskSkipped {
			t.Errorf("expected %s, got %s", events.TypeTaskSkipped, event.EventType())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_WorkflowStateUpdated(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	state := &core.WorkflowState{
		CurrentPhase: core.PhaseExecute,
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {Status: core.TaskStatusCompleted},
			"task-2": {Status: core.TaskStatusRunning},
			"task-3": {Status: core.TaskStatusFailed},
		},
	}
	notifier.WorkflowStateUpdated(state)

	select {
	case event := <-ch:
		e, ok := event.(events.WorkflowStateUpdatedEvent)
		if !ok {
			t.Fatal("event is not WorkflowStateUpdatedEvent")
		}
		if e.TotalTasks != 3 {
			t.Errorf("expected 3 total tasks, got %d", e.TotalTasks)
		}
		if e.Completed != 1 {
			t.Errorf("expected 1 completed, got %d", e.Completed)
		}
		if e.Failed != 1 {
			t.Errorf("expected 1 failed, got %d", e.Failed)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_Log(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	notifier.Log("info", "workflow", "test message")

	select {
	case event := <-ch:
		if event.EventType() != events.TypeLog {
			t.Errorf("expected %s, got %s", events.TypeLog, event.EventType())
		}
		e, ok := event.(events.LogEvent)
		if !ok {
			t.Fatal("event is not LogEvent")
		}
		if e.Message != "[workflow] test message" {
			t.Errorf("expected message '[workflow] test message', got %s", e.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_AgentEvent(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	data := map[string]interface{}{"key": "value"}
	notifier.AgentEvent("tool_use", "analyzer", "reading file", data)

	select {
	case event := <-ch:
		if event.EventType() != events.TypeAgentEvent {
			t.Errorf("expected %s, got %s", events.TypeAgentEvent, event.EventType())
		}
		e, ok := event.(events.AgentStreamEvent)
		if !ok {
			t.Fatal("event is not AgentStreamEvent")
		}
		if e.Agent != "analyzer" {
			t.Errorf("expected agent 'analyzer', got %s", e.Agent)
		}
		if e.Data["key"] != "value" {
			t.Errorf("expected data key 'value', got %v", e.Data["key"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestWebOutputNotifier_WorkflowLifecycle(t *testing.T) {
	bus := events.New(10)
	ch := bus.Subscribe()

	notifier := NewWebOutputNotifier(bus, "wf-test-123")

	// Test WorkflowStarted
	notifier.WorkflowStarted("Test prompt")
	event := <-ch
	if event.EventType() != events.TypeWorkflowStarted {
		t.Errorf("expected %s, got %s", events.TypeWorkflowStarted, event.EventType())
	}

	// Test WorkflowCompleted (uses priority channel)
	notifier.WorkflowCompleted(10*time.Second, 0.05)
	event = <-ch
	if event.EventType() != events.TypeWorkflowCompleted {
		t.Errorf("expected %s, got %s", events.TypeWorkflowCompleted, event.EventType())
	}

	// Test WorkflowFailed (uses priority channel)
	notifier.WorkflowFailed("execute", errors.New("test failure"))
	event = <-ch
	if event.EventType() != events.TypeWorkflowFailed {
		t.Errorf("expected %s, got %s", events.TypeWorkflowFailed, event.EventType())
	}
}

func TestWebOutputNotifier_InterfaceImplementation(t *testing.T) {
	// This test verifies that WebOutputNotifier properly implements the interface
	// through the compile-time check in notifier.go
	bus := events.New(10)
	notifier := NewWebOutputNotifier(bus, "wf-test-123")
	if notifier == nil {
		t.Fatal("expected non-nil notifier")
	}
}
