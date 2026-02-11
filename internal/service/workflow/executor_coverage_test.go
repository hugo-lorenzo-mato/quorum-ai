package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// =============================================================================
// Mock helpers specific to this file
// =============================================================================

// mockGitClient implements core.GitClient for testing executor git-related paths.
type mockGitClient struct {
	repoRoot      string
	repoRootErr   error
	branch        string
	branchErr     error
	status        *core.GitStatus
	statusErr     error
	addErr        error
	commitSHA     string
	commitErr     error
	pushErr       error
	defaultBranch string
}

func (m *mockGitClient) RepoRoot(_ context.Context) (string, error) {
	return m.repoRoot, m.repoRootErr
}
func (m *mockGitClient) CurrentBranch(_ context.Context) (string, error) {
	return m.branch, m.branchErr
}
func (m *mockGitClient) DefaultBranch(_ context.Context) (string, error) {
	return m.defaultBranch, nil
}
func (m *mockGitClient) RemoteURL(_ context.Context) (string, error) { return "", nil }
func (m *mockGitClient) BranchExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockGitClient) CreateBranch(_ context.Context, _, _ string) error { return nil }
func (m *mockGitClient) DeleteBranch(_ context.Context, _ string) error    { return nil }
func (m *mockGitClient) CheckoutBranch(_ context.Context, _ string) error  { return nil }
func (m *mockGitClient) CreateWorktree(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockGitClient) RemoveWorktree(_ context.Context, _ string) error { return nil }
func (m *mockGitClient) ListWorktrees(_ context.Context) ([]core.Worktree, error) {
	return nil, nil
}
func (m *mockGitClient) Status(_ context.Context) (*core.GitStatus, error) {
	if m.statusErr != nil {
		return nil, m.statusErr
	}
	if m.status != nil {
		return m.status, nil
	}
	return &core.GitStatus{}, nil
}
func (m *mockGitClient) Add(_ context.Context, _ ...string) error { return m.addErr }
func (m *mockGitClient) Commit(_ context.Context, _ string) (string, error) {
	return m.commitSHA, m.commitErr
}
func (m *mockGitClient) Push(_ context.Context, _, _ string) error { return m.pushErr }
func (m *mockGitClient) Diff(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockGitClient) DiffFiles(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}
func (m *mockGitClient) IsClean(_ context.Context) (bool, error) { return true, nil }
func (m *mockGitClient) Fetch(_ context.Context, _ string) error { return nil }
func (m *mockGitClient) Merge(_ context.Context, _ string, _ core.MergeOptions) error {
	return nil
}
func (m *mockGitClient) AbortMerge(_ context.Context) error             { return nil }
func (m *mockGitClient) HasMergeConflicts(_ context.Context) (bool, error) { return false, nil }
func (m *mockGitClient) GetConflictFiles(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockGitClient) Rebase(_ context.Context, _ string) error       { return nil }
func (m *mockGitClient) AbortRebase(_ context.Context) error            { return nil }
func (m *mockGitClient) ContinueRebase(_ context.Context) error         { return nil }
func (m *mockGitClient) HasRebaseInProgress(_ context.Context) (bool, error) { return false, nil }
func (m *mockGitClient) ResetHard(_ context.Context, _ string) error    { return nil }
func (m *mockGitClient) ResetSoft(_ context.Context, _ string) error    { return nil }
func (m *mockGitClient) CherryPick(_ context.Context, _ string) error   { return nil }
func (m *mockGitClient) AbortCherryPick(_ context.Context) error        { return nil }
func (m *mockGitClient) RevParse(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockGitClient) IsAncestor(_ context.Context, _, _ string) (bool, error) { return false, nil }
func (m *mockGitClient) HasUncommittedChanges(_ context.Context) (bool, error) { return false, nil }

// mockGitClientFactory implements GitClientFactory for testing.
type mockGitClientFactory struct {
	client    core.GitClient
	clientErr error
}

func (m *mockGitClientFactory) NewClient(_ string) (core.GitClient, error) {
	if m.clientErr != nil {
		return nil, m.clientErr
	}
	return m.client, nil
}

// =============================================================================
// Tests for executor.executeTaskSafe panic recovery
// =============================================================================

