package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalConfigPath(t *testing.T) {
	t.Parallel()

	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath() error = %v", err)
	}

	if path == "" {
		t.Error("GlobalConfigPath() returned empty string")
	}

	// Path should end with global-config.yaml
	if !strings.HasSuffix(path, "global-config.yaml") {
		t.Errorf("GlobalConfigPath() = %q, want suffix 'global-config.yaml'", path)
	}

	// Path should contain .quorum-registry
	if !strings.Contains(path, ".quorum-registry") {
		t.Errorf("GlobalConfigPath() = %q, want to contain '.quorum-registry'", path)
	}

	// Path should be absolute
	if !filepath.IsAbs(path) {
		t.Errorf("GlobalConfigPath() = %q, want absolute path", path)
	}
}

func TestGlobalConfigPath_ContainsHomeDir(t *testing.T) {
	t.Parallel()

	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath() error = %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}

	if !strings.HasPrefix(path, homeDir) {
		t.Errorf("GlobalConfigPath() = %q, want prefix %q", path, homeDir)
	}
}

func TestEnsureGlobalConfigFile_CreatesNewFile(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	path, err := EnsureGlobalConfigFile()
	if err != nil {
		t.Fatalf("EnsureGlobalConfigFile() error = %v", err)
	}

	if path == "" {
		t.Fatal("EnsureGlobalConfigFile() returned empty path")
	}

	// File should exist
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}

	// File should not be empty
	if info.Size() == 0 {
		t.Error("global config file should not be empty")
	}

	// File should have restrictive permissions
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// File content should match DefaultConfigYAML
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(data) != DefaultConfigYAML {
		t.Error("global config file content does not match DefaultConfigYAML")
	}
}

func TestEnsureGlobalConfigFile_ExistingFile(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create the file first
	registryDir := filepath.Join(tmpDir, ".quorum-registry")
	if err := os.MkdirAll(registryDir, 0o750); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	customContent := "# Custom config\nagents:\n  default: claude\n"
	configPath := filepath.Join(registryDir, "global-config.yaml")
	if err := os.WriteFile(configPath, []byte(customContent), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// EnsureGlobalConfigFile should return the existing file without overwriting
	path, err := EnsureGlobalConfigFile()
	if err != nil {
		t.Fatalf("EnsureGlobalConfigFile() error = %v", err)
	}

	if path != configPath {
		t.Errorf("EnsureGlobalConfigFile() = %q, want %q", path, configPath)
	}

	// Content should still be the custom content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(data) != customContent {
		t.Error("existing config file content should not be overwritten")
	}
}

func TestEnsureGlobalConfigFile_CreatesDirectory(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Ensure .quorum-registry directory does not exist
	registryDir := filepath.Join(tmpDir, ".quorum-registry")
	if _, err := os.Stat(registryDir); err == nil {
		t.Fatal(".quorum-registry should not exist before test")
	}

	path, err := EnsureGlobalConfigFile()
	if err != nil {
		t.Fatalf("EnsureGlobalConfigFile() error = %v", err)
	}

	// Directory should have been created
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat directory error = %v", err)
	}
	if !dirInfo.IsDir() {
		t.Error("parent should be a directory")
	}
}

func TestEnsureGlobalConfigFile_Idempotent(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Call twice
	path1, err := EnsureGlobalConfigFile()
	if err != nil {
		t.Fatalf("First EnsureGlobalConfigFile() error = %v", err)
	}

	path2, err := EnsureGlobalConfigFile()
	if err != nil {
		t.Fatalf("Second EnsureGlobalConfigFile() error = %v", err)
	}

	if path1 != path2 {
		t.Errorf("paths should be equal: %q vs %q", path1, path2)
	}

	// Content should still be the default
	data, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(data) != DefaultConfigYAML {
		t.Error("content should still match DefaultConfigYAML after second call")
	}
}

func TestGlobalConfigPath_DeterministicPath(t *testing.T) {
	t.Parallel()

	path1, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("First GlobalConfigPath() error = %v", err)
	}

	path2, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("Second GlobalConfigPath() error = %v", err)
	}

	if path1 != path2 {
		t.Errorf("GlobalConfigPath() not deterministic: %q vs %q", path1, path2)
	}
}

func TestGlobalConfigPath_StructureIsCorrect(t *testing.T) {
	t.Parallel()

	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath() error = %v", err)
	}

	dir := filepath.Dir(path)
	base := filepath.Base(dir)
	if base != ".quorum-registry" {
		t.Errorf("parent directory = %q, want '.quorum-registry'", base)
	}

	filename := filepath.Base(path)
	if filename != "global-config.yaml" {
		t.Errorf("filename = %q, want 'global-config.yaml'", filename)
	}
}
