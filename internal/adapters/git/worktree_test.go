package git_test

import (
	"context"
	"os"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestWorktreeManager_Create(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree with new branch
	wt, err := manager.Create(context.Background(), "test-task", "task-branch")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wt.Branch, "task-branch")

	// Verify directory exists
	_, err = os.Stat(wt.Path)
	testutil.AssertNoError(t, err)

	// Verify it's a different path
	if wt.Path == repo.Path {
		t.Fatal("worktree path should be different from main repo")
	}
}

func TestWorktreeManager_CreateExisting(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create first worktree
	_, err = manager.Create(context.Background(), "test", "branch1")
	testutil.AssertNoError(t, err)

	// Try to create with same name
	_, err = manager.Create(context.Background(), "test", "branch2")
	testutil.AssertError(t, err)
}

func TestWorktreeManager_List(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Initially just the main worktree
	worktrees, err := manager.List(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(worktrees), 1)

	// Add worktrees
	manager.Create(context.Background(), "wt1", "branch1")
	manager.Create(context.Background(), "wt2", "branch2")

	worktrees, err = manager.List(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(worktrees), 3)
}

func TestWorktreeManager_Remove(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree
	wt, err := manager.Create(context.Background(), "test", "test-branch")
	testutil.AssertNoError(t, err)

	// Remove it
	err = manager.Remove(context.Background(), wt.Path, false)
	testutil.AssertNoError(t, err)

	// Verify removed
	_, err = os.Stat(wt.Path)
	if !os.IsNotExist(err) {
		t.Fatal("worktree directory should be removed")
	}
}

func TestWorktreeManager_Get(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree
	created, err := manager.Create(context.Background(), "test", "test-branch")
	testutil.AssertNoError(t, err)

	// Get it
	wt, err := manager.Get(context.Background(), "test")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, wt.Path, created.Path)
	testutil.AssertEqual(t, wt.Branch, "test-branch")
}

func TestWorktreeManager_GetNotFound(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	_, err = manager.Get(context.Background(), "nonexistent")
	testutil.AssertError(t, err)
}

func TestWorktreeManager_ListManaged(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create managed worktree
	manager.Create(context.Background(), "managed", "managed-branch")

	// List managed (should only include ones in our base dir)
	managed, err := manager.ListManaged(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(managed), 1)
	testutil.AssertEqual(t, managed[0].Branch, "managed-branch")
}

func TestWorktreeManager_CreateFromCommit(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	commit := repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create detached worktree
	wt, err := manager.CreateFromCommit(context.Background(), "detached", commit[:10])
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, wt.Detached, "should be detached")
}

func TestWorktreeManager_BaseDir(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	testutil.AssertEqual(t, manager.BaseDir(), worktreeDir)
}

func TestWorktreeManager_RemoveInvalidPath(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Try to remove a path outside our base dir
	err = manager.Remove(context.Background(), "/tmp/random", false)
	testutil.AssertError(t, err)
}

func TestWorktreeManager_CreateClient(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	// Create worktree
	wt, err := manager.Create(context.Background(), "test", "test-branch")
	testutil.AssertNoError(t, err)

	// Create client for worktree
	wtClient, err := manager.CreateClient(wt.Path)
	testutil.AssertNoError(t, err)

	// Verify it works
	branch, err := wtClient.CurrentBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "test-branch")
}