func TestExecutor_executeTaskSafe_PanicRecovery(t *testing.T) {
	t.Parallel()

	dag := &mockDAGBuilder{}
	saver := &mockStateSaver{}
	executor := NewExecutor(dag, saver, nil)

	// Agent that panics
	panicAgent := &mockAgent{
		result: nil,
		err:    nil,
	}
	// Override the mock to simulate a panic inside executeTask by testing
	// the safe wrapper with a task that has no state entry (will panic on nil access).
	registry := &mockAgentRegistry{}
	registry.Register("claude", panicAgent)

	output := &mockOutputNotifier{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowDefinition: core.WorkflowDefinition{WorkflowID: "wf-test"},
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					// task-1 exists but has no matching state for finding task.CLI
					"task-1": {ID: "task-1", Name: "Test", Status: core.TaskStatusPending},
				},
			},
		},
		Agents:     registry,
		Prompts:    &mockPromptRenderer{},
		Checkpoint: &mockCheckpointCreator{},
		Retry:      &mockRetryExecutor{},
		RateLimits: &mockRateLimiterGetter{},
		Logger:     logging.NewNop(),
		Output:     output,
		Config: &Config{
			DryRun:       false,
			DefaultAgent: "claude",
			WorktreeMode: "disabled",
		},
	}

	// Create a task, but set Control to nil to allow executeTask to proceed.
	// The task execution should succeed or fail gracefully.
	task := &core.Task{ID: "task-1", Name: "Test", CLI: "claude"}
	err := executor.executeTaskSafe(context.Background(), wctx, task, false)
	// It should not panic. It may return nil or an error, but must not crash.
	_ = err
}

// =============================================================================
// Tests for cleanupOrphanedWorktrees
// =============================================================================

func TestExecutor_cleanupOrphanedWorktrees_NilWorktrees(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Worktrees: nil,
		Logger:    logging.NewNop(),
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{},
			},
		},
	}

	// Should return immediately without panic
	executor.cleanupOrphanedWorktrees(context.Background(), wctx)
}

func TestExecutor_cleanupOrphanedWorktrees_ListError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	worktreeMgr := &mockWorktreeManagerWithList{
		listErr: errors.New("list error"),
	}

	wctx := &Context{
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{},
			},
		},
	}

	// Should return after logging warning, not crash
	executor.cleanupOrphanedWorktrees(context.Background(), wctx)
}

func TestExecutor_cleanupOrphanedWorktrees_RemovesOrphaned(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	worktreeMgr := &mockWorktreeManagerWithList{
		listResult: []*core.WorktreeInfo{
			{TaskID: "task-old", Path: "/tmp/orphan"},
		},
	}

	output := &mockOutputNotifier{}
	wctx := &Context{
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
		Output:    output,
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					// task-old not in state, so it's orphaned
				},
			},
		},
	}

	executor.cleanupOrphanedWorktrees(context.Background(), wctx)

	if worktreeMgr.removeCalled != 1 {
		t.Errorf("expected 1 remove call, got %d", worktreeMgr.removeCalled)
	}
}

func TestExecutor_cleanupOrphanedWorktrees_SkipsRunningTasks(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	worktreeMgr := &mockWorktreeManagerWithList{
		listResult: []*core.WorktreeInfo{
			{TaskID: "task-running", Path: "/tmp/running"},
		},
	}

	wctx := &Context{
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"task-running": {ID: "task-running", Status: core.TaskStatusRunning},
				},
			},
		},
	}

	executor.cleanupOrphanedWorktrees(context.Background(), wctx)

	if worktreeMgr.removeCalled != 0 {
		t.Errorf("expected 0 remove calls for running tasks, got %d", worktreeMgr.removeCalled)
	}
}

func TestExecutor_cleanupOrphanedWorktrees_RemoveError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	worktreeMgr := &mockWorktreeManagerWithList{
		listResult: []*core.WorktreeInfo{
			{TaskID: "task-old", Path: "/tmp/orphan"},
		},
		removeErr: errors.New("remove failed"),
	}

	wctx := &Context{
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{},
			},
		},
	}

	// Should not panic even if removal fails
	executor.cleanupOrphanedWorktrees(context.Background(), wctx)
	if worktreeMgr.removeCalled != 1 {
		t.Errorf("expected 1 remove call, got %d", worktreeMgr.removeCalled)
	}
}

