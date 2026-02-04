package project

import (
	"errors"
	"fmt"
)

// Sentinel errors for project operations
var (
	// ErrProjectNotFound indicates the requested project doesn't exist in registry
	ErrProjectNotFound = errors.New("project not found")

	// ErrProjectAlreadyExists indicates a project with the same path is already registered
	ErrProjectAlreadyExists = errors.New("project already registered")

	// ErrNotQuorumProject indicates the path doesn't contain a valid .quorum directory
	ErrNotQuorumProject = errors.New("not a valid quorum project")

	// ErrProjectOffline indicates the project directory is not accessible
	ErrProjectOffline = errors.New("project directory not accessible")

	// ErrInvalidPath indicates the provided path is invalid
	ErrInvalidPath = errors.New("invalid project path")

	// ErrRegistryCorrupted indicates the registry file is corrupted
	ErrRegistryCorrupted = errors.New("registry file corrupted")

	// ErrNoDefaultProject indicates no default project is configured
	ErrNoDefaultProject = errors.New("no default project configured")

	// ErrRegistryClosed indicates the registry has been closed
	ErrRegistryClosed = errors.New("registry is closed")
)

// ProjectValidationError provides detailed validation failure information
type ProjectValidationError struct {
	ProjectID string
	Path      string
	Reason    string
	Err       error
}

// Error returns the error message
func (e *ProjectValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("project validation failed for %s (%s): %s: %v",
			e.ProjectID, e.Path, e.Reason, e.Err)
	}
	return fmt.Sprintf("project validation failed for %s (%s): %s",
		e.ProjectID, e.Path, e.Reason)
}

// Unwrap returns the underlying error
func (e *ProjectValidationError) Unwrap() error {
	return e.Err
}

// RegistryError wraps registry operation errors with context
type RegistryError struct {
	Op  string // Operation that failed (e.g., "load", "save", "add")
	Err error
}

// Error returns the error message
func (e *RegistryError) Error() string {
	return fmt.Sprintf("registry %s failed: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *RegistryError) Unwrap() error {
	return e.Err
}

// NewRegistryError creates a new RegistryError
func NewRegistryError(op string, err error) *RegistryError {
	return &RegistryError{Op: op, Err: err}
}

// NewValidationError creates a new ProjectValidationError
func NewValidationError(projectID, path, reason string, err error) *ProjectValidationError {
	return &ProjectValidationError{
		ProjectID: projectID,
		Path:      path,
		Reason:    reason,
		Err:       err,
	}
}
