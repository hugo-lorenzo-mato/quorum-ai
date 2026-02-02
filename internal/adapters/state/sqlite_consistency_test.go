package state

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// TestSQLiteStateManager_GetActiveWorkflowID_DetectsGhostWorkflow verifies that
// GetActiveWorkflowID detects and cleans ghost workflows (workflows marked as
// failed but still in active_workflow table).
// Regression: wf-20260130-030319-atstd was active_workflow with status=failed.
func TestSQLiteStateManager_GetActiveWorkflowID_DetectsGhostWorkflow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Create a failed workflow
	now := time.Now().Truncate(time.Second)
	state := &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-ghost-test-001",
		Status:       core.WorkflowStatusFailed,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "test prompt that failed",
		Error:        "simulated error for testing ghost workflow detection",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Force inconsistency: set the failed workflow as active
	// This simulates a ghost workflow scenario
	_, err = manager.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO active_workflow (id, workflow_id, updated_at) VALUES (1, ?, ?)",
		state.WorkflowID, time.Now())
	if err != nil {
		t.Fatalf("Failed to insert active_workflow: %v", err)
	}

	// GetActiveWorkflowID should detect and clean the ghost workflow
	activeID, err := manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "" {
		t.Errorf("GetActiveWorkflowID() = %q, want empty for ghost workflow", activeID)
	}

	// Verify the active_workflow table is now clean
	var count int
	err = manager.readDB.QueryRow("SELECT COUNT(*) FROM active_workflow").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count active_workflow: %v", err)
	}
	if count != 0 {
		t.Errorf("active_workflow count = %d, want 0 after ghost cleanup", count)
	}
}

// TestSQLiteStateManager_GetActiveWorkflowID_DetectsOrphanReference verifies that
// GetActiveWorkflowID detects references to non-existent workflows.
func TestSQLiteStateManager_GetActiveWorkflowID_DetectsOrphanReference(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Force an orphan reference: set active_workflow to a non-existent workflow
	_, err = manager.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO active_workflow (id, workflow_id, updated_at) VALUES (1, ?, ?)",
		"wf-nonexistent-12345", time.Now())
	if err != nil {
		t.Fatalf("Failed to insert orphan reference: %v", err)
	}

	// GetActiveWorkflowID should detect and clean the orphan reference
	activeID, err := manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "" {
		t.Errorf("GetActiveWorkflowID() = %q, want empty for orphan reference", activeID)
	}

	// Verify cleanup occurred
	var count int
	err = manager.readDB.QueryRow("SELECT COUNT(*) FROM active_workflow").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count active_workflow: %v", err)
	}
	if count != 0 {
		t.Errorf("active_workflow count = %d, want 0 after orphan cleanup", count)
	}
}

// TestSQLiteStateManager_GetActiveWorkflowID_PreservesValidWorkflow verifies that
// GetActiveWorkflowID does NOT clean up workflows with valid statuses.
func TestSQLiteStateManager_GetActiveWorkflowID_PreservesValidWorkflow(t *testing.T) {
	testCases := []struct {
		name   string
		status core.WorkflowStatus
	}{
		{"pending", core.WorkflowStatusPending},
		{"running", core.WorkflowStatusRunning},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "state.db")

			manager, err := NewSQLiteStateManager(dbPath)
			if err != nil {
				t.Fatalf("NewSQLiteStateManager() error = %v", err)
			}
			defer manager.Close()

			// Create a workflow with valid status
			now := time.Now().Truncate(time.Second)
			state := &core.WorkflowState{
				Version:      1,
				WorkflowID:   core.WorkflowID("wf-valid-" + tc.name),
				Status:       tc.status,
				CurrentPhase: core.PhaseAnalyze,
				Prompt:       "test prompt",
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := manager.Save(ctx, state); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			// Set as active
			if err := manager.SetActiveWorkflowID(ctx, state.WorkflowID); err != nil {
				t.Fatalf("SetActiveWorkflowID() error = %v", err)
			}

			// GetActiveWorkflowID should preserve the valid workflow
			activeID, err := manager.GetActiveWorkflowID(ctx)
			if err != nil {
				t.Fatalf("GetActiveWorkflowID() error = %v", err)
			}
			if activeID != state.WorkflowID {
				t.Errorf("GetActiveWorkflowID() = %q, want %q for valid workflow", activeID, state.WorkflowID)
			}
		})
	}
}

