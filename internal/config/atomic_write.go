package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to a file atomically.
// It writes to a temp file in the same directory and renames it over the target.
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	perm := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if err := tmpFile.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}

	return nil
}

// CalculateETag returns a quoted strong ETag for content.
func CalculateETag(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%q", hex.EncodeToString(sum[:]))
}
