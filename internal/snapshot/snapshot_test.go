package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
	"gopkg.in/yaml.v3"
)

func TestExportImportRoundTrip(t *testing.T) {
	sourceRoot := t.TempDir()
	registryPath := filepath.Join(sourceRoot, "registry", "projects.yaml")
	globalConfigPath := filepath.Join(sourceRoot, "registry", "global-config.yaml")
	snapshotPath := filepath.Join(sourceRoot, "snapshot.tar.gz")

	p1 := mustCreateProjectFixture(t, sourceRoot, "proj-1", "project-one", true, true)
	p2 := mustCreateProjectFixture(t, sourceRoot, "proj-2", "project-two", false, true)

	cfg := &project.RegistryConfig{
		Version:        1,
		DefaultProject: p1.ID,
		Projects:       []*project.Project{p1, p2},
	}
	mustWriteRegistryFixture(t, registryPath, cfg)
	mustWriteFile(t, globalConfigPath, []byte("global-config: true\n"), 0o600)

	exportResult, err := Export(&ExportOptions{
		OutputPath:       snapshotPath,
		IncludeWorktrees: false,
		RegistryPath:     registryPath,
		GlobalConfigPath: globalConfigPath,
		QuorumVersion:    "test-version",
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if exportResult.Manifest.ProjectCount != 2 {
		t.Fatalf("ProjectCount = %d, want 2", exportResult.Manifest.ProjectCount)
	}
	for _, file := range exportResult.Manifest.Files {
		if filepath.ToSlash(file.Path) == "" {
			t.Fatalf("manifest file path should never be empty")
		}
		if file.Path == "projects/proj-1/.worktrees/task-1/note.txt" {
			t.Fatalf("worktree file should not be exported when include_worktrees=false")
		}
	}

	destRoot := t.TempDir()
	destRegistryPath := filepath.Join(destRoot, "registry", "projects.yaml")
	destGlobalConfigPath := filepath.Join(destRoot, "registry", "global-config.yaml")

	pathMap := map[string]string{
		p1.Path: filepath.Join(destRoot, "mapped-project-one"),
		p2.Path: filepath.Join(destRoot, "mapped-project-two"),
	}

	report, err := Import(&ImportOptions{
		InputPath:          snapshotPath,
		Mode:               ImportModeReplace,
		ConflictPolicy:     ConflictOverwrite,
		PathMap:            pathMap,
		PreserveProjectIDs: true,
		IncludeWorktrees:   false,
		RegistryPath:       destRegistryPath,
		GlobalConfigPath:   destGlobalConfigPath,
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(report.Projects) != 2 {
		t.Fatalf("len(report.Projects) = %d, want 2", len(report.Projects))
	}

	destRegistry, err := readRegistryConfig(destRegistryPath)
	if err != nil {
		t.Fatalf("readRegistryConfig() error = %v", err)
	}
	if len(destRegistry.Projects) != 2 {
		t.Fatalf("len(destRegistry.Projects) = %d, want 2", len(destRegistry.Projects))
	}
	if destRegistry.DefaultProject != p1.ID {
		t.Fatalf("DefaultProject = %q, want %q", destRegistry.DefaultProject, p1.ID)
	}

	for _, proj := range destRegistry.Projects {
		if proj.ID == p1.ID && proj.Path != pathMap[p1.Path] {
			t.Fatalf("mapped path for proj-1 = %q, want %q", proj.Path, pathMap[p1.Path])
		}
		if proj.ID == p2.ID && proj.Path != pathMap[p2.Path] {
			t.Fatalf("mapped path for proj-2 = %q, want %q", proj.Path, pathMap[p2.Path])
		}
	}

	if _, err := os.Stat(filepath.Join(pathMap[p1.Path], ".quorum", "state", "state.json")); err != nil {
		t.Fatalf("expected restored project file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pathMap[p1.Path], ".worktrees", "task-1", "note.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected worktree file to be absent when include_worktrees=false")
	}

	globalData, err := os.ReadFile(destGlobalConfigPath)
	if err != nil {
		t.Fatalf("reading restored global config: %v", err)
	}
	if string(globalData) != "global-config: true\n" {
		t.Fatalf("global config content = %q", string(globalData))
	}
}

func TestImportDryRunDoesNotMutate(t *testing.T) {
	tmp := t.TempDir()
	snapshotPath := filepath.Join(tmp, "snapshot.tar.gz")
	registryPath := filepath.Join(tmp, "registry", "projects.yaml")
	globalPath := filepath.Join(tmp, "registry", "global-config.yaml")

	p := mustCreateProjectFixture(t, tmp, "proj-1", "source", true, false)
	mustWriteRegistryFixture(t, registryPath, &project.RegistryConfig{
		Version:        1,
		DefaultProject: p.ID,
		Projects:       []*project.Project{p},
	})
	mustWriteFile(t, globalPath, []byte("global-config: src\n"), 0o600)

	if _, err := Export(&ExportOptions{
		OutputPath:       snapshotPath,
		IncludeWorktrees: false,
		RegistryPath:     registryPath,
		GlobalConfigPath: globalPath,
	}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	destRoot := t.TempDir()
	destRegistryPath := filepath.Join(destRoot, "registry", "projects.yaml")
	destGlobalPath := filepath.Join(destRoot, "registry", "global-config.yaml")
	existingCfg := &project.RegistryConfig{
		Version:        1,
		DefaultProject: "existing",
		Projects: []*project.Project{{
			ID:           "existing",
			Path:         filepath.Join(destRoot, "existing"),
			Name:         "Existing",
			Status:       project.StatusHealthy,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ConfigMode:   project.ConfigModeCustom,
		}},
	}
	mustWriteRegistryFixture(t, destRegistryPath, existingCfg)
	mustWriteFile(t, destGlobalPath, []byte("global-config: existing\n"), 0o600)

	beforeRegistry, err := os.ReadFile(destRegistryPath)
	if err != nil {
		t.Fatalf("reading registry before import: %v", err)
	}
	beforeGlobal, err := os.ReadFile(destGlobalPath)
	if err != nil {
		t.Fatalf("reading global before import: %v", err)
	}

	report, err := Import(&ImportOptions{
		InputPath:          snapshotPath,
		Mode:               ImportModeReplace,
		DryRun:             true,
		ConflictPolicy:     ConflictOverwrite,
		PreserveProjectIDs: true,
		IncludeWorktrees:   false,
		RegistryPath:       destRegistryPath,
		GlobalConfigPath:   destGlobalPath,
	})
	if err != nil {
		t.Fatalf("Import() dry-run error = %v", err)
	}
	if !report.DryRun {
		t.Fatalf("report.DryRun = false, want true")
	}

	afterRegistry, err := os.ReadFile(destRegistryPath)
	if err != nil {
		t.Fatalf("reading registry after import: %v", err)
	}
	afterGlobal, err := os.ReadFile(destGlobalPath)
	if err != nil {
		t.Fatalf("reading global after import: %v", err)
	}

	if string(beforeRegistry) != string(afterRegistry) {
		t.Fatalf("registry changed during dry-run")
	}
	if string(beforeGlobal) != string(afterGlobal) {
		t.Fatalf("global config changed during dry-run")
	}
}

func TestImportConflictPolicies(t *testing.T) {
	sourceRoot := t.TempDir()
	snapshotPath := filepath.Join(sourceRoot, "snapshot.tar.gz")
	sourceRegistryPath := filepath.Join(sourceRoot, "registry", "projects.yaml")
	sourceGlobalPath := filepath.Join(sourceRoot, "registry", "global-config.yaml")

	sourceProject := mustCreateProjectFixture(t, sourceRoot, "proj-source", "src", true, false)
	mustWriteRegistryFixture(t, sourceRegistryPath, &project.RegistryConfig{
		Version:        1,
		DefaultProject: sourceProject.ID,
		Projects:       []*project.Project{sourceProject},
	})
	mustWriteFile(t, sourceGlobalPath, []byte("global-config: source\n"), 0o600)

	if _, err := Export(&ExportOptions{
		OutputPath:       snapshotPath,
		IncludeWorktrees: false,
		RegistryPath:     sourceRegistryPath,
		GlobalConfigPath: sourceGlobalPath,
	}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	t.Run("fail", func(t *testing.T) {
		destRoot := t.TempDir()
		destRegistryPath := filepath.Join(destRoot, "registry", "projects.yaml")
		destGlobalPath := filepath.Join(destRoot, "registry", "global-config.yaml")

		existingPath := filepath.Join(destRoot, "existing")
		mustWriteRegistryFixture(t, destRegistryPath, &project.RegistryConfig{
			Version:        1,
			DefaultProject: "existing-id",
			Projects: []*project.Project{{
				ID:           "existing-id",
				Path:         existingPath,
				Name:         "Existing",
				Status:       project.StatusHealthy,
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				ConfigMode:   project.ConfigModeCustom,
			}},
		})
		mustWriteFile(t, destGlobalPath, []byte("global-config: existing\n"), 0o600)

		_, err := Import(&ImportOptions{
			InputPath:          snapshotPath,
			Mode:               ImportModeMerge,
			ConflictPolicy:     ConflictFail,
			PathMap:            map[string]string{sourceProject.Path: existingPath},
			PreserveProjectIDs: true,
			RegistryPath:       destRegistryPath,
			GlobalConfigPath:   destGlobalPath,
		})
		if err == nil {
			t.Fatalf("expected conflict error with policy=fail")
		}
	})

	t.Run("skip", func(t *testing.T) {
		destRoot := t.TempDir()
		destRegistryPath := filepath.Join(destRoot, "registry", "projects.yaml")
		destGlobalPath := filepath.Join(destRoot, "registry", "global-config.yaml")

		existingPath := filepath.Join(destRoot, "existing")
		mustWriteRegistryFixture(t, destRegistryPath, &project.RegistryConfig{
			Version:        1,
			DefaultProject: "existing-id",
			Projects: []*project.Project{{
				ID:           "existing-id",
				Path:         existingPath,
				Name:         "Existing",
				Status:       project.StatusHealthy,
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				ConfigMode:   project.ConfigModeCustom,
			}},
		})
		mustWriteFile(t, destGlobalPath, []byte("global-config: existing\n"), 0o600)

		report, err := Import(&ImportOptions{
			InputPath:          snapshotPath,
			Mode:               ImportModeMerge,
			ConflictPolicy:     ConflictSkip,
			PathMap:            map[string]string{sourceProject.Path: existingPath},
			PreserveProjectIDs: true,
			RegistryPath:       destRegistryPath,
			GlobalConfigPath:   destGlobalPath,
		})
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}
		if len(report.Projects) != 1 || report.Projects[0].Action != "skipped" {
			t.Fatalf("expected one skipped project, got %+v", report.Projects)
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		destRoot := t.TempDir()
		destRegistryPath := filepath.Join(destRoot, "registry", "projects.yaml")
		destGlobalPath := filepath.Join(destRoot, "registry", "global-config.yaml")

		existingPath := filepath.Join(destRoot, "existing")
		mustWriteRegistryFixture(t, destRegistryPath, &project.RegistryConfig{
			Version:        1,
			DefaultProject: "existing-id",
			Projects: []*project.Project{{
				ID:           "existing-id",
				Path:         existingPath,
				Name:         "Existing",
				Status:       project.StatusHealthy,
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				ConfigMode:   project.ConfigModeCustom,
			}},
		})
		mustWriteFile(t, destGlobalPath, []byte("global-config: existing\n"), 0o600)

		report, err := Import(&ImportOptions{
			InputPath:          snapshotPath,
			Mode:               ImportModeMerge,
			ConflictPolicy:     ConflictOverwrite,
			PathMap:            map[string]string{sourceProject.Path: existingPath},
			PreserveProjectIDs: true,
			RegistryPath:       destRegistryPath,
			GlobalConfigPath:   destGlobalPath,
		})
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}
		if len(report.Projects) != 1 || report.Projects[0].Action != "overwritten" {
			t.Fatalf("expected one overwritten project, got %+v", report.Projects)
		}

		cfg, err := readRegistryConfig(destRegistryPath)
		if err != nil {
			t.Fatalf("readRegistryConfig() error = %v", err)
		}
		if len(cfg.Projects) != 1 {
			t.Fatalf("len(cfg.Projects) = %d, want 1", len(cfg.Projects))
		}
		if cfg.Projects[0].ID != "existing-id" {
			t.Fatalf("overwritten project id = %q, want existing-id", cfg.Projects[0].ID)
		}
	})
}

func TestValidateSnapshot_InvalidFile(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "invalid.tar.gz")
	mustWriteFile(t, invalidPath, []byte("not-a-gzip"), 0o600)

	if _, err := ValidateSnapshot(invalidPath); err == nil {
		t.Fatalf("expected ValidateSnapshot() to fail for invalid archive")
	}
}

func mustCreateProjectFixture(t *testing.T, root, id, name string, withConfig bool, withWorktree bool) *project.Project {
	t.Helper()
	path := filepath.Join(root, name)
	mustMkdirAll(t, filepath.Join(path, ".quorum", "state"))
	mustMkdirAll(t, filepath.Join(path, ".quorum", "logs"))
	mustWriteFile(t, filepath.Join(path, ".quorum", "state", "state.json"), []byte(`{"ok":true}`), 0o600)
	if withConfig {
		mustWriteFile(t, filepath.Join(path, ".quorum", "config.yaml"), []byte("log:\n  level: info\n"), 0o600)
	}
	if withWorktree {
		mustMkdirAll(t, filepath.Join(path, ".worktrees", "task-1"))
		mustWriteFile(t, filepath.Join(path, ".worktrees", "task-1", "note.txt"), []byte("worktree"), 0o600)
	}

	mode := project.ConfigModeInheritGlobal
	if withConfig {
		mode = project.ConfigModeCustom
	}

	now := time.Now().UTC()
	return &project.Project{
		ID:           id,
		Path:         path,
		Name:         name,
		Status:       project.StatusHealthy,
		CreatedAt:    now,
		LastAccessed: now,
		ConfigMode:   mode,
	}
}

func mustWriteRegistryFixture(t *testing.T, path string, cfg *project.RegistryConfig) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	mustWriteFile(t, path, data, 0o600)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}
