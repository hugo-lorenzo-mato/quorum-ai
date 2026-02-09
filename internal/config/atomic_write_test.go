package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestAtomicWrite_BasicOperation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := []byte("log:\n  level: info\n")
	if err := AtomicWrite(configPath, content); err != nil {
		t.Fatalf("AtomicWrite error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("content mismatch: got %q, want %q", string(data), string(content))
	}

	files, _ := filepath.Glob(filepath.Join(tmpDir, ".config.yaml.*"))
	if len(files) != 0 {
		t.Fatalf("expected no temp files, found %d", len(files))
	}
}

func TestAtomicWrite_OverwriteExisting(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("original"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	newContent := []byte("updated")
	if err := AtomicWrite(configPath, newContent); err != nil {
		t.Fatalf("AtomicWrite error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != string(newContent) {
		t.Fatalf("content mismatch: got %q, want %q", string(data), string(newContent))
	}
}

func TestAtomicWrite_PreservesPermissions(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - Unix permissions not supported")
	}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("original"), 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := AtomicWrite(configPath, []byte("updated")); err != nil {
		t.Fatalf("AtomicWrite error: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if info.Mode().Perm() != os.FileMode(0o600) {
		t.Fatalf("expected perms 0600, got %v", info.Mode().Perm())
	}
}

func TestAtomicWrite_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("initial"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			content := []byte(fmt.Sprintf("content from goroutine %d", n))
			_ = AtomicWrite(configPath, content)
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !strings.Contains(string(data), "content from goroutine") {
		t.Fatalf("expected content from goroutine, got %q", string(data))
	}
}

func TestCalculateETag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "simple content",
			content: []byte("log:\n  level: info\n"),
		},
		{
			name:    "empty content",
			content: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			etag1 := CalculateETag(tt.content)
			etag2 := CalculateETag(tt.content)

			if etag1 != etag2 {
				t.Fatalf("expected ETag to be consistent")
			}
			if len(etag1) == 0 || etag1[0] != '"' || etag1[len(etag1)-1] != '"' {
				t.Fatalf("expected quoted ETag, got %q", etag1)
			}
		})
	}
}

func TestCalculateETag_DifferentContent(t *testing.T) {
	t.Parallel()
	content1 := []byte("log:\n  level: info\n")
	content2 := []byte("log:\n  level: debug\n")

	etag1 := CalculateETag(content1)
	etag2 := CalculateETag(content2)

	if etag1 == etag2 {
		t.Fatal("expected different content to produce different ETags")
	}
}

func TestAtomicWrite_FailsOnInvalidPath(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping invalid path test on Windows - path handling differs")
	}
	err := AtomicWrite("/nonexistent/directory/config.yaml", []byte("content"))
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
