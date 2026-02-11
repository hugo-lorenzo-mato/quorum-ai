package core

import (
	"fmt"
	"time"
)

// SkipReasonNotSelected is stored in TaskState.Error when a task is skipped because it
// was not selected for execution.
const SkipReasonNotSelected = "skipped: not selected for this execution"

// TaskSelectionResult describes the outcome of applying a task selection to a workflow.
//
// SelectedTaskIDs are the user-provided selections (deduplicated, in provided order).
// EffectiveSelectedTaskIDs include SelectedTaskIDs plus all transitive dependencies.
// SkippedTaskIDs are tasks that were pending and were marked as skipped.
type TaskSelectionResult struct {
	SelectedTaskIDs          []TaskID
	EffectiveSelectedTaskIDs []TaskID
	SkippedTaskIDs           []TaskID
}

// ApplyTaskSelection marks any non-selected pending tasks as skipped.
//
// The selection is expanded by dependency closure, so selecting a task automatically
// selects its dependencies.
//
// This function mutates state in-place.
func ApplyTaskSelection(state *WorkflowState, selectedTaskIDs []TaskID, now time.Time) (*TaskSelectionResult, error) {
	if state == nil {
		return nil, ErrValidation("NIL_STATE", "workflow state cannot be nil")
	}
	if len(state.Tasks) == 0 {
		return nil, ErrValidation("NO_TASKS", "no tasks available for selection")
	}
	if len(selectedTaskIDs) == 0 {
		return nil, ErrValidation("EMPTY_SELECTION", "selected_task_ids cannot be empty")
	}

	// Deduplicate while preserving the provided order.
	selected := make([]TaskID, 0, len(selectedTaskIDs))
	seen := make(map[TaskID]bool, len(selectedTaskIDs))
	for _, rawID := range selectedTaskIDs {
		id := TaskID(rawID)
		if id == "" {
			return nil, ErrValidation("INVALID_TASK_ID", "task id cannot be empty")
		}
		if seen[id] {
			continue
		}
		if _, ok := state.Tasks[id]; !ok {
			return nil, ErrValidation("UNKNOWN_TASK_ID", fmt.Sprintf("unknown task id: %s", id))
		}
		seen[id] = true
		selected = append(selected, id)
	}
	if len(selected) == 0 {
		return nil, ErrValidation("EMPTY_SELECTION", "selected_task_ids cannot be empty")
	}

	// Compute dependency closure.
	closure := make(map[TaskID]bool, len(state.Tasks))
	stack := append([]TaskID{}, selected...)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if closure[id] {
			continue
		}
		closure[id] = true

		ts := state.Tasks[id]
		if ts == nil {
			return nil, ErrState("TASK_STATE_MISSING", fmt.Sprintf("task state missing: %s", id))
		}
		for _, depID := range ts.Dependencies {
			if depID == "" {
				return nil, ErrState("INVALID_DEPENDENCY", fmt.Sprintf("task %s has empty dependency id", id))
			}
			if _, ok := state.Tasks[depID]; !ok {
				return nil, ErrState("MISSING_DEPENDENCY", fmt.Sprintf("task %s depends on missing task %s", id, depID))
			}
			if !closure[depID] {
				stack = append(stack, depID)
			}
		}
	}

	// Stable ordering for effective selection: follow TaskOrder when available.
	effective := make([]TaskID, 0, len(closure))
	if len(state.TaskOrder) > 0 {
		for _, id := range state.TaskOrder {
			if closure[id] {
				effective = append(effective, id)
			}
		}
	} else {
		// Fallback (shouldn't happen in normal operation): iterate selected then remaining.
		effective = append(effective, selected...)
		for id := range closure {
			found := false
			for _, s := range effective {
				if s == id {
					found = true
					break
				}
			}
			if !found {
				effective = append(effective, id)
			}
		}
	}

	// Mark non-selected pending tasks as skipped.
	skipped := make([]TaskID, 0)
	applySkip := func(id TaskID, ts *TaskState) {
		if ts == nil {
			return
		}
		if ts.Status != TaskStatusPending {
			return
		}
		ts.Status = TaskStatusSkipped
		ts.Error = SkipReasonNotSelected
		completedAt := now
		ts.CompletedAt = &completedAt
		skipped = append(skipped, id)
	}

	if len(state.TaskOrder) > 0 {
		for _, id := range state.TaskOrder {
			if closure[id] {
				continue
			}
			applySkip(id, state.Tasks[id])
		}
	} else {
		for id, ts := range state.Tasks {
			if closure[id] {
				continue
			}
			applySkip(id, ts)
		}
	}

	return &TaskSelectionResult{
		SelectedTaskIDs:          selected,
		EffectiveSelectedTaskIDs: effective,
		SkippedTaskIDs:           skipped,
	}, nil
}
