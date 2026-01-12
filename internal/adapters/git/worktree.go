package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// resolvePath resolves symlinks and returns an absolute path.
// This is needed for cross-platform path comparison (e.g., macOS /var -> /private/var).
func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If we can't resolve, return absolute path
		abs, err := filepath.Abs(path)
		if err != nil {
			return path
		}
		return abs
	}
	return resolved
}

// WorktreeManager manages git worktrees.
type WorktreeManager struct {
	git     *Client
	baseDir string
	prefix  string
}

// NewWorktreeManager creates a new worktree manager.
func NewWorktreeManager(git *Client, baseDir string) *WorktreeManager {
	if baseDir == "" {
		baseDir = filepath.Join(git.RepoPath(), ".worktrees")
	}

	return &WorktreeManager{
		git:     git,
		baseDir: baseDir,
		prefix:  "quorum-",
	}
}

// Worktree represents a git worktree.
type Worktree struct {
	Path      string
	Branch    string
	Commit    string
	Detached  bool
	Locked    bool
	Prunable  bool
	CreatedAt time.Time
}

// Create creates a new worktree for a branch.
func (m *WorktreeManager) Create(ctx context.Context, name, branch string) (*Worktree, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating worktree directory: %w", err)
	}

	// Generate worktree path
	worktreePath := filepath.Join(m.baseDir, m.prefix+name)

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return nil, core.ErrValidation("WORKTREE_EXISTS",
			fmt.Sprintf("worktree %s already exists", name))
	}

	// Determine if branch exists
	branches, err := m.git.ListBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	branchExists := false
	for _, b := range branches {
		if b == branch {
			branchExists = true
			break
		}
	}

	// Create worktree
	var args []string
	if branchExists {
		args = []string{"worktree", "add", worktreePath, branch}
	} else {
		// Create new branch from current HEAD
		args = []string{"worktree", "add", "-b", branch, worktreePath}
	}

	_, err = m.git.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}

	// Get worktree info
	worktrees, err := m.List(ctx)
	if err != nil {
		return nil, err
	}

	resolvedPath := resolvePath(worktreePath)
	for _, wt := range worktrees {
		if resolvePath(wt.Path) == resolvedPath {
			wt.CreatedAt = time.Now()
			return &wt, nil
		}
	}

	return &Worktree{
		Path:      worktreePath,
		Branch:    branch,
		CreatedAt: time.Now(),
	}, nil
}

// CreateFromCommit creates a detached worktree from a commit.
func (m *WorktreeManager) CreateFromCommit(ctx context.Context, name, commit string) (*Worktree, error) {
	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating worktree directory: %w", err)
	}

	worktreePath := filepath.Join(m.baseDir, m.prefix+name)

	if _, err := os.Stat(worktreePath); err == nil {
		return nil, core.ErrValidation("WORKTREE_EXISTS",
			fmt.Sprintf("worktree %s already exists", name))
	}

	_, err := m.git.run(ctx, "worktree", "add", "--detach", worktreePath, commit)
	if err != nil {
		return nil, fmt.Errorf("creating detached worktree: %w", err)
	}

	return &Worktree{
		Path:      worktreePath,
		Commit:    commit,
		Detached:  true,
		CreatedAt: time.Now(),
	}, nil
}

