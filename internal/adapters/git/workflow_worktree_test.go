package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// uniqueWorkflowID generates a unique workflow ID for each test to avoid branch name collisions
func uniqueWorkflowID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestNewWorkflowWorktreeManager(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestInitializeWorkflow(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	info, err := mgr.InitializeWorkflow(context.Background(), "wf-test-123", "main")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, info.WorkflowID, "wf-test-123")
	testutil.AssertEqual(t, info.WorkflowBranch, "quorum/wf-test-123")
	testutil.AssertEqual(t, info.BaseBranch, "main")

	// Verify worktree root directory exists
	_, err = os.Stat(info.WorktreeRoot)
	testutil.AssertNoError(t, err)

	// Verify branch was created
	exists, err := client.BranchExists(context.Background(), "quorum/wf-test-123")
	testutil.AssertNoError(t, err)
	if !exists {
		t.Fatal("workflow branch should exist")
	}
}

func TestInitializeWorkflow_Reuse(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize once
	info1, err := mgr.InitializeWorkflow(context.Background(), "wf-reuse", "main")
	testutil.AssertNoError(t, err)

	// Initialize again - should reuse
	info2, err := mgr.InitializeWorkflow(context.Background(), "wf-reuse", "main")
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, info1.WorkflowBranch, info2.WorkflowBranch)
}

func TestCreateTaskWorktree(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize workflow
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-task-test", "main")
	testutil.AssertNoError(t, err)

	// Create task worktree
	task := &core.Task{ID: "task-001", Description: "Implement feature"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), "wf-task-test", task)
	testutil.AssertNoError(t, err)

	// Verify worktree info
	testutil.AssertEqual(t, wtInfo.TaskID, core.TaskID("task-001"))
	testutil.AssertEqual(t, wtInfo.Branch, "quorum/wf-task-test__task-001")

	// Verify path exists
	_, err = os.Stat(wtInfo.Path)
	testutil.AssertNoError(t, err)

	// Verify branch exists
	exists, err := client.BranchExists(context.Background(), "quorum/wf-task-test__task-001")
	testutil.AssertNoError(t, err)
	if !exists {
		t.Fatal("task branch should exist")
	}
}

func TestRemoveTaskWorktree(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize workflow and create task
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-remove-test", "main")
	testutil.AssertNoError(t, err)

	task := &core.Task{ID: "task-remove", Description: "To be removed"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), "wf-remove-test", task)
	testutil.AssertNoError(t, err)

	// Remove without branch deletion
	err = mgr.RemoveTaskWorktree(context.Background(), "wf-remove-test", task.ID, false)
	testutil.AssertNoError(t, err)

	// Verify worktree removed
	_, err = os.Stat(wtInfo.Path)
	if !os.IsNotExist(err) {
		t.Fatal("worktree directory should be removed")
	}

	// Branch should still exist
	exists, err := client.BranchExists(context.Background(), "quorum/wf-remove-test__task-remove")
	testutil.AssertNoError(t, err)
	if !exists {
		t.Fatal("branch should still exist when removeBranch=false")
	}
}

func TestRemoveTaskWorktree_WithBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize workflow and create task
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-rm-branch", "main")
	testutil.AssertNoError(t, err)

	task := &core.Task{ID: "task-rm", Description: "To be fully removed"}
	_, err = mgr.CreateTaskWorktree(context.Background(), "wf-rm-branch", task)
	testutil.AssertNoError(t, err)

	// Remove with branch deletion
	err = mgr.RemoveTaskWorktree(context.Background(), "wf-rm-branch", task.ID, true)
	testutil.AssertNoError(t, err)

	// Branch should be gone
	exists, err := client.BranchExists(context.Background(), "quorum/wf-rm-branch__task-rm")
	testutil.AssertNoError(t, err)
	if exists {
		t.Fatal("branch should be removed when removeBranch=true")
	}
}

