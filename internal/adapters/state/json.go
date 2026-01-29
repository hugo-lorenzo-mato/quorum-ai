package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/fsutil"
)

// JSONStateManager implements StateManager with JSON file storage.
// Supports multi-workflow storage with active workflow tracking.
type JSONStateManager struct {
	baseDir      string // Base directory for state files (e.g., .quorum/state)
	workflowsDir string // Directory for workflow files (e.g., .quorum/state/workflows)
	activePath   string // Path to active workflow ID file
	lockPath     string
	lockTTL      time.Duration

	// Legacy single-file support for migration
	legacyPath       string
	legacyBackupPath string
}

// JSONStateManagerOption configures the manager.
type JSONStateManagerOption func(*JSONStateManager)

// NewJSONStateManager creates a new JSON state manager.
// The path parameter is the base state directory (e.g., ".quorum/state/state.json").
// For backwards compatibility, if the legacy file exists, it will be used.
func NewJSONStateManager(path string, opts ...JSONStateManagerOption) *JSONStateManager {
	baseDir := filepath.Dir(path)
	m := &JSONStateManager{
		baseDir:          baseDir,
		workflowsDir:     filepath.Join(baseDir, "workflows"),
		activePath:       filepath.Join(baseDir, "active.json"),
		lockPath:         path + ".lock",
		lockTTL:          time.Hour,
		legacyPath:       path,
		legacyBackupPath: path + ".bak",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithBackupPath sets the legacy backup file path.
func WithBackupPath(path string) JSONStateManagerOption {
	return func(m *JSONStateManager) {
		m.legacyBackupPath = path
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

// Save persists workflow state atomically and sets it as the active workflow.
func (m *JSONStateManager) Save(ctx context.Context, state *core.WorkflowState) error {
	// Ensure workflows directory exists
	if err := os.MkdirAll(m.workflowsDir, 0o750); err != nil {
		return fmt.Errorf("creating workflows directory: %w", err)
	}

	// Determine workflow file path
	workflowPath := m.workflowPath(state.WorkflowID)

	// Create backup of existing workflow state if it exists
	if _, err := os.Stat(workflowPath); err == nil {
		backupPath := workflowPath + ".bak"
		if data, readErr := fsutil.ReadFileScoped(workflowPath); readErr == nil {
			_ = atomicWriteFile(backupPath, data, 0o600)
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

	// Atomic write to workflow file
	if err := atomicWriteFile(workflowPath, data, 0o600); err != nil {
		return fmt.Errorf("writing workflow state file: %w", err)
	}

	// Set this workflow as active
	if err := m.SetActiveWorkflowID(ctx, state.WorkflowID); err != nil {
		return fmt.Errorf("setting active workflow: %w", err)
	}

	return nil
}

// workflowPath returns the file path for a workflow by ID.
func (m *JSONStateManager) workflowPath(id core.WorkflowID) string {
	return filepath.Join(m.workflowsDir, string(id)+".json")
}

// Load retrieves the active workflow state.
// First tries the new multi-workflow structure, then falls back to legacy single-file.
func (m *JSONStateManager) Load(ctx context.Context) (*core.WorkflowState, error) {
	// Try to get active workflow ID
	activeID, err := m.GetActiveWorkflowID(ctx)
	if err == nil && activeID != "" {
		// Load the active workflow
		state, loadErr := m.LoadByID(ctx, activeID)
		if loadErr != nil {
			return nil, loadErr
		}
		if state != nil {
			return state, nil
		}
	}

	// Fall back to legacy single-file for backwards compatibility
	if _, statErr := os.Stat(m.legacyPath); statErr == nil {
		state, loadErr := m.loadFromPath(m.legacyPath)
		if loadErr != nil {
			// Try loading from legacy backup
			backupState, backupErr := m.loadFromPath(m.legacyBackupPath)
			if backupErr != nil {
				return nil, fmt.Errorf("loading legacy state: %w (backup also failed: %v)", loadErr, backupErr)
			}
			return backupState, nil
		}
		return state, nil
	}

	return nil, nil
}

// LoadByID retrieves a specific workflow state by its ID.
func (m *JSONStateManager) LoadByID(_ context.Context, id core.WorkflowID) (*core.WorkflowState, error) {
	workflowPath := m.workflowPath(id)
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return nil, nil
	}

	state, err := m.loadFromPath(workflowPath)
	if err != nil {
		// Try backup
		backupPath := workflowPath + ".bak"
		backupState, backupErr := m.loadFromPath(backupPath)
		if backupErr != nil {
			return nil, fmt.Errorf("loading workflow %s: %w (backup also failed: %v)", id, err, backupErr)
		}
		return backupState, nil
	}
	return state, nil
}

// activeWorkflowFile stores the active workflow ID.
type activeWorkflowFile struct {
	WorkflowID core.WorkflowID `json:"workflow_id"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// GetActiveWorkflowID returns the ID of the currently active workflow.
func (m *JSONStateManager) GetActiveWorkflowID(_ context.Context) (core.WorkflowID, error) {
	data, err := fsutil.ReadFileScoped(m.activePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading active workflow file: %w", err)
	}

	var active activeWorkflowFile
	if err := json.Unmarshal(data, &active); err != nil {
		return "", fmt.Errorf("parsing active workflow file: %w", err)
	}

	return active.WorkflowID, nil
}

// SetActiveWorkflowID sets the active workflow ID.
func (m *JSONStateManager) SetActiveWorkflowID(_ context.Context, id core.WorkflowID) error {
	// Ensure base directory exists
	if err := os.MkdirAll(m.baseDir, 0o750); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	active := activeWorkflowFile{
		WorkflowID: id,
		UpdatedAt:  time.Now(),
	}

	data, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling active workflow: %w", err)
	}

	if err := atomicWriteFile(m.activePath, data, 0o600); err != nil {
		return fmt.Errorf("writing active workflow file: %w", err)
	}

	return nil
}

// ListWorkflows returns summaries of all available workflows.
func (m *JSONStateManager) ListWorkflows(ctx context.Context) ([]core.WorkflowSummary, error) {
	var summaries []core.WorkflowSummary

	// Get active workflow ID for marking
	activeID, _ := m.GetActiveWorkflowID(ctx)

	// Check for legacy workflow first
	if _, err := os.Stat(m.legacyPath); err == nil {
		state, loadErr := m.loadFromPath(m.legacyPath)
		if loadErr == nil && state != nil {
			summaries = append(summaries, m.stateToSummary(state, activeID))
		}
	}

	// Scan workflows directory
	if _, err := os.Stat(m.workflowsDir); os.IsNotExist(err) {
		return summaries, nil
	}

	entries, err := os.ReadDir(m.workflowsDir)
	if err != nil {
		return summaries, fmt.Errorf("reading workflows directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isJSONFile(entry.Name()) || isBackupFile(entry.Name()) {
			continue
		}

		workflowPath := filepath.Join(m.workflowsDir, entry.Name())
		state, loadErr := m.loadFromPath(workflowPath)
		if loadErr != nil {
			continue // Skip corrupted files
		}

		summaries = append(summaries, m.stateToSummary(state, activeID))
	}

	return summaries, nil
}

// stateToSummary converts a WorkflowState to a WorkflowSummary.
func (m *JSONStateManager) stateToSummary(state *core.WorkflowState, activeID core.WorkflowID) core.WorkflowSummary {
	prompt := state.Prompt
	if len(prompt) > 80 {
		prompt = prompt[:77] + "..."
	}

	return core.WorkflowSummary{
		WorkflowID:   state.WorkflowID,
		Title:        state.Title,
		Status:       state.Status,
		CurrentPhase: state.CurrentPhase,
		Prompt:       prompt,
		CreatedAt:    state.CreatedAt,
		UpdatedAt:    state.UpdatedAt,
		IsActive:     state.WorkflowID == activeID,
	}
}

// isJSONFile checks if a filename is a JSON file.
func isJSONFile(name string) bool {
	return filepath.Ext(name) == ".json"
}

// isBackupFile checks if a filename is a backup file.
func isBackupFile(name string) bool {
	return strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, ".json.bak")
}

func (m *JSONStateManager) loadFromPath(path string) (*core.WorkflowState, error) {
	data, err := fsutil.ReadFileScoped(path)
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

	// Ensure defaults for legacy task states without output fields.
	for _, task := range envelope.State.Tasks {
		if task == nil {
			continue
		}
		if task.Output == "" && task.Status == core.TaskStatusCompleted {
			task.Output = "[output not captured - legacy state]"
		}
	}

	return envelope.State, nil
}

// createBackup creates a backup of a specific workflow file.
func (m *JSONStateManager) createBackupForWorkflow(id core.WorkflowID) error {
	workflowPath := m.workflowPath(id)
	data, err := fsutil.ReadFileScoped(workflowPath)
	if err != nil {
		return err
	}
	return atomicWriteFile(workflowPath+".bak", data, 0o600)
}

// Exists checks if any state exists (active workflow or legacy).
func (m *JSONStateManager) Exists() bool {
	// Check for active workflow
	if _, err := os.Stat(m.activePath); err == nil {
		return true
	}
	// Check for any workflow files
	if _, err := os.Stat(m.workflowsDir); err == nil {
		entries, _ := os.ReadDir(m.workflowsDir)
		for _, e := range entries {
			if !e.IsDir() && isJSONFile(e.Name()) && !isBackupFile(e.Name()) {
				return true
			}
		}
	}
	// Check for legacy state file
	_, err := os.Stat(m.legacyPath)
	return err == nil
}

// Backup creates a backup of the active workflow state.
func (m *JSONStateManager) Backup(ctx context.Context) error {
	activeID, err := m.GetActiveWorkflowID(ctx)
	if err != nil || activeID == "" {
		// Try legacy backup
		if _, statErr := os.Stat(m.legacyPath); statErr == nil {
			data, readErr := fsutil.ReadFileScoped(m.legacyPath)
			if readErr != nil {
				return readErr
			}
			return atomicWriteFile(m.legacyBackupPath, data, 0o600)
		}
		return nil
	}
	return m.createBackupForWorkflow(activeID)
}

// Restore restores from the most recent backup of the active workflow.
func (m *JSONStateManager) Restore(ctx context.Context) (*core.WorkflowState, error) {
	activeID, err := m.GetActiveWorkflowID(ctx)
	if err != nil || activeID == "" {
		// Try legacy restore
		return m.loadFromPath(m.legacyBackupPath)
	}
	return m.loadFromPath(m.workflowPath(activeID) + ".bak")
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
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	// Check for existing lock
	if data, err := fsutil.ReadFileScoped(m.lockPath); err == nil {
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
			if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing stale lock: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading lock file: %w", err)
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
	f, err := os.OpenFile(m.lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return core.ErrState("LOCK_ACQUIRE_FAILED", "lock file created by another process")
		}
		return fmt.Errorf("creating lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		if rmErr := os.Remove(m.lockPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("writing lock file: %w (cleanup failed: %v)", err, rmErr)
		}
		return fmt.Errorf("writing lock file: %w", err)
	}

	return nil
}

// ReleaseLock releases the lock.
func (m *JSONStateManager) ReleaseLock(_ context.Context) error {
	// Verify we own the lock
	data, err := fsutil.ReadFileScoped(m.lockPath)
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

	if err := os.Remove(m.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing lock file: %w", err)
	}
	return nil
}

// Path returns the legacy state file path (for backwards compatibility).
func (m *JSONStateManager) Path() string {
	return m.legacyPath
}

// BackupPath returns the legacy backup file path (for backwards compatibility).
func (m *JSONStateManager) BackupPath() string {
	return m.legacyBackupPath
}

// WorkflowsDir returns the directory containing workflow files.
func (m *JSONStateManager) WorkflowsDir() string {
	return m.workflowsDir
}

// processExists checks if a process is running.
func processExists(pid int) bool {
	// Windows reports no access when signaling the current process; treat that as existing.
	if runtime.GOOS == "windows" && pid == os.Getpid() {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// MigrateState migrates state files from legacy paths to the new unified path.
// It checks if state exists at the old location (.orchestrator/) and migrates
// to the new location (.quorum/state/) if the new location doesn't exist.
// Returns true if migration was performed.
func MigrateState(newPath string, logger interface {
	Info(msg string, args ...interface{})
}) (bool, error) {
	// Legacy paths to check
	legacyPaths := []string{
		".orchestrator/state.json",
	}

	// If new state already exists, no migration needed
	if _, err := os.Stat(newPath); err == nil {
		return false, nil
	}

	for _, legacyPath := range legacyPaths {
		if _, err := os.Stat(legacyPath); err != nil {
			continue
		}

		// Found legacy state, migrate it
		if err := migrateStateFile(legacyPath, newPath, logger); err != nil {
			return false, fmt.Errorf("migrating state from %s: %w", legacyPath, err)
		}

		// Also migrate backup if it exists
		legacyBackup := legacyPath + ".bak"
		newBackup := newPath + ".bak"
		if _, err := os.Stat(legacyBackup); err == nil {
			if err := migrateStateFile(legacyBackup, newBackup, logger); err != nil {
				// Non-fatal, just log
				if logger != nil {
					logger.Info("failed to migrate backup file", "error", err)
				}
			}
		}

		return true, nil
	}

	return false, nil
}

func migrateStateFile(src, dst string, logger interface {
	Info(msg string, args ...interface{})
}) error {
	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dstDir, err)
	}

	// Read source file
	data, err := fsutil.ReadFileScoped(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}

	// Write to destination
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", dst, err)
	}

	if logger != nil {
		logger.Info("migrated state file", "from", src, "to", dst)
	}

	return nil
}

// DeactivateWorkflow clears the active workflow without deleting any data.
func (m *JSONStateManager) DeactivateWorkflow(_ context.Context) error {
	// Remove the active workflow file
	if err := os.Remove(m.activePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing active workflow file: %w", err)
	}
	return nil
}

// ArchiveWorkflows moves completed workflows to an archive location.
// Returns the number of workflows archived.
func (m *JSONStateManager) ArchiveWorkflows(ctx context.Context) (int, error) {
	archiveDir := filepath.Join(m.baseDir, "archive")
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return 0, fmt.Errorf("creating archive directory: %w", err)
	}

	// Get active workflow ID to avoid archiving it
	activeID, _ := m.GetActiveWorkflowID(ctx)

	// Check if workflows directory exists
	if _, err := os.Stat(m.workflowsDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(m.workflowsDir)
	if err != nil {
		return 0, fmt.Errorf("reading workflows directory: %w", err)
	}

	archived := 0
	for _, entry := range entries {
		if entry.IsDir() || !isJSONFile(entry.Name()) || isBackupFile(entry.Name()) {
			continue
		}

		workflowPath := filepath.Join(m.workflowsDir, entry.Name())
		state, loadErr := m.loadFromPath(workflowPath)
		if loadErr != nil {
			continue // Skip corrupted files
		}

		// Skip active workflow
		if state.WorkflowID == activeID {
			continue
		}

		// Only archive completed or failed workflows
		if state.Status != core.WorkflowStatusCompleted && state.Status != core.WorkflowStatusFailed {
			continue
		}

		// Move to archive
		archivePath := filepath.Join(archiveDir, entry.Name())
		if err := os.Rename(workflowPath, archivePath); err != nil {
			return archived, fmt.Errorf("moving workflow %s to archive: %w", state.WorkflowID, err)
		}

		// Also move backup if exists
		backupPath := workflowPath + ".bak"
		if _, err := os.Stat(backupPath); err == nil {
			archiveBackupPath := archivePath + ".bak"
			_ = os.Rename(backupPath, archiveBackupPath)
		}

		archived++
	}

	// Archive legacy state if completed
	if _, err := os.Stat(m.legacyPath); err == nil {
		state, loadErr := m.loadFromPath(m.legacyPath)
		if loadErr == nil && (state.Status == core.WorkflowStatusCompleted || state.Status == core.WorkflowStatusFailed) {
			legacyArchive := filepath.Join(archiveDir, filepath.Base(m.legacyPath))
			if err := os.Rename(m.legacyPath, legacyArchive); err == nil {
				archived++
				// Also move legacy backup
				if _, statErr := os.Stat(m.legacyBackupPath); statErr == nil {
					_ = os.Rename(m.legacyBackupPath, legacyArchive+".bak")
				}
			}
		}
	}

	return archived, nil
}

// PurgeAllWorkflows deletes all workflow data permanently.
// Returns the number of workflows deleted.
func (m *JSONStateManager) PurgeAllWorkflows(_ context.Context) (int, error) {
	deleted := 0

	// Remove active workflow file
	if err := os.Remove(m.activePath); err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("removing active workflow file: %w", err)
	}

	// Count and remove workflow files
	if _, err := os.Stat(m.workflowsDir); err == nil {
		entries, err := os.ReadDir(m.workflowsDir)
		if err != nil {
			return 0, fmt.Errorf("reading workflows directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if isJSONFile(entry.Name()) && !isBackupFile(entry.Name()) {
				deleted++
			}
			// Remove all files (including backups)
			filePath := filepath.Join(m.workflowsDir, entry.Name())
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return deleted, fmt.Errorf("removing workflow file %s: %w", entry.Name(), err)
			}
		}
	}

	// Remove legacy state files
	if _, err := os.Stat(m.legacyPath); err == nil {
		if err := os.Remove(m.legacyPath); err == nil {
			deleted++
		}
	}
	if _, err := os.Stat(m.legacyBackupPath); err == nil {
		_ = os.Remove(m.legacyBackupPath)
	}

	// Remove archive directory if exists
	archiveDir := filepath.Join(m.baseDir, "archive")
	if _, err := os.Stat(archiveDir); err == nil {
		entries, _ := os.ReadDir(archiveDir)
		for _, entry := range entries {
			filePath := filepath.Join(archiveDir, entry.Name())
			_ = os.Remove(filePath)
		}
		_ = os.Remove(archiveDir)
	}

	return deleted, nil
}

// DeleteWorkflow deletes a single workflow by ID.
// Returns error if workflow does not exist.
func (m *JSONStateManager) DeleteWorkflow(ctx context.Context, id core.WorkflowID) error {
	workflowPath := m.workflowPath(id)

	// Check workflow exists and get report_path for cleanup
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return fmt.Errorf("workflow not found: %s", id)
	}

	// Load workflow to get report_path before deleting
	var reportPath string
	if state, err := m.LoadByID(ctx, id); err == nil && state != nil {
		reportPath = state.ReportPath
	}

	// Remove workflow file
	if err := os.Remove(workflowPath); err != nil {
		return fmt.Errorf("removing workflow file: %w", err)
	}

	// Remove backup file if exists
	backupPath := workflowPath + ".bak"
	if _, err := os.Stat(backupPath); err == nil {
		_ = os.Remove(backupPath)
	}

	// Clear active workflow if it matches
	activeID, _ := m.GetActiveWorkflowID(ctx)
	if activeID == id {
		_ = m.DeactivateWorkflow(ctx)
	}

	// Delete report directory (best effort, don't fail if it doesn't exist)
	m.deleteReportDirectory(reportPath, string(id))

	return nil
}

// deleteReportDirectory removes the workflow's report directory.
// It tries the stored reportPath first, then falls back to default location.
func (m *JSONStateManager) deleteReportDirectory(reportPath, workflowID string) {
	// Try stored report path first
	if reportPath != "" {
		if err := os.RemoveAll(reportPath); err == nil {
			return
		}
	}

	// Fall back to default location: .quorum/runs/{workflow-id}
	defaultPath := filepath.Join(".quorum", "runs", workflowID)
	_ = os.RemoveAll(defaultPath)
}

// UpdateHeartbeat updates the heartbeat timestamp for a running workflow.
// This is used for zombie detection - workflows with stale heartbeats are considered dead.
func (m *JSONStateManager) UpdateHeartbeat(ctx context.Context, id core.WorkflowID) error {
	state, err := m.LoadByID(ctx, id)
	if err != nil {
		return fmt.Errorf("loading workflow: %w", err)
	}
	if state == nil {
		return fmt.Errorf("workflow not found: %s", id)
	}
	if state.Status != core.WorkflowStatusRunning {
		return fmt.Errorf("workflow not running: %s", id)
	}

	now := time.Now().UTC()
	state.HeartbeatAt = &now
	return m.Save(ctx, state)
}

// FindZombieWorkflows returns workflows with status "running" but stale heartbeats.
// A workflow is considered a zombie if its heartbeat is older than the threshold.
func (m *JSONStateManager) FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error) {
	summaries, err := m.ListWorkflows(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}

	cutoff := time.Now().UTC().Add(-staleThreshold)
	var zombies []*core.WorkflowState

	for _, summary := range summaries {
		if summary.Status != core.WorkflowStatusRunning {
			continue
		}

		state, err := m.LoadByID(ctx, summary.WorkflowID)
		if err != nil || state == nil {
			continue
		}

		// Consider zombie if no heartbeat or heartbeat is stale
		if state.HeartbeatAt == nil || state.HeartbeatAt.Before(cutoff) {
			zombies = append(zombies, state)
		}
	}

	return zombies, nil
}

// AcquireWorkflowLock acquires an exclusive lock for a specific workflow.
// JSONStateManager uses file-based locking and delegates to the global lock.
func (m *JSONStateManager) AcquireWorkflowLock(ctx context.Context, workflowID core.WorkflowID) error {
	// JSON state manager uses single-file storage, so per-workflow locking
	// falls back to global locking for simplicity.
	return m.AcquireLock(ctx)
}

// ReleaseWorkflowLock releases the lock for a specific workflow.
func (m *JSONStateManager) ReleaseWorkflowLock(ctx context.Context, workflowID core.WorkflowID) error {
	return m.ReleaseLock(ctx)
}

// RefreshWorkflowLock extends the lock expiration time for a workflow.
// For JSON state manager, this is a no-op since file-based locks don't expire actively.
func (m *JSONStateManager) RefreshWorkflowLock(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// SetWorkflowRunning marks a workflow as currently executing.
// For JSON state manager, this is tracked only in memory.
func (m *JSONStateManager) SetWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil // No-op for JSON state manager
}

// ClearWorkflowRunning removes a workflow from the running state.
func (m *JSONStateManager) ClearWorkflowRunning(_ context.Context, _ core.WorkflowID) error {
	return nil // No-op for JSON state manager
}

// ListRunningWorkflows returns IDs of all currently executing workflows.
// For JSON state manager, this returns an empty list since running state is not tracked.
func (m *JSONStateManager) ListRunningWorkflows(_ context.Context) ([]core.WorkflowID, error) {
	return nil, nil
}

// IsWorkflowRunning checks if a specific workflow is currently executing.
// For JSON state manager, this always returns false since running state is not tracked.
func (m *JSONStateManager) IsWorkflowRunning(_ context.Context, _ core.WorkflowID) (bool, error) {
	return false, nil
}

// UpdateWorkflowHeartbeat updates the heartbeat timestamp for a running workflow.
// For JSON state manager, this is a no-op.
func (m *JSONStateManager) UpdateWorkflowHeartbeat(_ context.Context, _ core.WorkflowID) error {
	return nil
}

// Verify that JSONStateManager implements core.StateManager.
var _ core.StateManager = (*JSONStateManager)(nil)
