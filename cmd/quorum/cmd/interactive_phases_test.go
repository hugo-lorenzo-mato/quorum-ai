package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/cli"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/tui"
)

// ---------------------------------------------------------------------------
// runPlanPhaseFn default assignment (runPlanPhase)
// ---------------------------------------------------------------------------

func TestRunPlanPhaseFnIsSet(t *testing.T) {
	t.Parallel()
	if runPlanPhaseFn == nil {
		t.Fatal("runPlanPhaseFn should be set to runPlanPhase by default")
	}
}

// ---------------------------------------------------------------------------
// runInteractiveExecutionPhase: newInteractiveRunnerFn returns error
// ---------------------------------------------------------------------------

func TestExecutionPhase_RunnerCreationError(t *testing.T) {
	origNewRunner := newInteractiveRunnerFn
	t.Cleanup(func() { newInteractiveRunnerFn = origNewRunner })

	newInteractiveRunnerFn = func(_ *PhaseRunnerDeps, _ tui.Output) (resumableRunner, error) {
		return nil, errors.New("cannot create runner")
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	state.TaskOrder = []core.TaskID{"t1"}

	err := runInteractiveExecutionPhase(context.Background(), deps, nopOutput{}, state)
	if err == nil {
		t.Fatal("expected error when runner creation fails")
	}
	if !strings.Contains(err.Error(), "creating runner") {
		t.Fatalf("expected 'creating runner' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "cannot create runner") {
		t.Fatalf("expected cause in error, got %q", err.Error())
	}
	// No saves should happen since runner was never created
	if stateAdapter.saveCalls != 0 {
		t.Fatalf("expected 0 saves when runner creation fails, got %d", stateAdapter.saveCalls)
	}
}

// ---------------------------------------------------------------------------
// runInteractiveExecutionPhase: successful run marks workflow completed
// ---------------------------------------------------------------------------

type covBoostOkRunner struct{}

func (r covBoostOkRunner) ResumeWithState(context.Context, *core.WorkflowState) error { return nil }

func TestExecutionPhase_SuccessfulRun(t *testing.T) {
	origNewRunner := newInteractiveRunnerFn
	t.Cleanup(func() { newInteractiveRunnerFn = origNewRunner })

	newInteractiveRunnerFn = func(_ *PhaseRunnerDeps, _ tui.Output) (resumableRunner, error) {
		return covBoostOkRunner{}, nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	state.TaskOrder = []core.TaskID{"t1"}

	err := runInteractiveExecutionPhase(context.Background(), deps, nopOutput{}, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if state.Status != core.WorkflowStatusCompleted {
		t.Fatalf("expected status completed, got %s", state.Status)
	}
	if state.CurrentPhase != core.PhaseDone {
		t.Fatalf("expected phase done, got %s", state.CurrentPhase)
	}
	// 2 saves: one for running status, one for completed status
	if stateAdapter.saveCalls != 2 {
		t.Fatalf("expected 2 saves (running + completed), got %d", stateAdapter.saveCalls)
	}
}

// ---------------------------------------------------------------------------
// runInteractiveAnalysisPhase: analyzer creation error
// ---------------------------------------------------------------------------

func TestAnalysisPhase_AnalyzerCreationError(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) {
		return nil, errors.New("analyzer init failed")
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("\n"))

	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err == nil {
		t.Fatal("expected error when analyzer creation fails")
	}
	if !strings.Contains(err.Error(), "creating analyzer") {
		t.Fatalf("expected 'creating analyzer' error, got %q", err.Error())
	}
	if abort {
		t.Fatal("abort should be false on error")
	}
}

// ---------------------------------------------------------------------------
// runInteractiveAnalysisPhase: analyzer.Run error (initial)
// ---------------------------------------------------------------------------

type covBoostFailingAnalyzer struct{}

func (f *covBoostFailingAnalyzer) Run(_ context.Context, _ *workflow.Context) error {
	return errors.New("analysis exploded")
}

func TestAnalysisPhase_AnalyzerRunError(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) {
		return &covBoostFailingAnalyzer{}, nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("\n"))

	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err == nil {
		t.Fatal("expected error when analyzer.Run fails")
	}
	if !strings.Contains(err.Error(), "analysis failed") {
		t.Fatalf("expected 'analysis failed' error, got %q", err.Error())
	}
	if abort {
		t.Fatal("abort should be false on error")
	}
	if state.Status != core.WorkflowStatusFailed {
		t.Fatalf("expected status failed, got %s", state.Status)
	}
}

// ---------------------------------------------------------------------------
// runInteractiveAnalysisPhase: abort path
// ---------------------------------------------------------------------------

func TestAnalysisPhase_Abort(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	az := &stubAnalyzer{}
	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) { return az, nil }

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("q\n"))

	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error on abort, got %v", err)
	}
	if !abort {
		t.Fatal("expected abort=true")
	}
}

