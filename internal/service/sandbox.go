package service

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Sandbox provides a restricted execution environment.
type Sandbox struct {
	workspaceRoot string
	allowedPaths  []string
	deniedPaths   []string
}

// NewSandbox creates a new sandbox.
func NewSandbox(workspaceRoot string) *Sandbox {
	absRoot, _ := filepath.Abs(workspaceRoot)
	return &Sandbox{
		workspaceRoot: absRoot,
		allowedPaths:  []string{absRoot},
		deniedPaths: []string{
			"/etc",
			"/usr",
			"/bin",
			"/sbin",
			filepath.Join(os.Getenv("HOME"), ".ssh"),
			filepath.Join(os.Getenv("HOME"), ".gnupg"),
			filepath.Join(os.Getenv("HOME"), ".aws"),
		},
	}
}

// AllowPath adds a path to the allowed list.
func (s *Sandbox) AllowPath(path string) {
	absPath, _ := filepath.Abs(path)
	s.allowedPaths = append(s.allowedPaths, absPath)
}

// DenyPath adds a path to the denied list.
func (s *Sandbox) DenyPath(path string) {
	absPath, _ := filepath.Abs(path)
	s.deniedPaths = append(s.deniedPaths, absPath)
}

// IsPathAllowed checks if a path is allowed.
func (s *Sandbox) IsPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// Check denied paths first
	for _, denied := range s.deniedPaths {
		if strings.HasPrefix(absPath, denied) {
			return false
		}
	}

	// Check allowed paths
	for _, allowed := range s.allowedPaths {
		if strings.HasPrefix(absPath, allowed) {
			return true
		}
	}

	return false
}

// WorkspaceRoot returns the workspace root path.
func (s *Sandbox) WorkspaceRoot() string {
	return s.workspaceRoot
}

// ValidateOperation validates an operation in sandbox context.
func (s *Sandbox) ValidateOperation(op Operation) error {
	switch op.Type {
	case OpTypeFileWrite:
		// Would need actual path to validate
		if !op.InWorkspace {
			return core.ErrValidation("SANDBOX_VIOLATION", "writes restricted to workspace")
		}
	case OpTypeShell:
		if op.IsDestructive {
			return core.ErrValidation("SANDBOX_VIOLATION", "destructive shell commands blocked")
		}
	}

	return nil
}

// ValidatePath validates if a path can be accessed.
func (s *Sandbox) ValidatePath(path string, write bool) error {
	if !s.IsPathAllowed(path) {
		if write {
			return core.ErrValidation("SANDBOX_VIOLATION",
				"write access denied to path outside workspace")
		}
		return core.ErrValidation("SANDBOX_VIOLATION",
			"read access denied to path outside workspace")
	}
	return nil
}

// SafeCommands returns a list of commands safe for sandbox execution.
func SafeCommands() []string {
	return []string{
		"ls", "cat", "head", "tail", "grep", "find", "wc",
		"git status", "git diff", "git log", "git branch", "git show",
		"go build", "go test", "go fmt", "go vet", "go mod",
		"npm test", "npm run lint", "npm run build",
		"make check", "make test", "make build",
		"cargo build", "cargo test", "cargo check",
		"python -m pytest", "python -m mypy",
	}
}

// DangerousPatterns returns patterns that indicate dangerous operations.
func DangerousPatterns() []string {
	return []string{
		"rm -rf",
		"rm -fr",
		"git push --force",
		"git push -f",
		"git reset --hard",
		"DROP TABLE",
		"DELETE FROM",
		"> /dev/",
		">> /dev/",
		"chmod 777",
		"chmod -R 777",
		"curl | sh",
		"curl | bash",
		"wget | sh",
		"wget | bash",
		":(){ :|:& };:",
		"mkfs",
		"dd if=",
	}
}

// IsDangerousCommand checks if a command matches dangerous patterns.
func IsDangerousCommand(cmd string) bool {
	lowerCmd := strings.ToLower(cmd)
	for _, pattern := range DangerousPatterns() {
		if strings.Contains(lowerCmd, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// IsSafeCommand checks if a command is in the safe list.
func IsSafeCommand(cmd string) bool {
	lowerCmd := strings.ToLower(strings.TrimSpace(cmd))
	for _, safe := range SafeCommands() {
		if strings.HasPrefix(lowerCmd, strings.ToLower(safe)) {
			return true
		}
	}
	return false
}