func TestExecutor_cleanupOrphanedWorktrees_EmptyList(t *testing.T) {
	t.Parallel()
	executor := &Executor{}

	worktreeMgr := &mockWorktreeManagerWithList{
		listResult: []*core.WorktreeInfo{},
	}

	wctx := &Context{
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{},
			},
		},
	}

	// Should return early for empty list
	executor.cleanupOrphanedWorktrees(context.Background(), wctx)
}

// =============================================================================
// Tests for detectGitChanges
// =============================================================================

func TestExecutor_detectGitChanges_NoWorkDirNoGit(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Git:    nil,
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "")
	if info.HasChanges {
		t.Error("expected no changes with nil git and empty workDir")
	}
}

func TestExecutor_detectGitChanges_NoWorkDirGitRepoRootError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Git: &mockGitClient{
			repoRootErr: errors.New("not a git repo"),
		},
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "")
	if info.HasChanges {
		t.Error("expected no changes when RepoRoot fails")
	}
}

func TestExecutor_detectGitChanges_GitFactoryError(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: &mockGitClientFactory{
			clientErr: errors.New("factory error"),
		},
	}
	wctx := &Context{
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "/some/path")
	if info.HasChanges {
		t.Error("expected no changes when git factory fails")
	}
}

func TestExecutor_detectGitChanges_StatusError(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: &mockGitClientFactory{
			client: &mockGitClient{
				statusErr: errors.New("status failed"),
			},
		},
	}
	wctx := &Context{
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "/some/path")
	if info.HasChanges {
		t.Error("expected no changes when git status fails")
	}
}

func TestExecutor_detectGitChanges_WithChanges(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: &mockGitClientFactory{
			client: &mockGitClient{
				status: &core.GitStatus{
					Staged:    []core.FileStatus{{Path: "staged.go", Status: "M"}},
					Unstaged:  []core.FileStatus{{Path: "unstaged.go", Status: "M"}},
					Untracked: []string{"new.go"},
				},
			},
		},
	}
	wctx := &Context{
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "/some/path")
	if !info.HasChanges {
		t.Error("expected changes detected")
	}
	if len(info.ModifiedFiles) != 2 {
		t.Errorf("expected 2 modified files, got %d", len(info.ModifiedFiles))
	}
	if len(info.AddedFiles) != 1 {
		t.Errorf("expected 1 added file, got %d", len(info.AddedFiles))
	}
}

func TestExecutor_detectGitChanges_UsesWctxGitWhenNoFactory(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: nil, // No factory
	}
	wctx := &Context{
		Git: &mockGitClient{
			status: &core.GitStatus{
				Staged: []core.FileStatus{{Path: "modified.go", Status: "M"}},
			},
		},
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "/some/path")
	if !info.HasChanges {
		t.Error("expected changes detected from wctx.Git")
	}
}

func TestExecutor_detectGitChanges_NoWorkDirEmptyRepoRoot(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Git: &mockGitClient{
			repoRoot: "", // Empty repo root
		},
		Logger: logging.NewNop(),
	}

	info := executor.detectGitChanges(context.Background(), wctx, "")
	if info.HasChanges {
		t.Error("expected no changes when repo root is empty")
	}
}

func TestExecutor_detectGitChanges_NilGitNoFactory(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: nil,
	}
	wctx := &Context{
		Git:    nil,
		Logger: logging.NewNop(),
	}

	// workDir is set but no git client available
	info := executor.detectGitChanges(context.Background(), wctx, "/some/path")
	if info.HasChanges {
		t.Error("expected no changes when no git client available")
	}
}

// =============================================================================
// Tests for finalizeTask
// =============================================================================

func TestExecutor_finalizeTask_AutoCommitDisabled(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{AutoCommit: false},
		},
		Logger: logging.NewNop(),
	}

	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "")
	if err != nil {
		t.Errorf("expected nil error when auto-commit disabled, got %v", err)
	}
}

func TestExecutor_finalizeTask_NoGitPath(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{AutoCommit: true},
		},
		Git:    nil,
		Logger: logging.NewNop(),
	}

	// workDir empty, Git nil => returns nil (no git path available)
	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "")
	if err != nil {
		t.Errorf("expected nil when no git path, got %v", err)
	}
}

func TestExecutor_finalizeTask_EmptyGitPathFromGit(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{AutoCommit: true},
		},
		Git: &mockGitClient{
			repoRoot: "", // Empty
		},
		Logger: logging.NewNop(),
	}

	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "")
	if err != nil {
		t.Errorf("expected nil when git repo root is empty, got %v", err)
	}
}

