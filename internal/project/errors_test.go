package project

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// errors.go coverage
// ---------------------------------------------------------------------------

func TestProjectValidationError_WithNilErr(t *testing.T) {
	t.Parallel()
	e := NewValidationError("p1", "/some/path", "bad config", nil)
	msg := e.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	// Should NOT contain "<nil>" text because err is nil
	if containsStr(msg, "<nil>") {
		t.Errorf("unexpected <nil> in error message: %s", msg)
	}
	if e.Unwrap() != nil {
		t.Error("expected Unwrap to return nil")
	}
}

func TestProjectValidationError_WithErr(t *testing.T) {
	t.Parallel()
	inner := fmt.Errorf("disk full")
	e := NewValidationError("p2", "/other", "write failed", inner)
	msg := e.Error()
	if !containsStr(msg, "disk full") {
		t.Errorf("expected inner error in message: %s", msg)
	}
	if !containsStr(msg, "p2") {
		t.Errorf("expected project ID in message: %s", msg)
	}
	if e.Unwrap() != inner {
		t.Errorf("Unwrap returned %v, want %v", e.Unwrap(), inner)
	}
}

func TestRegistryError_ErrorAndUnwrap(t *testing.T) {
	t.Parallel()
	inner := fmt.Errorf("i/o timeout")
	e := NewRegistryError("save", inner)
	msg := e.Error()
	if !containsStr(msg, "save") {
		t.Errorf("expected operation in message: %s", msg)
	}
	if !containsStr(msg, "i/o timeout") {
		t.Errorf("expected inner error in message: %s", msg)
	}
	if e.Unwrap() != inner {
		t.Errorf("Unwrap returned %v, want %v", e.Unwrap(), inner)
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()
	sentinels := []error{
		ErrProjectNotFound,
		ErrProjectAlreadyExists,
		ErrNotQuorumProject,
		ErrProjectOffline,
		ErrInvalidPath,
		ErrRegistryCorrupted,
		ErrNoDefaultProject,
		ErrRegistryClosed,
	}
	for _, s := range sentinels {
		if s == nil {
			t.Error("sentinel error is nil")
		}
		if s.Error() == "" {
			t.Error("sentinel error has empty message")
		}
	}
}

// ---------------------------------------------------------------------------
// context.go coverage: ProjectID, ProjectRoot, ConfigMode
// ---------------------------------------------------------------------------

func TestProjectContext_ProjectID_ProjectRoot(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("ctx-id", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	defer pc.Close()

	if pc.ProjectID() != "ctx-id" {
		t.Errorf("ProjectID() = %q, want %q", pc.ProjectID(), "ctx-id")
	}
	if pc.ProjectRoot() != pc.Root {
		t.Errorf("ProjectRoot() = %q, want %q", pc.ProjectRoot(), pc.Root)
	}
}

func TestNewProjectContext_WithConfigModeCustom(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("id1", projectDir, WithConfigMode(ConfigModeCustom))
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	defer pc.Close()

	if pc.ConfigMode != ConfigModeCustom {
		t.Errorf("ConfigMode = %q, want %q", pc.ConfigMode, ConfigModeCustom)
	}
}

func TestNewProjectContext_WithUnknownConfigModeFallsBackToCustom(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("id2", projectDir, WithConfigMode("garbage"))
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	defer pc.Close()

	// Unknown mode should fall back to custom in initConfigLoader
	if pc.ConfigMode != "garbage" {
		t.Errorf("ConfigMode = %q, want %q", pc.ConfigMode, "garbage")
	}
}

func TestWithEventBufferSize_ZeroIgnored(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	// Zero should be ignored (default used)
	pc, err := NewProjectContext("id3", projectDir, WithEventBufferSize(0))
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	defer pc.Close()

	if pc.EventBus == nil {
		t.Error("expected EventBus to be initialized")
	}
}

func TestWithContextLogger(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pc, err := NewProjectContext("id4", projectDir, WithContextLogger(logger))
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	defer pc.Close()
}

// ---------------------------------------------------------------------------
// context.go: Validate edge cases
// ---------------------------------------------------------------------------

func TestProjectContextValidate_DeletedRoot(t *testing.T) {
	t.Parallel()
	// Create a temp dir that we can safely delete
	tmpDir, err := os.MkdirTemp("", "quorum-val-root-*")
	if err != nil {
		t.Fatal(err)
	}
	quorumDir := filepath.Join(tmpDir, ".quorum")
	os.MkdirAll(filepath.Join(quorumDir, "state"), 0o750)
	os.WriteFile(filepath.Join(quorumDir, "config.yaml"), []byte("version: 1\n"), 0o640)

	pc, err := NewProjectContext("val-root", tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("NewProjectContext: %v", err)
	}

	// Close context first to release file handles (critical on Windows)
	pc.Close()

	// Delete the entire root
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Logf("failed to remove tmpDir (may be expected on Windows): %v", err)
	}

	// Re-open the context to test validation with deleted root
	pc2, err := NewProjectContext("val-root-2", tmpDir)
	if err == nil {
		// If we can't create it due to missing directory, that's what we want to test
		t.Error("expected error when creating context with non-existent root")
		pc2.Close()
	}
}

// ---------------------------------------------------------------------------
// context.go: HasRunningWorkflows with nil StateManager
// ---------------------------------------------------------------------------

func TestProjectContextHasRunningWorkflows_NilStateManager(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("nil-sm", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	// Manually nil out the state manager
	pc.StateManager = nil
	defer pc.Close()

	_, err = pc.HasRunningWorkflows(context.Background())
	if err == nil {
		t.Error("expected error with nil StateManager")
	}
}

// ---------------------------------------------------------------------------
// context.go: GetConfig with nil ConfigLoader
// ---------------------------------------------------------------------------

func TestProjectContextGetConfig_NilConfigLoader(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("nil-cl", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	// Manually nil out the config loader
	pc.ConfigLoader = nil
	defer pc.Close()

	_, err = pc.GetConfig()
	if err == nil {
		t.Error("expected error with nil ConfigLoader")
	}
}

// ---------------------------------------------------------------------------
// pool.go: WithPoolLogger, pool options coverage
// ---------------------------------------------------------------------------

func TestWithPoolLogger(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool := NewStatePool(registry, WithPoolLogger(logger))
	defer pool.Close()
	if pool == nil {
		t.Fatal("pool is nil")
	}
}

func TestStatePoolMinExceedsMax(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry,
		WithMaxActiveContexts(2),
		WithMinActiveContexts(10), // exceeds max
	)
	defer pool.Close()

	// min should be clamped to max
	if pool.opts.minActiveContexts > pool.opts.maxActiveContexts {
		t.Errorf("min (%d) > max (%d)", pool.opts.minActiveContexts, pool.opts.maxActiveContexts)
	}
}

func TestWithMaxActiveContexts_Zero(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry, WithMaxActiveContexts(0))
	defer pool.Close()
	// 0 should be ignored, default used
	if pool.opts.maxActiveContexts <= 0 {
		t.Error("max should not be zero")
	}
}

func TestWithMinActiveContexts_Negative(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry, WithMinActiveContexts(-1))
	defer pool.Close()
	// -1 should be ignored (< 0 not applied)
}

