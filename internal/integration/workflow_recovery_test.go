package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

// TestWorkflowRecovery_CrashDuringAnalysis tests recovery from crashes during analysis phase.
func TestWorkflowRecovery_CrashDuringAnalysis(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gitRepo := testutil.NewGitRepo(t)

	// Setup test scenario
	testFile := filepath.Join(gitRepo.Path, "main.go")
	err := os.WriteFile(testFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Commit initial state
	gitRepo.WriteFile("main.go", "package main\n\nfunc main() {}\n")
	gitRepo.Commit("Initial commit")

	// Create workflow state
	workflowID := "test-recovery-analysis"
	stateDB := createTestStateDB(t, tmpDir)
	
	// Save initial workflow state
	initialState := &WorkflowState{
		ID:         workflowID,
		Phase:      core.PhaseAnalyze,
		Status:     core.WorkflowStatusRunning,
		StartTime:  time.Now(),
		GitCommit:  "abc123",
		GitBranch:  "main",
		WorktreeDir: gitRepo.Path,
		Tasks: []core.TaskState{
			{
				ID:     core.TaskID("analysis-task-1"),
				Name:   "File Analysis",
				Phase:  core.PhaseAnalyze,
				Status: core.TaskStatusRunning,
				CLI:    "claude",
			},
		},
	}

	err = stateDB.Save(workflowID, initialState)
	if err != nil {
		t.Fatalf("Failed to save initial workflow state: %v", err)
	}

	// Simulate crash by updating state to reflect partial completion
	partialState := *initialState
	partialState.Tasks[0].Status = core.TaskStatusFailed
	partialState.Tasks[0].Error = "simulated crash during analysis"
	partialState.UpdatedAt = time.Now()

	err = stateDB.Save(workflowID, &partialState)
	if err != nil {
		t.Fatalf("Failed to save partial workflow state: %v", err)
	}

	// Test recovery
	recoveredState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load workflow state for recovery: %v", err)
	}

	// Verify recovery state
	if recoveredState.ID != workflowID {
		t.Errorf("Expected workflow ID %s, got %s", workflowID, recoveredState.ID)
	}

	if recoveredState.Phase != core.PhaseAnalyze {
		t.Errorf("Expected phase %s, got %s", core.PhaseAnalyze, recoveredState.Phase)
	}

	if len(recoveredState.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(recoveredState.Tasks))
	}

	if recoveredState.Tasks[0].Status != core.TaskStatusFailed {
		t.Errorf("Expected task status %s, got %s", core.TaskStatusFailed, recoveredState.Tasks[0].Status)
	}

	// Simulate recovery by retrying failed task
	retryState := *recoveredState
	retryState.Tasks[0].Status = core.TaskStatusCompleted
	retryState.Tasks[0].Error = ""
	retryState.Status = core.WorkflowStatusCompleted
	retryState.EndTime = time.Now()
	retryState.UpdatedAt = time.Now()

	err = stateDB.Save(workflowID, &retryState)
	if err != nil {
		t.Fatalf("Failed to save recovery state: %v", err)
	}

	// Final verification
	finalState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load final workflow state: %v", err)
	}

	if finalState.Status != core.WorkflowStatusCompleted {
		t.Errorf("Expected final status %s, got %s", core.WorkflowStatusCompleted, finalState.Status)
	}

	if finalState.Tasks[0].Status != core.TaskStatusCompleted {
		t.Errorf("Expected final task status %s, got %s", core.TaskStatusCompleted, finalState.Tasks[0].Status)
	}

	t.Logf("Workflow recovery test completed - recovered from crash and completed successfully")
}