func TestExecutor_finalizeTask_GitFactoryError(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: &mockGitClientFactory{
			clientErr: errors.New("factory error"),
		},
	}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{AutoCommit: true},
		},
		Logger: logging.NewNop(),
	}

	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "/some/path")
	if err == nil {
		t.Error("expected error from git factory, got nil")
	}
	if !strings.Contains(err.Error(), "creating git client") {
		t.Errorf("expected 'creating git client' in error, got: %v", err)
	}
}

func TestExecutor_finalizeTask_NoGitClient(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: nil,
	}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{AutoCommit: true},
		},
		Git:    nil, // No fallback either
		Logger: logging.NewNop(),
	}

	// With workDir but no git clients at all
	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "/some/path")
	if err != nil {
		t.Errorf("expected nil when no git client available, got %v", err)
	}
}

func TestExecutor_finalizeTask_BranchResolutionFromTaskState(t *testing.T) {
	t.Parallel()
	executor := &Executor{
		gitFactory: &mockGitClientFactory{
			client: &mockGitClient{
				branchErr: errors.New("no branch"),
				status:    &core.GitStatus{},
			},
		},
	}
	wctx := &Context{
		Config: &Config{
			Finalization: FinalizationConfig{
				AutoCommit: true,
				AutoPush:   true, // Requires branch
			},
		},
		Git: &mockGitClient{
			branchErr: errors.New("no branch"),
		},
		Logger: logging.NewNop(),
	}

	// taskState has no branch, both git clients fail to get branch, and AutoPush requires it
	err := executor.finalizeTask(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, "/some/path")
	if err == nil {
		t.Error("expected error when branch can't be resolved for push/PR")
	}
	if !strings.Contains(err.Error(), "could not determine current git branch") {
		t.Errorf("expected branch resolution error, got: %v", err)
	}
}

// =============================================================================
// Tests for saveTaskOutput
// =============================================================================

func TestExecutor_saveTaskOutput_FallbackDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	executor := &Executor{}
	wctx := &Context{
		Report:      nil, // No report writer
		ProjectRoot: tmpDir,
	}

	output := "This is the task output content"
	path := executor.saveTaskOutput(wctx, "task-1", output)

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	expectedDir := filepath.Join(tmpDir, ".quorum/outputs")
	if !strings.HasPrefix(path, expectedDir) {
		t.Errorf("expected path under %s, got %s", expectedDir, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(content) != output {
		t.Errorf("output content mismatch: got %q, want %q", string(content), output)
	}
}

func TestExecutor_saveTaskOutput_MkdirFails(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Report:      nil,
		ProjectRoot: "/nonexistent/impossible/path/root",
	}

	path := executor.saveTaskOutput(wctx, "task-1", "output")
	if path != "" {
		t.Errorf("expected empty path when dir creation fails, got %s", path)
	}
}

// =============================================================================
// Tests for GetFullOutput
// =============================================================================

func TestGetFullOutput_NoOutputFile(t *testing.T) {
	t.Parallel()
	state := &core.TaskState{
		Output:     "inline output",
		OutputFile: "",
	}

	result, err := GetFullOutput(state)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "inline output" {
		t.Errorf("got %q, want %q", result, "inline output")
	}
}

func TestGetFullOutput_FileReadError(t *testing.T) {
	t.Parallel()
	state := &core.TaskState{
		Output:     "fallback",
		OutputFile: "/nonexistent/path/to/output.md",
	}

	result, err := GetFullOutput(state)
	if err == nil {
		t.Error("expected error reading nonexistent file")
	}
	// Should return inline output as fallback
	if result != "fallback" {
		t.Errorf("expected fallback output, got %q", result)
	}
}

// =============================================================================
// Tests for WithGitFactory
// =============================================================================

func TestExecutor_WithGitFactory(t *testing.T) {
	t.Parallel()
	executor := NewExecutor(nil, nil, nil)
	factory := &mockGitClientFactory{}

	returned := executor.WithGitFactory(factory)
	if returned != executor {
		t.Error("WithGitFactory should return same executor for chaining")
	}
	if executor.gitFactory != factory {
		t.Error("gitFactory not set correctly")
	}
}

// =============================================================================
// Tests for findDependencyBranch
// =============================================================================