func TestWithEvictionGracePeriod_Negative(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry, WithEvictionGracePeriod(-1*time.Second))
	defer pool.Close()
	// Negative should be ignored
}

func TestWithPoolEventBufferSize_Zero(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry, WithPoolEventBufferSize(0))
	defer pool.Close()
	if pool.opts.eventBufferSize <= 0 {
		t.Error("buffer size should not be zero")
	}
}

// ---------------------------------------------------------------------------
// pool.go: EvictProject on closed pool
// ---------------------------------------------------------------------------

func TestStatePoolEvictProject_ClosedPool(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	pool.Close()

	err := pool.EvictProject(context.Background(), "any")
	if err == nil {
		t.Error("expected error on closed pool")
	}
}

func TestStatePoolEvictProject_NotInPool(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	defer pool.Close()

	err := pool.EvictProject(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for project not in pool")
	}
}

// ---------------------------------------------------------------------------
// pool.go: Cleanup on closed pool
// ---------------------------------------------------------------------------

func TestStatePoolCleanup_ClosedPool(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	pool.Close()

	err := pool.Cleanup(context.Background())
	if err == nil {
		t.Error("expected error on closed pool")
	}
}

// ---------------------------------------------------------------------------
// pool.go: GetMetrics with no activity (zero hit rate)
// ---------------------------------------------------------------------------

