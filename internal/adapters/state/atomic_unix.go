//go:build !windows

package state

import (
	"os"

	"github.com/google/renameio/v2"
)

// atomicWriteFile writes data to a file atomically.
// On Unix systems, this uses renameio for atomic writes.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return renameio.WriteFile(path, data, perm)
}
