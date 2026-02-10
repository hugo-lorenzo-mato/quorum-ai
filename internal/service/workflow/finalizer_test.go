package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// --- Mocks for finalizer tests ---

type mockFinalizerGit struct {
	isClean    bool
	isCleanErr error
	addErr     error
	commitSHA  string
	commitErr  error
	pushErr    error
}

func (m *mockFinalizerGit) IsClean(_ context.Context) (bool, error)   { return m.isClean, m.isCleanErr }
func (m *mockFinalizerGit) Add(_ context.Context, _ ...string) error  { return m.addErr }
func (m *mockFinalizerGit) Commit(_ context.Context, _ string) (string, error) {
	return m.commitSHA, m.commitErr
}
func (m *mockFinalizerGit) Push(_ context.Context, _, _ string) error { return m.pushErr }

// Stubs for unused interface methods
func (m *mockFinalizerGit) RepoRoot(_ context.Context) (string, error)            { return "", nil }
func (m *mockFinalizerGit) CurrentBranch(_ context.Context) (string, error)       { return "", nil }
func (m *mockFinalizerGit) DefaultBranch(_ context.Context) (string, error)       { return "", nil }
func (m *mockFinalizerGit) RemoteURL(_ context.Context) (string, error)           { return "", nil }
func (m *mockFinalizerGit) BranchExists(_ context.Context, _ string) (bool, error) { return false, nil }
func (m *mockFinalizerGit) CreateBranch(_ context.Context, _, _ string) error     { return nil }
func (m *mockFinalizerGit) DeleteBranch(_ context.Context, _ string) error        { return nil }
func (m *mockFinalizerGit) CheckoutBranch(_ context.Context, _ string) error      { return nil }
func (m *mockFinalizerGit) CreateWorktree(_ context.Context, _, _ string) error   { return nil }
func (m *mockFinalizerGit) RemoveWorktree(_ context.Context, _ string) error      { return nil }
func (m *mockFinalizerGit) ListWorktrees(_ context.Context) ([]core.Worktree, error) { return nil, nil }
func (m *mockFinalizerGit) Status(_ context.Context) (*core.GitStatus, error)     { return nil, nil }
func (m *mockFinalizerGit) Diff(_ context.Context, _, _ string) (string, error)   { return "", nil }
func (m *mockFinalizerGit) DiffFiles(_ context.Context, _, _ string) ([]string, error) { return nil, nil }
func (m *mockFinalizerGit) Fetch(_ context.Context, _ string) error               { return nil }
func (m *mockFinalizerGit) Merge(_ context.Context, _ string, _ core.MergeOptions) error { return nil }
func (m *mockFinalizerGit) AbortMerge(_ context.Context) error                          { return nil }
func (m *mockFinalizerGit) HasMergeConflicts(_ context.Context) (bool, error)            { return false, nil }
func (m *mockFinalizerGit) GetConflictFiles(_ context.Context) ([]string, error)         { return nil, nil }
func (m *mockFinalizerGit) Rebase(_ context.Context, _ string) error                     { return nil }
func (m *mockFinalizerGit) AbortRebase(_ context.Context) error                          { return nil }
func (m *mockFinalizerGit) ContinueRebase(_ context.Context) error                       { return nil }
func (m *mockFinalizerGit) HasRebaseInProgress(_ context.Context) (bool, error)          { return false, nil }
func (m *mockFinalizerGit) ResetHard(_ context.Context, _ string) error                  { return nil }
func (m *mockFinalizerGit) ResetSoft(_ context.Context, _ string) error                  { return nil }
func (m *mockFinalizerGit) CherryPick(_ context.Context, _ string) error                 { return nil }
func (m *mockFinalizerGit) AbortCherryPick(_ context.Context) error                      { return nil }
func (m *mockFinalizerGit) RevParse(_ context.Context, _ string) (string, error)         { return "", nil }
func (m *mockFinalizerGit) IsAncestor(_ context.Context, _, _ string) (bool, error)      { return false, nil }
func (m *mockFinalizerGit) HasUncommittedChanges(_ context.Context) (bool, error)        { return false, nil }

type mockFinalizerGitHub struct {
	defaultBranch    string
	defaultBranchErr error
	createPR         *core.PullRequest
	createPRErr      error
	mergePRErr       error
}

