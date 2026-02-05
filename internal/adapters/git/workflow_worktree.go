package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// WorkflowWorktreeManagerImpl implements core.WorkflowWorktreeManager.
type WorkflowWorktreeManagerImpl struct {
	mu       sync.Mutex // Protects concurrent git operations
	baseDir  string
	repoPath string
	git      *Client
	logger   *slog.Logger
}

// Ensure interface compliance.
var _ core.WorkflowWorktreeManager = (*WorkflowWorktreeManagerImpl)(nil)

// NewWorkflowWorktreeManager creates a new WorkflowWorktreeManager.
func NewWorkflowWorktreeManager(repoPath, baseDir string, git *Client, logger *slog.Logger) (*WorkflowWorktreeManagerImpl, error) {
	if logger == nil {
		logger = slog.Default()
	}

	absBaseDir := baseDir
	if !filepath.IsAbs(baseDir) {
		absBaseDir = filepath.Join(repoPath, baseDir)
	}

	if err := os.MkdirAll(absBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating worktree base directory: %w", err)
	}

	return &WorkflowWorktreeManagerImpl{
		baseDir:  absBaseDir,
		repoPath: repoPath,
		git:      git,
		logger:   logger,
	}, nil
}

// GetWorkflowBranch returns the branch name for a workflow.
func (m *WorkflowWorktreeManagerImpl) GetWorkflowBranch(workflowID string) string {
	return "quorum/" + workflowID
}

// GetTaskBranch returns the branch name for a task within a workflow.
// Uses "__" as delimiter to avoid Git branch hierarchy conflicts with workflow branch.
func (m *WorkflowWorktreeManagerImpl) GetTaskBranch(workflowID string, taskID core.TaskID) string {
	return fmt.Sprintf("quorum/%s__%s", workflowID, string(taskID))
}

// getWorkflowWorktreeRoot returns the worktree root path for a workflow.
func (m *WorkflowWorktreeManagerImpl) getWorkflowWorktreeRoot(workflowID string) string {
	return filepath.Join(m.baseDir, workflowID)
}

// getTaskWorktreePath returns the worktree path for a task.
func (m *WorkflowWorktreeManagerImpl) getTaskWorktreePath(workflowID string, task *core.Task) string {
	sanitized := sanitizeForPath(task.Description)
	if len(sanitized) > 30 {
		sanitized = sanitized[:30]
	}
	return filepath.Join(m.getWorkflowWorktreeRoot(workflowID), fmt.Sprintf("%s__%s", task.ID, sanitized))
}

