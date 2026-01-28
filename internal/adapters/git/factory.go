package git

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ClientFactory implements workflow.GitClientFactory interface.
// It creates git clients for specific repository paths, enabling
// task finalization (commit, push, PR) in worktrees.
type ClientFactory struct{}

// NewClientFactory creates a new git client factory.
func NewClientFactory() *ClientFactory {
	return &ClientFactory{}
}

// NewClient creates a git client for the given repository path.
// Implements workflow.GitClientFactory interface.
func (f *ClientFactory) NewClient(repoPath string) (core.GitClient, error) {
	return NewClient(repoPath)
}
