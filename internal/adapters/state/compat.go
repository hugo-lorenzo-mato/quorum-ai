package state

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// stateEnvelope is the archive export/import envelope for workflow snapshots.
type stateEnvelope struct {
	Version   int                 `json:"version"`
	Checksum  string              `json:"checksum"`
	UpdatedAt time.Time           `json:"updated_at"`
	State     *core.WorkflowState `json:"state"`
}

// lockInfo represents lock file contents.
type lockInfo struct {
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// processExists checks if a process is running.
func processExists(pid int) bool {
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
