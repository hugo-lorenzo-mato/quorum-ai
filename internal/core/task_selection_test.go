package core

import (
	"testing"
	"time"
)

func TestApplyTaskSelection_ClosureAndSkip(t *testing.T) {
	now := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	state := &WorkflowState{
		WorkflowRun: WorkflowRun{
			Tasks: map[TaskID]*TaskState{
				"a": {ID: "a", Name: "A", Status: TaskStatusPending},
				"b": {ID: "b", Name: "B", Status: TaskStatusPending, Dependencies: []TaskID{"a"}},
				"c": {ID: "c", Name: "C", Status: TaskStatusPending},
			},
			TaskOrder: []TaskID{"a", "b", "c"},
		},
	}

	res, err := ApplyTaskSelection(state, []TaskID{"b"}, now)
	if err != nil {
		t.Fatalf("ApplyTaskSelection() error = %v", err)
	}

	// Selected should be exactly what user asked (deduped).
	if len(res.SelectedTaskIDs) != 1 || res.SelectedTaskIDs[0] != "b" {
		t.Fatalf("SelectedTaskIDs = %#v, want [b]", res.SelectedTaskIDs)
	}

	// Effective should include dependencies in TaskOrder order.
	if got, want := len(res.EffectiveSelectedTaskIDs), 2; got != want {
		t.Fatalf("EffectiveSelectedTaskIDs len = %d, want %d (%#v)", got, want, res.EffectiveSelectedTaskIDs)
	}
	if res.EffectiveSelectedTaskIDs[0] != "a" || res.EffectiveSelectedTaskIDs[1] != "b" {
		t.Fatalf("EffectiveSelectedTaskIDs = %#v, want [a b]", res.EffectiveSelectedTaskIDs)
	}

	// Non-selected pending tasks should be skipped.
	if state.Tasks["c"].Status != TaskStatusSkipped {
		t.Fatalf("task c status = %s, want skipped", state.Tasks["c"].Status)
	}
	if state.Tasks["c"].Error != SkipReasonNotSelected {
		t.Fatalf("task c error = %q, want %q", state.Tasks["c"].Error, SkipReasonNotSelected)
	}
	if state.Tasks["c"].CompletedAt == nil || !state.Tasks["c"].CompletedAt.Equal(now) {
		t.Fatalf("task c completed_at = %#v, want %v", state.Tasks["c"].CompletedAt, now)
	}

	// Selected tasks should remain pending.
	if state.Tasks["a"].Status != TaskStatusPending {
		t.Fatalf("task a status = %s, want pending", state.Tasks["a"].Status)
	}
	if state.Tasks["b"].Status != TaskStatusPending {
		t.Fatalf("task b status = %s, want pending", state.Tasks["b"].Status)
	}
}

func TestApplyTaskSelection_DeduplicatesSelectedTaskIDs(t *testing.T) {
	state := &WorkflowState{
		WorkflowRun: WorkflowRun{
			Tasks: map[TaskID]*TaskState{
				"a": {ID: "a", Name: "A", Status: TaskStatusPending},
			},
			TaskOrder: []TaskID{"a"},
		},
	}

	res, err := ApplyTaskSelection(state, []TaskID{"a", "a"}, time.Now())
	if err != nil {
		t.Fatalf("ApplyTaskSelection() error = %v", err)
	}
	if len(res.SelectedTaskIDs) != 1 || res.SelectedTaskIDs[0] != "a" {
		t.Fatalf("SelectedTaskIDs = %#v, want [a]", res.SelectedTaskIDs)
	}
	if len(res.SkippedTaskIDs) != 0 {
		t.Fatalf("SkippedTaskIDs = %#v, want []", res.SkippedTaskIDs)
	}
}

func TestApplyTaskSelection_EmptySelectionRejected(t *testing.T) {
	state := &WorkflowState{
		WorkflowRun: WorkflowRun{
			Tasks: map[TaskID]*TaskState{
				"a": {ID: "a", Name: "A", Status: TaskStatusPending},
			},
		},
	}
	_, err := ApplyTaskSelection(state, nil, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyTaskSelection_UnknownTaskRejected(t *testing.T) {
	state := &WorkflowState{
		WorkflowRun: WorkflowRun{
			Tasks: map[TaskID]*TaskState{
				"a": {ID: "a", Name: "A", Status: TaskStatusPending},
			},
		},
	}
	_, err := ApplyTaskSelection(state, []TaskID{"nope"}, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}