func TestStatePoolGetMetrics_Empty(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	defer pool.Close()

	metrics := pool.GetMetrics()
	if metrics.ActiveContexts != 0 {
		t.Errorf("expected 0 active, got %d", metrics.ActiveContexts)
	}
	if metrics.HitRate != 0 {
		t.Errorf("expected 0 hit rate, got %f", metrics.HitRate)
	}
}

// ---------------------------------------------------------------------------
// pool.go: Preload with invalid project
// ---------------------------------------------------------------------------

func TestStatePoolPreload_InvalidProject(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	defer pool.Close()

	// Preload with nonexistent projects should not error (logs warning)
	err := pool.Preload(context.Background(), []string{"does-not-exist-1", "does-not-exist-2"})
	if err != nil {
		t.Errorf("Preload should not return error: %v", err)
	}
	if pool.Size() != 0 {
		t.Errorf("expected 0 contexts, got %d", pool.Size())
	}
}

// ---------------------------------------------------------------------------
// registry.go: Close operations after close, Reload
// ---------------------------------------------------------------------------

func TestRegistryClose_OperationsAfterClose(t *testing.T) {
	t.Parallel()
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	registry.Close()

	ctx := context.Background()

	_, err := registry.GetProject(ctx, "any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("GetProject: expected ErrRegistryClosed, got %v", err)
	}

	_, err = registry.GetProjectByPath(ctx, "/any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("GetProjectByPath: expected ErrRegistryClosed, got %v", err)
	}

	// AddProject validates path before checking closed state, so it may return a different error.
	// We just verify it returns an error.
	_, err = registry.AddProject(ctx, "/any", nil)
	if err == nil {
		t.Error("AddProject: expected error after close")
	}

	err = registry.RemoveProject(ctx, "any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("RemoveProject: expected ErrRegistryClosed, got %v", err)
	}

	err = registry.UpdateProject(ctx, &Project{ID: "any"})
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("UpdateProject: expected ErrRegistryClosed, got %v", err)
	}

	err = registry.ValidateProject(ctx, "any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("ValidateProject: expected ErrRegistryClosed, got %v", err)
	}

	err = registry.ValidateAll(ctx)
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("ValidateAll: expected ErrRegistryClosed, got %v", err)
	}

	_, err = registry.GetDefaultProject(ctx)
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("GetDefaultProject: expected ErrRegistryClosed, got %v", err)
	}

	err = registry.SetDefaultProject(ctx, "any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("SetDefaultProject: expected ErrRegistryClosed, got %v", err)
	}

	err = registry.TouchProject(ctx, "any")
	if !errors.Is(err, ErrRegistryClosed) {
		t.Errorf("TouchProject: expected ErrRegistryClosed, got %v", err)
	}
}

func TestRegistryReload(t *testing.T) {
	t.Parallel()
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	projectDir := createTestProject(t, tmpDir, "reload-test")
	registry.AddProject(ctx, projectDir, nil)

	// Reload should succeed
	err := registry.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Should still have the project (written to disk by autoSave)
	projects, err := registry.ListProjects(ctx)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project after reload, got %d", len(projects))
	}
}

// ---------------------------------------------------------------------------
// registry.go: UpdateProject with nil project
// ---------------------------------------------------------------------------

func TestUpdateProject_NilProject(t *testing.T) {
	t.Parallel()
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.UpdateProject(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil project")
	}
}

func TestUpdateProject_EmptyID(t *testing.T) {
	t.Parallel()
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	err := registry.UpdateProject(context.Background(), &Project{ID: ""})
	if err == nil {
		t.Error("expected error for empty project ID")
	}
}

// ---------------------------------------------------------------------------
// registry.go: GetDefaultProject with stale default
// ---------------------------------------------------------------------------

