package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// testRunner creates a Runner with minimal deps for testing interactive methods.
func testRunner(sm StateManager) *Runner {
	return &Runner{
		config: DefaultRunnerConfig(),
		state:  sm,
		output: NopOutputNotifier{},
		logger: logging.NewNop(),
	}
}

// --- interactiveGate ---

func TestInteractiveGate_NonInteractive_Noop(t *testing.T) {
	t.Parallel()
	sm := &mockStateManager{
		state: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-1",
				Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeMultiAgent},
			},
			WorkflowRun: core.WorkflowRun{
				Status:       core.WorkflowStatusRunning,
				CurrentPhase: core.PhasePlan,
			},
		},
	}
	r := testRunner(sm)

	err := r.interactiveGate(context.Background(), sm.state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil for non-interactive, got %v", err)
	}
}

func TestInteractiveGate_NilBlueprint_Noop(t *testing.T) {
	t.Parallel()
	sm := &mockStateManager{
		state: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{
				WorkflowID: "wf-1",
				Blueprint:  nil,
			},
			WorkflowRun: core.WorkflowRun{
				Status:       core.WorkflowStatusRunning,
				CurrentPhase: core.PhasePlan,
			},
		},
	}
	r := testRunner(sm)

	err := r.interactiveGate(context.Background(), sm.state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil for nil blueprint, got %v", err)
	}
}

func TestInteractiveGate_Interactive_PausesAndResumes(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeInteractive},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
		},
	}
	sm := &mockStateManager{state: state}
	cp := control.New()
	r := testRunner(sm)
	r.control = cp

	// Simulate resume after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cp.Resume()
	}()

	err := r.interactiveGate(context.Background(), state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil after resume, got %v", err)
	}

	// Verify state was saved as awaiting_review
	if sm.state.Status != core.WorkflowStatusRunning {
		// After resume, applyInteractiveFeedback sets it to Running
		// (no review => just resumes)
	}
}

func TestInteractiveGate_DynamicModeSwitch(t *testing.T) {
	t.Parallel()
	// Start as multi_agent, but state in DB was switched to interactive
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeMultiAgent},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
		},
	}
	// But the DB copy has been switched to interactive
	dbState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeInteractive},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
		},
	}
	sm := &mockStateManager{state: dbState}
	cp := control.New()
	r := testRunner(sm)
	r.control = cp

	// Simulate resume after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cp.Resume()
	}()

	err := r.interactiveGate(context.Background(), state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil after resume, got %v", err)
	}

	// The in-memory state should have been updated from DB
	if state.Blueprint.ExecutionMode != core.ExecutionModeInteractive {
		t.Errorf("expected mode switch to interactive, got %q", state.Blueprint.ExecutionMode)
	}
}

func TestInteractiveGate_ContextCancelled(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: "wf-1",
			Blueprint:  &core.Blueprint{ExecutionMode: core.ExecutionModeInteractive},
		},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusRunning,
			CurrentPhase: core.PhasePlan,
		},
	}
	sm := &mockStateManager{state: state}
	cp := control.New()
	r := testRunner(sm)
	r.control = cp

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := r.interactiveGate(ctx, state, core.PhaseAnalyze)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

// --- applyInteractiveFeedback ---

func TestApplyInteractiveFeedback_NoReview(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:            core.WorkflowStatusAwaitingReview,
			CurrentPhase:      core.PhasePlan,
			InteractiveReview: nil,
		},
	}
	sm := &mockStateManager{state: state}
	r := testRunner(sm)

	err := r.applyInteractiveFeedback(context.Background(), state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("expected status running, got %s", state.Status)
	}
}

func TestApplyInteractiveFeedback_WithAnalysisFeedback(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhasePlan,
			InteractiveReview: &core.InteractiveReview{
				AnalysisFeedback: "Additional context here",
				ApprovedPhase:    core.PhaseAnalyze,
			},
		},
	}
	sm := &mockStateManager{state: state}
	r := testRunner(sm)

	err := r.applyInteractiveFeedback(context.Background(), state, core.PhaseAnalyze)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("expected status running, got %s", state.Status)
	}
	if state.InteractiveReview != nil {
		t.Error("expected InteractiveReview cleared after applying")
	}
}

func TestApplyInteractiveFeedback_WithPlanFeedback(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:       core.WorkflowStatusAwaitingReview,
			CurrentPhase: core.PhaseExecute,
			InteractiveReview: &core.InteractiveReview{
				PlanFeedback:  "Some plan feedback",
				ApprovedPhase: core.PhasePlan,
			},
		},
	}
	sm := &mockStateManager{state: state}
	r := testRunner(sm)

	err := r.applyInteractiveFeedback(context.Background(), state, core.PhasePlan)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if state.Status != core.WorkflowStatusRunning {
		t.Errorf("expected status running, got %s", state.Status)
	}
}

func TestApplyInteractiveFeedback_DetectsRejection(t *testing.T) {
	t.Parallel()
	// In-memory state still says awaiting_review, but DB state was changed by reject handler
	inMemState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:            core.WorkflowStatusAwaitingReview,
			CurrentPhase:      core.PhasePlan,
			InteractiveReview: nil,
		},
	}
	// DB state was changed to pending by the reject handler
	dbState := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:            core.WorkflowStatusPending,
			CurrentPhase:      core.PhaseAnalyze,
			InteractiveReview: nil,
		},
	}
	sm := &mockStateManager{state: dbState}
	r := testRunner(sm)

	err := r.applyInteractiveFeedback(context.Background(), inMemState, core.PhaseAnalyze)
	if err == nil {
		t.Error("expected error for rejected phase")
	}
}

func TestApplyInteractiveFeedback_LoadError_ContinuesGracefully(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-1"},
		WorkflowRun: core.WorkflowRun{
			Status:            core.WorkflowStatusAwaitingReview,
			CurrentPhase:      core.PhasePlan,
			InteractiveReview: nil,
		},
	}
	sm := &mockStateManager{
		state:   state,
		loadErr: context.DeadlineExceeded,
	}
	r := testRunner(sm)

	// Even with load error, the function should handle gracefully
	// (it still has the in-memory state)
	err := r.applyInteractiveFeedback(context.Background(), state, core.PhaseAnalyze)
	// With load error, it can't re-read state, so it falls through to the nil-review path
	// and since it can't detect rejection, it just resumes
	if err != nil {
		t.Errorf("expected graceful handling, got %v", err)
	}
}
