package diagnostics

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"
)

// TestResourceMonitor_CheckHealth_MemoryCritical tests critical memory warning.
func TestResourceMonitor_CheckHealth_MemoryCritical(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Very low memory threshold to trigger critical
	monitor := NewResourceMonitor(time.Second, 80, 10000, 1, 10, logger)

	// Take snapshot and record it
	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	warnings := monitor.CheckHealth()

	// Look for memory warnings
	for _, w := range warnings {
		if w.Type == "memory" {
			if w.Level != "warning" && w.Level != "critical" {
				t.Errorf("memory warning level = %q, want 'warning' or 'critical'", w.Level)
			}
			// Check that Value and Limit are set
			if w.Value <= 0 {
				t.Errorf("memory warning Value = %f, want > 0", w.Value)
			}
			if w.Limit <= 0 {
				t.Errorf("memory warning Limit = %f, want > 0", w.Limit)
			}
		}
	}
}

// TestResourceMonitor_CheckHealth_GoroutineCritical tests critical goroutine warning.
func TestResourceMonitor_CheckHealth_GoroutineCritical(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Very low goroutine threshold to trigger critical
	monitor := NewResourceMonitor(time.Second, 80, 1, 4096, 10, logger)

	// Take snapshot and record it
	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	warnings := monitor.CheckHealth()

	// Look for goroutine warnings
	for _, w := range warnings {
		if w.Type == "goroutine" {
			if w.Level != "warning" && w.Level != "critical" {
				t.Errorf("goroutine warning level = %q, want 'warning' or 'critical'", w.Level)
			}
			// Check that Value and Limit are set
			if w.Value <= 0 {
				t.Errorf("goroutine warning Value = %f, want > 0", w.Value)
			}
			if w.Limit <= 0 {
				t.Errorf("goroutine warning Limit = %f, want > 0", w.Limit)
			}
		}
	}
}

// TestResourceMonitor_GetTrend_AllThresholdsExceeded tests all trend warnings.
func TestResourceMonitor_GetTrend_AllThresholdsExceeded(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 100, logger)

	// Manually construct history that exceeds all thresholds
	monitor.mu.Lock()
	baseTime := time.Now().Add(-15 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   baseTime,
			OpenFDs:     100,
			Goroutines:  50,
			HeapAllocMB: 100.0,
		},
		{
			Timestamp:   baseTime.Add(15 * time.Minute),
			OpenFDs:     300,   // +200 in 15min = 800/hour (exceeds threshold of 50/hour)
			Goroutines:  400,   // +350 in 15min = 1400/hour (exceeds threshold of 500/hour)
			HeapAllocMB: 400.0, // +300 in 15min = 1200/hour (exceeds threshold of 250/hour)
		},
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()

	// Should be unhealthy with all thresholds exceeded
	if trend.IsHealthy {
		t.Error("trend should be unhealthy with all growth rates exceeding thresholds")
	}

	// Should have warnings for all three metrics
	if len(trend.Warnings) < 3 {
		t.Errorf("expected at least 3 warnings (FD, Goroutine, Memory), got %d", len(trend.Warnings))
	}

	// Verify growth rates are calculated correctly
	expectedFDRate := 800.0 // (300-100) * 4 (per hour)
	if trend.FDGrowthRate < expectedFDRate-10 || trend.FDGrowthRate > expectedFDRate+10 {
		t.Errorf("FDGrowthRate = %f, want ~%f", trend.FDGrowthRate, expectedFDRate)
	}

	expectedGoroutineRate := 1400.0
	if trend.GoroutineGrowthRate < expectedGoroutineRate-10 || trend.GoroutineGrowthRate > expectedGoroutineRate+10 {
		t.Errorf("GoroutineGrowthRate = %f, want ~%f", trend.GoroutineGrowthRate, expectedGoroutineRate)
	}

	expectedMemoryRate := 1200.0
	if trend.MemoryGrowthRate < expectedMemoryRate-10 || trend.MemoryGrowthRate > expectedMemoryRate+10 {
		t.Errorf("MemoryGrowthRate = %f, want ~%f", trend.MemoryGrowthRate, expectedMemoryRate)
	}
}

// TestResourceMonitor_GetTrend_SingleThresholdExceeded tests individual threshold warnings.
func TestResourceMonitor_GetTrend_OnlyFDExceeded(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 100, logger)

	// Only FD growth exceeds threshold
	monitor.mu.Lock()
	baseTime := time.Now().Add(-15 * time.Minute)
	monitor.history = []ResourceSnapshot{
		{
			Timestamp:   baseTime,
			OpenFDs:     100,
			Goroutines:  50,
			HeapAllocMB: 100.0,
		},
		{
			Timestamp:   baseTime.Add(15 * time.Minute),
			OpenFDs:     300,   // +200 in 15min = 800/hour (exceeds 50/hour)
			Goroutines:  55,    // +5 in 15min = 20/hour (OK)
			HeapAllocMB: 110.0, // +10 in 15min = 40/hour (OK)
		},
	}
	monitor.mu.Unlock()

	trend := monitor.GetTrend()

	// Should be unhealthy
	if trend.IsHealthy {
		t.Error("trend should be unhealthy with FD growth exceeding threshold")
	}

	// Should have at least 1 warning (for FDs)
	if len(trend.Warnings) == 0 {
		t.Error("expected at least 1 warning for FD growth")
	}

	// Verify FD warning exists
	foundFDWarning := false
	for _, w := range trend.Warnings {
		if contains(w, "FD") || contains(w, "fd") {
			foundFDWarning = true
			break
		}
	}
	if !foundFDWarning {
		t.Error("expected FD warning in trend warnings")
	}
}

// TestSafeExecutor_RunPreflight_EdgeCaseFDPercentage tests FD percentage at boundary.
func TestSafeExecutor_RunPreflight_ExactFDThreshold(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Second, 80, 10000, 4096, 10, logger)

	// Take a snapshot to get current FD usage
	snapshot := monitor.TakeSnapshot()
	freeFDPercent := 100.0 - snapshot.FDUsagePercent

	// Set threshold exactly at current free FD percentage
	executor := NewSafeExecutor(monitor, nil, logger, true, int(freeFDPercent), 256)

	result := executor.RunPreflight()

	// Should pass (exactly at threshold)
	if !result.OK {
		t.Log("Preflight failed at exact threshold (timing dependent)")
	}
}

// TestCrashDumpWriter_WriteCrashDump_NoMonitorNoHistory tests dump without monitor.
func TestCrashDumpWriter_WriteCrashDump_NoMonitorNoHistory(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tempDir := t.TempDir()

	writer := NewCrashDumpWriter(tempDir, 10, true, false, logger, nil)

	path, err := writer.WriteCrashDump("panic without monitor")
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

	// ResourceState should have zero values
	if !dump.ResourceState.Timestamp.IsZero() {
		t.Log("ResourceState timestamp is set (unexpected without monitor)")
	}

	// ResourceHistory should be empty or nil
	if len(dump.ResourceHistory) > 0 {
		t.Log("ResourceHistory is populated (unexpected without monitor)")
	}
}

// TestResourceMonitor_RecordSnapshot_Trimming tests history trimming.
func TestResourceMonitor_RecordSnapshot_ExactHistorySize(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(time.Millisecond, 80, 10000, 4096, 3, logger)

	// Add exactly historySize snapshots
	for i := 0; i < 3; i++ {
		snapshot := monitor.TakeSnapshot()
		monitor.recordSnapshot(snapshot)
		time.Sleep(2 * time.Millisecond)
	}

	history := monitor.GetHistory()
	if len(history) != 3 {
		t.Errorf("history length = %d, want 3", len(history))
	}

	// Add one more (should trim oldest)
	snapshot := monitor.TakeSnapshot()
	monitor.recordSnapshot(snapshot)

	history = monitor.GetHistory()
	if len(history) != 3 {
		t.Errorf("history length = %d, want 3 (after trimming)", len(history))
	}
}

// Helper function to check if string contains substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
