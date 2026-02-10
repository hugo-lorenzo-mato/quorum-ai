package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockLogger struct {
	debugMsgs []string
	infoMsgs  []string
	warnMsgs  []string
}

func (m *mockLogger) Debug(msg string, args ...interface{}) { m.debugMsgs = append(m.debugMsgs, msg) }
func (m *mockLogger) Info(msg string, args ...interface{})  { m.infoMsgs = append(m.infoMsgs, msg) }
func (m *mockLogger) Warn(msg string, args ...interface{})  { m.warnMsgs = append(m.warnMsgs, msg) }

func TestNewFileEnforcement(t *testing.T) {
	fe := NewFileEnforcement(nil)
	if fe == nil {
		t.Fatal("expected non-nil")
	}
}

func TestFileEnforcement_EnsureDirectory(t *testing.T) {
	fe := NewFileEnforcement(nil)

	// Empty path should be no-op
	if err := fe.EnsureDirectory(""); err != nil {
		t.Errorf("empty path should succeed: %v", err)
	}

	// Create in temp dir
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "dir", "file.txt")
	if err := fe.EnsureDirectory(target); err != nil {
		t.Fatalf("EnsureDirectory failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(filepath.Dir(target))
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestFileEnforcement_VerifyOrWriteFallback_EmptyPath(t *testing.T) {
	fe := NewFileEnforcement(nil)
	created, err := fe.VerifyOrWriteFallback("", "output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected false for empty path")
	}
}

func TestFileEnforcement_VerifyOrWriteFallback_FileExists(t *testing.T) {
	logger := &mockLogger{}
	fe := NewFileEnforcement(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	created, err := fe.VerifyOrWriteFallback(path, "unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected createdByLLM=true for existing file")
	}
}

func TestFileEnforcement_VerifyOrWriteFallback_FallbackFromStdout(t *testing.T) {
	logger := &mockLogger{}
	fe := NewFileEnforcement(logger)

	dir := t.TempDir()
	path := filepath.Join(dir, "new-file.txt")

	created, err := fe.VerifyOrWriteFallback(path, "stdout content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected createdByLLM=false for fallback")
	}

	// Verify file was written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "stdout content" {
		t.Errorf("got content %q", string(data))
	}
}

func TestFileEnforcement_VerifyOrWriteFallback_NoStdout(t *testing.T) {
	fe := NewFileEnforcement(nil)

	dir := t.TempDir()
	path := filepath.Join(dir, "missing.txt")

	_, err := fe.VerifyOrWriteFallback(path, "")
	if err == nil {
		t.Fatal("expected error when no file and no stdout")
	}
	if !strings.Contains(err.Error(), "no stdout available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFileEnforcement_ValidateBeforeCheckpoint(t *testing.T) {
	fe := NewFileEnforcement(nil)

	// Empty path is valid
	if err := fe.ValidateBeforeCheckpoint(""); err != nil {
		t.Errorf("empty path should succeed: %v", err)
	}

	// Existing file is valid
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("x"), 0o600)
	if err := fe.ValidateBeforeCheckpoint(path); err != nil {
		t.Errorf("existing file should succeed: %v", err)
	}

	// Missing file fails
	if fe.ValidateBeforeCheckpoint(filepath.Join(dir, "missing.txt")) == nil {
		t.Error("expected error for missing file")
	}
}

func TestFileEnforcement_EnsureAndVerify(t *testing.T) {
	logger := &mockLogger{}
	fe := NewFileEnforcement(logger)

	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "file.txt")

	// Phase 1: ensure only
	created, err := fe.EnsureAndVerify(target, "", true)
	if err != nil {
		t.Fatalf("EnsureAndVerify (ensureOnly) failed: %v", err)
	}
	if created {
		t.Error("expected false for ensureOnly")
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(target)); err != nil {
		t.Fatal("directory should exist")
	}

	// Phase 2: verify with fallback
	created, err = fe.EnsureAndVerify(target, "fallback content", false)
	if err != nil {
		t.Fatalf("EnsureAndVerify (verify) failed: %v", err)
	}
	if created {
		t.Error("expected false (created from fallback)")
	}
}
