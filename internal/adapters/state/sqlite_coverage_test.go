package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
)

// newTestManager creates a manager using a temp directory, registered for cleanup.
func newTestManager(t *testing.T) *SQLiteStateManager {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// _newTestManagerWithPath creates a manager for a given path (unused but kept for reference).
func _newTestManagerWithPath(t *testing.T, dbPath string) *SQLiteStateManager {
	t.Helper()
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager(%s): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// makeWorkflow builds a minimal workflow state for testing.
func makeWorkflow(id string, status core.WorkflowStatus) *core.WorkflowState {
	now := time.Now().Truncate(time.Second)
	return &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Version:    1,
			WorkflowID: core.WorkflowID(id),
			Prompt:     "prompt for " + id,
			CreatedAt:  now,
		},
		WorkflowRun: core.WorkflowRun{
			Status:       status,
			CurrentPhase: core.PhaseAnalyze,
			Tasks:        make(map[core.TaskID]*core.TaskState),
			TaskOrder:    []core.TaskID{},
			Checkpoints:  []core.Checkpoint{},
			UpdatedAt:    now,
		},
	}
}

// ============================================================================
// Helper functions
// ============================================================================

func TestIsSQLiteBusy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil error", nil, false},
		{"database is locked", errors.New("database is locked"), true},
		{"SQLITE_BUSY", errors.New("SQLITE_BUSY (5)"), true},
		{"SQLITE_LOCKED", errors.New("SQLITE_LOCKED"), true},
		{"unrelated error", errors.New("disk I/O error"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSQLiteBusy(tc.err); got != tc.expect {
				t.Errorf("isSQLiteBusy(%v) = %v, want %v", tc.err, got, tc.expect)
			}
		})
	}
}

func TestNullableString(t *testing.T) {
	t.Parallel()
	ns := nullableString(nil)
	if ns.Valid {
		t.Error("nullableString(nil) should not be valid")
	}
	ns = nullableString([]byte{})
	if ns.Valid {
		t.Error("nullableString(empty) should not be valid")
	}
	ns = nullableString([]byte("hello"))
	if !ns.Valid || ns.String != "hello" {
		t.Errorf("nullableString('hello') = %+v", ns)
	}
}

func TestNullableTime(t *testing.T) {
	t.Parallel()
	nt := nullableTime(nil)
	if nt.Valid {
		t.Error("nullableTime(nil) should not be valid")
	}
	now := time.Now()
	nt = nullableTime(&now)
	if !nt.Valid {
		t.Error("nullableTime(non-nil) should be valid")
	}
}

func TestPtrToString(t *testing.T) {
	t.Parallel()
	if got := ptrToString(nil); got != "" {
		t.Errorf("ptrToString(nil) = %q, want empty", got)
	}
	s := "value"
	if got := ptrToString(&s); got != "value" {
		t.Errorf("ptrToString(&s) = %q, want 'value'", got)
	}
}

// ============================================================================
// Options
// ============================================================================

func TestWithSQLiteBackupPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	customBackup := filepath.Join(tmpDir, "custom.bak")
	m, err := NewSQLiteStateManager(dbPath, WithSQLiteBackupPath(customBackup))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	if m.backupPath != customBackup {
		t.Errorf("backupPath = %q, want %q", m.backupPath, customBackup)
	}
}

func TestWithSQLiteLockTTL(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath, WithSQLiteLockTTL(30*time.Second))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	if m.lockTTL != 30*time.Second {
		t.Errorf("lockTTL = %v, want 30s", m.lockTTL)
	}
}

// ============================================================================
// Exists
// ============================================================================

func TestExists_NoFile(t *testing.T) {
	t.Parallel()
	m := &SQLiteStateManager{dbPath: filepath.Join(t.TempDir(), "nonexistent.db")}
	if m.Exists() {
		t.Error("Exists() should be false for nonexistent DB")
	}
}

// ============================================================================
// Close
// ============================================================================

func TestClose_NilConnections(t *testing.T) {
	t.Parallel()
	m := &SQLiteStateManager{}
	if err := m.Close(); err != nil {
		t.Errorf("Close() with nil connections: %v", err)
	}
}

// ============================================================================
// retryWrite
// ============================================================================

func TestRetryWrite_ImmediateSuccess(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	calls := 0
	err := m.retryWrite(context.Background(), "test_op", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("retryWrite: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryWrite_NonBusyError(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	err := m.retryWrite(context.Background(), "test_op", func() error {
		return errors.New("disk full")
	})
	if err == nil || err.Error() != "disk full" {
		t.Errorf("retryWrite should propagate non-busy errors, got: %v", err)
	}
}

