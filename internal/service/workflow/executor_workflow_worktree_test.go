package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// mockWorkflowWorktreeManager implements core.WorkflowWorktreeManager for tests.
type mockWorkflowWorktreeManager struct {
	createInfo  *core.WorktreeInfo
	createErr   error
	mergeErr    error
	removeErr   error
	createCalls []createTaskWorktreeCall
	mergeCalls  []mergeTaskCall
	removeCalls []removeTaskWorktreeCall
}

type createTaskWorktreeCall struct {
	workflowID string
	task       *core.Task
}

type mergeTaskCall struct {
	workflowID string
	taskID     core.TaskID
	strategy   string
}

type removeTaskWorktreeCall struct {
	workflowID   string
	taskID       core.TaskID
	removeBranch bool
}

func (m *mockWorkflowWorktreeManager) CreateTaskWorktree(_ context.Context, workflowID string, task *core.Task) (*core.WorktreeInfo, error) {
	m.createCalls = append(m.createCalls, createTaskWorktreeCall{workflowID, task})
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.createInfo, nil
}

func (m *mockWorkflowWorktreeManager) MergeTaskToWorkflow(_ context.Context, workflowID string, taskID core.TaskID, strategy string) error {
	m.mergeCalls = append(m.mergeCalls, mergeTaskCall{workflowID, taskID, strategy})
	return m.mergeErr
}

func (m *mockWorkflowWorktreeManager) RemoveTaskWorktree(_ context.Context, workflowID string, taskID core.TaskID, removeBranch bool) error {
	m.removeCalls = append(m.removeCalls, removeTaskWorktreeCall{workflowID, taskID, removeBranch})
	return m.removeErr
}

func (m *mockWorkflowWorktreeManager) GetWorkflowBranch(workflowID string) string {
	return "quorum/" + workflowID
}

func (m *mockWorkflowWorktreeManager) GetTaskBranch(workflowID string, taskID core.TaskID) string {
	return "quorum/" + workflowID + "/" + string(taskID)
}

func (m *mockWorkflowWorktreeManager) InitializeWorkflow(_ context.Context, workflowID string, baseBranch string) (*core.WorkflowGitInfo, error) {
	return &core.WorkflowGitInfo{
		WorkflowID:     workflowID,
		WorkflowBranch: "quorum/" + workflowID,
		BaseBranch:     baseBranch,
	}, nil
}

func (m *mockWorkflowWorktreeManager) FinalizeWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockWorkflowWorktreeManager) CleanupWorkflow(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockWorkflowWorktreeManager) MergeAllTasksToWorkflow(_ context.Context, _ string, _ []core.TaskID, _ string) error {
	return nil
}

func (m *mockWorkflowWorktreeManager) GetWorkflowStatus(_ context.Context, _ string) (*core.WorkflowGitStatus, error) {
	return &core.WorkflowGitStatus{}, nil
}

func (m *mockWorkflowWorktreeManager) ListActiveWorkflows(_ context.Context) ([]*core.WorkflowGitInfo, error) {
	return nil, nil
}

