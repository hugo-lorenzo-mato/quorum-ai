package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// DefaultMergeOptions returns sensible defaults for merge operations.
func DefaultMergeOptions() core.MergeOptions {
	return core.MergeOptions{
		Strategy:      "recursive",
		NoFastForward: false,
	}
}

// Git operation errors.
var (
	ErrMergeConflict    = errors.New("merge conflict")
	ErrRebaseConflict   = errors.New("rebase conflict")
	ErrNothingToMerge   = errors.New("nothing to merge")
	ErrNotOnBranch      = errors.New("not on a branch")
	ErrMergeInProgress  = errors.New("merge already in progress")
	ErrRebaseInProgress = errors.New("rebase already in progress")
	ErrBranchNotFound   = errors.New("branch not found")
)

// Compile-time interface conformance check.
var _ core.GitClient = (*Client)(nil)

// Client wraps git CLI operations.
type Client struct {
	repoPath string
	timeout  time.Duration
	gitPath  string
}

// NewClient creates a new git client.
func NewClient(repoPath string) (*Client, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	gitPath, err := resolveGitBinaryPath(absPath)
	if err != nil {
		return nil, err
	}

	client := &Client{
		repoPath: absPath,
		timeout:  30 * time.Second,
		gitPath:  gitPath,
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

	// Security note: exec.CommandContext does not invoke a shell, so arguments are
	// not subject to shell interpolation. We still validate the binary location
	// at construction time and validate user-controlled args in higher-level
	// methods to prevent option/argument injection into git itself.
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
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

// runWithOutput executes a git command and returns both stdout and stderr even on error.
// This is useful for commands like merge where conflict info is in stdout.
func (c *Client) runWithOutput(ctx context.Context, args ...string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// See security note in run().
	cmd := exec.CommandContext(ctx, c.gitPath, args...)
	cmd.Dir = c.repoPath

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout, stderr, core.ErrTimeout("git command timed out")
		}
		return stdout, stderr, err
	}

	return stdout, stderr, nil
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
	if err := validateGitBranchName(name); err != nil {
		return err
	}
	_, err := c.run(ctx, "checkout", name)
	return err
}

// Checkout switches to a branch or creates it (internal use).
func (c *Client) Checkout(ctx context.Context, branch string, create bool) error {
	if err := validateGitBranchName(branch); err != nil {
		return err
	}
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
	if err := validateGitBranchName(name); err != nil {
		return err
	}
	if base != "" {
		if err := validateGitRev(base); err != nil {
			return err
		}
	}
	args := []string{"checkout", "-b", name}
	if base != "" {
		args = append(args, base)
	}
	_, err := c.run(ctx, args...)
	return err
}

// DeleteBranch deletes a branch (implements core.GitClient).
func (c *Client) DeleteBranch(ctx context.Context, name string) error {
	if err := validateGitBranchName(name); err != nil {
		return err
	}
	_, err := c.run(ctx, "branch", "-d", name)
	return err
}

// DeleteBranchForce forcibly deletes a branch (internal use).
func (c *Client) DeleteBranchForce(ctx context.Context, name string) error {
	if err := validateGitBranchName(name); err != nil {
		return err
	}
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
	if err := validateGitBranchName(name); err != nil {
		return false, err
	}
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
	if err := validateGitMessage(message); err != nil {
		return "", err
	}
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
	for _, p := range paths {
		if err := validateGitPathArg(p); err != nil {
			return err
		}
	}
	// Use "--" to prevent option injection if a path starts with "-".
	args := append([]string{"add", "--"}, paths...)
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
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	_, err := c.run(ctx, "fetch", remote)
	return err
}

// Push pushes to remote (implements core.GitClient).
func (c *Client) Push(ctx context.Context, remote, branch string) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if err := validateGitBranchName(branch); err != nil {
		return err
	}
	_, err := c.run(ctx, "push", remote, branch)
	return err
}

// PushForce pushes to remote with force-with-lease (internal use).
func (c *Client) PushForce(ctx context.Context, remote, branch string) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if err := validateGitBranchName(branch); err != nil {
		return err
	}
	_, err := c.run(ctx, "push", remote, branch, "--force-with-lease")
	return err
}

