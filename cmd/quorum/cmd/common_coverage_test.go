package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// --- parseDurationDefault ---

func TestParseDurationDefault_EmptyValue(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}
}

func TestParseDurationDefault_WhitespaceOnly(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("   ", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m fallback, got %v", d)
	}
}

func TestParseDurationDefault_ValidDuration(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("30s", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 30*time.Second {
		t.Errorf("expected 30s, got %v", d)
	}
}

func TestParseDurationDefault_InvalidDuration(t *testing.T) {
	t.Parallel()
	_, err := parseDurationDefault("notaduration", 5*time.Minute)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseDurationDefault_LeadingTrailingWhitespace(t *testing.T) {
	t.Parallel()
	d, err := parseDurationDefault("  1h  ", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 1*time.Hour {
		t.Errorf("expected 1h, got %v", d)
	}
}

// --- phaseTimeoutValue ---

func TestPhaseTimeoutValue_Analyze(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{
		Analyze: config.AnalyzePhaseConfig{Timeout: "10m"},
		Plan:    config.PlanPhaseConfig{Timeout: "20m"},
		Execute: config.ExecutePhaseConfig{Timeout: "30m"},
	}
	if got := phaseTimeoutValue(cfg, core.PhaseAnalyze); got != "10m" {
		t.Errorf("expected 10m, got %s", got)
	}
}

func TestPhaseTimeoutValue_Plan(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{
		Plan: config.PlanPhaseConfig{Timeout: "20m"},
	}
	if got := phaseTimeoutValue(cfg, core.PhasePlan); got != "20m" {
		t.Errorf("expected 20m, got %s", got)
	}
}

func TestPhaseTimeoutValue_Execute(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{
		Execute: config.ExecutePhaseConfig{Timeout: "30m"},
	}
	if got := phaseTimeoutValue(cfg, core.PhaseExecute); got != "30m" {
		t.Errorf("expected 30m, got %s", got)
	}
}

func TestPhaseTimeoutValue_Unknown(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{}
	if got := phaseTimeoutValue(cfg, core.Phase("unknown")); got != "" {
		t.Errorf("expected empty string for unknown phase, got %s", got)
	}
}

func TestPhaseTimeoutValue_Refine(t *testing.T) {
	t.Parallel()
	cfg := &config.PhasesConfig{}
	if got := phaseTimeoutValue(cfg, core.PhaseRefine); got != "" {
		t.Errorf("expected empty string for refine phase, got %s", got)
	}
}

// --- buildBlueprint ---

func TestBuildBlueprint_MultiAgent(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{
		MaxRetries: 5,
		Timeout:    30 * time.Minute,
		DryRun:     true,
		SingleAgent: workflow.SingleAgentConfig{
			Enabled: false,
		},
		Moderator: workflow.ModeratorConfig{
			Enabled:   true,
			Agent:     "claude",
			Threshold: 0.8,
			MinRounds: 1,
			MaxRounds: 3,
		},
		Refiner: workflow.RefinerConfig{
			Enabled: true,
			Agent:   "gemini",
		},
		Synthesizer: workflow.SynthesizerConfig{
			Agent: "codex",
		},
		PhaseTimeouts: workflow.PhaseTimeouts{
			Analyze: 10 * time.Minute,
			Plan:    20 * time.Minute,
			Execute: 30 * time.Minute,
		},
	}

	bp := buildBlueprint(cfg)
	if bp.ExecutionMode != "multi_agent" {
		t.Errorf("expected multi_agent, got %s", bp.ExecutionMode)
	}
	if bp.MaxRetries != 5 {
		t.Errorf("expected max_retries=5, got %d", bp.MaxRetries)
	}
	if bp.DryRun != true {
		t.Error("expected dry_run=true")
	}
	if bp.Consensus.Enabled != true {
		t.Error("expected consensus.enabled=true")
	}
	if bp.Consensus.Agent != "claude" {
		t.Errorf("expected consensus.agent=claude, got %s", bp.Consensus.Agent)
	}
	if bp.Consensus.Threshold != 0.8 {
		t.Errorf("expected consensus.threshold=0.8, got %f", bp.Consensus.Threshold)
	}
	if bp.Refiner.Enabled != true {
		t.Error("expected refiner.enabled=true")
	}
	if bp.Refiner.Agent != "gemini" {
		t.Errorf("expected refiner.agent=gemini, got %s", bp.Refiner.Agent)
	}
	if bp.Synthesizer.Agent != "codex" {
		t.Errorf("expected synthesizer.agent=codex, got %s", bp.Synthesizer.Agent)
	}
	if bp.Phases.Analyze.Timeout != 10*time.Minute {
		t.Errorf("expected analyze timeout 10m, got %v", bp.Phases.Analyze.Timeout)
	}
	if bp.Phases.Plan.Timeout != 20*time.Minute {
		t.Errorf("expected plan timeout 20m, got %v", bp.Phases.Plan.Timeout)
	}
	if bp.Phases.Execute.Timeout != 30*time.Minute {
		t.Errorf("expected execute timeout 30m, got %v", bp.Phases.Execute.Timeout)
	}
}

func TestBuildBlueprint_SingleAgent(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{
		SingleAgent: workflow.SingleAgentConfig{
			Enabled: true,
			Agent:   "claude",
			Model:   "opus",
		},
	}

	bp := buildBlueprint(cfg)
	if bp.ExecutionMode != "single_agent" {
		t.Errorf("expected single_agent, got %s", bp.ExecutionMode)
	}
	if bp.SingleAgent.Agent != "claude" {
		t.Errorf("expected single_agent.agent=claude, got %s", bp.SingleAgent.Agent)
	}
	if bp.SingleAgent.Model != "opus" {
		t.Errorf("expected single_agent.model=opus, got %s", bp.SingleAgent.Model)
	}
}

func TestBuildBlueprint_PlanSynthesizer(t *testing.T) {
	t.Parallel()
	cfg := &workflow.RunnerConfig{
		PlanSynthesizer: workflow.PlanSynthesizerConfig{
			Enabled: true,
			Agent:   "claude",
		},
	}

	bp := buildBlueprint(cfg)
	if !bp.PlanSynthesizer.Enabled {
		t.Error("expected plan_synthesizer.enabled=true")
	}
	if bp.PlanSynthesizer.Agent != "claude" {
		t.Errorf("expected plan_synthesizer.agent=claude, got %s", bp.PlanSynthesizer.Agent)
	}
}

// --- CreateWorkflowContext ---

func TestCreateWorkflowContext_Basic(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		Logger: logging.NewNop(),
		RunnerConfig: &workflow.RunnerConfig{
			DryRun:       true,
			DenyTools:    []string{"rm"},
			DefaultAgent: "claude",
			PhaseTimeouts: workflow.PhaseTimeouts{
				Analyze: 10 * time.Minute,
			},
		},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-test",
		},
	}

	wctx := CreateWorkflowContext(deps, state)
	if wctx == nil {
		t.Fatal("expected non-nil workflow context")
	}
	if wctx.State != state {
		t.Error("expected state to be set")
	}
	if wctx.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if !wctx.Config.DryRun {
		t.Error("expected dry_run=true")
	}
	if wctx.Config.DefaultAgent != "claude" {
		t.Errorf("expected default_agent=claude, got %s", wctx.Config.DefaultAgent)
	}
}

func TestCreateWorkflowContext_WorkflowIsolation_DisablesPRAndMerge(t *testing.T) {
	t.Parallel()

	// When workflow isolation is active (GitIsolation enabled, WorkflowWorktrees set, and WorkflowBranch set),
	// finalization AutoPR and AutoMerge should be disabled.
	deps := &PhaseRunnerDeps{
		Logger:            logging.NewNop(),
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		RunnerConfig: &workflow.RunnerConfig{
			Finalization: workflow.FinalizationConfig{
				AutoPR:    true,
				AutoMerge: true,
				AutoPush:  true,
			},
		},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-iso",
		},
		WorkflowRun: core.WorkflowRun{
			WorkflowBranch: "wf-branch-123",
		},
	}

	wctx := CreateWorkflowContext(deps, state)
	if wctx.Config.Finalization.AutoPR {
		t.Error("expected finalization.auto_pr=false when workflow isolation is active")
	}
	if wctx.Config.Finalization.AutoMerge {
		t.Error("expected finalization.auto_merge=false when workflow isolation is active")
	}
	// AutoPush should remain unchanged
	if !wctx.Config.Finalization.AutoPush {
		t.Error("expected finalization.auto_push=true (unchanged)")
	}
}

