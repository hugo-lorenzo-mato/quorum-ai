package project

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Predefined pleasant colors for project badges
var projectColors = []string{
	"#4A90D9", // Blue
	"#7B68EE", // Medium Slate Blue
	"#20B2AA", // Light Sea Green
	"#FF6B6B", // Coral Red
	"#FFA500", // Orange
	"#9370DB", // Medium Purple
	"#3CB371", // Medium Sea Green
	"#FFD700", // Gold
	"#00CED1", // Dark Turquoise
	"#FF69B4", // Hot Pink
}

// Registry defines the interface for project management
type Registry interface {
	// ListProjects returns all registered projects
	ListProjects(ctx context.Context) ([]*Project, error)

	// GetProject retrieves a project by ID
	GetProject(ctx context.Context, id string) (*Project, error)

	// GetProjectByPath retrieves a project by its filesystem path
	GetProjectByPath(ctx context.Context, path string) (*Project, error)

	// AddProject registers a new project from the given path
	AddProject(ctx context.Context, path string, opts *AddProjectOptions) (*Project, error)

	// RemoveProject unregisters a project by ID
	RemoveProject(ctx context.Context, id string) error

	// UpdateProject updates project metadata
	UpdateProject(ctx context.Context, project *Project) error

	// ValidateProject checks if a project is still valid and accessible
	ValidateProject(ctx context.Context, id string) error

	// ValidateAll validates all registered projects and updates their status
	ValidateAll(ctx context.Context) error

	// GetDefaultProject returns the default project for legacy endpoints
	GetDefaultProject(ctx context.Context) (*Project, error)

	// SetDefaultProject sets the default project for legacy endpoints
	SetDefaultProject(ctx context.Context, id string) error

	// TouchProject updates the last accessed time for a project
	TouchProject(ctx context.Context, id string) error

	// Reload reloads the registry from its backing store
	Reload() error

	// Close releases any resources held by the registry
	Close() error
}

// FileRegistry implements Registry using a YAML file for persistence
type FileRegistry struct {
	configPath    string
	config        *RegistryConfig
	mu            sync.RWMutex
	logger        *slog.Logger
	autoSave      bool
	backupEnabled bool
	closed        bool
	removedIDs    map[string]struct{}
}

// RegistryOption configures a FileRegistry
type RegistryOption func(*FileRegistry)

// WithLogger sets the logger for the registry
func WithLogger(logger *slog.Logger) RegistryOption {
	return func(r *FileRegistry) {
		r.logger = logger
	}
}

// WithConfigPath sets a custom config path
func WithConfigPath(path string) RegistryOption {
	return func(r *FileRegistry) {
		r.configPath = path
	}
}

// WithAutoSave enables/disables automatic saving after modifications
func WithAutoSave(enabled bool) RegistryOption {
	return func(r *FileRegistry) {
		r.autoSave = enabled
	}
}

// WithBackup enables/disables backup before save
func WithBackup(enabled bool) RegistryOption {
	return func(r *FileRegistry) {
		r.backupEnabled = enabled
	}
}

// NewFileRegistry creates a new file-based project registry
func NewFileRegistry(opts ...RegistryOption) (*FileRegistry, error) {
	r := &FileRegistry{
		logger:        slog.Default(),
		autoSave:      true,
		backupEnabled: true,
		removedIDs:    make(map[string]struct{}),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Determine config path if not set
	if r.configPath == "" {
		path, err := getRegistryPath()
		if err != nil {
			return nil, NewRegistryError("init", err)
		}
		r.configPath = path
	}

	// Load existing config or create new
	if err := r.load(); err != nil {
		return nil, err
	}

	r.logger.Info("project registry initialized",
		"config_path", r.configPath,
		"project_count", len(r.config.Projects))

	return r, nil
}

// getRegistryPath returns the default registry file path
func getRegistryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	quorumRegistryDir := filepath.Join(homeDir, ".quorum-registry")
	if err := os.MkdirAll(quorumRegistryDir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create registry directory: %w", err)
	}

	return filepath.Join(quorumRegistryDir, "projects.yaml"), nil
}

