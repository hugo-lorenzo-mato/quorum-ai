package diagnostics

import (
	"fmt"
	"io"
	"os/exec"

	"log/slog"
)

// PreflightResult contains the result of pre-execution checks.
type PreflightResult struct {
	OK       bool
	Warnings []string
	Errors   []string
	Snapshot ResourceSnapshot
}

// SafeExecutor wraps command execution with resource safety checks.
type SafeExecutor struct {
	monitor          *ResourceMonitor
	dumpWriter       *CrashDumpWriter
	logger           *slog.Logger
	preflightEnabled bool
	minFreeFDPercent int
	minFreeMemoryMB  int
}

// NewSafeExecutor creates a safe executor.
func NewSafeExecutor(
	monitor *ResourceMonitor,
	dumpWriter *CrashDumpWriter,
	logger *slog.Logger,
	preflightEnabled bool,
	minFreeFDPercent int,
	minFreeMemoryMB int,
) *SafeExecutor {
	return &SafeExecutor{
		monitor:          monitor,
		dumpWriter:       dumpWriter,
		logger:           logger,
		preflightEnabled: preflightEnabled,
		minFreeFDPercent: minFreeFDPercent,
		minFreeMemoryMB:  minFreeMemoryMB,
	}
}

// RunPreflight performs pre-execution health checks.
func (e *SafeExecutor) RunPreflight() PreflightResult {
	result := PreflightResult{OK: true}

	if !e.preflightEnabled || e.monitor == nil {
		return result
	}

	// Take snapshot for the result
	result.Snapshot = e.monitor.TakeSnapshot()

	// Check FD availability
	freeFDPercent := 100.0 - result.Snapshot.FDUsagePercent
	if e.minFreeFDPercent > 0 && freeFDPercent < float64(e.minFreeFDPercent) {
		result.OK = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("insufficient free FDs: %.1f%% free (minimum: %d%%)",
				freeFDPercent, e.minFreeFDPercent))
	} else if e.minFreeFDPercent > 0 && freeFDPercent < float64(e.minFreeFDPercent)*1.5 {
		// Warning if approaching threshold
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("FD usage approaching limit: %.1f%% free", freeFDPercent))
	}

	// Check trends if we have enough history
	if e.monitor != nil {
		trend := e.monitor.GetTrend()
		if !trend.IsHealthy {
			result.Warnings = append(result.Warnings, trend.Warnings...)
		}
	}

	return result
}

// PipeSet holds stdout and stderr pipes with their cleanup function.
type PipeSet struct {
	Stdout  io.ReadCloser
	Stderr  io.ReadCloser
	cleanup func()
	cleaned bool
}

// Cleanup closes the pipes and decrements active command count.
// Safe to call multiple times.
func (p *PipeSet) Cleanup() {
	if p.cleaned {
		return
	}
	p.cleaned = true
	if p.cleanup != nil {
		p.cleanup()
	}
}

// PrepareCommand sets up a command with safe pipe handling.
// Returns a PipeSet with a Cleanup function that MUST be called even if Start() fails.
//
// Usage:
//
//	pipes, err := executor.PrepareCommand(cmd)
//	if err != nil {
//	    return err
//	}
//	defer pipes.Cleanup() // Called even if Start() fails
//
//	if err := cmd.Start(); err != nil {
//	    return err // pipes.Cleanup() still runs
//	}
func (e *SafeExecutor) PrepareCommand(cmd *exec.Cmd) (*PipeSet, error) {
	// Track this command
	if e.monitor != nil {
		e.monitor.IncrementCommandCount()
	}

	var stdoutPipe, stderrPipe io.ReadCloser
	var err error

	// Create stdout pipe
	stdoutPipe, err = cmd.StdoutPipe()
	if err != nil {
		if e.monitor != nil {
			e.monitor.DecrementActiveCommands()
		}
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Create stderr pipe
	stderrPipe, err = cmd.StderrPipe()
	if err != nil {
		// Cleanup stdout pipe since stderr failed
		_ = stdoutPipe.Close()
		if e.monitor != nil {
			e.monitor.DecrementActiveCommands()
		}
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	// Create cleanup function
	pipes := &PipeSet{
		Stdout: stdoutPipe,
		Stderr: stderrPipe,
	}

	pipes.cleanup = func() {
		// Close pipes if they're still open
		// Note: After successful Start()+Wait(), these are already closed by exec package
		// But if Start() fails, we need to close them explicitly
		if stdoutPipe != nil {
			_ = stdoutPipe.Close()
		}
		if stderrPipe != nil {
			_ = stderrPipe.Close()
		}

		if e.monitor != nil {
			e.monitor.DecrementActiveCommands()
		}
	}

	return pipes, nil
}

// PrepareStderrOnly sets up a command with only stderr pipe.
// Returns the stderr pipe and a cleanup function that MUST be called even if Start() fails.
func (e *SafeExecutor) PrepareStderrOnly(cmd *exec.Cmd) (io.ReadCloser, func(), error) {
	// Track this command
	if e.monitor != nil {
		e.monitor.IncrementCommandCount()
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if e.monitor != nil {
			e.monitor.DecrementActiveCommands()
		}
		return nil, nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	cleanup := func() {
		if stderrPipe != nil {
			_ = stderrPipe.Close()
		}
		if e.monitor != nil {
			e.monitor.DecrementActiveCommands()
		}
	}

	return stderrPipe, cleanup, nil
}

// WrapExecution wraps a command execution function with crash dump recovery.
// The function will capture diagnostics and write a crash dump if a panic occurs.
//
// Usage:
//
//	result, err := executor.WrapExecution(func() (*Result, error) {
//	    // ... command execution logic
//	    return result, nil
//	})
func (e *SafeExecutor) WrapExecution(fn func() error) (err error) {
	if e.dumpWriter != nil {
		defer e.dumpWriter.RecoverAndReturn(&err)
	}
	return fn()
}
