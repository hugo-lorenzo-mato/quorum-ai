package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// JSONStateManager implements StateManager with JSON file storage.
type JSONStateManager struct {
	path       string
	backupPath string
	lockPath   string
	lockTTL    time.Duration
}

// JSONStateManagerOption configures the manager.
type JSONStateManagerOption func(*JSONStateManager)

// NewJSONStateManager creates a new JSON state manager.
func NewJSONStateManager(path string, opts ...JSONStateManagerOption) *JSONStateManager {
	m := &JSONStateManager{
		path:       path,
		backupPath: path + ".bak",
		lockPath:   path + ".lock",
		lockTTL:    time.Hour,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithBackupPath sets the backup file path.
func WithBackupPath(path string) JSONStateManagerOption {
	return func(m *JSONStateManager) {
		m.backupPath = path
	}
}

// WithLockTTL sets the lock TTL.
func WithLockTTL(ttl time.Duration) JSONStateManagerOption {
	return func(m *JSONStateManager) {
		m.lockTTL = ttl
	}
}

// stateEnvelope wraps state with metadata.
type stateEnvelope struct {
	Version   int                 `json:"version"`
	Checksum  string              `json:"checksum"`
	UpdatedAt time.Time           `json:"updated_at"`
	State     *core.WorkflowState `json:"state"`
}

// Save persists workflow state atomically.
func (m *JSONStateManager) Save(_ context.Context, state *core.WorkflowState) error {
	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Create backup of existing state
	if m.Exists() {
		if err := m.createBackup(); err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
	}

	// Update state timestamp
	state.UpdatedAt = time.Now()

	// Clear checksum before serializing to get consistent hash
	state.Checksum = ""

	// Serialize state
	stateBytes, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	// Calculate checksum
	hash := sha256.Sum256(stateBytes)
	checksum := hex.EncodeToString(hash[:])

	// Create envelope (checksum stored only in envelope, not in state)
	envelope := stateEnvelope{
		Version:   1,
		Checksum:  checksum,
		UpdatedAt: time.Now(),
		State:     state,
	}

	// Serialize envelope
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// Atomic write
	if err := atomicWriteFile(m.path, data, 0o644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// Load retrieves workflow state.
func (m *JSONStateManager) Load(_ context.Context) (*core.WorkflowState, error) {
	if !m.Exists() {
		return nil, nil
	}

	state, err := m.loadFromPath(m.path)
	if err != nil {
		// Try loading from backup
		backupState, backupErr := m.loadFromPath(m.backupPath)
		if backupErr != nil {
			return nil, fmt.Errorf("loading state: %w (backup also failed: %v)", err, backupErr)
		}
		return backupState, nil
	}
	return state, nil
}

func (m *JSONStateManager) loadFromPath(path string) (*core.WorkflowState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var envelope stateEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshaling envelope: %w", err)
	}

	// Clear checksum for verification (it was empty when checksum was calculated)
	envelope.State.Checksum = ""

	// Verify checksum
	stateBytes, err := json.Marshal(envelope.State)
	if err != nil {
		return nil, fmt.Errorf("marshaling state for checksum: %w", err)
	}

	hash := sha256.Sum256(stateBytes)
	checksum := hex.EncodeToString(hash[:])

	if checksum != envelope.Checksum {
		return nil, core.ErrState("STATE_CORRUPTED", "checksum mismatch")
	}

	return envelope.State, nil
}

func (m *JSONStateManager) createBackup() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	return atomicWriteFile(m.backupPath, data, 0o644)
}

// Exists checks if state file exists.
func (m *JSONStateManager) Exists() bool {
	_, err := os.Stat(m.path)
	return err == nil
}

// Backup creates a backup of the current state.
func (m *JSONStateManager) Backup(_ context.Context) error {
	if !m.Exists() {
		return nil
	}
	return m.createBackup()
}

// Restore restores from the most recent backup.
func (m *JSONStateManager) Restore(_ context.Context) (*core.WorkflowState, error) {
	return m.loadFromPath(m.backupPath)
}

// lockInfo represents lock file contents.
type lockInfo struct {
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// AcquireLock acquires an exclusive lock.
func (m *JSONStateManager) AcquireLock(_ context.Context) error {
	// Ensure directory exists
	dir := filepath.Dir(m.lockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	// Check for existing lock
	if data, err := os.ReadFile(m.lockPath); err == nil {
		var info lockInfo
		if err := json.Unmarshal(data, &info); err == nil {
			// Check if lock is stale
			if time.Since(info.AcquiredAt) < m.lockTTL {
				// Check if process is still alive
				if processExists(info.PID) {
					return core.ErrState("LOCK_ACQUIRE_FAILED",
						fmt.Sprintf("lock held by PID %d since %s", info.PID, info.AcquiredAt))
				}
			}
			// Stale lock, remove it
			os.Remove(m.lockPath)
		}
	}

	// Create lock file
	hostname, _ := os.Hostname()
	info := lockInfo{
		PID:        os.Getpid(),
		Hostname:   hostname,
		AcquiredAt: time.Now(),
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshaling lock info: %w", err)
	}

	// Write lock file exclusively
	f, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return core.ErrState("LOCK_ACQUIRE_FAILED", "lock file created by another process")
		}
		return fmt.Errorf("creating lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(m.lockPath)
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

// ReleaseLock releases the lock.
func (m *JSONStateManager) ReleaseLock(_ context.Context) error {
	// Verify we own the lock
	data, err := os.ReadFile(m.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already released
		}
		return fmt.Errorf("reading lock file: %w", err)
	}

	var info lockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("parsing lock info: %w", err)
	}

	if info.PID != os.Getpid() {
		return core.ErrState("LOCK_RELEASE_FAILED", "lock owned by different process")
	}

	return os.Remove(m.lockPath)
}

// Path returns the state file path.
func (m *JSONStateManager) Path() string {
	return m.path
}

// BackupPath returns the backup file path.
func (m *JSONStateManager) BackupPath() string {
	return m.backupPath
}

// processExists checks if a process is running.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Verify that JSONStateManager implements core.StateManager.
var _ core.StateManager = (*JSONStateManager)(nil)
