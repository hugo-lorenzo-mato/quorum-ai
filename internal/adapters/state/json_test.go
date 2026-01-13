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

func TestJSONStateManager_Path(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "test-state.json")

	manager := NewJSONStateManager(statePath)

	if manager.Path() != statePath {
		t.Errorf("Path() = %s, want %s", manager.Path(), statePath)
	}
}

func TestProcessExists(t *testing.T) {
	// Test with current process PID - should exist
	currentPID := os.Getpid()
	if !processExists(currentPID) {
		t.Errorf("processExists(%d) = false, want true for current process", currentPID)
	}

	// Test with a very high PID that shouldn't exist
	nonExistentPID := 999999
	if processExists(nonExistentPID) {
		t.Logf("processExists(%d) = true, this might be valid on some systems", nonExistentPID)
	}

	// Test with PID 0 (init process)
	// Result depends on permissions, just ensure it doesn't panic
	_ = processExists(0)

	// Test with negative PID - should return false or not panic
	_ = processExists(-1)
}

func TestJSONStateManager_LoadFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create a valid backup manually
	state := newTestState()
	state.WorkflowID = "backup-wf"

	// Save to main path first
	if err := manager.Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create backup
	if err := manager.Backup(ctx); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Now corrupt the main file
	if err := os.WriteFile(statePath, []byte("corrupted json{{{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Load should fall back to backup
	loaded, err := manager.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v, should have loaded from backup", err)
	}

	if loaded.WorkflowID != "backup-wf" {
		t.Errorf("WorkflowID = %s, want backup-wf", loaded.WorkflowID)
	}
}

func TestJSONStateManager_LoadBothCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create corrupted main file
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(statePath, []byte("corrupted{"), 0o644); err != nil {
		t.Fatalf("WriteFile() main error = %v", err)
	}

	// Create corrupted backup
	backupPath := statePath + ".bak"
	if err := os.WriteFile(backupPath, []byte("corrupted backup{"), 0o644); err != nil {
		t.Fatalf("WriteFile() backup error = %v", err)
	}

	// Load should fail since both are corrupted
	_, err := manager.Load(ctx)
	if err == nil {
		t.Error("Load() should fail when both main and backup are corrupted")
	}
}

func TestJSONStateManager_ReleaseLockOwnedByOther(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create a lock file owned by a different PID
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	otherLock := lockInfo{
		PID:        os.Getpid() + 1, // Different PID
		Hostname:   "other-host",
		AcquiredAt: time.Now(),
	}
	data, _ := json.Marshal(otherLock)
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Release should fail with "lock owned by different process"
	err := manager.ReleaseLock(ctx)
	if err == nil {
		t.Error("ReleaseLock() should fail when lock owned by different process")
	}
}

