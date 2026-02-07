package workflow

import (
	"os"
	"sync"
	"time"
)

// OutputWatchdogConfig controls the file-based output watchdog behavior.
type OutputWatchdogConfig struct {
	PollInterval    time.Duration // How often to check the file (default: 5s)
	StabilityWindow time.Duration // How long size must be stable before considering done (default: 15s)
	MinFileSize     int64         // Minimum file size to consider as valid output (default: 512 bytes)
}

// DefaultWatchdogConfig returns sensible defaults for the output watchdog.
func DefaultWatchdogConfig() OutputWatchdogConfig {
	return OutputWatchdogConfig{
		PollInterval:    5 * time.Second,
		StabilityWindow: 15 * time.Second,
		MinFileSize:     512,
	}
}

// OutputWatchdog monitors an output file path during agent execution.
// If the file appears and stabilizes (size unchanged for StabilityWindow),
// the watchdog signals that stable output is available for recovery.
type OutputWatchdog struct {
	path     string
	config   OutputWatchdogConfig
	logger   fileEnforcementLogger
	stableCh chan string
	stopCh   chan struct{}
	once     sync.Once
}

// NewOutputWatchdog creates a new output file watchdog.
func NewOutputWatchdog(path string, config OutputWatchdogConfig, logger fileEnforcementLogger) *OutputWatchdog {
	return &OutputWatchdog{
		path:     path,
		config:   config,
		logger:   logger,
		stableCh: make(chan string, 1),
		stopCh:   make(chan struct{}),
	}
}

// Start begins polling the output file in a background goroutine.
func (w *OutputWatchdog) Start() {
	go w.poll()
}

// Stop terminates the watchdog polling. Safe to call multiple times.
func (w *OutputWatchdog) Stop() {
	w.once.Do(func() {
		close(w.stopCh)
	})
}

// StableCh returns a channel that receives the file content when
// the output file has been stable for the configured window.
func (w *OutputWatchdog) StableCh() <-chan string {
	return w.stableCh
}

func (w *OutputWatchdog) poll() {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	var lastSize int64 = -1
	var stableSince time.Time

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			info, err := os.Stat(w.path)
			if err != nil {
				// File doesn't exist yet
				lastSize = -1
				stableSince = time.Time{}
				continue
			}

			size := info.Size()
			if size < w.config.MinFileSize {
				// File too small to be meaningful
				lastSize = size
				stableSince = time.Time{}
				continue
			}

			if size != lastSize {
				// File is still growing
				lastSize = size
				stableSince = time.Now()
				continue
			}

			// Size unchanged — check if stable long enough
			if stableSince.IsZero() {
				stableSince = time.Now()
				continue
			}

			if time.Since(stableSince) >= w.config.StabilityWindow {
				// File is stable — read and signal
				content, readErr := os.ReadFile(w.path)
				if readErr != nil {
					if w.logger != nil {
						w.logger.Warn("watchdog: failed to read stable file",
							"path", w.path, "error", readErr)
					}
					continue
				}

				if w.logger != nil {
					w.logger.Info("watchdog: output file stable",
						"path", w.path, "size", len(content))
				}

				select {
				case w.stableCh <- string(content):
				default:
				}
				return
			}
		}
	}
}
