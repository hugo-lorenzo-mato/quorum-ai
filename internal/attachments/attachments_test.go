package attachments

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Store accessors
// ---------------------------------------------------------------------------

func TestStore_BaseDir(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	expected := filepath.Join(root, ".quorum", "attachments")
	if s.BaseDir() != expected {
		t.Errorf("BaseDir() = %q, want %q", s.BaseDir(), expected)
	}
}

func TestStore_Root(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	if s.Root() != root {
		t.Errorf("Root() = %q, want %q", s.Root(), root)
	}
}

func TestStore_EnsureBaseDir(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	if err := s.EnsureBaseDir(); err != nil {
		t.Fatalf("EnsureBaseDir: %v", err)
	}

	info, err := os.Stat(s.BaseDir())
	if err != nil {
		t.Fatalf("Stat baseDir: %v", err)
	}
	if !info.IsDir() {
		t.Error("baseDir should be a directory")
	}
}

// ---------------------------------------------------------------------------
// validateOwner
// ---------------------------------------------------------------------------

func TestValidateOwner_InvalidType(t *testing.T) {
	s := NewStore(t.TempDir())

	_, err := s.Save(OwnerType("invalid"), "id1", strings.NewReader("x"), "f.txt")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

func TestValidateOwner_EmptyOwnerID(t *testing.T) {
	s := NewStore(t.TempDir())

	_, err := s.Save(OwnerWorkflow, "", strings.NewReader("x"), "f.txt")
	if err == nil {
		t.Error("expected error for empty owner ID")
	}
}

func TestValidateOwner_WhitespaceOwnerID(t *testing.T) {
	s := NewStore(t.TempDir())

	_, err := s.Save(OwnerChatSession, "   ", strings.NewReader("x"), "f.txt")
	if err == nil {
		t.Error("expected error for whitespace-only owner ID")
	}
}

// ---------------------------------------------------------------------------
// Save with OwnerChatSession
// ---------------------------------------------------------------------------

func TestStore_SaveChatSession(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	meta, err := s.Save(OwnerChatSession, "chat-1", strings.NewReader("hello"), "note.txt")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if meta.ID == "" {
		t.Error("expected non-empty attachment ID")
	}
	if meta.Size != 5 {
		t.Errorf("expected size 5, got %d", meta.Size)
	}
	if meta.ContentType == "" {
		t.Error("expected non-empty content type")
	}
}

// ---------------------------------------------------------------------------
// Save with binary content (content type detection)
// ---------------------------------------------------------------------------

func TestStore_SaveBinaryContent(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// PNG header bytes
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	meta, err := s.Save(OwnerWorkflow, "wf-1", bytes.NewReader(pngHeader), "image.png")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if meta.ContentType != "image/png" {
		t.Errorf("expected content type image/png, got %q", meta.ContentType)
	}
}

// ---------------------------------------------------------------------------
// List edge cases
// ---------------------------------------------------------------------------

func TestStore_ListEmpty(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// List on nonexistent dir should return empty, not error
	list, err := s.List(OwnerWorkflow, "nonexistent")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(list))
	}
}

func TestStore_ListInvalidOwner(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.List(OwnerType("bad"), "id1")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

func TestStore_ListEmptyOwnerID(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.List(OwnerWorkflow, "")
	if err == nil {
		t.Error("expected error for empty owner ID")
	}
}

func TestStore_ListWithCorruptMeta(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create a directory structure that looks like an attachment but has corrupt meta
	attachDir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-corrupt", "att-001")
	os.MkdirAll(attachDir, 0o750)
	os.WriteFile(filepath.Join(attachDir, "meta.json"), []byte("{invalid json}"), 0o600)

	// List should skip corrupt entries gracefully
	list, err := s.List(OwnerWorkflow, "wf-corrupt")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 attachments (corrupt skipped), got %d", len(list))
	}
}

func TestStore_ListWithMissingMeta(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create a directory without meta.json
	attachDir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-nometa", "att-002")
	os.MkdirAll(attachDir, 0o750)

	list, err := s.List(OwnerWorkflow, "wf-nometa")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(list))
	}
}