func TestCreateWorkflowContext_NoIsolation_PreservesPRSettings(t *testing.T) {
	t.Parallel()

	deps := &PhaseRunnerDeps{
		Logger: logging.NewNop(),
		RunnerConfig: &workflow.RunnerConfig{
			Finalization: workflow.FinalizationConfig{
				AutoPR:    true,
				AutoMerge: true,
			},
		},
	}
	state := &core.WorkflowState{}

	wctx := CreateWorkflowContext(deps, state)
	if !wctx.Config.Finalization.AutoPR {
		t.Error("expected finalization.auto_pr=true when no isolation")
	}
	if !wctx.Config.Finalization.AutoMerge {
		t.Error("expected finalization.auto_merge=true when no isolation")
	}
}

// --- EnsureWorkflowGitIsolation ---

type fakeWorkflowWorktreeManager struct {
	initCalled bool
	initResult *core.WorkflowGitInfo
	initErr    error
}

func (f *fakeWorkflowWorktreeManager) InitializeWorkflow(_ context.Context, _ string, _ string) (*core.WorkflowGitInfo, error) {
	f.initCalled = true
	return f.initResult, f.initErr
}

func (f *fakeWorkflowWorktreeManager) FinalizeWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (f *fakeWorkflowWorktreeManager) CleanupWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (f *fakeWorkflowWorktreeManager) CreateTaskWorktree(_ context.Context, _ string, _ *core.Task) (*core.WorktreeInfo, error) {
	return nil, nil
}

