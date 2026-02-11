package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// =============================================================================
// Client constructor and basic methods
// =============================================================================

func TestGitClient_RepoRoot(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	root, err := client.RepoRoot(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, root, repo.Path)
}

func TestGitClient_WithTimeout(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	client2 := client.WithTimeout(5 * time.Second)
	if client2 != client {
		t.Error("WithTimeout should return the same client")
	}

	// Verify it still works
	_, err = client.CurrentBranch(context.Background())
	testutil.AssertNoError(t, err)
}

// =============================================================================
// StatusLocal tests
// =============================================================================

func TestGitClient_StatusLocal(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	status, err := client.StatusLocal(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, status.IsClean(), "should be clean")
	testutil.AssertEqual(t, status.Branch, "main")

	// Add an untracked file
	repo.WriteFile("new.txt", "content")
	status, err = client.StatusLocal(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, status.IsClean(), "should not be clean with untracked file")
	testutil.AssertLen(t, status.Untracked, 1)
}

// =============================================================================
// Checkout and CheckoutBranch tests
// =============================================================================

func TestGitClient_CheckoutBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create a branch
	err = client.CreateBranch(context.Background(), "feature-x", "")
	testutil.AssertNoError(t, err)

	// Go back to main
	err = client.CheckoutBranch(context.Background(), "main")
	testutil.AssertNoError(t, err)

	branch, _ := client.CurrentBranch(context.Background())
	testutil.AssertEqual(t, branch, "main")

	// CheckoutBranch with invalid name
	err = client.CheckoutBranch(context.Background(), "-bad")
	testutil.AssertError(t, err)
}

// =============================================================================
// DeleteBranchForce test
// =============================================================================

func TestGitClient_DeleteBranchForce(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create a branch with unique content (diverged from main)
	err = client.CreateBranch(context.Background(), "force-delete", "")
	testutil.AssertNoError(t, err)

	repo.WriteFile("force.txt", "content")
	repo.Commit("Diverge")

	// Go back to main
	err = client.CheckoutBranch(context.Background(), "main")
	testutil.AssertNoError(t, err)

	// Force delete (since branch has unmerged changes)
	err = client.DeleteBranchForce(context.Background(), "force-delete")
	testutil.AssertNoError(t, err)

	exists, _ := client.BranchExists(context.Background(), "force-delete")
	testutil.AssertFalse(t, exists, "branch should be force-deleted")
}

func TestGitClient_DeleteBranchForce_InvalidName(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	err = client.DeleteBranchForce(context.Background(), "-bad")
	testutil.AssertError(t, err)
}

// =============================================================================
// Commit with validation
// =============================================================================

func TestGitClient_Commit_EmptyMessage(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	_, err = client.Commit(context.Background(), "")
	testutil.AssertError(t, err)
}

func TestGitClient_Commit_NulByte(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	_, err = client.Commit(context.Background(), "msg\x00bad")
	testutil.AssertError(t, err)
}

// =============================================================================
// Diff tests
// =============================================================================

func TestGitClient_Diff_EmptyRefs(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// No changes, empty diff
	diff, err := client.Diff(context.Background(), "", "")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, diff, "")
}

func TestGitClient_Diff_BetweenCommits(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	first := repo.Commit("First")
	repo.WriteFile("new.txt", "content")
	second := repo.Commit("Second")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	diff, err := client.Diff(context.Background(), first, second)
	testutil.AssertNoError(t, err)
	testutil.AssertContains(t, diff, "new.txt")
}

func TestGitClient_Diff_EmptyHead(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	first := repo.Commit("First")
	repo.WriteFile("new.txt", "content")
	repo.Commit("Second")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Empty head should default to HEAD
	diff, err := client.Diff(context.Background(), first, "")
	testutil.AssertNoError(t, err)
	testutil.AssertContains(t, diff, "new.txt")
}

func TestGitClient_DiffStaged(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// No staged changes
	diff, err := client.DiffStaged(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, diff, "")

	// Stage a change
	repo.WriteFile("README.md", "# Modified")
	err = client.Add(context.Background(), "README.md")
	testutil.AssertNoError(t, err)

	diff, err = client.DiffStaged(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertContains(t, diff, "Modified")
}

func TestGitClient_DiffFiles(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	first := repo.Commit("First")

	repo.WriteFile("a.txt", "a")
	repo.WriteFile("b.txt", "b")
	second := repo.Commit("Second")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	files, err := client.DiffFiles(context.Background(), first, second)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, files, 2)
}

