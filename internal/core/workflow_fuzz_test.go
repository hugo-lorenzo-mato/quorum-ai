//go:build go1.18

package core

import (
	"errors"
	"testing"
)

// FuzzWorkflowStateTransitions tests that the workflow state machine
// maintains valid invariants under arbitrary transition sequences.
func FuzzWorkflowStateTransitions(f *testing.F) {
	// Seed with common transition sequences
	// 0=Start, 1=Pause, 2=Resume, 3=Complete, 4=Fail, 5=Abort
	f.Add([]byte{0})          // Just start
	f.Add([]byte{0, 1})       // Start then pause
	f.Add([]byte{0, 1, 2})    // Start, pause, resume
	f.Add([]byte{0, 3})       // Start then complete
	f.Add([]byte{0, 4})       // Start then fail
	f.Add([]byte{0, 5})       // Start then abort
	f.Add([]byte{0, 1, 2, 3}) // Full lifecycle
	f.Add([]byte{1, 0, 1, 2}) // Invalid start, then valid
	f.Add([]byte{3, 0, 3})    // Complete without starting
	f.Add([]byte{0, 0, 0})    // Multiple starts
	f.Add([]byte{0, 1, 1, 2}) // Multiple pauses

	f.Fuzz(func(t *testing.T, sequence []byte) {
		wf := NewWorkflow("test", "test prompt", nil)

		// Initial state invariants
		if wf.Status != WorkflowStatusPending {
			t.Fatalf("new workflow should be pending, got %s", wf.Status)
		}
		if wf.StartedAt != nil {
			t.Fatal("new workflow should not have StartedAt")
		}
		if wf.CompletedAt != nil {
			t.Fatal("new workflow should not have CompletedAt")
		}

		// Track that we entered a terminal state
		var enteredTerminal bool

		for _, op := range sequence {
			switch op % 6 {
			case 0:
				_ = wf.Start()
			case 1:
				_ = wf.Pause()
			case 2:
				_ = wf.Resume()
			case 3:
				_ = wf.Complete()
			case 4:
				_ = wf.Fail(errors.New("test error"))
			case 5:
				_ = wf.Abort("user abort")
			}

			// Check invariants after each transition
			assertWorkflowInvariants(t, wf)

			if isTerminalState(wf.Status) {
				enteredTerminal = true
			}
		}

		// If we entered a terminal state, verify it's sticky
		if enteredTerminal {
			assertTerminalStateSticky(t, wf)
		}
	})
}

// FuzzWorkflowTaskOperations tests workflow task operations under fuzz.
func FuzzWorkflowTaskOperations(f *testing.F) {
	f.Add("task1", "Task title", uint8(0))
	f.Add("", "Empty ID", uint8(1))
	f.Add("task-with-long-id-that-might-cause-issues", "Long task", uint8(2))
	f.Add("task\nwith\nnewlines", "Newline task", uint8(0))
	f.Add("task with spaces", "Spaced task", uint8(1))

	f.Fuzz(func(t *testing.T, taskID string, title string, phase uint8) {
		wf := NewWorkflow("wf", "prompt", nil)

		// Convert phase to valid Phase
		phases := []Phase{PhaseAnalyze, PhasePlan, PhaseExecute}
		selectedPhase := phases[int(phase)%len(phases)]

		// Skip empty task IDs as they're expected to be invalid
		if taskID == "" {
			return
		}

		task := NewTask(TaskID(taskID), title, selectedPhase)

		// Should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic adding task %q: %v", taskID, r)
			}
		}()

		err := wf.AddTask(task)
		if err != nil {
			return // Some inputs may be invalid
		}

		// Verify task was added
		retrievedTask, ok := wf.GetTask(TaskID(taskID))
		if !ok {
			t.Errorf("task %q not found after adding", taskID)
		}
		if retrievedTask.Name != title {
			t.Errorf("task title mismatch: got %q, want %q", retrievedTask.Name, title)
		}

		// Adding same task again should fail
		err = wf.AddTask(task)
		if err == nil {
			t.Error("expected error when adding duplicate task")
		}
	})
}

