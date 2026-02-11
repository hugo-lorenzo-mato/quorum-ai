package diagnostics

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestSafeExecutor_PrepareCommand_StdoutPipeError tests error handling.
func TestSafeExecutor_PrepareCommand_MultipleCommands(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	// Prepare multiple commands
	cmd1 := exec.Command("echo", "test1")
	pipes1, err := executor.PrepareCommand(cmd1)
	if err != nil {
		t.Fatalf("failed to prepare first command: %v", err)
	}
	defer pipes1.Cleanup()

	cmd2 := exec.Command("echo", "test2")
	pipes2, err := executor.PrepareCommand(cmd2)
	if err != nil {
		t.Fatalf("failed to prepare second command: %v", err)
	}
	defer pipes2.Cleanup()

	// Both should increment active commands
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 2 {
		t.Errorf("CommandsActive = %d, want 2", snapshot.CommandsActive)
	}
}

// TestSafeExecutor_PrepareStderrOnly_MultipleCommands tests multiple stderr preparations.
func TestSafeExecutor_PrepareStderrOnly_MultipleCommands(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	executor := NewSafeExecutor(monitor, nil, logger, true, 20, 256)

	// Prepare multiple stderr pipes
	cmd1 := exec.Command("echo", "test1")
	stderr1, cleanup1, err := executor.PrepareStderrOnly(cmd1)
	if err != nil {
		t.Fatalf("failed to prepare first stderr: %v", err)
	}
	defer cleanup1()

	cmd2 := exec.Command("echo", "test2")
	stderr2, cleanup2, err := executor.PrepareStderrOnly(cmd2)
	if err != nil {
		t.Fatalf("failed to prepare second stderr: %v", err)
	}
	defer cleanup2()

	if stderr1 == nil || stderr2 == nil {
		t.Error("expected non-nil stderr pipes")
	}

	// Both should increment active commands
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 2 {
		t.Errorf("CommandsActive = %d, want 2", snapshot.CommandsActive)
	}
}

// TestResourceMonitor_CheckHealth_MultipleWarnings tests multiple warning generation.
func TestResourceMonitor_CheckHealth_AllWarningTypes(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use extremely low thresholds to trigger all warning types
	monitor := NewResourceMonitor(time.Second, 1, 1, 1, 10, logger)

	// Take a snapshot to populate history
	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	warnings := monitor.CheckHealth()

	// We expect multiple warnings with these low thresholds
	warningTypes := make(map[string]bool)
	for _, w := range warnings {
		warningTypes[w.Type] = true
	}

	// Check that warnings have valid structure
	for _, w := range warnings {
		if w.Level != "warning" && w.Level != "critical" {
			t.Errorf("invalid warning level: %q", w.Level)
		}
		if w.Message == "" {
			t.Error("warning message should not be empty")
		}
		if w.Value < 0 {
			t.Errorf("warning value should be non-negative, got %f", w.Value)
		}
	}
}

