package core

import "testing"

func TestWorkflow_AddTask(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	if err := wf.AddTask(nil); err == nil {
		t.Fatalf("expected error adding nil task")
	}

	task := NewTask("t1", "task", PhaseAnalyze)
	if err := wf.AddTask(task); err != nil {
		t.Fatalf("unexpected error adding task: %v", err)
	}
	if err := wf.AddTask(task); err == nil {
		t.Fatalf("expected error adding duplicate task")
	}
}

func TestWorkflow_TasksByPhase(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	_ = wf.AddTask(NewTask("t1", "task", PhaseAnalyze))
	_ = wf.AddTask(NewTask("t2", "task", PhasePlan))
	_ = wf.AddTask(NewTask("t3", "task", PhaseAnalyze))

	analyzeTasks := wf.TasksByPhase(PhaseAnalyze)
	if len(analyzeTasks) != 2 {
		t.Fatalf("expected 2 analyze tasks, got %d", len(analyzeTasks))
	}
}

func TestWorkflow_UpdateMetricsAndProgress(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	t1 := NewTask("t1", "task", PhaseAnalyze)
	t2 := NewTask("t2", "task", PhaseAnalyze)
	t1.CostUSD = 1.25
	t2.CostUSD = 2.75
	t1.TokensIn = 10
	t1.TokensOut = 20
	t2.TokensIn = 30
	t2.TokensOut = 40
	_ = wf.AddTask(t1)
	_ = wf.AddTask(t2)

	wf.UpdateMetrics()
	if wf.TotalCostUSD != 4.0 {
		t.Fatalf("expected total cost 4.0, got %.2f", wf.TotalCostUSD)
	}
	if wf.TotalTokensIn != 40 || wf.TotalTokensOut != 60 {
		t.Fatalf("unexpected token totals: in=%d out=%d", wf.TotalTokensIn, wf.TotalTokensOut)
	}

	if wf.Progress() != 0 {
		t.Fatalf("expected 0 progress with no completed tasks")
	}
	t1.Status = TaskStatusCompleted
	t2.Status = TaskStatusSkipped
	if wf.Progress() != 100 {
		t.Fatalf("expected 100 progress with completed+skipped tasks")
	}
}

func TestWorkflow_StateTransitions(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)

	if err := wf.Pause(); err == nil {
		t.Fatalf("expected error pausing when pending")
	}

	if err := wf.Start(); err != nil {
		t.Fatalf("unexpected error starting workflow: %v", err)
	}
	if wf.Status != WorkflowStatusRunning {
		t.Fatalf("expected running status, got %s", wf.Status)
	}

	if err := wf.Pause(); err != nil {
		t.Fatalf("unexpected error pausing workflow: %v", err)
	}
	if wf.Status != WorkflowStatusPaused {
		t.Fatalf("expected paused status, got %s", wf.Status)
	}

	if err := wf.Resume(); err != nil {
		t.Fatalf("unexpected error resuming workflow: %v", err)
	}
	if wf.Status != WorkflowStatusRunning {
		t.Fatalf("expected running status after resume, got %s", wf.Status)
	}

	if err := wf.Complete(); err != nil {
		t.Fatalf("unexpected error completing workflow: %v", err)
	}
	if wf.Status != WorkflowStatusCompleted {
		t.Fatalf("expected completed status, got %s", wf.Status)
	}
}

func TestWorkflow_AdvancePhase(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	if wf.CurrentPhase != PhaseAnalyze {
		t.Fatalf("expected initial phase analyze, got %s", wf.CurrentPhase)
	}
	if err := wf.AdvancePhase(); err != nil {
		t.Fatalf("unexpected error advancing phase: %v", err)
	}
	if wf.CurrentPhase != PhasePlan {
		t.Fatalf("expected phase plan, got %s", wf.CurrentPhase)
	}
	if err := wf.AdvancePhase(); err != nil {
		t.Fatalf("unexpected error advancing phase: %v", err)
	}
	if wf.CurrentPhase != PhaseExecute {
		t.Fatalf("expected phase execute, got %s", wf.CurrentPhase)
	}
	if err := wf.AdvancePhase(); err == nil {
		t.Fatalf("expected error advancing past final phase")
	}
}

func TestWorkflow_Validate(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	if err := wf.Validate(); err != nil {
		t.Fatalf("unexpected error validating workflow: %v", err)
	}

	missingID := NewWorkflow("", "prompt", nil)
	if err := missingID.Validate(); err == nil {
		t.Fatalf("expected error for missing workflow ID")
	}

	missingPrompt := NewWorkflow("w1", "", nil)
	if err := missingPrompt.Validate(); err == nil {
		t.Fatalf("expected error for missing workflow prompt")
	}
}