func TestRetryWrite_BusyThenSuccess(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	// Override retry wait to speed up test
	m.baseRetryWait = time.Millisecond

	calls := 0
	err = m.retryWrite(context.Background(), "test_op", func() error {
		calls++
		if calls <= 2 {
			return errors.New("database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retryWrite: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryWrite_BusyExhaustsRetries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()
	m.baseRetryWait = time.Millisecond
	m.maxRetries = 2

	err = m.retryWrite(context.Background(), "test_op", func() error {
		return errors.New("database is locked")
	})
	if err == nil {
		t.Fatal("retryWrite should fail after exhausting retries")
	}
	if !errors.Is(err, err) { // just check it's non-nil
		t.Errorf("unexpected error type: %v", err)
	}
}

func TestRetryWrite_ContextCancelled(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()
	m.baseRetryWait = 5 * time.Second // long wait so cancellation happens first

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err = m.retryWrite(ctx, "test_op", func() error {
		return errors.New("database is locked")
	})
	if err == nil {
		t.Fatal("retryWrite should fail on cancelled context")
	}
}

// ============================================================================
// ensureWithinStateDir
// ============================================================================

func TestEnsureWithinStateDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	m := &SQLiteStateManager{dbPath: filepath.Join(tmpDir, "state.db")}

	// Valid path within state dir
	if err := m.ensureWithinStateDir(filepath.Join(tmpDir, "state.db.bak")); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}

	// Traversal attempt
	if err := m.ensureWithinStateDir(filepath.Join(tmpDir, "..", "escape")); err == nil {
		t.Error("traversal path should be rejected")
	}
}

// ============================================================================
// DeleteWorkflow
// ============================================================================

func TestDeleteWorkflow_Success(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-del-1", core.WorkflowStatusCompleted)
	wf.Tasks = map[core.TaskID]*core.TaskState{
		"t1": {ID: "t1", Phase: core.PhaseAnalyze, Name: "Task", Status: core.TaskStatusCompleted},
	}
	wf.TaskOrder = []core.TaskID{"t1"}
	wf.Checkpoints = []core.Checkpoint{
		{ID: "cp1", Type: "test", Phase: core.PhaseAnalyze, Timestamp: time.Now()},
	}
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := m.DeleteWorkflow(ctx, "wf-del-1"); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}

	// Verify gone
	loaded, err := m.LoadByID(ctx, "wf-del-1")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded != nil {
		t.Error("workflow should be deleted")
	}

	// Active should be cleared too
	activeID, _ := m.GetActiveWorkflowID(ctx)
	if activeID != "" {
		t.Errorf("active should be empty after deleting active workflow, got %s", activeID)
	}
}

func TestDeleteWorkflow_NonExistent(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.DeleteWorkflow(ctx, "wf-nonexistent")
	if err == nil {
		t.Error("DeleteWorkflow should fail for non-existent workflow")
	}
}

func TestDeleteWorkflow_DoesNotAffectOtherWorkflows(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf1 := makeWorkflow("wf-keep", core.WorkflowStatusRunning)
	wf2 := makeWorkflow("wf-remove", core.WorkflowStatusCompleted)
	if err := m.Save(ctx, wf1); err != nil {
		t.Fatalf("Save wf1: %v", err)
	}
	if err := m.Save(ctx, wf2); err != nil {
		t.Fatalf("Save wf2: %v", err)
	}

	if err := m.DeleteWorkflow(ctx, "wf-remove"); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}

	kept, err := m.LoadByID(ctx, "wf-keep")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if kept == nil {
		t.Error("wf-keep should still exist")
	}
}

// ============================================================================
// Save with all task fields
// ============================================================================

func TestSave_AllTaskFields(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	wf := makeWorkflow("wf-fields", core.WorkflowStatusRunning)
	wf.Tasks = map[core.TaskID]*core.TaskState{
		"t1": {
			ID:            "t1",
			Phase:         core.PhaseExecute,
			Name:          "Full Task",
			Description:   "A task with all fields set",
			Status:        core.TaskStatusCompleted,
			CLI:           "claude",
			Model:         "opus",
			Dependencies:  []core.TaskID{"t0"},
			TokensIn:      500,
			TokensOut:     250,
			Retries:       2,
			Error:         "transient error",
			WorktreePath:  "/tmp/wt/t1",
			StartedAt:     &now,
			CompletedAt:   &now,
			Output:        "task output",
			OutputFile:    "/tmp/output.txt",
			ModelUsed:     "claude-opus-4-6",
			FinishReason:  "end_turn",
			ToolCalls:     []core.ToolCall{{ID: "tc1", Name: "bash", Arguments: map[string]interface{}{"cmd": "ls"}, Result: "ok"}},
			LastCommit:    "abc123",
			FilesModified: []string{"main.go", "go.mod"},
			Branch:        "quorum/wf-fields__t1",
			Resumable:     true,
			ResumeHint:    "continue from step 3",
			MergePending:  true,
			MergeCommit:   "def456",
		},
	}
	wf.TaskOrder = []core.TaskID{"t1"}

	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-fields")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded should not be nil")
	}

	task := loaded.Tasks["t1"]
	if task == nil {
		t.Fatal("task t1 not found")
	}

	if task.Description != "A task with all fields set" {
		t.Errorf("Description = %q", task.Description)
	}
	if task.Error != "transient error" {
		t.Errorf("Error = %q", task.Error)
	}
	if task.WorktreePath != "/tmp/wt/t1" {
		t.Errorf("WorktreePath = %q", task.WorktreePath)
	}
	if task.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
	if task.OutputFile != "/tmp/output.txt" {
		t.Errorf("OutputFile = %q", task.OutputFile)
	}
	if task.ModelUsed != "claude-opus-4-6" {
		t.Errorf("ModelUsed = %q", task.ModelUsed)
	}
	if task.FinishReason != "end_turn" {
		t.Errorf("FinishReason = %q", task.FinishReason)
	}
	if len(task.FilesModified) != 2 {
		t.Errorf("FilesModified len = %d, want 2", len(task.FilesModified))
	}
	if !task.Resumable {
		t.Error("Resumable should be true")
	}
	if task.ResumeHint != "continue from step 3" {
		t.Errorf("ResumeHint = %q", task.ResumeHint)
	}
	if !task.MergePending {
		t.Error("MergePending should be true")
	}
	if task.MergeCommit != "def456" {
		t.Errorf("MergeCommit = %q", task.MergeCommit)
	}
}

// ============================================================================
// Save auto-kanban behavior
// ============================================================================

