package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

type nopOutput struct{}

func (n nopOutput) WorkflowStarted(string)                         {}
func (n nopOutput) PhaseStarted(core.Phase)                        {}
func (n nopOutput) TaskStarted(*core.Task)                         {}
func (n nopOutput) TaskCompleted(*core.Task, time.Duration)        {}
func (n nopOutput) TaskFailed(*core.Task, error)                   {}
func (n nopOutput) TaskSkipped(*core.Task, string)                 {}
func (n nopOutput) WorkflowStateUpdated(*core.WorkflowState)       {}
func (n nopOutput) WorkflowCompleted(*core.WorkflowState)          {}
func (n nopOutput) WorkflowFailed(error)                           {}
func (n nopOutput) Log(string, string)                             {}
func (n nopOutput) Close() error                                   { return nil }
var _ tui.Output = (*nopOutput)(nil)

type fakeStateAdapter struct {
	saveCalls int
	last      *core.WorkflowState
}

func (f *fakeStateAdapter) Save(_ context.Context, state *core.WorkflowState) error {
	f.saveCalls++
	f.last = state
	return nil
}
func (f *fakeStateAdapter) Load(context.Context) (*core.WorkflowState, error) { return nil, errors.New("not implemented") }
func (f *fakeStateAdapter) LoadByID(context.Context, core.WorkflowID) (*core.WorkflowState, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeStateAdapter) AcquireLock(context.Context) error              { return nil }
func (f *fakeStateAdapter) ReleaseLock(context.Context) error              { return nil }
func (f *fakeStateAdapter) DeactivateWorkflow(context.Context) error       { return nil }
func (f *fakeStateAdapter) ArchiveWorkflows(context.Context) (int, error)  { return 0, nil }
func (f *fakeStateAdapter) PurgeAllWorkflows(context.Context) (int, error) { return 0, nil }
func (f *fakeStateAdapter) DeleteWorkflow(context.Context, core.WorkflowID) error {
	return nil
}

type stubAnalyzer struct {
	runs int
}

func (s *stubAnalyzer) Run(_ context.Context, wctx *workflow.Context) error {
	s.runs++
	content := "analysis run 1"
	if s.runs > 1 {
		content = "analysis run 2"
	}
	data, _ := json.Marshal(map[string]interface{}{"content": content})
	wctx.State.Checkpoints = append(wctx.State.Checkpoints, core.Checkpoint{
		Type:      "consolidated_analysis",
		Timestamp: time.Now(),
		Data:      data,
	})
	return nil
}

func newTestDeps(stateAdapter *fakeStateAdapter) *PhaseRunnerDeps {
	return &PhaseRunnerDeps{
		Logger:       logging.NewNop(),
		StateAdapter: stateAdapter,
		RunnerConfig: &workflow.RunnerConfig{DryRun: true},
	}
}

func newTestState() *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Prompt:     "p",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:      core.WorkflowStatusRunning,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:       make(map[core.TaskID]*core.TaskState),
			TaskOrder:   nil,
			Checkpoints: make([]core.Checkpoint, 0),
			UpdatedAt:   time.Now(),
		},
	}
}

type errRunner struct {
	err error
}

func (r errRunner) ResumeWithState(context.Context, *core.WorkflowState) error { return r.err }

func TestRunInteractiveAnalysisPhase_ContinueWithFeedbackPrependsAnalysis(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	az := &stubAnalyzer{}
	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) { return az, nil }

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	scanner := bufio.NewScanner(strings.NewReader("f\nmy feedback\n"))
	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatalf("expected abort=false")
	}
	if az.runs != 1 {
		t.Fatalf("expected analyzer to run once, got %d", az.runs)
	}

	got := workflow.GetConsolidatedAnalysis(state)
	if !strings.HasPrefix(got, "my feedback\n\n---\n\n") {
		t.Fatalf("expected feedback to be prepended to consolidated analysis, got %q", got)
	}
	if !strings.Contains(got, "analysis run 1") {
		t.Fatalf("expected stub analysis content in consolidated analysis, got %q", got)
	}
	if stateAdapter.saveCalls < 2 {
		t.Fatalf("expected at least 2 saves (post-analysis + post-feedback), got %d", stateAdapter.saveCalls)
	}
}

