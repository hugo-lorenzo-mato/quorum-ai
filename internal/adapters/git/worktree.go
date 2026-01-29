package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// Compile-time interface conformance check.
var _ core.WorktreeManager = (*TaskWorktreeManager)(nil)

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

const (
	worktreeNameSeparator = "__"
	worktreeLabelMaxLen   = 48
)

func validateTaskID(taskID string) error {
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		return core.ErrValidation("WORKTREE_TASK_ID_REQUIRED", "task id required for worktree")
	}
	if strings.Contains(trimmed, worktreeNameSeparator) {
		return core.ErrValidation("WORKTREE_TASK_ID_INVALID", "task id must not contain '__'")
	}
	if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, "/\\") {
		return core.ErrValidation("WORKTREE_TASK_ID_INVALID", "task id contains invalid path characters")
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return core.ErrValidation("WORKTREE_TASK_ID_INVALID", "task id contains invalid characters")
	}
	return nil
}

func normalizeLabel(input string, maxLen int) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	lastDash := false
	for _, r := range trimmed {
		if r >= 'A' && r <= 'Z' {
			r = r - 'A' + 'a'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
		if maxLen > 0 && b.Len() >= maxLen {
			break
		}
	}

	return strings.Trim(b.String(), "-")
}

func buildWorktreeName(task *core.Task) (name string, ok bool, err error) {
	if task == nil {
		return "", false, core.ErrValidation("WORKTREE_TASK_REQUIRED", "task required to create worktree")
	}
	taskID := strings.TrimSpace(string(task.ID))
	if err := validateTaskID(taskID); err != nil {
		return "", false, err
	}

	labelSource := strings.TrimSpace(task.Name)
	if labelSource == "" {
		labelSource = strings.TrimSpace(task.Description)
	}
	if labelSource == "" {
		return "", false, core.ErrValidation("WORKTREE_TASK_LABEL_REQUIRED", "task name or description required for worktree naming")
	}

	label := normalizeLabel(labelSource, worktreeLabelMaxLen)
	if label == "" {
		if err := validateWorktreeName(taskID); err != nil {
			return "", false, err
		}
		return taskID, true, nil
	}

	name = taskID + worktreeNameSeparator + label
	if err := validateWorktreeName(name); err != nil {
		return "", false, err
	}
	return name, false, nil
}

func validateWorktreeName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return core.ErrValidation("WORKTREE_NAME_REQUIRED", "worktree name required")
	}
	if strings.Contains(trimmed, "..") || strings.ContainsAny(trimmed, "/\\") {
		return core.ErrValidation("WORKTREE_NAME_INVALID", "worktree name contains invalid path characters")
	}
	return nil
}

func validateWorktreeBranch(branch string) error {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" {
		return core.ErrValidation("WORKTREE_BRANCH_REQUIRED", "worktree branch required")
	}
	if strings.Contains(trimmed, " ") || strings.Contains(trimmed, "..") {
		return core.ErrValidation("WORKTREE_BRANCH_INVALID", "worktree branch contains invalid characters")
	}
	return nil
}

func resolveWorktreeBranch(name, branch string) (string, error) {
	candidate := strings.TrimSpace(branch)
	if candidate == "" {
		candidate = "quorum/" + name
	}
	if err := validateWorktreeBranch(candidate); err != nil {
		return "", err
	}
	return candidate, nil
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
	return m.CreateFromBranch(ctx, name, branch, "")
}