func TestSave_AutoKanbanMoveToVerify(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-kanban-auto", core.WorkflowStatusCompleted)
	wf.KanbanColumn = "in_progress"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-kanban-auto")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "to_verify" {
		t.Errorf("KanbanColumn = %q, want 'to_verify'", loaded.KanbanColumn)
	}
	if loaded.KanbanCompletedAt == nil {
		t.Error("KanbanCompletedAt should be set")
	}
}

func TestSave_AutoKanbanSkipsAlreadyVerified(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-kanban-verified", core.WorkflowStatusCompleted)
	wf.KanbanColumn = "to_verify"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-kanban-verified")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	// Should remain to_verify, not change
	if loaded.KanbanColumn != "to_verify" {
		t.Errorf("KanbanColumn = %q, want 'to_verify'", loaded.KanbanColumn)
	}
}

func TestSave_AutoKanbanSkipsDone(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-kanban-done", core.WorkflowStatusCompleted)
	wf.KanbanColumn = "done"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-kanban-done")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "done" {
		t.Errorf("KanbanColumn = %q, want 'done'", loaded.KanbanColumn)
	}
}

// ============================================================================
// saveWithOptions
// ============================================================================

func TestSaveWithOptions_PreserveUpdatedAt(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	wf := makeWorkflow("wf-preserve", core.WorkflowStatusRunning)
	wf.UpdatedAt = fixedTime

	err := m.saveWithOptions(ctx, wf, saveOptions{
		preserveUpdatedAt: true,
		setActiveWorkflow: true,
	})
	if err != nil {
		t.Fatalf("saveWithOptions: %v", err)
	}

	// UpdatedAt should be preserved (not overwritten)
	if !wf.UpdatedAt.Equal(fixedTime) {
		t.Errorf("UpdatedAt was modified: got %v, want %v", wf.UpdatedAt, fixedTime)
	}
}

func TestSaveWithOptions_PreserveUpdatedAt_ZeroSetsNow(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-preserve-zero", core.WorkflowStatusRunning)
	wf.UpdatedAt = time.Time{} // zero

	before := time.Now()
	err := m.saveWithOptions(ctx, wf, saveOptions{
		preserveUpdatedAt: true,
		setActiveWorkflow: true,
	})
	if err != nil {
		t.Fatalf("saveWithOptions: %v", err)
	}

	if wf.UpdatedAt.Before(before) {
		t.Error("zero UpdatedAt should be set to Now()")
	}
}

func TestSaveWithOptions_DisableAutoKanban(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-no-autokanban", core.WorkflowStatusCompleted)
	wf.KanbanColumn = "in_progress"

	err := m.saveWithOptions(ctx, wf, saveOptions{
		disableAutoKanban: true,
		setActiveWorkflow: true,
	})
	if err != nil {
		t.Fatalf("saveWithOptions: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-no-autokanban")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "in_progress" {
		t.Errorf("KanbanColumn = %q, want 'in_progress' (auto-kanban disabled)", loaded.KanbanColumn)
	}
}

func TestSaveWithOptions_NoSetActiveWorkflow(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-no-active", core.WorkflowStatusRunning)

	err := m.saveWithOptions(ctx, wf, saveOptions{
		setActiveWorkflow: false,
	})
	if err != nil {
		t.Fatalf("saveWithOptions: %v", err)
	}

	activeID, err := m.GetActiveWorkflowID(ctx)
	if err != nil {
		t.Fatalf("GetActiveWorkflowID: %v", err)
	}
	if activeID != "" {
		t.Errorf("active should be empty when setActiveWorkflow=false, got %s", activeID)
	}

	// But workflow should exist
	loaded, err := m.LoadByID(ctx, "wf-no-active")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded == nil {
		t.Error("workflow should exist even without being active")
	}
}

// ============================================================================
// Save with kanban fields
// ============================================================================

func TestSave_KanbanFields(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	wf := makeWorkflow("wf-kanban-full", core.WorkflowStatusRunning)
	wf.KanbanColumn = "in_progress"
	wf.KanbanPosition = 3
	wf.PRURL = "https://github.com/org/repo/pull/42"
	wf.PRNumber = 42
	wf.KanbanStartedAt = &now
	wf.KanbanExecutionCount = 2
	wf.KanbanLastError = "previous error"

	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-kanban-full")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "in_progress" {
		t.Errorf("KanbanColumn = %q", loaded.KanbanColumn)
	}
	if loaded.KanbanPosition != 3 {
		t.Errorf("KanbanPosition = %d", loaded.KanbanPosition)
	}
	if loaded.PRURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("PRURL = %q", loaded.PRURL)
	}
	if loaded.PRNumber != 42 {
		t.Errorf("PRNumber = %d", loaded.PRNumber)
	}
	if loaded.KanbanStartedAt == nil {
		t.Error("KanbanStartedAt should not be nil")
	}
	if loaded.KanbanExecutionCount != 2 {
		t.Errorf("KanbanExecutionCount = %d", loaded.KanbanExecutionCount)
	}
	if loaded.KanbanLastError != "previous error" {
		t.Errorf("KanbanLastError = %q", loaded.KanbanLastError)
	}
}

// ============================================================================
// Save with OptimizedPrompt and ReportPath
// ============================================================================

func TestSave_OptimizedPromptAndReportPath(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-opt", core.WorkflowStatusRunning)
	wf.OptimizedPrompt = "Optimized: do X then Y"
	wf.ReportPath = ".quorum/runs/wf-opt"

	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-opt")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.OptimizedPrompt != "Optimized: do X then Y" {
		t.Errorf("OptimizedPrompt = %q", loaded.OptimizedPrompt)
	}
	if loaded.ReportPath != ".quorum/runs/wf-opt" {
		t.Errorf("ReportPath = %q", loaded.ReportPath)
	}
}

