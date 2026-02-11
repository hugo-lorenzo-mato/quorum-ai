package cmd

import (
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// drainOne reads exactly one event from ch, failing the test on timeout.
func drainOne(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

// assertNoEvent verifies nothing arrives on ch within a short window.
func assertNoEvent(t *testing.T, ch <-chan events.Event) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event received: %v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// ---------------------------------------------------------------------------
// chatOutputNotifier
// ---------------------------------------------------------------------------

func TestChatNotifier_PhaseStarted_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	// Should not panic.
	n.PhaseStarted(core.PhaseAnalyze)
}

func TestChatNotifier_PhaseStarted_ValidEventBus(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.PhaseStarted(core.PhaseAnalyze)

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypePhaseStarted {
		t.Fatalf("expected event type %q, got %q", events.TypePhaseStarted, ev.EventType())
	}
	pse, ok := ev.(events.PhaseStartedEvent)
	if !ok {
		t.Fatalf("expected PhaseStartedEvent, got %T", ev)
	}
	if pse.Phase != "analyze" {
		t.Fatalf("expected phase %q, got %q", "analyze", pse.Phase)
	}
}

func TestChatNotifier_TaskStarted_NilTask(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.TaskStarted(nil) // nil task → no publish
	assertNoEvent(t, ch)
}

func TestChatNotifier_TaskStarted_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	task := &core.Task{ID: "t1"}
	n.TaskStarted(task) // nil bus → no panic
}

func TestChatNotifier_TaskStarted_Valid(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	task := &core.Task{ID: "task-abc"}
	n.TaskStarted(task)

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypeTaskStarted {
		t.Fatalf("expected %q, got %q", events.TypeTaskStarted, ev.EventType())
	}
	tse, ok := ev.(events.TaskStartedEvent)
	if !ok {
		t.Fatalf("expected TaskStartedEvent, got %T", ev)
	}
	if tse.TaskID != "task-abc" {
		t.Fatalf("expected task id %q, got %q", "task-abc", tse.TaskID)
	}
}

func TestChatNotifier_TaskCompleted_NilTask(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.TaskCompleted(nil, 5*time.Second)
	assertNoEvent(t, ch)
}

func TestChatNotifier_TaskCompleted_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	task := &core.Task{ID: "t1", TokensIn: 10, TokensOut: 20}
	n.TaskCompleted(task, 5*time.Second)
}

func TestChatNotifier_TaskCompleted_Valid(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	task := &core.Task{ID: "tc-1", TokensIn: 100, TokensOut: 200}
	n.TaskCompleted(task, 3*time.Second)

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypeTaskCompleted {
		t.Fatalf("expected %q, got %q", events.TypeTaskCompleted, ev.EventType())
	}
	tce, ok := ev.(events.TaskCompletedEvent)
	if !ok {
		t.Fatalf("expected TaskCompletedEvent, got %T", ev)
	}
	if tce.TaskID != "tc-1" {
		t.Fatalf("expected task id %q, got %q", "tc-1", tce.TaskID)
	}
	if tce.TokensIn != 100 || tce.TokensOut != 200 {
		t.Fatalf("tokens mismatch: in=%d out=%d", tce.TokensIn, tce.TokensOut)
	}
	if tce.Duration != 3*time.Second {
		t.Fatalf("expected duration 3s, got %v", tce.Duration)
	}
}

func TestChatNotifier_TaskFailed_NilTask(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.TaskFailed(nil, errors.New("boom"))
	assertNoEvent(t, ch)
}

func TestChatNotifier_TaskFailed_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	task := &core.Task{ID: "t1"}
	n.TaskFailed(task, errors.New("boom"))
}

func TestChatNotifier_TaskFailed_Valid(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	task := &core.Task{ID: "tf-1", Retries: 1}
	n.TaskFailed(task, errors.New("timeout"))

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypeTaskFailed {
		t.Fatalf("expected %q, got %q", events.TypeTaskFailed, ev.EventType())
	}
	tfe, ok := ev.(events.TaskFailedEvent)
	if !ok {
		t.Fatalf("expected TaskFailedEvent, got %T", ev)
	}
	if tfe.TaskID != "tf-1" {
		t.Fatalf("expected task id %q, got %q", "tf-1", tfe.TaskID)
	}
	if !tfe.Retryable {
		t.Fatalf("expected retryable=true (Retries>0)")
	}
}

