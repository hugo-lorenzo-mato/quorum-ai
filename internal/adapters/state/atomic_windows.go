//go:build windows

package state

import (
	"os"
	"path/filepath"
	"time"
)

// atomicWriteFile writes data to a file atomically.
// On Windows, we use a write-rename pattern since renameio doesn't support Windows.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Write to a unique temp file in the same directory. A fixed ".tmp" name
	// causes collisions under concurrent writers (common in tests and CI).
	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return err
	}
	tempFile := f.Name()
	defer func() { _ = os.Remove(tempFile) }()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tempFile, perm)

	// Rename temp file to target. Windows does not allow renaming over an existing
	// file, and concurrent writers can temporarily lock the destination. Retry
	// with a small backoff to avoid flakiness.
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		if err := os.Rename(tempFile, path); err == nil {
			return nil
		} else {
			lastErr = err
		}

		// Best-effort replacement when destination exists.
		if _, statErr := os.Stat(path); statErr == nil {
			_ = os.Remove(path)
			if err := os.Rename(tempFile, path); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}

		time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
	}

	return lastErr
}
