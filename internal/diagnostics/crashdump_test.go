package diagnostics

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCrashDumpWriter(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	writer := NewCrashDumpWriter(
		"",    // default dir
		0,     // default max files
		true,  // include stack
		false, // don't include env
		logger,
		nil,
	)

	if writer == nil {
		t.Fatal("expected non-nil writer")
	}

	if writer.dir != ".quorum/crashdumps" {
		t.Errorf("expected default dir '.quorum/crashdumps', got %s", writer.dir)
	}

	if writer.maxFiles != 10 {
		t.Errorf("expected default maxFiles 10, got %d", writer.maxFiles)
	}
}

func TestCrashDumpWriter_SetCurrentContext(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	writer := NewCrashDumpWriter("", 10, true, false, logger, nil)

	writer.SetCurrentContext("analyze", "task-123")

	phase, ok := writer.currentPhase.Load().(string)
	if !ok || phase != "analyze" {
		t.Errorf("expected phase 'analyze', got %v", phase)
	}

	task, ok := writer.currentTask.Load().(string)
	if !ok || task != "task-123" {
		t.Errorf("expected task 'task-123', got %v", task)
	}
}

func TestCrashDumpWriter_SetCurrentCommand(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	writer := NewCrashDumpWriter("", 10, true, false, logger, nil)

	cmdCtx := &CommandContext{
		Path:    "/usr/bin/claude",
		Args:    []string{"--model", "opus"},
		WorkDir: "/tmp",
	}

	writer.SetCurrentCommand(cmdCtx)

	loaded := writer.currentCmd.Load()
	if loaded == nil {
		t.Fatal("expected non-nil command context")
	}

	loadedCmd, ok := loaded.(*CommandContext)
	if !ok {
		t.Fatalf("expected *CommandContext, got %T", loaded)
	}

	if loadedCmd.Path != "/usr/bin/claude" {
		t.Errorf("expected path '/usr/bin/claude', got %s", loadedCmd.Path)
	}

	// Clear command
	writer.ClearCurrentCommand()

	loaded = writer.currentCmd.Load()
	if loaded != (*CommandContext)(nil) {
		t.Error("expected nil command context after clear")
	}
}

func TestCrashDumpWriter_WriteCrashDump(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory for test
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(
		tempDir,
		5,
		true,  // include stack
		false, // don't include env
		logger,
		nil,
	)

	// Set context
	writer.SetCurrentContext("execute", "task-456")

	// Write crash dump
	path, err := writer.WriteCrashDump("test panic value")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	if path == "" {
		t.Fatal("expected non-empty path")
	}

	if !strings.HasPrefix(path, tempDir) {
		t.Errorf("expected path in temp dir, got %s", path)
	}

	// Verify file exists and is valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read crash dump: %v", err)
	}

	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("failed to parse crash dump JSON: %v", err)
	}

	// Verify dump contents
	if dump.PanicValue != "test panic value" {
		t.Errorf("expected panic value 'test panic value', got %s", dump.PanicValue)
	}

	if dump.CurrentPhase != "execute" {
		t.Errorf("expected phase 'execute', got %s", dump.CurrentPhase)
	}

	if dump.CurrentTask != "task-456" {
		t.Errorf("expected task 'task-456', got %s", dump.CurrentTask)
	}

	if dump.StackTrace == "" {
		t.Error("expected non-empty stack trace")
	}
}

func TestCrashDumpWriter_CleanupOldDumps(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory for test
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(
		tempDir,
		3, // Keep only 3 files
		false,
		false,
		logger,
		nil,
	)

	// Write more than maxFiles dumps
	for i := 0; i < 5; i++ {
		_, err := writer.WriteCrashDump("panic " + string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to write dump %d: %v", i, err)
		}
	}

	// Check that only maxFiles dumps remain
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	crashDumps := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") && strings.HasSuffix(e.Name(), ".json") {
			crashDumps++
		}
	}

	if crashDumps > 3 {
		t.Errorf("expected at most 3 crash dumps, found %d", crashDumps)
	}
}

func TestLoadLatestCrashDump(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create temp directory for test
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	// Write a crash dump
	_, err := writer.WriteCrashDump("latest panic")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	// Load it back
	dump, err := LoadLatestCrashDump(tempDir)
	if err != nil {
		t.Fatalf("failed to load latest crash dump: %v", err)
	}

	if dump.PanicValue != "latest panic" {
		t.Errorf("expected panic value 'latest panic', got %s", dump.PanicValue)
	}
}

func TestLoadLatestCrashDump_NoDumps(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	_, err := LoadLatestCrashDump(tempDir)
	if err == nil {
		t.Error("expected error when no dumps exist")
	}
}

func TestCrashDumpWriter_RedactEnvironment(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	writer := NewCrashDumpWriter("", 10, false, true, logger, nil)

	// Set some test environment variables
	os.Setenv("TEST_API_KEY", "secret123")
	os.Setenv("TEST_NORMAL_VAR", "visible")
	defer os.Unsetenv("TEST_API_KEY")
	defer os.Unsetenv("TEST_NORMAL_VAR")

	redacted := writer.redactEnvironment()

	// API_KEY should be redacted
	if val, ok := redacted["TEST_API_KEY"]; ok {
		if val != "[REDACTED]" {
			t.Errorf("expected TEST_API_KEY to be redacted, got %s", val)
		}
	}

	// NORMAL_VAR should be visible
	if val, ok := redacted["TEST_NORMAL_VAR"]; ok {
		if val != "visible" {
			t.Errorf("expected TEST_NORMAL_VAR to be 'visible', got %s", val)
		}
	}
}

func TestCrashDumpWriter_RecoverAndReturn(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	var capturedErr error

	func() {
		defer writer.RecoverAndReturn(&capturedErr)
		panic("test panic")
	}()

	if capturedErr == nil {
		t.Fatal("expected captured error after panic")
	}

	if !strings.Contains(capturedErr.Error(), "test panic") {
		t.Errorf("expected error to contain 'test panic', got %s", capturedErr.Error())
	}

	// Verify dump was written
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected crash dump file to be written")
	}
}

func TestCrashDumpWriter_WriteCrashDump_WithMonitor(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tempDir := t.TempDir()

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	monitor.TakeSnapshot() // Take at least one snapshot

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, monitor)

	path, err := writer.WriteCrashDump("panic with monitor")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	// Load and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read crash dump: %v", err)
	}

	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("failed to parse crash dump: %v", err)
	}

	// Should have resource state
	if dump.ResourceState.Timestamp.IsZero() {
		t.Error("expected non-zero resource state timestamp")
	}
}

func init() {
	// Ensure test temp dirs are cleaned up
	filepath.Clean(os.TempDir())
}