// load reads the registry from disk
func (r *FileRegistry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := os.ReadFile(r.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new empty config
			r.config = &RegistryConfig{
				Version:  1,
				Projects: make([]*Project, 0),
			}
			return nil
		}
		return NewRegistryError("load", err)
	}

	var config RegistryConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return NewRegistryError("load", fmt.Errorf("%w: %v", ErrRegistryCorrupted, err))
	}

	// Initialize empty projects slice if nil
	if config.Projects == nil {
		config.Projects = make([]*Project, 0)
	}

	r.config = &config
	r.removedIDs = make(map[string]struct{})
	return nil
}

// save writes the registry to disk
func (r *FileRegistry) save() error {
	// Caller must hold write lock

	// Merge with disk to preserve changes from other processes (CLI, other servers)
	r.mergeFromDisk()

	// Create backup if enabled
	if r.backupEnabled {
		if _, err := os.Stat(r.configPath); err == nil {
			backupPath := r.configPath + ".bak"
			data, _ := os.ReadFile(r.configPath)
			_ = os.WriteFile(backupPath, data, 0o600)
		}
	}

	data, err := yaml.Marshal(r.config)
	if err != nil {
		return NewRegistryError("save", err)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := r.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return NewRegistryError("save", err)
	}

	if err := os.Rename(tmpPath, r.configPath); err != nil {
		os.Remove(tmpPath)
		return NewRegistryError("save", err)
	}

	return nil
}

// mergeFromDisk reads the registry file and merges any new projects added by other processes
func (r *FileRegistry) mergeFromDisk() {
	// Read current file from disk
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		return // File doesn't exist or can't be read, nothing to merge
	}

	var diskConfig RegistryConfig
	if err := yaml.Unmarshal(data, &diskConfig); err != nil {
		return // Can't parse file, skip merge
	}

	// Build a map of our current projects by ID
	currentProjects := make(map[string]*Project)
	for _, p := range r.config.Projects {
		currentProjects[p.ID] = p
	}

	// Add any projects from disk that we don't have in memory
	for _, diskProject := range diskConfig.Projects {
		if _, removed := r.removedIDs[diskProject.ID]; removed {
			continue
		}
		if _, exists := currentProjects[diskProject.ID]; !exists {
			// This project was added by another process, preserve it
			r.config.Projects = append(r.config.Projects, diskProject)
		}
	}

	// If disk has a default project and we don't, use it
	if r.config.DefaultProject == "" && diskConfig.DefaultProject != "" {
		r.config.DefaultProject = diskConfig.DefaultProject
	}
}

// ListProjects returns all registered projects
func (r *FileRegistry) ListProjects(_ context.Context) ([]*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, ErrRegistryClosed
	}

	projects := make([]*Project, len(r.config.Projects))
	for i, p := range r.config.Projects {
		projects[i] = p.Clone()
	}
	return projects, nil
}

// GetProject retrieves a project by ID
func (r *FileRegistry) GetProject(_ context.Context, id string) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, ErrRegistryClosed
	}

	for _, p := range r.config.Projects {
		if p.ID == id {
			return p.Clone(), nil
		}
	}
	return nil, ErrProjectNotFound
}

// GetProjectByPath retrieves a project by its filesystem path
func (r *FileRegistry) GetProjectByPath(_ context.Context, path string) (*Project, error) {
	cleanPath := filepath.Clean(path)

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, ErrRegistryClosed
	}

	for _, p := range r.config.Projects {
		if filepath.Clean(p.Path) == cleanPath {
			return p.Clone(), nil
		}
	}
	return nil, ErrProjectNotFound
}

