package diagnostics

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"
)

// ResourceSnapshot captures system resource state at a point in time.
type ResourceSnapshot struct {
	Timestamp      time.Time     `json:"timestamp"`
	OpenFDs        int           `json:"open_fds"`
	MaxFDs         int           `json:"max_fds"`
	FDUsagePercent float64       `json:"fd_usage_percent"`
	Goroutines     int           `json:"goroutines"`
	HeapAllocMB    float64       `json:"heap_alloc_mb"`
	HeapInUseMB    float64       `json:"heap_in_use_mb"`
	StackInUseMB   float64       `json:"stack_in_use_mb"`
	GCPauseNS      uint64        `json:"gc_pause_ns"`
	NumGC          uint32        `json:"num_gc"`
	ProcessUptime  time.Duration `json:"process_uptime"`
	CommandsRun    int64         `json:"commands_run"`
	CommandsActive int           `json:"commands_active"`
}

// ResourceTrend captures resource usage trends over time.
type ResourceTrend struct {
	FDGrowthRate        float64  // FDs per hour
	GoroutineGrowthRate float64  // Goroutines per hour
	MemoryGrowthRate    float64  // MB per hour
	IsHealthy           bool     // Overall health assessment
	Warnings            []string // Trend-based warnings
}

// HealthWarning represents a single health concern.
type HealthWarning struct {
	Level   string  // "warning" or "critical"
	Type    string  // "fd", "goroutine", "memory"
	Message string  // Human-readable description
	Value   float64 // Current value
	Limit   float64 // Threshold that was exceeded
}

// ResourceMonitor tracks system resource usage over time.
type ResourceMonitor struct {
	interval           time.Duration
	fdThresholdPercent int
	goroutineThreshold int
	memoryThresholdMB  int
	historySize        int
	logger             *slog.Logger

	history []ResourceSnapshot
	mu      sync.RWMutex

	// Metrics
	commandsRun    atomic.Int64
	commandsActive atomic.Int32

	// Control
	stopCh  chan struct{}
	stopped atomic.Bool
	started time.Time
}

// NewResourceMonitor creates a new resource monitor.
func NewResourceMonitor(
	interval time.Duration,
	fdThresholdPercent int,
	goroutineThreshold int,
	memoryThresholdMB int,
	historySize int,
	logger *slog.Logger,
) *ResourceMonitor {
	if historySize <= 0 {
		historySize = 120 // Default: 1 hour at 30s intervals
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}

	return &ResourceMonitor{
		interval:           interval,
		fdThresholdPercent: fdThresholdPercent,
		goroutineThreshold: goroutineThreshold,
		memoryThresholdMB:  memoryThresholdMB,
		historySize:        historySize,
		logger:             logger,
		history:            make([]ResourceSnapshot, 0, historySize),
		stopCh:             make(chan struct{}),
		started:            time.Now(),
	}
}

// Start begins periodic resource monitoring.
func (m *ResourceMonitor) Start(ctx context.Context) {
	go func() {
		// Take initial snapshot
		snapshot := m.TakeSnapshot()
		m.recordSnapshot(snapshot)

		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				snapshot := m.TakeSnapshot()
				m.recordSnapshot(snapshot)

				// Check thresholds and log warnings
				warnings := m.CheckHealth()
				for _, w := range warnings {
					if m.logger != nil {
						m.logger.Warn("resource warning",
							"type", w.Type,
							"level", w.Level,
							"value", w.Value,
							"limit", w.Limit,
							"message", w.Message,
						)
					}
				}
			}
		}
	}()
}

// Stop halts the monitoring loop.
func (m *ResourceMonitor) Stop() {
	if m.stopped.CompareAndSwap(false, true) {
		close(m.stopCh)
	}
}

// TakeSnapshot captures current resource state.
func (m *ResourceMonitor) TakeSnapshot() ResourceSnapshot {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	openFDs, maxFDs := CountFDs()
	fdPercent := 0.0
	if maxFDs > 0 {
		fdPercent = float64(openFDs) / float64(maxFDs) * 100
	}

	return ResourceSnapshot{
		Timestamp:      time.Now(),
		OpenFDs:        openFDs,
		MaxFDs:         maxFDs,
		FDUsagePercent: fdPercent,
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocMB:    float64(memStats.HeapAlloc) / 1024 / 1024,
		HeapInUseMB:    float64(memStats.HeapInuse) / 1024 / 1024,
		StackInUseMB:   float64(memStats.StackInuse) / 1024 / 1024,
		GCPauseNS:      memStats.PauseNs[(memStats.NumGC+255)%256],
		NumGC:          memStats.NumGC,
		ProcessUptime:  time.Since(m.started),
		CommandsRun:    m.commandsRun.Load(),
		CommandsActive: int(m.commandsActive.Load()),
	}
}

func (m *ResourceMonitor) recordSnapshot(s ResourceSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, s)

	// Trim to history size
	if len(m.history) > m.historySize {
		m.history = m.history[len(m.history)-m.historySize:]
	}
}