// sanitizeForPath removes characters unsafe for file paths.
func sanitizeForPath(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// getMergeWorktreePath returns the path for the temporary merge worktree.
func (m *WorkflowWorktreeManagerImpl) getMergeWorktreePath(workflowID string) string {
	return filepath.Join(m.getWorkflowWorktreeRoot(workflowID), "_merge")
}

// createMergeWorktree creates a temporary worktree for merge operations.
// Returns a git client for the worktree and a cleanup function.
// The worktree is created pointing to the specified branch.
// This allows merge operations without affecting the user's working directory.
func (m *WorkflowWorktreeManagerImpl) createMergeWorktree(ctx context.Context, workflowID, branch string) (*Client, func(), error) {
	mergeWorktreePath := m.getMergeWorktreePath(workflowID)

	// Remove existing merge worktree if present (from previous failed run)
	if _, err := os.Stat(mergeWorktreePath); err == nil {
		_ = m.git.RemoveWorktree(ctx, mergeWorktreePath)
		_ = os.RemoveAll(mergeWorktreePath)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(mergeWorktreePath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating merge worktree parent: %w", err)
	}

	// Create worktree pointing to the target branch
	if err := m.git.CreateWorktree(ctx, mergeWorktreePath, branch); err != nil {
		return nil, nil, fmt.Errorf("creating merge worktree: %w", err)
	}

	// Create git client for the worktree
	mergeGit, err := NewClient(mergeWorktreePath)
	if err != nil {
		_ = m.git.RemoveWorktree(ctx, mergeWorktreePath)
		return nil, nil, fmt.Errorf("creating git client for merge worktree: %w", err)
	}

	cleanup := func() {
		if rmErr := m.git.RemoveWorktree(ctx, mergeWorktreePath); rmErr != nil {
			m.logger.Warn("failed to remove merge worktree", "path", mergeWorktreePath, "error", rmErr)
		}
		// Also remove the directory in case worktree remove didn't clean it
		_ = os.RemoveAll(mergeWorktreePath)
	}

	m.logger.Info("created merge worktree",
		"workflow_id", workflowID,
		"path", mergeWorktreePath,
		"branch", branch)

	return mergeGit, cleanup, nil
}

// InitializeWorkflow creates a workflow branch and worktree root directory.
func (m *WorkflowWorktreeManagerImpl) InitializeWorkflow(ctx context.Context, workflowID, baseBranch string) (*core.WorkflowGitInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("initializing workflow git isolation",
		"workflow_id", workflowID,
		"base_branch", baseBranch)

	// Determine base branch if not specified
	if baseBranch == "" {
		var err error
		baseBranch, err = m.git.DefaultBranch(ctx)
		if err != nil {
			baseBranch = "main" // Fallback
		}
	}

	workflowBranch := m.GetWorkflowBranch(workflowID)
	worktreeRoot := m.getWorkflowWorktreeRoot(workflowID)

	// Create the workflow branch from base branch using `git branch` (not checkout)
	// This avoids changing the current branch of the main repo
	_, err := m.git.run(ctx, "branch", workflowBranch, baseBranch)
	if err != nil {
		// Check if branch already exists (resume scenario)
		if !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("creating workflow branch %s: %w", workflowBranch, err)
		}
		m.logger.Info("workflow branch already exists, reusing", "branch", workflowBranch)
	}

	// Create the workflow worktree root directory
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating workflow worktree root: %w", err)
	}

	return &core.WorkflowGitInfo{
		WorkflowID:     workflowID,
		WorkflowBranch: workflowBranch,
		BaseBranch:     baseBranch,
		WorktreeRoot:   worktreeRoot,
		CreatedAt:      time.Now(),
		TaskCount:      0,
		PendingMerges:  0,
	}, nil
}

// CreateTaskWorktree creates a worktree for a task within the workflow namespace.
func (m *WorkflowWorktreeManagerImpl) CreateTaskWorktree(ctx context.Context, workflowID string, task *core.Task) (*core.WorktreeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("creating task worktree",
		"workflow_id", workflowID,
		"task_id", task.ID)

	workflowBranch := m.GetWorkflowBranch(workflowID)
	taskBranch := m.GetTaskBranch(workflowID, task.ID)
	worktreePath := m.getTaskWorktreePath(workflowID, task)

	// Ensure workflow worktree root exists
	worktreeRoot := m.getWorkflowWorktreeRoot(workflowID)
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating workflow worktree root: %w", err)
	}

	// Create task branch from workflow branch using `git branch` (not checkout)
	// This avoids changing the current branch of the main repo
	_, err := m.git.run(ctx, "branch", taskBranch, workflowBranch)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("creating task branch %s: %w", taskBranch, err)
		}
		m.logger.Info("task branch already exists, reusing", "branch", taskBranch)
	}

	// Create the worktree
	if err := m.git.CreateWorktree(ctx, worktreePath, taskBranch); err != nil {
		// Check if worktree already exists
		if _, statErr := os.Stat(worktreePath); statErr == nil {
			m.logger.Info("task worktree already exists, reusing", "path", worktreePath)
			return &core.WorktreeInfo{
				Path:   worktreePath,
				Branch: taskBranch,
				TaskID: task.ID,
			}, nil
		}
		return nil, fmt.Errorf("creating worktree at %s: %w", worktreePath, err)
	}

	m.logger.Info("task worktree created",
		"path", worktreePath,
		"branch", taskBranch)

	return &core.WorktreeInfo{
		Path:   worktreePath,
		Branch: taskBranch,
		TaskID: task.ID,
	}, nil
}

