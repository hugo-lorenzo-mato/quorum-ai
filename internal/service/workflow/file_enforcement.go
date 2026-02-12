package workflow

import (
	"fmt"
	"os"
	"path/filepath"
)

// fileEnforcementLogger is a minimal interface for logging in FileEnforcement.
type fileEnforcementLogger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
}

// FileEnforcement provides utilities to ensure files exist before creating checkpoints.
// This prevents the moderator from failing due to "Missing analysis files" errors.
type FileEnforcement struct {
	logger fileEnforcementLogger
}

// NewFileEnforcement creates a new FileEnforcement instance.
func NewFileEnforcement(logger fileEnforcementLogger) *FileEnforcement {
	return &FileEnforcement{logger: logger}
}

// EnsureDirectory creates the parent directory of filePath if it doesn't exist.
// This should be called BEFORE executing an agent that is expected to write to filePath.
func (fe *FileEnforcement) EnsureDirectory(filePath string) error {
	if filePath == "" {
		return nil
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	return nil
}

// VerifyOrWriteFallback verifies that filePath exists after agent execution.
// If the file doesn't exist but stdout has content, it writes stdout as a fallback.
// Returns (true, nil) if the file was created by the LLM.
// Returns (false, nil) if the file was created as a fallback.
// Returns (false, error) if the file couldn't be created.
func (fe *FileEnforcement) VerifyOrWriteFallback(filePath, stdout string) (createdByLLM bool, err error) {
	if filePath == "" {
		return false, nil
	}

	// Check if file already exists (created by LLM)
	if _, err := os.Stat(filePath); err == nil {
		if fe.logger != nil {
			fe.logger.Debug("file exists (created by LLM)", "path", filePath)
		}
		return true, nil
	}

	// File doesn't exist - try to write fallback from stdout
	if stdout == "" {
		return false, fmt.Errorf("file not created and no stdout available: %s", filePath)
	}

	// Ensure directory exists
	if err := fe.EnsureDirectory(filePath); err != nil {
		return false, err
	}

	// Write stdout as fallback
	if err := os.WriteFile(filePath, []byte(stdout), 0o600); err != nil {
		return false, fmt.Errorf("writing fallback file: %w", err)
	}

	if fe.logger != nil {
		fe.logger.Info("created fallback file from stdout", "path", filePath)
	}
	return false, nil
}

// ValidateBeforeCheckpoint validates that filePath exists before creating a checkpoint.
// This prevents creating checkpoints that reference non-existent files.
func (fe *FileEnforcement) ValidateBeforeCheckpoint(filePath string) error {
	if filePath == "" {
		return nil
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("cannot create checkpoint: file does not exist: %s", filePath)
	}
	return nil
}

// EnsureAndVerify is a convenience method that combines EnsureDirectory and VerifyOrWriteFallback.
// It's meant to be called in two phases:
// 1. Before execution: call with ensureOnly=true to create the directory
// 2. After execution: call with ensureOnly=false to verify/create the file
func (fe *FileEnforcement) EnsureAndVerify(filePath, stdout string, ensureOnly bool) (createdByLLM bool, err error) {
	if ensureOnly {
		return false, fe.EnsureDirectory(filePath)
	}
	return fe.VerifyOrWriteFallback(filePath, stdout)
}
