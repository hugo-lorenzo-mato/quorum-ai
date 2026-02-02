package api

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCalculateETag(t *testing.T) {
	cfg := &config.Config{
		Log: config.LogConfig{Level: "info", Format: "auto"},
	}

	etag1, err := calculateETag(cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, etag1)
	assert.Len(t, etag1, 32, "ETag should be 32 hex characters")

	// Same config should produce same ETag
	etag2, err := calculateETag(cfg)
	assert.NoError(t, err)
	assert.Equal(t, etag1, etag2)

	// Different config should produce different ETag
	cfg.Log.Level = "debug"
	etag3, err := calculateETag(cfg)
	assert.NoError(t, err)
	assert.NotEqual(t, etag1, etag3)
}

func TestCalculateETagFromFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Non-existent file should return empty ETag
	etag, err := calculateETagFromFile(configPath)
	assert.NoError(t, err)
	assert.Empty(t, etag)

	// Create file
	content := []byte("log:\n  level: info\n")
	err = os.WriteFile(configPath, content, 0o600)
	require.NoError(t, err)

	// Now should return valid ETag
	etag, err = calculateETagFromFile(configPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, etag)
	assert.Len(t, etag, 32)

	// Same content should produce same ETag
	etag2, err := calculateETagFromFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, etag, etag2)

	// Different content should produce different ETag
	content2 := []byte("log:\n  level: debug\n")
	err = os.WriteFile(configPath, content2, 0o600)
	require.NoError(t, err)

	etag3, err := calculateETagFromFile(configPath)
	assert.NoError(t, err)
	assert.NotEqual(t, etag, etag3)
}

func TestAtomicWriteConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".quorum", "config.yaml")

	cfg := &config.Config{
		Log: config.LogConfig{Level: "info", Format: "auto"},
	}

	err := atomicWriteConfig(cfg, configPath)
	assert.NoError(t, err)

	// Verify file exists and is readable
	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Contains(t, string(data), "level: info")

	// Verify permissions (Unix only - Windows doesn't support Unix permissions)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestAtomicWriteConfig_SnakeCaseKeys(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".quorum", "config.yaml")

	cfg := &config.Config{
		Workflow: config.WorkflowConfig{MaxRetries: 9},
		Git: config.GitConfig{
			Worktree: config.WorktreeConfig{AutoClean: true},
		},
		State: config.StateConfig{LockTTL: "2h"},
	}

	err := atomicWriteConfig(cfg, configPath)
	assert.NoError(t, err)

	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "max_retries: 9")
	assert.Contains(t, content, "auto_clean: true")
	assert.Contains(t, content, "lock_ttl: 2h")
	assert.NotContains(t, content, "maxretries:")
	assert.NotContains(t, content, "autoclean:")
	assert.NotContains(t, content, "lockttl:")
}

func TestAtomicWriteConfig_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nested", "deep", "config.yaml")

	cfg := &config.Config{
		Log: config.LogConfig{Level: "warn"},
	}

	err := atomicWriteConfig(cfg, configPath)
	assert.NoError(t, err)

	// File should exist
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestAtomicWriteConfig_ConcurrentWrites(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			cfg := &config.Config{
				Log: config.LogConfig{Level: fmt.Sprintf("level-%d", n)},
			}
			if err := atomicWriteConfig(cfg, configPath); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// All writes should succeed (atomic rename is serialized by kernel)
	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}

	// File should exist and be valid YAML
	data, err := os.ReadFile(configPath)
	assert.NoError(t, err)

	var cfg config.Config
	err = yaml.Unmarshal(data, &cfg)
	assert.NoError(t, err)
}

func TestAtomicWriteConfig_NoTempFileOnSuccess(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".quorum")
	configPath := filepath.Join(configDir, "config.yaml")

	cfg := &config.Config{
		Log: config.LogConfig{Level: "info"},
	}

	err := atomicWriteConfig(cfg, configPath)
	assert.NoError(t, err)

	// Check no temp files left
	entries, err := os.ReadDir(configDir)
	assert.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, entry.Name() != "config.yaml" && filepath.Ext(entry.Name()) == ".tmp",
			"temp file should not remain: %s", entry.Name())
	}
}

func TestETagMatch(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	cfg := &config.Config{
		Log: config.LogConfig{Level: "info"},
	}

	// Write config
	err := atomicWriteConfig(cfg, configPath)
	require.NoError(t, err)

	// Get ETag
	etag, err := calculateETagFromFile(configPath)
	require.NoError(t, err)

	// Matching ETag should pass
	matches, currentETag, err := ETagMatch(etag, configPath)
	assert.NoError(t, err)
	assert.True(t, matches)
	assert.Equal(t, etag, currentETag)

	// Non-matching ETag should fail
	matches, _, err = ETagMatch("wrong-etag", configPath)
	assert.NoError(t, err)
	assert.False(t, matches)

	// Empty ETag should always match (first save)
	matches, _, err = ETagMatch("", configPath)
	assert.NoError(t, err)
	assert.True(t, matches)
}

func TestETagMatch_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nonexistent.yaml")

	// Empty ETag with non-existent file should match
	matches, currentETag, err := ETagMatch("", configPath)
	assert.NoError(t, err)
	assert.True(t, matches)
	assert.Empty(t, currentETag)

	// Non-empty ETag with non-existent file should not match
	matches, _, err = ETagMatch("some-etag", configPath)
	assert.NoError(t, err)
	assert.False(t, matches)
}

func TestGetConfigFileMeta(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Non-existent file
	meta, err := getConfigFileMeta(configPath)
	assert.NoError(t, err)
	assert.False(t, meta.Exists)
	assert.Empty(t, meta.ETag)

	// Create file
	cfg := &config.Config{
		Log: config.LogConfig{Level: "info"},
	}
	err = atomicWriteConfig(cfg, configPath)
	require.NoError(t, err)

	// Now should have metadata
	meta, err = getConfigFileMeta(configPath)
	assert.NoError(t, err)
	assert.True(t, meta.Exists)
	assert.NotEmpty(t, meta.ETag)
	assert.False(t, meta.LastModified.IsZero())
}
