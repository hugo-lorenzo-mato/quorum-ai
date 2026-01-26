package state

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// NewStateManager creates a StateManager based on the specified backend type.
// Supported backends: "json" (default), "sqlite"
// The path should be the base state path (e.g., ".quorum/state/state.json" or ".quorum/state/state.db").
func NewStateManager(backend, path string) (core.StateManager, error) {
	backend = normalizeBackend(backend)

	switch backend {
	case "json":
		return NewJSONStateManager(path), nil
	case "sqlite":
		// Ensure path has .db extension for SQLite
		if !strings.HasSuffix(path, ".db") {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".db"
		}
		return NewSQLiteStateManager(path)
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