func (m *mockFinalizerGitHub) GetDefaultBranch(_ context.Context) (string, error) {
	return m.defaultBranch, m.defaultBranchErr
}
func (m *mockFinalizerGitHub) CreatePR(_ context.Context, _ core.CreatePROptions) (*core.PullRequest, error) {
	return m.createPR, m.createPRErr
}
func (m *mockFinalizerGitHub) MergePR(_ context.Context, _ int, _ core.MergePROptions) error {
	return m.mergePRErr
}

// Stubs for unused interface methods
func (m *mockFinalizerGitHub) GetRepo(_ context.Context) (*core.RepoInfo, error)    { return nil, nil }
func (m *mockFinalizerGitHub) GetPR(_ context.Context, _ int) (*core.PullRequest, error) { return nil, nil }
func (m *mockFinalizerGitHub) ListPRs(_ context.Context, _ core.ListPROptions) ([]*core.PullRequest, error) {
	return nil, nil
}
func (m *mockFinalizerGitHub) UpdatePR(_ context.Context, _ int, _ core.UpdatePROptions) error { return nil }
func (m *mockFinalizerGitHub) ClosePR(_ context.Context, _ int) error                         { return nil }
func (m *mockFinalizerGitHub) RequestReview(_ context.Context, _ int, _ []string) error        { return nil }
func (m *mockFinalizerGitHub) AddComment(_ context.Context, _ int, _ string) error             { return nil }
func (m *mockFinalizerGitHub) GetCheckStatus(_ context.Context, _ string) (*core.CheckStatus, error) {
	return nil, nil
}

func (m *mockFinalizerGitHub) WaitForChecks(_ context.Context, _ string, _ time.Duration) (*core.CheckStatus, error) {
	return nil, nil
}

// --- Tests ---

func TestNewTaskFinalizer_Defaults(t *testing.T) {
	t.Parallel()

	f := NewTaskFinalizer(nil, nil, FinalizationConfig{})

	if f.config.MergeStrategy != "squash" {
		t.Errorf("MergeStrategy = %q, want %q", f.config.MergeStrategy, "squash")
	}
	if f.config.Remote != "origin" {
		t.Errorf("Remote = %q, want %q", f.config.Remote, "origin")
	}
}

func TestNewTaskFinalizer_CustomValues(t *testing.T) {
	t.Parallel()

	cfg := FinalizationConfig{
		AutoCommit:    true,
		AutoPush:      true,
		AutoPR:        true,
		AutoMerge:     true,
		PRBaseBranch:  "develop",
		MergeStrategy: "rebase",
		Remote:        "upstream",
	}
	f := NewTaskFinalizer(nil, nil, cfg)

	if f.config.MergeStrategy != "rebase" {
		t.Errorf("MergeStrategy = %q, want %q", f.config.MergeStrategy, "rebase")
	}
	if f.config.Remote != "upstream" {
		t.Errorf("Remote = %q, want %q", f.config.Remote, "upstream")
	}
	if f.config.PRBaseBranch != "develop" {
		t.Errorf("PRBaseBranch = %q, want %q", f.config.PRBaseBranch, "develop")
	}
}

func TestTaskFinalizer_BuildCommitMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		task     *core.Task
		wantSubs []string
	}{
		{
			name: "with description",
			task: &core.Task{
				ID:          "task-1",
				Name:        "Add user authentication",
				Description: "Implement JWT-based auth flow",
			},
			wantSubs: []string{
				"feat(quorum): Add user authentication",
				"Implement JWT-based auth flow",
				"Task-ID: task-1",
				"Generated-By: quorum-ai",
			},
		},
		{
			name: "without description",
			task: &core.Task{
				ID:   "task-2",
				Name: "Fix login bug",
			},
			wantSubs: []string{
				"feat(quorum): Fix login bug",
				"Task-ID: task-2",
				"Generated-By: quorum-ai",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewTaskFinalizer(nil, nil, FinalizationConfig{})
			msg := f.buildCommitMessage(tt.task)
			for _, sub := range tt.wantSubs {
				if !strings.Contains(msg, sub) {
					t.Errorf("buildCommitMessage() missing %q in:\n%s", sub, msg)
				}
			}
		})
	}
}

