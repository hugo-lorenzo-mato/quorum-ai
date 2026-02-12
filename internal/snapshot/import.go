package snapshot

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// Import restores projects and global settings from a snapshot archive.
func Import(opts *ImportOptions) (*ImportReport, error) {
	if err := normalizeImportOptions(opts); err != nil {
		return nil, err
	}

	manifest, archiveFiles, err := loadSnapshotArchive(opts.InputPath)
	if err != nil {
		return nil, err
	}

	report := &ImportReport{
		Mode:           opts.Mode,
		DryRun:         opts.DryRun,
		ConflictPolicy: opts.ConflictPolicy,
		Manifest:       manifest,
		Projects:       make([]ProjectImportReport, 0, len(manifest.Projects)),
		Conflicts:      make([]string, 0),
		Warnings:       make([]string, 0),
	}

	currentCfg, err := readRegistryConfig(opts.RegistryPath)
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	workingCfg := cloneRegistryConfig(currentCfg)
	if opts.Mode == ImportModeReplace {
		workingCfg = &project.RegistryConfig{Version: 1, Projects: make([]*project.Project, 0, len(manifest.Projects))}
	}
	if workingCfg.Projects == nil {
		workingCfg.Projects = make([]*project.Project, 0)
	}

	restoreTargets := make(map[string]*project.Project, len(manifest.Projects))
	idMap := make(map[string]string, len(manifest.Projects))

	for _, entry := range manifest.Projects {
		target, projectReport, conflict, planErr := planProjectImport(workingCfg, entry, opts)
		if planErr != nil {
			return nil, planErr
		}
		report.Projects = append(report.Projects, projectReport)
		if conflict != "" {
			report.Conflicts = append(report.Conflicts, conflict)
		}
		if target == nil {
			continue
		}
		restoreTargets[entry.ID] = target
		idMap[entry.ID] = target.ID
	}

	if manifest.GlobalConfigPresent {
		if cfgEntry, ok := archiveFiles[globalConfigArchivePath]; ok {
			if err := applyGlobalConfigImport(opts, cfgEntry, report); err != nil {
				return nil, err
			}
		} else {
			report.Warnings = append(report.Warnings, "manifest indicates global config but archive entry is missing")
		}
	}

	if err := restoreProjectFiles(opts, manifest, archiveFiles, restoreTargets, report); err != nil {
		return nil, err
	}

	if opts.Mode == ImportModeReplace {
		if manifest.DefaultProjectID != "" {
			workingCfg.DefaultProject = idMap[manifest.DefaultProjectID]
		}
	} else if workingCfg.DefaultProject == "" && manifest.DefaultProjectID != "" {
		workingCfg.DefaultProject = idMap[manifest.DefaultProjectID]
	}

	if !opts.DryRun {
		if err := writeRegistryConfigAtomic(opts.RegistryPath, workingCfg); err != nil {
			return nil, fmt.Errorf("writing registry: %w", err)
		}
	}

	return report, nil
}

