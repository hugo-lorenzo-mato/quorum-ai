package attachments

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestStore_SaveListResolveDelete_RoundTrip(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	meta, err := s.Save(OwnerWorkflow, "wf-1", strings.NewReader("abc"), "../evil.txt")
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if meta.ID == "" {
		t.Fatalf("expected attachment ID")
	}
	if meta.Name == "" || strings.Contains(meta.Name, "..") {
		t.Fatalf("unexpected sanitized name: %q", meta.Name)
	}
	if meta.Size != 3 {
		t.Fatalf("expected size 3, got %d", meta.Size)
	}

	list, err := s.List(OwnerWorkflow, "wf-1")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(list))
	}

	gotMeta, abs, err := s.Resolve(OwnerWorkflow, "wf-1", meta.ID)
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if gotMeta.ID != meta.ID {
		t.Fatalf("resolved meta mismatch: %q != %q", gotMeta.ID, meta.ID)
	}
	b, err := io.ReadAll(mustOpen(t, abs))
	if err != nil {
		t.Fatalf("read resolved file: %v", err)
	}
	if string(b) != "abc" {
		t.Fatalf("unexpected file content: %q", string(b))
	}

	if err := s.Delete(OwnerWorkflow, "wf-1", meta.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	list, err = s.List(OwnerWorkflow, "wf-1")
	if err != nil {
		t.Fatalf("List after delete error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 attachments after delete, got %d", len(list))
	}
}

func TestStore_Save_RejectsTooLarge(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// +1 to exceed the limit; use a streaming reader.
	tooBig := bytes.Repeat([]byte("a"), MaxAttachmentSizeBytes+1)
	_, err := s.Save(OwnerChatSession, "chat-1", bytes.NewReader(tooBig), "big.bin")
	if err == nil {
		t.Fatalf("expected error for too-large attachment")
	}
}

func mustOpen(t *testing.T, path string) io.ReadCloser {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %q: %v", path, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}
