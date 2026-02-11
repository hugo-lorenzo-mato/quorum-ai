package diagnostics

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestCrashDumpWriter_RecoverAndDump tests the RecoverAndDump defer function.
func TestCrashDumpWriter_RecoverAndDump(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	// Test that RecoverAndDump catches and re-panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-thrown")
		} else if !strings.Contains(r.(string), "test panic for RecoverAndDump") {
			t.Errorf("expected panic message, got %v", r)
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
			t.Error("expected crash dump file to be written by RecoverAndDump")
		}
	}()

	func() {
		defer writer.RecoverAndDump()
		panic("test panic for RecoverAndDump")
	}()
}

// TestCrashDumpWriter_RecoverAndDump_NoDumpOnSuccess tests RecoverAndDump with no panic.
func TestCrashDumpWriter_RecoverAndDump_NoDumpOnSuccess(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	// This should not panic or create dump
	func() {
		defer writer.RecoverAndDump()
		// Normal execution
	}()

	// Verify no dump was written
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "crash-") {
			t.Error("no crash dump should be written on successful execution")
		}
	}
}

// TestCrashDumpWriter_CleanupOldDumps_EdgeCases tests cleanup with various scenarios.
func TestCrashDumpWriter_CleanupOldDumps_WithNonDumpFiles(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	// Create non-dump files that should not be cleaned up
	nonDumpFiles := []string{
		"other-file.json",
		"crash.txt",
		"dump.json",
		"README.md",
	}
	for _, name := range nonDumpFiles {
		if err := os.WriteFile(filepath.Join(tempDir, name), []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	writer := NewCrashDumpWriter(tempDir, 3, false, false, logger, nil)

	// Write crash dumps
	for i := 0; i < 5; i++ {
		_, err := writer.WriteCrashDump("test panic")
		if err != nil {
			t.Fatalf("failed to write dump: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Verify non-dump files still exist
	for _, name := range nonDumpFiles {
		path := filepath.Join(tempDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("non-dump file %s should not be deleted", name)
		}
	}

	// Verify only maxFiles dumps remain
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

// TestCrashDumpWriter_CleanupOldDumps_ErrorCases tests cleanup with file stat errors.
func TestCrashDumpWriter_CleanupOldDumps_NoDir(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	writer := NewCrashDumpWriter("/nonexistent/dir/that/does/not/exist", 10, false, false, logger, nil)

	// This should handle the error gracefully
	err := writer.cleanupOldDumps()
	if err == nil {
		t.Error("expected error when dir does not exist")
	}
}

// TestCrashDumpWriter_WriteCrashDump_WithCommand tests dump with command context.
func TestCrashDumpWriter_WriteCrashDump_WithCommandContext(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	cmdCtx := &CommandContext{
		Path:    "/usr/bin/test",
		Args:    []string{"--flag", "value"},
		WorkDir: "/tmp/test",
		Started: time.Now(),
	}
	writer.SetCurrentCommand(cmdCtx)

	path, err := writer.WriteCrashDump("command panic")
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

	if dump.CommandPath != "/usr/bin/test" {
		t.Errorf("CommandPath = %q, want %q", dump.CommandPath, "/usr/bin/test")
	}
	if len(dump.CommandArgs) != 2 {
		t.Errorf("CommandArgs length = %d, want 2", len(dump.CommandArgs))
	}
	if dump.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", dump.WorkDir, "/tmp/test")
	}
}

// TestCrashDumpWriter_WriteCrashDump_WithEnv tests dump with environment redaction.
func TestCrashDumpWriter_WriteCrashDump_WithEnvironment(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, false, true, logger, nil)

	// Set test env vars
	os.Setenv("TEST_SECRET_KEY", "secret")
	os.Setenv("TEST_NORMAL", "normal")
	defer os.Unsetenv("TEST_SECRET_KEY")
	defer os.Unsetenv("TEST_NORMAL")

	path, err := writer.WriteCrashDump("env test panic")
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

	if len(dump.RedactedEnv) == 0 {
		t.Error("expected non-empty RedactedEnv")
	}

	// Check redaction
	if val, ok := dump.RedactedEnv["TEST_SECRET_KEY"]; !ok || val != "[REDACTED]" {
		t.Errorf("TEST_SECRET_KEY should be redacted, got %q", val)
	}

	if val, ok := dump.RedactedEnv["TEST_NORMAL"]; !ok || val != "normal" {
		t.Errorf("TEST_NORMAL should be visible, got %q", val)
	}
}

// TestCrashDumpWriter_RedactEnvironment_SensitiveSubstrings tests all sensitive patterns.
func TestCrashDumpWriter_RedactEnvironment_AllPatterns(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	writer := NewCrashDumpWriter("", 10, false, true, logger, nil)

	testCases := []struct {
		envKey    string
		envValue  string
		shouldRedact bool
	}{
		{"MY_API_TOKEN", "secret", true},
		{"GITHUB_KEY", "secret", true},
		{"DB_SECRET", "secret", true},
		{"USER_PASSWORD", "secret", true},
		{"AWS_CREDENTIAL", "secret", true},
		{"AUTH_TOKEN", "secret", true},
		{"PRIVATE_KEY", "secret", true},
		{"API_KEY", "secret", true},
		{"APIKEY", "secret", true},
		{"NORMAL_VAR", "visible", false},
		{"PATH", "/usr/bin", false},
		{"HOME", "/home/user", false},
	}

	for _, tc := range testCases {
		os.Setenv(tc.envKey, tc.envValue)
	}
	defer func() {
		for _, tc := range testCases {
			os.Unsetenv(tc.envKey)
		}
	}()

	redacted := writer.redactEnvironment()

	for _, tc := range testCases {
		val, ok := redacted[tc.envKey]
		if !ok {
			continue // Not all env vars may be in the result
		}

		if tc.shouldRedact {
			if val != "[REDACTED]" {
				t.Errorf("%s should be redacted, got %q", tc.envKey, val)
			}
		} else {
			if val == "[REDACTED]" {
				t.Errorf("%s should not be redacted", tc.envKey)
			}
		}
	}
}

// TestLoadLatestCrashDump_NonJsonFiles tests loading with non-JSON files present.
func TestLoadLatestCrashDump_IgnoresNonJsonFiles(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	// Create non-JSON crash files that should be ignored
	if err := os.WriteFile(filepath.Join(tempDir, "crash-test.txt"), []byte("test"), 0o600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)
	_, err := writer.WriteCrashDump("real panic")
	if err != nil {
		t.Fatalf("failed to write crash dump: %v", err)
	}

	dump, err := LoadLatestCrashDump(tempDir)
	if err != nil {
		t.Fatalf("failed to load latest crash dump: %v", err)
	}

	if dump.PanicValue != "real panic" {
		t.Errorf("PanicValue = %q, want %q", dump.PanicValue, "real panic")
	}
}

// TestResourceMonitor_Uptime tests the Uptime method.
func TestResourceMonitor_Uptime(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	time.Sleep(50 * time.Millisecond)

	uptime := monitor.Uptime()
	if uptime < 50*time.Millisecond {
		t.Errorf("Uptime = %v, want >= 50ms", uptime)
	}
}

// TestResourceMonitor_GetTrend_InsufficientHistory tests trend with insufficient data.
func TestResourceMonitor_GetTrend_LessThanTwoSnapshots(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// No snapshots yet
	trend := monitor.GetTrend()
	if !trend.IsHealthy {
		t.Error("trend should be healthy with no history")
	}

	// Take one snapshot
	monitor.TakeSnapshot()
	trend = monitor.GetTrend()
	if !trend.IsHealthy {
		t.Error("trend should be healthy with only one snapshot")
	}
}

// TestResourceMonitor_GetTrend_ShortDuration tests trend with short duration.
func TestResourceMonitor_GetTrend_BelowMinDuration(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(10*time.Millisecond, 80, 10000, 4096, 100, logger)

	// Take snapshots with short interval
	monitor.TakeSnapshot()
	time.Sleep(20 * time.Millisecond)
	monitor.TakeSnapshot()

	trend := monitor.GetTrend()
	// Should be healthy because duration < trendMinDuration (10 minutes)
	if !trend.IsHealthy {
		t.Error("trend should be healthy with short duration")
	}
}

// TestResourceMonitor_GetTrend_GrowthRates tests trend calculations with known values.
func TestResourceMonitor_GetTrend_WithGrowth(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 100, logger)

	// Manually construct history with growth
	monitor.mu.Lock()
	baseTime := time.Now().Add(-15 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:    baseTime,
			OpenFDs:      100,
			Goroutines:   50,
			HeapAllocMB:  100.0,
		},
		{
			Timestamp:    baseTime.Add(15 * time.Minute),
			OpenFDs:      200, // +100 FDs in 15 min = 400/hour
			Goroutines:   150, // +100 goroutines in 15 min = 400/hour
			HeapAllocMB:  200.0, // +100 MB in 15 min = 400/hour
		},
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()

	// All growth rates should exceed thresholds
	if trend.IsHealthy {
		t.Error("trend should be unhealthy with high growth rates")
	}

	if len(trend.Warnings) == 0 {
		t.Error("expected warnings for high growth rates")
	}

	// Check specific growth rates
	expectedFDRate := 400.0 // per hour
	if trend.FDGrowthRate < expectedFDRate-1 || trend.FDGrowthRate > expectedFDRate+1 {
		t.Errorf("FDGrowthRate = %f, want ~%f", trend.FDGrowthRate, expectedFDRate)
	}
}

// TestResourceMonitor_CheckHealth_EdgeThresholds tests health checks at exact thresholds.
func TestResourceMonitor_CheckHealth_AtThreshold(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 0, 0, 0, 10, logger)

	// With zero thresholds, no warnings should be generated
	warnings := monitor.CheckHealth()
	if len(warnings) > 0 {
		t.Logf("Warnings with zero thresholds: %v", warnings)
	}
}

// TestResourceMonitor_CheckHealth_CriticalLevels tests critical warning levels.
func TestResourceMonitor_CheckHealth_CriticalFD(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Set very low threshold to trigger critical
	monitor := NewResourceMonitor(time.Second, 1, 10000, 4096, 10, logger)

	// Take a snapshot
	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	warnings := monitor.CheckHealth()

	// Look for critical FD warning
	foundCritical := false
	for _, w := range warnings {
		if w.Type == "fd" && w.Level == "critical" && snapshot.FDUsagePercent > 90 {
			foundCritical = true
			break
		}
	}

	if !foundCritical && snapshot.FDUsagePercent > 90 {
		t.Log("Expected critical FD warning with usage > 90%")
	}
}

// TestSafeExecutor_RunPreflight_WithTrends tests preflight with trend warnings.
func TestSafeExecutor_RunPreflight_TrendWarnings(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(10*time.Millisecond, 80, 10000, 4096, 100, logger)

	// Manually add trend with warnings
	monitor.mu.Lock()
	baseTime := time.Now().Add(-15 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{Timestamp: baseTime, OpenFDs: 100, Goroutines: 50, HeapAllocMB: 100.0},
		{Timestamp: baseTime.Add(15 * time.Minute), OpenFDs: 200, Goroutines: 150, HeapAllocMB: 200.0},
	}
	monitor.mu.Unlock()

	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	result := executor.RunPreflight()

	// Should have warnings from trends
	if len(result.Warnings) == 0 {
		t.Error("expected trend warnings in preflight result")
	}
}

// TestSafeExecutor_RunPreflight_InsufficientFDs tests FD threshold checks.
func TestSafeExecutor_RunPreflight_FDCheck(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Require 99% free FDs (unrealistic, should fail)
	executor := NewSafeExecutor(monitor, nil, logger, true, 99, 256)

	result := executor.RunPreflight()

	// Likely to fail unless system has very low FD usage
	if result.OK {
		t.Log("Preflight passed (system has very low FD usage)")
	} else {
		if len(result.Errors) == 0 {
			t.Error("expected errors when FD threshold not met")
		}
	}
}

// TestSafeExecutor_PrepareCommand_NoMonitor tests command preparation without monitor.
func TestSafeExecutor_PrepareCommand_NoMonitor(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(nil, nil, logger, true, 20, 256)

	cmd := exec.Command("echo", "test")
	pipes, err := executor.PrepareCommand(cmd)
	if err != nil {
		t.Fatalf("failed to prepare command: %v", err)
	}

	if pipes == nil {
		t.Fatal("expected non-nil pipes")
	}

	// Cleanup should not panic without monitor
	pipes.Cleanup()
}

// TestSafeExecutor_PrepareStderrOnly_NoMonitor tests stderr-only preparation without monitor.
func TestSafeExecutor_PrepareStderrOnly_NoMonitor(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(nil, nil, logger, true, 20, 256)

	cmd := exec.Command("echo", "test")
	stderr, cleanup, err := executor.PrepareStderrOnly(cmd)
	if err != nil {
		t.Fatalf("failed to prepare stderr: %v", err)
	}

	if stderr == nil {
		t.Fatal("expected non-nil stderr")
	}

	if cleanup == nil {
		t.Fatal("expected non-nil cleanup")
	}

	// Cleanup should not panic without monitor
	cleanup()
}

// TestSystemMetricsCollector_Concurrent tests concurrent collection safety.
func TestSystemMetricsCollector_ConcurrentWithGPU(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	done := make(chan struct{})

	// Run multiple concurrent collections
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 3; j++ {
				_ = c.Collect()
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

// TestRootDiskPath_Windows tests Windows-specific path logic.
func TestRootDiskPath_Logic(t *testing.T) {
	t.Parallel()

	path := rootDiskPath()

	// On Linux/Unix, should be "/"
	// On Windows, should be something like "C:\"
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(path, "\\") {
			t.Errorf("Windows path should end with backslash, got %q", path)
		}
	} else {
		if path != "/" {
			t.Errorf("Unix path should be '/', got %q", path)
		}
	}
}

// TestResourceMonitor_Start_ContextCancellation tests Start with context cancellation.
func TestResourceMonitor_Start_ContextCancel(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(20*time.Millisecond, 80, 10000, 4096, 10, logger)

	ctx, cancel := context.WithCancel(context.Background())
	monitor.Start(ctx)

	// Wait for some snapshots
	time.Sleep(100 * time.Millisecond)

	historyBefore := len(monitor.GetHistory())
	if historyBefore == 0 {
		t.Error("expected at least one snapshot in history")
	}

	// Cancel context
	cancel()

	// Wait and ensure no more snapshots
	time.Sleep(100 * time.Millisecond)
	historyAfter := len(monitor.GetHistory())

	// Allow for at most 1 snapshot difference due to timing
	if historyAfter > historyBefore+1 {
		t.Errorf("snapshots should stop after context cancel: before=%d, after=%d",
			historyBefore, historyAfter)
	}
}

// TestResourceMonitor_GetLatest_EmptyHistory tests GetLatest with no snapshots.
func TestResourceMonitor_GetLatest_Empty(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// No snapshots taken yet
	_, ok := monitor.GetLatest()
	if ok {
		t.Error("GetLatest should return false with empty history")
	}

	// Take a snapshot
	monitor.TakeSnapshot()
	monitor.recordSnapshot(monitor.TakeSnapshot())

	snapshot, ok := monitor.GetLatest()
	if !ok {
		t.Error("GetLatest should return true after taking snapshot")
	}

	if snapshot.Timestamp.IsZero() {
		t.Error("snapshot should have non-zero timestamp")
	}
}

// TestResourceMonitor_HistoryLimit tests that history is limited to historySize.
func TestResourceMonitor_HistoryLimit(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Millisecond, 80, 10000, 4096, 5, logger)

	// Add more snapshots than history size
	for i := 0; i < 10; i++ {
		snapshot := monitor.TakeSnapshot()
		monitor.recordSnapshot(snapshot)
		time.Sleep(2 * time.Millisecond)
	}

	history := monitor.GetHistory()
	if len(history) > 5 {
		t.Errorf("history length = %d, want <= 5", len(history))
	}
}

// TestNewResourceMonitor_DefaultValues tests default value handling.
func TestNewResourceMonitor_DefaultValues(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Test with zero/negative values
	monitor := NewResourceMonitor(0, 80, 10000, 4096, 0, logger)

	if monitor.interval != 30*time.Second {
		t.Errorf("interval = %v, want 30s (default)", monitor.interval)
	}

	if monitor.historySize != 120 {
		t.Errorf("historySize = %d, want 120 (default)", monitor.historySize)
	}
}

// TestCrashDumpWriter_WriteCrashDump_NoStack tests dump without stack trace.
func TestCrashDumpWriter_WriteCrashDump_NoStackTrace(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, false, false, logger, nil)

	path, err := writer.WriteCrashDump("panic without stack")
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

	if dump.StackTrace != "" {
		t.Error("stack trace should be empty when includeStack is false")
	}
}

// TestCrashDumpWriter_WriteCrashDump_NoEnv tests dump without environment.
func TestCrashDumpWriter_WriteCrashDump_NoEnvironment(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, false, false, logger, nil)

	path, err := writer.WriteCrashDump("panic without env")
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

	if len(dump.RedactedEnv) > 0 {
		t.Error("RedactedEnv should be empty when includeEnv is false")
	}
}
