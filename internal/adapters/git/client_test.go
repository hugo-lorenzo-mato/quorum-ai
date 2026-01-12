package git_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestGitClient_NewClient(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	if client.RepoPath() != repo.Path {
		t.Errorf("RepoPath() = %s, want %s", client.RepoPath(), repo.Path)
	}
}

func TestGitClient_NewClient_NotARepo(t *testing.T) {
	dir := testutil.TempDir(t)

	_, err := git.NewClient(dir)
	testutil.AssertError(t, err)
}

func TestGitClient_Status(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Clean status
	status, err := client.Status(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, status.Unstaged, 0)
	testutil.AssertLen(t, status.Untracked, 0)

	isClean, err := client.IsClean(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, isClean, "should be clean")

	// Add untracked file
	repo.WriteFile("new.txt", "new content")
	status, err = client.Status(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, status.Untracked, 1)

	isClean, err = client.IsClean(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, isClean, "should not be clean")
}

func TestGitClient_CurrentBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	branch, err := client.CurrentBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "main")
}

func TestGitClient_CreateBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create new branch
	err = client.CreateBranch(context.Background(), "feature", "")
	testutil.AssertNoError(t, err)

	branch, err := client.CurrentBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "feature")
}

func TestGitClient_ListBranches(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create additional branch
	err = client.CreateBranch(context.Background(), "feature", "")
	testutil.AssertNoError(t, err)

	branches, err := client.ListBranches(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, branches, 2)
}

func TestGitClient_BranchExists(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	exists, err := client.BranchExists(context.Background(), "main")
	testutil.AssertNoError(t, err)
	testutil.AssertTrue(t, exists, "main should exist")

	exists, err = client.BranchExists(context.Background(), "nonexistent")
	testutil.AssertNoError(t, err)
	testutil.AssertFalse(t, exists, "nonexistent should not exist")
}

func TestGitClient_CommitAll(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Make a change
	repo.WriteFile("file.txt", "content")

	// Commit all
	hash, err := client.CommitAll(context.Background(), "Add file")
	testutil.AssertNoError(t, err)
	if len(hash) != 40 {
		t.Errorf("hash length = %d, want 40", len(hash))
	}

	// Verify commit
	commits, err := client.Log(context.Background(), 2)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, commits, 2)
	testutil.AssertEqual(t, commits[0].Subject, "Add file")
}

func TestGitClient_Log(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("First")
	repo.WriteFile("file2.txt", "content")
	repo.Commit("Second")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	commits, err := client.Log(context.Background(), 5)
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, commits, 2)

	testutil.AssertEqual(t, commits[0].Subject, "Second")
	testutil.AssertEqual(t, commits[0].AuthorName, "Test User")
	testutil.AssertEqual(t, commits[0].AuthorEmail, "test@example.com")
	if len(commits[0].Hash) != 40 {
		t.Errorf("hash length = %d, want 40", len(commits[0].Hash))
	}
}

func TestGitClient_Diff(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// No diff initially
	diff, err := client.DiffUnstaged(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, diff, "")

	// Make a change
	repo.WriteFile("README.md", "# Test\n\nMore content")

	diff, err = client.DiffUnstaged(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertContains(t, diff, "More content")
}

func TestGitClient_Checkout(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create and switch to new branch
	err = client.Checkout(context.Background(), "new-branch", true)
	testutil.AssertNoError(t, err)

	branch, _ := client.CurrentBranch(context.Background())
	testutil.AssertEqual(t, branch, "new-branch")

	// Switch back to main
	err = client.Checkout(context.Background(), "main", false)
	testutil.AssertNoError(t, err)

	branch, _ = client.CurrentBranch(context.Background())
	testutil.AssertEqual(t, branch, "main")
}

func TestGitClient_DeleteBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create and switch back
	err = client.CreateBranch(context.Background(), "to-delete", "")
	testutil.AssertNoError(t, err)
	err = client.Checkout(context.Background(), "main", false)
	testutil.AssertNoError(t, err)

	// Delete branch
	err = client.DeleteBranch(context.Background(), "to-delete")
	testutil.AssertNoError(t, err)

	exists, _ := client.BranchExists(context.Background(), "to-delete")
	testutil.AssertFalse(t, exists, "branch should be deleted")
}

func TestGitClient_CurrentCommit(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	expectedHash := repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	hash, err := client.CurrentCommit(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, hash, expectedHash)
}

func TestGitClient_DefaultBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	branch, err := client.DefaultBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "main")
}

func TestGitClient_Add(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Create new file
	newFile := filepath.Join(repo.Path, "new.txt")
	os.WriteFile(newFile, []byte("content"), 0o644)

	// Add it
	err = client.Add(context.Background(), "new.txt")
	testutil.AssertNoError(t, err)

	// Check status
	status, err := client.Status(context.Background())
	testutil.AssertNoError(t, err)
	// File should be staged
	testutil.AssertLen(t, status.Untracked, 0)
}
