//go:build windows

package diagnostics

// CountFDs returns the number of open file descriptors and the maximum allowed.
// On Windows, this information is not easily accessible via the same mechanisms
// as Linux/macOS. This stub returns 0, 0 to indicate unavailable data.
// Future enhancement: use NtQuerySystemInformation or GetProcessHandleCount.
func CountFDs() (open, limit int) {
	// Windows doesn't have /proc/self/fd or /dev/fd equivalents.
	// GetProcessHandleCount could be used but requires additional Windows API calls.
	// For now, return 0 to indicate FD monitoring is not available on Windows.
	return 0, 0
}
