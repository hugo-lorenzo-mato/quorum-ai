package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupTestRegistry(t *testing.T) (*FileRegistry, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "quorum-registry-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "projects.yaml")

	registry, err := NewFileRegistry(
		WithConfigPath(configPath),
		WithAutoSave(true),
		WithBackup(false),
	)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create registry: %v", err)
	}

	cleanup := func() {
		registry.Close()
		os.RemoveAll(tmpDir)
	}

	return registry, tmpDir, cleanup
}

func createTestProject(t *testing.T, baseDir, name string) string {
	t.Helper()

	projectDir := filepath.Join(baseDir, name)
	quorumDir := filepath.Join(projectDir, ".quorum")

	if err := os.MkdirAll(quorumDir, 0o750); err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	// Create a minimal config
	configPath := filepath.Join(quorumDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0o640); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	return projectDir
}

func TestNewFileRegistry(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}

	projects, err := registry.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestAddProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	project, err := registry.AddProject(ctx, projectDir, nil)
	if err != nil {
		t.Fatalf("AddProject failed: %v", err)
	}

	if project.ID == "" {
		t.Error("expected non-empty project ID")
	}
	if project.Path != projectDir {
		t.Errorf("expected path %s, got %s", projectDir, project.Path)
	}
	if project.Status != StatusHealthy {
		t.Errorf("expected status %s, got %s", StatusHealthy, project.Status)
	}
	if project.Name == "" {
		t.Error("expected non-empty name")
	}
	if project.Color == "" {
		t.Error("expected non-empty color")
	}
}

func TestAddProjectWithOptions(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	opts := &AddProjectOptions{
		Name:  "Custom Name",
		Color: "#FF0000",
	}

	project, err := registry.AddProject(ctx, projectDir, opts)
	if err != nil {
		t.Fatalf("AddProject failed: %v", err)
	}

	if project.Name != "Custom Name" {
		t.Errorf("expected name 'Custom Name', got '%s'", project.Name)
	}
	if project.Color != "#FF0000" {
		t.Errorf("expected color '#FF0000', got '%s'", project.Color)
	}
}

func TestAddProjectAlreadyExists(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	_, err := registry.AddProject(ctx, projectDir, nil)
	if err != nil {
		t.Fatalf("first AddProject failed: %v", err)
	}

	_, err = registry.AddProject(ctx, projectDir, nil)
	if err != ErrProjectAlreadyExists {
		t.Errorf("expected ErrProjectAlreadyExists, got %v", err)
	}
}

func TestAddProjectNotQuorum(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	// Create directory without .quorum
	projectDir := filepath.Join(tmpDir, "not-quorum")
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	ctx := context.Background()
	_, err := registry.AddProject(ctx, projectDir, nil)
	if err == nil {
		t.Error("expected error for non-quorum project")
	}
}

func TestGetProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	retrieved, err := registry.GetProject(ctx, added.ID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}

	if retrieved.ID != added.ID {
		t.Errorf("ID mismatch: expected %s, got %s", added.ID, retrieved.ID)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	_, err := registry.GetProject(context.Background(), "nonexistent")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestGetProjectByPath(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	retrieved, err := registry.GetProjectByPath(ctx, projectDir)
	if err != nil {
		t.Fatalf("GetProjectByPath failed: %v", err)
	}

	if retrieved.ID != added.ID {
		t.Errorf("ID mismatch: expected %s, got %s", added.ID, retrieved.ID)
	}
}

func TestRemoveProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	err := registry.RemoveProject(ctx, added.ID)
	if err != nil {
		t.Fatalf("RemoveProject failed: %v", err)
	}

	_, err = registry.GetProject(ctx, added.ID)
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound after removal, got %v", err)
	}
}

func TestRemoveProjectNotFound(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.RemoveProject(context.Background(), "nonexistent")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestValidateProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	// Should be healthy
	err := registry.ValidateProject(ctx, added.ID)
	if err != nil {
		t.Errorf("ValidateProject failed: %v", err)
	}

	// Remove the .quorum directory
	os.RemoveAll(filepath.Join(projectDir, ".quorum"))

	// Should now fail
	err = registry.ValidateProject(ctx, added.ID)
	if err == nil {
		t.Error("expected validation to fail after removing .quorum")
	}

	// Check status was updated
	project, _ := registry.GetProject(ctx, added.ID)
	if project.Status != StatusOffline {
		t.Errorf("expected status %s, got %s", StatusOffline, project.Status)
	}
}

