package api

import (
	"context"
	"fmt"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/kanban"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// KanbanStatePoolProvider implements kanban.ProjectStateProvider using project.StatePool.
// This adapter bridges the kanban engine with the project management infrastructure.
type KanbanStatePoolProvider struct {
	pool     *project.StatePool
	registry project.Registry
}

// NewKanbanStatePoolProvider creates a new provider for multi-project Kanban support.
func NewKanbanStatePoolProvider(pool *project.StatePool, registry project.Registry) *KanbanStatePoolProvider {
	return &KanbanStatePoolProvider{
		pool:     pool,
		registry: registry,
	}
}

// ListActiveProjects returns all healthy projects from the registry.
func (p *KanbanStatePoolProvider) ListActiveProjects(ctx context.Context) ([]kanban.ProjectInfo, error) {
	if p.registry == nil {
		return nil, fmt.Errorf("registry not configured")
	}

	projects, err := p.registry.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	result := make([]kanban.ProjectInfo, 0, len(projects))
	for _, proj := range projects {
		// Only include healthy or degraded projects (skip offline)
		if proj.Status == project.StatusOffline {
			continue
		}

		result = append(result, kanban.ProjectInfo{
			ID:   proj.ID,
			Name: proj.Name,
			Path: proj.Path,
		})
	}

	return result, nil
}

// GetProjectStateManager returns the KanbanStateManager for a specific project.
func (p *KanbanStatePoolProvider) GetProjectStateManager(ctx context.Context, projectID string) (kanban.KanbanStateManager, error) {
	if p.pool == nil {
		return nil, fmt.Errorf("state pool not configured")
	}

	// Get or create project context from pool
	pc, err := p.pool.GetContext(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project context: %w", err)
	}

	if pc == nil || pc.StateManager == nil {
		return nil, fmt.Errorf("project has no state manager")
	}

	// Type assert to KanbanStateManager
	kanbanSM, ok := pc.StateManager.(kanban.KanbanStateManager)
	if !ok {
		return nil, fmt.Errorf("project state manager does not support Kanban operations")
	}

	return kanbanSM, nil
}

// GetProjectEventBus returns the EventBus for a specific project.
func (p *KanbanStatePoolProvider) GetProjectEventBus(ctx context.Context, projectID string) kanban.EventPublisher {
	if p.pool == nil {
		return nil
	}

	// Get project context (don't create if not exists)
	pc, err := p.pool.GetContext(ctx, projectID)
	if err != nil || pc == nil {
		return nil
	}

	return pc.EventBus
}

// GetProjectExecutionContext returns a context decorated with the ProjectContext.
// This is used for background execution (Kanban) so the executor sees project-scoped resources.
func (p *KanbanStatePoolProvider) GetProjectExecutionContext(ctx context.Context, projectID string) (context.Context, error) {
	if p.pool == nil {
		return nil, fmt.Errorf("state pool not configured")
	}

	pc, err := p.pool.GetContext(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("getting project context: %w", err)
	}
	if pc == nil {
		return nil, fmt.Errorf("project context not available")
	}

	return middleware.WithProjectContext(ctx, pc), nil
}

// Ensure KanbanStatePoolProvider implements kanban.ProjectStateProvider
var _ kanban.ProjectStateProvider = (*KanbanStatePoolProvider)(nil)