// AddProject registers a new project from the given path
func (r *FileRegistry) AddProject(_ context.Context, path string, opts *AddProjectOptions) (*Project, error) {
	// Ensure absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, NewRegistryError("add", fmt.Errorf("%w: %v", ErrInvalidPath, err))
	}
	absPath = filepath.Clean(absPath)

	// Validate the project path
	if err := ValidateProjectPath(absPath); err != nil {
		return nil, NewRegistryError("add", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, ErrRegistryClosed
	}

	// Check if already registered
	for _, p := range r.config.Projects {
		if filepath.Clean(p.Path) == absPath {
			return nil, ErrProjectAlreadyExists
		}
	}

	// Generate project details
	id := generateProjectID()
	name := ""
	color := ""

	if opts != nil {
		name = opts.Name
		color = opts.Color
	}

	if name == "" {
		name = generateProjectName(absPath)
	}
	if color == "" {
		color = generateProjectColor(id)
	}

	// Determine initial config mode.
	// If the project already has a config file, default to custom.
	// Otherwise, default to inheriting the global config.
	configMode := ConfigModeInheritGlobal
	configPath := filepath.Join(absPath, ".quorum", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		configMode = ConfigModeCustom
	}

	// Determine initial status
	// Missing project config is not an error when inheriting the global config.
	status := StatusHealthy
	statusMsg := ""

	now := time.Now()
	project := &Project{
		ID:            id,
		Path:          absPath,
		Name:          name,
		LastAccessed:  now,
		Status:        status,
		StatusMessage: statusMsg,
		Color:         color,
		CreatedAt:     now,
		ConfigMode:    configMode,
	}

	r.config.Projects = append(r.config.Projects, project)

	// Set as default if first project
	if r.config.DefaultProject == "" {
		r.config.DefaultProject = id
	}

	if r.autoSave {
		if err := r.save(); err != nil {
			// Rollback
			r.config.Projects = r.config.Projects[:len(r.config.Projects)-1]
			return nil, err
		}
	}

	r.logger.Info("project registered",
		"id", id,
		"name", name,
		"path", absPath,
		"status", status)

	return project.Clone(), nil
}

// RemoveProject unregisters a project by ID
func (r *FileRegistry) RemoveProject(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	index := -1
	prevDefault := r.config.DefaultProject
	for i, p := range r.config.Projects {
		if p.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return ErrProjectNotFound
	}

	removedProject := r.config.Projects[index]

	// Remove from slice
	r.config.Projects = append(r.config.Projects[:index], r.config.Projects[index+1:]...)
	r.removedIDs[id] = struct{}{}

	// Clear default if this was the default project
	if r.config.DefaultProject == id {
		if len(r.config.Projects) > 0 {
			r.config.DefaultProject = r.config.Projects[0].ID
		} else {
			r.config.DefaultProject = ""
		}
	}

	if r.autoSave {
		if err := r.save(); err != nil {
			// Rollback
			r.config.Projects = append(r.config.Projects[:index],
				append([]*Project{removedProject}, r.config.Projects[index:]...)...)
			r.config.DefaultProject = prevDefault
			delete(r.removedIDs, id)
			return err
		}
		delete(r.removedIDs, id)
	}

	r.logger.Info("project removed", "id", id, "name", removedProject.Name)
	return nil
}

// UpdateProject updates project metadata
func (r *FileRegistry) UpdateProject(_ context.Context, project *Project) error {
	if project == nil || project.ID == "" {
		return NewRegistryError("update", fmt.Errorf("invalid project"))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	for i, p := range r.config.Projects {
		if p.ID == project.ID {
			r.config.Projects[i] = project.Clone()
			if r.autoSave {
				return r.save()
			}
			return nil
		}
	}

	return ErrProjectNotFound
}

// ValidateProject checks if a project is still valid and accessible
func (r *FileRegistry) ValidateProject(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	var project *Project
	var index int
	for i, p := range r.config.Projects {
		if p.ID == id {
			project = p
			index = i
			break
		}
	}

	if project == nil {
		return ErrProjectNotFound
	}

	// Check if directory exists
	info, err := os.Stat(project.Path)
	if err != nil {
		project.Status = StatusOffline
		if os.IsNotExist(err) {
			project.StatusMessage = "Project directory not found"
		} else if os.IsPermission(err) {
			project.StatusMessage = "Permission denied accessing project directory"
		} else {
			project.StatusMessage = fmt.Sprintf("Cannot access project directory: %v", err)
		}
		r.config.Projects[index] = project
		if r.autoSave {
			_ = r.save()
		}
		return NewValidationError(id, project.Path, project.StatusMessage, err)
	}

	if !info.IsDir() {
		project.Status = StatusOffline
		project.StatusMessage = "Path is not a directory"
		r.config.Projects[index] = project
		if r.autoSave {
			_ = r.save()
		}
		return NewValidationError(id, project.Path, project.StatusMessage, nil)
	}

	// Check .quorum directory
	quorumDir := filepath.Join(project.Path, ".quorum")
	if _, err := os.Stat(quorumDir); err != nil {
		project.Status = StatusOffline
		project.StatusMessage = ".quorum directory not found"
		r.config.Projects[index] = project
		if r.autoSave {
			_ = r.save()
		}
		return NewValidationError(id, project.Path, project.StatusMessage, err)
	}

	// Check config file
	configPath := filepath.Join(quorumDir, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		// Missing project config is only a warning when the project is in custom mode.
		// In inherit_global mode, the effective config comes from the global config.
		if project.ConfigMode == ConfigModeCustom {
			project.Status = StatusDegraded
			project.StatusMessage = "Project configuration file not found or inaccessible"
		} else {
			project.Status = StatusHealthy
			project.StatusMessage = ""
		}
		r.config.Projects[index] = project
		if r.autoSave {
			_ = r.save()
		}
		// Degraded is not an error, just a warning
		return nil
	}

	// All checks passed
	project.Status = StatusHealthy
	project.StatusMessage = ""
	r.config.Projects[index] = project
	if r.autoSave {
		_ = r.save()
	}

	return nil
}

// ValidateAll validates all registered projects and updates their status
func (r *FileRegistry) ValidateAll(ctx context.Context) error {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return ErrRegistryClosed
	}

	ids := make([]string, len(r.config.Projects))
	for i, p := range r.config.Projects {
		ids[i] = p.ID
	}
	r.mu.RUnlock()

	var lastErr error
	for _, id := range ids {
		if err := r.ValidateProject(ctx, id); err != nil {
			lastErr = err
			r.logger.Warn("project validation failed", "id", id, "error", err)
		}
	}
	return lastErr
}

