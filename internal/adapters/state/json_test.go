package state

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func newTestState() *core.WorkflowState {
	now := time.Now()
	return &core.WorkflowState{
		Version:      1,
		WorkflowID:   "wf-test-123",
		Status:       core.WorkflowStatusRunning,
		CurrentPhase: core.PhaseAnalyze,
		Prompt:       "Test workflow prompt",
		Tasks: map[core.TaskID]*core.TaskState{
			"task-1": {
				ID:     "task-1",
				Phase:  core.PhaseAnalyze,
				Name:   "Test Task",
				Status: core.TaskStatusPending,
				CLI:    "claude",
			},
		},
		TaskOrder: []core.TaskID{"task-1"},
		Config: &core.WorkflowConfig{
			ConsensusThreshold: 0.75,
			MaxRetries:         3,
			Timeout:            time.Hour,
		},
		Metrics: &core.StateMetrics{
			TotalCostUSD:   0.05,
			TotalTokensIn:  1000,
			TotalTokensOut: 500,
		},
		Checkpoints: []core.Checkpoint{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestJSONStateManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	state := newTestState()

	ctx := context.Background()
	err := manager.Save(ctx, state)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if !manager.Exists() {
		t.Error("state file should exist after save")
	}

	// Verify file contents
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var envelope stateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if envelope.Version != 1 {
		t.Errorf("Version = %d, want 1", envelope.Version)
	}
	if envelope.Checksum == "" {
		t.Error("Checksum should not be empty")
	}
	if envelope.State.WorkflowID != "wf-test-123" {
		t.Errorf("WorkflowID = %s, want wf-test-123", envelope.State.WorkflowID)
	}
}

func TestJSONStateManager_Load(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	originalState := newTestState()

	ctx := context.Background()

	// Save first
	if err := manager.Save(ctx, originalState); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loadedState, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loadedState.WorkflowID != originalState.WorkflowID {
		t.Errorf("WorkflowID = %s, want %s", loadedState.WorkflowID, originalState.WorkflowID)
	}
	if loadedState.Status != originalState.Status {
		t.Errorf("Status = %s, want %s", loadedState.Status, originalState.Status)
	}
	if loadedState.Prompt != originalState.Prompt {
		t.Errorf("Prompt = %s, want %s", loadedState.Prompt, originalState.Prompt)
	}
}

func TestJSONStateManager_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "nonexistent.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	state, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for non-existent state", err)
	}
	if state != nil {
		t.Error("state should be nil for non-existent file")
	}
}

func TestJSONStateManager_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	state := newTestState()
	ctx := context.Background()

	// Save multiple times to test atomic writes
	for i := 0; i < 5; i++ {
		state.WorkflowID = core.WorkflowID("wf-" + string(rune('a'+i)))
		if err := manager.Save(ctx, state); err != nil {
			t.Fatalf("Save() iteration %d error = %v", i, err)
		}

		// Verify immediately after save
		loaded, err := manager.Load(ctx)
		if err != nil {
			t.Fatalf("Load() iteration %d error = %v", i, err)
		}
		if loaded.WorkflowID != state.WorkflowID {
			t.Errorf("iteration %d: WorkflowID = %s, want %s", i, loaded.WorkflowID, state.WorkflowID)
		}
	}
}

func TestJSONStateManager_ChecksumVerification(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	state := newTestState()
	ctx := context.Background()

	// Save
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Corrupt the file
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var envelope stateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Modify state but keep old checksum
	envelope.State.Prompt = "CORRUPTED"
	corruptedData, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if err := os.WriteFile(statePath, corruptedData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should fail with checksum error
	_, err = manager.Load(ctx)
	if err == nil {
		t.Error("Load() should fail with corrupted checksum")
	}
}

func TestJSONStateManager_BackupRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Save first state
	state1 := newTestState()
	state1.WorkflowID = "wf-first"
	if err := manager.Save(ctx, state1); err != nil {
		t.Fatalf("Save() first error = %v", err)
	}

	// Save second state (creates backup)
	state2 := newTestState()
	state2.WorkflowID = "wf-second"
	if err := manager.Save(ctx, state2); err != nil {
		t.Fatalf("Save() second error = %v", err)
	}

	// Verify backup exists
	backupPath := manager.BackupPath()
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup file should exist")
	}

	// Restore from backup
	restored, err := manager.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	if restored.WorkflowID != "wf-first" {
		t.Errorf("Restored WorkflowID = %s, want wf-first", restored.WorkflowID)
	}
}

