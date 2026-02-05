// Package workflow provides crash recovery capabilities for zombie workflows.
package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

// RecoveryStateManager defines the state manager interface for recovery operations.
type RecoveryStateManager interface {
	LoadByID(ctx context.Context, id core.WorkflowID) (*core.WorkflowState, error)
	Save(ctx context.Context, state *core.WorkflowState) error
	FindZombieWorkflows(ctx context.Context, staleThreshold time.Duration) ([]*core.WorkflowState, error)
}

// RecoveryManager handles workflow crash recovery.
type RecoveryManager struct {
	stateManager   RecoveryStateManager
	logger         *logging.Logger
	staleThreshold time.Duration
	repoPath       string
}

// RecoveryResult contains the result of a recovery operation.
type RecoveryResult struct {
	WorkflowID       core.WorkflowID
	RecoveredChanges bool
	RecoveryBranch   string
	AbortedMerge     bool
	AbortedRebase    bool
	ResetTasks       []core.TaskID
	Error            error
}

// NewRecoveryManager creates a new recovery manager.
func NewRecoveryManager(
	sm RecoveryStateManager,
	repoPath string,
	logger *logging.Logger,
) *RecoveryManager {
	return &RecoveryManager{
		stateManager:   sm,
		logger:         logger,
		staleThreshold: 5 * time.Minute,
		repoPath:       repoPath,
	}
}

// SetStaleThreshold configures the staleness threshold.
func (r *RecoveryManager) SetStaleThreshold(d time.Duration) {
	r.staleThreshold = d
}

// FindZombieWorkflows returns workflows with stale heartbeats.
func (r *RecoveryManager) FindZombieWorkflows(ctx context.Context) ([]*core.WorkflowState, error) {
	return r.stateManager.FindZombieWorkflows(ctx, r.staleThreshold)
}

// RecoverWorkflow attempts to recover a crashed workflow.
func (r *RecoveryManager) RecoverWorkflow(ctx context.Context, workflowID core.WorkflowID) (*RecoveryResult, error) {
	result := &RecoveryResult{WorkflowID: workflowID}

	r.logger.Info("starting workflow recovery", "workflow_id", workflowID)

	// Load workflow state
	state, err := r.stateManager.LoadByID(ctx, workflowID)
	if err != nil {
		result.Error = fmt.Errorf("loading workflow: %w", err)
		return result, result.Error
	}
	if state == nil {
		result.Error = fmt.Errorf("workflow not found: %s", workflowID)
		return result, result.Error
	}

	// Step 1: Recover uncommitted changes
	if err := r.recoverUncommittedChanges(ctx, state, result); err != nil {
		r.logger.Warn("failed to recover uncommitted changes",
			"workflow_id", workflowID,
			"error", err)
	}

	// Step 2: Abort incomplete Git operations
	if err := r.abortIncompleteGitOps(ctx, state, result); err != nil {
		r.logger.Warn("failed to abort incomplete git operations",
			"workflow_id", workflowID,
			"error", err)
	}

	// Step 3: Reset running task statuses
	if err := r.resetRunningTasks(ctx, state, result); err != nil {
		r.logger.Warn("failed to reset running tasks",
			"workflow_id", workflowID,
			"error", err)
	}

	// Step 4: Update workflow status to paused
	state.Status = core.WorkflowStatusPaused
	if err := r.stateManager.Save(ctx, state); err != nil {
		result.Error = fmt.Errorf("saving recovered state: %w", err)
		return result, result.Error
	}

	r.logger.Info("workflow recovery complete",
		"workflow_id", workflowID,
		"recovered_changes", result.RecoveredChanges,
		"reset_tasks", len(result.ResetTasks))

	return result, nil
}

// recoverUncommittedChanges saves uncommitted work from worktrees.
func (r *RecoveryManager) recoverUncommittedChanges(ctx context.Context, state *core.WorkflowState, result *RecoveryResult) error {
	for _, taskState := range state.Tasks {
		if taskState.Status != core.TaskStatusRunning {
			continue
		}

		if taskState.WorktreePath == "" {
			continue
		}

		// Check if worktree exists and has uncommitted changes
		if _, err := os.Stat(taskState.WorktreePath); os.IsNotExist(err) {
			continue
		}

		hasChanges, err := r.hasUncommittedChanges(ctx, taskState.WorktreePath)
		if err != nil {
			r.logger.Warn("failed to check for uncommitted changes",
				"worktree", taskState.WorktreePath,
				"error", err)
			continue
		}

		if !hasChanges {
			continue
		}

		// Create recovery branch
		recoveryBranch := fmt.Sprintf("%s-recovery-%d", taskState.Branch, time.Now().Unix())
		if err := r.commitToRecoveryBranch(ctx, taskState.WorktreePath, recoveryBranch); err != nil {
			r.logger.Warn("failed to create recovery branch",
				"worktree", taskState.WorktreePath,
				"error", err)
			continue
		}

		result.RecoveredChanges = true
		result.RecoveryBranch = recoveryBranch

		r.logger.Info("recovered uncommitted changes",
			"task_id", taskState.ID,
			"recovery_branch", recoveryBranch)
	}

	return nil
}

// hasUncommittedChanges checks if a worktree has uncommitted changes.
func (r *RecoveryManager) hasUncommittedChanges(ctx context.Context, worktreePath string) (bool, error) {
	wtGit, err := git.NewClient(worktreePath)
	if err != nil {
		return false, err
	}

	return wtGit.HasUncommittedChanges(ctx)
}

