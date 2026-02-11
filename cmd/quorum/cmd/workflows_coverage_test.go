package cmd

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// --- formatStatus ---

func TestFormatStatus_Pending(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatusPending); got != "pending" {
		t.Errorf("expected pending, got %s", got)
	}
}

func TestFormatStatus_Running(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatusRunning); got != "running" {
		t.Errorf("expected running, got %s", got)
	}
}

func TestFormatStatus_Completed(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatusCompleted); got != "completed" {
		t.Errorf("expected completed, got %s", got)
	}
}

func TestFormatStatus_Failed(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatusFailed); got != "failed" {
		t.Errorf("expected failed, got %s", got)
	}
}

func TestFormatStatus_Unknown(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatus("custom")); got != "custom" {
		t.Errorf("expected custom, got %s", got)
	}
}

func TestFormatStatus_Empty(t *testing.T) {
	t.Parallel()
	if got := formatStatus(core.WorkflowStatus("")); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

// --- formatPhase ---

func TestFormatPhase_Refine(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.PhaseRefine); got != "refine" {
		t.Errorf("expected refine, got %s", got)
	}
}

func TestFormatPhase_Analyze(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.PhaseAnalyze); got != "analyze" {
		t.Errorf("expected analyze, got %s", got)
	}
}

func TestFormatPhase_Plan(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.PhasePlan); got != "plan" {
		t.Errorf("expected plan, got %s", got)
	}
}

func TestFormatPhase_Execute(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.PhaseExecute); got != "execute" {
		t.Errorf("expected execute, got %s", got)
	}
}

func TestFormatPhase_Unknown(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.Phase("custom")); got != "custom" {
		t.Errorf("expected custom, got %s", got)
	}
}