// =============================================================================
// Stash tests
// =============================================================================

func TestGitClient_Stash(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Make a change
	repo.WriteFile("README.md", "# Modified")

	// Stash with message
	err = client.Stash(context.Background(), "test stash")
	testutil.AssertNoError(t, err)

	// Working directory should be clean now
	clean, err := client.IsClean(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, clean, "should be clean after stash")

	// Pop stash
	err = client.StashPop(context.Background())
	testutil.AssertNoError(t, err)

	// Should be dirty again
	clean, err = client.IsClean(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, clean, "should be dirty after stash pop")
}

func TestGitClient_StashEmpty(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Stash without message on clean repo
	err = client.Stash(context.Background(), "")
	// Git may error or succeed with "No local changes to save" - either is acceptable
	_ = err
}

// =============================================================================
// Clean tests
// =============================================================================

func TestGitClient_Clean(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create untracked files
	repo.WriteFile("untracked.txt", "content")
	if err := os.MkdirAll(filepath.Join(repo.Path, "untracked-dir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Path, "untracked-dir", "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Clean with force and directories
	err = client.Clean(context.Background(), true, true)
	testutil.AssertNoError(t, err)

	// Untracked files should be gone
	clean, err := client.IsClean(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, clean, "should be clean after clean -f -d")
}

// =============================================================================
// Reset tests
// =============================================================================

func TestGitClient_Reset(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Stage a change
	repo.WriteFile("README.md", "# Modified")
	err = client.Add(context.Background(), "README.md")
	testutil.AssertNoError(t, err)

	// Reset mixed (default)
	err = client.Reset(context.Background(), "mixed", "")
	testutil.AssertNoError(t, err)
}

// =============================================================================
// Fetch/Push/Pull validation tests
// =============================================================================

func TestGitClient_Fetch_InvalidRemote(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Empty remote
	testutil.AssertError(t, client.Fetch(context.Background(), ""))

	// Remote with dash prefix
	testutil.AssertError(t, client.Fetch(context.Background(), "-bad"))
}

func TestGitClient_Push_InvalidInputs(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Invalid remote
	testutil.AssertError(t, client.Push(context.Background(), "", "main"))
	// Invalid branch
	testutil.AssertError(t, client.Push(context.Background(), "origin", ""))
}

func TestGitClient_PushForce(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	remote := testutil.CreateBareRemote(t)
	repo.SetRemote("origin", remote)

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Push first
	testutil.AssertNoError(t, client.Push(context.Background(), "origin", "main"))

	// Force push
	testutil.AssertNoError(t, client.PushForce(context.Background(), "origin", "main"))
}

func TestGitClient_PushForce_InvalidInputs(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	testutil.AssertError(t, client.PushForce(context.Background(), "", "main"))
	testutil.AssertError(t, client.PushForce(context.Background(), "origin", ""))
}

func TestGitClient_Pull_InvalidInputs(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	testutil.AssertError(t, client.Pull(context.Background(), "", "main"))
	testutil.AssertError(t, client.Pull(context.Background(), "origin", ""))
}

// =============================================================================
// DefaultBranch fallback tests
// =============================================================================

func TestGitClient_DefaultBranch_MasterFallback(t *testing.T) {
	t.Parallel()
	dir := testutil.TempDir(t)

	// Create a repo with master instead of main
	repo := &testutil.GitRepo{Path: dir}
	cmd := func(args ...string) {
		_, err := repo.Run(args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	cmd("init")
	cmd("config", "user.email", "test@example.com")
	cmd("config", "user.name", "Test User")
	cmd("checkout", "-b", "master")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd("add", "-A")
	cmd("commit", "-m", "Initial commit")

	client, err := git.NewClient(dir)
	testutil.AssertNoError(t, err)

	branch, err := client.DefaultBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "master")
}

// =============================================================================
// RemoteURL / RemoteURLFor tests
// =============================================================================

func TestGitClient_RemoteURL(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	remote := testutil.CreateBareRemote(t)
	repo.SetRemote("origin", remote)

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	url, err := client.RemoteURL(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, url, remote)
}

func TestGitClient_RemoteURLFor(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	remote := testutil.CreateBareRemote(t)
	repo.SetRemote("upstream", remote)

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	url, err := client.RemoteURLFor(context.Background(), "upstream")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, url, remote)

	// Empty remote defaults to origin
	_, err = client.RemoteURLFor(context.Background(), "")
	// Should error since no origin remote
	testutil.AssertError(t, err)
}

// =============================================================================
// Worktree operations
// =============================================================================

func TestGitClient_CreateWorktree_InvalidBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	err = client.CreateWorktree(context.Background(), t.TempDir(), "")
	testutil.AssertError(t, err)
}

func TestGitClient_ListWorktrees(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktrees, err := client.ListWorktrees(context.Background())
	testutil.AssertNoError(t, err)
	// Should have at least the main worktree
	if len(worktrees) < 1 {
		t.Error("expected at least 1 worktree")
	}

	// Main worktree should be marked
	found := false
	for _, wt := range worktrees {
		if wt.IsMain {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find main worktree")
	}
}

func TestGitClient_CreateAndRemoveWorktree(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	wtDir := filepath.Join(t.TempDir(), "my-worktree")
	err = client.CreateWorktree(context.Background(), wtDir, "wt-branch")
	testutil.AssertNoError(t, err)

	// Verify directory exists
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}

	// Remove it
	err = client.RemoveWorktree(context.Background(), wtDir)
	testutil.AssertNoError(t, err)
}

func TestGitClient_CreateWorktree_ExistingBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create a branch first
	err = client.CreateBranch(context.Background(), "existing-branch", "")
	testutil.AssertNoError(t, err)
	err = client.CheckoutBranch(context.Background(), "main")
	testutil.AssertNoError(t, err)

	// Create worktree pointing to existing branch
	wtDir := filepath.Join(t.TempDir(), "existing-wt")
	err = client.CreateWorktree(context.Background(), wtDir, "existing-branch")
	testutil.AssertNoError(t, err)

	// Clean up
	_ = client.RemoveWorktree(context.Background(), wtDir)
}

