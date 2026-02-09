package workflow

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type nopLogger struct{}

func (nopLogger) Debug(_ string, _ ...interface{}) {}
func (nopLogger) Info(_ string, _ ...interface{})  {}
func (nopLogger) Warn(_ string, _ ...interface{})  {}

func TestOutputWatchdog_FileDoesNotExist(t *testing.T) {
	cfg := OutputWatchdogConfig{
		PollInterval:    10 * time.Millisecond,
		StabilityWindow: 30 * time.Millisecond,
		MinFileSize:     10,
	}
	w := NewOutputWatchdog("/nonexistent/path/file.md", cfg, nopLogger{})
	w.Start()
	defer w.Stop()

	select {
	case <-w.StableCh():
		t.Fatal("should not signal for nonexistent file")
	case <-time.After(100 * time.Millisecond):
		// Expected: no signal
	}
}

func TestOutputWatchdog_FileTooSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")
	if err := os.WriteFile(path, []byte("tiny"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := OutputWatchdogConfig{
		PollInterval:    10 * time.Millisecond,
		StabilityWindow: 30 * time.Millisecond,
		MinFileSize:     512,
	}
	w := NewOutputWatchdog(path, cfg, nopLogger{})
	w.Start()
	defer w.Stop()

	select {
	case <-w.StableCh():
		t.Fatal("should not signal for file below MinFileSize")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestOutputWatchdog_GrowingFileThenStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")

	cfg := OutputWatchdogConfig{
		PollInterval:    10 * time.Millisecond,
		StabilityWindow: 40 * time.Millisecond,
		MinFileSize:     10,
	}
	w := NewOutputWatchdog(path, cfg, nopLogger{})
	w.Start()
	defer w.Stop()

	// Write growing content
	content := make([]byte, 100)
	for i := range content {
		content[i] = 'A'
	}
	if err := os.WriteFile(path, content[:50], 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	// Grow the file
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for stability
	select {
	case got := <-w.StableCh():
		if len(got) != 100 {
			t.Fatalf("expected 100 bytes, got %d", len(got))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stable signal")
	}
}

func TestOutputWatchdog_StableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.md")

	content := make([]byte, 1024)
	for i := range content {
		content[i] = 'B'
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := OutputWatchdogConfig{
		PollInterval:    10 * time.Millisecond,
		StabilityWindow: 30 * time.Millisecond,
		MinFileSize:     10,
	}
	w := NewOutputWatchdog(path, cfg, nopLogger{})
	w.Start()
	defer w.Stop()

	select {
	case got := <-w.StableCh():
		if len(got) != 1024 {
			t.Fatalf("expected 1024 bytes, got %d", len(got))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for stable signal")
	}
}

func TestOutputWatchdog_StopClean(t *testing.T) {
	cfg := OutputWatchdogConfig{
		PollInterval:    10 * time.Millisecond,
		StabilityWindow: 30 * time.Millisecond,
		MinFileSize:     10,
	}
	w := NewOutputWatchdog("/nonexistent", cfg, nopLogger{})
	w.Start()

	// Stop should not panic
	w.Stop()
	w.Stop() // Double-stop should be safe
}
