package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
	"gopkg.in/yaml.v3"
)

// Export creates a snapshot archive with global registry/config and project data.
func Export(opts *ExportOptions) (*ExportResult, error) {
	if err := normalizeExportOptions(opts); err != nil {
		return nil, err
	}

	registryCfg, err := readRegistryConfig(opts.RegistryPath)
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	selectedProjects, err := selectProjectsForExport(registryCfg, opts.ProjectIDs)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("creating snapshot file: %w", err)
	}
	defer out.Close()

	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	subsetCfg := &project.RegistryConfig{
		Version:        registryCfg.Version,
		DefaultProject: "",
		Projects:       make([]*project.Project, 0, len(selectedProjects)),
	}

	manifest := &Manifest{
		Version:             FormatVersion,
		CreatedAt:           time.Now().UTC(),
		QuorumVersion:       opts.QuorumVersion,
		IncludeWorktrees:    opts.IncludeWorktrees,
		GlobalConfigPresent: false,
		ProjectCount:        len(selectedProjects),
		DefaultProjectID:    "",
		Projects:            make([]ProjectEntry, 0, len(selectedProjects)),
		Files:               make([]FileEntry, 0),
	}

	for _, p := range selectedProjects {
		subsetCfg.Projects = append(subsetCfg.Projects, p.Clone())
		if registryCfg.DefaultProject == p.ID {
			subsetCfg.DefaultProject = p.ID
			manifest.DefaultProjectID = p.ID
		}

		manifest.Projects = append(manifest.Projects, ProjectEntry{
			ID:            p.ID,
			Path:          p.Path,
			Name:          p.Name,
			Color:         p.Color,
			Status:        string(p.Status),
			StatusMessage: p.StatusMessage,
			ConfigMode:    p.ConfigMode,
			CreatedAt:     p.CreatedAt,
			LastAccessed:  p.LastAccessed,
		})
	}

	sort.Slice(manifest.Projects, func(i, j int) bool {
		return manifest.Projects[i].ID < manifest.Projects[j].ID
	})

	registryBytes, err := yaml.Marshal(subsetCfg)
	if err != nil {
		return nil, fmt.Errorf("encoding registry: %w", err)
	}
	if err := addBytesToArchive(tarWriter, manifest, registryArchivePath, registryBytes, 0o600); err != nil {
		return nil, err
	}

	if globalData, globalMode, globalExists, readErr := tryReadGlobalConfig(opts.GlobalConfigPath); readErr != nil {
		return nil, fmt.Errorf("reading global config: %w", readErr)
	} else if globalExists {
		manifest.GlobalConfigPresent = true
		if err := addBytesToArchive(tarWriter, manifest, globalConfigArchivePath, globalData, globalMode); err != nil {
			return nil, err
		}
	}

	for _, p := range selectedProjects {
		files, listErr := listProjectFiles(p.Path, opts.IncludeWorktrees)
		if listErr != nil {
			return nil, fmt.Errorf("listing files for project %s: %w", p.ID, listErr)
		}

		for _, filePath := range files {
			relPath, relErr := filepath.Rel(p.Path, filePath)
			if relErr != nil {
				return nil, fmt.Errorf("computing relative path for %s: %w", filePath, relErr)
			}
			if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
				return nil, fmt.Errorf("invalid project file path: %s", filePath)
			}

			archivePath, cleanErr := cleanArchivePath(archivePathJoin(projectsArchiveRoot, p.ID, relPath))
			if cleanErr != nil {
				return nil, fmt.Errorf("invalid archive path for %s: %w", filePath, cleanErr)
			}

			data, mode, readErr := readFileWithMode(filePath)
			if readErr != nil {
				return nil, fmt.Errorf("reading file %s: %w", filePath, readErr)
			}
			if err := addBytesToArchive(tarWriter, manifest, archivePath, data, mode); err != nil {
				return nil, err
			}
		}
	}

	manifestData, err := encodeManifest(manifest)
	if err != nil {
		return nil, fmt.Errorf("encoding manifest: %w", err)
	}
	if err := writeTarEntry(tarWriter, manifestArchivePath, manifestData, 0o600); err != nil {
		return nil, fmt.Errorf("writing manifest: %w", err)
	}

	return &ExportResult{
		OutputPath: opts.OutputPath,
		Manifest:   manifest,
	}, nil
}

func selectProjectsForExport(cfg *project.RegistryConfig, selectedIDs []string) ([]*project.Project, error) {
	if cfg == nil {
		return nil, fmt.Errorf("registry configuration is nil")
	}
	if len(selectedIDs) == 0 {
		projects := make([]*project.Project, 0, len(cfg.Projects))
		for _, p := range cfg.Projects {
			if p == nil {
				continue
			}
			projects = append(projects, p.Clone())
		}
		return projects, nil
	}

	lookup := make(map[string]*project.Project, len(cfg.Projects))
	for _, p := range cfg.Projects {
		if p == nil {
			continue
		}
		lookup[p.ID] = p
	}

	projects := make([]*project.Project, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		p, ok := lookup[id]
		if !ok {
			return nil, fmt.Errorf("project %s not found in registry", id)
		}
		projects = append(projects, p.Clone())
	}
	return projects, nil
}

func listProjectFiles(projectPath string, includeWorktrees bool) ([]string, error) {
	roots := []string{filepath.Join(projectPath, ".quorum")}
	if includeWorktrees {
		roots = append(roots, filepath.Join(projectPath, ".worktrees"))
	}

	files := make([]string, 0)
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

func tryReadGlobalConfig(path string) ([]byte, int64, bool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, 0, false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}
	if info.IsDir() {
		return nil, 0, false, fmt.Errorf("global config path is a directory: %s", path)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path resolved by caller
	if err != nil {
		return nil, 0, false, err
	}
	return data, int64(info.Mode().Perm()), true, nil
}

func readFileWithMode(path string) ([]byte, int64, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is discovered from project roots
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	return data, int64(info.Mode().Perm()), nil
}

func addBytesToArchive(tw *tar.Writer, manifest *Manifest, archivePath string, data []byte, mode int64) error {
	cleanPath, err := cleanArchivePath(archivePath)
	if err != nil {
		return fmt.Errorf("invalid archive path: %w", err)
	}

	if err := writeTarEntry(tw, cleanPath, data, mode); err != nil {
		return fmt.Errorf("writing archive entry %s: %w", cleanPath, err)
	}

	hash := sha256.Sum256(data)
	manifest.Files = append(manifest.Files, FileEntry{
		Path:   cleanPath,
		SHA256: hex.EncodeToString(hash[:]),
		Size:   int64(len(data)),
		Mode:   mode,
	})
	return nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte, mode int64) error {
	header := &tar.Header{
		Name:     filepath.ToSlash(name),
		Mode:     mode,
		Size:     int64(len(data)),
		ModTime:  time.Now(),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