func TestTaskFinalizer_BuildPRBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		task     *core.Task
		wantSubs []string
		notWant  []string
	}{
		{
			name: "with description",
			task: &core.Task{
				ID:          "task-1",
				Name:        "Add API endpoints",
				Description: "REST endpoints for user management",
			},
			wantSubs: []string{
				"## Summary",
				"Add API endpoints",
				"## Description",
				"REST endpoints for user management",
				"Task ID: `task-1`",
				"quorum-ai",
			},
		},
		{
			name: "without description",
			task: &core.Task{
				ID:   "task-2",
				Name: "Quick fix",
			},
			wantSubs: []string{
				"## Summary",
				"Quick fix",
				"Task ID: `task-2`",
			},
			notWant: []string{
				"## Description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := NewTaskFinalizer(nil, nil, FinalizationConfig{})
			body := f.buildPRBody(tt.task)
			for _, sub := range tt.wantSubs {
				if !strings.Contains(body, sub) {
					t.Errorf("buildPRBody() missing %q in:\n%s", sub, body)
				}
			}
			for _, sub := range tt.notWant {
				if strings.Contains(body, sub) {
					t.Errorf("buildPRBody() should not contain %q in:\n%s", sub, body)
				}
			}
		})
	}
}

func TestTaskFinalizer_Finalize_NilGit(t *testing.T) {
	t.Parallel()

	f := NewTaskFinalizer(nil, nil, FinalizationConfig{})
	_, err := f.Finalize(context.Background(), &core.Task{}, "", "branch")
	if err == nil {
		t.Error("Finalize() should return error when git is nil")
	}
	if !strings.Contains(err.Error(), "git client not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTaskFinalizer_Finalize_CleanWorktree(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: true}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{AutoCommit: true})
	result, err := f.Finalize(context.Background(), &core.Task{}, "", "branch")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if result.CommitSHA != "" {
		t.Error("expected empty CommitSHA for clean worktree")
	}
}

func TestTaskFinalizer_Finalize_IsCleanError(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isCleanErr: fmt.Errorf("git status failed")}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{AutoCommit: true})
	_, err := f.Finalize(context.Background(), &core.Task{}, "", "branch")
	if err == nil {
		t.Error("Finalize() should return error when IsClean fails")
	}
}

func TestTaskFinalizer_Finalize_CommitOnly(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: false, commitSHA: "abc123"}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{AutoCommit: true})
	result, err := f.Finalize(context.Background(), &core.Task{ID: "t1", Name: "test"}, "", "branch")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if result.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %q, want %q", result.CommitSHA, "abc123")
	}
	if result.Pushed {
		t.Error("should not push when AutoPush is false")
	}
}

func TestTaskFinalizer_Finalize_CommitAndPush(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: false, commitSHA: "def456"}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{
		AutoCommit: true,
		AutoPush:   true,
	})
	result, err := f.Finalize(context.Background(), &core.Task{ID: "t1", Name: "test"}, "", "feat/branch")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if !result.Pushed {
		t.Error("expected Pushed = true")
	}
}

func TestTaskFinalizer_Finalize_CommitError(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: false, commitErr: fmt.Errorf("commit failed")}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{AutoCommit: true})
	_, err := f.Finalize(context.Background(), &core.Task{ID: "t1", Name: "test"}, "", "branch")
	if err == nil {
		t.Error("expected error on commit failure")
	}
}

func TestTaskFinalizer_Finalize_PushError(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: false, commitSHA: "abc", pushErr: fmt.Errorf("push failed")}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{
		AutoCommit: true,
		AutoPush:   true,
	})
	_, err := f.Finalize(context.Background(), &core.Task{ID: "t1", Name: "test"}, "", "branch")
	if err == nil {
		t.Error("expected error on push failure")
	}
}

func TestTaskFinalizer_Finalize_NoCommitNoAutoCommit(t *testing.T) {
	t.Parallel()

	git := &mockFinalizerGit{isClean: false}
	f := NewTaskFinalizer(git, nil, FinalizationConfig{AutoCommit: false, AutoPush: true})
	result, err := f.Finalize(context.Background(), &core.Task{ID: "t1", Name: "test"}, "", "branch")
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	// No commit = no push even if AutoPush is true
	if result.Pushed {
		t.Error("should not push without commit")
	}
}
