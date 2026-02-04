// Package middleware provides HTTP middleware for the Quorum AI API.
package middleware

import (
	"context"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// RegistryAdapter adapts project.Registry to the middleware.ProjectRegistry interface.
type RegistryAdapter struct {
	registry project.Registry
}

// NewRegistryAdapter creates a new RegistryAdapter.
func NewRegistryAdapter(registry project.Registry) *RegistryAdapter {
	return &RegistryAdapter{registry: registry}
}

// GetDefaultProject returns the default project ID, or empty if not set.
func (a *RegistryAdapter) GetDefaultProject() string {
	ctx := context.Background()
	p, err := a.registry.GetDefaultProject(ctx)
	if err != nil || p == nil {
		return ""
	}
	return p.ID
}

// Exists checks if a project with the given ID exists.
func (a *RegistryAdapter) Exists(id string) bool {
	ctx := context.Background()
	_, err := a.registry.GetProject(ctx, id)
	return err == nil
}

// StatePoolAdapter adapts project.StatePool to the middleware.ProjectStatePool interface.
type StatePoolAdapter struct {
	pool *project.StatePool
}

// NewStatePoolAdapter creates a new StatePoolAdapter.
func NewStatePoolAdapter(pool *project.StatePool) *StatePoolAdapter {
	return &StatePoolAdapter{pool: pool}
}

// GetContext returns a ProjectContext for the given project ID.
func (a *StatePoolAdapter) GetContext(ctx context.Context, projectID string) (ProjectContext, error) {
	return a.pool.GetContext(ctx, projectID)
}
