//go:build windows

package diagnostics

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procGetProcessHandleCount = kernel32.NewProc("GetProcessHandleCount")
)

// CountFDs returns the number of open handles and a soft limit.
// On Windows, handles include files, threads, events, mutexes, etc.
// This is broader than POSIX file descriptors but serves as a useful
// resource leak indicator.
func CountFDs() (open, limit int) {
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, 0
	}

	var count uint32
	ret, _, _ := procGetProcessHandleCount.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&count)),
	)
	if ret == 0 {
		return 0, 0
	}

	// Windows has no rlimit equivalent for handles.
	// Theoretical max is ~16 million, but we use 10000 as a practical
	// soft limit for percentage calculations and threshold warnings.
	const softLimit = 10000

	// #nosec G115 -- handle count is always within int range
	return int(count), softLimit
}