// Remove removes a worktree.
func (m *WorktreeManager) Remove(ctx context.Context, path string, force bool) error {
	// Check if path is within our base directory (using resolved paths for cross-platform)
	resolvedPath := resolvePath(path)
	resolvedBase := resolvePath(m.baseDir)
	if !strings.HasPrefix(resolvedPath, resolvedBase) {
		return core.ErrValidation("INVALID_WORKTREE",
			"worktree is not managed by this manager")
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	_, err := m.git.run(ctx, args...)
	return err
}

// List returns all worktrees.
func (m *WorktreeManager) List(ctx context.Context) ([]Worktree, error) {
	output, err := m.git.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	return m.parseWorktreeList(output), nil
}

// parseWorktreeList parses git worktree list output.
func (m *WorktreeManager) parseWorktreeList(output string) []Worktree {
	worktrees := make([]Worktree, 0)
	var current *Worktree

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "worktree ") {
			if current != nil {
				worktrees = append(worktrees, *current)
			}
			current = &Worktree{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if current != nil {
			if strings.HasPrefix(line, "HEAD ") {
				current.Commit = strings.TrimPrefix(line, "HEAD ")
			} else if strings.HasPrefix(line, "branch ") {
				current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			} else if line == "detached" {
				current.Detached = true
			} else if line == "locked" {
				current.Locked = true
			} else if line == "prunable" {
				current.Prunable = true
			}
		}
	}

	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees
}

// ListManaged returns only worktrees created by this manager.
func (m *WorktreeManager) ListManaged(ctx context.Context) ([]Worktree, error) {
	all, err := m.List(ctx)
	if err != nil {
		return nil, err
	}

	resolvedBase := resolvePath(m.baseDir)
	managed := make([]Worktree, 0)
	for _, wt := range all {
		if strings.HasPrefix(resolvePath(wt.Path), resolvedBase) {
			managed = append(managed, wt)
		}
	}
	return managed, nil
}

// Get returns a specific worktree.
func (m *WorktreeManager) Get(ctx context.Context, name string) (*Worktree, error) {
	path := filepath.Join(m.baseDir, m.prefix+name)

	worktrees, err := m.List(ctx)
	if err != nil {
		return nil, err
	}

	resolvedPath := resolvePath(path)
	for _, wt := range worktrees {
		if resolvePath(wt.Path) == resolvedPath {
			return &wt, nil
		}
	}

	return nil, core.ErrNotFound("worktree", name)
}

// Lock locks a worktree to prevent accidental removal.
func (m *WorktreeManager) Lock(ctx context.Context, path, reason string) error {
	args := []string{"worktree", "lock", path}
	if reason != "" {
		args = append(args, "--reason", reason)
	}
	_, err := m.git.run(ctx, args...)
	return err
}

// Unlock unlocks a worktree.
func (m *WorktreeManager) Unlock(ctx context.Context, path string) error {
	_, err := m.git.run(ctx, "worktree", "unlock", path)
	return err
}

// Prune removes stale worktree entries.
func (m *WorktreeManager) Prune(ctx context.Context, dryRun bool) ([]string, error) {
	args := []string{"worktree", "prune"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, "--verbose")

	output, err := m.git.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Parse pruned paths
	pruned := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Removing") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pruned = append(pruned, parts[1])
			}
		}
	}

	return pruned, nil
}

// CleanupStale removes all stale worktrees created by this manager.
func (m *WorktreeManager) CleanupStale(ctx context.Context, maxAge time.Duration) (int, error) {
	managed, err := m.ListManaged(ctx)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	now := time.Now()

	for _, wt := range managed {
		// Check if directory still exists
		info, err := os.Stat(wt.Path)
		if os.IsNotExist(err) {
			continue
		}

		// Check age based on modification time
		if info != nil && maxAge > 0 {
			age := now.Sub(info.ModTime())
			if age < maxAge {
				continue
			}
		}

		// Remove if prunable or forced by age
		if wt.Prunable || (maxAge > 0 && info != nil) {
			if err := m.Remove(ctx, wt.Path, true); err == nil {
				cleaned++
			}
		}
	}

	// Also run git prune
	m.Prune(ctx, false)

	return cleaned, nil
}

// CreateClient creates a git client for a worktree.
func (m *WorktreeManager) CreateClient(worktreePath string) (*Client, error) {
	return NewClient(worktreePath)
}

// BaseDir returns the base directory for worktrees.
func (m *WorktreeManager) BaseDir() string {
	return m.baseDir
}

// WithPrefix sets a custom prefix for worktree names.
func (m *WorktreeManager) WithPrefix(prefix string) *WorktreeManager {
	m.prefix = prefix
	return m
}
