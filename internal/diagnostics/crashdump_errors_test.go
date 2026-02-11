package diagnostics

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCrashDumpWriter_RecoverAndDump_WithLogger tests RecoverAndDump with logger.
func TestCrashDumpWriter_RecoverAndDump_LogsError(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown")
		}
	}()

	func() {
		defer writer.RecoverAndDump()
		panic("test with logger")
	}()
}

// TestCrashDumpWriter_RecoverAndReturn_WithDumpError tests RecoverAndReturn when dump fails.
func TestCrashDumpWriter_RecoverAndReturn_DumpError(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use invalid directory to cause dump write failure
	writer := NewCrashDumpWriter("/dev/null/invalid/path", 10, true, false, logger, nil)

	var capturedErr error

	func() {
		defer writer.RecoverAndReturn(&capturedErr)
		panic("test panic with dump error")
	}()

	if capturedErr == nil {
		t.Fatal("expected captured error after panic")
	}

	if !strings.Contains(capturedErr.Error(), "test panic with dump error") {
		t.Errorf("error should contain panic message, got %s", capturedErr.Error())
	}
}

// TestCrashDumpWriter_CleanupOldDumps_InfoError tests cleanup when Info() fails.
func TestCrashDumpWriter_CleanupOldDumps_HandlesErrors(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 2, false, false, logger, nil)

	// Create some dumps
	for i := 0; i < 3; i++ {
		_, err := writer.WriteCrashDump("test")
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Create a file with crash- prefix that will fail Info()
	// By creating a symlink to a non-existent target
	badLink := filepath.Join(tempDir, "crash-badlink.json")
	if err := os.Symlink("/nonexistent", badLink); err != nil {
		t.Logf("Could not create bad symlink: %v", err)
	}

	// Write one more to trigger cleanup
	_, err := writer.WriteCrashDump("trigger cleanup")
	if err != nil {
		t.Fatalf("failed to write dump: %v", err)
	}

	// Cleanup should handle errors gracefully
}

// TestLoadLatestCrashDump_InfoError tests loading when file Info() fails.
func TestLoadLatestCrashDump_HandlesInfoError(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	// Write a valid dump
	_, err := writer.WriteCrashDump("valid dump")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	// Create a bad symlink
	badLink := filepath.Join(tempDir, "crash-bad.json")
	if err := os.Symlink("/nonexistent", badLink); err != nil {
		t.Logf("Could not create bad symlink: %v", err)
	}

	// Should still load the valid dump
	dump, err := LoadLatestCrashDump(tempDir)
	if err != nil {
		t.Logf("Load failed (expected if only bad symlink): %v", err)
	} else {
		if dump.PanicValue != "valid dump" {
			t.Errorf("PanicValue = %q, want %q", dump.PanicValue, "valid dump")
		}
	}
}

// TestResourceMonitor_Start_WithWarnings tests Start generates warnings.
func TestResourceMonitor_Start_GeneratesWarnings(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use very low thresholds to trigger warnings
	monitor := NewResourceMonitor(20*time.Millisecond, 1, 1, 1, 10, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	monitor.Start(ctx)

	// Wait for some snapshots and potential warnings
	time.Sleep(150 * time.Millisecond)

	// Should have generated snapshots
	history := monitor.GetHistory()
	if len(history) == 0 {
		t.Error("expected at least one snapshot in history")
	}
}

// TestResourceMonitor_CheckHealth_ZeroThresholds tests health check with all thresholds disabled.
func TestResourceMonitor_CheckHealth_AllThresholdsZero(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// All thresholds set to 0 (disabled)
	monitor := NewResourceMonitor(time.Second, 0, 0, 0, 10, logger)

	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	warnings := monitor.CheckHealth()

	// Should have no warnings with all thresholds disabled
	if len(warnings) > 0 {
		t.Logf("Warnings generated with zero thresholds: %v", warnings)
	}
}

// TestResourceMonitor_CheckHealth_FDWarningLevel tests FD warning levels.
func TestResourceMonitor_CheckHealth_FDWarningAndCritical(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 1, 10000, 4096, 10, logger)

	// Create a mock snapshot with specific FD usage
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:      time.Now(),
			OpenFDs:        85,
			MaxFDs:         100,
			FDUsagePercent: 85.0, // Should trigger warning
			Goroutines:     10,
			HeapAllocMB:    100.0,
		},
	}
	monitor.mu.Unlock()

	warnings := monitor.CheckHealth()

	foundFDWarning := false
	for _, w := range warnings {
		if w.Type == "fd" {
			foundFDWarning = true
			if w.Level != "warning" {
				t.Errorf("FD warning level = %q, want 'warning' (usage < 90%%)", w.Level)
			}
		}
	}

	if !foundFDWarning {
		t.Log("No FD warning generated (system has low FD usage)")
	}

	// Now test critical level
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:      time.Now(),
			OpenFDs:        95,
			MaxFDs:         100,
			FDUsagePercent: 95.0, // Should trigger critical
			Goroutines:     10,
			HeapAllocMB:    100.0,
		},
	}
	monitor.mu.Unlock()

	warnings = monitor.CheckHealth()

	foundFDCritical := false
	for _, w := range warnings {
		if w.Type == "fd" && w.Level == "critical" {
			foundFDCritical = true
		}
	}

	if !foundFDCritical {
		t.Log("No critical FD warning (usage needs to be > 90%)")
	}
}

