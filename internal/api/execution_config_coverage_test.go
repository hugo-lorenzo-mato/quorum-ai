package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// ---------------------------------------------------------------------------
// resolveConfigMode
// ---------------------------------------------------------------------------

func TestResolveConfigMode_ExplicitInheritGlobal(t *testing.T) {
	t.Parallel()
	mode, err := resolveConfigMode("/any/root", project.ConfigModeInheritGlobal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeInheritGlobal {
		t.Errorf("expected %q, got %q", project.ConfigModeInheritGlobal, mode)
	}
}

func TestResolveConfigMode_ExplicitCustom(t *testing.T) {
	t.Parallel()
	mode, err := resolveConfigMode("/any/root", project.ConfigModeCustom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeCustom {
		t.Errorf("expected %q, got %q", project.ConfigModeCustom, mode)
	}
}

func TestResolveConfigMode_InferCustomWhenConfigExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("log:\n  level: info\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mode, err := resolveConfigMode(tmpDir, "") // no explicit mode
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeCustom {
		t.Errorf("expected %q, got %q", project.ConfigModeCustom, mode)
	}
}

func TestResolveConfigMode_InferInheritWhenNoConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// No .quorum/config.yaml created.

	mode, err := resolveConfigMode(tmpDir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeInheritGlobal {
		t.Errorf("expected %q, got %q", project.ConfigModeInheritGlobal, mode)
	}
}

func TestResolveConfigMode_WhitespaceExplicit(t *testing.T) {
	t.Parallel()
	mode, err := resolveConfigMode("/root", "  custom  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != project.ConfigModeCustom {
		t.Errorf("expected %q, got %q", project.ConfigModeCustom, mode)
	}
}

// ---------------------------------------------------------------------------
// ResolveEffectiveExecutionConfig
// ---------------------------------------------------------------------------

func TestResolveEffectiveExecutionConfig_NoProjectContext(t *testing.T) {
	t.Parallel()
	_, err := ResolveEffectiveExecutionConfig(context.Background())
	if err == nil {
		t.Fatal("expected error when no project context")
	}
}

func TestResolveEffectiveExecutionConfig_EmptyProjectRoot(t *testing.T) {
	t.Parallel()
	pc := &project.ProjectContext{
		ID:   "p1",
		Root: "",
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	_, err := ResolveEffectiveExecutionConfig(ctx)
	if err == nil {
		t.Fatal("expected error for empty project root")
	}
}

func TestResolveEffectiveExecutionConfig_CustomModeSuccess(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a valid project config file.
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(config.DefaultConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	pc := &project.ProjectContext{
		ID:         "p1",
		Root:       tmpDir,
		ConfigMode: project.ConfigModeCustom,
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	result, err := ResolveEffectiveExecutionConfig(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Config == nil {
		t.Fatal("expected non-nil config")
	}
	if result.ConfigScope != "project" {
		t.Errorf("expected scope 'project', got %q", result.ConfigScope)
	}
	if result.ConfigMode != project.ConfigModeCustom {
		t.Errorf("expected mode %q, got %q", project.ConfigModeCustom, result.ConfigMode)
	}
	if result.ConfigPath == "" {
		t.Error("expected non-empty config path")
	}
	if result.FileETag == "" {
		t.Error("expected non-empty FileETag")
	}
	if result.EffectiveETag == "" {
		t.Error("expected non-empty EffectiveETag")
	}
	if result.ProjectID != "p1" {
		t.Errorf("expected 'p1', got %q", result.ProjectID)
	}
	if result.ProjectRoot != tmpDir {
		t.Errorf("expected %q, got %q", tmpDir, result.ProjectRoot)
	}
	if result.RawYAML == nil {
		t.Error("expected non-nil RawYAML")
	}
}

func TestResolveEffectiveExecutionConfig_InheritGlobal(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))

	// Create global config so EnsureGlobalConfigFile finds it.
	globalDir := filepath.Join(tmpDir, ".config", "quorum")
	if err := os.MkdirAll(globalDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "config.yaml"), []byte(config.DefaultConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir() // separate empty project dir without .quorum
	pc := &project.ProjectContext{
		ID:         "p2",
		Root:       projectDir,
		ConfigMode: project.ConfigModeInheritGlobal,
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	result, err := ResolveEffectiveExecutionConfig(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ConfigScope != "global" {
		t.Errorf("expected scope 'global', got %q", result.ConfigScope)
	}
	if result.ConfigMode != project.ConfigModeInheritGlobal {
		t.Errorf("expected mode %q, got %q", project.ConfigModeInheritGlobal, result.ConfigMode)
	}
}

func TestResolveEffectiveExecutionConfig_ConfigFileMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// No .quorum/config.yaml and explicit custom mode → file missing error.

	pc := &project.ProjectContext{
		ID:         "p3",
		Root:       tmpDir,
		ConfigMode: project.ConfigModeCustom,
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	_, err := ResolveEffectiveExecutionConfig(ctx)
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestResolveEffectiveExecutionConfig_InvalidConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Write invalid YAML that parses but fails validation.
	// An empty YAML still loads defaults, so write something that triggers a validation error.
	invalidConfig := `
log:
  level: "info"
workflow:
  timeout: "5m"
  max_retries: 3
agents:
  default: "nonexistent_agent"
  claude:
    enabled: false
  gemini:
    enabled: false
  codex:
    enabled: false
  copilot:
    enabled: false
  opencode:
    enabled: false
phases:
  analyze:
    refiner:
      enabled: false
    moderator:
      enabled: false
    synthesizer:
      agent: "claude"
git:
  worktree:
    dir: ".worktrees"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(invalidConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	pc := &project.ProjectContext{
		ID:         "p4",
		Root:       tmpDir,
		ConfigMode: project.ConfigModeCustom,
	}
	ctx := middleware.WithProjectContext(context.Background(), pc)

	_, err := ResolveEffectiveExecutionConfig(ctx)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

// ---------------------------------------------------------------------------
// EffectiveExecutionConfig struct fields
// ---------------------------------------------------------------------------

func TestEffectiveExecutionConfig_StructFields(t *testing.T) {
	t.Parallel()
	cfg := &EffectiveExecutionConfig{
		Config:        &config.Config{},
		RawYAML:       []byte("test"),
		ConfigPath:    "/path/to/config.yaml",
		ConfigScope:   "project",
		ConfigMode:    "custom",
		FileETag:      "abc123",
		EffectiveETag: "def456",
		ProjectID:     "p1",
		ProjectRoot:   "/project",
	}

	if cfg.ProjectID != "p1" {
		t.Errorf("expected 'p1', got %q", cfg.ProjectID)
	}
	if cfg.FileETag != "abc123" {
		t.Errorf("expected 'abc123', got %q", cfg.FileETag)
	}
}