func TestFormatPhase_Empty(t *testing.T) {
	t.Parallel()
	if got := formatPhase(core.Phase("")); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

// --- formatWorkflowTime ---

func TestFormatWorkflowTime_ZeroTime(t *testing.T) {
	t.Parallel()
	if got := formatWorkflowTime(time.Time{}); got != "-" {
		t.Errorf("expected -, got %s", got)
	}
}

func TestFormatWorkflowTime_ValidTime(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	expected := "2024-03-15 10:30"
	if got := formatWorkflowTime(tm); got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestFormatWorkflowTime_MinTime(t *testing.T) {
	t.Parallel()
	// Non-zero but very old time
	tm := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	got := formatWorkflowTime(tm)
	if got == "-" {
		t.Error("non-zero time should not return -")
	}
	if got != "1970-01-01 00:00" {
		t.Errorf("unexpected format: %s", got)
	}
}

// --- truncateString ---

func TestTruncateString_ShortString(t *testing.T) {
	t.Parallel()
	if got := truncateString("hello", 10); got != "hello" {
		t.Errorf("expected hello, got %s", got)
	}
}

func TestTruncateString_ExactLength(t *testing.T) {
	t.Parallel()
	if got := truncateString("hello", 5); got != "hello" {
		t.Errorf("expected hello, got %s", got)
	}
}

func TestTruncateString_LongString(t *testing.T) {
	t.Parallel()
	got := truncateString("hello world foo bar", 10)
	if got != "hello w..." {
		t.Errorf("expected 'hello w...', got '%s'", got)
	}
}

func TestTruncateString_EmptyString(t *testing.T) {
	t.Parallel()
	if got := truncateString("", 10); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}

func TestTruncateString_WithNewlines(t *testing.T) {
	t.Parallel()
	got := truncateString("line1\nline2\rline3", 50)
	if got != "line1 line2line3" {
		t.Errorf("expected newlines removed, got %s", got)
	}
}

func TestTruncateString_WithNewlinesAndTruncation(t *testing.T) {
	t.Parallel()
	got := truncateString("line1\nline2\nline3\nline4", 15)
	if got != "line1 line2 ..." {
		t.Errorf("expected truncated with newlines removed, got '%s'", got)
	}
}

func TestTruncateString_OnlyNewlines(t *testing.T) {
	t.Parallel()
	got := truncateString("\n\n\n", 10)
	if got != "   " {
		t.Errorf("expected spaces, got '%s'", got)
	}
}

func TestTruncateString_CarriageReturn(t *testing.T) {
	t.Parallel()
	got := truncateString("hello\r\nworld", 50)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", got)
	}
}

// --- dependencyIndices (from interactive.go) ---

func TestDependencyIndices_Normal(t *testing.T) {
	t.Parallel()
	indexByID := map[core.TaskID]int{
		"t1": 1,
		"t2": 2,
		"t3": 3,
	}
	deps := []core.TaskID{"t1", "t3"}
	result := dependencyIndices(deps, indexByID)
	if len(result) != 2 {
		t.Fatalf("expected 2 indices, got %d", len(result))
	}
	if result[0] != "1" || result[1] != "3" {
		t.Errorf("expected [1, 3], got %v", result)
	}
}

func TestDependencyIndices_Empty(t *testing.T) {
	t.Parallel()
	indexByID := map[core.TaskID]int{"t1": 1}
	result := dependencyIndices(nil, indexByID)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestDependencyIndices_MissingDep(t *testing.T) {
	t.Parallel()
	indexByID := map[core.TaskID]int{"t1": 1}
	deps := []core.TaskID{"t1", "t2"}
	result := dependencyIndices(deps, indexByID)
	if len(result) != 1 {
		t.Fatalf("expected 1 index (t2 missing), got %d", len(result))
	}
	if result[0] != "1" {
		t.Errorf("expected [1], got %v", result)
	}
}

// --- chatOutputNotifier ---

func TestChatOutputNotifier_NilEventBus(t *testing.T) {
	t.Parallel()
	n := &chatOutputNotifier{eventBus: nil}

	// All methods should not panic with nil event bus
	n.PhaseStarted(core.PhaseAnalyze)
	n.TaskStarted(nil)
	n.TaskStarted(&core.Task{ID: "t1"})
	n.TaskCompleted(nil, time.Second)
	n.TaskCompleted(&core.Task{ID: "t1"}, time.Second)
	n.TaskFailed(nil, nil)
	n.TaskFailed(&core.Task{ID: "t1"}, nil)
	n.TaskSkipped(nil, "reason")
	n.TaskSkipped(&core.Task{ID: "t1"}, "reason")
	n.WorkflowStateUpdated(nil)
	n.WorkflowStateUpdated(&core.WorkflowState{})
	n.Log("info", "source", "message")
	n.AgentEvent("kind", "agent", "message", nil)
}

func TestChatOutputNotifier_WorkflowStateUpdated_CountsTasks(t *testing.T) {
	t.Parallel()
	// We just verify no panics with various task states
	n := &chatOutputNotifier{eventBus: nil}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			CurrentPhase: core.PhaseExecute,
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusCompleted},
				"t2": {Status: core.TaskStatusFailed},
				"t3": {Status: core.TaskStatusSkipped},
				"t4": {Status: core.TaskStatusPending},
			},
		},
	}
	n.WorkflowStateUpdated(state) // Should not panic
}

// --- tracingChatOutputNotifier ---

func TestTracingChatOutputNotifier_NilTracer(t *testing.T) {
	t.Parallel()
	n := &tracingChatOutputNotifier{eventBus: nil, tracer: nil}

	// All methods should not panic with nil tracer
	n.PhaseStarted(core.PhaseAnalyze)
	n.WorkflowStateUpdated(&core.WorkflowState{})
	n.TaskSkipped(&core.Task{ID: "t1"}, "reason") // noop implementation
	n.Log("info", "source", "message")
	n.AgentEvent("kind", "agent", "message", nil)
}

// --- formatTime (from trace.go) ---

func TestFormatTime_ZeroTime(t *testing.T) {
	t.Parallel()
	if got := formatTime(time.Time{}); got != "-" {
		t.Errorf("expected -, got %s", got)
	}
}

func TestFormatTime_ValidTime(t *testing.T) {
	t.Parallel()
	tm := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	got := formatTime(tm)
	if got == "-" {
		t.Error("non-zero time should not return -")
	}
	expected := "2024-03-15T10:30:00Z"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