// RemoveTaskWorktree removes a task's worktree and optionally its branch.
func (m *WorkflowWorktreeManagerImpl) RemoveTaskWorktree(ctx context.Context, workflowID string, taskID core.TaskID, removeBranch bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("removing task worktree",
		"workflow_id", workflowID,
		"task_id", taskID,
		"remove_branch", removeBranch)

	taskBranch := m.GetTaskBranch(workflowID, taskID)

	// Find the worktree path
	worktreeRoot := m.getWorkflowWorktreeRoot(workflowID)
	entries, err := os.ReadDir(worktreeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already cleaned up
		}
		return fmt.Errorf("reading worktree root: %w", err)
	}

	var worktreePath string
	taskIDPrefix := string(taskID) + "__"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), taskIDPrefix) {
			worktreePath = filepath.Join(worktreeRoot, entry.Name())
			break
		}
	}

	// Remove the worktree
	if worktreePath != "" {
		if err := m.git.RemoveWorktree(ctx, worktreePath); err != nil {
			m.logger.Warn("failed to remove worktree", "path", worktreePath, "error", err)
		}
	}

	// Remove the branch if requested
	if removeBranch {
		if err := m.git.DeleteBranchForce(ctx, taskBranch); err != nil {
			m.logger.Warn("failed to delete branch", "branch", taskBranch, "error", err)
		}
	}

	return nil
}

// MergeTaskToWorkflow merges a task branch into the workflow branch.
func (m *WorkflowWorktreeManagerImpl) MergeTaskToWorkflow(ctx context.Context, workflowID string, taskID core.TaskID, strategy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.mergeTaskToWorkflowLocked(ctx, workflowID, taskID, strategy)
}

// mergeTaskToWorkflowLocked is the internal implementation without mutex.
// Caller must hold m.mu.
// Uses a temporary worktree to perform the merge, avoiding checkout in the user's working directory.
func (m *WorkflowWorktreeManagerImpl) mergeTaskToWorkflowLocked(ctx context.Context, workflowID string, taskID core.TaskID, strategy string) error {
	m.logger.Info("merging task to workflow",
		"workflow_id", workflowID,
		"task_id", taskID,
		"strategy", strategy)

	workflowBranch := m.GetWorkflowBranch(workflowID)
	taskBranch := m.GetTaskBranch(workflowID, taskID)

	// Create a temporary worktree for the merge operation.
	// This avoids doing checkout in the main repo, which would fail if user has uncommitted changes.
	mergeGit, cleanup, err := m.createMergeWorktree(ctx, workflowID, workflowBranch)
	if err != nil {
		return fmt.Errorf("creating merge worktree: %w", err)
	}
	defer cleanup()

	// Perform merge based on strategy (all operations happen in the merge worktree)
	switch strategy {
	case "rebase":
		return m.rebaseTaskToWorkflowInWorktree(ctx, mergeGit, workflowBranch, taskBranch)
	case "parallel":
		// For parallel, just attempt merge without special handling
		return m.mergeTaskSequentialInWorktree(ctx, mergeGit, taskBranch, string(taskID))
	default: // "sequential" or empty
		return m.mergeTaskSequentialInWorktree(ctx, mergeGit, taskBranch, string(taskID))
	}
}

// mergeTaskSequentialInWorktree performs a merge in the provided worktree git client.
// This is the isolated version that doesn't touch the user's working directory.
func (m *WorkflowWorktreeManagerImpl) mergeTaskSequentialInWorktree(ctx context.Context, worktreeGit *Client, taskBranch, taskID string) error {
	message := fmt.Sprintf("Merge task %s", taskID)

	// Execute merge with --no-ff in the worktree
	_, err := worktreeGit.run(ctx, "merge", "--no-ff", "-m", message, taskBranch)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "conflict") {
			// Abort the merge and return conflict error
			_, _ = worktreeGit.run(ctx, "merge", "--abort")
			return fmt.Errorf("merge conflict for task %s: %w", taskID, ErrMergeConflict)
		}
		return fmt.Errorf("merging task branch: %w", err)
	}

	return nil
}