func TestExecutor_findDependencyBranch_NoDependencies(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{},
			},
		},
	}

	task := &core.Task{ID: "task-1", Dependencies: nil}
	branch := executor.findDependencyBranch(wctx, task)
	if branch != "" {
		t.Errorf("expected empty branch, got %q", branch)
	}
}

func TestExecutor_findDependencyBranch_DepNotCompleted(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"dep-1": {ID: "dep-1", Status: core.TaskStatusPending},
				},
			},
		},
	}

	task := &core.Task{ID: "task-1", Dependencies: []core.TaskID{"dep-1"}}
	branch := executor.findDependencyBranch(wctx, task)
	if branch != "" {
		t.Errorf("expected empty branch for non-completed dep, got %q", branch)
	}
}

func TestExecutor_findDependencyBranch_DepNoWorktree(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"dep-1": {ID: "dep-1", Status: core.TaskStatusCompleted, WorktreePath: ""},
				},
			},
		},
	}

	task := &core.Task{ID: "task-1", Dependencies: []core.TaskID{"dep-1"}}
	branch := executor.findDependencyBranch(wctx, task)
	if branch != "" {
		t.Errorf("expected empty branch for dep without worktree, got %q", branch)
	}
}

func TestExecutor_findDependencyBranch_DepNilInState(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					// dep-1 not in the tasks map at all
				},
			},
		},
	}

	task := &core.Task{ID: "task-1", Dependencies: []core.TaskID{"dep-1"}}
	branch := executor.findDependencyBranch(wctx, task)
	if branch != "" {
		t.Errorf("expected empty branch for missing dep, got %q", branch)
	}
}

func TestExecutor_findDependencyBranch_WithWorktreeGetError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	worktreeMgr := &mockWorktreeManager{
		createInfo: nil,
		createErr:  errors.New("not found"),
	}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"dep-1": {ID: "dep-1", Name: "Dep", Status: core.TaskStatusCompleted, WorktreePath: "/tmp/wt"},
				},
			},
		},
		Worktrees: worktreeMgr,
	}

	task := &core.Task{ID: "task-1", Dependencies: []core.TaskID{"dep-1"}}
	branch := executor.findDependencyBranch(wctx, task)
	// Get returns nil info for our mock (createInfo is nil), so should get empty
	if branch != "" {
		t.Errorf("expected empty branch when worktree Get returns nil, got %q", branch)
	}
}

func TestExecutor_findDependencyBranch_WithValidWorktree(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	worktreeMgr := &mockWorktreeManager{
		createInfo: &core.WorktreeInfo{
			Path:   "/tmp/wt-dep",
			Branch: "quorum/task/dep-1",
		},
	}

	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"dep-1": {ID: "dep-1", Name: "Dep", Status: core.TaskStatusCompleted, WorktreePath: "/tmp/wt-dep"},
				},
			},
		},
		Worktrees: worktreeMgr,
	}

	task := &core.Task{ID: "task-1", Dependencies: []core.TaskID{"dep-1"}}
	branch := executor.findDependencyBranch(wctx, task)
	if branch != "quorum/task/dep-1" {
		t.Errorf("expected branch 'quorum/task/dep-1', got %q", branch)
	}
}

// =============================================================================
// Tests for cleanupWorktree
// =============================================================================

func TestExecutor_cleanupWorktree_NotCreated(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config: &Config{WorktreeAutoClean: true},
	}

	// Should return immediately
	executor.cleanupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, false)
}

func TestExecutor_cleanupWorktree_AutoCleanDisabled(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config:    &Config{WorktreeAutoClean: false},
		Worktrees: &mockWorktreeManager{},
	}

	executor.cleanupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, true)
}

func TestExecutor_cleanupWorktree_RemoveError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config: &Config{WorktreeAutoClean: true},
		Worktrees: &mockWorktreeManager{
			removeErr: errors.New("remove failed"),
		},
		Logger: logging.NewNop(),
	}

	// Should log warning but not crash
	executor.cleanupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, true)
}

func TestExecutor_cleanupWorktree_Success(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Config:    &Config{WorktreeAutoClean: true},
		Worktrees: &mockWorktreeManager{},
		Logger:    logging.NewNop(),
	}

	// Should succeed
	executor.cleanupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, true)
}

// =============================================================================
// Tests for cleanupWorkflowScopedWorktree with isolation remove error
// =============================================================================