// TestSQLiteStateManager_GetActiveWorkflowID_DetectsCompletedWorkflow verifies that
// GetActiveWorkflowID also cleans up completed workflows that are still marked as active.
func TestSQLiteStateManager_GetActiveWorkflowID_DetectsCompletedWorkflow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Create a completed workflow
	now := time.Now().Truncate(time.Second)
	state := &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-completed-ghost",
		Status:       core.WorkflowStatusCompleted,
		CurrentPhase: "", // Empty means fully completed
		Prompt:       "test prompt that completed",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Force inconsistency: set the completed workflow as active
	_, err = manager.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO active_workflow (id, workflow_id, updated_at) VALUES (1, ?, ?)",
		state.WorkflowID, time.Now())
	if err != nil {
		t.Fatalf("Failed to insert active_workflow: %v", err)
	}

	// GetActiveWorkflowID should detect and clean
	activeID, err := manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "" {
		t.Errorf("GetActiveWorkflowID() = %q, want empty for completed workflow", activeID)
	}
}

// Note: TestSQLiteStateManager_CleanupOnStartup_GhostWorkflow is already defined
// in sqlite_test.go - not duplicating here.

// TestSQLiteStateManager_DeactivateWorkflow_ClearsActiveWorkflow verifies that
// DeactivateWorkflow properly clears the active workflow.
func TestSQLiteStateManager_DeactivateWorkflow_ClearsActiveWorkflow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Create and activate a workflow
	now := time.Now().Truncate(time.Second)
	state := &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-deactivate-test",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "test prompt",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := manager.SetActiveWorkflowID(ctx, state.WorkflowID); err != nil {
		t.Fatalf("SetActiveWorkflowID() error = %v", err)
	}

	// Verify it's active
	activeID, _ := manager.GetActiveWorkflowID(ctx)
	if activeID != state.WorkflowID {
		t.Fatalf("Workflow should be active before deactivation")
	}

	// Deactivate
	if err := manager.DeactivateWorkflow(ctx); err != nil {
		t.Fatalf("DeactivateWorkflow() error = %v", err)
	}

	// Verify it's no longer active
	activeID, err = manager.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID() error = %v", err)
	}
	if activeID != "" {
		t.Errorf("GetActiveWorkflowID() = %q, want empty after deactivation", activeID)
	}

	// But the workflow itself should still exist
	loaded, err := manager.LoadByID(ctx, state.WorkflowID)
	if err != nil {
		t.Fatalf("LoadByID() error = %v", err)
	}
	if loaded == nil {
		t.Error("Workflow should still exist after deactivation")
	}
}

// TestSQLiteStateManager_FindWorkflowsByPrompt_DetectsDuplicates verifies that
// FindWorkflowsByPrompt correctly identifies workflows with identical prompts.
func TestSQLiteStateManager_FindWorkflowsByPrompt_DetectsDuplicates(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Create multiple workflows with the same prompt
	prompt := "Analyze the system architecture and identify bottlenecks"
	now := time.Now().Truncate(time.Second)

	for i, status := range []core.WorkflowStatus{
		core.WorkflowStatusFailed,
		core.WorkflowStatusCompleted,
		core.WorkflowStatusRunning,
	} {
		state := &core.WorkflowState{
			Version:      1,
			WorkflowID:   core.WorkflowID(time.Now().Format("20060102-150405") + string(rune('a'+i))),
			Status:       status,
			CurrentPhase: core.PhaseAnalyze,
			Prompt:       prompt,
			CreatedAt:    now.Add(time.Duration(i) * time.Hour),
			UpdatedAt:    now.Add(time.Duration(i) * time.Hour),
		}
		if err := manager.Save(ctx, state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Find workflows with the same prompt
	duplicates, err := manager.FindWorkflowsByPrompt(ctx, prompt)
	if err != nil {
		t.Fatalf("FindWorkflowsByPrompt() error = %v", err)
	}

	if len(duplicates) != 3 {
		t.Errorf("FindWorkflowsByPrompt() returned %d duplicates, want 3", len(duplicates))
	}
}

// TestSQLiteStateManager_FindWorkflowsByPrompt_NoDuplicates verifies that
// FindWorkflowsByPrompt returns empty for unique prompts.
func TestSQLiteStateManager_FindWorkflowsByPrompt_NoDuplicates(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")

	manager, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager() error = %v", err)
	}
	defer manager.Close()

	// Create a workflow
	now := time.Now().Truncate(time.Second)
	state := &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-unique-prompt",
		Status:       core.WorkflowStatusCompleted,
		CurrentPhase: "",
		Prompt:       "This is a unique prompt",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Search for a different prompt
	duplicates, err := manager.FindWorkflowsByPrompt(ctx, "This is a different prompt")
	if err != nil {
		t.Fatalf("FindWorkflowsByPrompt() error = %v", err)
	}

	if len(duplicates) != 0 {
		t.Errorf("FindWorkflowsByPrompt() returned %d duplicates, want 0", len(duplicates))
	}
}