func TestChatNotifier_TaskSkipped_NilTask(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.TaskSkipped(nil, "dep failed")
	assertNoEvent(t, ch)
}

func TestChatNotifier_TaskSkipped_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	task := &core.Task{ID: "t1"}
	n.TaskSkipped(task, "dep failed")
}

func TestChatNotifier_TaskSkipped_Valid(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	task := &core.Task{ID: "ts-1"}
	n.TaskSkipped(task, "dependency failed")

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypeTaskSkipped {
		t.Fatalf("expected %q, got %q", events.TypeTaskSkipped, ev.EventType())
	}
	tse, ok := ev.(events.TaskSkippedEvent)
	if !ok {
		t.Fatalf("expected TaskSkippedEvent, got %T", ev)
	}
	if tse.TaskID != "ts-1" {
		t.Fatalf("expected task id %q, got %q", "ts-1", tse.TaskID)
	}
	if tse.Reason != "dependency failed" {
		t.Fatalf("expected reason %q, got %q", "dependency failed", tse.Reason)
	}
}

func TestChatNotifier_WorkflowStateUpdated_NilState(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.WorkflowStateUpdated(nil)
	assertNoEvent(t, ch)
}

func TestChatNotifier_WorkflowStateUpdated_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	state := &core.WorkflowState{}
	n.WorkflowStateUpdated(state)
}

func TestChatNotifier_WorkflowStateUpdated_EmptyTasks(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
		},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        map[core.TaskID]*core.TaskState{},
		},
	}
	n.WorkflowStateUpdated(state)

	ev := drainOne(t, ch)
	wse, ok := ev.(events.WorkflowStateUpdatedEvent)
	if !ok {
		t.Fatalf("expected WorkflowStateUpdatedEvent, got %T", ev)
	}
	if wse.Phase != "analyze" {
		t.Fatalf("expected phase %q, got %q", "analyze", wse.Phase)
	}
	if wse.TotalTasks != 0 || wse.Completed != 0 || wse.Failed != 0 || wse.Skipped != 0 {
		t.Fatalf("expected all zeros, got total=%d completed=%d failed=%d skipped=%d",
			wse.TotalTasks, wse.Completed, wse.Failed, wse.Skipped)
	}
}

func TestChatNotifier_WorkflowStateUpdated_MixedStatuses(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-2",
		},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {ID: "t1", Status: core.TaskStatusCompleted},
				"t2": {ID: "t2", Status: core.TaskStatusCompleted},
				"t3": {ID: "t3", Status: core.TaskStatusFailed},
				"t4": {ID: "t4", Status: core.TaskStatusSkipped},
				"t5": {ID: "t5", Status: core.TaskStatusRunning},
				"t6": {ID: "t6", Status: core.TaskStatusPending},
			},
		},
	}
	n.WorkflowStateUpdated(state)

	ev := drainOne(t, ch)
	wse, ok := ev.(events.WorkflowStateUpdatedEvent)
	if !ok {
		t.Fatalf("expected WorkflowStateUpdatedEvent, got %T", ev)
	}
	if wse.TotalTasks != 6 {
		t.Fatalf("expected 6 total tasks, got %d", wse.TotalTasks)
	}
	if wse.Completed != 2 {
		t.Fatalf("expected 2 completed, got %d", wse.Completed)
	}
	if wse.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", wse.Failed)
	}
	if wse.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", wse.Skipped)
	}
}

func TestChatNotifier_Log_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	n.Log("info", "runner", "hello") // no panic
}

func TestChatNotifier_Log_ValidEventBus(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &chatOutputNotifier{eventBus: bus}
	n.Log("warn", "planner", "something happened")

	ev := drainOne(t, ch)
	if ev.EventType() != "log" {
		t.Fatalf("expected event type %q, got %q", "log", ev.EventType())
	}
}

func TestChatNotifier_AgentEvent_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}
	n.AgentEvent("tool_use", "claude", "reading file", map[string]interface{}{"tool": "read"})
}

func TestChatNotifier_AgentEvent_ValidEventBus(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	data := map[string]interface{}{"key": "value"}
	n := &chatOutputNotifier{eventBus: bus}
	n.AgentEvent("tool_use", "gemini", "searching", data)

	ev := drainOne(t, ch)
	if ev.EventType() != events.TypeAgentEvent {
		t.Fatalf("expected %q, got %q", events.TypeAgentEvent, ev.EventType())
	}
	ase, ok := ev.(events.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", ev)
	}
	if ase.Agent != "gemini" {
		t.Fatalf("expected agent %q, got %q", "gemini", ase.Agent)
	}
	if ase.Message != "searching" {
		t.Fatalf("expected message %q, got %q", "searching", ase.Message)
	}
	if ase.Data["key"] != "value" {
		t.Fatalf("expected data key=value, got %v", ase.Data)
	}
}

