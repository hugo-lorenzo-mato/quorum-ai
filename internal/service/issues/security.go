package issues

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Security-related errors.
var (
	// ErrPathTraversal indicates an attempted path traversal attack.
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrInvalidFilename indicates an invalid filename.
	ErrInvalidFilename = errors.New("invalid filename")

	// ErrAbsolutePath indicates an absolute path was provided where relative was expected.
	ErrAbsolutePath = errors.New("absolute path not allowed")
)

// ValidateOutputPath validates that a filename is safe to write within the given directory.
// It prevents path traversal attacks by ensuring the resulting path is within the allowed directory.
//
// Returns the validated full path or an error if the path is unsafe.
func ValidateOutputPath(baseDir, filename string) (string, error) {
	// Reject empty filename
	if filename == "" {
		return "", fmt.Errorf("%w: filename is empty", ErrInvalidFilename)
	}

	// Clean the filename (normalizes path separators, removes redundant elements)
	cleanName := filepath.Clean(filename)

	// Reject absolute paths
	if filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("%w: %s", ErrAbsolutePath, filename)
	}

	// Reject paths with directory traversal
	if strings.Contains(cleanName, "..") {
		return "", fmt.Errorf("%w: %s contains parent directory reference", ErrPathTraversal, filename)
	}

	// Reject paths that try to escape via multiple components
	// e.g., "foo/../../bar" would clean to "../bar"
	if strings.HasPrefix(cleanName, "..") {
		return "", fmt.Errorf("%w: %s attempts to escape base directory", ErrPathTraversal, filename)
	}

	// Build the full path
	fullPath := filepath.Join(baseDir, cleanName)

	// Get absolute paths for comparison
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base directory: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolving full path: %w", err)
	}

	// Ensure the full path is within the base directory
	// Add trailing separator to prevent prefix matching issues
	// (e.g., /base/dir matching /base/directory)
	baseDirWithSep := absBaseDir + string(filepath.Separator)
	if !strings.HasPrefix(absFullPath+string(filepath.Separator), baseDirWithSep) &&
		absFullPath != absBaseDir {
		return "", fmt.Errorf("%w: %s is outside allowed directory %s", ErrPathTraversal, filename, baseDir)
	}

	return fullPath, nil
}

// SanitizeFilename removes or replaces potentially dangerous characters from a filename.
// This is a defensive measure to ensure filenames are safe for the filesystem.
func SanitizeFilename(filename string) string {
	// Get just the filename, not any directory components
	filename = filepath.Base(filename)

	// Replace potentially dangerous characters
	dangerous := []string{
		"..", // Parent directory reference
		"/",  // Path separator (Unix)
		"\\", // Path separator (Windows)
		":",  // Drive separator (Windows) / alternative data streams
		"*",  // Wildcard
		"?",  // Wildcard
		"\"", // Quote
		"<",  // Redirect
		">",  // Redirect
		"|",  // Pipe
		"\x00", // Null byte
	}

	result := filename
	for _, char := range dangerous {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Collapse multiple underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	// Trim leading/trailing underscores and whitespace
	result = strings.Trim(result, "_ \t\n\r")

	// If the filename is now empty, use a default
	if result == "" {
		result = "unnamed"
	}

	return result
}

// IsValidIssueFilename checks if a filename is valid for an issue file.
// Valid issue files must:
// - End with .md extension
// - Not be hidden (start with .)
// - Not contain path separators
// - Be within reasonable length
func IsValidIssueFilename(filename string) bool {
	// Must have .md extension
	if !strings.HasSuffix(strings.ToLower(filename), ".md") {
		return false
	}

	// Must not be hidden
	if strings.HasPrefix(filename, ".") {
		return false
	}

	// Must not contain path separators
	if strings.ContainsAny(filename, "/\\") {
		return false
	}

	// Must be reasonable length (not too short, not too long)
	if len(filename) < 4 || len(filename) > 255 {
		return false
	}

	return true
}

// ValidateAndSanitizeFilename combines validation and sanitization.
// Returns the sanitized filename and any error if the filename is fundamentally invalid.
func ValidateAndSanitizeFilename(filename string) (string, error) {
	// First sanitize
	sanitized := SanitizeFilename(filename)

	// Ensure .md extension
	if !strings.HasSuffix(strings.ToLower(sanitized), ".md") {
		sanitized += ".md"
	}

	// Validate the sanitized result
	if !IsValidIssueFilename(sanitized) {
		return "", fmt.Errorf("%w: %s could not be made valid", ErrInvalidFilename, filename)
	}

	return sanitized, nil
}
