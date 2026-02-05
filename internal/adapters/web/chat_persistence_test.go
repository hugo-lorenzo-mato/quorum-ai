package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/chat"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestChatHandler_LoadsSessionsFromResolvedStoreAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "chat.db")

	// "Run 1": create store + handler and create a session.
	store1, err := chat.NewSQLiteChatStore(dbPath)
	if err != nil {
		t.Fatalf("creating sqlite chat store: %v", err)
	}

	projectRoot := filepath.Join(dir, "projectA")
	ctx := context.Background()

	h1 := NewChatHandler(
		nil, // agents not needed for this test
		nil,
		nil,
		nil, // no global store
		WithChatStoreResolver(func(_ context.Context) core.ChatStore { return store1 }),
		WithProjectRootResolver(func(_ context.Context) string { return projectRoot }),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/sessions", bytes.NewBufferString(`{}`)).WithContext(ctx)
	h1.CreateSession(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateSession status=%d body=%s", rec.Code, rec.Body.String())
	}

	var created ChatSession
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal CreateSession response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created session id, got empty")
	}

	_ = store1.Close()

	// "Run 2": new handler instance (sessions map empty) but same persisted store.
	store2, err := chat.NewSQLiteChatStore(dbPath)
	if err != nil {
		t.Fatalf("reopening sqlite chat store: %v", err)
	}
	defer func() { _ = store2.Close() }()

	h2 := NewChatHandler(
		nil,
		nil,
		nil,
		nil,
		WithChatStoreResolver(func(_ context.Context) core.ChatStore { return store2 }),
		WithProjectRootResolver(func(_ context.Context) string { return projectRoot }),
	)

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/chat/sessions", nil).WithContext(ctx)
	h2.ListSessions(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("ListSessions status=%d body=%s", rec2.Code, rec2.Body.String())
	}

	var sessions []ChatSession
	if err := json.Unmarshal(rec2.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("unmarshal ListSessions response: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session after restart, got %d (%s)", len(sessions), rec2.Body.String())
	}
	if sessions[0].ID != created.ID {
		t.Fatalf("expected session id %q, got %q", created.ID, sessions[0].ID)
	}
}