// ---------------------------------------------------------------------------
// tracingChatOutputNotifier
// ---------------------------------------------------------------------------

func TestTracingNotifier_PhaseStarted_NilTracer(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{tracer: nil}
	n.PhaseStarted(core.PhaseAnalyze) // safe no-op
}

func TestTracingNotifier_TaskStarted_NilTracer(t *testing.T) {
	t.Parallel()
	// With nil tracer the guard `if n.tracer != nil` prevents dereferencing task.
	n := &tracingChatOutputNotifier{tracer: nil}
	task := &core.Task{ID: "t1", Name: "test", CLI: "claude"}
	n.TaskStarted(task) // safe no-op
}

func TestTracingNotifier_TaskCompleted_NilTracer(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{tracer: nil}
	task := &core.Task{ID: "t1", Name: "test", TokensIn: 50, TokensOut: 100}
	n.TaskCompleted(task, 2*time.Second) // safe no-op
}

func TestTracingNotifier_TaskFailed_NilTracer(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{tracer: nil}
	task := &core.Task{ID: "t1", Name: "test"}
	n.TaskFailed(task, errors.New("oops")) // safe no-op
}

func TestTracingNotifier_TaskSkipped_Noop(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{tracer: nil}
	task := &core.Task{ID: "t1"}
	n.TaskSkipped(task, "reason") // always a no-op
}

func TestTracingNotifier_WorkflowStateUpdated_NilTracer(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{tracer: nil}
	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Status: core.WorkflowStatusRunning,
			Tasks:  map[core.TaskID]*core.TaskState{"t1": {Status: core.TaskStatusCompleted}},
		},
	}
	n.WorkflowStateUpdated(state) // safe no-op
}

func TestTracingNotifier_Log_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{eventBus: nil, tracer: nil}
	n.Log("info", "src", "msg") // no panic
}

func TestTracingNotifier_Log_ValidEventBus(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	n := &tracingChatOutputNotifier{eventBus: bus, tracer: nil}
	n.Log("error", "executor", "failed to run")

	ev := drainOne(t, ch)
	if ev.EventType() != "log" {
		t.Fatalf("expected %q, got %q", "log", ev.EventType())
	}
}

func TestTracingNotifier_AgentEvent_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{eventBus: nil, tracer: nil}
	n.AgentEvent("thinking", "codex", "reasoning...", nil)
}

func TestTracingNotifier_AgentEvent_ValidEventBus(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()
	ch := bus.Subscribe()

	data := map[string]interface{}{"step": 3}
	n := &tracingChatOutputNotifier{eventBus: bus, tracer: nil}
	n.AgentEvent("progress", "claude", "step 3", data)

	ev := drainOne(t, ch)
	ase, ok := ev.(events.AgentStreamEvent)
	if !ok {
		t.Fatalf("expected AgentStreamEvent, got %T", ev)
	}
	if ase.Agent != "claude" {
		t.Fatalf("expected agent %q, got %q", "claude", ase.Agent)
	}
	if ase.Data["step"] != 3 {
		t.Fatalf("expected data step=3, got %v", ase.Data)
	}
}

// ---------------------------------------------------------------------------
// defaultAgentName (additional edge cases)
// ---------------------------------------------------------------------------

func TestDefaultAgentName_Empty(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	if got := defaultAgentName(cfg); got != "claude" {
		t.Fatalf("expected %q, got %q", "claude", got)
	}
}

func TestDefaultAgentName_Set(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Agents.Default = "gemini"
	if got := defaultAgentName(cfg); got != "gemini" {
		t.Fatalf("expected %q, got %q", "gemini", got)
	}
}

// ---------------------------------------------------------------------------
// connectRegistryToOutputNotifier — nil notifier early return
// ---------------------------------------------------------------------------

func TestConnectRegistry_NilNotifier(t *testing.T) {
	t.Parallel()
	reg := cli.NewRegistry()
	// Should return immediately without panic.
	connectRegistryToOutputNotifier(reg, nil)
}