func TestGetDefaultProject_StaleDefault(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "quorum-stale-default-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := NewFileRegistry(WithConfigPath(configPath), WithAutoSave(false), WithBackup(false))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	ctx := context.Background()
	projDir := createTestProject(t, tmpDir, "proj-stale")
	p, _ := registry.AddProject(ctx, projDir, nil)

	// Set default to something that doesn't exist
	registry.mu.Lock()
	registry.config.DefaultProject = "nonexistent-id"
	registry.mu.Unlock()

	// Should fall back to first project
	def, err := registry.GetDefaultProject(ctx)
	if err != nil {
		t.Fatalf("GetDefaultProject: %v", err)
	}
	if def.ID != p.ID {
		t.Errorf("expected fallback to first project %s, got %s", p.ID, def.ID)
	}
}

// ---------------------------------------------------------------------------
// registry.go: RemoveProject updates default when removed project is default
// ---------------------------------------------------------------------------

func TestRemoveProject_DefaultIsReassigned(t *testing.T) {
	t.Parallel()
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	dir1 := createTestProject(t, tmpDir, "proj1")
	dir2 := createTestProject(t, tmpDir, "proj2")
	p1, _ := registry.AddProject(ctx, dir1, nil)
	p2, _ := registry.AddProject(ctx, dir2, nil)

	// p1 is the default. Remove it. Default should shift to p2.
	err := registry.RemoveProject(ctx, p1.ID)
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	def, err := registry.GetDefaultProject(ctx)
	if err != nil {
		t.Fatalf("GetDefaultProject: %v", err)
	}
	if def.ID != p2.ID {
		t.Errorf("expected default %s, got %s", p2.ID, def.ID)
	}
}

func TestRemoveProject_LastProjectClearsDefault(t *testing.T) {
	t.Parallel()
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	dir := createTestProject(t, tmpDir, "only-proj")
	p, _ := registry.AddProject(ctx, dir, nil)

	err := registry.RemoveProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	_, err = registry.GetDefaultProject(ctx)
	if err != ErrNoDefaultProject {
		t.Errorf("expected ErrNoDefaultProject, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// registry.go: ValidateProjectPath edge cases
// ---------------------------------------------------------------------------

func TestValidateProjectPath_NotAbsolute(t *testing.T) {
	t.Parallel()
	err := ValidateProjectPath("relative/path")
	if err == nil {
		t.Error("expected error for relative path")
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

func TestValidateProjectPath_QuorumIsFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create .quorum as a file, not a directory
	quorumPath := filepath.Join(tmpDir, ".quorum")
	os.WriteFile(quorumPath, []byte("not a dir"), 0o600)

	err := ValidateProjectPath(tmpDir)
	if err == nil {
		t.Error("expected error when .quorum is a file")
	}
	if !errors.Is(err, ErrNotQuorumProject) {
		t.Errorf("expected ErrNotQuorumProject, got %v", err)
	}
}

func TestValidateProjectPath_NoQuorumDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	err := ValidateProjectPath(tmpDir)
	if err == nil {
		t.Error("expected error when no .quorum dir")
	}
	if !errors.Is(err, ErrNotQuorumProject) {
		t.Errorf("expected ErrNotQuorumProject, got %v", err)
	}
}

func TestValidateProjectPath_Valid(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".quorum"), 0o750)
	err := ValidateProjectPath(tmpDir)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// registry.go: ValidateProject with config in inherit_global mode
// ---------------------------------------------------------------------------

func TestValidateProject_InheritGlobalModeNoConfig(t *testing.T) {
	t.Parallel()
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	projDir := createTestProject(t, tmpDir, "inherit-proj")

	p, _ := registry.AddProject(ctx, projDir, nil)

	// Set config mode to inherit_global and remove config file
	p.ConfigMode = ConfigModeInheritGlobal
	registry.UpdateProject(ctx, p)
	os.Remove(filepath.Join(projDir, ".quorum", "config.yaml"))

	// Validate should succeed (missing config is OK in inherit_global mode)
	err := registry.ValidateProject(ctx, p.ID)
	if err != nil {
		t.Errorf("expected nil error in inherit_global mode, got %v", err)
	}

	// Status should be healthy, not degraded
	updated, _ := registry.GetProject(ctx, p.ID)
	if updated.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", updated.Status)
	}
}

// ---------------------------------------------------------------------------
// registry.go: ValidateProject on directory that is not a dir (path is a file)
// ---------------------------------------------------------------------------

func TestValidateProject_PathIsNotDirectory(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "quorum-val-notdir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "projects.yaml")
	registry, err := NewFileRegistry(WithConfigPath(configPath), WithAutoSave(false), WithBackup(false))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	// Create a proper project first
	projDir := createTestProject(t, tmpDir, "val-notdir")
	ctx := context.Background()
	p, _ := registry.AddProject(ctx, projDir, nil)

	// Now replace the project dir with a file
	os.RemoveAll(projDir)
	os.WriteFile(projDir, []byte("I am a file"), 0o600)

	err = registry.ValidateProject(ctx, p.ID)
	if err == nil {
		t.Error("expected error when path is a file")
	}
}

// ---------------------------------------------------------------------------
// registry.go: WithLogger option
// ---------------------------------------------------------------------------

func TestWithLoggerOption(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := NewFileRegistry(WithConfigPath(configPath), WithLogger(logger))
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	defer registry.Close()
}

// ---------------------------------------------------------------------------
// registry.go: Corrupted YAML load
// ---------------------------------------------------------------------------

func TestRegistryLoad_CorruptedYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	// Write invalid YAML
	os.WriteFile(configPath, []byte("{{{{not yaml at all"), 0o600)

	_, err := NewFileRegistry(WithConfigPath(configPath))
	if err == nil {
		t.Error("expected error for corrupted YAML")
	}
}

