package api

import (
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Windows reports no access when signaling the current process; treat that as existing.
	if runtime.GOOS == "windows" && pid == os.Getpid() {
		return true
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds, so we send signal 0.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func isLocalHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") || host == "127.0.0.1" {
		return true
	}
	local, err := os.Hostname()
	if err != nil || local == "" {
		return false
	}
	return strings.EqualFold(host, local)
}

func isProvablyOrphan(rec *core.RunningWorkflowRecord) bool {
	if rec == nil || rec.LockHolderPID == nil {
		return false
	}
	if !isLocalHost(rec.LockHolderHost) {
		return false
	}
	return !processExists(*rec.LockHolderPID)
}
