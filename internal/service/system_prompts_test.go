package service

import (
	"strings"
	"testing"
)

func TestListSystemPrompts_ReturnsMetadata(t *testing.T) {
	metas, err := ListSystemPrompts()
	if err != nil {
		t.Fatalf("ListSystemPrompts() error = %v", err)
	}
	if len(metas) == 0 {
		t.Fatalf("expected at least one system prompt")
	}

	for _, m := range metas {
		if strings.TrimSpace(m.ID) == "" {
			t.Fatalf("missing id in meta: %#v", m)
		}
		if strings.TrimSpace(m.Title) == "" {
			t.Fatalf("missing title in meta (id=%s)", m.ID)
		}
		if strings.TrimSpace(m.WorkflowPhase) == "" {
			t.Fatalf("missing workflow_phase in meta (id=%s)", m.ID)
		}
		if strings.TrimSpace(m.Step) == "" {
			t.Fatalf("missing step in meta (id=%s)", m.ID)
		}
		if strings.TrimSpace(m.Status) == "" {
			t.Fatalf("missing status in meta (id=%s)", m.ID)
		}
		if len(m.UsedBy) == 0 {
			t.Fatalf("missing used_by in meta (id=%s)", m.ID)
		}
		if len(m.Sha256) != 64 {
			t.Fatalf("sha256 should be 64 hex chars (id=%s), got=%q", m.ID, m.Sha256)
		}
	}
}

func TestGetSystemPrompt_ReturnsContentWithoutFrontmatter(t *testing.T) {
	p, err := GetSystemPrompt("analyze-v1")
	if err != nil {
		t.Fatalf("GetSystemPrompt() error = %v", err)
	}
	if p == nil {
		t.Fatalf("expected non-nil prompt")
	}
	if strings.HasPrefix(strings.TrimSpace(p.Content), "---") {
		t.Fatalf("expected content to exclude frontmatter, got prefix=%q", p.Content[:min(32, len(p.Content))])
	}
	if !strings.Contains(p.Content, "# Analysis Request") {
		t.Fatalf("expected content to include prompt body header")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