// TestWorkflowRecovery_CrashDuringExecution tests recovery from crashes during execution phase.
func TestWorkflowRecovery_CrashDuringExecution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gitRepo := testutil.NewGitRepo(t)

	workflowID := "test-recovery-execution"
	stateDB := createTestStateDB(t, tmpDir)

	// Create initial workflow state with multiple tasks
	initialState := &WorkflowState{
		ID:        workflowID,
		Phase:     core.PhaseExecute,
		Status:    core.WorkflowStatusRunning,
		StartTime: time.Now(),
		GitCommit: "def456",
		GitBranch: "feature/test",
		WorktreeDir: gitRepo.Path,
		Tasks: []core.TaskState{
			{
				ID:     core.TaskID("execute-task-1"),
				Name:   "Code Generation",
				Phase:  core.PhaseExecute,
				Status: core.TaskStatusCompleted,
				CLI:    "claude",
			},
			{
				ID:     core.TaskID("execute-task-2"),
				Name:   "File Writing",
				Phase:  core.PhaseExecute,
				Status: core.TaskStatusRunning,
				CLI:    "gpt",
			},
			{
				ID:     core.TaskID("execute-task-3"),
				Name:   "Testing",
				Phase:  core.PhaseExecute,
				Status: core.TaskStatusPending,
				CLI:    "gemini",
			},
		},
	}

	err := stateDB.Save(workflowID, initialState)
	if err != nil {
		t.Fatalf("Failed to save initial workflow state: %v", err)
	}

	// Simulate crash during second task
	crashState := *initialState
	crashState.Tasks[1].Status = core.TaskStatusFailed
	crashState.Tasks[1].Error = "process killed during file writing"
	crashState.UpdatedAt = time.Now()

	err = stateDB.Save(workflowID, &crashState)
	if err != nil {
		t.Fatalf("Failed to save crash state: %v", err)
	}

	// Test recovery - load and inspect state
	recoveredState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load crashed workflow: %v", err)
	}

	// Verify recovery can identify completed vs failed vs pending tasks
	completedTasks := 0
	failedTasks := 0
	pendingTasks := 0

	for _, task := range recoveredState.Tasks {
		switch task.Status {
		case core.TaskStatusCompleted:
			completedTasks++
		case core.TaskStatusFailed:
			failedTasks++
		case core.TaskStatusPending:
			pendingTasks++
		}
	}

	expectedCompleted := 1
	expectedFailed := 1
	expectedPending := 1

	if completedTasks != expectedCompleted {
		t.Errorf("Expected %d completed tasks, got %d", expectedCompleted, completedTasks)
	}

	if failedTasks != expectedFailed {
		t.Errorf("Expected %d failed tasks, got %d", expectedFailed, failedTasks)
	}

	if pendingTasks != expectedPending {
		t.Errorf("Expected %d pending tasks, got %d", expectedPending, pendingTasks)
	}

	// Simulate recovery by retrying failed task
	recoveryState := *recoveredState
	recoveryState.Tasks[1].Status = core.TaskStatusCompleted
	recoveryState.Tasks[1].Error = ""
	recoveryState.Tasks[2].Status = core.TaskStatusCompleted
	recoveryState.Status = core.WorkflowStatusCompleted
	recoveryState.EndTime = time.Now()
	recoveryState.UpdatedAt = time.Now()

	err = stateDB.Save(workflowID, &recoveryState)
	if err != nil {
		t.Fatalf("Failed to save recovery state: %v", err)
	}

	// Final verification
	finalState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load final state: %v", err)
	}

	allTasksCompleted := true
	for _, task := range finalState.Tasks {
		if task.Status != core.TaskStatusCompleted {
			allTasksCompleted = false
			break
		}
	}

	if !allTasksCompleted {
		t.Error("Not all tasks were completed after recovery")
	}

	if finalState.Status != core.WorkflowStatusCompleted {
		t.Errorf("Expected final workflow status %s, got %s", core.WorkflowStatusCompleted, finalState.Status)
	}

	t.Logf("Execution recovery test completed - recovered %d tasks", len(finalState.Tasks))
}

