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
