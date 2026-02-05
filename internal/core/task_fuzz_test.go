//go:build go1.18

package core

import (
	"errors"
	"testing"
)

// FuzzTaskStateTransitions tests task state machine invariants.
func FuzzTaskStateTransitions(f *testing.F) {
	// Seed with common transition sequences
	// 0=MarkRunning, 1=MarkCompleted, 2=MarkFailed, 3=MarkSkipped
	f.Add([]byte{0})       // Just start
	f.Add([]byte{0, 1})    // Start then complete
	f.Add([]byte{0, 2})    // Start then fail
	f.Add([]byte{3})       // Skip without starting
	f.Add([]byte{0, 0})    // Double start
	f.Add([]byte{1, 0, 1}) // Complete without starting
	f.Add([]byte{0, 1, 2}) // Complete then fail (should be no-op)

	f.Fuzz(func(t *testing.T, sequence []byte) {
		task := NewTask("test", "test task", PhaseExecute)

		// Initial state invariants
		if task.Status != TaskStatusPending {
			t.Fatalf("new task should be pending, got %s", task.Status)
		}
		if task.StartedAt != nil {
			t.Fatal("new task should not have StartedAt")
		}
		if task.CompletedAt != nil {
			t.Fatal("new task should not have CompletedAt")
		}

		for _, op := range sequence {
			previousStatus := task.Status

			switch op % 4 {
			case 0:
				_ = task.MarkRunning()
			case 1:
				_ = task.MarkCompleted(nil)
			case 2:
				_ = task.MarkFailed(errors.New("test error"))
			case 3:
				_ = task.MarkSkipped("reason")
			}

			// Check invariants after each transition
			assertTaskInvariants(t, task, previousStatus)
		}
	})
}

// FuzzTaskWithDependencies tests task dependency operations.
func FuzzTaskWithDependencies(f *testing.F) {
	f.Add("dep1", "dep2", "dep3")
	f.Add("", "", "")
	f.Add("same", "same", "same")
	f.Add("a", "b", "c")

	f.Fuzz(func(t *testing.T, dep1, dep2, dep3 string) {
		task := NewTask("test", "test task", PhaseExecute)

		// Collect non-empty deps
		var deps []TaskID
		for _, dep := range []string{dep1, dep2, dep3} {
			if dep != "" {
				deps = append(deps, TaskID(dep))
			}
		}

		task.WithDependencies(deps...)

		// Dependencies should be stored
		if len(task.Dependencies) != len(deps) {
			t.Errorf("dependency count mismatch: got %d, want %d", len(task.Dependencies), len(deps))
		}
	})
}

// FuzzTaskRetryLogic tests task retry count logic.
func FuzzTaskRetryLogic(f *testing.F) {
	f.Add(0, 3)
	f.Add(1, 3)
	f.Add(3, 3)
	f.Add(10, 3)
	f.Add(0, 0)
	f.Add(0, 10)

	f.Fuzz(func(t *testing.T, retries int, maxRetries int) {
		task := NewTask("test", "test task", PhaseExecute)

		// Set retries and max
		if retries >= 0 {
			task.Retries = retries
		}
		if maxRetries >= 0 {
			task.MaxRetries = maxRetries
		}

		// Need to be in failed state to retry
		_ = task.MarkRunning()
		_ = task.MarkFailed(errors.New("test"))

		// CanRetry should be deterministic
		canRetry1 := task.CanRetry()
		canRetry2 := task.CanRetry()

		if canRetry1 != canRetry2 {
			t.Error("CanRetry should be deterministic")
		}

		// If retries >= maxRetries, should not be able to retry
		if task.Retries >= task.MaxRetries && task.CanRetry() {
			t.Errorf("should not be able to retry when retries (%d) >= maxRetries (%d)",
				task.Retries, task.MaxRetries)
		}
	})
}

// FuzzTaskReset tests task reset for retry.
func FuzzTaskReset(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(2)
	f.Add(3)
	f.Add(5)

	f.Fuzz(func(t *testing.T, maxRetries int) {
		if maxRetries < 0 {
			return
		}

		task := NewTask("test", "test task", PhaseExecute)
		task.MaxRetries = maxRetries

		// Run through multiple retries
		for i := 0; i <= maxRetries; i++ {
			_ = task.MarkRunning()
			_ = task.MarkFailed(errors.New("test error"))

			if i < maxRetries {
				// Should be able to retry
				if !task.CanRetry() {
					t.Errorf("should be able to retry at attempt %d (max=%d)", i, maxRetries)
				}
				if err := task.Reset(); err != nil {
					t.Errorf("reset failed at attempt %d: %v", i, err)
				}
				// After reset, status should be pending
				if task.Status != TaskStatusPending {
					t.Errorf("status should be pending after reset, got %s", task.Status)
				}
			} else {
				// Should not be able to retry
				if task.CanRetry() {
					t.Errorf("should not be able to retry at attempt %d (max=%d)", i, maxRetries)
				}
			}
		}
	})
}

// FuzzTaskValidation tests task validation logic.
func FuzzTaskValidation(f *testing.F) {
	f.Add("task1", "Task Name")
	f.Add("", "Task Name")
	f.Add("task1", "")
	f.Add("", "")
	f.Add("task-with-special-chars-!@#$%", "Special Task")

	f.Fuzz(func(t *testing.T, id string, name string) {
		task := &Task{
			ID:     TaskID(id),
			Name:   name,
			Status: TaskStatusPending,
		}

		err := task.Validate()

		// Empty ID should fail
		if id == "" && err == nil {
			t.Error("expected error for empty task ID")
		}

		// Empty name should fail
		if id != "" && name == "" && err == nil {
			t.Error("expected error for empty task name")
		}

		// Valid task should pass
		if id != "" && name != "" && err != nil {
			t.Errorf("unexpected error for valid task: %v", err)
		}
	})
}

// assertTaskInvariants checks that task state invariants hold.
func assertTaskInvariants(t *testing.T, task *Task, previousStatus TaskStatus) {
	t.Helper()

	// Status must be valid
	validStatuses := map[TaskStatus]bool{
		TaskStatusPending:   true,
		TaskStatusRunning:   true,
		TaskStatusCompleted: true,
		TaskStatusFailed:    true,
		TaskStatusSkipped:   true,
	}
	if !validStatuses[task.Status] {
		t.Fatalf("invalid status: %s", task.Status)
	}

	// StartedAt should be set if running or terminal (except skipped)
	if task.Status == TaskStatusRunning && task.StartedAt == nil {
		t.Fatalf("StartedAt should be set when status is %s", task.Status)
	}

	// CompletedAt should be set only for terminal states
	if task.IsTerminal() && task.CompletedAt == nil {
		t.Fatalf("CompletedAt should be set when status is %s", task.Status)
	}

	// Terminal states should be sticky (except for reset which is tested separately)
	if isTaskTerminal(previousStatus) && task.Status != previousStatus {
		t.Fatalf("terminal status changed from %s to %s", previousStatus, task.Status)
	}
}

// isTaskTerminal returns true if the task status is terminal.
func isTaskTerminal(status TaskStatus) bool {
	return status == TaskStatusCompleted ||
		status == TaskStatusFailed ||
		status == TaskStatusSkipped
}
