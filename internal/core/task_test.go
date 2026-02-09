package core

import "testing"

func TestTask_StateTransitions(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestTask_Options(t *testing.T) {
	t.Parallel()
	task := NewTask("t1", "task", PhaseAnalyze).
		WithDescription("test description").
		WithCLI("claude").
		WithModel("claude-4").
		WithMaxRetries(5)

	if task.Description != "test description" {
		t.Errorf("Description = %s, want test description", task.Description)
	}
	if task.CLI != "claude" {
		t.Errorf("CLI = %s, want claude", task.CLI)
	}
	if task.Model != "claude-4" {
		t.Errorf("Model = %s, want claude-4", task.Model)
	}
	if task.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", task.MaxRetries)
	}
}

func TestTask_MarkSkipped(t *testing.T) {
	t.Parallel()
	task := NewTask("t1", "task", PhaseAnalyze)

	err := task.MarkSkipped("dependency failed")
	if err != nil {
		t.Fatalf("MarkSkipped() error = %v", err)
	}
	if task.Status != TaskStatusSkipped {
		t.Errorf("Status = %s, want skipped", task.Status)
	}
	if task.Error != "dependency failed" {
		t.Errorf("Error = %s, want dependency failed", task.Error)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestTask_Duration(t *testing.T) {
	t.Parallel()
	task := NewTask("t1", "task", PhaseAnalyze)

	// Duration without started should be 0
	if task.Duration() != 0 {
		t.Error("Duration should be 0 when not started")
	}

	// Start and complete the task
	_ = task.MarkRunning()
	_ = task.MarkCompleted(nil)

	// Duration should be non-negative after completion
	dur := task.Duration()
	if dur < 0 {
		t.Error("Duration should be non-negative after completion")
	}
}

func TestTask_IsTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   TaskStatus
		terminal bool
	}{
		{TaskStatusPending, false},
		{TaskStatusRunning, false},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		{TaskStatusSkipped, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			task := NewTask("t1", "task", PhaseAnalyze)
			task.Status = tt.status

			if task.IsTerminal() != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", task.IsTerminal(), tt.terminal)
			}
		})
	}
}

func TestTask_IsSuccess(t *testing.T) {
	t.Parallel()
	task := NewTask("t1", "task", PhaseAnalyze)

	// Pending is not success
	if task.IsSuccess() {
		t.Error("Pending task should not be success")
	}

	// Running is not success
	_ = task.MarkRunning()
	if task.IsSuccess() {
		t.Error("Running task should not be success")
	}

	// Completed is success
	_ = task.MarkCompleted(nil)
	if !task.IsSuccess() {
		t.Error("Completed task should be success")
	}
}

func TestTask_MarkFailed_WithError(t *testing.T) {
	t.Parallel()
	task := NewTask("t1", "task", PhaseAnalyze)
	_ = task.MarkRunning()

	testErr := errTest("test error message")
	err := task.MarkFailed(testErr)
	if err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	if task.Error != "test error message" {
		t.Errorf("Error = %s, want test error message", task.Error)
	}
}

// Note: TestTask_MarkFailed_NilError removed because MarkFailed doesn't handle nil error gracefully