// commitToRecoveryBranch creates a recovery commit with uncommitted changes.
func (r *RecoveryManager) commitToRecoveryBranch(ctx context.Context, worktreePath, recoveryBranch string) error {
	wtGit, err := git.NewClient(worktreePath)
	if err != nil {
		return err
	}

	// Create and checkout recovery branch
	if err := wtGit.CreateBranch(ctx, recoveryBranch, "HEAD"); err != nil {
		return fmt.Errorf("creating recovery branch: %w", err)
	}

	if err := wtGit.CheckoutBranch(ctx, recoveryBranch); err != nil {
		return fmt.Errorf("checking out recovery branch: %w", err)
	}

	// Stage all changes
	if err := wtGit.Add(ctx, "-A"); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	// Commit
	message := fmt.Sprintf("Recovery commit - uncommitted changes from crash at %s",
		time.Now().Format(time.RFC3339))
	if _, err := wtGit.Commit(ctx, message); err != nil {
		return fmt.Errorf("creating recovery commit: %w", err)
	}

	return nil
}

// abortIncompleteGitOps aborts any incomplete merge/rebase operations.
func (r *RecoveryManager) abortIncompleteGitOps(ctx context.Context, state *core.WorkflowState, result *RecoveryResult) error {
	// Check main repo
	if err := r.abortIncompleteOpsInPath(ctx, r.repoPath, result); err != nil {
		return err
	}

	// Check all task worktrees
	for _, taskState := range state.Tasks {
		if taskState.WorktreePath == "" {
			continue
		}
		if _, err := os.Stat(taskState.WorktreePath); os.IsNotExist(err) {
			continue
		}
		if err := r.abortIncompleteOpsInPath(ctx, taskState.WorktreePath, result); err != nil {
			r.logger.Warn("failed to abort git ops in worktree",
				"path", taskState.WorktreePath,
				"error", err)
		}
	}

	return nil
}

// abortIncompleteOpsInPath aborts incomplete ops in a specific directory.
func (r *RecoveryManager) abortIncompleteOpsInPath(ctx context.Context, path string, result *RecoveryResult) error {
	gitDir := filepath.Join(path, ".git")

	// For worktrees, .git is a file pointing to the real git directory
	// Check if .git is a file (worktree) or directory (main repo)
	info, err := os.Stat(gitDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No .git, not a repo
		}
		return fmt.Errorf("stat git dir: %w", err)
	}

	// If .git is a file, read the actual gitdir path
	if !info.IsDir() {
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return fmt.Errorf("read git dir pointer: %w", err)
		}
		// Parse "gitdir: /path/to/gitdir"
		var actualGitDir string
		_, _ = fmt.Sscanf(string(content), "gitdir: %s", &actualGitDir)
		if actualGitDir != "" {
			gitDir = actualGitDir
		}
	}

	// Check for incomplete merge
	mergeHead := filepath.Join(gitDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHead); err == nil {
		r.logger.Info("aborting incomplete merge", "path", path)
		gitClient, err := git.NewClient(path)
		if err != nil {
			return err
		}
		if err := gitClient.AbortMerge(ctx); err != nil {
			r.logger.Warn("failed to abort merge", "error", err)
		} else {
			result.AbortedMerge = true
		}
	}

	// Check for incomplete rebase
	rebaseApply := filepath.Join(gitDir, "rebase-apply")
	rebaseMerge := filepath.Join(gitDir, "rebase-merge")
	if _, err := os.Stat(rebaseApply); err == nil {
		r.logger.Info("aborting incomplete rebase", "path", path)
		gitClient, err := git.NewClient(path)
		if err != nil {
			return err
		}
		if err := gitClient.AbortRebase(ctx); err != nil {
			r.logger.Warn("failed to abort rebase", "error", err)
		} else {
			result.AbortedRebase = true
		}
	} else if _, err := os.Stat(rebaseMerge); err == nil {
		r.logger.Info("aborting incomplete rebase", "path", path)
		gitClient, err := git.NewClient(path)
		if err != nil {
			return err
		}
		if err := gitClient.AbortRebase(ctx); err != nil {
			r.logger.Warn("failed to abort rebase", "error", err)
		} else {
			result.AbortedRebase = true
		}
	}

	// Check for incomplete cherry-pick
	cherryPickHead := filepath.Join(gitDir, "CHERRY_PICK_HEAD")
	if _, err := os.Stat(cherryPickHead); err == nil {
		r.logger.Info("aborting incomplete cherry-pick", "path", path)
		gitClient, err := git.NewClient(path)
		if err != nil {
			return err
		}
		if err := gitClient.AbortCherryPick(ctx); err != nil {
			r.logger.Warn("failed to abort cherry-pick", "error", err)
		}
	}

	return nil
}

// resetRunningTasks marks running tasks as pending for retry.
func (r *RecoveryManager) resetRunningTasks(_ context.Context, state *core.WorkflowState, result *RecoveryResult) error {
	for taskID, taskState := range state.Tasks {
		if taskState.Status != core.TaskStatusRunning {
			continue
		}

		r.logger.Info("resetting running task", "task_id", taskID)

		taskState.Status = core.TaskStatusPending
		taskState.Retries++
		taskState.Resumable = true
		taskState.Error = "Task interrupted by crash - will be retried"

		result.ResetTasks = append(result.ResetTasks, taskID)
	}

	return nil
}