func TestJSONStateManager_Lock(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Acquire lock
	if err := manager.AcquireLock(ctx); err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	// Try to acquire again (should fail from same process - but we're the same PID)
	// Actually, the implementation checks for existing lock, so we need another approach
	// For this test, just verify lock file exists
	lockPath := statePath + ".lock"
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist after AcquireLock")
	}

	// Release lock
	if err := manager.ReleaseLock(ctx); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should not exist after ReleaseLock")
	}
}

func TestJSONStateManager_ReleaseLockIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Release lock when not acquired should not error
	if err := manager.ReleaseLock(ctx); err != nil {
		t.Errorf("ReleaseLock() without lock error = %v", err)
	}
}

func TestJSONStateManager_StaleLock(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create manager with very short TTL
	manager := NewJSONStateManager(statePath, WithLockTTL(1*time.Millisecond))
	ctx := context.Background()

	// Create a stale lock file manually
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	staleLock := lockInfo{
		PID:        99999, // Non-existent PID
		Hostname:   "old-host",
		AcquiredAt: time.Now().Add(-time.Hour), // Old timestamp
	}
	data, _ := json.Marshal(staleLock)
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Should be able to acquire lock (stale lock removed)
	if err := manager.AcquireLock(ctx); err != nil {
		t.Errorf("AcquireLock() with stale lock error = %v", err)
	}

	// Cleanup
	manager.ReleaseLock(ctx)
}

func TestJSONStateManager_Options(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	backupPath := filepath.Join(tmpDir, "custom-backup.json")

	manager := NewJSONStateManager(statePath,
		WithBackupPath(backupPath),
		WithLockTTL(30*time.Minute),
	)

	if manager.BackupPath() != backupPath {
		t.Errorf("BackupPath() = %s, want %s", manager.BackupPath(), backupPath)
	}

	if manager.lockTTL != 30*time.Minute {
		t.Errorf("lockTTL = %v, want 30m", manager.lockTTL)
	}
}

func TestJSONStateManager_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	deepPath := filepath.Join(tmpDir, "deep", "nested", "state.json")

	manager := NewJSONStateManager(deepPath)
	state := newTestState()
	ctx := context.Background()

	// Save should create directories
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !manager.Exists() {
		t.Error("state file should exist in nested directory")
	}
}

func TestJSONStateManager_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create a complex state
	state := newTestState()
	state.Tasks["task-2"] = &core.TaskState{
		ID:           "task-2",
		Phase:        core.PhasePlan,
		Name:         "Second Task",
		Status:       core.TaskStatusCompleted,
		CLI:          "gemini",
		Model:        "gemini-2.5-flash",
		Dependencies: []core.TaskID{"task-1"},
		TokensIn:     500,
		TokensOut:    200,
		CostUSD:      0.01,
	}
	state.TaskOrder = append(state.TaskOrder, "task-2")
	state.Checkpoints = []core.Checkpoint{
		{
			ID:        "cp-1",
			Phase:     core.PhaseAnalyze,
			Timestamp: time.Now(),
			Message:   "First checkpoint",
		},
	}

	// Save
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify complex structure
	if len(loaded.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(loaded.Tasks))
	}

	task2 := loaded.Tasks["task-2"]
	if task2 == nil {
		t.Fatal("task-2 should exist")
	}
	if task2.Status != core.TaskStatusCompleted {
		t.Errorf("task-2 Status = %s, want completed", task2.Status)
	}
	if len(task2.Dependencies) != 1 || task2.Dependencies[0] != "task-1" {
		t.Error("task-2 dependencies not preserved")
	}

	if len(loaded.Checkpoints) != 1 {
		t.Errorf("len(Checkpoints) = %d, want 1", len(loaded.Checkpoints))
	}
}

func TestJSONStateManager_Backup(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	state := newTestState()
	ctx := context.Background()

	// Save state
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Explicitly backup
	if err := manager.Backup(ctx); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(manager.BackupPath()); os.IsNotExist(err) {
		t.Error("backup file should exist")
	}
}

func TestJSONStateManager_BackupNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Backup when no state exists should not error
	if err := manager.Backup(ctx); err != nil {
		t.Errorf("Backup() with no state error = %v", err)
	}
}