// CreateFromBranch creates a new worktree for a branch, optionally from a base branch.
// If baseBranch is empty and the branch doesn't exist, it will be created from HEAD.
// If baseBranch is specified and the branch doesn't exist, it will be created from baseBranch.
func (m *WorktreeManager) CreateFromBranch(ctx context.Context, name, branch, baseBranch string) (*Worktree, error) {
	if err := validateWorktreeName(name); err != nil {
		return nil, err
	}
	if err := validateWorktreeBranch(branch); err != nil {
		return nil, err
	}

	// Ensure base directory exists
	if err := os.MkdirAll(m.baseDir, 0o750); err != nil {
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
		// Create new branch
		if baseBranch != "" {
			// Create from specified base branch (for dependencies)
			args = []string{"worktree", "add", "-b", branch, worktreePath, baseBranch}
		} else {
			// Create from current HEAD
			args = []string{"worktree", "add", "-b", branch, worktreePath}
		}
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
	if err := validateWorktreeName(name); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(m.baseDir, 0o750); err != nil {
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

		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil {
				worktrees = append(worktrees, *current)
			}
			current = &Worktree{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		case current != nil:
			switch {
			case strings.HasPrefix(line, "HEAD "):
				current.Commit = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			case line == "detached":
				current.Detached = true
			case line == "locked":
				current.Locked = true
			case line == "prunable":
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

	// Also run git prune (errors are non-fatal for cleanup)
	_, _ = m.Prune(ctx, false)

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

// =============================================================================
// TaskWorktreeManager - implements core.WorktreeManager
// =============================================================================

// TaskWorktreeManager wraps WorktreeManager to implement core.WorktreeManager.
// It provides TaskID-based worktree management on top of the low-level WorktreeManager.
type TaskWorktreeManager struct {
	manager *WorktreeManager
	logger  *logging.Logger
}

// NewTaskWorktreeManager creates a new task-aware worktree manager.
func NewTaskWorktreeManager(git *Client, baseDir string) *TaskWorktreeManager {
	return &TaskWorktreeManager{
		manager: NewWorktreeManager(git, baseDir),
		logger:  logging.NewNop(),
	}
}

// Create creates a new worktree for a task (implements core.WorktreeManager).
func (m *TaskWorktreeManager) Create(ctx context.Context, task *core.Task, branch string) (*core.WorktreeInfo, error) {
	return m.CreateFromBranch(ctx, task, branch, "")
}

// CreateFromBranch creates a new worktree for a task from a specified base branch.
// This is useful for dependent tasks that need to start from another task's branch.
func (m *TaskWorktreeManager) CreateFromBranch(ctx context.Context, task *core.Task, branch, baseBranch string) (*core.WorktreeInfo, error) {
	name, usedFallback, err := buildWorktreeName(task)
	if err != nil {
		return nil, err
	}
	if usedFallback {
		m.logger.Warn("worktree label normalized empty, using task id only",
			"task_id", task.ID,
			"task_name", task.Name,
			"task_description", task.Description,
			"worktree_name", name,
		)
	}
	resolvedBranch, err := resolveWorktreeBranch(name, branch)
	if err != nil {
		return nil, err
	}
	wt, err := m.manager.CreateFromBranch(ctx, name, resolvedBranch, baseBranch)
	if err != nil {
		return nil, err
	}

	return &core.WorktreeInfo{
		TaskID:    task.ID,
		Path:      wt.Path,
		Branch:    wt.Branch,
		CreatedAt: wt.CreatedAt,
		Status:    core.WorktreeStatusActive,
	}, nil
}

// Get retrieves worktree info for a task (implements core.WorktreeManager).
func (m *TaskWorktreeManager) Get(ctx context.Context, task *core.Task) (*core.WorktreeInfo, error) {
	name, _, err := buildWorktreeName(task)
	if err != nil {
		return nil, err
	}
	wt, err := m.manager.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	status := core.WorktreeStatusActive
	if wt.Prunable {
		status = core.WorktreeStatusStale
	}

	return &core.WorktreeInfo{
		TaskID:    task.ID,
		Path:      wt.Path,
		Branch:    wt.Branch,
		CreatedAt: wt.CreatedAt,
		Status:    status,
	}, nil
}

// Remove cleans up a task's worktree (implements core.WorktreeManager).
func (m *TaskWorktreeManager) Remove(ctx context.Context, task *core.Task) error {
	name, _, err := buildWorktreeName(task)
	if err != nil {
		return err
	}
	wt, err := m.manager.Get(ctx, name)
	if err != nil {
		return err
	}
	return m.manager.Remove(ctx, wt.Path, false)
}

// CleanupStale removes worktrees for completed/failed tasks (implements core.WorktreeManager).
func (m *TaskWorktreeManager) CleanupStale(ctx context.Context) error {
	// Use a default max age of 24 hours for stale worktrees
	_, err := m.manager.CleanupStale(ctx, 24*time.Hour)
	return err
}

// List returns all managed worktrees (implements core.WorktreeManager).
func (m *TaskWorktreeManager) List(ctx context.Context) ([]*core.WorktreeInfo, error) {
	managed, err := m.manager.ListManaged(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*core.WorktreeInfo, 0, len(managed))
	for _, wt := range managed {
		// Extract TaskID from path by removing prefix
		name := filepath.Base(wt.Path)
		if strings.HasPrefix(name, m.manager.prefix) {
			name = strings.TrimPrefix(name, m.manager.prefix)
		}
		taskID := name
		if sepIdx := strings.Index(name, worktreeNameSeparator); sepIdx > -1 {
			taskID = name[:sepIdx]
		}

		status := core.WorktreeStatusActive
		if wt.Prunable {
			status = core.WorktreeStatusStale
		}

		result = append(result, &core.WorktreeInfo{
			TaskID:    core.TaskID(taskID),
			Path:      wt.Path,
			Branch:    wt.Branch,
			CreatedAt: wt.CreatedAt,
			Status:    status,
		})
	}

	return result, nil
}

// CleanupWorkflow removes all resources for a workflow.
func (m *TaskWorktreeManager) CleanupWorkflow(ctx context.Context, workflowID string, removeBranch bool) error {
	// ... existing code ...
	return nil // Placeholder
}

// MergeTaskToWorkflow merges a task's branch into the workflow branch.
func (m *TaskWorktreeManager) MergeTaskToWorkflow(ctx context.Context, workflowID string, taskID core.TaskID, strategy, strategyOption string) error {
	// Placeholder implementation
	return nil
}

// Manager returns the underlying WorktreeManager for advanced operations.
func (m *TaskWorktreeManager) Manager() *WorktreeManager {
	return m.manager
}

// WithLogger sets the logger for worktree manager warnings.
func (m *TaskWorktreeManager) WithLogger(logger *logging.Logger) *TaskWorktreeManager {
	if logger != nil {
		m.logger = logger
	}
	return m
}