// GetHistory returns historical snapshots.
func (m *ResourceMonitor) GetHistory() []ResourceSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ResourceSnapshot, len(m.history))
	copy(result, m.history)
	return result
}

// GetLatest returns the most recent snapshot.
func (m *ResourceMonitor) GetLatest() (ResourceSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.history) == 0 {
		return ResourceSnapshot{}, false
	}
	return m.history[len(m.history)-1], true
}

// GetTrend analyzes recent snapshots for concerning trends.
func (m *ResourceMonitor) GetTrend() ResourceTrend {
	history := m.GetHistory()
	if len(history) < 2 {
		return ResourceTrend{IsHealthy: true}
	}

	first := history[0]
	last := history[len(history)-1]
	duration := last.Timestamp.Sub(first.Timestamp).Hours()

	if duration < 0.01 { // Less than 36 seconds
		return ResourceTrend{IsHealthy: true}
	}

	trend := ResourceTrend{
		FDGrowthRate:        float64(last.OpenFDs-first.OpenFDs) / duration,
		GoroutineGrowthRate: float64(last.Goroutines-first.Goroutines) / duration,
		MemoryGrowthRate:    (last.HeapAllocMB - first.HeapAllocMB) / duration,
		IsHealthy:           true,
	}

	// Check for concerning trends
	if trend.FDGrowthRate > 10 { // More than 10 FDs/hour
		trend.IsHealthy = false
		trend.Warnings = append(trend.Warnings,
			fmt.Sprintf("FD count growing at %.1f/hour (potential leak)", trend.FDGrowthRate))
	}
	if trend.GoroutineGrowthRate > 100 { // More than 100 goroutines/hour
		trend.IsHealthy = false
		trend.Warnings = append(trend.Warnings,
			fmt.Sprintf("Goroutine count growing at %.1f/hour (potential leak)", trend.GoroutineGrowthRate))
	}
	if trend.MemoryGrowthRate > 100 { // More than 100 MB/hour
		trend.IsHealthy = false
		trend.Warnings = append(trend.Warnings,
			fmt.Sprintf("Memory growing at %.1f MB/hour", trend.MemoryGrowthRate))
	}

	return trend
}

// IncrementCommandCount is called when a command starts.
func (m *ResourceMonitor) IncrementCommandCount() {
	m.commandsRun.Add(1)
	m.commandsActive.Add(1)
}

// DecrementActiveCommands is called when a command completes.
func (m *ResourceMonitor) DecrementActiveCommands() {
	m.commandsActive.Add(-1)
}

// CheckHealth returns warnings if thresholds are exceeded.
func (m *ResourceMonitor) CheckHealth() []HealthWarning {
	snapshot, ok := m.GetLatest()
	if !ok {
		snapshot = m.TakeSnapshot()
	}

	var warnings []HealthWarning

	// FD threshold check
	if m.fdThresholdPercent > 0 && snapshot.FDUsagePercent > float64(m.fdThresholdPercent) {
		level := "warning"
		if snapshot.FDUsagePercent > 90 {
			level = "critical"
		}
		warnings = append(warnings, HealthWarning{
			Level: level,
			Type:  "fd",
			Message: fmt.Sprintf("FD usage at %.1f%% (threshold: %d%%)",
				snapshot.FDUsagePercent, m.fdThresholdPercent),
			Value: snapshot.FDUsagePercent,
			Limit: float64(m.fdThresholdPercent),
		})
	}

	// Goroutine threshold check
	if m.goroutineThreshold > 0 && snapshot.Goroutines > m.goroutineThreshold {
		level := "warning"
		if snapshot.Goroutines > m.goroutineThreshold*2 {
			level = "critical"
		}
		warnings = append(warnings, HealthWarning{
			Level: level,
			Type:  "goroutine",
			Message: fmt.Sprintf("Goroutine count at %d (threshold: %d)",
				snapshot.Goroutines, m.goroutineThreshold),
			Value: float64(snapshot.Goroutines),
			Limit: float64(m.goroutineThreshold),
		})
	}

	// Memory threshold check
	if m.memoryThresholdMB > 0 && snapshot.HeapAllocMB > float64(m.memoryThresholdMB) {
		level := "warning"
		if snapshot.HeapAllocMB > float64(m.memoryThresholdMB)*1.5 {
			level = "critical"
		}
		warnings = append(warnings, HealthWarning{
			Level: level,
			Type:  "memory",
			Message: fmt.Sprintf("Heap usage at %.1f MB (threshold: %d MB)",
				snapshot.HeapAllocMB, m.memoryThresholdMB),
			Value: snapshot.HeapAllocMB,
			Limit: float64(m.memoryThresholdMB),
		})
	}

	return warnings
}

// Uptime returns the process uptime.
func (m *ResourceMonitor) Uptime() time.Duration {
	return time.Since(m.started)
}