// rebaseTaskToWorkflowInWorktree performs a rebase (via cherry-pick) in the provided worktree.
// This is the isolated version that doesn't touch the user's working directory.
func (m *WorkflowWorktreeManagerImpl) rebaseTaskToWorkflowInWorktree(ctx context.Context, worktreeGit *Client, workflowBranch, taskBranch string) error {
	// For rebase strategy, we cherry-pick commits from task branch
	// This maintains linear history

	// Get commits that are in taskBranch but not in workflowBranch
	// Note: we use the main git client here since this is a read-only operation
	commits, err := m.getUniqueCommits(ctx, workflowBranch, taskBranch)
	if err != nil {
		return fmt.Errorf("getting unique commits: %w", err)
	}

	// Cherry-pick each commit in the worktree
	for _, commit := range commits {
		_, err := worktreeGit.run(ctx, "cherry-pick", commit)
		if err != nil {
			if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "conflict") {
				_, _ = worktreeGit.run(ctx, "cherry-pick", "--abort")
				return fmt.Errorf("cherry-pick conflict for commit %s: %w", commit, ErrMergeConflict)
			}
			return fmt.Errorf("cherry-picking commit %s: %w", commit, err)
		}
	}

	return nil
}

func (m *WorkflowWorktreeManagerImpl) getUniqueCommits(ctx context.Context, base, head string) ([]string, error) {
	// Get commits in head that are not in base
	output, err := m.git.run(ctx, "log", "--format=%H", base+".."+head)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	commits := strings.Split(strings.TrimSpace(output), "\n")
	// Reverse to apply in chronological order
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits, nil
}

