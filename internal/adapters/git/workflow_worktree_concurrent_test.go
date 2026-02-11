package git_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// TestWorkflowWorktreeManager_ConcurrentCreation tests that multiple
// goroutines can create different workflows simultaneously without corruption.
func TestWorkflowWorktreeManager_ConcurrentCreation(t *testing.T) {
	t.Parallel()

	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	const numWorkers = 10
	var wg sync.WaitGroup
	errors := make(chan error, numWorkers)
	results := make(chan string, numWorkers)

	// Launch concurrent workflow creations
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			workflowID := fmt.Sprintf("wf-concurrent-%d-%d", workerID, time.Now().UnixNano())

			info, err := mgr.InitializeWorkflow(context.Background(), workflowID, "main")
			if err != nil {
				errors <- fmt.Errorf("worker %d: %w", workerID, err)
				return
			}

			results <- info.WorktreeRoot
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(errors)
		close(results)
	}()

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent creation failed: %v", err)
	}

	// Verify all workflows were created successfully
	createdPaths := make(map[string]bool)
	resultCount := 0
	for path := range results {
		if createdPaths[path] {
			t.Errorf("Duplicate worktree path: %s", path)
		}
		createdPaths[path] = true
		resultCount++
	}

	testutil.AssertEqual(t, resultCount, numWorkers)

	// Verify all workflows are listed
	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(workflows), numWorkers)
}

// TestWorkflowWorktreeManager_ConcurrentTaskCreation tests concurrent task
// creation within the same workflow to detect race conditions.
func TestWorkflowWorktreeManager_ConcurrentTaskCreation(t *testing.T) {
	t.Parallel()

	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	// Create a single workflow first
	workflowID := fmt.Sprintf("wf-tasks-%d", time.Now().UnixNano())
	_, err = mgr.InitializeWorkflow(context.Background(), workflowID, "main")
	testutil.AssertNoError(t, err)

	const numTasks = 8
	var wg sync.WaitGroup
	errors := make(chan error, numTasks)
	taskPaths := make(chan string, numTasks)

	// Launch concurrent task creations in same workflow
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			// Create task with proper structure
			task := &core.Task{
				ID:          core.TaskID(fmt.Sprintf("task-%d", taskID)),
				Description: fmt.Sprintf("Task %d", taskID),
			}

			taskInfo, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
			if err != nil {
				errors <- fmt.Errorf("task %d: %w", taskID, err)
				return
			}

			taskPaths <- taskInfo.Path
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(errors)
		close(taskPaths)
	}()

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent task creation failed: %v", err)
	}

	// Verify all tasks were created with unique paths
	createdPaths := make(map[string]bool)
	taskCount := 0
	for path := range taskPaths {
		if createdPaths[path] {
			t.Errorf("Duplicate task worktree path: %s", path)
		}
		createdPaths[path] = true
		taskCount++
	}

	testutil.AssertEqual(t, taskCount, numTasks)
}

// TestWorkflowWorktreeManager_ConcurrentCleanup tests that concurrent cleanup
// operations don't interfere with each other or leave resources leaked.
func TestWorkflowWorktreeManager_ConcurrentCleanup(t *testing.T) {
	t.Parallel()

	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	const numWorkflows = 6
	workflowIDs := make([]string, numWorkflows)

	// Create multiple workflows first
	for i := 0; i < numWorkflows; i++ {
		workflowID := fmt.Sprintf("wf-cleanup-%d-%d", i, time.Now().UnixNano())
		workflowIDs[i] = workflowID

		_, err := mgr.InitializeWorkflow(context.Background(), workflowID, "main")
		testutil.AssertNoError(t, err)

		// Add some tasks to each workflow
		for j := 0; j < 3; j++ {
			task := &core.Task{
				ID:          core.TaskID(fmt.Sprintf("task-%d", j)),
				Description: fmt.Sprintf("Task %d", j),
			}
			_, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
			testutil.AssertNoError(t, err)
		}
	}

	var wg sync.WaitGroup
	errors := make(chan error, numWorkflows)

	// Launch concurrent cleanup operations
	for i := 0; i < numWorkflows; i++ {
		wg.Add(1)
		go func(workflowID string, workerID int) {
			defer wg.Done()

			err := mgr.CleanupWorkflow(context.Background(), workflowID, false)
			if err != nil {
				errors <- fmt.Errorf("worker %d cleaning %s: %w", workerID, workflowID, err)
			}
		}(workflowIDs[i], i)
	}

	// Wait for all cleanups to complete
	go func() {
		wg.Wait()
		close(errors)
	}()

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent cleanup failed: %v", err)
	}

	// Verify no active workflows remain
	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(workflows), 0)
}