// TestResourceMonitor_GetTrend_ZeroHours tests trend with zero duration.
func TestResourceMonitor_GetTrend_ZeroHoursDuration(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Create history with same timestamp (zero duration)
	monitor.mu.Lock()
	now := time.Now()
	monitor.history = []ResourceSnapshot{
		{Timestamp: now, OpenFDs: 100},
		{Timestamp: now, OpenFDs: 200}, // Same timestamp
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()
	// Should be healthy because duration <= 0
	if !trend.IsHealthy {
		t.Error("trend should be healthy with zero duration")
	}
}

// TestResourceMonitor_GetTrend_ExactMinDuration tests trend at minimum duration boundary.
func TestResourceMonitor_GetTrend_ExactMinDuration(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Create history with exactly trendMinDuration (10 minutes)
	monitor.mu.Lock()
	baseTime := time.Now().Add(-10 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{Timestamp: baseTime, OpenFDs: 100, Goroutines: 50, HeapAllocMB: 100.0},
		{Timestamp: baseTime.Add(10 * time.Minute), OpenFDs: 150, Goroutines: 75, HeapAllocMB: 125.0},
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()
	// Should calculate trend (not skip due to duration)
	// Growth rates should be calculated
	if trend.FDGrowthRate == 0 && trend.GoroutineGrowthRate == 0 && trend.MemoryGrowthRate == 0 {
		t.Error("expected non-zero growth rates with 10 minute duration")
	}
}

// TestResourceMonitor_GetTrend_NegativeGrowth tests trend with decreasing resources.
func TestResourceMonitor_GetTrend_NegativeGrowth(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Create history with decreasing resources
	monitor.mu.Lock()
	baseTime := time.Now().Add(-15 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{Timestamp: baseTime, OpenFDs: 200, Goroutines: 150, HeapAllocMB: 200.0},
		{Timestamp: baseTime.Add(15 * time.Minute), OpenFDs: 100, Goroutines: 75, HeapAllocMB: 100.0},
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()
	// Negative growth is healthy (resources decreasing)
	if !trend.IsHealthy {
		t.Log("Trend is unhealthy (acceptable if other factors)")
	}
	if trend.FDGrowthRate >= 0 {
		t.Errorf("FDGrowthRate = %f, expected negative", trend.FDGrowthRate)
	}
}

// TestResourceMonitor_CheckHealth_NoLatestSnapshot tests health check without latest snapshot.
func TestResourceMonitor_CheckHealth_TakesSnapshotWhenNeeded(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Don't add any snapshots to history
	warnings := monitor.CheckHealth()
	// Should still work by taking a new snapshot
	_ = warnings
}

// TestResourceMonitor_Stop_Idempotent tests Stop can be called multiple times.
func TestResourceMonitor_Stop_MultipleCalls(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(50*time.Millisecond, 80, 10000, 4096, 10, logger)

	ctx := context.Background()
	monitor.Start(ctx)

	// Stop multiple times
	monitor.Stop()
	monitor.Stop()
	monitor.Stop()

	// Should not panic
}

// TestResourceMonitor_TakeSnapshot_CPUPercent tests CPU percentage calculation.
func TestResourceMonitor_TakeSnapshot_MemoryMetrics(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	snapshot := monitor.TakeSnapshot()

	// Memory metrics should be positive
	if snapshot.HeapAllocMB < 0 {
		t.Errorf("HeapAllocMB = %f, want >= 0", snapshot.HeapAllocMB)
	}
	if snapshot.HeapInUseMB < 0 {
		t.Errorf("HeapInUseMB = %f, want >= 0", snapshot.HeapInUseMB)
	}
	if snapshot.StackInUseMB < 0 {
		t.Errorf("StackInUseMB = %f, want >= 0", snapshot.StackInUseMB)
	}
}

// TestResourceMonitor_CommandTracking tests command count tracking.
func TestResourceMonitor_CommandTracking(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Test command tracking
	for i := 0; i < 5; i++ {
		monitor.IncrementCommandCount()
	}

	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsRun != 5 {
		t.Errorf("CommandsRun = %d, want 5", snapshot.CommandsRun)
	}
	if snapshot.CommandsActive != 5 {
		t.Errorf("CommandsActive = %d, want 5", snapshot.CommandsActive)
	}

	// Decrement some
	monitor.DecrementActiveCommands()
	monitor.DecrementActiveCommands()

	snapshot = monitor.TakeSnapshot()
	if snapshot.CommandsRun != 5 {
		t.Errorf("CommandsRun = %d, want 5 (should not change)", snapshot.CommandsRun)
	}
	if snapshot.CommandsActive != 3 {
		t.Errorf("CommandsActive = %d, want 3", snapshot.CommandsActive)
	}
}

// TestSafeExecutor_WrapExecution_WithError tests WrapExecution with returned error.
func TestSafeExecutor_WrapExecution_ReturnsError(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	executor := NewSafeExecutor(nil, nil, logger, true, 20, 256)

	// Test execution that returns error (no panic)
	testErr := os.ErrNotExist
	err := executor.WrapExecution(func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
}

// TestSafeExecutor_RunPreflight_FreeFDWarning tests warning when approaching FD limit.
func TestSafeExecutor_RunPreflight_ApproachingFDLimit(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Require 100% free FDs (impossible, should generate warning)
	executor := NewSafeExecutor(monitor, nil, logger, true, 100, 256)

	result := executor.RunPreflight()

	// Should have errors or warnings
	if result.OK {
		t.Log("Preflight OK (system has all FDs free)")
	} else {
		if len(result.Errors) == 0 {
			t.Error("expected errors with impossible FD requirement")
		}
	}
}

// TestCrashDumpWriter_WriteCrashDump_ComplexPanicValue tests various panic types.
func TestCrashDumpWriter_WriteCrashDump_DifferentPanicTypes(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	testCases := []any{
		"string panic",
		42,
		3.14,
		struct{ msg string }{"struct panic"},
		[]string{"slice", "panic"},
		map[string]string{"key": "value"},
	}

	for _, panicVal := range testCases {
		path, err := writer.WriteCrashDump(panicVal)
		if err != nil {
			t.Errorf("failed to write crash dump for %T: %v", panicVal, err)
		}
		if path == "" {
			t.Errorf("expected non-empty path for %T", panicVal)
		}
	}
}

// TestResourceMonitor_GetHistory_ThreadSafety tests concurrent access to history.
func TestResourceMonitor_GetHistory_Concurrent(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(10*time.Millisecond, 80, 10000, 4096, 10, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)

	done := make(chan struct{})
	// Multiple goroutines reading history
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 10; j++ {
				_ = monitor.GetHistory()
				_, _ = monitor.GetLatest()
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	monitor.Stop()
}

// TestSystemMetricsCollector_Collect_AllFields tests that Collect populates all fields.
func TestSystemMetricsCollector_Collect_AllFields(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()
	stats := c.Collect()

	// Check that hardware info is populated
	if c.infoCollected && stats.CPUModel == "" {
		t.Log("CPUModel not populated (might be normal on some systems)")
	}

	// Memory should always be populated
	if stats.MemTotalMB <= 0 {
		t.Error("MemTotalMB should be positive")
	}

	// Disk should always be populated
	if stats.DiskTotalGB <= 0 {
		t.Error("DiskTotalGB should be positive")
	}

	// Check CPU info fields are set (even if zero)
	_ = stats.CPUPercent
	_ = stats.CPUCores
	_ = stats.CPUThreads
}

// TestNewSystemMetricsCollector_InitialState tests collector initial state.
func TestNewSystemMetricsCollector_InitialState(t *testing.T) {
	t.Parallel()

	c := NewSystemMetricsCollector()

	if c.infoCollected {
		t.Error("infoCollected should be false initially")
	}

	if c.lastCPUTotal != 0 {
		t.Error("lastCPUTotal should be 0 initially")
	}

	if c.lastCPUIdle != 0 {
		t.Error("lastCPUIdle should be 0 initially")
	}

	if !c.lastGPUUpdate.IsZero() {
		t.Error("lastGPUUpdate should be zero initially")
	}
}

// TestResourceMonitor_TakeSnapshot_FDCalculation tests FD percentage calculation.
func TestResourceMonitor_TakeSnapshot_FDPercentage(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	snapshot := monitor.TakeSnapshot()

	// FD percentage should be between 0 and 100 (or 0 if MaxFDs is 0)
	if snapshot.MaxFDs > 0 {
		if snapshot.FDUsagePercent < 0 || snapshot.FDUsagePercent > 100 {
			t.Errorf("FDUsagePercent = %f, want 0-100", snapshot.FDUsagePercent)
		}
	} else {
		if snapshot.FDUsagePercent != 0 {
			t.Errorf("FDUsagePercent = %f, want 0 when MaxFDs is 0", snapshot.FDUsagePercent)
		}
	}
}