func TestRunInteractiveAnalysisPhase_RerunClearsCheckpointsAndReanalyzes(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	az := &stubAnalyzer{}
	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) { return az, nil }

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	scanner := bufio.NewScanner(strings.NewReader("r\n"))
	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatalf("expected abort=false")
	}
	if az.runs != 2 {
		t.Fatalf("expected analyzer to run twice (initial + rerun), got %d", az.runs)
	}
	if len(state.Checkpoints) != 1 {
		t.Fatalf("expected checkpoints to be cleared before rerun and replaced, got %d", len(state.Checkpoints))
	}
	if !strings.Contains(workflow.GetConsolidatedAnalysis(state), "analysis run 2") {
		t.Fatalf("expected updated analysis after rerun")
	}
}

func TestRunInteractivePlanningPhase_ReplanRerunsPlannerAndPrependsFeedback(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	planRuns := 0
	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		planRuns++
		if st.Tasks == nil {
			st.Tasks = make(map[core.TaskID]*core.TaskState)
		}
		st.Tasks = make(map[core.TaskID]*core.TaskState)
		st.TaskOrder = nil
		var id core.TaskID = "t1"
		var name = "First plan"
		if planRuns > 1 {
			id = "t2"
			name = "Second plan"
		}
		st.Tasks[id] = &core.TaskState{ID: id, Phase: core.PhaseExecute, Name: name, Status: core.TaskStatusPending, CLI: "codex"}
		st.TaskOrder = []core.TaskID{id}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	// Seed consolidated analysis so replanInteractive can prepend feedback.
	data, _ := json.Marshal(map[string]interface{}{"content": "base analysis"})
	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		Type:      "consolidated_analysis",
		Timestamp: time.Now(),
		Data:      data,
	})

	// 'r' => replan, then provide feedback, then Enter to continue.
	scanner := bufio.NewScanner(strings.NewReader("r\nplease change plan\n\n"))
	abort, err := runInteractivePlanningPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatalf("expected abort=false")
	}
	if planRuns != 2 {
		t.Fatalf("expected planner to run twice (initial + replan), got %d", planRuns)
	}
	if len(state.TaskOrder) != 1 || state.TaskOrder[0] != "t2" {
		t.Fatalf("expected final plan to be second run, got %v", state.TaskOrder)
	}

	analysis := workflow.GetConsolidatedAnalysis(state)
	if !strings.HasPrefix(analysis, "User feedback on plan: please change plan\n\n---\n\n") {
		t.Fatalf("expected replan feedback to be prepended, got %q", analysis)
	}
}

func TestRunInteractivePlanningPhase_EditTriggersSave(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	// Provide at least one task so the edit prompt exercises the non-empty path.
	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		if st.Tasks == nil {
			st.Tasks = make(map[core.TaskID]*core.TaskState)
		}
		st.Tasks["t1"] = &core.TaskState{ID: "t1", Phase: core.PhaseExecute, Name: "A", Status: core.TaskStatusPending, CLI: "codex"}
		st.TaskOrder = []core.TaskID{"t1"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	// 'e' => edit, then Enter to exit edit, then Enter to continue.
	scanner := bufio.NewScanner(strings.NewReader("e\n\n\n"))
	abort, err := runInteractivePlanningPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatalf("expected abort=false")
	}
	if stateAdapter.saveCalls != 1 {
		t.Fatalf("expected exactly 1 save for edit action, got %d", stateAdapter.saveCalls)
	}
}

func TestRunInteractiveExecutionPhase_FailedRunMarksWorkflowFailed(t *testing.T) {
	origNewRunner := newInteractiveRunnerFn
	t.Cleanup(func() { newInteractiveRunnerFn = origNewRunner })

	newInteractiveRunnerFn = func(_ *PhaseRunnerDeps, _ tui.Output) (resumableRunner, error) {
		return errRunner{err: errors.New("boom")}, nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	state.TaskOrder = []core.TaskID{"t1"}

	err := runInteractiveExecutionPhase(context.Background(), deps, nopOutput{}, state)
	if err == nil || !strings.Contains(err.Error(), "execution failed") {
		t.Fatalf("expected execution failed error, got %v", err)
	}
	if state.Status != core.WorkflowStatusFailed {
		t.Fatalf("expected status failed, got %s", state.Status)
	}
	if stateAdapter.saveCalls != 2 {
		t.Fatalf("expected 2 saves (running + failed), got %d", stateAdapter.saveCalls)
	}
}