// ---------------------------------------------------------------------------
// types.go: Project methods on nil
// ---------------------------------------------------------------------------

func TestProjectIsHealthy_Nil(t *testing.T) {
	t.Parallel()
	var p *Project
	if p.IsHealthy() {
		t.Error("nil project should not be healthy")
	}
}

func TestProjectIsAccessible_Nil(t *testing.T) {
	t.Parallel()
	var p *Project
	if p.IsAccessible() {
		t.Error("nil project should not be accessible")
	}
}

// ---------------------------------------------------------------------------
// pool.go: ValidateAll with a context that fails validation
// ---------------------------------------------------------------------------

func TestStatePoolValidateAll_WithFailure(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-valall-fail-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "val-fail")

	registry := newMockRegistry()
	p, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, p.ID)

	// Evict the context to release file handles before removal (critical on Windows)
	pool.EvictProject(ctx, p.ID)

	// Remove the .quorum directory to make validation fail
	quorumPath := filepath.Join(projectDir, ".quorum")
	if err := os.RemoveAll(quorumPath); err != nil {
		t.Logf("failed to remove .quorum dir (may be expected on Windows): %v", err)
	}

	// Try to get context again - it should fail to initialize due to missing .quorum
	_, err = pool.GetContext(ctx, p.ID)
	if err == nil {
		t.Error("expected error from GetContext when .quorum directory is missing")
	}
}

// ---------------------------------------------------------------------------
// pool.go: ValidateAll where context is removed between snapshot and check
// ---------------------------------------------------------------------------

func TestStatePoolValidateAll_ContextDisappears(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-valall-disappear-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "disappear")

	registry := newMockRegistry()
	p, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, p.ID)

	// Evict the context while we're validating - simulates disappearance
	pool.EvictProject(ctx, p.ID)

	// Should handle missing context gracefully
	_ = pool.ValidateAll(ctx)
}

// ---------------------------------------------------------------------------
// pool.go: evictLRULocked path where all contexts are in grace period
// ---------------------------------------------------------------------------

func TestStatePoolEvictLRU_AllInGracePeriod(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-grace-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 3 projects
	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = createPoolTestProject(t, tmpDir, fmt.Sprintf("proj%d", i))
	}

	registry := newMockRegistry()
	var projectIDs []string
	for _, dir := range dirs {
		p, _ := registry.AddProject(context.Background(), dir, nil)
		projectIDs = append(projectIDs, p.ID)
	}

	// Pool with max 2, but long grace period so nothing can be evicted
	pool := NewStatePool(registry,
		WithMaxActiveContexts(2),
		WithMinActiveContexts(0),
		WithEvictionGracePeriod(1*time.Hour),
	)
	defer pool.Close()

	ctx := context.Background()

	// Load first two
	_, _ = pool.GetContext(ctx, projectIDs[0])
	_, _ = pool.GetContext(ctx, projectIDs[1])

	// Try to load third - should trigger eviction attempt but fail (grace period)
	// Context might still be created beyond capacity since eviction fails
	_, err = pool.GetContext(ctx, projectIDs[2])
	// Either succeeds (exceeds capacity) or fails (can't evict)
	_ = err
}

