package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// Compile-time interface conformance check.
var _ core.GitClient = (*Client)(nil)

// Client wraps git CLI operations.
type Client struct {
	repoPath string
	timeout  time.Duration
}

// NewClient creates a new git client.
func NewClient(repoPath string) (*Client, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	client := &Client{
		repoPath: absPath,
		timeout:  30 * time.Second,
	}

	// Verify it's a git repository
	if err := client.verifyRepo(); err != nil {
		return nil, err
	}

	return client, nil
}

// verifyRepo checks if path is a git repository.
func (c *Client) verifyRepo() error {
	_, err := c.run(context.Background(), "rev-parse", "--git-dir")
	if err != nil {
		return core.ErrValidation("NOT_GIT_REPO", fmt.Sprintf("%s is not a git repository", c.repoPath))
	}
	return nil
}

// run executes a git command.
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", core.ErrTimeout("git command timed out")
		}
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// RepoRoot returns the repository root path (implements core.GitClient).
func (c *Client) RepoRoot(_ context.Context) (string, error) {
	return c.repoPath, nil
}

// Status returns the repository status (implements core.GitClient).
func (c *Client) Status(ctx context.Context) (*core.GitStatus, error) {
	output, err := c.run(ctx, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, err
	}

	return parseStatusToCore(output), nil
}

// StatusLocal returns the repository status with local types (for internal use).
func (c *Client) StatusLocal(ctx context.Context) (*Status, error) {
	output, err := c.run(ctx, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return nil, err
	}

	return parseStatus(output), nil
}

// Status represents git repository status.
type Status struct {
	Branch       string
	Upstream     string
	Ahead        int
	Behind       int
	Staged       []string
	Modified     []string
	Untracked    []string
	HasConflicts bool
}

// IsClean returns true if there are no changes.
func (s *Status) IsClean() bool {
	return len(s.Staged) == 0 && len(s.Modified) == 0 && len(s.Untracked) == 0 && !s.HasConflicts
}

func parseStatus(output string) *Status {
	status := &Status{
		Staged:    make([]string, 0),
		Modified:  make([]string, 0),
		Untracked: make([]string, 0),
	}

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.upstream "):
			status.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				_, _ = fmt.Sscanf(parts[2], "+%d", &status.Ahead)
				_, _ = fmt.Sscanf(parts[3], "-%d", &status.Behind)
			}
		case len(line) > 2:
			// Parse status lines
			switch line[0] {
			case '1': // Ordinary changed entry
				// Format: 1 XY ... path
				if len(line) > 113 {
					path := line[113:]
					xy := line[2:4]
					if xy[0] != '.' {
						status.Staged = append(status.Staged, path)
					}
					if xy[1] != '.' {
						status.Modified = append(status.Modified, path)
					}
				}
			case '2': // Renamed/copied
				// Similar parsing for renames
			case '?': // Untracked
				status.Untracked = append(status.Untracked, strings.TrimPrefix(line, "? "))
			case 'u': // Unmerged (conflict)
				status.HasConflicts = true
			}
		}
	}

	return status
}

// parseStatusToCore parses git status output to core.GitStatus.
func parseStatusToCore(output string) *core.GitStatus {
	local := parseStatus(output)

	status := &core.GitStatus{
		Branch:       local.Branch,
		Ahead:        local.Ahead,
		Behind:       local.Behind,
		Staged:       make([]core.FileStatus, 0, len(local.Staged)),
		Unstaged:     make([]core.FileStatus, 0, len(local.Modified)),
		Untracked:    local.Untracked,
		HasConflicts: local.HasConflicts,
	}

	for _, path := range local.Staged {
		status.Staged = append(status.Staged, core.FileStatus{Path: path, Status: "M"})
	}
	for _, path := range local.Modified {
		status.Unstaged = append(status.Unstaged, core.FileStatus{Path: path, Status: "M"})
	}

	return status
}

// CurrentBranch returns the current branch name.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	return c.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
}

// CurrentCommit returns the current commit hash.
func (c *Client) CurrentCommit(ctx context.Context) (string, error) {
	return c.run(ctx, "rev-parse", "HEAD")
}

// CheckoutBranch switches to a branch (implements core.GitClient).
func (c *Client) CheckoutBranch(ctx context.Context, name string) error {
	_, err := c.run(ctx, "checkout", name)
	return err
}

