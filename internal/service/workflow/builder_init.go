// Package workflow provides the workflow orchestration components.
package workflow

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/git"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/adapters/github"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/logging"
)

func init() {
	// Initialize git factories with concrete implementations
	SetGitFactories(
		// GitClient factory
		func(cwd string) (core.GitClient, error) {
			return git.NewClient(cwd)
		},
		// WorktreeManager factory
		func(gc core.GitClient, worktreeDir string, logger *logging.Logger) WorktreeManager {
			// Type assert to get the concrete git.Client
			if gitClient, ok := gc.(*git.Client); ok {
				return git.NewTaskWorktreeManager(gitClient, worktreeDir).WithLogger(logger)
			}
			return nil
		},
		// GitHubClient factory
		func() (core.GitHubClient, error) {
			return github.NewClientFromRepo()
		},
		// GitClientFactory factory
		func() GitClientFactory {
			return git.NewClientFactory()
		},
	)
}
