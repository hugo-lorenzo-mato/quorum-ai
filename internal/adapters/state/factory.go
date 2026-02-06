package state

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// StateManagerOptions configures state manager creation.
type StateManagerOptions struct {
	// LockTTL is the duration after which a lock is considered stale.
	// If zero, the backend's default is used (typically 1 hour).
	LockTTL time.Duration

	// BackupPath is the path to store SQLite backups (optional).
	// If empty, the backend default is used (typically "<db>.bak").
	BackupPath string
}

// NewStateManager creates a StateManager (SQLite) at the specified path.
// The path should be the state DB path (e.g., ".quorum/state/state.db").
func NewStateManager(path string) (core.StateManager, error) {
	return NewStateManagerWithOptions(path, StateManagerOptions{})
}

// NewStateManagerWithOptions creates a StateManager (SQLite) with additional options.
func NewStateManagerWithOptions(path string, opts StateManagerOptions) (core.StateManager, error) {
	// Ensure path has .db extension for SQLite
	if !strings.HasSuffix(path, ".db") {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".db"
	}

	var sqliteOpts []SQLiteStateManagerOption
	if opts.LockTTL > 0 {
		sqliteOpts = append(sqliteOpts, WithSQLiteLockTTL(opts.LockTTL))
	}
	if strings.TrimSpace(opts.BackupPath) != "" {
		sqliteOpts = append(sqliteOpts, WithSQLiteBackupPath(opts.BackupPath))
	}

	sm, err := NewSQLiteStateManager(path, sqliteOpts...)
	if err != nil {
		return nil, err
	}

	return sm, nil
}

// Closeable is an optional interface for StateManagers that need cleanup.
type Closeable interface {
	Close() error
}

// CloseStateManager safely closes a StateManager if it implements Closeable.
func CloseStateManager(sm core.StateManager) error {
	if closeable, ok := sm.(Closeable); ok {
		return closeable.Close()
	}
	return nil
}
