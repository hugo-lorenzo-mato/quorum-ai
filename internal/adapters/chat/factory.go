package chat

import (
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// NewChatStore creates a ChatStore (SQLite) at the specified path.
// The path should be the chat DB path (e.g., ".quorum/state/chat.db").
func NewChatStore(path string) (core.ChatStore, error) {
	// Ensure path has .db extension for SQLite
	if !strings.HasSuffix(path, ".db") {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".db"
	}

	store, err := NewSQLiteChatStore(path)
	if err != nil {
		return nil, err
	}
	return store, nil
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
