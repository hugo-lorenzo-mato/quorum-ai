package snapshot

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
	"gopkg.in/yaml.v3"
)

func normalizeImportOptions(opts *ImportOptions) error {
	if opts == nil {
		return fmt.Errorf("options are required")
	}
	if strings.TrimSpace(opts.InputPath) == "" {
		return fmt.Errorf("input path is required")
	}
	if opts.Mode == "" {
		opts.Mode = ImportModeMerge
	}
	if opts.Mode != ImportModeMerge && opts.Mode != ImportModeReplace {
		return fmt.Errorf("invalid import mode: %s", opts.Mode)
	}

	if opts.ConflictPolicy == "" {
		opts.ConflictPolicy = ConflictSkip
	}
	if opts.ConflictPolicy != ConflictSkip && opts.ConflictPolicy != ConflictOverwrite && opts.ConflictPolicy != ConflictFail {
		return fmt.Errorf("invalid conflict policy: %s", opts.ConflictPolicy)
	}

	if !opts.PreserveProjectIDs {
		// explicit false is allowed
	} else {
		opts.PreserveProjectIDs = true
	}

	if strings.TrimSpace(opts.RegistryPath) == "" {
		path, err := project.DefaultRegistryPath()
		if err != nil {
			return fmt.Errorf("resolving registry path: %w", err)
		}
		opts.RegistryPath = path
	}
	if strings.TrimSpace(opts.GlobalConfigPath) == "" {
		path, err := config.GlobalConfigPath()
		if err != nil {
			return fmt.Errorf("resolving global config path: %w", err)
		}
		opts.GlobalConfigPath = path
	}

	if opts.PathMap == nil {
		opts.PathMap = make(map[string]string)
	}
	return nil
}

func normalizeExportOptions(opts *ExportOptions) error {
	if opts == nil {
		return fmt.Errorf("options are required")
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	if strings.TrimSpace(opts.RegistryPath) == "" {
		path, err := project.DefaultRegistryPath()
		if err != nil {
			return fmt.Errorf("resolving registry path: %w", err)
		}
		opts.RegistryPath = path
	}
	if strings.TrimSpace(opts.GlobalConfigPath) == "" {
		path, err := config.GlobalConfigPath()
		if err != nil {
			return fmt.Errorf("resolving global config path: %w", err)
		}
		opts.GlobalConfigPath = path
	}
	return nil
}

func computeSHA256ForFile(path string) (string, int64, error) {
	f, err := os.Open(path) // #nosec G304 -- path validated by callers
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	hash := sha256.New()
	n, err := io.Copy(hash, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}

func archivePathJoin(parts ...string) string {
	p := filepath.Join(parts...)
	return filepath.ToSlash(p)
}

func cleanArchivePath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty archive path")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute archive path is not allowed: %s", p)
	}
	clean := filepath.Clean(strings.TrimPrefix(p, "./"))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("invalid archive path: %s", p)
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, `..\`) {
		return "", fmt.Errorf("path traversal detected: %s", p)
	}
	return clean, nil
}

func ensureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o750)
}

func readRegistryConfig(path string) (*project.RegistryConfig, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller controls path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &project.RegistryConfig{
				Version:  1,
				Projects: make([]*project.Project, 0),
			}, nil
		}
		return nil, err
	}
	var cfg project.RegistryConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Projects == nil {
		cfg.Projects = make([]*project.Project, 0)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	return &cfg, nil
}

func writeRegistryConfigAtomic(path string, cfg *project.RegistryConfig) error {
	if cfg == nil {
		return fmt.Errorf("registry config is nil")
	}
	if cfg.Projects == nil {
		cfg.Projects = make([]*project.Project, 0)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func cloneRegistryConfig(cfg *project.RegistryConfig) *project.RegistryConfig {
	out := &project.RegistryConfig{
		Version:        cfg.Version,
		DefaultProject: cfg.DefaultProject,
		Projects:       make([]*project.Project, 0, len(cfg.Projects)),
	}
	for _, p := range cfg.Projects {
		if p == nil {
			continue
		}
		out.Projects = append(out.Projects, p.Clone())
	}
	return out
}

func randomProjectID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("proj-%d", os.Getpid())
	}
	return fmt.Sprintf("proj-%s", hex.EncodeToString(b)[:12])
}

func mapProjectPath(path string, pathMap map[string]string) string {
	if len(pathMap) == 0 {
		return path
	}
	if mapped, ok := pathMap[path]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return path
}

func sortFileEntries(entries []FileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
}

func encodeManifest(manifest *Manifest) ([]byte, error) {
	sortFileEntries(manifest.Files)
	return json.MarshalIndent(manifest, "", "  ")
}

func decodeManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if manifest.Version != FormatVersion {
		return nil, fmt.Errorf("unsupported snapshot version: %d", manifest.Version)
	}
	return &manifest, nil
}