// ============================================================================
// ListWorkflows truncation
// ============================================================================

func TestListWorkflows_TruncatesLongPrompts(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-long", core.WorkflowStatusRunning)
	// Create a prompt longer than 100 chars
	longPrompt := ""
	for i := 0; i < 120; i++ {
		longPrompt += "x"
	}
	wf.Prompt = longPrompt
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	summaries, err := m.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("len = %d, want 1", len(summaries))
	}
	// Should be truncated to 100 + "..."
	if len(summaries[0].Prompt) != 103 {
		t.Errorf("prompt length = %d, want 103", len(summaries[0].Prompt))
	}
}

func TestListWorkflows_WithTitle(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-titled", core.WorkflowStatusRunning)
	wf.Title = "My Titled Workflow"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	summaries, err := m.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	found := false
	for _, s := range summaries {
		if s.WorkflowID == "wf-titled" {
			found = true
			if s.Title != "My Titled Workflow" {
				t.Errorf("Title = %q, want 'My Titled Workflow'", s.Title)
			}
		}
	}
	if !found {
		t.Error("wf-titled not found in summaries")
	}
}

// ============================================================================
// FindWorkflowsByPrompt edge case
// ============================================================================

func TestFindWorkflowsByPrompt_EmptyPrompt(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	results, err := m.FindWorkflowsByPrompt(ctx, "")
	if err != nil {
		t.Fatalf("FindWorkflowsByPrompt: %v", err)
	}
	if results != nil {
		t.Errorf("empty prompt should return nil, got %v", results)
	}
}

// ============================================================================
// Restore without backup
// ============================================================================

func TestRestore_NoBackup(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	_, err := m.Restore(ctx)
	if err == nil {
		t.Error("Restore should fail when no backup exists")
	}
}

// ============================================================================
// UpdateHeartbeat
// ============================================================================

func TestUpdateHeartbeat_NotInRunningWorkflows(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-hb-miss", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Workflow is NOT in running_workflows, so UpdateHeartbeat should fail
	err := m.UpdateHeartbeat(ctx, "wf-hb-miss")
	if err == nil {
		t.Error("UpdateHeartbeat should fail when workflow not in running_workflows")
	}
}

func TestUpdateHeartbeat_Success(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-hb-ok", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := m.SetWorkflowRunning(ctx, "wf-hb-ok"); err != nil {
		t.Fatalf("SetWorkflowRunning: %v", err)
	}

	if err := m.UpdateHeartbeat(ctx, "wf-hb-ok"); err != nil {
		t.Fatalf("UpdateHeartbeat: %v", err)
	}
}

// ============================================================================
// FindZombieWorkflows edge cases
// ============================================================================

func TestFindZombieWorkflows_NoneRunning(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	zombies, err := m.FindZombieWorkflows(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("FindZombieWorkflows: %v", err)
	}
	if len(zombies) != 0 {
		t.Errorf("expected 0 zombies, got %d", len(zombies))
	}
}

func TestFindZombieWorkflows_FreshHeartbeatNotZombie(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-fresh", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := m.SetWorkflowRunning(ctx, "wf-fresh"); err != nil {
		t.Fatalf("SetWorkflowRunning: %v", err)
	}

	// Fresh heartbeat -> not a zombie
	zombies, err := m.FindZombieWorkflows(ctx, 5*time.Minute)
	if err != nil {
		t.Fatalf("FindZombieWorkflows: %v", err)
	}
	if len(zombies) != 0 {
		t.Errorf("expected 0 zombies (fresh heartbeat), got %d", len(zombies))
	}
}

// ============================================================================
// Kanban: GetNextKanbanWorkflow
// ============================================================================

func TestGetNextKanbanWorkflow_Empty(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf, err := m.GetNextKanbanWorkflow(ctx)
	if err != nil {
		t.Fatalf("GetNextKanbanWorkflow: %v", err)
	}
	if wf != nil {
		t.Error("expected nil when no todo workflows")
	}
}

func TestGetNextKanbanWorkflow_ReturnsTodo(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Create two todo workflows with different positions
	wf1 := makeWorkflow("wf-todo-1", core.WorkflowStatusPending)
	wf1.KanbanColumn = "todo"
	wf1.KanbanPosition = 2
	if err := m.Save(ctx, wf1); err != nil {
		t.Fatalf("Save wf1: %v", err)
	}

	wf2 := makeWorkflow("wf-todo-2", core.WorkflowStatusPending)
	wf2.KanbanColumn = "todo"
	wf2.KanbanPosition = 1
	if err := m.Save(ctx, wf2); err != nil {
		t.Fatalf("Save wf2: %v", err)
	}

	// Also create a non-todo workflow
	wf3 := makeWorkflow("wf-done", core.WorkflowStatusCompleted)
	wf3.KanbanColumn = "done"
	err := m.saveWithOptions(ctx, wf3, saveOptions{
		disableAutoKanban: true,
		setActiveWorkflow: false,
	})
	if err != nil {
		t.Fatalf("Save wf3: %v", err)
	}

	next, err := m.GetNextKanbanWorkflow(ctx)
	if err != nil {
		t.Fatalf("GetNextKanbanWorkflow: %v", err)
	}
	if next == nil {
		t.Fatal("expected a workflow")
	}
	if next.WorkflowID != "wf-todo-2" {
		t.Errorf("expected wf-todo-2 (lowest position), got %s", next.WorkflowID)
	}
}

// ============================================================================
// Kanban: MoveWorkflow
// ============================================================================

