package diagnostics

import (
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestNewSafeExecutor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(
		nil, // no monitor
		nil, // no dump writer
		logger,
		true, // preflight enabled
		20,   // min free FD percent
		256,  // min free memory MB
	)

	if executor == nil {
		t.Fatal("expected non-nil executor")
	}

	if !executor.preflightEnabled {
		t.Error("expected preflight to be enabled")
	}
}

func TestSafeExecutor_RunPreflight_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(
		nil,
		nil,
		logger,
		false, // preflight disabled
		20,
		256,
	)

	result := executor.RunPreflight()

	if !result.OK {
		t.Error("expected OK when preflight is disabled")
	}
}

func TestSafeExecutor_RunPreflight_NoMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(
		nil, // no monitor
		nil,
		logger,
		true,
		20,
		256,
	)

	result := executor.RunPreflight()

	if !result.OK {
		t.Error("expected OK when no monitor is available")
	}
}

func TestSafeExecutor_RunPreflight_WithMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(
		monitor,
		nil,
		logger,
		true,
		20, // 20% free FDs required
		256,
	)

	result := executor.RunPreflight()

	// Under normal conditions, preflight should pass
	if !result.OK {
		t.Logf("Preflight failed: %v", result.Errors)
	}

	// Should have a snapshot
	if result.Snapshot.Timestamp.IsZero() {
		t.Error("expected non-zero snapshot timestamp")
	}
}

func TestSafeExecutor_PrepareCommand(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(
		monitor,
		nil,
		logger,
		true,
		20,
		256,
	)

	// Create a simple command
	cmd := exec.Command("echo", "hello")

	pipes, err := executor.PrepareCommand(cmd)
	if err != nil {
		t.Fatalf("failed to prepare command: %v", err)
	}

	if pipes == nil {
		t.Fatal("expected non-nil pipes")
	}

	if pipes.Stdout == nil {
		t.Error("expected non-nil stdout pipe")
	}

	if pipes.Stderr == nil {
		t.Error("expected non-nil stderr pipe")
	}

	// Active commands should be incremented
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 1 {
		t.Errorf("expected 1 active command, got %d", snapshot.CommandsActive)
	}

	// Cleanup
	pipes.Cleanup()

	// Active commands should be decremented
	snapshot = monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("expected 0 active commands after cleanup, got %d", snapshot.CommandsActive)
	}
}

func TestSafeExecutor_PrepareCommand_DoubleCleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(
		monitor,
		nil,
		logger,
		true,
		20,
		256,
	)

	cmd := exec.Command("echo", "hello")

	pipes, err := executor.PrepareCommand(cmd)
	if err != nil {
		t.Fatalf("failed to prepare command: %v", err)
	}

	// Cleanup multiple times should be safe
	pipes.Cleanup()
	pipes.Cleanup()
	pipes.Cleanup()

	// Should still be 0
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("expected 0 active commands after multiple cleanups, got %d", snapshot.CommandsActive)
	}
}

func TestSafeExecutor_PrepareStderrOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(
		monitor,
		nil,
		logger,
		true,
		20,
		256,
	)

	cmd := exec.Command("echo", "hello")

	stderr, cleanup, err := executor.PrepareStderrOnly(cmd)
	if err != nil {
		t.Fatalf("failed to prepare stderr: %v", err)
	}

	if stderr == nil {
		t.Error("expected non-nil stderr pipe")
	}

	if cleanup == nil {
		t.Error("expected non-nil cleanup function")
	}

	// Active commands should be incremented
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 1 {
		t.Errorf("expected 1 active command, got %d", snapshot.CommandsActive)
	}

	// Cleanup
	cleanup()

	// Active commands should be decremented
	snapshot = monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("expected 0 active commands after cleanup, got %d", snapshot.CommandsActive)
	}
}

func TestSafeExecutor_WrapExecution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tempDir := t.TempDir()
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	dumpWriter := NewCrashDumpWriter(tempDir, 10, true, false, logger, monitor)

	executor := NewSafeExecutor(
		monitor,
		dumpWriter,
		logger,
		true,
		20,
		256,
	)

	// Test normal execution
	var executed bool
	err := executor.WrapExecution(func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !executed {
		t.Error("expected function to be executed")
	}
}

func TestSafeExecutor_WrapExecution_WithPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tempDir := t.TempDir()
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)
	dumpWriter := NewCrashDumpWriter(tempDir, 10, true, false, logger, monitor)

	executor := NewSafeExecutor(
		monitor,
		dumpWriter,
		logger,
		true,
		20,
		256,
	)

	// Test execution with panic
	err := executor.WrapExecution(func() error {
		panic("test panic in WrapExecution")
	})

	if err == nil {
		t.Error("expected error after panic")
	}

	// Error message should contain the panic value
	if err != nil && !containsSubstring(err.Error(), "test panic") {
		t.Errorf("expected error to contain panic message, got %s", err.Error())
	}
}

func TestSafeExecutor_WrapExecution_NoDumpWriter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(
		nil,
		nil, // no dump writer
		logger,
		true,
		20,
		256,
	)

	// Test normal execution without dump writer
	err := executor.WrapExecution(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestPipeSet_Cleanup(t *testing.T) {
	pipeSet := &PipeSet{
		cleaned: false,
	}

	// First cleanup should work
	pipeSet.Cleanup()

	if !pipeSet.cleaned {
		t.Error("expected cleaned to be true after Cleanup()")
	}

	// Second cleanup should be a no-op
	pipeSet.Cleanup()
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
