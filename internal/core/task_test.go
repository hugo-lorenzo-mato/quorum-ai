package core

import "testing"

func TestTask_StateTransitions(t *testing.T) {
	task := NewTask("t1", "task", PhaseAnalyze)

	if err := task.MarkCompleted(nil); err == nil {
		t.Fatalf("expected error completing from pending")
	}

	if err := task.MarkRunning(); err != nil {
		t.Fatalf("unexpected error starting task: %v", err)
	}
	if task.Status != TaskStatusRunning {
		t.Fatalf("expected status running, got %s", task.Status)
	}
	if task.StartedAt == nil {
		t.Fatalf("expected StartedAt to be set")
	}

	if err := task.MarkRunning(); err == nil {
		t.Fatalf("expected error starting from running")
	}

	if err := task.MarkCompleted(nil); err != nil {
		t.Fatalf("unexpected error completing task: %v", err)
	}
	if task.Status != TaskStatusCompleted {
		t.Fatalf("expected status completed, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}
}

func TestTask_IsReady(t *testing.T) {
	task := NewTask("t1", "task", PhaseAnalyze).
		WithDependencies("t0", "t2")

	completed := map[TaskID]bool{"t0": true}
	if task.IsReady(completed) {
		t.Fatalf("expected task not ready with missing dependency")
	}

	completed["t2"] = true
	if !task.IsReady(completed) {
		t.Fatalf("expected task ready when all dependencies are complete")
	}

	task.Status = TaskStatusRunning
	if task.IsReady(completed) {
		t.Fatalf("expected task not ready when not pending")
	}
}

func TestTask_Retry(t *testing.T) {
	task := NewTask("t1", "task", PhaseAnalyze)
	if err := task.MarkRunning(); err != nil {
		t.Fatalf("unexpected error starting task: %v", err)
	}
	if err := task.MarkFailed(errTest("boom")); err != nil {
		t.Fatalf("unexpected error failing task: %v", err)
	}

	if !task.CanRetry() {
		t.Fatalf("expected task to be retryable")
	}

	if err := task.Reset(); err != nil {
		t.Fatalf("unexpected error resetting task: %v", err)
	}
	if task.Retries != 1 {
		t.Fatalf("expected retries to increment, got %d", task.Retries)
	}
	if task.Status != TaskStatusPending {
		t.Fatalf("expected status pending after reset, got %s", task.Status)
	}

	task.Status = TaskStatusFailed
	task.Retries = task.MaxRetries
	if task.CanRetry() {
		t.Fatalf("expected task not retryable at max retries")
	}
	if err := task.Reset(); err == nil {
		t.Fatalf("expected error when resetting beyond max retries")
	}
}

func TestTask_Validate(t *testing.T) {
	valid := NewTask("t1", "task", PhaseAnalyze)
	if err := valid.Validate(); err != nil {
		t.Fatalf("unexpected error validating task: %v", err)
	}

	missingID := NewTask("", "task", PhaseAnalyze)
	if err := missingID.Validate(); err == nil {
		t.Fatalf("expected error for missing ID")
	}

	missingName := NewTask("t1", "", PhaseAnalyze)
	if err := missingName.Validate(); err == nil {
		t.Fatalf("expected error for missing name")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }
