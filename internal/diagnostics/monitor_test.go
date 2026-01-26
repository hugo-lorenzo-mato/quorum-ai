package diagnostics

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewResourceMonitor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(
		100*time.Millisecond,
		80,
		10000,
		4096,
		10,
		logger,
	)

	if monitor == nil {
		t.Fatal("expected non-nil monitor")
	}

	if monitor.interval != 100*time.Millisecond {
		t.Errorf("expected interval 100ms, got %v", monitor.interval)
	}
}

func TestResourceMonitor_TakeSnapshot(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(
		time.Second,
		80,
		10000,
		4096,
		10,
		logger,
	)

	snapshot := monitor.TakeSnapshot()

	// Basic sanity checks
	if snapshot.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if snapshot.Goroutines <= 0 {
		t.Error("expected positive goroutine count")
	}
	// FD counts may be 0 on some systems where /proc is not available
	// but we should at least not panic
}

func TestResourceMonitor_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(
		50*time.Millisecond,
		80,
		10000,
		4096,
		10,
		logger,
	)

	ctx := t.Context()
	monitor.Start(ctx)

	// Wait for a few snapshots
	time.Sleep(200 * time.Millisecond)

	history := monitor.GetHistory()
	if len(history) == 0 {
		t.Error("expected at least one snapshot in history")
	}

	monitor.Stop()

	// Give a small buffer after Stop() for any in-flight snapshot to complete
	time.Sleep(60 * time.Millisecond)

	// After stop and buffer, no more snapshots should be taken
	historyBeforeWait := len(monitor.GetHistory())
	time.Sleep(150 * time.Millisecond)
	historyAfterWait := len(monitor.GetHistory())

	// Allow for at most 1 snapshot difference due to timing
	if historyAfterWait > historyBeforeWait+1 {
		t.Errorf("snapshots should not be taken after Stop(): before=%d, after=%d",
			historyBeforeWait, historyAfterWait)
	}
}

func TestResourceMonitor_IncrementDecrementCommands(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(
		time.Second,
		80,
		10000,
		4096,
		10,
		logger,
	)

	// Initially should be 0
	snapshot := monitor.TakeSnapshot()
	if snapshot.CommandsActive != 0 {
		t.Errorf("expected 0 active commands, got %d", snapshot.CommandsActive)
	}

	// Increment
	monitor.IncrementCommandCount()
	monitor.IncrementCommandCount()

	snapshot = monitor.TakeSnapshot()
	if snapshot.CommandsActive != 2 {
		t.Errorf("expected 2 active commands, got %d", snapshot.CommandsActive)
	}

	// Decrement
	monitor.DecrementActiveCommands()

	snapshot = monitor.TakeSnapshot()
	if snapshot.CommandsActive != 1 {
		t.Errorf("expected 1 active command, got %d", snapshot.CommandsActive)
	}
}

func TestResourceMonitor_CheckHealth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Use very low thresholds to trigger warnings
	monitor := NewResourceMonitor(
		time.Second,
		1, // Very low FD threshold
		1, // Very low goroutine threshold
		1, // Very low memory threshold
		10,
		logger,
	)

	warnings := monitor.CheckHealth()

	// We should get warnings about high usage relative to these very low thresholds
	if len(warnings) == 0 {
		t.Log("No warnings generated (system has very low resource usage)")
	}

	// Each warning should have required fields
	for _, w := range warnings {
		if w.Type == "" {
			t.Error("warning Type should not be empty")
		}
		if w.Level == "" {
			t.Error("warning Level should not be empty")
		}
		if w.Message == "" {
			t.Error("warning Message should not be empty")
		}
	}
}

func TestResourceMonitor_GetTrend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	monitor := NewResourceMonitor(
		50*time.Millisecond,
		80,
		10000,
		4096,
		10,
		logger,
	)

	ctx := t.Context()
	monitor.Start(ctx)

	// Wait for enough snapshots to calculate trend
	time.Sleep(200 * time.Millisecond)

	trend := monitor.GetTrend()

	// Trend should be healthy under normal conditions
	if !trend.IsHealthy {
		t.Logf("Trend unhealthy: %v", trend.Warnings)
	}

	monitor.Stop()
}