func TestStore_ListSkipsFiles(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create a file (not a directory) in the owner dir
	ownerDir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-files")
	os.MkdirAll(ownerDir, 0o750)
	os.WriteFile(filepath.Join(ownerDir, "stray-file.txt"), []byte("stray"), 0o600)

	list, err := s.List(OwnerWorkflow, "wf-files")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// Resolve edge cases
// ---------------------------------------------------------------------------

func TestStore_Resolve_InvalidOwner(t *testing.T) {
	s := NewStore(t.TempDir())
	_, _, err := s.Resolve(OwnerType("bad"), "id1", "att-1")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

func TestStore_Resolve_EmptyAttachmentID(t *testing.T) {
	s := NewStore(t.TempDir())
	_, _, err := s.Resolve(OwnerWorkflow, "wf-1", "")
	if err == nil {
		t.Error("expected error for empty attachment ID")
	}
}

func TestStore_Resolve_WhitespaceAttachmentID(t *testing.T) {
	s := NewStore(t.TempDir())
	_, _, err := s.Resolve(OwnerWorkflow, "wf-1", "   ")
	if err == nil {
		t.Error("expected error for whitespace-only attachment ID")
	}
}

func TestStore_Resolve_NotFound(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	_, _, err := s.Resolve(OwnerWorkflow, "wf-1", "nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent attachment")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestStore_Resolve_CorruptMeta(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create dir with corrupt meta.json
	dir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-1", "att-corrupt")
	os.MkdirAll(dir, 0o750)
	os.WriteFile(filepath.Join(dir, "meta.json"), []byte("not json"), 0o600)

	_, _, err := s.Resolve(OwnerWorkflow, "wf-1", "att-corrupt")
	if err == nil {
		t.Error("expected error for corrupt meta.json")
	}
}

// ---------------------------------------------------------------------------
// Delete edge cases
// ---------------------------------------------------------------------------

func TestStore_Delete_InvalidOwner(t *testing.T) {
	s := NewStore(t.TempDir())
	err := s.Delete(OwnerType("bad"), "id1", "att-1")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

func TestStore_Delete_EmptyAttachmentID(t *testing.T) {
	s := NewStore(t.TempDir())
	err := s.Delete(OwnerWorkflow, "wf-1", "")
	if err == nil {
		t.Error("expected error for empty attachment ID")
	}
}

func TestStore_Delete_Nonexistent(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Should not error when deleting nonexistent attachment (RemoveAll on nonexistent is no-op)
	err := s.Delete(OwnerWorkflow, "wf-1", "nonexistent")
	if err != nil {
		t.Errorf("Delete nonexistent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DeleteAll
// ---------------------------------------------------------------------------

func TestStore_DeleteAll_InvalidOwner(t *testing.T) {
	s := NewStore(t.TempDir())
	err := s.DeleteAll(OwnerType("bad"), "id1")
	if err == nil {
		t.Error("expected error for invalid owner type")
	}
}

func TestStore_DeleteAll_EmptyOwnerID(t *testing.T) {
	s := NewStore(t.TempDir())
	err := s.DeleteAll(OwnerWorkflow, "")
	if err == nil {
		t.Error("expected error for empty owner ID")
	}
}

func TestStore_DeleteAll_Success(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Save two attachments
	_, err := s.Save(OwnerWorkflow, "wf-del-all", strings.NewReader("a"), "a.txt")
	if err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	_, err = s.Save(OwnerWorkflow, "wf-del-all", strings.NewReader("b"), "b.txt")
	if err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	list, _ := s.List(OwnerWorkflow, "wf-del-all")
	if len(list) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(list))
	}

	// DeleteAll
	err = s.DeleteAll(OwnerWorkflow, "wf-del-all")
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	list, _ = s.List(OwnerWorkflow, "wf-del-all")
	if len(list) != 0 {
		t.Errorf("expected 0 attachments after DeleteAll, got %d", len(list))
	}
}

func TestStore_DeleteAll_Nonexistent(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Should not error
	err := s.DeleteAll(OwnerWorkflow, "nonexistent")
	if err != nil {
		t.Errorf("DeleteAll nonexistent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// sanitizeFilename
// ---------------------------------------------------------------------------

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"../evil.txt", "evil.txt"},
		{".", "attachment"},
		{"..", "attachment"},
		{"", "attachment"},
		{"   ", "attachment"},
		{"path/to/file.txt", "file.txt"},
		{"path\\to\\file.txt", "path_to_file.txt"},
		{strings.Repeat("a", 300), strings.Repeat("a", 200)},
		{"file\x00name.txt", "filename.txt"},
	}

	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Save and Resolve round trip (chat session owner)
// ---------------------------------------------------------------------------

func TestStore_SaveAndResolve_ChatSession(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	content := "chat attachment content"
	meta, err := s.Save(OwnerChatSession, "chat-99", strings.NewReader(content), "msg.txt")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	resolved, absPath, err := s.Resolve(OwnerChatSession, "chat-99", meta.ID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.ID != meta.ID {
		t.Errorf("ID mismatch: %q != %q", resolved.ID, meta.ID)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

// ---------------------------------------------------------------------------
// OwnerType constants
// ---------------------------------------------------------------------------

func TestOwnerTypeConstants(t *testing.T) {
	if OwnerChatSession != "chat" {
		t.Errorf("OwnerChatSession = %q", OwnerChatSession)
	}
	if OwnerWorkflow != "workflows" {
		t.Errorf("OwnerWorkflow = %q", OwnerWorkflow)
	}
}

// ---------------------------------------------------------------------------
// Save with large content under limit
// ---------------------------------------------------------------------------

func TestStore_Save_ExactLimit(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create content exactly at the limit
	exactSize := bytes.Repeat([]byte("x"), MaxAttachmentSizeBytes)
	meta, err := s.Save(OwnerWorkflow, "wf-exact", bytes.NewReader(exactSize), "exact.bin")
	if err != nil {
		t.Fatalf("Save at exact limit: %v", err)
	}
	if meta.Size != int64(MaxAttachmentSizeBytes) {
		t.Errorf("size = %d, want %d", meta.Size, MaxAttachmentSizeBytes)
	}
}

// ---------------------------------------------------------------------------
// Save with empty content
// ---------------------------------------------------------------------------

func TestStore_Save_EmptyContent(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	meta, err := s.Save(OwnerWorkflow, "wf-empty", strings.NewReader(""), "empty.txt")
	if err != nil {
		t.Fatalf("Save empty: %v", err)
	}
	if meta.Size != 0 {
		t.Errorf("expected size 0, got %d", meta.Size)
	}
}

// ---------------------------------------------------------------------------
// List: non-IsNotExist error (permissions)
// ---------------------------------------------------------------------------

func TestStore_ListPermissionError(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create the owner directory then make it unreadable
	ownerDir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-noperm")
	os.MkdirAll(ownerDir, 0o750)

	// Make directory unreadable
	os.Chmod(ownerDir, 0o000)
	t.Cleanup(func() { os.Chmod(ownerDir, 0o750) })

	_, err := s.List(OwnerWorkflow, "wf-noperm")
	if err == nil {
		t.Skip("permissions not enforced")
	}
	// Error should NOT be "not found" but rather a permission error
}

// ---------------------------------------------------------------------------
// Resolve: non-IsNotExist error (permissions on meta.json)
// ---------------------------------------------------------------------------

func TestStore_ResolvePermissionError(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	// Create the attachment dir with meta.json but make it unreadable
	dir := filepath.Join(s.BaseDir(), string(OwnerWorkflow), "wf-perm", "att-perm")
	os.MkdirAll(dir, 0o750)
	metaPath := filepath.Join(dir, "meta.json")
	os.WriteFile(metaPath, []byte(`{"id":"att-perm"}`), 0o600)

	// Make meta.json unreadable
	os.Chmod(metaPath, 0o000)
	t.Cleanup(func() { os.Chmod(metaPath, 0o600) })

	_, _, err := s.Resolve(OwnerWorkflow, "wf-perm", "att-perm")
	if err == nil {
		t.Skip("permissions not enforced")
	}
}

// ---------------------------------------------------------------------------
// Multiple attachments for same owner
// ---------------------------------------------------------------------------

func TestStore_MultipleAttachments(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)

	for i := 0; i < 5; i++ {
		_, err := s.Save(OwnerWorkflow, "wf-multi", strings.NewReader("content"), "file.txt")
		if err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	list, err := s.List(OwnerWorkflow, "wf-multi")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 5 {
		t.Errorf("expected 5 attachments, got %d", len(list))
	}
}
