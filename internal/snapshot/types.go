package snapshot

import (
	"time"
)

const (
	// FormatVersion is the current snapshot manifest format version.
	FormatVersion = 1

	manifestArchivePath     = "manifest.json"
	registryArchivePath     = "registry/projects.yaml"
	globalConfigArchivePath = "registry/global-config.yaml"
	projectsArchiveRoot     = "projects"
)

// ConflictPolicy controls how import handles destination conflicts.
type ConflictPolicy string

const (
	ConflictSkip      ConflictPolicy = "skip"
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictFail      ConflictPolicy = "fail"
)

// ImportMode controls how snapshot import applies registry/project data.
type ImportMode string

const (
	ImportModeMerge   ImportMode = "merge"
	ImportModeReplace ImportMode = "replace"
)

// FileEntry describes one archived file.
type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mode   int64  `json:"mode"`
}

// ProjectEntry captures project metadata embedded in the manifest.
type ProjectEntry struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	Color         string    `json:"color,omitempty"`
	Status        string    `json:"status,omitempty"`
	StatusMessage string    `json:"status_message,omitempty"`
	ConfigMode    string    `json:"config_mode,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LastAccessed  time.Time `json:"last_accessed"`
}

// Manifest is the metadata file stored at snapshot root.
type Manifest struct {
	Version             int            `json:"version"`
	CreatedAt           time.Time      `json:"created_at"`
	QuorumVersion       string         `json:"quorum_version,omitempty"`
	IncludeWorktrees    bool           `json:"include_worktrees"`
	GlobalConfigPresent bool           `json:"global_config_present"`
	ProjectCount        int            `json:"project_count"`
	DefaultProjectID    string         `json:"default_project_id,omitempty"`
	Projects            []ProjectEntry `json:"projects"`
	Files               []FileEntry    `json:"files"`
}

// ExportOptions configures snapshot export behavior.
type ExportOptions struct {
	OutputPath       string
	IncludeWorktrees bool
	ProjectIDs       []string
	RegistryPath     string
	GlobalConfigPath string
	QuorumVersion    string
}

// ExportResult describes an export operation.
type ExportResult struct {
	OutputPath string    `json:"output_path"`
	Manifest   *Manifest `json:"manifest"`
}

// ImportOptions configures snapshot import behavior.
type ImportOptions struct {
	InputPath string

	Mode           ImportMode
	DryRun         bool
	ConflictPolicy ConflictPolicy
	PathMap        map[string]string

	PreserveProjectIDs bool
	IncludeWorktrees   bool

	RegistryPath     string
	GlobalConfigPath string
}

// ProjectImportReport is the per-project result from import.
type ProjectImportReport struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Path     string `json:"path"`
	Action   string `json:"action"`
	Reason   string `json:"reason,omitempty"`
}

// ImportReport summarizes import execution.
type ImportReport struct {
	Mode           ImportMode            `json:"mode"`
	DryRun         bool                  `json:"dry_run"`
	ConflictPolicy ConflictPolicy        `json:"conflict_policy"`
	Manifest       *Manifest             `json:"manifest"`
	Projects       []ProjectImportReport `json:"projects"`
	Conflicts      []string              `json:"conflicts,omitempty"`
	Warnings       []string              `json:"warnings,omitempty"`
	RestoredFiles  int                   `json:"restored_files"`
	SkippedFiles   int                   `json:"skipped_files"`
}