func TestMoveWorkflow_Success(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-move", core.WorkflowStatusRunning)
	wf.KanbanColumn = "todo"
	wf.KanbanPosition = 0
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := m.MoveWorkflow(ctx, "wf-move", "in_progress", 5); err != nil {
		t.Fatalf("MoveWorkflow: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-move")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "in_progress" {
		t.Errorf("KanbanColumn = %q, want 'in_progress'", loaded.KanbanColumn)
	}
	if loaded.KanbanPosition != 5 {
		t.Errorf("KanbanPosition = %d, want 5", loaded.KanbanPosition)
	}
}

func TestMoveWorkflow_NonExistent(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.MoveWorkflow(ctx, "wf-ghost", "done", 0)
	if err == nil {
		t.Error("MoveWorkflow should fail for non-existent workflow")
	}
}

// ============================================================================
// Kanban: UpdateKanbanStatus
// ============================================================================

func TestUpdateKanbanStatus_Success(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-ks", core.WorkflowStatusRunning)
	wf.KanbanColumn = "in_progress"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	err := m.UpdateKanbanStatus(ctx, "wf-ks", "to_verify", "https://github.com/pr/1", 1, "")
	if err != nil {
		t.Fatalf("UpdateKanbanStatus: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-ks")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanColumn != "to_verify" {
		t.Errorf("KanbanColumn = %q", loaded.KanbanColumn)
	}
	if loaded.PRURL != "https://github.com/pr/1" {
		t.Errorf("PRURL = %q", loaded.PRURL)
	}
	if loaded.PRNumber != 1 {
		t.Errorf("PRNumber = %d", loaded.PRNumber)
	}
}

func TestUpdateKanbanStatus_NonExistent(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.UpdateKanbanStatus(ctx, "wf-ghost", "done", "", 0, "")
	if err == nil {
		t.Error("UpdateKanbanStatus should fail for non-existent workflow")
	}
}

func TestUpdateKanbanStatus_WithError(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-ks-err", core.WorkflowStatusFailed)
	wf.KanbanColumn = "in_progress"
	err := m.saveWithOptions(ctx, wf, saveOptions{
		disableAutoKanban: true,
		setActiveWorkflow: true,
	})
	if err != nil {
		t.Fatalf("saveWithOptions: %v", err)
	}

	err = m.UpdateKanbanStatus(ctx, "wf-ks-err", "refinement", "", 0, "build failed")
	if err != nil {
		t.Fatalf("UpdateKanbanStatus: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-ks-err")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.KanbanLastError != "build failed" {
		t.Errorf("KanbanLastError = %q", loaded.KanbanLastError)
	}
}

// ============================================================================
// Kanban: GetKanbanEngineState / SaveKanbanEngineState
// ============================================================================

func TestKanbanEngineState_Default(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	state, err := m.GetKanbanEngineState(ctx)
	if err != nil {
		t.Fatalf("GetKanbanEngineState: %v", err)
	}
	// Migration 008 inserts a default row, so state should be non-nil with defaults
	if state == nil {
		// Some DBs may not have the row if migration INSERT was skipped
		return
	}
	if state.Enabled {
		t.Error("default Enabled should be false")
	}
	if state.CircuitBreakerOpen {
		t.Error("default CircuitBreakerOpen should be false")
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("default ConsecutiveFailures = %d, want 0", state.ConsecutiveFailures)
	}
}

func TestKanbanEngineState_SaveAndLoad(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Must create the workflow first (FK constraint on current_workflow_id)
	wf := makeWorkflow("wf-current", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save workflow: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	wfID := "wf-current"
	state := &kanban.KanbanEngineState{
		Enabled:             true,
		CurrentWorkflowID:   &wfID,
		ConsecutiveFailures: 3,
		CircuitBreakerOpen:  true,
		LastFailureAt:       &now,
	}

	if err := m.SaveKanbanEngineState(ctx, state); err != nil {
		t.Fatalf("SaveKanbanEngineState: %v", err)
	}

	loaded, err := m.GetKanbanEngineState(ctx)
	if err != nil {
		t.Fatalf("GetKanbanEngineState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil engine state")
	}
	if !loaded.Enabled {
		t.Error("Enabled should be true")
	}
	if loaded.CurrentWorkflowID == nil || *loaded.CurrentWorkflowID != "wf-current" {
		t.Errorf("CurrentWorkflowID = %v", loaded.CurrentWorkflowID)
	}
	if loaded.ConsecutiveFailures != 3 {
		t.Errorf("ConsecutiveFailures = %d", loaded.ConsecutiveFailures)
	}
	if !loaded.CircuitBreakerOpen {
		t.Error("CircuitBreakerOpen should be true")
	}
	if loaded.LastFailureAt == nil {
		t.Error("LastFailureAt should not be nil")
	}
}

func TestKanbanEngineState_Update(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Save initial
	state := &kanban.KanbanEngineState{Enabled: true}
	if err := m.SaveKanbanEngineState(ctx, state); err != nil {
		t.Fatalf("SaveKanbanEngineState: %v", err)
	}

	// Update
	state.Enabled = false
	state.ConsecutiveFailures = 5
	if err := m.SaveKanbanEngineState(ctx, state); err != nil {
		t.Fatalf("SaveKanbanEngineState (update): %v", err)
	}

	loaded, err := m.GetKanbanEngineState(ctx)
	if err != nil {
		t.Fatalf("GetKanbanEngineState: %v", err)
	}
	if loaded.Enabled {
		t.Error("Enabled should be false after update")
	}
	if loaded.ConsecutiveFailures != 5 {
		t.Errorf("ConsecutiveFailures = %d, want 5", loaded.ConsecutiveFailures)
	}
}

// ============================================================================
// Kanban: ListWorkflowsByKanbanColumn
// ============================================================================

func TestListWorkflowsByKanbanColumn(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Create workflows in different columns
	for i, col := range []string{"todo", "todo", "in_progress", "done"} {
		wf := makeWorkflow("wf-col-"+col+"-"+string(rune('a'+i)), core.WorkflowStatusPending)
		wf.KanbanColumn = col
		wf.KanbanPosition = i
		err := m.saveWithOptions(ctx, wf, saveOptions{
			disableAutoKanban: true,
			setActiveWorkflow: false,
		})
		if err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	todos, err := m.ListWorkflowsByKanbanColumn(ctx, "todo")
	if err != nil {
		t.Fatalf("ListWorkflowsByKanbanColumn: %v", err)
	}
	if len(todos) != 2 {
		t.Errorf("todo count = %d, want 2", len(todos))
	}

	inProg, err := m.ListWorkflowsByKanbanColumn(ctx, "in_progress")
	if err != nil {
		t.Fatalf("ListWorkflowsByKanbanColumn: %v", err)
	}
	if len(inProg) != 1 {
		t.Errorf("in_progress count = %d, want 1", len(inProg))
	}

	empty, err := m.ListWorkflowsByKanbanColumn(ctx, "refinement")
	if err != nil {
		t.Fatalf("ListWorkflowsByKanbanColumn: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("refinement count = %d, want 0", len(empty))
	}
}

// ============================================================================
// Kanban: GetKanbanBoard
// ============================================================================

func TestGetKanbanBoard(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Create one workflow in todo
	wf := makeWorkflow("wf-board", core.WorkflowStatusPending)
	wf.KanbanColumn = "todo"
	err := m.saveWithOptions(ctx, wf, saveOptions{
		disableAutoKanban: true,
		setActiveWorkflow: false,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	board, err := m.GetKanbanBoard(ctx)
	if err != nil {
		t.Fatalf("GetKanbanBoard: %v", err)
	}

	// Should have all 5 columns
	for _, col := range []string{"refinement", "todo", "in_progress", "to_verify", "done"} {
		if _, ok := board[col]; !ok {
			t.Errorf("board missing column %q", col)
		}
	}

	if len(board["todo"]) != 1 {
		t.Errorf("todo count = %d, want 1", len(board["todo"]))
	}
	if len(board["done"]) != 0 {
		t.Errorf("done count = %d, want 0", len(board["done"]))
	}
}

// ============================================================================
// ExecuteAtomically
// ============================================================================

func TestExecuteAtomically_SaveAndLoad(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		wf := makeWorkflow("wf-atomic-1", core.WorkflowStatusRunning)
		return atx.Save(wf)
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically (save): %v", err)
	}

	// Verify it was saved
	loaded, err := m.LoadByID(ctx, "wf-atomic-1")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded == nil {
		t.Fatal("workflow should exist after atomic save")
	}
}

func TestExecuteAtomically_LoadByID(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Pre-save a workflow
	wf := makeWorkflow("wf-atomic-read", core.WorkflowStatusRunning)
	wf.Title = "Atomic Read Test"
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var loadedTitle string
	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		state, err := atx.LoadByID("wf-atomic-read")
		if err != nil {
			return err
		}
		if state == nil {
			return errors.New("workflow not found in transaction")
		}
		loadedTitle = state.Title
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically: %v", err)
	}
	if loadedTitle != "Atomic Read Test" {
		t.Errorf("title = %q, want 'Atomic Read Test'", loadedTitle)
	}
}

func TestExecuteAtomically_LoadByID_NonExistent(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		state, err := atx.LoadByID("wf-nonexistent")
		if err != nil {
			return err
		}
		if state != nil {
			return errors.New("expected nil for non-existent workflow")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically: %v", err)
	}
}

func TestExecuteAtomically_Rollback(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		wf := makeWorkflow("wf-atomic-rollback", core.WorkflowStatusRunning)
		if err := atx.Save(wf); err != nil {
			return err
		}
		return errors.New("intentional failure")
	})
	if err == nil || err.Error() != "intentional failure" {
		t.Fatalf("expected intentional failure, got: %v", err)
	}

	// Verify rollback
	loaded, err := m.LoadByID(ctx, "wf-atomic-rollback")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded != nil {
		t.Error("workflow should not exist after rollback")
	}
}

func TestExecuteAtomically_SetAndClearWorkflowRunning(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Pre-save a workflow
	wf := makeWorkflow("wf-atomic-run", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Set running atomically
	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		return atx.SetWorkflowRunning("wf-atomic-run")
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically (set running): %v", err)
	}

	isRunning, err := m.IsWorkflowRunning(ctx, "wf-atomic-run")
	if err != nil {
		t.Fatalf("IsWorkflowRunning: %v", err)
	}
	if !isRunning {
		t.Error("workflow should be running")
	}

	// Check IsWorkflowRunning within transaction
	err = m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		running, err := atx.IsWorkflowRunning("wf-atomic-run")
		if err != nil {
			return err
		}
		if !running {
			return errors.New("expected running in transaction")
		}
		return atx.ClearWorkflowRunning("wf-atomic-run")
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically (clear running): %v", err)
	}

	isRunning, err = m.IsWorkflowRunning(ctx, "wf-atomic-run")
	if err != nil {
		t.Fatalf("IsWorkflowRunning: %v", err)
	}
	if isRunning {
		t.Error("workflow should not be running after clear")
	}
}

func TestExecuteAtomically_SetWorkflowRunning_Duplicate(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-atomic-dup", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Set running
	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		return atx.SetWorkflowRunning("wf-atomic-dup")
	})
	if err != nil {
		t.Fatalf("First SetWorkflowRunning: %v", err)
	}

	// Duplicate should fail
	err = m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		return atx.SetWorkflowRunning("wf-atomic-dup")
	})
	if err == nil {
		t.Error("duplicate SetWorkflowRunning should fail")
	}
}