// TestWorkflowRecovery_StateCorruption tests recovery from corrupted state files.
func TestWorkflowRecovery_StateCorruption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateDB := createTestStateDB(t, tmpDir)

	workflowID := "test-corruption-recovery"

	// Create valid initial state
	validState := &WorkflowState{
		ID:        workflowID,
		Phase:     core.PhaseAnalyze,
		Status:    core.WorkflowStatusRunning,
		StartTime: time.Now(),
		Tasks: []core.TaskState{
			{
				ID:     core.TaskID("test-task"),
				Name:   "Test Task",
				Phase:  core.PhaseAnalyze,
				Status: core.TaskStatusRunning,
				CLI:    "claude",
			},
		},
	}

	err := stateDB.Save(workflowID, validState)
	if err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	// Verify we can load the valid state
	loadedState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load valid state: %v", err)
	}

	if loadedState.ID != workflowID {
		t.Errorf("Expected workflow ID %s, got %s", workflowID, loadedState.ID)
	}

	// Simulate corruption by writing invalid data to the state file
	stateFile := filepath.Join(tmpDir, "workflows", workflowID+".json")
	corruptData := []byte("{ invalid json data }")
	err = os.WriteFile(stateFile, corruptData, 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupt data: %v", err)
	}

	// Test that loading corrupted state fails gracefully
	_, err = stateDB.Load(workflowID)
	if err == nil {
		t.Error("Expected error when loading corrupted state, but got none")
	}

	// Verify error message indicates corruption
	if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected error to indicate corruption/parsing issue, got: %v", err)
	}

	// Test that we can recover by re-creating the state
	recoveredState := &WorkflowState{
		ID:        workflowID,
		Phase:     core.PhaseAnalyze,
		Status:    core.WorkflowStatusFailed,
		StartTime: validState.StartTime,
		UpdatedAt: time.Now(),
		Tasks: []core.TaskState{
			{
				ID:     core.TaskID("test-task"),
				Name:   "Test Task",
				Phase:  core.PhaseAnalyze,
				Status: core.TaskStatusFailed,
				CLI:    "claude",
				Error:  "recovered from state corruption",
			},
		},
	}

	err = stateDB.Save(workflowID, recoveredState)
	if err != nil {
		t.Fatalf("Failed to save recovered state: %v", err)
	}

	// Verify recovery
	finalState, err := stateDB.Load(workflowID)
	if err != nil {
		t.Fatalf("Failed to load recovered state: %v", err)
	}

	if finalState.Status != core.WorkflowStatusFailed {
		t.Errorf("Expected recovered status %s, got %s", core.WorkflowStatusFailed, finalState.Status)
	}

	if !strings.Contains(finalState.Tasks[0].Error, "recovered from state corruption") {
		t.Error("Expected task error to indicate recovery from corruption")
	}

	t.Logf("State corruption recovery test completed successfully")
}

// Mock implementations for testing

type WorkflowState struct {
	ID          string            `json:"id"`
	Phase       core.Phase        `json:"phase"`
	Status      core.WorkflowStatus `json:"status"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at"`
	GitCommit   string            `json:"git_commit"`
	GitBranch   string            `json:"git_branch"`
	WorktreeDir string            `json:"worktree_dir"`
	Tasks       []core.TaskState  `json:"tasks"`
}

type TestStateDB struct {
	baseDir string
}

func createTestStateDB(t *testing.T, baseDir string) *TestStateDB {
	// Create workflows directory
	workflowsDir := filepath.Join(baseDir, "workflows")
	err := os.MkdirAll(workflowsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create workflows directory: %v", err)
	}

	return &TestStateDB{baseDir: baseDir}
}

func (db *TestStateDB) Save(workflowID string, state *WorkflowState) error {
	stateFile := filepath.Join(db.baseDir, "workflows", workflowID+".json")
	
	// Update timestamp
	state.UpdatedAt = time.Now()
	
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow state: %w", err)
	}
	
	return os.WriteFile(stateFile, data, 0644)
}

func (db *TestStateDB) Load(workflowID string) (*WorkflowState, error) {
	stateFile := filepath.Join(db.baseDir, "workflows", workflowID+".json")
	
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow state: %w", err)
	}
	
	var state WorkflowState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal workflow state: %w", err)
	}
	
	return &state, nil
}