// TestContext_UseWorkflowIsolation tests the UseWorkflowIsolation method.
func TestContext_UseWorkflowIsolation(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *Context
		expected bool
	}{
		{
			name: "enabled with all fields",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: true},
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             &core.WorkflowState{WorkflowRun: core.WorkflowRun{WorkflowBranch: "quorum/wf-001"}},
			},
			expected: true,
		},
		{
			name: "disabled in config",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: false},
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             &core.WorkflowState{WorkflowRun: core.WorkflowRun{WorkflowBranch: "quorum/wf-001"}},
			},
			expected: false,
		},
		{
			name: "nil git isolation",
			ctx: &Context{
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             &core.WorkflowState{WorkflowRun: core.WorkflowRun{WorkflowBranch: "quorum/wf-001"}},
			},
			expected: false,
		},
		{
			name: "missing worktree manager",
			ctx: &Context{
				GitIsolation: &GitIsolationConfig{Enabled: true},
				State:        &core.WorkflowState{WorkflowRun: core.WorkflowRun{WorkflowBranch: "quorum/wf-001"}},
			},
			expected: false,
		},
		{
			name: "missing workflow branch",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: true},
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             &core.WorkflowState{},
			},
			expected: false,
		},
		{
			name: "nil state",
			ctx: &Context{
				GitIsolation:      &GitIsolationConfig{Enabled: true},
				WorkflowWorktrees: &mockWorkflowWorktreeManager{},
				State:             nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctx.UseWorkflowIsolation()
			if result != tt.expected {
				t.Errorf("UseWorkflowIsolation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestExecutor_setupWorkflowScopedWorktree_Isolation tests worktree creation with isolation enabled.
func TestExecutor_setupWorkflowScopedWorktree_Isolation(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{
		createInfo: &core.WorktreeInfo{
			Path:   "/tmp/worktrees/wf-001/task-001",
			Branch: "quorum/wf-001/task-001",
		},
	}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}
	taskState := &core.TaskState{}

	workDir, created := executor.setupWorkflowScopedWorktree(context.Background(), wctx, task, taskState, true)

	if !created {
		t.Error("Expected worktree to be created")
	}
	if workDir != "/tmp/worktrees/wf-001/task-001" {
		t.Errorf("Expected workDir = /tmp/worktrees/wf-001/task-001, got %s", workDir)
	}
	if taskState.Branch != "quorum/wf-001/task-001" {
		t.Errorf("Expected branch = quorum/wf-001/task-001, got %s", taskState.Branch)
	}
	if len(mockWtMgr.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(mockWtMgr.createCalls))
	}
	if mockWtMgr.createCalls[0].workflowID != "wf-001" {
		t.Errorf("Expected workflowID = wf-001, got %s", mockWtMgr.createCalls[0].workflowID)
	}
}

// TestExecutor_setupWorkflowScopedWorktree_Fallback tests fallback to legacy worktree.
func TestExecutor_setupWorkflowScopedWorktree_Fallback(t *testing.T) {
	mockLegacyMgr := &mockWorktreeManager{
		createInfo: &core.WorktreeInfo{
			Path:   "/tmp/worktrees/task-001",
			Branch: "quorum/task-001",
		},
	}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation: nil, // Not enabled
		Worktrees:    mockLegacyMgr,
		State:        &core.WorkflowState{WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"}},
		Logger:       logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}
	taskState := &core.TaskState{}

	workDir, created := executor.setupWorkflowScopedWorktree(context.Background(), wctx, task, taskState, true)

	if !created {
		t.Error("Expected worktree to be created via legacy manager")
	}
	if workDir != "/tmp/worktrees/task-001" {
		t.Errorf("Expected workDir = /tmp/worktrees/task-001, got %s", workDir)
	}
}

// TestExecutor_setupWorkflowScopedWorktree_NotEnabled tests when useWorktrees is false.
func TestExecutor_setupWorkflowScopedWorktree_NotEnabled(t *testing.T) {
	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: &mockWorkflowWorktreeManager{},
		State:             &core.WorkflowState{WorkflowRun: core.WorkflowRun{WorkflowBranch: "quorum/wf-001"}},
		Logger:            logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}
	taskState := &core.TaskState{}

	workDir, created := executor.setupWorkflowScopedWorktree(context.Background(), wctx, task, taskState, false)

	if created {
		t.Error("Expected worktree not to be created when useWorktrees=false")
	}
	if workDir != "" {
		t.Errorf("Expected empty workDir, got %s", workDir)
	}
}

// TestExecutor_mergeTaskToWorkflow_Success tests successful merge.
func TestExecutor_mergeTaskToWorkflow_Success(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true, MergeStrategy: "sequential"},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	err := executor.mergeTaskToWorkflow(context.Background(), wctx, task)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(mockWtMgr.mergeCalls) != 1 {
		t.Errorf("Expected 1 merge call, got %d", len(mockWtMgr.mergeCalls))
	}
	if mockWtMgr.mergeCalls[0].strategy != "sequential" {
		t.Errorf("Expected strategy = sequential, got %s", mockWtMgr.mergeCalls[0].strategy)
	}
}