// =============================================================================
// Merge with squash and no-commit options
// =============================================================================

func TestGitClient_Merge_Squash(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	repo.CreateBranch("feature-squash")
	repo.WriteFile("f1.txt", "content1")
	repo.Commit("Feature commit 1")
	repo.WriteFile("f2.txt", "content2")
	repo.Commit("Feature commit 2")

	repo.Checkout("main")

	opts := core.MergeOptions{Squash: true}
	err = client.Merge(context.Background(), "feature-squash", opts)
	testutil.AssertNoError(t, err)
}

func TestGitClient_Merge_NoCommit(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	repo.CreateBranch("feature-nocommit")
	repo.WriteFile("feature-nc.txt", "content")
	repo.Commit("Add feature")

	repo.Checkout("main")

	opts := core.MergeOptions{NoCommit: true}
	err = client.Merge(context.Background(), "feature-nocommit", opts)
	testutil.AssertNoError(t, err)
}

func TestGitClient_Merge_WithStrategy(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	repo.CreateBranch("feature-strategy")
	repo.WriteFile("feature-s.txt", "content")
	repo.Commit("Add feature")

	repo.Checkout("main")

	opts := core.MergeOptions{
		Strategy:       "recursive",
		StrategyOption: "theirs",
		NoFastForward:  true,
		Message:        "Custom merge message",
	}
	err = client.Merge(context.Background(), "feature-strategy", opts)
	testutil.AssertNoError(t, err)
}

func TestGitClient_Merge_BranchNotFound(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	err = client.Merge(context.Background(), "nonexistent-branch", git.DefaultMergeOptions())
	testutil.AssertError(t, err)
}

// =============================================================================
// HasMergeConflicts when no merge in progress
// =============================================================================

func TestGitClient_HasMergeConflicts_NoMerge(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	has, err := client.HasMergeConflicts(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, has, "should not have conflicts")
}

// =============================================================================
// GetConflictFiles when no conflicts
// =============================================================================

func TestGitClient_GetConflictFiles_NoConflicts(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	files, err := client.GetConflictFiles(context.Background())
	testutil.AssertNoError(t, err)
	if files != nil {
		t.Errorf("expected nil conflict files, got %v", files)
	}
}

// =============================================================================
// findGitDir tests (via HasRebaseInProgress/HasMergeConflicts)
// =============================================================================

