package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// =============================================================================
// WorkflowWorktreeManager additional coverage
// =============================================================================

func TestNewWorkflowWorktreeManager_RelativeBaseDir(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Relative base dir should be resolved relative to repoPath
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, ".worktrees", client, nil)
	testutil.AssertNoError(t, err)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}

	// Verify the directory was created
	expectedDir := filepath.Join(repo.Path, ".worktrees")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to exist", expectedDir)
	}
}

func TestNewWorkflowWorktreeManager_AbsoluteBaseDir(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	absDir := t.TempDir()
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, absDir, client, nil)
	testutil.AssertNoError(t, err)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

// =============================================================================
// InitializeWorkflow additional paths
// =============================================================================

func TestInitializeWorkflow_DefaultBaseBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Empty baseBranch should default to detected default branch
	info, err := mgr.InitializeWorkflow(context.Background(), "wf-default-base", "")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, info.BaseBranch, "main")
	testutil.AssertEqual(t, info.WorkflowBranch, "quorum/wf-default-base")
}

// =============================================================================
// MergeTaskToWorkflow with different strategies
// =============================================================================

func TestMergeTaskToWorkflow_Parallel(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := uniqueWorkflowID("wf-parallel")

	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	task := &core.Task{ID: "task-parallel", Description: "Parallel task"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Make changes in task worktree
	if err := os.WriteFile(filepath.Join(wtInfo.Path, "parallel.txt"), []byte("parallel content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	taskClient, err := git.NewClient(wtInfo.Path)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, taskClient.Add(context.Background(), "parallel.txt"))
	_, err = taskClient.Commit(context.Background(), "Add parallel file")
	testutil.AssertNoError(t, err)

	// Merge with parallel strategy
	err = mgr.MergeTaskToWorkflow(context.Background(), workflowID, task.ID, "parallel")
	testutil.AssertNoError(t, err)
}

func TestMergeTaskToWorkflow_Rebase(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := uniqueWorkflowID("wf-rebase")

	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	task := &core.Task{ID: "task-rebase", Description: "Rebase task"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Make changes
	if err := os.WriteFile(filepath.Join(wtInfo.Path, "rebase.txt"), []byte("rebase content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	taskClient, err := git.NewClient(wtInfo.Path)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, taskClient.Add(context.Background(), "rebase.txt"))
	_, err = taskClient.Commit(context.Background(), "Add rebase file")
	testutil.AssertNoError(t, err)

	// Merge with rebase strategy
	err = mgr.MergeTaskToWorkflow(context.Background(), workflowID, task.ID, "rebase")
	testutil.AssertNoError(t, err)
}

// =============================================================================
// MergeAllTasksToWorkflow with errors
// =============================================================================

func TestMergeAllTasksToWorkflow_PartialFailure(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := uniqueWorkflowID("wf-partial-fail")

	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Create one valid task with changes
	task1 := &core.Task{ID: "task-ok", Description: "Valid task"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task1)
	testutil.AssertNoError(t, err)

	if err := os.WriteFile(filepath.Join(wtInfo.Path, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tc, err := git.NewClient(wtInfo.Path)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, tc.Add(context.Background(), "ok.txt"))
	_, err = tc.Commit(context.Background(), "ok commit")
	testutil.AssertNoError(t, err)

	// Include a nonexistent task ID to trigger an error
	taskIDs := []core.TaskID{task1.ID, "nonexistent-task"}

	err = mgr.MergeAllTasksToWorkflow(context.Background(), workflowID, taskIDs, "sequential")
	// Should report errors but not panic
	testutil.AssertError(t, err)
}

// =============================================================================
// FinalizeWorkflow with merge
// =============================================================================

func TestFinalizeWorkflow_WithMerge(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := uniqueWorkflowID("wf-finalize-merge")

	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Create a task and make changes
	task := &core.Task{ID: "task-fin", Description: "Finalize task"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	if err := os.WriteFile(filepath.Join(wtInfo.Path, "finalize.txt"), []byte("finalize"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	tc, err := git.NewClient(wtInfo.Path)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, tc.Add(context.Background(), "finalize.txt"))
	_, err = tc.Commit(context.Background(), "finalize commit")
	testutil.AssertNoError(t, err)

	// Merge task to workflow first
	err = mgr.MergeTaskToWorkflow(context.Background(), workflowID, task.ID, "sequential")
	testutil.AssertNoError(t, err)

	// Detach HEAD in the main repo so that FinalizeWorkflow can create a
	// worktree for the "main" branch (git forbids two worktrees pointing to
	// the same branch).
	_, detachErr := repo.Run("checkout", "--detach", "HEAD")
	if detachErr != nil {
		t.Fatalf("detach HEAD: %v", detachErr)
	}

	// Finalize with merge to base
	err = mgr.FinalizeWorkflow(context.Background(), workflowID, true)
	testutil.AssertNoError(t, err)

	// Switch back to main and verify the merge landed
	repo.Checkout("main")
	mainClient, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)
	branch, _ := mainClient.CurrentBranch(context.Background())
	testutil.AssertEqual(t, branch, "main")
}

// =============================================================================
// RemoveTaskWorktree edge cases
// =============================================================================

func TestRemoveTaskWorktree_NonexistentWorkflow(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Should not error for nonexistent workflow root (already cleaned up)
	err = mgr.RemoveTaskWorktree(context.Background(), "nonexistent-wf", "task-id", false)
	testutil.AssertNoError(t, err)
}

// =============================================================================
// GetWorkflowStatus additional tests
// =============================================================================

func TestGetWorkflowStatus_WithAheadBehind(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := uniqueWorkflowID("wf-status-ahead")

	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	status, err := mgr.GetWorkflowStatus(context.Background(), workflowID)
	testutil.AssertNoError(t, err)

	if status == nil {
		t.Fatal("status should not be nil")
	}
	// Initially no ahead/behind
	if status.AheadOfBase != 0 {
		t.Errorf("expected 0 ahead, got %d", status.AheadOfBase)
	}
}

// =============================================================================
// ListActiveWorkflows edge cases
// =============================================================================

func TestListActiveWorkflows_NonexistentBaseDir(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	nonexistentDir := filepath.Join(t.TempDir(), "does-not-exist")
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, nonexistentDir, client, nil)
	testutil.AssertNoError(t, err)

	// Remove the base dir to simulate nonexistent
	if err := os.RemoveAll(nonexistentDir); err != nil {
		t.Fatalf("removeall: %v", err)
	}

	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(workflows))
	}
}

func TestListActiveWorkflows_SkipsNonWorkflowDirs(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Create a directory that doesn't have a corresponding branch
	if err := os.MkdirAll(filepath.Join(worktreeDir, "random-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	// random-dir should be skipped because it has no matching branch
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(workflows))
	}
}

func TestListActiveWorkflows_IncludesFiles(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Create a file (not a directory) in the base dir
	if err := os.WriteFile(filepath.Join(worktreeDir, "not-a-dir"), []byte("file"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	// File entries should be skipped
	if len(workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(workflows))
	}
}

// =============================================================================
// CreateTaskWorktree reuse scenario
// =============================================================================

func TestCreateTaskWorktree_ReusesExisting(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := "wf-reuse-wt"
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	task := &core.Task{ID: "task-reuse", Description: "Reuse test"}
	wt1, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Calling again should reuse the existing worktree
	wt2, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wt1.Path, wt2.Path)
}

// =============================================================================
// SanitizeForPath edge cases (tested through getTaskWorktreePath)
// =============================================================================

func TestCreateTaskWorktree_LongDescription(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := "wf-long-desc"
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Description longer than 30 chars (truncated by sanitizeForPath)
	task := &core.Task{
		ID:          "task-long",
		Description: "This is a very long description that should be truncated by the path sanitizer",
	}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	base := filepath.Base(wtInfo.Path)
	// Should start with task ID
	if len(base) < 9 || base[:9] != "task-long" {
		t.Errorf("path should start with task ID, got %q", base)
	}
}

func TestCreateTaskWorktree_SpecialCharsDescription(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	workflowID := "wf-special-chars"
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Description with special characters
	task := &core.Task{
		ID:          "task-special2",
		Description: "Feature: Add User/Auth \\Support! @#$%",
	}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Path should not contain special characters
	base := filepath.Base(wtInfo.Path)
	if len(base) < 13 || base[:13] != "task-special2" {
		t.Errorf("path should start with task ID, got %q", base)
	}
}

// =============================================================================
// CleanupWorkflow edge cases
// =============================================================================

func TestCleanupWorkflow_NoWorktreeRoot(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Cleanup a workflow that was never initialized (no worktree root)
	err = mgr.CleanupWorkflow(context.Background(), "nonexistent-wf", true)
	testutil.AssertNoError(t, err)
}

// =============================================================================
// WorktreeManager additional coverage
// =============================================================================

func TestWorktreeManager_Lock_Unlock(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree
	wt, err := manager.Create(context.Background(), "locktest", "lock-branch")
	testutil.AssertNoError(t, err)

	// Lock it
	err = manager.Lock(context.Background(), wt.Path, "test reason")
	testutil.AssertNoError(t, err)

	// Unlock it
	err = manager.Unlock(context.Background(), wt.Path)
	testutil.AssertNoError(t, err)
}

func TestWorktreeManager_LockNoReason(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	wt, err := manager.Create(context.Background(), "locknoreason", "locknr-branch")
	testutil.AssertNoError(t, err)

	// Lock without reason
	err = manager.Lock(context.Background(), wt.Path, "")
	testutil.AssertNoError(t, err)

	err = manager.Unlock(context.Background(), wt.Path)
	testutil.AssertNoError(t, err)
}

func TestWorktreeManager_Prune(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Prune (nothing to prune)
	pruned, err := manager.Prune(context.Background(), false)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, pruned, 0)

	// Dry-run prune
	pruned, err = manager.Prune(context.Background(), true)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, pruned, 0)
}

func TestWorktreeManager_WithPrefix(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir).WithPrefix("custom-")

	wt, err := manager.Create(context.Background(), "test", "prefix-branch")
	testutil.AssertNoError(t, err)

	// Path should use custom prefix
	base := filepath.Base(wt.Path)
	if base != "custom-test" {
		t.Errorf("expected path base %q, got %q", "custom-test", base)
	}
}

func TestWorktreeManager_DefaultBaseDir(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Empty baseDir should default to .worktrees
	manager := git.NewWorktreeManager(client, "")
	expected := filepath.Join(repo.Path, ".worktrees")
	testutil.AssertEqual(t, manager.BaseDir(), expected)
}

func TestWorktreeManager_CreateFromBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create a base branch with changes
	err = client.CreateBranch(context.Background(), "base-branch", "")
	testutil.AssertNoError(t, err)
	repo.WriteFile("base.txt", "base content")
	repo.Commit("Base commit")
	err = client.CheckoutBranch(context.Background(), "main")
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree from a specific base branch
	wt, err := manager.CreateFromBranch(context.Background(), "derived", "derived-branch", "base-branch")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wt.Branch, "derived-branch")

	// Verify the worktree has the base content
	_, statErr := os.Stat(filepath.Join(wt.Path, "base.txt"))
	if os.IsNotExist(statErr) {
		t.Error("expected base.txt to exist in derived worktree")
	}
}