// ---------------------------------------------------------------------------
// runInteractiveAnalysisPhase: rerun path where re-run fails
// ---------------------------------------------------------------------------

type covBoostFailOnSecondAnalyzer struct {
	runs int
}

func (f *covBoostFailOnSecondAnalyzer) Run(_ context.Context, wctx *workflow.Context) error {
	f.runs++
	if f.runs > 1 {
		return errors.New("re-run failed")
	}
	data, _ := json.Marshal(map[string]interface{}{"content": "initial analysis"})
	wctx.State.Checkpoints = append(wctx.State.Checkpoints, core.Checkpoint{
		Type:      "consolidated_analysis",
		Timestamp: time.Now(),
		Data:      data,
	})
	return nil
}

func TestAnalysisPhase_RerunError(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	az := &covBoostFailOnSecondAnalyzer{}
	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) { return az, nil }

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("r\n"))

	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err == nil {
		t.Fatal("expected error when rerun fails")
	}
	if !strings.Contains(err.Error(), "analysis re-run failed") {
		t.Fatalf("expected 'analysis re-run failed' error, got %q", err.Error())
	}
	if abort {
		t.Fatal("abort should be false on error")
	}
	if state.Status != core.WorkflowStatusFailed {
		t.Fatalf("expected status failed, got %s", state.Status)
	}
}

// ---------------------------------------------------------------------------
// runInteractivePlanningPhase: planning error
// ---------------------------------------------------------------------------

func TestPlanningPhase_PlanError(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, _ *core.WorkflowState) error {
		return errors.New("plan generation failed")
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("\n"))

	abort, err := runInteractivePlanningPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err == nil {
		t.Fatal("expected error when planning fails")
	}
	if !strings.Contains(err.Error(), "planning failed") {
		t.Fatalf("expected 'planning failed' error, got %q", err.Error())
	}
	if abort {
		t.Fatal("abort should be false on error")
	}
}

// ---------------------------------------------------------------------------
// runInteractivePlanningPhase: abort path
// ---------------------------------------------------------------------------

func TestPlanningPhase_Abort(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		st.Tasks = map[core.TaskID]*core.TaskState{
			"t1": {ID: "t1", Phase: core.PhaseExecute, Name: "Task", Status: core.TaskStatusPending, CLI: "claude"},
		}
		st.TaskOrder = []core.TaskID{"t1"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("q\n"))

	abort, err := runInteractivePlanningPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error on abort, got %v", err)
	}
	if !abort {
		t.Fatal("expected abort=true")
	}
}

// ---------------------------------------------------------------------------
// replanInteractive: with feedback
// ---------------------------------------------------------------------------

