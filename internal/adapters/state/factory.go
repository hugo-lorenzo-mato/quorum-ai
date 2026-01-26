package state

import (
	"fmt"
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
}

// NewStateManager creates a StateManager based on the specified backend type.
// Supported backends: "json" (default), "sqlite"
// The path should be the base state path (e.g., ".quorum/state/state.json" or ".quorum/state/state.db").
func NewStateManager(backend, path string) (core.StateManager, error) {
	return NewStateManagerWithOptions(backend, path, StateManagerOptions{})
}

// NewStateManagerWithOptions creates a StateManager with additional options.
func NewStateManagerWithOptions(backend, path string, opts StateManagerOptions) (core.StateManager, error) {
	backend = normalizeBackend(backend)

	switch backend {
	case "json":
		var jsonOpts []JSONStateManagerOption
		if opts.LockTTL > 0 {
			jsonOpts = append(jsonOpts, WithLockTTL(opts.LockTTL))
		}
		return NewJSONStateManager(path, jsonOpts...), nil
	case "sqlite":
		// Ensure path has .db extension for SQLite
		if !strings.HasSuffix(path, ".db") {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".db"
		}
		var sqliteOpts []SQLiteStateManagerOption
		if opts.LockTTL > 0 {
			sqliteOpts = append(sqliteOpts, WithSQLiteLockTTL(opts.LockTTL))
		}
		return NewSQLiteStateManager(path, sqliteOpts...)
	default:
		return nil, fmt.Errorf("unsupported state backend: %q (supported: json, sqlite)", backend)
	}
}

// normalizeBackend normalizes the backend string to lowercase and handles empty values.
func normalizeBackend(backend string) string {
	backend = strings.ToLower(strings.TrimSpace(backend))
	if backend == "" {
		return "json"
	}
	return backend
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
