//go:build !windows

package cli

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// configureProcAttr sets up process group isolation so child processes
// can be signaled as a group.
func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// setActiveProcess records the running command for graceful termination.
func (b *BaseAdapter) setActiveProcess(cmd *exec.Cmd) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeCmd = cmd
}

// clearActiveProcess clears the active command reference.
func (b *BaseAdapter) clearActiveProcess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.activeCmd = nil
}

// GracefulKill sends SIGTERM to the process group, waits for gracePeriod,
// then sends SIGKILL if the process hasn't exited.
//
// This function does NOT call cmd.Wait(). The caller is expected to call
// cmd.Wait() separately (typically via a waitDone channel). Calling cmd.Wait()
// here would race with the caller's Wait and block forever on Go 1.20+.
func (b *BaseAdapter) GracefulKill(gracePeriod time.Duration) error {
	b.mu.Lock()
	cmd := b.activeCmd
	b.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		// Process may have already exited
		return fmt.Errorf("getpgid(%d): %w", pid, err)
	}

	// Send SIGTERM to the entire process group
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		// ESRCH means process already gone
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("sigterm pgid %d: %w", pgid, err)
	}

	// Poll for process exit within grace period. We avoid cmd.Wait() here
	// because it races with the caller's cmd.Wait() goroutine — on Go 1.20+
	// only one Wait can succeed, the other blocks forever on Process.Wait.
	deadline := time.After(gracePeriod)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			// Process didn't exit, escalate to SIGKILL
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
			return nil
		case <-ticker.C:
			// Signal 0 checks if the process (group leader) is still alive
			if err := syscall.Kill(pid, 0); err != nil {
				return nil // Process exited
			}
		}
	}
}
