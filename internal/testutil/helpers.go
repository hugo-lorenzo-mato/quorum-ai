package testutil

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ErrTest is a generic test error.
var ErrTest = errors.New("test error")

// TempDir creates a temporary directory for tests.
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "quorum-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// TempFile creates a temporary file with content.
func TempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

// AssertNoError fails if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertEqual fails if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// AssertContains fails if s does not contain substr.
func AssertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}

// AssertNotContains fails if s contains substr.
func AssertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("expected %q to not contain %q", s, substr)
	}
}

// AssertLen fails if len(s) != want.
func AssertLen[T any](t *testing.T, s []T, want int) {
	t.Helper()
	if len(s) != want {
		t.Fatalf("len() = %d, want %d", len(s), want)
	}
}

// AssertTrue fails if b is false.
func AssertTrue(t *testing.T, b bool, msg string) {
	t.Helper()
	if !b {
		t.Fatalf("expected true: %s", msg)
	}
}

// AssertFalse fails if b is true.
func AssertFalse(t *testing.T, b bool, msg string) {
	t.Helper()
	if b {
		t.Fatalf("expected false: %s", msg)
	}
}

// GitRepo is a temporary git repository for testing.
type GitRepo struct {
	Path string
	t    *testing.T
}

// NewGitRepo creates a new temporary git repository.
func NewGitRepo(t *testing.T) *GitRepo {
	t.Helper()

	dir := TempDir(t)

	repo := &GitRepo{
		Path: dir,
		t:    t,
	}

	repo.run("init")
	repo.run("config", "user.email", "test@example.com")
	repo.run("config", "user.name", "Test User")
	// Set default branch to main for consistency
	repo.run("checkout", "-b", "main")

	return repo
}

// run executes a git command in the repo.
func (r *GitRepo) run(args ...string) string {
	r.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %s: %v", args, output, err)
	}

	return strings.TrimSpace(string(output))
}

// Run executes a git command (exported for test access).
func (r *GitRepo) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path

	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

// WriteFile creates a file in the repo.
func (r *GitRepo) WriteFile(name, content string) {
	r.t.Helper()

	path := filepath.Join(r.Path, name)

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.t.Fatalf("creating directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing file: %v", err)
	}
}

// Commit stages all and commits.
func (r *GitRepo) Commit(message string) string {
	r.t.Helper()

	r.run("add", "-A")
	r.run("commit", "-m", message, "--allow-empty")

	return r.run("rev-parse", "HEAD")
}

// CreateBranch creates a new branch.
func (r *GitRepo) CreateBranch(name string) {
	r.t.Helper()
	r.run("checkout", "-b", name)
}

// Checkout switches to a branch.
func (r *GitRepo) Checkout(name string) {
	r.t.Helper()
	r.run("checkout", name)
}

// CurrentBranch returns the current branch name.
func (r *GitRepo) CurrentBranch() string {
	r.t.Helper()
	return r.run("rev-parse", "--abbrev-ref", "HEAD")
}

// CreateTag creates a tag.
func (r *GitRepo) CreateTag(name string) {
	r.t.Helper()
	r.run("tag", name)
}

// SetRemote sets up a remote (can be another GitRepo).
func (r *GitRepo) SetRemote(name, url string) {
	r.t.Helper()
	r.run("remote", "add", name, url)
}

// Clone creates a clone of this repository.
func (r *GitRepo) Clone(t *testing.T) *GitRepo {
	t.Helper()

	dir := TempDir(t)

	cmd := exec.Command("git", "clone", r.Path, dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("cloning repo: %v", err)
	}

	return &GitRepo{
		Path: dir,
		t:    t,
	}
}

// CreateBareRemote creates a bare repository to use as remote.
func CreateBareRemote(t *testing.T) string {
	t.Helper()

	dir := TempDir(t)

	cmd := exec.Command("git", "init", "--bare", dir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("creating bare repo: %v", err)
	}

	return dir
}
