package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// --- resolveProjectByValue ---

func TestResolveProjectByValue_AmbiguousName(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two subdirectories with .quorum folders and similar names
	proj1Dir := filepath.Join(tmpDir, "project-alpha-one")
	proj2Dir := filepath.Join(tmpDir, "project-alpha-two")
	for _, d := range []string{proj1Dir, proj2Dir} {
		if err := os.MkdirAll(filepath.Join(d, ".quorum"), 0o755); err != nil {
			t.Fatalf("failed to create .quorum directory: %v", err)
		}
	}

	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	_, err = registry.AddProject(ctx, proj1Dir, &project.AddProjectOptions{Name: "Project Alpha One"})
	if err != nil {
		t.Fatalf("failed to add project 1: %v", err)
	}
	_, err = registry.AddProject(ctx, proj2Dir, &project.AddProjectOptions{Name: "Project Alpha Two"})
	if err != nil {
		t.Fatalf("failed to add project 2: %v", err)
	}

	// "Alpha" matches both projects
	_, err = resolveProjectByValue(ctx, registry, "Alpha")
	if err == nil {
		t.Fatal("expected error for ambiguous name")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
}

func TestResolveProjectByValue_CaseInsensitiveExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "proj")
	if err := os.MkdirAll(filepath.Join(projDir, ".quorum"), 0o755); err != nil {
		t.Fatalf("failed to create .quorum directory: %v", err)
	}

	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	p, err := registry.AddProject(ctx, projDir, &project.AddProjectOptions{Name: "MyProject"})
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// Exact match (case insensitive)
	resolved, err := resolveProjectByValue(ctx, registry, "myproject")
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}
}

func TestResolveProjectByValue_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "relative-test")
	if err := os.MkdirAll(filepath.Join(projDir, ".quorum"), 0o755); err != nil {
		t.Fatalf("failed to create .quorum directory: %v", err)
	}

	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	p, err := registry.AddProject(ctx, projDir, nil)
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// Resolve by absolute path
	resolved, err := resolveProjectByValue(ctx, registry, projDir)
	if err != nil {
		t.Fatalf("failed to resolve by absolute path: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}
}

// --- initializeProject ---

func TestInitializeProject_CreatesStructure(t *testing.T) {
	tmpDir := t.TempDir()

	err := initializeProject(tmpDir, false)
	if err != nil {
		t.Fatalf("initializeProject failed: %v", err)
	}

	// Verify .quorum/config.yaml exists
	configPath := filepath.Join(tmpDir, ".quorum", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("expected config.yaml to exist: %v", err)
	}

	// Verify subdirectories
	for _, dir := range []string{"state", "logs", "runs"} {
		dirPath := filepath.Join(tmpDir, ".quorum", dir)
		if _, err := os.Stat(dirPath); err != nil {
			t.Errorf("expected %s directory to exist: %v", dir, err)
		}
	}
}

func TestInitializeProject_AlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// First init
	if err := initializeProject(tmpDir, false); err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Write some content to the config
	configPath := filepath.Join(tmpDir, ".quorum", "config.yaml")
	if err := os.WriteFile(configPath, []byte("custom: true"), 0o600); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	// Second init without force should be a no-op
	if err := initializeProject(tmpDir, false); err != nil {
		t.Fatalf("second init failed: %v", err)
	}

	// Verify custom config preserved
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(data) != "custom: true" {
		t.Errorf("expected config to be preserved, got: %s", string(data))
	}
}

func TestInitializeProject_ForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// First init
	if err := initializeProject(tmpDir, false); err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	// Write custom config
	configPath := filepath.Join(tmpDir, ".quorum", "config.yaml")
	if err := os.WriteFile(configPath, []byte("custom: true"), 0o600); err != nil {
		t.Fatalf("failed to write custom config: %v", err)
	}

	// Force reinit
	if err := initializeProject(tmpDir, true); err != nil {
		t.Fatalf("force init failed: %v", err)
	}

	// Verify default config was written (not custom)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(data) == "custom: true" {
		t.Error("expected config to be overwritten")
	}
}

// --- tryResolveProjectPath ---

func TestTryResolveProjectPath_AbsolutePath(t *testing.T) {
	t.Parallel()
	result := tryResolveProjectPath("/some/absolute/path")
	if result != "/some/absolute/path" {
		t.Errorf("expected /some/absolute/path, got %s", result)
	}
}

func TestTryResolveProjectPath_RelativePath(t *testing.T) {
	t.Parallel()
	result := tryResolveProjectPath("./relative/path")
	if result != "./relative/path" {
		t.Errorf("expected ./relative/path, got %s", result)
	}
}

func TestTryResolveProjectPath_NameOrID(t *testing.T) {
	t.Parallel()
	result := tryResolveProjectPath("my-project")
	if result != "" {
		t.Errorf("expected empty string for name/ID, got %s", result)
	}
}

// --- GetProjectID ---

func TestGetProjectID_Empty(t *testing.T) {
	original := projectID
	defer func() { projectID = original }()

	projectID = ""
	if got := GetProjectID(); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestGetProjectID_WithValue(t *testing.T) {
	original := projectID
	defer func() { projectID = original }()

	projectID = "proj-abc"
	if got := GetProjectID(); got != "proj-abc" {
		t.Errorf("expected proj-abc, got %s", got)
	}
}