// MergeAllTasksToWorkflow merges all completed task branches to workflow branch.
func (m *WorkflowWorktreeManagerImpl) MergeAllTasksToWorkflow(ctx context.Context, workflowID string, taskIDs []core.TaskID, strategy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("merging all tasks to workflow",
		"workflow_id", workflowID,
		"task_count", len(taskIDs),
		"strategy", strategy)

	var errs []string
	for _, taskID := range taskIDs {
		if err := m.mergeTaskToWorkflowLocked(ctx, workflowID, taskID, strategy); err != nil {
			errs = append(errs, fmt.Sprintf("task %s: %v", taskID, err))
			// Continue with other tasks unless we have a critical failure
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("merge errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// FinalizeWorkflow completes workflow Git operations and optionally merges to base.
// Uses a temporary worktree to perform the merge, avoiding checkout in the user's working directory.
func (m *WorkflowWorktreeManagerImpl) FinalizeWorkflow(ctx context.Context, workflowID string, merge bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("finalizing workflow",
		"workflow_id", workflowID,
		"merge_to_base", merge)

	workflowBranch := m.GetWorkflowBranch(workflowID)

	if merge {
		// Get base branch
		baseBranch, err := m.git.DefaultBranch(ctx)
		if err != nil {
			baseBranch = "main"
		}

		// Create a temporary worktree for the finalize merge.
		// We use a different path than the task merge worktree to avoid conflicts.
		finalizeWorktreePath := filepath.Join(m.getWorkflowWorktreeRoot(workflowID), "_finalize")

		// Remove existing finalize worktree if present (from previous failed run)
		if _, statErr := os.Stat(finalizeWorktreePath); statErr == nil {
			_ = m.git.RemoveWorktree(ctx, finalizeWorktreePath)
			_ = os.RemoveAll(finalizeWorktreePath)
		}

		// Create worktree pointing to the base branch
		if err := m.git.CreateWorktree(ctx, finalizeWorktreePath, baseBranch); err != nil {
			return fmt.Errorf("creating finalize worktree: %w", err)
		}

		// Ensure cleanup of the finalize worktree
		defer func() {
			if rmErr := m.git.RemoveWorktree(ctx, finalizeWorktreePath); rmErr != nil {
				m.logger.Warn("failed to remove finalize worktree", "path", finalizeWorktreePath, "error", rmErr)
			}
			_ = os.RemoveAll(finalizeWorktreePath)
		}()

		// Create git client for the finalize worktree
		finalizeGit, err := NewClient(finalizeWorktreePath)
		if err != nil {
			return fmt.Errorf("creating git client for finalize worktree: %w", err)
		}

		m.logger.Info("created finalize worktree",
			"workflow_id", workflowID,
			"path", finalizeWorktreePath,
			"base_branch", baseBranch)

		// Merge workflow branch into base (in the worktree)
		message := fmt.Sprintf("Merge workflow %s", workflowID)
		_, err = finalizeGit.run(ctx, "merge", "--no-ff", "-m", message, workflowBranch)
		if err != nil {
			if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "conflict") {
				_, _ = finalizeGit.run(ctx, "merge", "--abort")
				return fmt.Errorf("conflict merging workflow to base: %w", ErrMergeConflict)
			}
			return fmt.Errorf("merging workflow branch: %w", err)
		}

		m.logger.Info("workflow merged to base branch",
			"workflow_id", workflowID,
			"workflow_branch", workflowBranch,
			"base_branch", baseBranch)
	}

	// Cleanup task worktrees (but keep branches for history)
	if err := m.cleanupWorkflowLocked(ctx, workflowID, false); err != nil {
		m.logger.Warn("failed to cleanup workflow", "error", err)
	}

	return nil
}

// CleanupWorkflow removes all Git artifacts for a workflow.
func (m *WorkflowWorktreeManagerImpl) CleanupWorkflow(ctx context.Context, workflowID string, removeWorkflowBranch bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cleanupWorkflowLocked(ctx, workflowID, removeWorkflowBranch)
}

// cleanupWorkflowLocked is the internal implementation without mutex.
// Caller must hold m.mu.
func (m *WorkflowWorktreeManagerImpl) cleanupWorkflowLocked(ctx context.Context, workflowID string, removeWorkflowBranch bool) error {
	m.logger.Info("cleaning up workflow",
		"workflow_id", workflowID,
		"remove_workflow_branch", removeWorkflowBranch)

	worktreeRoot := m.getWorkflowWorktreeRoot(workflowID)

	// Remove all task worktrees in the workflow directory
	entries, err := os.ReadDir(worktreeRoot)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				worktreePath := filepath.Join(worktreeRoot, entry.Name())
				if err := m.git.RemoveWorktree(ctx, worktreePath); err != nil {
					m.logger.Warn("failed to remove worktree", "path", worktreePath, "error", err)
				}
			}
		}
	}

	// Remove the workflow worktree root directory
	if err := os.RemoveAll(worktreeRoot); err != nil {
		m.logger.Warn("failed to remove worktree root", "path", worktreeRoot, "error", err)
	}

	// Remove task branches
	// Task branches are named: quorum/<workflow-id>__<task-id>
	workflowBranchPrefix := m.GetWorkflowBranch(workflowID) + "__"
	branches, err := m.listBranchesWithPrefix(ctx, workflowBranchPrefix)
	if err == nil {
		for _, branch := range branches {
			if err := m.git.DeleteBranchForce(ctx, branch); err != nil {
				m.logger.Warn("failed to delete branch", "branch", branch, "error", err)
			}
		}
	}

	// Remove workflow branch if requested
	if removeWorkflowBranch {
		workflowBranch := m.GetWorkflowBranch(workflowID)
		if err := m.git.DeleteBranchForce(ctx, workflowBranch); err != nil {
			m.logger.Warn("failed to delete workflow branch", "branch", workflowBranch, "error", err)
		}
	}

	return nil
}

func (m *WorkflowWorktreeManagerImpl) listBranchesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	output, err := m.git.run(ctx, "branch", "--list", prefix+"*")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	var branches []string
	for _, line := range strings.Split(output, "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	return branches, nil
}