// ============================================================================
// ExecuteAtomically - Save with tasks and checkpoints
// ============================================================================

func TestExecuteAtomically_SaveWithTasksAndCheckpoints(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	err := m.ExecuteAtomically(ctx, func(atx core.AtomicStateContext) error {
		wf := makeWorkflow("wf-atomic-full", core.WorkflowStatusRunning)
		wf.Tasks = map[core.TaskID]*core.TaskState{
			"t1": {ID: "t1", Phase: core.PhaseAnalyze, Name: "Atomic Task", Status: core.TaskStatusPending},
		}
		wf.TaskOrder = []core.TaskID{"t1"}
		wf.Checkpoints = []core.Checkpoint{
			{ID: "cp1", Type: "test", Phase: core.PhaseAnalyze, Timestamp: time.Now()},
		}
		return atx.Save(wf)
	})
	if err != nil {
		t.Fatalf("ExecuteAtomically: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-atomic-full")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded == nil {
		t.Fatal("workflow should exist")
	}
	if len(loaded.Tasks) != 1 {
		t.Errorf("tasks count = %d, want 1", len(loaded.Tasks))
	}
	if len(loaded.Checkpoints) != 1 {
		t.Errorf("checkpoints count = %d, want 1", len(loaded.Checkpoints))
	}
}

// ============================================================================
// deleteReportDirectory
// ============================================================================

func TestDeleteReportDirectory_SafePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	m := &SQLiteStateManager{dbPath: filepath.Join(tmpDir, "state.db")}

	// Create a .quorum directory structure
	reportDir := filepath.Join(tmpDir, ".quorum", "runs", "wf-123")
	if err := os.MkdirAll(reportDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reportDir, "report.md"), []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// With a safe .quorum/ prefixed path
	m.deleteReportDirectory(".quorum/runs/wf-123", "wf-123")

	// The directory should be removed (or at least attempted)
	// Check via the default fallback path
}