func TestGitClient_findGitDir_Worktree(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	wtDir := filepath.Join(t.TempDir(), "wt-gitdir")
	err = client.CreateWorktree(context.Background(), wtDir, "gitdir-branch")
	testutil.AssertNoError(t, err)
	defer func() { _ = client.RemoveWorktree(context.Background(), wtDir) }()

	// Create client for worktree and check rebase/merge status
	wtClient, err := git.NewClient(wtDir)
	testutil.AssertNoError(t, err)

	// These use findGitDir internally
	inProgress, err := wtClient.HasRebaseInProgress(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, inProgress, "no rebase should be in progress")

	has, err := wtClient.HasMergeConflicts(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, has, "no conflicts should exist")
}

// =============================================================================
// Add with validation
// =============================================================================

func TestGitClient_Add_EmptyPath(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	err = client.Add(context.Background(), "")
	testutil.AssertError(t, err)
}

func TestGitClient_Add_MultiplePaths(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	repo.WriteFile("a.txt", "a")
	repo.WriteFile("b.txt", "b")

	err = client.Add(context.Background(), "a.txt", "b.txt")
	testutil.AssertNoError(t, err)
}

// =============================================================================
// HasUncommittedChanges with staged changes
// =============================================================================

func TestGitClient_HasUncommittedChanges_Staged(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Stage a change
	repo.WriteFile("README.md", "# Modified")
	err = client.Add(context.Background(), "README.md")
	testutil.AssertNoError(t, err)

	hasChanges, err := client.HasUncommittedChanges(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, hasChanges, "should detect staged changes")
}

// =============================================================================
// CreateBranch with base validation
// =============================================================================

func TestGitClient_CreateBranch_InvalidName(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Invalid branch name
	err = client.CreateBranch(context.Background(), "-bad", "")
	testutil.AssertError(t, err)

	// Invalid base
	err = client.CreateBranch(context.Background(), "good-name", "-bad")
	testutil.AssertError(t, err)
}

// =============================================================================
// DefaultMergeOptions test
// =============================================================================

func TestDefaultMergeOptions(t *testing.T) {
	t.Parallel()
	opts := git.DefaultMergeOptions()
	if opts.Strategy != "recursive" {
		t.Errorf("Strategy = %q, want %q", opts.Strategy, "recursive")
	}
	if opts.NoFastForward {
		t.Error("NoFastForward should be false by default")
	}
}

// =============================================================================
// ContinueRebase test
// =============================================================================

func TestGitClient_ContinueRebase_NoRebase(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// ContinueRebase when no rebase is in progress should error
	err = client.ContinueRebase(context.Background())
	testutil.AssertError(t, err)
}

// =============================================================================
// Log with empty repo
// =============================================================================

func TestGitClient_Log_WithDateParsing(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	commits, err := client.Log(context.Background(), 1)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, commits, 1)

	if commits[0].Date.IsZero() {
		t.Error("commit date should not be zero")
	}
}

// =============================================================================
// Validation edge cases
// =============================================================================

func TestGitClient_BranchValidation_EdgeCases(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Branch with @{ forbidden
	err = client.CreateBranch(context.Background(), "bad@{branch", "")
	testutil.AssertError(t, err)

	// Branch with // forbidden
	err = client.CreateBranch(context.Background(), "bad//branch", "")
	testutil.AssertError(t, err)

	// Branch ending with /
	err = client.CreateBranch(context.Background(), "bad/", "")
	testutil.AssertError(t, err)

	// Branch starting with /
	err = client.CreateBranch(context.Background(), "/bad", "")
	testutil.AssertError(t, err)

	// Branch ending with .
	err = client.CreateBranch(context.Background(), "bad.", "")
	testutil.AssertError(t, err)

	// Branch ending with .lock
	err = client.CreateBranch(context.Background(), "bad.lock", "")
	testutil.AssertError(t, err)

	// Branch = @
	err = client.CreateBranch(context.Background(), "@", "")
	testutil.AssertError(t, err)

	// Forbidden characters
	err = client.CreateBranch(context.Background(), "bad~name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad^name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad:name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad?name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad*name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad[name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad\\name", "")
	testutil.AssertError(t, err)

	// Control character
	err = client.CreateBranch(context.Background(), "bad\x01name", "")
	testutil.AssertError(t, err)

	// Whitespace
	err = client.CreateBranch(context.Background(), "bad name", "")
	testutil.AssertError(t, err)
	err = client.CreateBranch(context.Background(), "bad\tname", "")
	testutil.AssertError(t, err)
}

func TestGitClient_RemoteValidation_NulByte(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	testutil.AssertError(t, client.Fetch(context.Background(), "bad\x00remote"))
}
