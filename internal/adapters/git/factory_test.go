package git_test

import (
	"context"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestNewClientFactory(t *testing.T) {
	t.Parallel()
	factory := git.NewClientFactory()
	if factory == nil {
		t.Fatal("factory should not be nil")
	}
}

func TestClientFactory_NewClient(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	factory := git.NewClientFactory()
	client, err := factory.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	if client == nil {
		t.Fatal("client should not be nil")
	}

	// Verify it returns a core.GitClient
	var _ core.GitClient = client

	// Verify it works
	branch, err := client.CurrentBranch(context.Background())
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, branch, "main")
}

func TestClientFactory_NewClient_InvalidPath(t *testing.T) {
	t.Parallel()
	factory := git.NewClientFactory()

	// Non-git directory
	dir := testutil.TempDir(t)
	_, err := factory.NewClient(dir)
	testutil.AssertError(t, err)
}

func TestClientFactory_NewClient_NonexistentPath(t *testing.T) {
	t.Parallel()
	factory := git.NewClientFactory()

	_, err := factory.NewClient("/nonexistent/path/that/does/not/exist")
	testutil.AssertError(t, err)
}

func TestClientFactory_ImplementsInterface(t *testing.T) {
	t.Parallel()
	repo := testutil.NewGitRepo(t)
	repo.WriteFile("README.md", "# Test")
	repo.Commit("Initial commit")

	factory := git.NewClientFactory()

	// Verify the client returned satisfies core.GitClient
	client, err := factory.NewClient(repo.Path)
	testutil.AssertNoError(t, err)

	// Exercise multiple interface methods
	ctx := context.Background()

	root, err := client.RepoRoot(ctx)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, root, repo.Path)

	_, err = client.Status(ctx)
	testutil.AssertNoError(t, err)

	_, err = client.CurrentBranch(ctx)
	testutil.AssertNoError(t, err)

	_, err = client.IsClean(ctx)
	testutil.AssertNoError(t, err)
}
