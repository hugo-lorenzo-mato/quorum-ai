package git_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
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

func TestWorktreeManager_Create_EmptyBranch(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewWorktreeManager(client, worktreeDir)

	_, err = manager.Create(context.Background(), "test-task", "")
	testutil.AssertError(t, err)
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

// =============================================================================
// TaskWorktreeManager tests (core.WorktreeManager implementation)
// =============================================================================

func TestTaskWorktreeManager_Create(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	// Create worktree using TaskID
	task := &core.Task{ID: "task-123", Name: "Feature setup"}
	info, err := manager.Create(context.Background(), task, "feature-branch")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, string(info.TaskID), "task-123")
	testutil.AssertEqual(t, info.Branch, "feature-branch")

	// Verify path exists
	_, err = os.Stat(info.Path)
	testutil.AssertNoError(t, err)
}

func TestTaskWorktreeManager_Create_MissingName(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	_, err = manager.Create(context.Background(), &core.Task{ID: "task-321"}, "branch-321")
	testutil.AssertError(t, err)
}

func TestTaskWorktreeManager_Create_NonAsciiName(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	var buf bytes.Buffer
	logger := logging.New(logging.Config{
		Level:  "warn",
		Format: "text",
		Output: &buf,
	})
	manager := git.NewTaskWorktreeManager(client, worktreeDir).WithLogger(logger)

	task := &core.Task{ID: "task-901", Name: "日本語のタスク"}
	info, err := manager.Create(context.Background(), task, "branch-901")
	testutil.AssertNoError(t, err)

	base := filepath.Base(info.Path)
	testutil.AssertEqual(t, base, "quorum-task-901")

	output := buf.String()
	if !strings.Contains(output, "worktree label normalized empty, using task id only") {
		t.Fatalf("expected fallback log message, got: %s", output)
	}
	if !strings.Contains(output, "worktree_name=task-901") {
		t.Fatalf("expected worktree_name in log, got: %s", output)
	}
	if !strings.Contains(output, "task_name=日本語のタスク") {
		t.Fatalf("expected task_name in log, got: %s", output)
	}
}

func TestTaskWorktreeManager_Create_NonAsciiDescription(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	var buf bytes.Buffer
	logger := logging.New(logging.Config{
		Level:  "warn",
		Format: "text",
		Output: &buf,
	})
	manager := git.NewTaskWorktreeManager(client, worktreeDir).WithLogger(logger)

	task := &core.Task{ID: "task-902", Description: "日本語の説明"}
	info, err := manager.Create(context.Background(), task, "branch-902")
	testutil.AssertNoError(t, err)

	base := filepath.Base(info.Path)
	testutil.AssertEqual(t, base, "quorum-task-902")

	output := buf.String()
	if !strings.Contains(output, "worktree label normalized empty, using task id only") {
		t.Fatalf("expected fallback log message, got: %s", output)
	}
	if !strings.Contains(output, "worktree_name=task-902") {
		t.Fatalf("expected worktree_name in log, got: %s", output)
	}
	if !strings.Contains(output, "task_description=日本語の説明") {
		t.Fatalf("expected task_description in log, got: %s", output)
	}
}

func TestTaskWorktreeManager_Get(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	// Create worktree
	task := &core.Task{ID: "task-456", Name: "Fetch metadata"}
	_, err = manager.Create(context.Background(), task, "test-branch")
	testutil.AssertNoError(t, err)

	// Get it back
	info, err := manager.Get(context.Background(), task)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, string(info.TaskID), "task-456")
	testutil.AssertEqual(t, info.Branch, "test-branch")
}

func TestTaskWorktreeManager_Remove(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	// Create and remove
	task := &core.Task{ID: "task-789", Name: "Cleanup staging"}
	info, err := manager.Create(context.Background(), task, "remove-branch")
	testutil.AssertNoError(t, err)

	err = manager.Remove(context.Background(), task)
	testutil.AssertNoError(t, err)

	// Should no longer exist
	_, err = os.Stat(info.Path)
	testutil.AssertError(t, err)
}

func TestTaskWorktreeManager_List(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	manager := git.NewTaskWorktreeManager(client, worktreeDir)

	// Create multiple worktrees
	_, err = manager.Create(context.Background(), &core.Task{ID: "task-a", Name: "Alpha task"}, "branch-a")
	testutil.AssertNoError(t, err)
	_, err = manager.Create(context.Background(), &core.Task{ID: "task-b", Name: "Beta task"}, "branch-b")
	testutil.AssertNoError(t, err)

	// List them
	list, err := manager.List(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertLen(t, list, 2)
}