// GetDefaultProject returns the default project for legacy endpoints
func (r *FileRegistry) GetDefaultProject(_ context.Context) (*Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, ErrRegistryClosed
	}

	if r.config.DefaultProject == "" {
		if len(r.config.Projects) == 0 {
			return nil, ErrNoDefaultProject
		}
		// Return first project if no default set
		return r.config.Projects[0].Clone(), nil
	}

	for _, p := range r.config.Projects {
		if p.ID == r.config.DefaultProject {
			return p.Clone(), nil
		}
	}

	// Default project no longer exists, return first
	if len(r.config.Projects) > 0 {
		return r.config.Projects[0].Clone(), nil
	}

	return nil, ErrNoDefaultProject
}

// SetDefaultProject sets the default project for legacy endpoints
func (r *FileRegistry) SetDefaultProject(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	// Verify project exists
	found := false
	for _, p := range r.config.Projects {
		if p.ID == id {
			found = true
			break
		}
	}

	if !found {
		return ErrProjectNotFound
	}

	r.config.DefaultProject = id

	if r.autoSave {
		return r.save()
	}
	return nil
}

// TouchProject updates the last accessed time for a project
func (r *FileRegistry) TouchProject(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return ErrRegistryClosed
	}

	for _, p := range r.config.Projects {
		if p.ID == id {
			p.LastAccessed = time.Now()
			if r.autoSave {
				return r.save()
			}
			return nil
		}
	}

	return ErrProjectNotFound
}

// Close releases any resources held by the registry
func (r *FileRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	// Final save
	return r.save()
}

// Reload reloads the registry from disk
func (r *FileRegistry) Reload() error {
	return r.load()
}

// Count returns the number of registered projects
func (r *FileRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.config.Projects)
}

// validateProjectPath validates that a path is a valid Quorum project
// ValidateProjectPath checks that a path is absolute, clean, and contains a .quorum directory.
func ValidateProjectPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: path must be absolute", ErrInvalidPath)
	}

	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("%w: path contains invalid sequences", ErrInvalidPath)
	}

	quorumDir := filepath.Join(cleanPath, ".quorum")
	info, err := os.Stat(quorumDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: .quorum directory not found at %s", ErrNotQuorumProject, path)
		}
		return fmt.Errorf("%w: cannot access .quorum directory: %v", ErrProjectOffline, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: .quorum is not a directory", ErrNotQuorumProject)
	}

	return nil
}

// generateProjectID creates a cryptographically random project ID
func generateProjectID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("proj-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("proj-%s", hex.EncodeToString(b)[:12])
}

// generateProjectName creates a human-readable name from a path
func generateProjectName(path string) string {
	name := filepath.Base(path)
	if name != "" {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return name
}

// generateProjectColor creates a consistent color from a project ID
func generateProjectColor(id string) string {
	hash := sha256.Sum256([]byte(id))
	index := int(hash[0]) % len(projectColors)
	return projectColors[index]
}