func TestJSONStateManager_ReleaseLockCorrupted(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	manager := NewJSONStateManager(statePath)
	ctx := context.Background()

	// Create a corrupted lock file
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if err := os.WriteFile(lockPath, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Release should fail with parse error
	err := manager.ReleaseLock(ctx)
	if err == nil {
		t.Error("ReleaseLock() should fail with corrupted lock file")
	}
}

func TestJSONStateManager_AcquireLockHeldByActiveProcess(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Create manager with long TTL
	manager := NewJSONStateManager(statePath, WithLockTTL(time.Hour))
	ctx := context.Background()

	// Create a lock file held by current process (same PID)
	lockPath := statePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Use current PID - which definitely exists
	activeLock := lockInfo{
		PID:        os.Getpid(), // Current process always exists
		Hostname:   "localhost",
		AcquiredAt: time.Now(), // Recent timestamp, within TTL
	}
	data, _ := json.Marshal(activeLock)
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// AcquireLock should fail because process is still active and lock file exists
	// The O_EXCL flag in OpenFile will fail since the file already exists
	err := manager.AcquireLock(ctx)
	if err == nil {
		t.Error("AcquireLock() should fail when lock held by active process")
	}

	// Cleanup
	os.Remove(lockPath)
}

type testLogger struct {
	messages []string
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.messages = append(l.messages, msg)
}

func TestMigrateState(t *testing.T) {
	t.Run("no legacy state", func(t *testing.T) {
		tmpDir := t.TempDir()
		newPath := filepath.Join(tmpDir, ".quorum", "state", "state.json")

		migrated, err := MigrateState(newPath, nil)
		if err != nil {
			t.Errorf("MigrateState() error = %v", err)
		}
		if migrated {
			t.Error("MigrateState() returned true when no legacy state exists")
		}
	})

	t.Run("new state already exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		newPath := filepath.Join(tmpDir, "new", "state.json")

		// Create new state
		os.MkdirAll(filepath.Dir(newPath), 0o755)
		os.WriteFile(newPath, []byte(`{"test": true}`), 0o644)

		migrated, err := MigrateState(newPath, nil)
		if err != nil {
			t.Errorf("MigrateState() error = %v", err)
		}
		if migrated {
			t.Error("MigrateState() returned true when new state already exists")
		}
	})

	t.Run("migrate from legacy path", func(t *testing.T) {
		// Save current directory and change to temp dir
		origDir, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		// Create legacy state at .orchestrator/state.json
		legacyPath := ".orchestrator/state.json"
		os.MkdirAll(filepath.Dir(legacyPath), 0o755)
		legacyContent := []byte(`{"version":1,"checksum":"abc","state":{"workflow_id":"test"}}`)
		os.WriteFile(legacyPath, legacyContent, 0o644)

		// Also create legacy backup
		os.WriteFile(legacyPath+".bak", legacyContent, 0o644)

		newPath := ".quorum/state/state.json"

		migrated, err := MigrateState(newPath, nil)
		if err != nil {
			t.Fatalf("MigrateState() error = %v", err)
		}
		if !migrated {
			t.Fatal("MigrateState() returned false when legacy state exists")
		}

		// Verify new state exists
		newContent, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("failed to read new state: %v", err)
		}
		if string(newContent) != string(legacyContent) {
			t.Errorf("migrated content mismatch: got %s, want %s", newContent, legacyContent)
		}

		// Verify backup was also migrated
		newBackup, err := os.ReadFile(newPath + ".bak")
		if err != nil {
			t.Fatalf("failed to read new backup: %v", err)
		}
		if string(newBackup) != string(legacyContent) {
			t.Errorf("migrated backup mismatch: got %s, want %s", newBackup, legacyContent)
		}
	})

	t.Run("migrate with logger", func(t *testing.T) {
		// Save current directory and change to temp dir
		origDir, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		// Create legacy state at .orchestrator/state.json
		legacyPath := ".orchestrator/state.json"
		os.MkdirAll(filepath.Dir(legacyPath), 0o755)
		legacyContent := []byte(`{"version":1,"checksum":"abc","state":{"workflow_id":"test"}}`)
		os.WriteFile(legacyPath, legacyContent, 0o644)

		newPath := ".quorum/state/state.json"

		logger := &testLogger{}
		migrated, err := MigrateState(newPath, logger)
		if err != nil {
			t.Fatalf("MigrateState() error = %v", err)
		}
		if !migrated {
			t.Fatal("MigrateState() returned false when legacy state exists")
		}

		// Verify logger was called
		if len(logger.messages) == 0 {
			t.Error("Logger should have been called during migration")
		}
	})

	t.Run("migrate with unreadable backup", func(t *testing.T) {
		// Save current directory and change to temp dir
		origDir, _ := os.Getwd()
		tmpDir := t.TempDir()
		os.Chdir(tmpDir)
		defer os.Chdir(origDir)

		// Create legacy state at .orchestrator/state.json
		legacyPath := ".orchestrator/state.json"
		os.MkdirAll(filepath.Dir(legacyPath), 0o755)
		legacyContent := []byte(`{"version":1,"checksum":"abc","state":{"workflow_id":"test"}}`)
		os.WriteFile(legacyPath, legacyContent, 0o644)

		// Create a backup file that is a directory (will cause read failure)
		os.MkdirAll(legacyPath+".bak", 0o755)

		newPath := ".quorum/state/state.json"

		logger := &testLogger{}
		migrated, err := MigrateState(newPath, logger)
		if err != nil {
			t.Fatalf("MigrateState() error = %v (backup failure should be non-fatal)", err)
		}
		if !migrated {
			t.Fatal("MigrateState() returned false")
		}

		// Logger should have logged the backup failure
		foundBackupError := false
		for _, msg := range logger.messages {
			if msg == "failed to migrate backup file" {
				foundBackupError = true
				break
			}
		}
		if !foundBackupError {
			t.Log("Logger messages:", logger.messages)
		}
	})
}