func TestWorkflow_GetTask(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	task := NewTask("t1", "test task", PhaseAnalyze)
	_ = wf.AddTask(task)

	// Get existing task
	got, ok := wf.GetTask("t1")
	if !ok {
		t.Fatal("expected to find task t1")
	}
	if got.Name != "test task" {
		t.Fatalf("expected task name 'test task', got %s", got.Name)
	}

	// Get non-existent task
	_, ok = wf.GetTask("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent task")
	}
}

func TestWorkflow_CompletedTasks(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	t1 := NewTask("t1", "task1", PhaseAnalyze)
	t2 := NewTask("t2", "task2", PhaseAnalyze)
	t3 := NewTask("t3", "task3", PhaseAnalyze)

	t1.Status = TaskStatusCompleted
	t2.Status = TaskStatusRunning
	t3.Status = TaskStatusCompleted

	_ = wf.AddTask(t1)
	_ = wf.AddTask(t2)
	_ = wf.AddTask(t3)

	completed := wf.CompletedTasks()
	if len(completed) != 2 {
		t.Fatalf("expected 2 completed tasks, got %d", len(completed))
	}
	if !completed["t1"] || !completed["t3"] {
		t.Fatal("expected t1 and t3 to be completed")
	}
}

func TestWorkflow_ReadyTasks(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	t1 := NewTask("t1", "task1", PhaseAnalyze)
	t2 := NewTask("t2", "task2", PhaseAnalyze)
	t2.Dependencies = []TaskID{"t1"}

	_ = wf.AddTask(t1)
	_ = wf.AddTask(t2)

	// Initially only t1 is ready (no dependencies)
	ready := wf.ReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task, got %d", len(ready))
	}
	if ready[0].ID != "t1" {
		t.Fatal("expected t1 to be ready")
	}

	// After t1 completes, t2 should be ready
	t1.Status = TaskStatusCompleted
	ready = wf.ReadyTasks()
	if len(ready) != 1 {
		t.Fatalf("expected 1 ready task after completion, got %d", len(ready))
	}
	if ready[0].ID != "t2" {
		t.Fatal("expected t2 to be ready after t1 completes")
	}
}

func TestWorkflow_FailAndAbort(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	_ = wf.Start()

	// Test Fail
	err := wf.Fail(ErrExecution("TEST_ERROR", "test error"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Status != WorkflowStatusFailed {
		t.Fatalf("expected failed status, got %s", wf.Status)
	}
	if wf.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}

	// Test Abort
	wf2 := NewWorkflow("w2", "prompt", nil)
	_ = wf2.Start()
	_ = wf2.Abort("user cancelled")
	if wf2.Status != WorkflowStatusAborted {
		t.Fatalf("expected aborted status, got %s", wf2.Status)
	}
	if wf2.Error != "user cancelled" {
		t.Fatalf("expected error 'user cancelled', got %s", wf2.Error)
	}
}

func TestWorkflow_Duration(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)

	// Duration before start should be 0
	if wf.Duration() != 0 {
		t.Fatal("expected 0 duration before start")
	}

	// Duration after start should be non-negative
	// (may be 0 on some platforms with low time resolution)
	_ = wf.Start()
	if wf.Duration() < 0 {
		t.Fatal("expected non-negative duration after start")
	}
}

func TestWorkflow_IsTerminal(t *testing.T) {
	tests := []struct {
		status   WorkflowStatus
		terminal bool
	}{
		{WorkflowStatusPending, false},
		{WorkflowStatusRunning, false},
		{WorkflowStatusPaused, false},
		{WorkflowStatusCompleted, true},
		{WorkflowStatusFailed, true},
		{WorkflowStatusAborted, true},
	}

	for _, tt := range tests {
		wf := &Workflow{Status: tt.status}
		if wf.IsTerminal() != tt.terminal {
			t.Errorf("IsTerminal() for %s: got %v, want %v", tt.status, wf.IsTerminal(), tt.terminal)
		}
	}
}

func TestWorkflow_StartFromPaused(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)
	_ = wf.Start()
	_ = wf.Pause()

	// Start from paused state should work
	if err := wf.Start(); err != nil {
		t.Fatalf("unexpected error starting from paused: %v", err)
	}
	if wf.Status != WorkflowStatusRunning {
		t.Fatalf("expected running status, got %s", wf.Status)
	}
}

func TestWorkflow_ResumeErrors(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)

	// Resume from pending should fail
	if err := wf.Resume(); err == nil {
		t.Fatal("expected error resuming from pending")
	}

	_ = wf.Start()
	// Resume from running should fail
	if err := wf.Resume(); err == nil {
		t.Fatal("expected error resuming from running")
	}
}

func TestWorkflow_CompleteErrors(t *testing.T) {
	wf := NewWorkflow("w1", "prompt", nil)

	// Complete from pending should fail
	if err := wf.Complete(); err == nil {
		t.Fatal("expected error completing from pending")
	}
}