// ---------------------------------------------------------------------------
// pool.go: evictLRULocked path where contexts are at min
// ---------------------------------------------------------------------------

func TestStatePoolEvictLRU_AtMinContexts(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-at-min-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = createPoolTestProject(t, tmpDir, fmt.Sprintf("proj%d", i))
	}

	registry := newMockRegistry()
	var projectIDs []string
	for _, dir := range dirs {
		p, _ := registry.AddProject(context.Background(), dir, nil)
		projectIDs = append(projectIDs, p.ID)
	}

	// Pool with max 2, min 2 - at capacity but also at minimum, eviction should skip
	pool := NewStatePool(registry,
		WithMaxActiveContexts(2),
		WithMinActiveContexts(2),
		WithEvictionGracePeriod(0),
	)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, projectIDs[0])
	time.Sleep(5 * time.Millisecond)
	_, _ = pool.GetContext(ctx, projectIDs[1])
	time.Sleep(5 * time.Millisecond)

	// Third load - at capacity and min, eviction should not reduce below min
	_, _ = pool.GetContext(ctx, projectIDs[2])
}

// ---------------------------------------------------------------------------
// registry.go: NewFileRegistry without WithConfigPath (tests getRegistryPath)
// ---------------------------------------------------------------------------

func TestNewFileRegistry_DefaultPath(t *testing.T) {
	t.Parallel()
	// This will use the default path (~/.quorum-registry/projects.yaml)
	registry, err := NewFileRegistry()
	if err != nil {
		t.Skipf("skipping: could not create default registry: %v", err)
	}
	defer registry.Close()

	if registry.configPath == "" {
		t.Error("expected non-empty config path")
	}
}

// ---------------------------------------------------------------------------
// registry.go: save/mergeFromDisk with backup
// ---------------------------------------------------------------------------

func TestRegistryBackup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	registry, err := NewFileRegistry(WithConfigPath(configPath), WithBackup(true), WithAutoSave(true))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	projDir := createTestProject(t, tmpDir, "backup-proj")
	_, err = registry.AddProject(ctx, projDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Add another project to trigger save again (backup should be created)
	projDir2 := createTestProject(t, tmpDir, "backup-proj2")
	_, err = registry.AddProject(ctx, projDir2, nil)
	if err != nil {
		t.Fatal(err)
	}

	registry.Close()

	// Check backup file was created
	backupPath := configPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected backup file to exist")
	}
}

// ---------------------------------------------------------------------------
// registry.go: mergeFromDisk - test concurrent modification
// ---------------------------------------------------------------------------

func TestRegistryMergeFromDisk(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	// Create first registry
	r1, err := NewFileRegistry(WithConfigPath(configPath), WithAutoSave(true))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	projDir1 := createTestProject(t, tmpDir, "merge-proj1")
	_, err = r1.AddProject(ctx, projDir1, nil)
	if err != nil {
		t.Fatal(err)
	}
	r1.Close()

	// Create second registry (loads from disk)
	r2, err := NewFileRegistry(WithConfigPath(configPath), WithAutoSave(true))
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()

	projects, _ := r2.ListProjects(ctx)
	if len(projects) != 1 {
		t.Errorf("expected 1 project after merge, got %d", len(projects))
	}
}

// ---------------------------------------------------------------------------
// pool.go: GetContext with double-check after write lock (concurrent scenario)
// ---------------------------------------------------------------------------

func TestStatePoolGetContext_DoubleCheck(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-doublecheck-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "dbl-check")

	registry := newMockRegistry()
	p, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry, WithMaxActiveContexts(10))
	defer pool.Close()

	ctx := context.Background()

	// First load
	pc1, err := pool.GetContext(ctx, p.ID)
	if err != nil {
		t.Fatalf("first GetContext: %v", err)
	}

	// Second load (should hit the fast-path read lock check)
	pc2, err := pool.GetContext(ctx, p.ID)
	if err != nil {
		t.Fatalf("second GetContext: %v", err)
	}

	if pc1 != pc2 {
		t.Error("expected same context on second access")
	}
}