// TestExecutor_mergeTaskToWorkflow_DefaultStrategy tests default merge strategy.
func TestExecutor_mergeTaskToWorkflow_DefaultStrategy(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true, MergeStrategy: ""}, // Empty strategy
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	err := executor.mergeTaskToWorkflow(context.Background(), wctx, task)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if mockWtMgr.mergeCalls[0].strategy != "sequential" {
		t.Errorf("Expected default strategy = sequential, got %s", mockWtMgr.mergeCalls[0].strategy)
	}
}

// TestExecutor_mergeTaskToWorkflow_Conflict tests merge conflict handling.
func TestExecutor_mergeTaskToWorkflow_Conflict(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{
		mergeErr: errors.New("merge conflict: files conflict"),
	}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true, MergeStrategy: "sequential"},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun: core.WorkflowRun{
				WorkflowBranch: "quorum/wf-001",
				Tasks:          map[core.TaskID]*core.TaskState{"task-001": {}},
			},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	err := executor.mergeTaskToWorkflow(context.Background(), wctx, task)

	if err == nil {
		t.Error("Expected error for merge conflict")
	}
	taskState := wctx.State.Tasks["task-001"]
	if !taskState.Resumable {
		t.Error("Expected task to be marked as resumable")
	}
	if !taskState.MergePending {
		t.Error("Expected MergePending to be true")
	}
}

// TestExecutor_mergeTaskToWorkflow_NoIsolation tests merge when isolation is disabled.
func TestExecutor_mergeTaskToWorkflow_NoIsolation(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      nil, // Not enabled
		WorkflowWorktrees: mockWtMgr,
		State:             &core.WorkflowState{WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"}},
		Logger:            logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	err := executor.mergeTaskToWorkflow(context.Background(), wctx, task)

	if err != nil {
		t.Errorf("Expected no error when isolation disabled, got %v", err)
	}
	if len(mockWtMgr.mergeCalls) != 0 {
		t.Errorf("Expected no merge calls when isolation disabled, got %d", len(mockWtMgr.mergeCalls))
	}
}

// TestExecutor_cleanupWorkflowScopedWorktree_Isolation tests cleanup with isolation.
func TestExecutor_cleanupWorkflowScopedWorktree_Isolation(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	executor.cleanupWorkflowScopedWorktree(context.Background(), wctx, task, true)

	if len(mockWtMgr.removeCalls) != 1 {
		t.Errorf("Expected 1 remove call, got %d", len(mockWtMgr.removeCalls))
	}
	// Should not remove branch (false) - it's needed for merge
	if mockWtMgr.removeCalls[0].removeBranch {
		t.Error("Expected removeBranch = false")
	}
}

// TestExecutor_cleanupWorkflowScopedWorktree_NotCreated tests cleanup when worktree was not created.
func TestExecutor_cleanupWorkflowScopedWorktree_NotCreated(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	// worktreeCreated = false
	executor.cleanupWorkflowScopedWorktree(context.Background(), wctx, task, false)

	if len(mockWtMgr.removeCalls) != 0 {
		t.Errorf("Expected no remove calls when worktree was not created, got %d", len(mockWtMgr.removeCalls))
	}
}

// TestExecutor_setupWorktreeWithIsolation_CreateError tests error handling during worktree creation.
func TestExecutor_setupWorktreeWithIsolation_CreateError(t *testing.T) {
	mockWtMgr := &mockWorkflowWorktreeManager{
		createErr: errors.New("disk full"),
	}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation:      &GitIsolationConfig{Enabled: true},
		WorkflowWorktrees: mockWtMgr,
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-001"},
			WorkflowRun:        core.WorkflowRun{WorkflowBranch: "quorum/wf-001"},
		},
		Logger: logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}
	taskState := &core.TaskState{}

	workDir, created := executor.setupWorktreeWithIsolation(context.Background(), wctx, task, taskState)

	// Should fall back to non-isolated execution
	if created {
		t.Error("Expected worktree not to be created on error")
	}
	if workDir != "" {
		t.Errorf("Expected empty workDir on error, got %s", workDir)
	}
}