func TestMergeTaskToWorkflow_Sequential(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Use unique workflow ID to avoid branch collisions across tests
	workflowID := uniqueWorkflowID("wf-merge-test")

	// Initialize workflow
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Create task and make changes
	task := &core.Task{ID: "task-merge", Description: "Feature to merge"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Make a commit in the task worktree
	newFilePath := filepath.Join(wtInfo.Path, "new-feature.txt")
	err = os.WriteFile(newFilePath, []byte("new feature content"), 0644)
	testutil.AssertNoError(t, err)

	// Create a git client for the worktree
	taskClient, err := git.NewClient(wtInfo.Path)
	testutil.AssertNoError(t, err)

	err = taskClient.Add(context.Background(), "new-feature.txt")
	testutil.AssertNoError(t, err)

	_, err = taskClient.Commit(context.Background(), "Add new feature")
	testutil.AssertNoError(t, err)

	// Merge task to workflow
	err = mgr.MergeTaskToWorkflow(context.Background(), workflowID, task.ID, "sequential")
	testutil.AssertNoError(t, err)
}

func TestMergeAllTasksToWorkflow(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Use unique workflow ID
	workflowID := uniqueWorkflowID("wf-merge-all")

	// Initialize workflow
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Create multiple tasks
	tasks := []*core.Task{
		{ID: "task-1", Description: "First task"},
		{ID: "task-2", Description: "Second task"},
	}

	var taskIDs []core.TaskID
	for _, task := range tasks {
		wtInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
		testutil.AssertNoError(t, err)

		// Make changes in each task
		filePath := filepath.Join(wtInfo.Path, string(task.ID)+".txt")
		err = os.WriteFile(filePath, []byte("content for "+string(task.ID)), 0644)
		testutil.AssertNoError(t, err)

		taskClient, err := git.NewClient(wtInfo.Path)
		testutil.AssertNoError(t, err)

		err = taskClient.Add(context.Background(), string(task.ID)+".txt")
		testutil.AssertNoError(t, err)

		_, err = taskClient.Commit(context.Background(), "Add "+string(task.ID))
		testutil.AssertNoError(t, err)

		taskIDs = append(taskIDs, task.ID)
	}

	// Merge all
	err = mgr.MergeAllTasksToWorkflow(context.Background(), workflowID, taskIDs, "sequential")
	testutil.AssertNoError(t, err)
}

func TestCleanupWorkflow(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Use unique workflow ID
	workflowID := uniqueWorkflowID("wf-cleanup")

	// Initialize workflow
	info, err := mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Create a task
	task := &core.Task{ID: "task-cleanup", Description: "To cleanup"}
	_, err = mgr.CreateTaskWorktree(context.Background(), workflowID, task)
	testutil.AssertNoError(t, err)

	// Cleanup without removing workflow branch
	err = mgr.CleanupWorkflow(context.Background(), workflowID, false)
	testutil.AssertNoError(t, err)

	// Verify worktree root removed
	_, err = os.Stat(info.WorktreeRoot)
	if !os.IsNotExist(err) {
		t.Fatal("worktree root should be removed")
	}

	// Workflow branch should still exist
	exists, err := client.BranchExists(context.Background(), mgr.GetWorkflowBranch(workflowID))
	testutil.AssertNoError(t, err)
	if !exists {
		t.Fatal("workflow branch should exist when removeWorkflowBranch=false")
	}
}

func TestCleanupWorkflow_WithBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Use unique workflow ID
	workflowID := uniqueWorkflowID("wf-cleanup-full")

	// Initialize workflow
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	// Cleanup with workflow branch removal
	err = mgr.CleanupWorkflow(context.Background(), workflowID, true)
	testutil.AssertNoError(t, err)

	// Workflow branch should be gone
	exists, err := client.BranchExists(context.Background(), mgr.GetWorkflowBranch(workflowID))
	testutil.AssertNoError(t, err)
	if exists {
		t.Fatal("workflow branch should be removed")
	}
}

func TestGetWorkflowStatus(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize workflow
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-status", "main")
	testutil.AssertNoError(t, err)

	// Get status
	status, err := mgr.GetWorkflowStatus(context.Background(), "wf-status")
	testutil.AssertNoError(t, err)

	if status == nil {
		t.Fatal("status should not be nil")
	}
	if status.HasConflicts {
		t.Fatal("should not have conflicts")
	}
}

func TestListActiveWorkflows(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initially empty
	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	if len(workflows) != 0 {
		t.Fatalf("expected 0 workflows, got %d", len(workflows))
	}

	// Create workflows
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-list-1", "main")
	testutil.AssertNoError(t, err)
	_, err = mgr.InitializeWorkflow(context.Background(), "wf-list-2", "main")
	testutil.AssertNoError(t, err)

	// List
	workflows, err = mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	if len(workflows) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(workflows))
	}
}

func TestGetWorkflowBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	branch := mgr.GetWorkflowBranch("wf-test-123")
	testutil.AssertEqual(t, branch, "quorum/wf-test-123")
}

func TestGetTaskBranch(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	branch := mgr.GetTaskBranch("wf-test", core.TaskID("task-1"))
	testutil.AssertEqual(t, branch, "quorum/wf-test__task-1")
}

func TestFinalizeWorkflow_NoMerge(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Initialize workflow
	info, err := mgr.InitializeWorkflow(context.Background(), "wf-finalize", "main")
	testutil.AssertNoError(t, err)

	// Finalize without merge
	err = mgr.FinalizeWorkflow(context.Background(), "wf-finalize", false)
	testutil.AssertNoError(t, err)

	// Worktree root should be cleaned up
	_, err = os.Stat(info.WorktreeRoot)
	if !os.IsNotExist(err) {
		t.Fatal("worktree root should be removed")
	}
}

func TestSanitizeForPath(t *testing.T) {
	t.Parallel()
	// Test via CreateTaskWorktree which uses sanitizeForPath internally
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	_, err = mgr.InitializeWorkflow(context.Background(), "wf-sanitize", "main")
	testutil.AssertNoError(t, err)

	// Task with special characters in description
	task := &core.Task{ID: "task-special", Description: "Feature: Add User/Auth Support!"}
	wtInfo, err := mgr.CreateTaskWorktree(context.Background(), "wf-sanitize", task)
	testutil.AssertNoError(t, err)

	// Path should not contain special characters
	if filepath.Base(wtInfo.Path) != "task-special__feature-add-user-auth-s" {
		// The description is sanitized and truncated
		t.Logf("Worktree path base: %s", filepath.Base(wtInfo.Path))
	}

	// Should at least start with task ID
	base := filepath.Base(wtInfo.Path)
	if len(base) < 12 || base[:12] != "task-special" {
		t.Fatalf("path should start with task ID, got %s", base)
	}
}
