package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"gopkg.in/yaml.v3"
)

// calculateETag generates an ETag from config content.
// Uses SHA-256 hash of the YAML representation.
func calculateETag(cfg *config.Config) (string, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config for ETag: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16]), nil // Use first 16 bytes (32 hex chars)
}

// calculateETagFromFile generates an ETag from file content.
func calculateETagFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No file = no ETag
		}
		return "", fmt.Errorf("failed to read config for ETag: %w", err)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:16]), nil
}

// atomicWriteConfig writes config atomically using temp file + rename.
// This ensures the config file is never in a partially-written state.
func atomicWriteConfig(cfg *config.Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create temp file in same directory (required for atomic rename)
	tempFile, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up temp file on any error
	defer func() {
		if tempPath != "" {
			os.Remove(tempPath)
		}
	}()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk to ensure data is persisted
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set proper permissions
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename (on POSIX systems)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Clear temp path to prevent cleanup
	tempPath = ""

	// Sync directory to ensure rename is persisted
	dirFile, err := os.Open(dir)
	if err == nil {
		dirFile.Sync()
		dirFile.Close()
	}

	return nil
}

// ETagMatch compares provided ETag with current file ETag.
// Returns (matches, currentETag, error).
func ETagMatch(providedETag, configPath string) (bool, string, error) {
	currentETag, err := calculateETagFromFile(configPath)
	if err != nil {
		return false, "", err
	}

	// If no ETag provided, always match (first save)
	if providedETag == "" {
		return true, currentETag, nil
	}

	// If no file exists, match only if provided is empty
	if currentETag == "" {
		return providedETag == "", "", nil
	}

	return providedETag == currentETag, currentETag, nil
}

// ConfigFileMeta contains metadata about the config file.
type ConfigFileMeta struct {
	ETag         string
	LastModified time.Time
	Exists       bool
}

// getConfigFileMeta retrieves metadata about the config file.
func getConfigFileMeta(path string) (ConfigFileMeta, error) {
	meta := ConfigFileMeta{}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return meta, nil // File doesn't exist
		}
		return meta, err
	}

	meta.Exists = true
	meta.LastModified = info.ModTime()

	etag, err := calculateETagFromFile(path)
	if err != nil {
		return meta, err
	}
	meta.ETag = etag

	return meta, nil
}
