//go:build windows

package state

import (
	"os"
	"path/filepath"
)

// atomicWriteFile writes data to a file atomically.
// On Windows, we use a write-rename pattern since renameio doesn't support Windows.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write to temp file in same directory
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, perm); err != nil {
		return err
	}

	// Rename temp file to target (atomic on Windows when same volume)
	if err := os.Rename(tempFile, path); err != nil {
		// Windows does not allow renaming over an existing file.
		if _, statErr := os.Stat(path); statErr == nil {
			_ = os.Remove(path)
			if retryErr := os.Rename(tempFile, path); retryErr == nil {
				return nil
			} else {
				err = retryErr
			}
		}
		os.Remove(tempFile) // Clean up on failure
		return err
	}

	return nil
}
