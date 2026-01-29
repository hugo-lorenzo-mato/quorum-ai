// Package chat provides chat session persistence implementations.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// JSONChatStore implements ChatStore using JSON files.
// Each session is stored as a separate JSON file containing session metadata and messages.
type JSONChatStore struct {
	mu           sync.RWMutex
	baseDir      string // Base directory for chat files (e.g., .quorum/chat)
	sessionsDir  string // Directory for session files
}

// NewJSONChatStore creates a new JSON-based chat store.
// The path should be the base chat directory (e.g., ".quorum/chat").
func NewJSONChatStore(path string) (*JSONChatStore, error) {
	store := &JSONChatStore{
		baseDir:     path,
		sessionsDir: filepath.Join(path, "sessions"),
	}

	// Ensure directories exist
	if err := os.MkdirAll(store.sessionsDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating chat sessions directory: %w", err)
	}

	return store, nil
}

// chatFileEnvelope wraps session data with messages for file storage.
type chatFileEnvelope struct {
	Version  int                    `json:"version"`
	Session  *core.ChatSessionState `json:"session"`
	Messages []*core.ChatMessageState `json:"messages"`
}

// sessionPath returns the file path for a session by ID.
func (s *JSONChatStore) sessionPath(id string) string {
	return filepath.Join(s.sessionsDir, id+".json")
}

// SaveSession persists a chat session.
func (s *JSONChatStore) SaveSession(ctx context.Context, session *core.ChatSessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := s.sessionPath(session.ID)

	// Load existing messages if file exists
	var messages []*core.ChatMessageState
	if data, err := os.ReadFile(sessionPath); err == nil {
		var envelope chatFileEnvelope
		if err := json.Unmarshal(data, &envelope); err == nil {
			messages = envelope.Messages
		}
	}

	envelope := chatFileEnvelope{
		Version:  1,
		Session:  session,
		Messages: messages,
	}

	return s.writeEnvelope(sessionPath, &envelope)
}

// LoadSession retrieves a chat session by ID.
func (s *JSONChatStore) LoadSession(ctx context.Context, id string) (*core.ChatSessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPath := s.sessionPath(id)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var envelope chatFileEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}

	return envelope.Session, nil
}

// ListSessions returns all chat sessions.
func (s *JSONChatStore) ListSessions(ctx context.Context) ([]*core.ChatSessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*core.ChatSessionState{}, nil
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var sessions []*core.ChatSessionState
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(s.sessionsDir, entry.Name())
		data, err := os.ReadFile(sessionPath)
		if err != nil {
			continue // Skip unreadable files
		}

		var envelope chatFileEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			continue // Skip malformed files
		}

		sessions = append(sessions, envelope.Session)
	}

	return sessions, nil
}

// DeleteSession removes a chat session and all its messages.
func (s *JSONChatStore) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := s.sessionPath(id)
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing session file: %w", err)
	}

	return nil
}

// SaveMessage adds a message to a session.
func (s *JSONChatStore) SaveMessage(ctx context.Context, msg *core.ChatMessageState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := s.sessionPath(msg.SessionID)

	// Load existing envelope
	var envelope chatFileEnvelope
	if data, err := os.ReadFile(sessionPath); err == nil {
		if err := json.Unmarshal(data, &envelope); err != nil {
			return fmt.Errorf("parsing session file: %w", err)
		}
	} else if os.IsNotExist(err) {
		return fmt.Errorf("session not found: %s", msg.SessionID)
	} else {
		return fmt.Errorf("reading session file: %w", err)
	}

	// Append message
	envelope.Messages = append(envelope.Messages, msg)

	// Update session timestamp
	if envelope.Session != nil {
		envelope.Session.UpdatedAt = msg.Timestamp
	}

	return s.writeEnvelope(sessionPath, &envelope)
}

// LoadMessages retrieves all messages for a session.
func (s *JSONChatStore) LoadMessages(ctx context.Context, sessionID string) ([]*core.ChatMessageState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPath := s.sessionPath(sessionID)
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var envelope chatFileEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}

	return envelope.Messages, nil
}

// writeEnvelope writes the envelope to disk atomically.
func (s *JSONChatStore) writeEnvelope(path string, envelope *chatFileEnvelope) error {
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling envelope: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// Close is a no-op for JSON store but satisfies the Closeable interface.
func (s *JSONChatStore) Close() error {
	return nil
}