// Checkout switches to a branch or creates it (internal use).
func (c *Client) Checkout(ctx context.Context, branch string, create bool) error {
	args := []string{"checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, branch)

	_, err := c.run(ctx, args...)
	return err
}

// CreateBranch creates a new branch from a base.
func (c *Client) CreateBranch(ctx context.Context, name, base string) error {
	args := []string{"checkout", "-b", name}
	if base != "" {
		args = append(args, base)
	}
	_, err := c.run(ctx, args...)
	return err
}

// DeleteBranch deletes a branch (implements core.GitClient).
func (c *Client) DeleteBranch(ctx context.Context, name string) error {
	_, err := c.run(ctx, "branch", "-d", name)
	return err
}

// DeleteBranchForce forcibly deletes a branch (internal use).
func (c *Client) DeleteBranchForce(ctx context.Context, name string) error {
	_, err := c.run(ctx, "branch", "-D", name)
	return err
}

// ListBranches returns all local branches.
func (c *Client) ListBranches(ctx context.Context) ([]string, error) {
	output, err := c.run(ctx, "branch", "--list", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}

	branches := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// BranchExists checks if a branch exists.
func (c *Client) BranchExists(ctx context.Context, name string) (bool, error) {
	branches, err := c.ListBranches(ctx)
	if err != nil {
		return false, err
	}
	for _, b := range branches {
		if b == name {
			return true, nil
		}
	}
	return false, nil
}

// Commit creates a commit with the given message.
func (c *Client) Commit(ctx context.Context, message string) (string, error) {
	_, err := c.run(ctx, "commit", "-m", message)
	if err != nil {
		return "", err
	}
	return c.CurrentCommit(ctx)
}

// CommitAll stages all changes and commits.
func (c *Client) CommitAll(ctx context.Context, message string) (string, error) {
	_, err := c.run(ctx, "add", "-A")
	if err != nil {
		return "", err
	}
	return c.Commit(ctx, message)
}

// Add stages files.
func (c *Client) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add"}, paths...)
	_, err := c.run(ctx, args...)
	return err
}

// Diff returns the diff between base and head (implements core.GitClient).
func (c *Client) Diff(ctx context.Context, base, head string) (string, error) {
	if base == "" && head == "" {
		// Return unstaged diff if no refs given
		return c.run(ctx, "diff")
	}
	if head == "" {
		head = "HEAD"
	}
	return c.run(ctx, "diff", base+"..."+head)
}

// DiffStaged returns the diff for staged changes (internal use).
func (c *Client) DiffStaged(ctx context.Context) (string, error) {
	return c.run(ctx, "diff", "--staged")
}

// DiffUnstaged returns the diff for unstaged changes (internal use).
func (c *Client) DiffUnstaged(ctx context.Context) (string, error) {
	return c.run(ctx, "diff")
}

// DiffFiles returns list of files changed between commits.
func (c *Client) DiffFiles(ctx context.Context, base, head string) ([]string, error) {
	output, err := c.run(ctx, "diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Log returns recent commit history.
func (c *Client) Log(ctx context.Context, count int) ([]Commit, error) {
	output, err := c.run(ctx, "log", fmt.Sprintf("-n%d", count),
		"--format=%H|%an|%ae|%s|%ci")
	if err != nil {
		return nil, err
	}

	commits := make([]Commit, 0)
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) == 5 {
			date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[4])
			commits = append(commits, Commit{
				Hash:        parts[0],
				AuthorName:  parts[1],
				AuthorEmail: parts[2],
				Subject:     parts[3],
				Date:        date,
			})
		}
	}
	return commits, nil
}

// Commit represents a git commit.
type Commit struct {
	Hash        string
	AuthorName  string
	AuthorEmail string
	Subject     string
	Date        time.Time
}

// Fetch fetches from remote.
func (c *Client) Fetch(ctx context.Context, remote string) error {
	_, err := c.run(ctx, "fetch", remote)
	return err
}

// Push pushes to remote (implements core.GitClient).
func (c *Client) Push(ctx context.Context, remote, branch string) error {
	_, err := c.run(ctx, "push", remote, branch)
	return err
}

// PushForce pushes to remote with force-with-lease (internal use).
func (c *Client) PushForce(ctx context.Context, remote, branch string) error {
	_, err := c.run(ctx, "push", remote, branch, "--force-with-lease")
	return err
}

// Pull pulls from remote.
func (c *Client) Pull(ctx context.Context, remote, branch string) error {
	_, err := c.run(ctx, "pull", remote, branch)
	return err
}