func TestValidateProjectDegraded(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	// Remove only the config file (keep .quorum directory)
	os.Remove(filepath.Join(projectDir, ".quorum", "config.yaml"))

	// Should succeed but mark as degraded
	err := registry.ValidateProject(ctx, added.ID)
	if err != nil {
		t.Errorf("ValidateProject should succeed for degraded: %v", err)
	}

	project, _ := registry.GetProject(ctx, added.ID)
	if project.Status != StatusDegraded {
		t.Errorf("expected status %s, got %s", StatusDegraded, project.Status)
	}
}

func TestDefaultProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// No default when empty
	_, err := registry.GetDefaultProject(ctx)
	if err != ErrNoDefaultProject {
		t.Errorf("expected ErrNoDefaultProject, got %v", err)
	}

	// Add first project - should become default
	projectDir1 := createTestProject(t, tmpDir, "proj1")
	proj1, _ := registry.AddProject(ctx, projectDir1, nil)

	defaultProj, err := registry.GetDefaultProject(ctx)
	if err != nil {
		t.Fatalf("GetDefaultProject failed: %v", err)
	}
	if defaultProj.ID != proj1.ID {
		t.Errorf("expected default to be %s, got %s", proj1.ID, defaultProj.ID)
	}

	// Add second project
	projectDir2 := createTestProject(t, tmpDir, "proj2")
	proj2, _ := registry.AddProject(ctx, projectDir2, nil)

	// Set second as default
	err = registry.SetDefaultProject(ctx, proj2.ID)
	if err != nil {
		t.Fatalf("SetDefaultProject failed: %v", err)
	}

	defaultProj, _ = registry.GetDefaultProject(ctx)
	if defaultProj.ID != proj2.ID {
		t.Errorf("expected default to be %s, got %s", proj2.ID, defaultProj.ID)
	}
}

func TestSetDefaultProjectNotFound(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.SetDefaultProject(context.Background(), "nonexistent")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestTouchProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)
	originalTime := added.LastAccessed

	// Touch the project
	err := registry.TouchProject(ctx, added.ID)
	if err != nil {
		t.Fatalf("TouchProject failed: %v", err)
	}

	updated, _ := registry.GetProject(ctx, added.ID)
	if !updated.LastAccessed.After(originalTime) && !updated.LastAccessed.Equal(originalTime) {
		t.Error("expected LastAccessed to be updated or equal")
	}
}

func TestTouchProjectNotFound(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.TouchProject(context.Background(), "nonexistent")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestUpdateProject(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	added, _ := registry.AddProject(ctx, projectDir, nil)

	// Update the project
	added.Name = "Updated Name"
	added.Color = "#00FF00"

	err := registry.UpdateProject(ctx, added)
	if err != nil {
		t.Fatalf("UpdateProject failed: %v", err)
	}

	updated, _ := registry.GetProject(ctx, added.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", updated.Name)
	}
	if updated.Color != "#00FF00" {
		t.Errorf("expected color '#00FF00', got '%s'", updated.Color)
	}
}

func TestUpdateProjectNotFound(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	project := &Project{ID: "nonexistent", Name: "Test"}
	err := registry.UpdateProject(context.Background(), project)
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

func TestListProjects(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Add multiple projects
	projectDir1 := createTestProject(t, tmpDir, "proj1")
	projectDir2 := createTestProject(t, tmpDir, "proj2")
	projectDir3 := createTestProject(t, tmpDir, "proj3")

	registry.AddProject(ctx, projectDir1, nil)
	registry.AddProject(ctx, projectDir2, nil)
	registry.AddProject(ctx, projectDir3, nil)

	projects, err := registry.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}

	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}
}

func TestPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "quorum-registry-persist-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "projects.yaml")
	projectDir := createTestProject(t, tmpDir, "test-project")
	ctx := context.Background()

	// Create registry and add project
	registry1, _ := NewFileRegistry(WithConfigPath(configPath))
	added, _ := registry1.AddProject(ctx, projectDir, &AddProjectOptions{Name: "Persisted"})
	registry1.Close()

	// Create new registry instance
	registry2, _ := NewFileRegistry(WithConfigPath(configPath))
	defer registry2.Close()

	projects, _ := registry2.ListProjects(ctx)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].ID != added.ID {
		t.Errorf("expected ID %s, got %s", added.ID, projects[0].ID)
	}
	if projects[0].Name != "Persisted" {
		t.Errorf("expected name 'Persisted', got '%s'", projects[0].Name)
	}
}

func TestRegistryClose(t *testing.T) {
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations should fail after close
	_, err = registry.ListProjects(context.Background())
	if err != ErrRegistryClosed {
		t.Errorf("expected ErrRegistryClosed, got %v", err)
	}
}

func TestCount(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	if registry.Count() != 0 {
		t.Errorf("expected count 0, got %d", registry.Count())
	}

	projectDir := createTestProject(t, tmpDir, "test-project")
	registry.AddProject(ctx, projectDir, nil)

	if registry.Count() != 1 {
		t.Errorf("expected count 1, got %d", registry.Count())
	}
}

