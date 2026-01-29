package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// NewChatStore creates a ChatStore based on the specified backend type.
// Supported backends: "json" (default), "sqlite"
// The path should be the base chat path (e.g., ".quorum/chat" for JSON or ".quorum/chat.db" for SQLite).
func NewChatStore(backend, path string) (core.ChatStore, error) {
	backend = normalizeBackend(backend)

	switch backend {
	case "json":
		return NewJSONChatStore(path)
	case "sqlite":
		// Ensure path has .db extension for SQLite
		if !strings.HasSuffix(path, ".db") {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".db"
		}
		return NewSQLiteChatStore(path)
	default:
		return nil, fmt.Errorf("unsupported chat store backend: %q (supported: json, sqlite)", backend)
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

// Closeable is an optional interface for ChatStores that need cleanup.
type Closeable interface {
	Close() error
}

// CloseChatStore safely closes a ChatStore if it implements Closeable.
func CloseChatStore(store core.ChatStore) error {
	if closeable, ok := store.(Closeable); ok {
		return closeable.Close()
	}
	return nil
}
