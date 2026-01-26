//go:build darwin

package diagnostics

import (
	"os"
	"syscall"
)

// CountFDs returns the number of open file descriptors and the maximum allowed.
func CountFDs() (open, limit int) {
	// Count open FDs by reading /dev/fd (macOS equivalent of /proc/self/fd)
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		return 0, 0
	}
	open = len(entries)

	// Get max FDs from rlimit
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err == nil {
		// #nosec G115 -- rlimit values are always within int range on supported platforms
		limit = int(rlim.Cur)
	}

	return open, limit
}