func (f *fakeWorkflowWorktreeManager) RemoveTaskWorktree(_ context.Context, _ string, _ core.TaskID, _ bool) error {
	return nil
}

func (f *fakeWorkflowWorktreeManager) MergeTaskToWorkflow(_ context.Context, _ string, _ core.TaskID, _ string) error {
	return nil
}

func (f *fakeWorkflowWorktreeManager) MergeAllTasksToWorkflow(_ context.Context, _ string, _ []core.TaskID, _ string) error {
	return nil
}

func (f *fakeWorkflowWorktreeManager) GetWorkflowStatus(_ context.Context, _ string) (*core.WorkflowGitStatus, error) {
	return nil, nil
}

func (f *fakeWorkflowWorktreeManager) ListActiveWorkflows(_ context.Context) ([]*core.WorkflowGitInfo, error) {
	return nil, nil
}

func (f *fakeWorkflowWorktreeManager) GetWorkflowBranch(_ string) string {
	return ""
}

func (f *fakeWorkflowWorktreeManager) GetTaskBranch(_ string, _ core.TaskID) string {
	return ""
}

func TestEnsureWorkflowGitIsolation_NilDeps(t *testing.T) {
	t.Parallel()
	changed, err := EnsureWorkflowGitIsolation(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for nil deps")
	}
}

func TestEnsureWorkflowGitIsolation_NilState(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for nil state")
	}
}

func TestEnsureWorkflowGitIsolation_EmptyWorkflowID(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{}
	state := &core.WorkflowState{}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for empty workflow ID")
	}
}

func TestEnsureWorkflowGitIsolation_DryRun(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig: &workflow.RunnerConfig{DryRun: true},
		GitIsolation: &workflow.GitIsolationConfig{Enabled: true},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for dry run")
	}
}

func TestEnsureWorkflowGitIsolation_IsolationDisabled(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig: &workflow.RunnerConfig{},
		GitIsolation: &workflow.GitIsolationConfig{Enabled: false},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for disabled isolation")
	}
}

func TestEnsureWorkflowGitIsolation_NilGitIsolation(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig: &workflow.RunnerConfig{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for nil GitIsolation")
	}
}

func TestEnsureWorkflowGitIsolation_NilWorktreeManager(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: nil,
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false for nil WorkflowWorktrees")
	}
}

func TestEnsureWorkflowGitIsolation_AlreadyHasBranch(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			WorkflowBranch: "existing-branch",
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false when branch already exists")
	}
}

func TestEnsureWorkflowGitIsolation_TasksAlreadyRan(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusCompleted},
			},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false when tasks already ran")
	}
}

func TestEnsureWorkflowGitIsolation_TaskWithBranch(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusPending, Branch: "some-branch"},
			},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false when task has branch set")
	}
}

func TestEnsureWorkflowGitIsolation_TaskWithLastCommit(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusPending, LastCommit: "abc123"},
			},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false when task has last commit set")
	}
}

func TestEnsureWorkflowGitIsolation_TaskWithWorktreePath(t *testing.T) {
	t.Parallel()
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &fakeWorkflowWorktreeManager{},
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": {Status: core.TaskStatusPending, WorktreePath: "/tmp/wt"},
			},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Error("expected changed=false when task has worktree path set")
	}
}

func TestEnsureWorkflowGitIsolation_NilTaskSkipped(t *testing.T) {
	t.Parallel()
	fakeMgr := &fakeWorkflowWorktreeManager{
		initResult: &core.WorkflowGitInfo{WorkflowBranch: "wf-branch-new"},
	}
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: fakeMgr,
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Tasks: map[core.TaskID]*core.TaskState{
				"t1": nil, // nil task should be skipped
			},
		},
	}
	changed, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when nil task is skipped and init succeeds")
	}
	if state.WorkflowBranch != "wf-branch-new" {
		t.Errorf("expected workflow branch to be set, got %s", state.WorkflowBranch)
	}
}

func TestEnsureWorkflowGitIsolation_InitError(t *testing.T) {
	t.Parallel()
	fakeMgr := &fakeWorkflowWorktreeManager{
		initErr: context.DeadlineExceeded,
	}
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: fakeMgr,
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	_, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err == nil {
		t.Fatal("expected error from init")
	}
}

