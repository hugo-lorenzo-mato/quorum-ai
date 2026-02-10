package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileScoped_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	b, err := ReadFileScoped(p)
	if err != nil {
		t.Fatalf("ReadFileScoped error: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("unexpected content: %q", string(b))
	}
}

func TestReadFileScoped_RejectsInvalidPath(t *testing.T) {
	for _, p := range []string{"", ".", string(filepath.Separator)} {
		if _, err := ReadFileScoped(p); err == nil {
			t.Fatalf("expected error for %q", p)
		}
	}
}