func planProjectImport(cfg *project.RegistryConfig, entry ProjectEntry, opts *ImportOptions) (*project.Project, ProjectImportReport, string, error) {
	targetPathRaw := strings.TrimSpace(mapProjectPath(entry.Path, opts.PathMap))
	if targetPathRaw == "" {
		report := ProjectImportReport{
			SourceID: entry.ID,
			TargetID: "",
			Path:     "",
			Action:   "skipped",
			Reason:   "target path is empty after path mapping",
		}
		return nil, report, "", nil
	}

	targetPathAbs, err := filepath.Abs(targetPathRaw)
	if err != nil {
		return nil, ProjectImportReport{}, "", fmt.Errorf("resolving target path for project %s: %w", entry.ID, err)
	}
	targetPath := filepath.Clean(targetPathAbs)

	candidate := projectFromManifestEntry(entry, targetPath)
	if !opts.PreserveProjectIDs {
		candidate.ID = randomProjectID()
	}

	indexByPath := findProjectIndexByPath(cfg, targetPath)
	indexByID := findProjectIndexByID(cfg, candidate.ID)
	existingIndex := -1
	if indexByPath >= 0 {
		existingIndex = indexByPath
	} else if indexByID >= 0 {
		existingIndex = indexByID
	}

	if existingIndex >= 0 {
		existing := cfg.Projects[existingIndex]
		sameIdentity := existing != nil && existing.ID == candidate.ID && filepath.Clean(existing.Path) == targetPath
		if sameIdentity {
			candidate.ID = existing.ID
			cfg.Projects[existingIndex] = candidate
			report := ProjectImportReport{
				SourceID: entry.ID,
				TargetID: candidate.ID,
				Path:     targetPath,
				Action:   "updated",
			}
			return candidate, report, "", nil
		}

		conflictMsg := fmt.Sprintf("project %s conflicts with existing project (existing_id=%s, path=%s)", entry.ID, existing.ID, existing.Path)
		switch opts.ConflictPolicy {
		case ConflictSkip:
			report := ProjectImportReport{
				SourceID: entry.ID,
				TargetID: existing.ID,
				Path:     targetPath,
				Action:   "skipped",
				Reason:   conflictMsg,
			}
			return nil, report, conflictMsg, nil
		case ConflictFail:
			return nil, ProjectImportReport{}, conflictMsg, fmt.Errorf("import conflict: %s", conflictMsg)
		case ConflictOverwrite:
			candidate.ID = existing.ID
			cfg.Projects[existingIndex] = candidate
			report := ProjectImportReport{
				SourceID: entry.ID,
				TargetID: candidate.ID,
				Path:     targetPath,
				Action:   "overwritten",
				Reason:   conflictMsg,
			}
			return candidate, report, conflictMsg, nil
		default:
			return nil, ProjectImportReport{}, conflictMsg, fmt.Errorf("unsupported conflict policy: %s", opts.ConflictPolicy)
		}
	}

	if !opts.PreserveProjectIDs {
		for findProjectIndexByID(cfg, candidate.ID) >= 0 {
			candidate.ID = randomProjectID()
		}
	} else if findProjectIndexByID(cfg, candidate.ID) >= 0 {
		conflictMsg := fmt.Sprintf("project %s conflicts on id %s", entry.ID, candidate.ID)
		switch opts.ConflictPolicy {
		case ConflictSkip:
			report := ProjectImportReport{
				SourceID: entry.ID,
				TargetID: candidate.ID,
				Path:     targetPath,
				Action:   "skipped",
				Reason:   conflictMsg,
			}
			return nil, report, conflictMsg, nil
		case ConflictFail:
			return nil, ProjectImportReport{}, conflictMsg, fmt.Errorf("import conflict: %s", conflictMsg)
		case ConflictOverwrite:
			idx := findProjectIndexByID(cfg, candidate.ID)
			cfg.Projects[idx] = candidate
			report := ProjectImportReport{
				SourceID: entry.ID,
				TargetID: candidate.ID,
				Path:     targetPath,
				Action:   "overwritten",
				Reason:   conflictMsg,
			}
			return candidate, report, conflictMsg, nil
		default:
			return nil, ProjectImportReport{}, conflictMsg, fmt.Errorf("unsupported conflict policy: %s", opts.ConflictPolicy)
		}
	}

	cfg.Projects = append(cfg.Projects, candidate)
	report := ProjectImportReport{
		SourceID: entry.ID,
		TargetID: candidate.ID,
		Path:     targetPath,
		Action:   "added",
	}
	return candidate, report, "", nil
}

func restoreProjectFiles(
	opts *ImportOptions,
	manifest *Manifest,
	archiveFiles map[string]archivedFile,
	restoreTargets map[string]*project.Project,
	report *ImportReport,
) error {
	for _, fileEntry := range manifest.Files {
		if fileEntry.Path == registryArchivePath || fileEntry.Path == globalConfigArchivePath {
			continue
		}

		sourceID, relPath, ok := parseProjectArchivePath(fileEntry.Path)
		if !ok {
			report.SkippedFiles++
			report.Warnings = append(report.Warnings, fmt.Sprintf("skipping unrecognized project file entry: %s", fileEntry.Path))
			continue
		}

		targetProject, selected := restoreTargets[sourceID]
		if !selected {
			report.SkippedFiles++
			continue
		}

		if !opts.IncludeWorktrees && (relPath == ".worktrees" || strings.HasPrefix(relPath, ".worktrees/")) {
			report.SkippedFiles++
			continue
		}

		archiveFile, ok := archiveFiles[fileEntry.Path]
		if !ok {
			return fmt.Errorf("archive entry missing for %s", fileEntry.Path)
		}

		targetFilePath, resolveErr := resolveTargetProjectFilePath(targetProject.Path, relPath)
		if resolveErr != nil {
			return fmt.Errorf("resolving target file %s: %w", fileEntry.Path, resolveErr)
		}

		if opts.DryRun {
			report.RestoredFiles++
			continue
		}

		exists := false
		if _, statErr := os.Stat(targetFilePath); statErr == nil {
			exists = true
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("checking target file %s: %w", targetFilePath, statErr)
		}

		if exists {
			switch opts.ConflictPolicy {
			case ConflictSkip:
				report.SkippedFiles++
				continue
			case ConflictFail:
				return fmt.Errorf("file conflict at %s", targetFilePath)
			case ConflictOverwrite:
				// Continue and overwrite.
			}
		}

		if err := ensureParentDir(targetFilePath); err != nil {
			return fmt.Errorf("creating target directory for %s: %w", targetFilePath, err)
		}

		var mode os.FileMode
		if archiveFile.Mode < 0 || archiveFile.Mode > math.MaxUint32 {
			mode = 0o600
		} else {
			mode = os.FileMode(archiveFile.Mode)
		}
		if mode == 0 {
			mode = 0o600
		}
		if err := os.WriteFile(targetFilePath, archiveFile.Data, mode); err != nil {
			return fmt.Errorf("writing file %s: %w", targetFilePath, err)
		}
		report.RestoredFiles++
	}

	if !opts.DryRun {
		for _, target := range restoreTargets {
			if target == nil {
				continue
			}
			if err := os.MkdirAll(filepath.Join(target.Path, ".quorum"), 0o750); err != nil {
				return fmt.Errorf("ensuring .quorum directory for %s: %w", target.Path, err)
			}
		}
	}

	return nil
}

