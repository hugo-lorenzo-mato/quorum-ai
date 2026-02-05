package workflow

import (
	"context"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/report"
)

type mockWorkflowIsolationManager struct {
	initCalls []struct {
		workflowID string
		baseBranch string
	}
}

func (m *mockWorkflowIsolationManager) InitializeWorkflow(_ context.Context, workflowID string, baseBranch string) (*core.WorkflowGitInfo, error) {
	m.initCalls = append(m.initCalls, struct {
		workflowID string
		baseBranch string
	}{workflowID: workflowID, baseBranch: baseBranch})

	return &core.WorkflowGitInfo{
		WorkflowID:     workflowID,
		WorkflowBranch: "quorum/" + workflowID,
		BaseBranch:     baseBranch,
	}, nil
}

func (m *mockWorkflowIsolationManager) FinalizeWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockWorkflowIsolationManager) CleanupWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockWorkflowIsolationManager) CreateTaskWorktree(_ context.Context, _ string, task *core.Task) (*core.WorktreeInfo, error) {
	return &core.WorktreeInfo{TaskID: task.ID, Path: "/tmp", Branch: "branch"}, nil
}

func (m *mockWorkflowIsolationManager) RemoveTaskWorktree(_ context.Context, _ string, _ core.TaskID, _ bool) error {
	return nil
}

func (m *mockWorkflowIsolationManager) MergeTaskToWorkflow(_ context.Context, _ string, _ core.TaskID, _ string) error {
	return nil
}

func (m *mockWorkflowIsolationManager) MergeAllTasksToWorkflow(_ context.Context, _ string, _ []core.TaskID, _ string) error {
	return nil
}

func (m *mockWorkflowIsolationManager) GetWorkflowStatus(_ context.Context, _ string) (*core.WorkflowGitStatus, error) {
	return &core.WorkflowGitStatus{}, nil
}

func (m *mockWorkflowIsolationManager) ListActiveWorkflows(_ context.Context) ([]*core.WorkflowGitInfo, error) {
	return nil, nil
}

func (m *mockWorkflowIsolationManager) GetWorkflowBranch(workflowID string) string {
	return "quorum/" + workflowID
}

func (m *mockWorkflowIsolationManager) GetTaskBranch(workflowID string, taskID core.TaskID) string {
	return "quorum/" + workflowID + "__" + string(taskID)
}

func TestRunner_ensureWorkflowGitIsolation_SetsWorkflowBranch(t *testing.T) {
	mgr := &mockWorkflowIsolationManager{}
	r := &Runner{
		config: &RunnerConfig{DryRun: false},
		gitIsolation: &GitIsolationConfig{
			Enabled: true,
		},
		workflowWorktrees: mgr,
		logger:            logging.NewNop(),
	}

	state := &core.WorkflowState{WorkflowID: "wf-001"}

	changed, err := r.ensureWorkflowGitIsolation(context.Background(), state)
	if err != nil {
		t.Fatalf("ensureWorkflowGitIsolation() error = %v", err)
	}
	if !changed {
		t.Fatalf("ensureWorkflowGitIsolation() changed = false, want true")
	}
	if state.WorkflowBranch != "quorum/wf-001" {
		t.Fatalf("WorkflowBranch = %q, want %q", state.WorkflowBranch, "quorum/wf-001")
	}
	if len(mgr.initCalls) != 1 {
		t.Fatalf("InitializeWorkflow calls = %d, want 1", len(mgr.initCalls))
	}
	if mgr.initCalls[0].workflowID != "wf-001" {
		t.Fatalf("InitializeWorkflow workflowID = %q, want %q", mgr.initCalls[0].workflowID, "wf-001")
	}
}

func TestRunner_createContext_DisablesTaskPRsUnderIsolation(t *testing.T) {
	r := &Runner{
		config: &RunnerConfig{
			Finalization: FinalizationConfig{
				AutoPR:    true,
				AutoMerge: true,
			},
			Report: report.Config{Enabled: false},
		},
		gitIsolation:      &GitIsolationConfig{Enabled: true},
		workflowWorktrees: &mockWorkflowIsolationManager{},
		logger:            logging.NewNop(),
	}

	state := &core.WorkflowState{
		WorkflowID:     "wf-001",
		WorkflowBranch: "quorum/wf-001",
	}

	wctx := r.createContext(state)
	if wctx.Config.Finalization.AutoPR {
		t.Fatalf("Finalization.AutoPR = true, want false under workflow isolation")
	}
	if wctx.Config.Finalization.AutoMerge {
		t.Fatalf("Finalization.AutoMerge = true, want false under workflow isolation")
	}
}