// GetWorkflowStatus returns the current Git status of a workflow.
func (m *WorkflowWorktreeManagerImpl) GetWorkflowStatus(ctx context.Context, workflowID string) (*core.WorkflowGitStatus, error) {
	workflowBranch := m.GetWorkflowBranch(workflowID)
	baseBranch, err := m.git.DefaultBranch(ctx)
	if err != nil {
		baseBranch = "main"
	}

	status := &core.WorkflowGitStatus{}

	// Check for conflicts via status
	gitStatus, err := m.git.StatusLocal(ctx)
	if err == nil {
		status.HasConflicts = gitStatus.HasConflicts
	}

	// Get ahead/behind counts
	aheadBehind, err := m.getAheadBehind(ctx, workflowBranch, baseBranch)
	if err == nil {
		status.AheadOfBase = aheadBehind.ahead
		status.BehindBase = aheadBehind.behind
	}

	// Get unmerged tasks
	status.UnmergedTasks = m.findUnmergedTasks(ctx, workflowID)

	// Get last merge commit
	lastCommit, err := m.git.run(ctx, "rev-parse", workflowBranch)
	if err == nil && len(lastCommit) >= 8 {
		status.LastMergeCommit = lastCommit[:8]
	}

	return status, nil
}

type aheadBehindInfo struct {
	ahead  int
	behind int
}

func (m *WorkflowWorktreeManagerImpl) getAheadBehind(ctx context.Context, branch, base string) (aheadBehindInfo, error) {
	output, err := m.git.run(ctx, "rev-list", "--left-right", "--count", base+"..."+branch)
	if err != nil {
		return aheadBehindInfo{}, err
	}

	var behind, ahead int
	if _, err := fmt.Sscanf(output, "%d\t%d", &behind, &ahead); err != nil {
		return aheadBehindInfo{}, fmt.Errorf("parsing ahead/behind: %w", err)
	}

	return aheadBehindInfo{ahead: ahead, behind: behind}, nil
}

func (m *WorkflowWorktreeManagerImpl) findUnmergedTasks(ctx context.Context, workflowID string) []core.TaskID {
	// List all task branches that haven't been merged to workflow branch
	// Task branches use format: quorum/<workflowID>__<taskID>
	workflowBranch := m.GetWorkflowBranch(workflowID)
	taskBranchPrefix := workflowBranch + "__"

	branches, err := m.listBranchesWithPrefix(ctx, taskBranchPrefix)
	if err != nil {
		return nil
	}

	var unmerged []core.TaskID
	for _, branch := range branches {
		// Check if branch is merged into workflow branch
		isAncestor, err := m.isAncestor(ctx, branch, workflowBranch)
		if err != nil || !isAncestor {
			// Extract task ID from branch name (format: quorum/<wfID>__<taskID>)
			taskPart := strings.TrimPrefix(branch, taskBranchPrefix)
			unmerged = append(unmerged, core.TaskID(taskPart))
		}
	}

	return unmerged
}

func (m *WorkflowWorktreeManagerImpl) isAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := m.git.run(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		// Exit code 1 means not an ancestor (not an error)
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListActiveWorkflows returns information about all active workflow worktrees.
func (m *WorkflowWorktreeManagerImpl) ListActiveWorkflows(ctx context.Context) ([]*core.WorkflowGitInfo, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading worktree base directory: %w", err)
	}

	var workflows []*core.WorkflowGitInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		workflowID := entry.Name()
		worktreeRoot := filepath.Join(m.baseDir, workflowID)
		workflowBranch := m.GetWorkflowBranch(workflowID)

		// Check if workflow branch exists
		exists, err := m.git.BranchExists(ctx, workflowBranch)
		if err != nil || !exists {
			continue
		}

		// Count tasks
		taskEntries, err := os.ReadDir(worktreeRoot)
		taskCount := 0
		if err == nil {
			for _, te := range taskEntries {
				if te.IsDir() {
					taskCount++
				}
			}
		}

		info, err := entry.Info()
		createdAt := time.Now()
		if err == nil {
			createdAt = info.ModTime()
		}

		workflows = append(workflows, &core.WorkflowGitInfo{
			WorkflowID:     workflowID,
			WorkflowBranch: workflowBranch,
			BaseBranch:     "main", // Would need to track this
			WorktreeRoot:   worktreeRoot,
			CreatedAt:      createdAt,
			TaskCount:      taskCount,
		})
	}

	return workflows, nil
}