// Stash stashes current changes.
func (c *Client) Stash(ctx context.Context, message string) error {
	args := []string{"stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	_, err := c.run(ctx, args...)
	return err
}

// StashPop pops the top stash.
func (c *Client) StashPop(ctx context.Context) error {
	_, err := c.run(ctx, "stash", "pop")
	return err
}

// Clean removes untracked files.
func (c *Client) Clean(ctx context.Context, directories, force bool) error {
	args := []string{"clean"}
	if force {
		args = append(args, "-f")
	}
	if directories {
		args = append(args, "-d")
	}
	_, err := c.run(ctx, args...)
	return err
}

// Reset resets the repository.
func (c *Client) Reset(ctx context.Context, mode, ref string) error {
	args := []string{"reset", "--" + mode}
	if ref != "" {
		args = append(args, ref)
	}
	_, err := c.run(ctx, args...)
	return err
}

// RepoPath returns the repository path.
func (c *Client) RepoPath() string {
	return c.repoPath
}

// WithTimeout sets the command timeout.
func (c *Client) WithTimeout(d time.Duration) *Client {
	c.timeout = d
	return c
}

// DefaultBranch returns the default branch (main or master).
func (c *Client) DefaultBranch(ctx context.Context) (string, error) {
	// Try to detect from remote
	output, err := c.run(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(output, "refs/remotes/origin/"), nil
	}

	// Fallback: check if main or master exists
	branches, _ := c.ListBranches(ctx)
	for _, b := range branches {
		if b == "main" {
			return "main", nil
		}
	}
	for _, b := range branches {
		if b == "master" {
			return "master", nil
		}
	}

	// Default to main
	return "main", nil
}

// RemoteURL returns the URL of the origin remote (implements core.GitClient).
func (c *Client) RemoteURL(ctx context.Context) (string, error) {
	return c.run(ctx, "remote", "get-url", "origin")
}

// RemoteURLFor returns the URL of a specific remote (internal use).
func (c *Client) RemoteURLFor(ctx context.Context, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return c.run(ctx, "remote", "get-url", remote)
}

// IsClean returns true if the working directory has no changes (implements core.GitClient).
func (c *Client) IsClean(ctx context.Context) (bool, error) {
	status, err := c.StatusLocal(ctx)
	if err != nil {
		return false, err
	}
	return status.IsClean(), nil
}

// CreateWorktree creates a new worktree for a branch (implements core.GitClient).
func (c *Client) CreateWorktree(ctx context.Context, path, branch string) error {
	if err := validateWorktreeBranch(branch); err != nil {
		return err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating worktree parent directory: %w", err)
	}

	// Determine if branch exists
	branches, err := c.ListBranches(ctx)
	if err != nil {
		return fmt.Errorf("listing branches: %w", err)
	}

	branchExists := false
	for _, b := range branches {
		if b == branch {
			branchExists = true
			break
		}
	}

	var args []string
	if branchExists {
		args = []string{"worktree", "add", path, branch}
	} else {
		args = []string{"worktree", "add", "-b", branch, path}
	}

	_, err = c.run(ctx, args...)
	return err
}

// RemoveWorktree removes a worktree (implements core.GitClient).
func (c *Client) RemoveWorktree(ctx context.Context, path string) error {
	_, err := c.run(ctx, "worktree", "remove", path)
	return err
}

// ListWorktrees returns all worktrees (implements core.GitClient).
func (c *Client) ListWorktrees(ctx context.Context) ([]core.Worktree, error) {
	output, err := c.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	return parseWorktreesToCore(output, c.repoPath), nil
}

// parseWorktreesToCore parses git worktree list output to core.Worktree slice.
func parseWorktreesToCore(output, mainRepoPath string) []core.Worktree {
	worktrees := make([]core.Worktree, 0)
	var current *core.Worktree

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "worktree "):
			if current != nil {
				worktrees = append(worktrees, *current)
			}
			path := strings.TrimPrefix(line, "worktree ")
			current = &core.Worktree{
				Path:   path,
				IsMain: path == mainRepoPath,
			}
		case current != nil:
			switch {
			case strings.HasPrefix(line, "HEAD "):
				current.Commit = strings.TrimPrefix(line, "HEAD ")
			case strings.HasPrefix(line, "branch "):
				current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			case line == "locked":
				current.IsLocked = true
			}
		}
	}

	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees
}

// Run executes a raw git command and returns the output.
// This is exposed for operations not covered by dedicated methods.
func (c *Client) Run(ctx context.Context, args ...string) (string, error) {
	return c.run(ctx, args...)
}

// HasUncommittedChanges returns true if the repository has uncommitted changes.
// This includes staged, unstaged, and untracked files.
func (c *Client) HasUncommittedChanges(ctx context.Context) (bool, error) {
	status, err := c.StatusLocal(ctx)
	if err != nil {
		return false, err
	}
	return !status.IsClean(), nil
}

// AbortMerge aborts an in-progress merge operation.
func (c *Client) AbortMerge(ctx context.Context) error {
	_, err := c.run(ctx, "merge", "--abort")
	return err
}

// AbortRebase aborts an in-progress rebase operation.
func (c *Client) AbortRebase(ctx context.Context) error {
	_, err := c.run(ctx, "rebase", "--abort")
	return err
}

// AbortCherryPick aborts an in-progress cherry-pick operation.
func (c *Client) AbortCherryPick(ctx context.Context) error {
	_, err := c.run(ctx, "cherry-pick", "--abort")
	return err
}