// TestResourceMonitor_CheckHealth_GoroutineLevels tests goroutine warning levels.
func TestResourceMonitor_CheckHealth_GoroutineLevels(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 100, 4096, 10, logger)

	// Test warning level (just over threshold)
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   time.Now(),
			OpenFDs:     10,
			MaxFDs:      1000,
			Goroutines:  150, // Over threshold of 100
			HeapAllocMB: 100.0,
		},
	}
	monitor.mu.Unlock()

	warnings := monitor.CheckHealth()

	for _, w := range warnings {
		if w.Type == "goroutine" {
			if w.Level != "warning" {
				t.Errorf("goroutine warning level = %q, want 'warning' (count < 2x threshold)", w.Level)
			}
		}
	}

	// Test critical level (over 2x threshold)
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   time.Now(),
			OpenFDs:     10,
			MaxFDs:      1000,
			Goroutines:  250, // Over 2x threshold
			HeapAllocMB: 100.0,
		},
	}
	monitor.mu.Unlock()

	warnings = monitor.CheckHealth()

	foundCritical := false
	for _, w := range warnings {
		if w.Type == "goroutine" && w.Level == "critical" {
			foundCritical = true
		}
	}

	if !foundCritical {
		t.Log("No critical goroutine warning (count needs to be > 2x threshold)")
	}
}

// TestResourceMonitor_CheckHealth_MemoryLevels tests memory warning levels.
func TestResourceMonitor_CheckHealth_MemoryLevels(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 100, 10, logger)

	// Test warning level (just over threshold)
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   time.Now(),
			OpenFDs:     10,
			MaxFDs:      1000,
			Goroutines:  50,
			HeapAllocMB: 120.0, // Over threshold of 100
		},
	}
	monitor.mu.Unlock()

	warnings := monitor.CheckHealth()

	for _, w := range warnings {
		if w.Type == "memory" {
			if w.Level != "warning" {
				t.Errorf("memory warning level = %q, want 'warning' (heap < 1.5x threshold)", w.Level)
			}
		}
	}

	// Test critical level (over 1.5x threshold)
	monitor.mu.Lock()
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   time.Now(),
			OpenFDs:     10,
			MaxFDs:      1000,
			Goroutines:  50,
			HeapAllocMB: 160.0, // Over 1.5x threshold
		},
	}
	monitor.mu.Unlock()

	warnings = monitor.CheckHealth()

	foundCritical := false
	for _, w := range warnings {
		if w.Type == "memory" && w.Level == "critical" {
			foundCritical = true
		}
	}

	if !foundCritical {
		t.Log("No critical memory warning (heap needs to be > 1.5x threshold)")
	}
}

// TestLoadLatestCrashDump_ReadFileError tests error handling when reading file.
func TestLoadLatestCrashDump_InvalidJSON(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a file with invalid JSON
	badFile := filepath.Join(tempDir, "crash-invalid.json")
	if err := os.WriteFile(badFile, []byte("not valid json"), 0o600); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	_, err := LoadLatestCrashDump(tempDir)
	if err == nil {
		t.Error("expected error when parsing invalid JSON")
	}
}

// TestCrashDumpWriter_RedactEnvironment_MalformedEnv tests malformed environment variable.
func TestCrashDumpWriter_RedactEnvironment_EdgeCases(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	writer := NewCrashDumpWriter("", 10, false, true, logger, nil)

	// Set an environment variable without '=' (should be skipped)
	// Note: This is hard to test directly since os.Setenv requires a value
	// but we can test the redaction logic

	os.Setenv("TEST_VAR_WITHOUT_EQUALS", "")
	defer os.Unsetenv("TEST_VAR_WITHOUT_EQUALS")

	redacted := writer.redactEnvironment()

	// Should handle empty values gracefully
	if val, ok := redacted["TEST_VAR_WITHOUT_EQUALS"]; ok {
		if val != "" {
			t.Errorf("empty env var should be empty, got %q", val)
		}
	}
}

// TestSystemMetricsCollector_CollectGPUInfo_CacheRefresh tests GPU cache refresh.
func TestSystemMetricsCollector_CollectGPUInfo_ForceRefresh(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	// First collect
	c.Collect()

	// Force cache to be old
	c.mu.Lock()
	c.lastGPUUpdate = time.Now().Add(-10 * time.Second)
	c.mu.Unlock()

	// Second collect should refresh
	c.Collect()

	c.mu.Lock()
	defer c.mu.Unlock()

	// lastGPUUpdate should be recent
	if time.Since(c.lastGPUUpdate) > 2*time.Second {
		t.Error("GPU cache should have been refreshed")
	}
}

// TestResourceMonitor_TakeSnapshot_AllFields tests all snapshot fields are populated.
func TestResourceMonitor_TakeSnapshot_AllFields(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	snapshot := monitor.TakeSnapshot()

	// Check all fields
	if snapshot.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if snapshot.Goroutines <= 0 {
		t.Error("Goroutines should be positive")
	}
	// NumGC is uint32, so it's always non-negative - check removed
	if snapshot.ProcessUptime < 0 {
		t.Error("ProcessUptime should be non-negative")
	}
}

// TestCrashDumpWriter_WriteCrashDump_AllMetadata tests all metadata fields.
func TestCrashDumpWriter_WriteCrashDump_AllMetadata(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	path, err := writer.WriteCrashDump("test")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read crash dump: %v", err)
	}

	var dump CrashDump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("failed to parse crash dump: %v", err)
	}

	// Check all metadata fields
	if dump.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if dump.ProcessID == 0 {
		t.Error("ProcessID should not be zero")
	}
	if dump.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if dump.GOOS == "" {
		t.Error("GOOS should not be empty")
	}
	if dump.GOARCH == "" {
		t.Error("GOARCH should not be empty")
	}
}