func TestProjectStatusMethods(t *testing.T) {
	tests := []struct {
		status   ProjectStatus
		valid    bool
		strValue string
	}{
		{StatusHealthy, true, "healthy"},
		{StatusDegraded, true, "degraded"},
		{StatusOffline, true, "offline"},
		{StatusInitializing, true, "initializing"},
		{ProjectStatus("invalid"), false, "invalid"},
	}

	for _, tt := range tests {
		if tt.status.IsValid() != tt.valid {
			t.Errorf("status %s IsValid() = %v, want %v", tt.status, tt.status.IsValid(), tt.valid)
		}
		if tt.status.String() != tt.strValue {
			t.Errorf("status %s String() = %s, want %s", tt.status, tt.status.String(), tt.strValue)
		}
	}
}

func TestProjectClone(t *testing.T) {
	original := &Project{
		ID:            "test-id",
		Path:          "/test/path",
		Name:          "Test",
		Status:        StatusHealthy,
		StatusMessage: "OK",
		Color:         "#FF0000",
	}

	clone := original.Clone()

	if clone == original {
		t.Error("clone should be a different instance")
	}
	if clone.ID != original.ID {
		t.Errorf("ID mismatch: %s != %s", clone.ID, original.ID)
	}
	if clone.Path != original.Path {
		t.Errorf("Path mismatch: %s != %s", clone.Path, original.Path)
	}

	// Verify nil handling
	var nilProject *Project
	if nilProject.Clone() != nil {
		t.Error("clone of nil should be nil")
	}
}

func TestProjectIsHealthy(t *testing.T) {
	healthy := &Project{Status: StatusHealthy}
	degraded := &Project{Status: StatusDegraded}
	offline := &Project{Status: StatusOffline}

	if !healthy.IsHealthy() {
		t.Error("expected healthy project to be healthy")
	}
	if degraded.IsHealthy() {
		t.Error("expected degraded project to not be healthy")
	}
	if offline.IsHealthy() {
		t.Error("expected offline project to not be healthy")
	}
}

func TestProjectIsAccessible(t *testing.T) {
	healthy := &Project{Status: StatusHealthy}
	degraded := &Project{Status: StatusDegraded}
	offline := &Project{Status: StatusOffline}

	if !healthy.IsAccessible() {
		t.Error("expected healthy project to be accessible")
	}
	if !degraded.IsAccessible() {
		t.Error("expected degraded project to be accessible")
	}
	if offline.IsAccessible() {
		t.Error("expected offline project to not be accessible")
	}
}

func TestGenerateProjectID(t *testing.T) {
	id1 := generateProjectID()
	id2 := generateProjectID()

	if id1 == id2 {
		t.Error("expected different IDs")
	}
	if len(id1) < 10 {
		t.Errorf("expected ID length >= 10, got %d", len(id1))
	}
	if id1[:5] != "proj-" {
		t.Errorf("expected ID to start with 'proj-', got %s", id1[:5])
	}
}

func TestGenerateProjectName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/user/my-project", "My project"},
		{"/home/user/my_project", "My project"},
		{"/home/user/MyProject", "MyProject"},
		{"/home/user/project", "Project"},
	}

	for _, tt := range tests {
		got := generateProjectName(tt.path)
		if got != tt.expected {
			t.Errorf("generateProjectName(%s) = %s, want %s", tt.path, got, tt.expected)
		}
	}
}

func TestGenerateProjectColor(t *testing.T) {
	color1 := generateProjectColor("proj-abc123")
	color2 := generateProjectColor("proj-abc123")
	color3 := generateProjectColor("proj-def456")

	if color1 != color2 {
		t.Error("same ID should produce same color")
	}
	if color1[0] != '#' {
		t.Errorf("expected color to start with '#', got %s", color1)
	}
	// Different IDs might produce same color (pigeonhole principle), so we don't test that
	_ = color3
}

func TestValidateAll(t *testing.T) {
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()

	// Add multiple projects
	projectDir1 := createTestProject(t, tmpDir, "proj1")
	projectDir2 := createTestProject(t, tmpDir, "proj2")

	registry.AddProject(ctx, projectDir1, nil)
	registry.AddProject(ctx, projectDir2, nil)

	// Should succeed initially
	err := registry.ValidateAll(ctx)
	if err != nil {
		t.Errorf("ValidateAll should succeed: %v", err)
	}

	// Remove one project directory
	os.RemoveAll(projectDir1)

	// Should fail now (but continue validating others)
	err = registry.ValidateAll(ctx)
	if err == nil {
		t.Error("expected ValidateAll to return error after removing directory")
	}
}