func TestWorktreeManager_RemoveForce(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	wt, err := manager.Create(context.Background(), "force-rm", "force-rm-branch")
	testutil.AssertNoError(t, err)

	// Add uncommitted changes to make it dirty
	if err := os.WriteFile(filepath.Join(wt.Path, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Force remove
	err = manager.Remove(context.Background(), wt.Path, true)
	testutil.AssertNoError(t, err)
}

func TestWorktreeManager_CleanupStale(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create a worktree
	_, err = manager.Create(context.Background(), "stale-test", "stale-branch")
	testutil.AssertNoError(t, err)

	// Cleanup with 0 maxAge should not remove (as files are brand new)
	cleaned, err := manager.CleanupStale(context.Background(), 24*60*60*1e9) // 24h
	testutil.AssertNoError(t, err)
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned (too new), got %d", cleaned)
	}
}

// =============================================================================
// TaskWorktreeManager additional coverage
// =============================================================================

func TestTaskWorktreeManager_Manager(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	inner := manager.Manager()
	if inner == nil {
		t.Error("Manager() should not return nil")
	}
}

func TestTaskWorktreeManager_CleanupStale(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	err = manager.CleanupStale(context.Background())
	testutil.AssertNoError(t, err)
}

func TestTaskWorktreeManager_CreateFromBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create a source branch
	err = client.CreateBranch(context.Background(), "source-branch", "")
	testutil.AssertNoError(t, err)
	repo.WriteFile("source.txt", "source content")
	repo.Commit("Source commit")
	err = client.CheckoutBranch(context.Background(), "main")
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	task := &core.Task{ID: "task-from-branch", Name: "From branch task"}
	info, err := manager.CreateFromBranch(context.Background(), task, "derived-task-branch", "source-branch")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, info.Branch, "derived-task-branch")
}