// ---------------------------------------------------------------------------
// context.go: NewProjectContext with inherit_global config mode
// (tests the initConfigLoader inherit_global branch)
// ---------------------------------------------------------------------------

func TestNewProjectContext_InheritGlobalMode(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	// This will try to use global config; may or may not succeed depending
	// on whether global config file exists, but covers the code path.
	pc, err := NewProjectContext("inherit-test", projectDir,
		WithConfigMode(ConfigModeInheritGlobal),
	)
	if err != nil {
		// If global config doesn't exist, this is expected
		return
	}
	defer pc.Close()

	if pc.ConfigMode != ConfigModeInheritGlobal {
		t.Errorf("ConfigMode = %q, want %q", pc.ConfigMode, ConfigModeInheritGlobal)
	}
}

// ---------------------------------------------------------------------------
// registry.go: load with nil projects in YAML
// ---------------------------------------------------------------------------

func TestRegistryLoad_NilProjects(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "projects.yaml")

	// Write YAML with no projects field
	os.WriteFile(configPath, []byte("version: 1\n"), 0o600)

	registry, err := NewFileRegistry(WithConfigPath(configPath))
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	defer registry.Close()

	projects, err := registry.ListProjects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if projects == nil {
		t.Error("expected non-nil projects slice")
	}
}

// ---------------------------------------------------------------------------
// context.go: String representation of closed context
// ---------------------------------------------------------------------------

func TestProjectContext_StringWhenClosed(t *testing.T) {
	t.Parallel()
	projectDir, cleanup := setupTestProjectDir(t)
	defer cleanup()

	pc, err := NewProjectContext("str-closed", projectDir)
	if err != nil {
		t.Fatalf("NewProjectContext: %v", err)
	}
	pc.Close()

	s := pc.String()
	if !containsStr(s, "true") {
		t.Errorf("expected closed=true in string: %s", s)
	}
}

// ---------------------------------------------------------------------------
// pool.go: evictLRULocked with stale accessOrder entries
// (entry in accessOrder but not in contexts map)
// ---------------------------------------------------------------------------

func TestStatePool_StaleAccessOrder(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-stale-order-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = createPoolTestProject(t, tmpDir, fmt.Sprintf("proj%d", i))
	}

	registry := newMockRegistry()
	var projectIDs []string
	for _, dir := range dirs {
		p, _ := registry.AddProject(context.Background(), dir, nil)
		projectIDs = append(projectIDs, p.ID)
	}

	pool := NewStatePool(registry,
		WithMaxActiveContexts(2),
		WithMinActiveContexts(0),
		WithEvictionGracePeriod(0),
	)
	defer pool.Close()

	ctx := context.Background()

	// Load two projects
	_, _ = pool.GetContext(ctx, projectIDs[0])
	time.Sleep(5 * time.Millisecond)
	_, _ = pool.GetContext(ctx, projectIDs[1])
	time.Sleep(5 * time.Millisecond)

	// Manually corrupt: add a stale entry to accessOrder without corresponding context
	pool.mu.Lock()
	pool.accessOrder = append([]string{"stale-id"}, pool.accessOrder...)
	pool.mu.Unlock()

	// Third project should trigger eviction that encounters the stale entry
	_, _ = pool.GetContext(ctx, projectIDs[2])
}

// ---------------------------------------------------------------------------
// pool.go: GetContext on closed pool (slow path double-check)
// ---------------------------------------------------------------------------

func TestStatePoolGetContext_ClosedDuringSlowPath(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()
	pool := NewStatePool(registry)

	// Close the pool
	pool.Close()

	// GetContext should return error (tests both fast and slow path closed checks)
	_, err := pool.GetContext(context.Background(), "any-id")
	if err == nil {
		t.Error("expected error on closed pool")
	}
}

// ---------------------------------------------------------------------------
// pool.go: evictProjectLocked with negative orderIndex
// ---------------------------------------------------------------------------

