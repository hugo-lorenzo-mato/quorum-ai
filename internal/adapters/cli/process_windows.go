//go:build windows

package cli

import (
	"os/exec"
	"time"
)

// configureProcAttr is a no-op on Windows (Setpgid not supported).
func configureProcAttr(_ *exec.Cmd) {}

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

// GracefulKill on Windows falls back to Process.Kill().
func (b *BaseAdapter) GracefulKill(_ time.Duration) error {
	b.mu.Lock()
	cmd := b.activeCmd
	b.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