func TestEnsureWorkflowGitIsolation_EmptyBranchFromInit(t *testing.T) {
	t.Parallel()
	fakeMgr := &fakeWorkflowWorktreeManager{
		initResult: &core.WorkflowGitInfo{WorkflowBranch: ""},
	}
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: fakeMgr,
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	_, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err == nil {
		t.Fatal("expected error for empty branch from init")
	}
}

func TestEnsureWorkflowGitIsolation_NilInfoFromInit(t *testing.T) {
	t.Parallel()
	fakeMgr := &fakeWorkflowWorktreeManager{
		initResult: nil,
	}
	deps := &PhaseRunnerDeps{
		RunnerConfig:      &workflow.RunnerConfig{},
		GitIsolation:      &workflow.GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: fakeMgr,
	}
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
	}
	_, err := EnsureWorkflowGitIsolation(context.Background(), deps, state)
	if err == nil {
		t.Fatal("expected error for nil info from init")
	}
}

// --- InitializeWorkflowState ---

func TestInitializeWorkflowState_NilBlueprint(t *testing.T) {
	t.Parallel()
	state := InitializeWorkflowState("my prompt", nil)
	if state.Prompt != "my prompt" {
		t.Errorf("expected prompt 'my prompt', got %s", state.Prompt)
	}
	if state.Blueprint != nil {
		t.Error("expected nil blueprint")
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("expected running status, got %s", state.Status)
	}
	if state.CurrentPhase != core.PhaseRefine {
		t.Errorf("expected refine phase, got %s", state.CurrentPhase)
	}
	if state.Tasks == nil {
		t.Error("expected initialized tasks map")
	}
	if state.TaskOrder == nil {
		t.Error("expected initialized task order slice")
	}
	if state.Checkpoints == nil {
		t.Error("expected initialized checkpoints slice")
	}
	if state.Metrics == nil {
		t.Error("expected initialized metrics")
	}
}

func TestInitializeWorkflowState_EmptyPrompt(t *testing.T) {
	t.Parallel()
	state := InitializeWorkflowState("", nil)
	if state.Prompt != "" {
		t.Error("expected empty prompt")
	}
	if state.WorkflowID == "" {
		t.Error("expected non-empty workflow ID")
	}
}

// --- generateCmdWorkflowID ---

func TestGenerateCmdWorkflowID_Uniqueness(t *testing.T) {
	t.Parallel()
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateCmdWorkflowID()
		if ids[id] {
			t.Fatalf("duplicate workflow ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateCmdWorkflowID_HasPrefix(t *testing.T) {
	t.Parallel()
	id := generateCmdWorkflowID()
	if len(id) < 3 || id[:3] != "wf-" {
		t.Errorf("expected ID to start with 'wf-', got %s", id)
	}
}

// --- handleTUICompletion ---

func TestHandleTUICompletion_NilChannel(t *testing.T) {
	t.Parallel()
	err := handleTUICompletion(nil, nil)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestHandleTUICompletion_NilChannelWithWorkflowError(t *testing.T) {
	t.Parallel()
	wfErr := context.DeadlineExceeded
	err := handleTUICompletion(nil, wfErr)
	if err != wfErr {
		t.Errorf("expected workflow error, got %v", err)
	}
}

func TestHandleTUICompletion_ChannelWithTUIError(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1)
	tuiErr := context.Canceled
	ch <- tuiErr
	err := handleTUICompletion(ch, nil)
	if err != tuiErr {
		t.Errorf("expected TUI error, got %v", err)
	}
}

func TestHandleTUICompletion_ChannelWithBothErrors(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1)
	ch <- context.Canceled
	wfErr := context.DeadlineExceeded
	// When both TUI error and workflow error exist, workflow error wins
	err := handleTUICompletion(ch, wfErr)
	if err != wfErr {
		t.Errorf("expected workflow error to take precedence, got %v", err)
	}
}

func TestHandleTUICompletion_ChannelNoReadyValue(t *testing.T) {
	t.Parallel()
	ch := make(chan error, 1)
	// Channel exists but nothing ready -- should return workflow error (nil)
	err := handleTUICompletion(ch, nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// --- defaultWorkflowTimeout / defaultPhaseTimeout constants ---

func TestDefaultTimeoutConstants(t *testing.T) {
	t.Parallel()
	if defaultWorkflowTimeout != 12*time.Hour {
		t.Errorf("expected 12h, got %v", defaultWorkflowTimeout)
	}
	if defaultPhaseTimeout != 2*time.Hour {
		t.Errorf("expected 2h, got %v", defaultPhaseTimeout)
	}
}