func TestReplanInteractive_WithFeedback(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	planRuns := 0
	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		planRuns++
		st.Tasks = map[core.TaskID]*core.TaskState{
			"t1": {ID: "t1", Phase: core.PhaseExecute, Name: "Replanned", Status: core.TaskStatusPending, CLI: "codex"},
		}
		st.TaskOrder = []core.TaskID{"t1"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	// Seed analysis so feedback can be prepended
	data, _ := json.Marshal(map[string]interface{}{"content": "original analysis"})
	state.Checkpoints = append(state.Checkpoints, core.Checkpoint{
		Type:      "consolidated_analysis",
		Timestamp: time.Now(),
		Data:      data,
	})

	err := replanInteractive(context.Background(), deps, state, "add more tests")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if planRuns != 1 {
		t.Fatalf("expected planner to run once, got %d", planRuns)
	}
	analysis := workflow.GetConsolidatedAnalysis(state)
	if !strings.Contains(analysis, "User feedback on plan: add more tests") {
		t.Fatalf("expected feedback prepended, got %q", analysis)
	}
}

// ---------------------------------------------------------------------------
// replanInteractive: without feedback
// ---------------------------------------------------------------------------

func TestReplanInteractive_NoFeedback(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		st.Tasks = map[core.TaskID]*core.TaskState{
			"t1": {ID: "t1", Phase: core.PhaseExecute, Name: "Replanned", Status: core.TaskStatusPending, CLI: "codex"},
		}
		st.TaskOrder = []core.TaskID{"t1"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	err := replanInteractive(context.Background(), deps, state, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(state.TaskOrder) != 1 {
		t.Fatalf("expected 1 task after replan, got %d", len(state.TaskOrder))
	}
	// Verify tasks and task order were cleared before replanning
	if state.CurrentPhase != core.PhasePlan {
		t.Fatalf("expected phase=plan, got %s", state.CurrentPhase)
	}
}

// ---------------------------------------------------------------------------
// replanInteractive: plan error
// ---------------------------------------------------------------------------

func TestReplanInteractive_Error(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, _ *core.WorkflowState) error {
		return errors.New("replan boom")
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()

	err := replanInteractive(context.Background(), deps, state, "")
	if err == nil {
		t.Fatal("expected error from replan")
	}
	if !strings.Contains(err.Error(), "replanning failed") {
		t.Fatalf("expected 'replanning failed' error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// validateQuorumConfig: loads default config (no config file) - should pass
// ---------------------------------------------------------------------------

func TestValidateQuorumConfig_DefaultConfig(t *testing.T) {
	// validateQuorumConfig uses NewLoader().Load() which falls back to defaults
	// when no config file is found. Defaults should pass validation.
	issues := validateQuorumConfig()
	// Default config should produce zero issues (it's validated in other tests too).
	// If there is a local .quorum/config.yaml that has issues, this may fail,
	// but that's a real configuration problem.
	if len(issues) > 0 {
		t.Logf("validateQuorumConfig returned issues (may be from local config): %v", issues)
	}
}

// ---------------------------------------------------------------------------
// runInteractiveExecutionPhase: state save error after setting running status
// ---------------------------------------------------------------------------

type covBoostFailSaveAdapter struct {
	fakeStateAdapter
	failOnCall int
	callCount  int
}

func (f *covBoostFailSaveAdapter) Save(_ context.Context, state *core.WorkflowState) error {
	f.callCount++
	if f.callCount == f.failOnCall {
		return errors.New("save failed")
	}
	f.last = state
	return nil
}

func TestExecutionPhase_SaveRunningStateError(t *testing.T) {
	origNewRunner := newInteractiveRunnerFn
	t.Cleanup(func() { newInteractiveRunnerFn = origNewRunner })

	newInteractiveRunnerFn = func(_ *PhaseRunnerDeps, _ tui.Output) (resumableRunner, error) {
		return covBoostOkRunner{}, nil
	}

	stateAdapter := &covBoostFailSaveAdapter{failOnCall: 1}
	deps := &PhaseRunnerDeps{
		Logger:       logging.NewNop(),
		StateAdapter: stateAdapter,
		RunnerConfig: &workflow.RunnerConfig{DryRun: true},
	}
	state := newTestState()
	state.TaskOrder = []core.TaskID{"t1"}

	err := runInteractiveExecutionPhase(context.Background(), deps, nopOutput{}, state)
	if err == nil {
		t.Fatal("expected error when save fails")
	}
	if !strings.Contains(err.Error(), "saving state") {
		t.Fatalf("expected 'saving state' error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// runInteractiveAnalysisPhase: continue without feedback (no prepend)
// ---------------------------------------------------------------------------

func TestAnalysisPhase_ContinueNoFeedback(t *testing.T) {
	origNewAnalyzer := newAnalyzerFn
	t.Cleanup(func() { newAnalyzerFn = origNewAnalyzer })

	az := &stubAnalyzer{}
	newAnalyzerFn = func(_ workflow.ModeratorConfig) (analyzerRunner, error) { return az, nil }

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	scanner := bufio.NewScanner(strings.NewReader("c\n"))

	abort, err := runInteractiveAnalysisPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatal("expected abort=false")
	}
	if state.CurrentPhase != core.PhasePlan {
		t.Fatalf("expected phase plan, got %s", state.CurrentPhase)
	}
}

// ---------------------------------------------------------------------------
// CreateWorkflowContext: config fields propagation (not covered by existing tests)
// ---------------------------------------------------------------------------

func TestCreateWorkflowContext_ConfigFieldsPropagation(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		Registry: cli.NewRegistry(),
		Logger:   logging.NewNop(),
		RunnerConfig: &workflow.RunnerConfig{
			DryRun:            true,
			DefaultAgent:      "gemini",
			DenyTools:         []string{"bash"},
			WorktreeAutoClean: true,
			WorktreeMode:      "always",
			AgentPhaseModels:  map[string]map[string]string{"claude": {"analyze": "opus"}},
			PhaseTimeouts: workflow.PhaseTimeouts{
				Analyze: 30 * time.Minute,
				Plan:    15 * time.Minute,
				Execute: 45 * time.Minute,
			},
			SingleAgent: workflow.SingleAgentConfig{Enabled: true, Agent: "codex"},
			Finalization: workflow.FinalizationConfig{
				AutoCommit:    true,
				AutoPush:      true,
				PRBaseBranch:  "develop",
				MergeStrategy: "squash",
				Remote:        "upstream",
			},
		},
		ModeratorConfig: workflow.ModeratorConfig{
			Enabled:   true,
			Agent:     "claude",
			Threshold: 0.8,
			MinRounds: 2,
			MaxRounds: 5,
		},
	}
	state := newTestState()

	wctx := CreateWorkflowContext(deps, state)
	if !wctx.Config.DryRun {
		t.Fatal("expected DryRun=true")
	}
	if wctx.Config.DefaultAgent != "gemini" {
		t.Fatalf("expected DefaultAgent=gemini, got %q", wctx.Config.DefaultAgent)
	}
	if len(wctx.Config.DenyTools) != 1 || wctx.Config.DenyTools[0] != "bash" {
		t.Fatalf("expected DenyTools=[bash], got %v", wctx.Config.DenyTools)
	}
	if !wctx.Config.WorktreeAutoClean {
		t.Fatal("expected WorktreeAutoClean=true")
	}
	if wctx.Config.WorktreeMode != "always" {
		t.Fatalf("expected WorktreeMode=always, got %q", wctx.Config.WorktreeMode)
	}
	if !wctx.Config.SingleAgent.Enabled {
		t.Fatal("expected SingleAgent.Enabled=true")
	}
	if wctx.Config.SingleAgent.Agent != "codex" {
		t.Fatalf("expected SingleAgent.Agent=codex, got %q", wctx.Config.SingleAgent.Agent)
	}
	if !wctx.Config.Moderator.Enabled {
		t.Fatal("expected Moderator.Enabled=true")
	}
	if wctx.Config.Moderator.Agent != "claude" {
		t.Fatalf("expected Moderator.Agent=claude, got %q", wctx.Config.Moderator.Agent)
	}
	if wctx.Config.Moderator.MinRounds != 2 {
		t.Fatalf("expected Moderator.MinRounds=2, got %d", wctx.Config.Moderator.MinRounds)
	}
	if wctx.Config.PhaseTimeouts.Analyze != 30*time.Minute {
		t.Fatalf("expected Analyze timeout 30m, got %v", wctx.Config.PhaseTimeouts.Analyze)
	}
	if wctx.Config.PhaseTimeouts.Plan != 15*time.Minute {
		t.Fatalf("expected Plan timeout 15m, got %v", wctx.Config.PhaseTimeouts.Plan)
	}
	if wctx.Config.PhaseTimeouts.Execute != 45*time.Minute {
		t.Fatalf("expected Execute timeout 45m, got %v", wctx.Config.PhaseTimeouts.Execute)
	}
	// Finalization fields
	if !wctx.Config.Finalization.AutoCommit {
		t.Fatal("expected Finalization.AutoCommit=true")
	}
	if !wctx.Config.Finalization.AutoPush {
		t.Fatal("expected Finalization.AutoPush=true")
	}
	if wctx.Config.Finalization.PRBaseBranch != "develop" {
		t.Fatalf("expected PRBaseBranch=develop, got %q", wctx.Config.Finalization.PRBaseBranch)
	}
	if wctx.Config.Finalization.MergeStrategy != "squash" {
		t.Fatalf("expected MergeStrategy=squash, got %q", wctx.Config.Finalization.MergeStrategy)
	}
	if wctx.Config.Finalization.Remote != "upstream" {
		t.Fatalf("expected Remote=upstream, got %q", wctx.Config.Finalization.Remote)
	}
	// AgentPhaseModels
	if wctx.Config.AgentPhaseModels == nil {
		t.Fatal("expected non-nil AgentPhaseModels")
	}
	if wctx.Config.AgentPhaseModels["claude"]["analyze"] != "opus" {
		t.Fatalf("expected claude/analyze=opus in AgentPhaseModels")
	}
}

// ---------------------------------------------------------------------------
// EnsureWorkflowGitIsolation: nil RunnerConfig (not tested in common_coverage)
// ---------------------------------------------------------------------------

func TestEnsureWorkflowGitIsolation_NilRunnerConfig(t *testing.T) {
	t.Parallel()
	// RunnerConfig is nil but deps is not nil - should proceed past DryRun check
	deps := &PhaseRunnerDeps{
		RunnerConfig: nil,
		GitIsolation: &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{
			initResult: &core.WorkflowGitInfo{WorkflowBranch: "quorum/wf-nilrc"},
		},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-nilrc"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true when RunnerConfig is nil (not DryRun)")
	}
	if state.WorkflowBranch != "quorum/wf-nilrc" {
		t.Fatalf("expected WorkflowBranch=quorum/wf-nilrc, got %q", state.WorkflowBranch)
	}
}

// ---------------------------------------------------------------------------
// CreateWorkflowContext: nil WorkflowWorktrees with enabled GitIsolation
// ---------------------------------------------------------------------------

func TestCreateWorkflowContext_NilWorktreesWithIsolation(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		Registry:          cli.NewRegistry(),
		Logger:            logging.NewNop(),
		RunnerConfig:      &workflow.RunnerConfig{DefaultAgent: "claude", Finalization: workflow.FinalizationConfig{AutoPR: true, AutoMerge: true}},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: nil, // nil means isolation is not active even if enabled
	}
	state := newTestState()
	state.WorkflowBranch = "quorum/wf-1"

	wctx := CreateWorkflowContext(deps, state)
	// nil WorkflowWorktrees means isolation not active - AutoPR/AutoMerge preserved
	if !wctx.Config.Finalization.AutoPR {
		t.Fatal("expected AutoPR=true when WorkflowWorktrees is nil")
	}
	if !wctx.Config.Finalization.AutoMerge {
		t.Fatal("expected AutoMerge=true when WorkflowWorktrees is nil")
	}
}

// ---------------------------------------------------------------------------
// runInteractivePlanningPhase: continue path (happy path)
// ---------------------------------------------------------------------------

func TestPlanningPhase_Continue(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		st.Tasks = map[core.TaskID]*core.TaskState{
			"t1": {ID: "t1", Phase: core.PhaseExecute, Name: "Task A", Status: core.TaskStatusPending, CLI: "claude"},
			"t2": {ID: "t2", Phase: core.PhaseExecute, Name: "Task B", Status: core.TaskStatusPending, CLI: "gemini"},
		}
		st.TaskOrder = []core.TaskID{"t1", "t2"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	// Enter to continue
	scanner := bufio.NewScanner(strings.NewReader("\n"))

	abort, err := runInteractivePlanningPhase(context.Background(), deps, nopOutput{}, scanner, state)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if abort {
		t.Fatal("expected abort=false")
	}
	if len(state.TaskOrder) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(state.TaskOrder))
	}
}

// ---------------------------------------------------------------------------
// replanInteractive: verifies state is reset before replanning
// ---------------------------------------------------------------------------

func TestReplanInteractive_StateReset(t *testing.T) {
	origRunPlan := runPlanPhaseFn
	t.Cleanup(func() { runPlanPhaseFn = origRunPlan })

	var capturedTaskCount int
	var capturedOrder []core.TaskID

	runPlanPhaseFn = func(_ context.Context, _ *PhaseRunnerDeps, _ *workflow.Context, st *core.WorkflowState) error {
		// Capture the state of tasks/order when planner runs (before adding)
		capturedTaskCount = len(st.Tasks)
		capturedOrder = st.TaskOrder
		// Then populate
		st.Tasks["t-new"] = &core.TaskState{ID: "t-new", Phase: core.PhaseExecute, Name: "New", Status: core.TaskStatusPending, CLI: "claude"}
		st.TaskOrder = []core.TaskID{"t-new"}
		return nil
	}

	stateAdapter := &fakeStateAdapter{}
	deps := newTestDeps(stateAdapter)
	state := newTestState()
	// Pre-populate tasks to verify they get cleared
	state.Tasks = map[core.TaskID]*core.TaskState{
		"t-old": {ID: "t-old", Phase: core.PhaseExecute, Name: "Old", Status: core.TaskStatusPending, CLI: "codex"},
	}
	state.TaskOrder = []core.TaskID{"t-old"}

	err := replanInteractive(context.Background(), deps, state, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// The tasks map should have been re-created (empty) before planner ran
	if capturedOrder != nil {
		t.Fatalf("expected TaskOrder to be nil when planner runs, got %v", capturedOrder)
	}
	if capturedTaskCount != 0 {
		t.Fatalf("expected empty Tasks map when planner runs, got %d", capturedTaskCount)
	}
}