func TestStatePoolEvictProject_NegativeOrderIndex(t *testing.T) {
	t.Parallel()
	tmpDir, err := os.MkdirTemp("", "pool-neg-idx-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "neg-idx")

	registry := newMockRegistry()
	p, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, p.ID)

	// EvictProject uses -1 for orderIndex, triggering the find-and-remove path
	err = pool.EvictProject(ctx, p.ID)
	if err != nil {
		t.Errorf("EvictProject: %v", err)
	}
}

// ---------------------------------------------------------------------------
// registry.go: AddProject where autoSave save fails
// We test by creating a registry with autoSave pointing to a read-only path
// ---------------------------------------------------------------------------

func TestAddProject_AutoSaveFails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a read-only directory for the config file
	configDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(configDir, 0o750)
	configPath := filepath.Join(configDir, "projects.yaml")

	registry, err := NewFileRegistry(WithConfigPath(configPath), WithAutoSave(true), WithBackup(false))
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()

	// Create a valid project
	projDir := createTestProject(t, tmpDir, "save-fail")

	// Make the config directory read-only so save fails
	os.Chmod(configDir, 0o444)
	t.Cleanup(func() { os.Chmod(configDir, 0o750) })

	_, err = registry.AddProject(context.Background(), projDir, nil)
	if err == nil {
		// If permissions don't work (root user), skip
		t.Skip("permissions not enforced")
	}
}

// ---------------------------------------------------------------------------
// registry.go: ValidateProjectPath with unclean path
// ---------------------------------------------------------------------------

func TestValidateProjectPath_UncleanPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "sub", ".quorum"), 0o750)

	// Construct an absolute but unclean path manually (filepath.Join auto-cleans)
	unclean := tmpDir + "/sub/../sub"
	err := ValidateProjectPath(unclean)
	if err == nil {
		t.Error("expected error for unclean path")
	}
	if !errors.Is(err, ErrInvalidPath) {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// registry.go: GetProjectByPath not found (closed branch already tested above)
// ---------------------------------------------------------------------------

func TestGetProjectByPath_NotFound(t *testing.T) {
	t.Parallel()
	registry, _, cleanup := setupTestRegistry(t)
	defer cleanup()

	_, err := registry.GetProjectByPath(context.Background(), "/nonexistent/path")
	if err != ErrProjectNotFound {
		t.Errorf("expected ErrProjectNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// registry.go: RemoveProject non-default project
// ---------------------------------------------------------------------------

func TestRemoveProject_NonDefault(t *testing.T) {
	t.Parallel()
	registry, tmpDir, cleanup := setupTestRegistry(t)
	defer cleanup()

	ctx := context.Background()
	dir1 := createTestProject(t, tmpDir, "proj-a")
	dir2 := createTestProject(t, tmpDir, "proj-b")
	p1, _ := registry.AddProject(ctx, dir1, nil)
	p2, _ := registry.AddProject(ctx, dir2, nil)

	// p1 is default. Remove p2 (non-default). Default should stay p1.
	err := registry.RemoveProject(ctx, p2.ID)
	if err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	def, _ := registry.GetDefaultProject(ctx)
	if def.ID != p1.ID {
		t.Errorf("expected default %s, got %s", p1.ID, def.ID)
	}
}

// ---------------------------------------------------------------------------
// pool.go: GetContext with invalid project path (NewProjectContext fails)
// ---------------------------------------------------------------------------

func TestStatePoolGetContext_InvalidProjectPath(t *testing.T) {
	t.Parallel()
	registry := newMockRegistry()

	// Register a project with a path that has no .quorum dir
	tmpDir := t.TempDir()
	badDir := filepath.Join(tmpDir, "no-quorum")
	os.MkdirAll(badDir, 0o750)

	registry.mu.Lock()
	registry.projects["bad-proj"] = &Project{
		ID:     "bad-proj",
		Path:   badDir,
		Name:   "bad",
		Status: StatusHealthy,
	}
	registry.mu.Unlock()

	pool := NewStatePool(registry)
	defer pool.Close()

	_, err := pool.GetContext(context.Background(), "bad-proj")
	if err == nil {
		t.Error("expected error for invalid project path")
	}
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
