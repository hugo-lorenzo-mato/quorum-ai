// Package project provides multi-project management capabilities for Quorum AI.
// It includes a registry for tracking projects, context encapsulation for
// project-specific resources, and a pool for managing active project contexts.
package project

import (
	"time"
)

// ProjectStatus represents the health state of a project
type ProjectStatus string

const (
	// StatusHealthy indicates the project is fully operational
	StatusHealthy ProjectStatus = "healthy"
	// StatusDegraded indicates partial functionality (e.g., config issues)
	StatusDegraded ProjectStatus = "degraded"
	// StatusOffline indicates the project directory is not accessible
	StatusOffline ProjectStatus = "offline"
	// StatusInitializing indicates first-time setup is in progress
	StatusInitializing ProjectStatus = "initializing"
)

// Config modes for project configuration inheritance.
const (
	ConfigModeInheritGlobal = "inherit_global"
	ConfigModeCustom        = "custom"
)

// String returns the string representation of the status
func (s ProjectStatus) String() string {
	return string(s)
}

// IsValid checks if the status is a valid value
func (s ProjectStatus) IsValid() bool {
	switch s {
	case StatusHealthy, StatusDegraded, StatusOffline, StatusInitializing:
		return true
	default:
		return false
	}
}

// Project represents a registered Quorum project
type Project struct {
	// ID is the unique identifier for the project (cryptographically random)
	ID string `yaml:"id" json:"id"`
	// Path is the absolute filesystem path to the project root
	Path string `yaml:"path" json:"path"`
	// Name is the human-readable name of the project
	Name string `yaml:"name" json:"name"`
	// LastAccessed is the timestamp of the last access to this project
	LastAccessed time.Time `yaml:"last_accessed" json:"last_accessed"`
	// Status indicates the current health state of the project
	Status ProjectStatus `yaml:"status" json:"status"`
	// StatusMessage provides additional context for non-healthy statuses
	StatusMessage string `yaml:"status_message,omitempty" json:"status_message,omitempty"`
	// Color is the UI accent color for the project badge
	Color string `yaml:"color,omitempty" json:"color,omitempty"`
	// CreatedAt is the timestamp when the project was registered
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	// ConfigMode controls whether this project uses the global config (inherit_global)
	// or a project-specific config file (custom).
	//
	// Values:
	// - "inherit_global": use global config as the effective config for this project.
	// - "custom": use <project>/.quorum/config.yaml as the effective config.
	//
	// If empty, callers should infer a default (e.g., custom if project config exists).
	ConfigMode string `yaml:"config_mode,omitempty" json:"config_mode,omitempty"`
	// Enabled controls whether the project is active. Disabled projects are skipped
	// by the state pool, kanban engine, and orphan cleanup. Pointer type so that
	// nil (missing from YAML) defaults to enabled for backward compatibility.
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// Clone creates a deep copy of the project
func (p *Project) Clone() *Project {
	if p == nil {
		return nil
	}
	clone := &Project{
		ID:            p.ID,
		Path:          p.Path,
		Name:          p.Name,
		LastAccessed:  p.LastAccessed,
		Status:        p.Status,
		StatusMessage: p.StatusMessage,
		Color:         p.Color,
		CreatedAt:     p.CreatedAt,
		ConfigMode:    p.ConfigMode,
	}
	if p.Enabled != nil {
		v := *p.Enabled
		clone.Enabled = &v
	}
	return clone
}

// IsEnabled returns true if the project is enabled.
// A nil Enabled field (missing from YAML) is treated as enabled for backward compatibility.
func (p *Project) IsEnabled() bool {
	if p == nil {
		return false
	}
	return p.Enabled == nil || *p.Enabled
}

// IsHealthy returns true if the project status is healthy
func (p *Project) IsHealthy() bool {
	return p != nil && p.Status == StatusHealthy
}

// IsAccessible returns true if the project can be accessed (healthy or degraded)
func (p *Project) IsAccessible() bool {
	return p != nil && (p.Status == StatusHealthy || p.Status == StatusDegraded)
}

// RegistryConfig holds the persisted registry data
type RegistryConfig struct {
	// Version is the schema version of the registry file
	Version int `yaml:"version"`
	// DefaultProject is the ID of the default project for legacy endpoints
	DefaultProject string `yaml:"default_project,omitempty"`
	// Projects is the list of all registered projects
	Projects []*Project `yaml:"projects"`
}

// AddProjectOptions provides options when adding a project
type AddProjectOptions struct {
	// Name is the custom name for the project (auto-generated from path if empty)
	Name string
	// Color is the custom UI color (auto-generated if empty)
	Color string
}
