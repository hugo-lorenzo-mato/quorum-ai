package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// ---------------------------------------------------------------------------
// handleListSystemPrompts
// ---------------------------------------------------------------------------

func TestHandleListSystemPrompts_Success(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system-prompts/", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var prompts []service.SystemPromptMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &prompts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The embedded prompts should be present (at least one).
	if len(prompts) == 0 {
		t.Error("expected at least one system prompt")
	}

	// Each prompt should have required fields.
	for _, p := range prompts {
		if p.ID == "" {
			t.Error("expected non-empty ID")
		}
		if p.Title == "" {
			t.Errorf("expected non-empty Title for prompt %q", p.ID)
		}
		if p.WorkflowPhase == "" {
			t.Errorf("expected non-empty WorkflowPhase for prompt %q", p.ID)
		}
		if p.Sha256 == "" {
			t.Errorf("expected non-empty Sha256 for prompt %q", p.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// handleGetSystemPrompt
// ---------------------------------------------------------------------------

func TestHandleGetSystemPrompt_NotFound(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system-prompts/nonexistent-prompt-id", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetSystemPrompt_Success(t *testing.T) {
	t.Parallel()
	sm := newMockStateManager()
	eb := events.New(100)
	t.Cleanup(func() { eb.Close() })
	srv := NewServer(sm, eb, WithRoot(t.TempDir()))

	// First, list prompts to get a real ID.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/system-prompts/", nil)
	listRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", listRec.Code)
	}

	var prompts []service.SystemPromptMeta
	if err := json.Unmarshal(listRec.Body.Bytes(), &prompts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(prompts) == 0 {
		t.Skip("no embedded system prompts found, skipping get test")
	}

	promptID := prompts[0].ID

	// Now GET the specific prompt.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/system-prompts/"+promptID, nil)
	getRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var prompt service.SystemPrompt
	if err := json.Unmarshal(getRec.Body.Bytes(), &prompt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if prompt.ID != promptID {
		t.Errorf("expected ID %q, got %q", promptID, prompt.ID)
	}
	if prompt.Content == "" {
		t.Error("expected non-empty Content")
	}
	if prompt.Title == "" {
		t.Error("expected non-empty Title")
	}
}