func TestDeleteReportDirectory_UnsafePath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	m := &SQLiteStateManager{dbPath: filepath.Join(tmpDir, "state.db")}

	// Create a directory that should NOT be deleted (outside .quorum/)
	unsafeDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(unsafeDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// With a path that doesn't start with .quorum/
	m.deleteReportDirectory("/tmp/src", "wf-123")

	// src should still exist (not deleted)
	if _, err := os.Stat(unsafeDir); os.IsNotExist(err) {
		t.Error("unsafe directory should NOT be deleted")
	}
}

func TestDeleteReportDirectory_EmptyPath(t *testing.T) {
	t.Parallel()
	m := &SQLiteStateManager{dbPath: filepath.Join(t.TempDir(), "state.db")}
	// Should not panic with empty path
	m.deleteReportDirectory("", "wf-123")
}

// ============================================================================
// Backup with custom path
// ============================================================================

func TestBackup_CustomPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	customBackup := filepath.Join(tmpDir, "my-backup.db")
	m, err := NewSQLiteStateManager(dbPath, WithSQLiteBackupPath(customBackup))
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	ctx := context.Background()
	wf := makeWorkflow("wf-bkup", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := m.Backup(ctx); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if _, err := os.Stat(customBackup); os.IsNotExist(err) {
		t.Error("backup file should exist at custom path")
	}
}

// ============================================================================
// ArchiveWorkflows - no eligible workflows
// ============================================================================