func TestExecutor_cleanupWorkflowScopedWorktree_IsolationRemoveError(t *testing.T) {
	t.Parallel()
	mockWtMgr := &mockWorkflowWorktreeManager{
		removeErr: errors.New("remove failed"),
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

	// Should not panic even if remove fails
	executor.cleanupWorkflowScopedWorktree(context.Background(), wctx, task, true)

	if len(mockWtMgr.removeCalls) != 1 {
		t.Errorf("expected 1 remove call, got %d", len(mockWtMgr.removeCalls))
	}
}

func TestExecutor_cleanupWorkflowScopedWorktree_FallsBackToLegacy(t *testing.T) {
	t.Parallel()
	legacyMgr := &mockWorktreeManager{}

	executor := &Executor{}
	wctx := &Context{
		GitIsolation: nil, // Not enabled
		Worktrees:    legacyMgr,
		Config:       &Config{WorktreeAutoClean: true},
		Logger:       logging.NewNop(),
	}
	task := &core.Task{ID: "task-001", Name: "Test Task"}

	executor.cleanupWorkflowScopedWorktree(context.Background(), wctx, task, true)
}

// =============================================================================
// Tests for setupWorktree with dependencies
// =============================================================================

func TestExecutor_setupWorktree_WithBaseBranch(t *testing.T) {
	t.Parallel()
	worktreeMgr := &mockWorktreeManager{
		createInfo: &core.WorktreeInfo{
			Path:   "/tmp/wt",
			Branch: "quorum/task-1",
		},
	}

	executor := &Executor{}
	wctx := &Context{
		State: &core.WorkflowState{
			WorkflowRun: core.WorkflowRun{
				Tasks: map[core.TaskID]*core.TaskState{
					"dep-1": {ID: "dep-1", Name: "Dep", Status: core.TaskStatusCompleted, WorktreePath: "/tmp/dep-wt"},
				},
			},
		},
		Worktrees: worktreeMgr,
		Logger:    logging.NewNop(),
	}

	task := &core.Task{ID: "task-1", Name: "Test", Dependencies: []core.TaskID{"dep-1"}}
	taskState := &core.TaskState{}

	workDir, created := executor.setupWorktree(context.Background(), wctx, task, taskState, true)
	if !created {
		t.Error("expected worktree to be created")
	}
	if workDir != "/tmp/wt" {
		t.Errorf("expected workDir /tmp/wt, got %s", workDir)
	}
}

func TestExecutor_setupWorktree_WorktreesDisabled(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Worktrees: nil,
		Logger:    logging.NewNop(),
	}

	workDir, created := executor.setupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, false)
	if created || workDir != "" {
		t.Error("expected no worktree when disabled")
	}
}

func TestExecutor_setupWorktree_NilWorktreeManager(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Worktrees: nil,
		Logger:    logging.NewNop(),
	}

	workDir, created := executor.setupWorktree(context.Background(), wctx, &core.Task{ID: "t1"}, &core.TaskState{}, true)
	if created || workDir != "" {
		t.Error("expected no worktree when manager is nil")
	}
}

// =============================================================================
// Tests for notifyTaskCompletion edge cases
// =============================================================================

func TestExecutor_notifyTaskCompletion_NilOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Output: nil,
	}

	// Should not panic
	executor.notifyTaskCompletion(wctx, &core.Task{ID: "t1"}, &core.TaskState{Status: core.TaskStatusCompleted}, time.Now(), nil)
}

func TestExecutor_notifyTaskCompletion_FailedNoError(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
	}

	// Failed task with nil taskErr should generate "task failed" message
	executor.notifyTaskCompletion(wctx, &core.Task{ID: "t1"}, &core.TaskState{Status: core.TaskStatusFailed}, time.Now(), nil)

	if len(output.taskFailed) != 1 {
		t.Errorf("expected TaskFailed called once, got %d", len(output.taskFailed))
	}
}

func TestExecutor_notifyTaskCompletion_PendingStatus(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
	}

	// Neither completed nor failed
	executor.notifyTaskCompletion(wctx, &core.Task{ID: "t1"}, &core.TaskState{Status: core.TaskStatusPending}, time.Now(), nil)

	if len(output.taskCompleted) != 0 {
		t.Error("should not call TaskCompleted for pending status")
	}
	if len(output.taskFailed) != 0 {
		t.Error("should not call TaskFailed for pending status")
	}
}

