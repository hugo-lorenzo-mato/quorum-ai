package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func TestNewRecoveryManager(t *testing.T) {
	t.Parallel()

	rm := NewRecoveryManager(nil, "/repo", logging.NewNop())
	if rm == nil {
		t.Fatal("NewRecoveryManager() returned nil")
	}
	if rm.repoPath != "/repo" {
		t.Errorf("repoPath = %q, want %q", rm.repoPath, "/repo")
	}
	if rm.staleThreshold != 5*time.Minute {
		t.Errorf("staleThreshold = %v, want %v", rm.staleThreshold, 5*time.Minute)
	}
}

func TestRecoveryManager_SetStaleThreshold(t *testing.T) {
	t.Parallel()

	rm := NewRecoveryManager(nil, "/repo", logging.NewNop())
	rm.SetStaleThreshold(10 * time.Minute)
	if rm.staleThreshold != 10*time.Minute {
		t.Errorf("staleThreshold = %v, want %v", rm.staleThreshold, 10*time.Minute)
	}
}

func TestResetRunningTasks(t *testing.T) {
	t.Parallel()

	rm := NewRecoveryManager(nil, "/repo", logging.NewNop())

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {
					ID:     "task-1",
					Status: core.TaskStatusRunning,
				},
				"task-2": {
					ID:     "task-2",
					Status: core.TaskStatusCompleted,
				},
				"task-3": {
					ID:      "task-3",
					Status:  core.TaskStatusRunning,
					Retries: 1,
				},
				"task-4": {
					ID:     "task-4",
					Status: core.TaskStatusPending,
				},
			},
		},
	}

	result := &RecoveryResult{}
	err := rm.resetRunningTasks(context.TODO(), state, result)
	if err != nil {
		t.Fatalf("resetRunningTasks() error = %v", err)
	}

	// Running tasks should be reset
	if state.Tasks["task-1"].Status != core.TaskStatusPending {
		t.Errorf("task-1 status = %v, want Pending", state.Tasks["task-1"].Status)
	}
	if state.Tasks["task-1"].Retries != 1 {
		t.Errorf("task-1 retries = %d, want 1", state.Tasks["task-1"].Retries)
	}
	if !state.Tasks["task-1"].Resumable {
		t.Error("task-1 should be resumable")
	}
	if state.Tasks["task-1"].Error == "" {
		t.Error("task-1 should have error message")
	}

	// task-3 was already running with 1 retry
	if state.Tasks["task-3"].Retries != 2 {
		t.Errorf("task-3 retries = %d, want 2", state.Tasks["task-3"].Retries)
	}

	// Completed task should be unchanged
	if state.Tasks["task-2"].Status != core.TaskStatusCompleted {
		t.Errorf("task-2 status = %v, want Completed", state.Tasks["task-2"].Status)
	}

	// Pending task should be unchanged
	if state.Tasks["task-4"].Status != core.TaskStatusPending {
		t.Errorf("task-4 status = %v, want Pending", state.Tasks["task-4"].Status)
	}

	// Check reset tasks in result
	if len(result.ResetTasks) != 2 {
		t.Errorf("len(ResetTasks) = %d, want 2", len(result.ResetTasks))
	}
}

func TestResetRunningTasks_NoRunningTasks(t *testing.T) {
	t.Parallel()

	rm := NewRecoveryManager(nil, "/repo", logging.NewNop())

	state := &core.WorkflowState{
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"task-1": {ID: "task-1", Status: core.TaskStatusCompleted},
				"task-2": {ID: "task-2", Status: core.TaskStatusPending},
			},
		},
	}

	result := &RecoveryResult{}
	err := rm.resetRunningTasks(context.TODO(), state, result)
	if err != nil {
		t.Fatalf("resetRunningTasks() error = %v", err)
	}
	if len(result.ResetTasks) != 0 {
		t.Errorf("len(ResetTasks) = %d, want 0", len(result.ResetTasks))
	}
}

func TestRecoveryResult_Fields(t *testing.T) {
	t.Parallel()

	result := &RecoveryResult{
		WorkflowID:       "wf-123",
		RecoveredChanges: true,
		RecoveryBranch:   "task-1-recovery-12345",
		AbortedMerge:     true,
		AbortedRebase:    false,
		ResetTasks:       []core.TaskID{"task-1", "task-2"},
	}

	if result.WorkflowID != "wf-123" {
		t.Errorf("WorkflowID = %v, want wf-123", result.WorkflowID)
	}
	if !result.RecoveredChanges {
		t.Error("expected RecoveredChanges = true")
	}
	if result.RecoveryBranch != "task-1-recovery-12345" {
		t.Errorf("RecoveryBranch = %q", result.RecoveryBranch)
	}
	if !result.AbortedMerge {
		t.Error("expected AbortedMerge = true")
	}
	if result.AbortedRebase {
		t.Error("expected AbortedRebase = false")
	}
	if len(result.ResetTasks) != 2 {
		t.Errorf("len(ResetTasks) = %d, want 2", len(result.ResetTasks))
	}
}
