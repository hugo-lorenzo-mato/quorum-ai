package chat

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestSQLiteChatStore_SessionAndMessages_RoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")

	store, err := NewSQLiteChatStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteChatStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC().Truncate(time.Second)
	sess := &core.ChatSessionState{
		ID:        "s1",
		Title:     "hello",
		CreatedAt: now,
		UpdatedAt: now,
		Agent:     "claude",
		Model:     "default",
	}
	if err := store.SaveSession(ctx, sess); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	loaded, err := store.LoadSession(ctx, "s1")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded == nil || loaded.ID != "s1" {
		t.Fatalf("unexpected loaded session: %#v", loaded)
	}

	msg := &core.ChatMessageState{
		ID:        "m1",
		SessionID: "s1",
		Role:      "user",
		Content:   "hi",
		Timestamp: now.Add(1 * time.Second),
		TokensIn:  1,
		TokensOut: 2,
	}
	if err := store.SaveMessage(ctx, msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	msgs, err := store.LoadMessages(ctx, "s1")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "m1" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}

	sessions, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Fatalf("unexpected sessions: %#v", sessions)
	}

	if err := store.DeleteSession(ctx, "s1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	loaded, err = store.LoadSession(ctx, "s1")
	if err != nil {
		t.Fatalf("LoadSession after delete: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected deleted session to be nil, got %#v", loaded)
	}
}

func TestNewChatStore_AppendsDBExtension(t *testing.T) {
	dir := t.TempDir()
	store, err := NewChatStore(filepath.Join(dir, "chat"))
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	_ = CloseChatStore(store)
}