// Pull pulls from remote.
func (c *Client) Pull(ctx context.Context, remote, branch string) error {
	if err := validateGitRemoteName(remote); err != nil {
		return err
	}
	if err := validateGitBranchName(branch); err != nil {
		return err
	}
	_, err := c.run(ctx, "pull", remote, branch)
	return err
}

func resolveGitBinaryPath(repoAbs string) (string, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found in PATH: %w", err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolving git path: %w", err)
	}

	real := abs
	if rr, err := filepath.EvalSymlinks(abs); err == nil {
		real = rr
	}

	info, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("stat git binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("git binary is not a regular file: %s", real)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("git binary is not executable: %s", real)
	}

	// Defensive: avoid executing a "git" that lives inside the repository itself.
	// This reduces risk if PATH is manipulated to include "." or repo directories.
	if isPathWithinDir(repoAbs, real) {
		return "", fmt.Errorf("refusing to execute git from within repository: %s", real)
	}

	return real, nil
}

func isPathWithinDir(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func validateGitRemoteName(remote string) error {
	if err := validateNoNul("remote", remote); err != nil {
		return err
	}
	if remote == "" {
		return core.ErrValidation("INVALID_REMOTE", "remote name must not be empty")
	}
	if strings.HasPrefix(remote, "-") {
		return core.ErrValidation("INVALID_REMOTE", "remote name must not start with '-'")
	}
	for _, r := range remote {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return core.ErrValidation("INVALID_REMOTE", fmt.Sprintf("remote name contains invalid character: %q", r))
	}
	return nil
}

func validateGitBranchName(name string) error {
	if err := validateNoNul("branch", name); err != nil {
		return err
	}
	if name == "" {
		return core.ErrValidation("INVALID_BRANCH", "branch name must not be empty")
	}
	if strings.HasPrefix(name, "-") {
		return core.ErrValidation("INVALID_BRANCH", "branch name must not start with '-'")
	}
	// Conservative refname validation (subset of `git check-ref-format --branch`).
	if strings.Contains(name, " ") || strings.Contains(name, "\t") || strings.Contains(name, "\n") || strings.Contains(name, "\r") {
		return core.ErrValidation("INVALID_BRANCH", "branch name must not contain whitespace")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || strings.Contains(name, "//") {
		return core.ErrValidation("INVALID_BRANCH", "branch name contains forbidden sequence")
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") {
		return core.ErrValidation("INVALID_BRANCH", "branch name has forbidden prefix/suffix")
	}
	for _, r := range name {
		switch r {
		case '~', '^', ':', '?', '*', '[', '\\':
			return core.ErrValidation("INVALID_BRANCH", fmt.Sprintf("branch name contains forbidden character: %q", r))
		}
		if r < 0x20 || r == 0x7f {
			return core.ErrValidation("INVALID_BRANCH", "branch name contains control character")
		}
	}
	if name == "@" {
		return core.ErrValidation("INVALID_BRANCH", "branch name '@' is not allowed")
	}
	return nil
}

func validateGitRev(rev string) error {
	if err := validateNoNul("rev", rev); err != nil {
		return err
	}
	if strings.HasPrefix(rev, "-") {
		return core.ErrValidation("INVALID_REV", "rev must not start with '-'")
	}
	return nil
}

func validateGitPathArg(p string) error {
	if err := validateNoNul("path", p); err != nil {
		return err
	}
	if p == "" {
		return core.ErrValidation("INVALID_PATH", "path must not be empty")
	}
	return nil
}

func validateGitMessage(msg string) error {
	if err := validateNoNul("message", msg); err != nil {
		return err
	}
	if msg == "" {
		return core.ErrValidation("INVALID_MESSAGE", "message must not be empty")
	}
	return nil
}

func validateNoNul(field, value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return core.ErrValidation("INVALID_INPUT", fmt.Sprintf("%s contains NUL byte", field))
	}
	return nil
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

// =============================================================================
// Merge Operations
// =============================================================================

// Merge merges a branch into the current branch.
func (c *Client) Merge(ctx context.Context, branch string, opts core.MergeOptions) error {
	args := []string{"merge"}

	// Add strategy
	if opts.Strategy != "" {
		args = append(args, "-s", opts.Strategy)
	}

	// Add strategy option
	if opts.StrategyOption != "" {
		args = append(args, "-X", opts.StrategyOption)
	}

	// Add flags
	if opts.NoCommit {
		args = append(args, "--no-commit")
	}
	if opts.NoFastForward {
		args = append(args, "--no-ff")
	}
	if opts.Squash {
		args = append(args, "--squash")
	}
	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}

	args = append(args, branch)

	stdout, stderr, err := c.runWithOutput(ctx, args...)
	if err != nil {
		// Check for conflict (git outputs conflict info to stdout)
		if strings.Contains(stdout, "CONFLICT") ||
			strings.Contains(stdout, "Automatic merge failed") ||
			strings.Contains(stderr, "CONFLICT") {
			return fmt.Errorf("%w: %s", ErrMergeConflict, stdout)
		}
		// Check for nothing to merge
		if strings.Contains(stdout, "Already up to date") ||
			strings.Contains(stderr, "Already up to date") {
			return nil // Not an error, just nothing to do
		}
		// Check for branch not found
		if strings.Contains(stderr, "not something we can merge") ||
			strings.Contains(stdout, "not something we can merge") {
			return fmt.Errorf("%w: %s", ErrBranchNotFound, branch)
		}
		return fmt.Errorf("git merge: %w: %s%s", err, stdout, stderr)
	}

	return nil
}

// AbortMerge aborts a merge in progress.
func (c *Client) AbortMerge(ctx context.Context) error {
	_, err := c.run(ctx, "merge", "--abort")
	if err != nil {
		// May fail if no merge in progress
		if strings.Contains(err.Error(), "no merge to abort") ||
			strings.Contains(err.Error(), "There is no merge to abort") {
			return nil
		}
		return err
	}
	return nil
}

// HasMergeConflicts checks if there are unresolved merge conflicts.
func (c *Client) HasMergeConflicts(ctx context.Context) (bool, error) {
	// Check if MERGE_HEAD exists (merge in progress)
	gitDir := c.findGitDir()
	mergeHead := filepath.Join(gitDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHead); os.IsNotExist(err) {
		return false, nil
	}

	// Check for unmerged files
	output, err := c.run(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(output) != "", nil
}

// GetConflictFiles returns the list of files with conflicts.
func (c *Client) GetConflictFiles(ctx context.Context) ([]string, error) {
	output, err := c.run(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return nil, nil
	}

	files := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// =============================================================================
// Rebase Operations
// =============================================================================

// Rebase rebases the current branch onto another branch.
func (c *Client) Rebase(ctx context.Context, onto string) error {
	stdout, stderr, err := c.runWithOutput(ctx, "rebase", onto)
	if err != nil {
		// Rebase conflict info can be in stdout or stderr
		if strings.Contains(stdout, "CONFLICT") ||
			strings.Contains(stderr, "CONFLICT") ||
			strings.Contains(stderr, "could not apply") ||
			strings.Contains(stdout, "could not apply") {
			return fmt.Errorf("%w: %s%s", ErrRebaseConflict, stdout, stderr)
		}
		return fmt.Errorf("git rebase: %w: %s%s", err, stdout, stderr)
	}
	return nil
}

// AbortRebase aborts a rebase in progress.
func (c *Client) AbortRebase(ctx context.Context) error {
	_, err := c.run(ctx, "rebase", "--abort")
	if err != nil {
		if strings.Contains(err.Error(), "No rebase in progress") ||
			strings.Contains(err.Error(), "no rebase in progress") {
			return nil
		}
		return err
	}
	return nil
}

// ContinueRebase continues a rebase after resolving conflicts.
func (c *Client) ContinueRebase(ctx context.Context) error {
	_, err := c.run(ctx, "rebase", "--continue")
	return err
}

// HasRebaseInProgress checks if a rebase is in progress.
func (c *Client) HasRebaseInProgress(_ context.Context) (bool, error) {
	gitDir := c.findGitDir()
	rebaseApply := filepath.Join(gitDir, "rebase-apply")
	rebaseMerge := filepath.Join(gitDir, "rebase-merge")

	if _, err := os.Stat(rebaseApply); err == nil {
		return true, nil
	}
	if _, err := os.Stat(rebaseMerge); err == nil {
		return true, nil
	}
	return false, nil
}

// =============================================================================
// Reset Operations
// =============================================================================

// ResetHard performs a hard reset to the given reference.
func (c *Client) ResetHard(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "reset", "--hard", ref)
	return err
}

// ResetSoft performs a soft reset to the given reference.
func (c *Client) ResetSoft(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "reset", "--soft", ref)
	return err
}

// =============================================================================
// Cherry-Pick Operations
// =============================================================================

// CherryPick cherry-picks a commit onto the current branch.
func (c *Client) CherryPick(ctx context.Context, commit string) error {
	stdout, stderr, err := c.runWithOutput(ctx, "cherry-pick", commit)
	if err != nil {
		// Cherry-pick conflict info can be in stdout or stderr
		if strings.Contains(stdout, "CONFLICT") ||
			strings.Contains(stderr, "CONFLICT") ||
			strings.Contains(stderr, "could not apply") {
			return fmt.Errorf("%w: commit %s", ErrMergeConflict, commit)
		}
		return fmt.Errorf("git cherry-pick: %w: %s%s", err, stdout, stderr)
	}
	return nil
}

// AbortCherryPick aborts a cherry-pick in progress.
func (c *Client) AbortCherryPick(ctx context.Context) error {
	_, err := c.run(ctx, "cherry-pick", "--abort")
	if err != nil {
		if strings.Contains(err.Error(), "no cherry-pick") ||
			strings.Contains(err.Error(), "error: no cherry-pick or revert in progress") {
			return nil
		}
		return err
	}
	return nil
}

// =============================================================================
// Query Operations
// =============================================================================

// RevParse resolves a revision to its SHA.
func (c *Client) RevParse(ctx context.Context, ref string) (string, error) {
	return c.run(ctx, "rev-parse", ref)
}

// IsAncestor checks if ancestor is an ancestor of commit.
func (c *Client) IsAncestor(ctx context.Context, ancestor, commit string) (bool, error) {
	_, err := c.run(ctx, "merge-base", "--is-ancestor", ancestor, commit)
	if err != nil {
		// Exit code 1 means not an ancestor
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// =============================================================================
// Status Operations
// =============================================================================

// HasUncommittedChanges checks if there are any uncommitted changes.
func (c *Client) HasUncommittedChanges(ctx context.Context) (bool, error) {
	// Check for staged changes
	staged, err := c.run(ctx, "diff", "--cached", "--name-only")
	if err != nil {
		return false, err
	}
	if staged != "" {
		return true, nil
	}

	// Check for unstaged changes
	unstaged, err := c.run(ctx, "diff", "--name-only")
	if err != nil {
		return false, err
	}
	if unstaged != "" {
		return true, nil
	}

	// Check for untracked files
	untracked, err := c.run(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return untracked != "", nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// findGitDir finds the .git directory for the repository.
// Handles both regular repositories and worktrees.
func (c *Client) findGitDir() string {
	gitPath := filepath.Join(c.repoPath, ".git")

	info, err := os.Stat(gitPath)
	if err != nil {
		return gitPath // Fallback to default
	}

	// If .git is a directory, it's a regular repo
	if info.IsDir() {
		return gitPath
	}

	// If .git is a file, it's a worktree - read the path from it
	content, err := os.ReadFile(gitPath)
	if err != nil {
		return gitPath
	}

	// Parse "gitdir: /path/to/main/.git/worktrees/name"
	gitdir := strings.TrimSpace(string(content))
	if strings.HasPrefix(gitdir, "gitdir: ") {
		return strings.TrimPrefix(gitdir, "gitdir: ")
	}

	return gitPath
}