// FuzzWorkflowConfig tests that workflow blueprint values are handled safely.
func FuzzWorkflowConfig(f *testing.F) {
	f.Add(0.0, 0, int64(0), true, true)
	f.Add(0.5, 3, int64(3600), false, false)
	f.Add(1.0, 10, int64(7200), true, false)
	f.Add(-0.5, -1, int64(-1000), false, true)
	f.Add(2.0, 100, int64(86400), true, true)

	f.Fuzz(func(t *testing.T, threshold float64, retries int, timeoutSec int64, dryRun bool, sandbox bool) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic creating workflow with config: %v", r)
			}
		}()

		bp := &Blueprint{
			Consensus:  BlueprintConsensus{Threshold: threshold},
			MaxRetries: retries,
			DryRun:     dryRun,
			Sandbox:    sandbox,
		}

		wf := NewWorkflow("test", "prompt", bp)

		// Workflow should be created regardless of blueprint values
		if wf == nil {
			t.Error("workflow should not be nil")
			return
		}

		// Blueprint should be set
		if wf.Blueprint == nil {
			t.Error("workflow blueprint should not be nil")
			return
		}

		// Values should be preserved (even if invalid - validation is separate)
		if wf.Blueprint.Consensus.Threshold != threshold {
			t.Errorf("threshold not preserved: got %f, want %f", wf.Blueprint.Consensus.Threshold, threshold)
		}
		if wf.Blueprint.MaxRetries != retries {
			t.Errorf("retries not preserved: got %d, want %d", wf.Blueprint.MaxRetries, retries)
		}
	})
}

// assertWorkflowInvariants checks that workflow state invariants hold.
func assertWorkflowInvariants(t *testing.T, wf *Workflow) {
	t.Helper()

	// Status must be valid
	validStatuses := map[WorkflowStatus]bool{
		WorkflowStatusPending:   true,
		WorkflowStatusRunning:   true,
		WorkflowStatusPaused:    true,
		WorkflowStatusCompleted: true,
		WorkflowStatusFailed:    true,
		WorkflowStatusAborted:   true,
	}
	if !validStatuses[wf.Status] {
		t.Fatalf("invalid status: %s", wf.Status)
	}

	// StartedAt should be set if running or paused (states that require starting)
	// Note: Abort and Fail can be called from any state, so they don't require StartedAt
	if (wf.Status == WorkflowStatusRunning || wf.Status == WorkflowStatusPaused) && wf.StartedAt == nil {
		t.Fatalf("StartedAt should be set when status is %s", wf.Status)
	}

	// CompletedAt should be set only for terminal states
	if isTerminalState(wf.Status) && wf.CompletedAt == nil {
		t.Fatalf("CompletedAt should be set when status is %s", wf.Status)
	}

	// Error should be set for failed and aborted statuses
	if (wf.Status == WorkflowStatusFailed || wf.Status == WorkflowStatusAborted) && wf.Error == "" {
		t.Fatalf("Error should be set when status is %s", wf.Status)
	}
}

// isTerminalState returns true if the status is a terminal state.
func isTerminalState(status WorkflowStatus) bool {
	return status == WorkflowStatusCompleted ||
		status == WorkflowStatusFailed ||
		status == WorkflowStatusAborted
}

// assertTerminalStateSticky verifies that terminal states can't be changed.
func assertTerminalStateSticky(t *testing.T, wf *Workflow) {
	t.Helper()

	if !isTerminalState(wf.Status) {
		return
	}

	originalStatus := wf.Status

	// Try all transitions - all should fail or be no-ops
	_ = wf.Start()
	_ = wf.Pause()
	_ = wf.Resume()

	if wf.Status != originalStatus {
		t.Fatalf("terminal state %s changed to %s", originalStatus, wf.Status)
	}
}