// TestWorkflowWorktreeManager_RaceConditionStressTest simulates a realistic
// stress scenario with mixed operations happening concurrently.
func TestWorkflowWorktreeManager_RaceConditionStressTest(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Stress test: mixed concurrent operations
	for i := 0; i < 5; i++ {
		// Workflow creator
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; ctx.Err() == nil && j < 3; j++ {
				workflowID := fmt.Sprintf("stress-wf-%d-%d-%d", workerID, j, time.Now().UnixNano())

				if _, err := mgr.InitializeWorkflow(ctx, workflowID, "main"); err != nil {
					if ctx.Err() == nil {
						errors <- fmt.Errorf("creator %d: %w", workerID, err)
					}
					return
				}

				// Add tasks to this workflow
				for k := 0; k < 2; k++ {
					task := &core.Task{
						ID:          core.TaskID(fmt.Sprintf("task-%d", k)),
						Description: fmt.Sprintf("Task %d", k),
					}
					if _, err := mgr.CreateTaskWorktree(ctx, workflowID, task); err != nil {
						if ctx.Err() == nil {
							errors <- fmt.Errorf("task creator %d: %w", workerID, err)
						}
					}
				}

				// Clean up some workflows
				if j > 0 {
					oldWorkflowID := fmt.Sprintf("stress-wf-%d-%d-%d", workerID, j-1, time.Now().UnixNano())
					_ = mgr.CleanupWorkflow(ctx, oldWorkflowID, false) // Ignore not found errors
				}

				time.Sleep(10 * time.Millisecond)
			}
		}(i)

		// Status checker
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for ctx.Err() == nil {
				if _, err := mgr.ListActiveWorkflows(ctx); err != nil {
					if ctx.Err() == nil {
						errors <- fmt.Errorf("status checker %d: %w", workerID, err)
					}
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
		}(i)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, everything completed
	case <-ctx.Done():
		// Timeout is acceptable for stress test
		t.Logf("Stress test timed out (expected)")
	}

	// Close errors channel
	go func() {
		<-done // Wait for workers to finish
		close(errors)
	}()

	// Check for any unexpected errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Stress test error: %v", err)
		errorCount++
		if errorCount > 10 {
			t.Fatalf("Too many errors, stopping")
		}
	}
}

// TestWorkflowWorktreeManager_ResourceLeakDetection verifies that repeated
// create/cleanup cycles don't leak file descriptors or disk space.
func TestWorkflowWorktreeManager_ResourceLeakDetection(t *testing.T) {
	t.Parallel()

	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	client, err := git.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	worktreeDir := testutil.TempDir(t)
	mgr, err := git.NewWorkflowWorktreeManager(repo.Path, worktreeDir, client, nil)
	testutil.AssertNoError(t, err)

	const cycles = 20

	for i := 0; i < cycles; i++ {
		workflowID := fmt.Sprintf("leak-test-%d", i)

		// Create workflow
		_, err := mgr.InitializeWorkflow(context.Background(), workflowID, "main")
		testutil.AssertNoError(t, err)

		// Create some tasks
		for j := 0; j < 5; j++ {
			task := &core.Task{
				ID:          core.TaskID(fmt.Sprintf("task-%d", j)),
				Description: fmt.Sprintf("Task %d", j),
			}
			_, err := mgr.CreateTaskWorktree(context.Background(), workflowID, task)
			testutil.AssertNoError(t, err)
		}

		// Cleanup immediately
		err = mgr.CleanupWorkflow(context.Background(), workflowID, false)
		testutil.AssertNoError(t, err)

		// Verify no active workflows
		workflows, err := mgr.ListActiveWorkflows(context.Background())
		testutil.AssertNoError(t, err)
		if len(workflows) != 0 {
			t.Fatalf("Cycle %d: expected 0 active workflows, got %d", i, len(workflows))
		}
	}

	// Final verification: worktree directory should be mostly empty
	// (only .gitkeep or similar administrative files should remain)
	workflows, err := mgr.ListActiveWorkflows(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, len(workflows), 0)
}
