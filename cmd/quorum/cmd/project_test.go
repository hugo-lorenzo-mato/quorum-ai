package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

func TestResolveProjectByValue_ByPath(t *testing.T) {
	t.Parallel()
	// Create a temporary directory with .quorum
	tmpDir := t.TempDir()
	quorumDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(quorumDir, 0755); err != nil {
		t.Fatalf("failed to create .quorum directory: %v", err)
	}

	// Create a temporary registry
	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	// Add a project
	p, err := registry.AddProject(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// Test resolving by path
	resolved, err := resolveProjectByValue(ctx, registry, tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve by path: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}
}

func TestResolveProjectByValue_ByID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	quorumDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(quorumDir, 0755); err != nil {
		t.Fatalf("failed to create .quorum directory: %v", err)
	}

	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	p, err := registry.AddProject(ctx, tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// Test resolving by ID
	resolved, err := resolveProjectByValue(ctx, registry, p.ID)
	if err != nil {
		t.Fatalf("failed to resolve by ID: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}
}

func TestResolveProjectByValue_ByName(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	quorumDir := filepath.Join(tmpDir, ".quorum")
	if err := os.MkdirAll(quorumDir, 0755); err != nil {
		t.Fatalf("failed to create .quorum directory: %v", err)
	}

	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	customName := "My Test Project"
	p, err := registry.AddProject(ctx, tmpDir, &project.AddProjectOptions{Name: customName})
	if err != nil {
		t.Fatalf("failed to add project: %v", err)
	}

	// Test resolving by exact name
	resolved, err := resolveProjectByValue(ctx, registry, customName)
	if err != nil {
		t.Fatalf("failed to resolve by name: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}

	// Test resolving by partial name (case insensitive)
	resolved, err = resolveProjectByValue(ctx, registry, "test project")
	if err != nil {
		t.Fatalf("failed to resolve by partial name: %v", err)
	}
	if resolved.ID != p.ID {
		t.Errorf("expected ID %s, got %s", p.ID, resolved.ID)
	}
}

func TestResolveProjectByValue_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := project.NewFileRegistry(project.WithConfigPath(registryPath))
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	defer registry.Close()

	ctx := context.Background()

	_, err = resolveProjectByValue(ctx, registry, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestProjectCmd_Structure(t *testing.T) {
	t.Parallel()
	// Verify project command exists
	if projectCmd == nil {
		t.Fatal("projectCmd is nil")
	}
	if projectCmd.Use != "project" {
		t.Errorf("expected Use 'project', got %s", projectCmd.Use)
	}

	// Verify subcommands
	subcommands := projectCmd.Commands()
	expectedCmds := map[string]bool{
		"add":      false,
		"list":     false,
		"remove":   false,
		"default":  false,
		"validate": false,
	}

	for _, cmd := range subcommands {
		if _, ok := expectedCmds[cmd.Name()]; ok {
			expectedCmds[cmd.Name()] = true
		}
	}

	for name, found := range expectedCmds {
		if !found {
			t.Errorf("expected subcommand %s not found", name)
		}
	}
}

func TestGetProjectID(t *testing.T) {
	t.Parallel()
	// Save original value
	original := projectID
	defer func() { projectID = original }()

	// Test empty
	projectID = ""
	if got := GetProjectID(); got != "" {
		t.Errorf("expected empty string, got %s", got)
	}

	// Test with value
	projectID = "test-project-id"
	if got := GetProjectID(); got != "test-project-id" {
		t.Errorf("expected test-project-id, got %s", got)
	}
}
