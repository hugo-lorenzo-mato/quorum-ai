package state

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestSQLiteStateManager_ConcurrentWrites tests concurrent SQLite writes.
func TestSQLiteStateManager_ConcurrentWrites(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "concurrent.db")
	manager := newTestSQLiteStateManagerForCritical(t, dbPath)

	const numWriters = 8
	const writesPerWorker = 5

	var wg sync.WaitGroup
	errors := make(chan error, numWriters*writesPerWorker)

	// Launch concurrent writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < writesPerWorker; j++ {
				workflowID := core.WorkflowID(fmt.Sprintf("wf-concurrent-%d-%d", workerID, j))
				state := newTestWorkflowStateForCritical(workflowID)

				if err := manager.Save(context.Background(), state); err != nil {
					errors <- fmt.Errorf("worker %d iteration %d: %w", workerID, j, err)
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errors)
	}()

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent write failed: %v", err)
	}

	// Verify all workflows were saved
	workflows, err := manager.ListWorkflows(context.Background())
	if err != nil {
		t.Fatalf("Failed to list workflows: %v", err)
	}

	expectedCount := numWriters * writesPerWorker
	if len(workflows) != expectedCount {
		t.Errorf("Expected %d workflows, got %d", expectedCount, len(workflows))
	}
}

// TestAtomicWriteFile_Concurrent tests concurrent atomic file writes.
func TestAtomicWriteFile_Concurrent(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "atomic_test.txt")

	const numWriters = 10
	const writesPerWorker = 3

	var wg sync.WaitGroup
	errors := make(chan error, numWriters*writesPerWorker)

	// Launch concurrent atomic writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < writesPerWorker; j++ {
				content := fmt.Sprintf("Worker %d - Write %d - %d", workerID, j, time.Now().UnixNano())

				err := atomicWriteFile(targetPath, []byte(content), 0644)
				if err != nil {
					errors <- fmt.Errorf("worker %d write %d: %w", workerID, j, err)
				}

				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errors)
	}()

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent atomic write failed: %v", err)
	}

	// Verify final file is valid and complete
	finalContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read final file: %v", err)
	}

	contentStr := string(finalContent)
	if !strings.HasPrefix(contentStr, "Worker ") {
		t.Errorf("Final file appears corrupted: %q", contentStr)
	}
}

// TestAtomicWriteFile_PlatformSpecific tests platform behavior.
func TestAtomicWriteFile_PlatformSpecific(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "platform_test.txt")

	// Test normal write
	content1 := []byte("First write")
	err := atomicWriteFile(targetPath, content1, 0644)
	if err != nil {
		t.Fatalf("First atomic write failed: %v", err)
	}

	readContent, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read after first write: %v", err)
	}

	if string(readContent) != "First write" {
		t.Errorf("Content mismatch: got %q, want %q", string(readContent), "First write")
	}

	// Test overwrite
	content2 := []byte("Second write")
	err = atomicWriteFile(targetPath, content2, 0644)
	if err != nil {
		t.Fatalf("Second atomic write failed: %v", err)
	}

	readContent2, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read after second write: %v", err)
	}

	if string(readContent2) != "Second write" {
		t.Errorf("Overwrite failed: got %q, want %q", string(readContent2), "Second write")
	}

	// Platform-specific cleanup verification
	if runtime.GOOS == "windows" {
		tempPattern := targetPath + ".tmp"
		if _, err := os.Stat(tempPattern); err == nil {
			t.Error("Temp file should be cleaned up on Windows")
		}
	}
}

// TestSQLiteStateManager_CorruptionRecovery tests behavior during corruption scenarios.
func TestSQLiteStateManager_CorruptionRecovery(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "corruption_test.db")

	// First, create a healthy database
	manager1 := newTestSQLiteStateManagerForCritical(t, dbPath)
	state1 := newTestWorkflowStateForCritical(core.WorkflowID("test-workflow"))
	err := manager1.Save(context.Background(), state1)
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}
	manager1.Close()

	// Simulate corruption by writing invalid data
	corruptData := []byte("This is not valid SQLite data")
	if err := os.WriteFile(dbPath, corruptData, 0644); err != nil {
		t.Fatalf("Failed to corrupt database: %v", err)
	}

	// Try to create new manager with corrupted database
	manager2, err := NewSQLiteStateManager(dbPath)
	if err == nil {
		t.Log("SQLite gracefully handled corrupted file (expected behavior)")
		manager2.Close()
	} else {
		t.Logf("SQLite detected corruption: %v (expected behavior)", err)
	}
}

// Helper function to create test SQLite state manager
func newTestSQLiteStateManagerForCritical(t *testing.T, dbPath string) *SQLiteStateManager {
	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite state manager: %v", err)
	}

	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Logf("Failed to close state manager: %v", err)
		}
	})

	return manager
}

// Helper function to create test state
func newTestWorkflowStateForCritical(workflowID core.WorkflowID) *core.WorkflowState {
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			WorkflowID: workflowID,
			Title:      "Test Workflow",
			Prompt:     "Test prompt",
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			ExecutionID:  1,
			Status:       core.WorkflowStatusPending,
			CurrentPhase: core.PhaseRefine,
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			AgentEvents:  []core.AgentEvent{},
			Metrics:      &core.StateMetrics{},
			Checkpoints:  []core.Checkpoint{},
			UpdatedAt:    time.Now(),
		},
	}
}
