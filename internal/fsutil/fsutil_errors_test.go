package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ReadFileScoped: additional coverage
// ---------------------------------------------------------------------------

func TestReadFileScoped_NonexistentFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "does-not-exist.txt")

	_, err := ReadFileScoped(p)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadFileScoped_NonexistentDirectory(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nodir", "file.txt")

	_, err := ReadFileScoped(p)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestReadFileScoped_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(p, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty content, got %d bytes", len(data))
	}
}

func TestReadFileScoped_LargeFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "large.txt")
	content := make([]byte, 1024*1024) // 1MB
	for i := range content {
		content[i] = byte('A' + (i % 26))
	}
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(data))
	}
}

func TestReadFileScoped_NestedPath(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(nested, "deep.txt")
	if err := os.WriteFile(p, []byte("deep"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if string(data) != "deep" {
		t.Errorf("expected %q, got %q", "deep", string(data))
	}
}

func TestReadFileScoped_UnnormalizedPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(p, []byte("norm"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Use a path with "." components
	unnormalized := filepath.Join(dir, ".", "file.txt")
	data, err := ReadFileScoped(unnormalized)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if string(data) != "norm" {
		t.Errorf("expected %q, got %q", "norm", string(data))
	}
}

func TestReadFileScoped_BinaryContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "binary.bin")
	content := []byte{0x00, 0x01, 0xFF, 0xFE, 0x89, 0x50, 0x4E, 0x47}
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if len(data) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(data))
	}
	for i, b := range data {
		if b != content[i] {
			t.Errorf("byte %d: expected %x, got %x", i, content[i], b)
		}
	}
}

func TestReadFileScoped_DirectoryAsPath(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	os.MkdirAll(subdir, 0o750)

	// Trying to read a directory should fail
	_, err := ReadFileScoped(subdir)
	if err == nil {
		t.Error("expected error when reading directory as file")
	}
}

func TestReadFileScoped_SpecialCharsInFilename(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file with spaces & chars.txt")
	if err := os.WriteFile(p, []byte("special"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped: %v", err)
	}
	if string(data) != "special" {
		t.Errorf("expected %q, got %q", "special", string(data))
	}
}