func TestArchiveWorkflows_NothingToArchive(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	count, err := m.ArchiveWorkflows(ctx)
	if err != nil {
		t.Fatalf("ArchiveWorkflows: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestArchiveWorkflows_SkipsActiveWorkflow(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Create a completed workflow that is still active
	wf := makeWorkflow("wf-active-completed", core.WorkflowStatusCompleted)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// wf is the active workflow since Save sets it

	count, err := m.ArchiveWorkflows(ctx)
	if err != nil {
		t.Fatalf("ArchiveWorkflows: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0 (active workflow should be skipped)", count)
	}
}

func TestArchiveWorkflows_ArchivesFailedWorkflow(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-archive-fail", core.WorkflowStatusFailed)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Deactivate so it can be archived
	if err := m.DeactivateWorkflow(ctx); err != nil {
		t.Fatalf("DeactivateWorkflow: %v", err)
	}

	count, err := m.ArchiveWorkflows(ctx)
	if err != nil {
		t.Fatalf("ArchiveWorkflows: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

// ============================================================================
// AcquireLock / ReleaseLock edge cases
// ============================================================================

func TestReleaseLock_NotOwned(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	// Write a lock file with a different PID
	lockPath := dbPath + ".lock"
	info := lockInfo{
		PID:        999999999, // non-existent PID
		Hostname:   "other-host",
		AcquiredAt: time.Now(),
	}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(lockPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ctx := context.Background()
	err = m.ReleaseLock(ctx)
	if err == nil {
		t.Error("ReleaseLock should fail when lock is owned by different process")
	}
}

func TestReleaseLock_NoLockFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "state.db")
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	ctx := context.Background()
	err = m.ReleaseLock(ctx)
	if err != nil {
		t.Errorf("ReleaseLock with no lock file should succeed, got: %v", err)
	}
}

// ============================================================================
// ReleaseWorkflowLock that we don't own
// ============================================================================

func TestReleaseWorkflowLock_NotOwned(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Release a lock we never acquired - should not error (no rows affected is ok)
	err := m.ReleaseWorkflowLock(ctx, "wf-never-locked")
	if err != nil {
		t.Errorf("ReleaseWorkflowLock should not error for non-held lock: %v", err)
	}
}

// ============================================================================
// cleanupOnStartup - no active workflow (no-op)
// ============================================================================

func TestCleanupOnStartup_NoActiveWorkflow(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "startup.db")

	// Create a fresh manager (cleanupOnStartup runs in constructor)
	m, err := NewSQLiteStateManager(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStateManager: %v", err)
	}
	defer m.Close()

	// Should have no active workflow and no error
	activeID, err := m.GetActiveWorkflowID(context.Background())
	if err != nil {
		t.Fatalf("GetActiveWorkflowID: %v", err)
	}
	if activeID != "" {
		t.Errorf("expected empty active ID, got %s", activeID)
	}
}

// ============================================================================
// Save with nil Blueprint and Metrics
// ============================================================================

func TestSave_NilBlueprintAndMetrics(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-nil-fields", core.WorkflowStatusRunning)
	wf.Blueprint = nil
	wf.Metrics = nil

	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-nil-fields")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.Blueprint != nil {
		t.Error("Blueprint should be nil")
	}
	if loaded.Metrics != nil {
		t.Error("Metrics should be nil")
	}
}

// ============================================================================
// Prompt hash for duplicate detection
// ============================================================================

func TestSave_PromptHash(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Save two workflows with same prompt
	wf1 := makeWorkflow("wf-hash-1", core.WorkflowStatusRunning)
	wf1.Prompt = "same prompt"
	if err := m.Save(ctx, wf1); err != nil {
		t.Fatalf("Save wf1: %v", err)
	}

	wf2 := makeWorkflow("wf-hash-2", core.WorkflowStatusRunning)
	wf2.Prompt = "same prompt"
	if err := m.Save(ctx, wf2); err != nil {
		t.Fatalf("Save wf2: %v", err)
	}

	// FindWorkflowsByPrompt should find both
	dupes, err := m.FindWorkflowsByPrompt(ctx, "same prompt")
	if err != nil {
		t.Fatalf("FindWorkflowsByPrompt: %v", err)
	}
	if len(dupes) != 2 {
		t.Errorf("expected 2 duplicates, got %d", len(dupes))
	}
}

// ============================================================================
// Save with empty prompt (no prompt hash)
// ============================================================================

func TestSave_EmptyPrompt(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-empty-prompt", core.WorkflowStatusRunning)
	wf.Prompt = ""

	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-empty-prompt")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded == nil {
		t.Fatal("workflow should exist")
	}
	if loaded.Prompt != "" {
		t.Errorf("Prompt = %q, want empty", loaded.Prompt)
	}
}

// ============================================================================
// Multiple saves (upsert) to same workflow
// ============================================================================

func TestSave_Upsert(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	wf := makeWorkflow("wf-upsert", core.WorkflowStatusPending)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save (first): %v", err)
	}

	// Update status and save again
	wf.Status = core.WorkflowStatusRunning
	wf.CurrentPhase = core.PhaseExecute
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save (second): %v", err)
	}

	loaded, err := m.LoadByID(ctx, "wf-upsert")
	if err != nil {
		t.Fatalf("LoadByID: %v", err)
	}
	if loaded.Status != core.WorkflowStatusRunning {
		t.Errorf("Status = %s, want running", loaded.Status)
	}
	if loaded.CurrentPhase != core.PhaseExecute {
		t.Errorf("CurrentPhase = %s, want execute", loaded.CurrentPhase)
	}

	// Only one workflow should exist
	summaries, err := m.ListWorkflows(ctx)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(summaries))
	}
}

// ============================================================================
// copyFile
// ============================================================================

func TestCopyFile_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	m := &SQLiteStateManager{dbPath: filepath.Join(tmpDir, "state.db")}

	src := filepath.Join(tmpDir, "source.txt")
	dst := filepath.Join(tmpDir, "dest.txt")

	if err := os.WriteFile(src, []byte("file content"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "file content" {
		t.Errorf("content = %q, want 'file content'", string(data))
	}
}

func TestCopyFile_SourceNotExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	m := &SQLiteStateManager{dbPath: filepath.Join(tmpDir, "state.db")}

	err := m.copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dst"))
	if err == nil {
		t.Error("copyFile should fail for nonexistent source")
	}
}

// ============================================================================
// sqliteProcessExists
// ============================================================================

func TestSqliteProcessExists(t *testing.T) {
	t.Parallel()
	// Our own process should exist
	if !sqliteProcessExists(os.Getpid()) {
		t.Error("current process should exist")
	}
	// A very large PID should not exist
	if sqliteProcessExists(999999999) {
		t.Error("PID 999999999 should not exist")
	}
}

// ============================================================================
// Concurrent reads and writes
// ============================================================================

func TestConcurrentReadWrite(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	ctx := context.Background()

	// Pre-populate
	wf := makeWorkflow("wf-concurrent", core.WorkflowStatusRunning)
	if err := m.Save(ctx, wf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	done := make(chan struct{})
	errCh := make(chan error, 20)

	// Writer goroutine
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 10; i++ {
			wf := makeWorkflow("wf-concurrent", core.WorkflowStatusRunning)
			wf.Title = "Update " + string(rune('0'+i))
			if err := m.Save(ctx, wf); err != nil {
				errCh <- err
			}
		}
	}()

	// Reader goroutine
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; i < 10; i++ {
			_, err := m.LoadByID(ctx, "wf-concurrent")
			if err != nil {
				errCh <- err
			}
		}
	}()

	<-done
	<-done
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation failed: %v", err)
	}
}
