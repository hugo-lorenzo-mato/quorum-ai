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

	// Wait for process to exit within grace period
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(gracePeriod):
		// Process didn't exit, escalate to SIGKILL
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return nil
	}
}