func resolveTargetProjectFilePath(projectPath, relPath string) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(relPath))
	if cleanRel == "." || cleanRel == "" {
		return "", fmt.Errorf("invalid relative file path")
	}
	if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("path traversal blocked: %s", relPath)
	}
	return filepath.Join(projectPath, cleanRel), nil
}

func applyGlobalConfigImport(opts *ImportOptions, cfgEntry archivedFile, report *ImportReport) error {
	if opts.DryRun {
		report.RestoredFiles++
		return nil
	}

	exists := false
	if _, err := os.Stat(opts.GlobalConfigPath); err == nil {
		exists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking global config destination: %w", err)
	}

	if exists {
		switch opts.ConflictPolicy {
		case ConflictSkip:
			report.SkippedFiles++
			return nil
		case ConflictFail:
			return fmt.Errorf("global config already exists at %s", opts.GlobalConfigPath)
		case ConflictOverwrite:
			// Continue and overwrite.
		}
	}

	if err := ensureParentDir(opts.GlobalConfigPath); err != nil {
		return fmt.Errorf("creating global config directory: %w", err)
	}

	var mode os.FileMode
	if cfgEntry.Mode < 0 || cfgEntry.Mode > math.MaxUint32 {
		mode = 0o600
	} else {
		mode = os.FileMode(cfgEntry.Mode)
	}
	if mode == 0 {
		mode = 0o600
	}
	if err := os.WriteFile(opts.GlobalConfigPath, cfgEntry.Data, mode); err != nil {
		return fmt.Errorf("writing global config: %w", err)
	}
	report.RestoredFiles++
	return nil
}

func projectFromManifestEntry(entry ProjectEntry, targetPath string) *project.Project {
	status := project.ProjectStatus(entry.Status)
	if !status.IsValid() {
		status = project.StatusHealthy
	}

	configMode := strings.TrimSpace(entry.ConfigMode)
	if configMode != project.ConfigModeInheritGlobal && configMode != project.ConfigModeCustom {
		configMode = project.ConfigModeCustom
	}

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	lastAccessed := entry.LastAccessed
	if lastAccessed.IsZero() {
		lastAccessed = createdAt
	}

	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = filepath.Base(targetPath)
	}

	return &project.Project{
		ID:            entry.ID,
		Path:          targetPath,
		Name:          name,
		LastAccessed:  lastAccessed,
		Status:        status,
		StatusMessage: entry.StatusMessage,
		Color:         entry.Color,
		CreatedAt:     createdAt,
		ConfigMode:    configMode,
	}
}

func parseProjectArchivePath(p string) (sourceID, relPath string, ok bool) {
	prefix := projectsArchiveRoot + "/"
	if !strings.HasPrefix(p, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(p, prefix)
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func findProjectIndexByPath(cfg *project.RegistryConfig, path string) int {
	for i, p := range cfg.Projects {
		if p == nil {
			continue
		}
		if filepath.Clean(p.Path) == filepath.Clean(path) {
			return i
		}
	}
	return -1
}

func findProjectIndexByID(cfg *project.RegistryConfig, id string) int {
	for i, p := range cfg.Projects {
		if p == nil {
			continue
		}
		if p.ID == id {
			return i
		}
	}
	return -1
}