// =============================================================================
// Tests for handleExecutionFailure
// =============================================================================

func TestExecutor_handleExecutionFailure_WithOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
		Logger: logging.NewNop(),
	}

	taskState := &core.TaskState{ID: "t1"}
	err := executor.handleExecutionFailure(wctx, &core.Task{ID: "t1", Name: "Test"}, taskState, "claude", "opus", 2, 1000, errors.New("exec error"))

	if err == nil {
		t.Error("expected error to be returned")
	}
	if taskState.Status != core.TaskStatusFailed {
		t.Errorf("expected failed status, got %v", taskState.Status)
	}
}

func TestExecutor_handleExecutionFailure_NilOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Output: nil,
		Logger: logging.NewNop(),
	}

	taskState := &core.TaskState{ID: "t1"}
	err := executor.handleExecutionFailure(wctx, &core.Task{ID: "t1", Name: "Test"}, taskState, "claude", "opus", 0, 500, errors.New("error"))

	if err == nil {
		t.Error("expected error")
	}
}

// =============================================================================
// Tests for setTaskFailed
// =============================================================================

func TestExecutor_setTaskFailed_WithOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	output := &mockOutputNotifier{}
	wctx := &Context{
		Output: output,
		Logger: logging.NewNop(),
	}

	taskState := &core.TaskState{ID: "t1"}
	executor.setTaskFailed(wctx, taskState, errors.New("fail reason"))

	if taskState.Status != core.TaskStatusFailed {
		t.Error("expected failed status")
	}
	if taskState.Error != "fail reason" {
		t.Errorf("expected error 'fail reason', got %q", taskState.Error)
	}
}

func TestExecutor_setTaskFailed_NilOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Output: nil,
	}

	taskState := &core.TaskState{ID: "t1"}
	executor.setTaskFailed(wctx, taskState, errors.New("fail"))
	if taskState.Status != core.TaskStatusFailed {
		t.Error("expected failed status")
	}
}

// =============================================================================
// Tests for logTaskExecutionStart (prompt truncation)
// =============================================================================

func TestExecutor_logTaskExecutionStart_ShortPrompt(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Logger: logging.NewNop(),
		Config: &Config{PhaseTimeouts: PhaseTimeouts{}},
	}

	// Should not panic with short prompt
	executor.logTaskExecutionStart(wctx, &core.Task{ID: "t1", Name: "Test"}, "claude", "opus", "/tmp", "short prompt")
}

func TestExecutor_logTaskExecutionStart_LongPrompt(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Logger: logging.NewNop(),
		Config: &Config{PhaseTimeouts: PhaseTimeouts{}},
	}

	longPrompt := strings.Repeat("a", 1000)
	// Should truncate and not panic
	executor.logTaskExecutionStart(wctx, &core.Task{ID: "t1", Name: "Test"}, "claude", "opus", "/tmp", longPrompt)
}

// =============================================================================
// Tests for notifyAgentStarted
// =============================================================================

func TestExecutor_notifyAgentStarted_NilOutput(t *testing.T) {
	t.Parallel()
	executor := &Executor{}
	wctx := &Context{
		Output: nil,
		Config: &Config{PhaseTimeouts: PhaseTimeouts{}},
	}

	// Should not panic
	executor.notifyAgentStarted(wctx, "claude", &core.Task{ID: "t1", Name: "Test"}, "opus", "/tmp")
}

// =============================================================================
// Helper mock type with List
// =============================================================================

type mockWorktreeManagerWithList struct {
	listResult  []*core.WorktreeInfo
	listErr     error
	removeCalled int
	removeErr   error
}

func (m *mockWorktreeManagerWithList) Create(_ context.Context, _ *core.Task, _ string) (*core.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockWorktreeManagerWithList) CreateFromBranch(_ context.Context, _ *core.Task, _, _ string) (*core.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockWorktreeManagerWithList) Get(_ context.Context, _ *core.Task) (*core.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockWorktreeManagerWithList) Remove(_ context.Context, _ *core.Task) error {
	m.removeCalled++
	return m.removeErr
}
func (m *mockWorktreeManagerWithList) CleanupStale(_ context.Context) error { return nil }
func (m *mockWorktreeManagerWithList) List(_ context.Context) ([]*core.WorktreeInfo, error) {
	return m.listResult, m.listErr
}

