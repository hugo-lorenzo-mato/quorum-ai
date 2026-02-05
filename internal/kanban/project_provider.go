package kanban

import (
	"context"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ProjectInfo contains basic information about a project for Kanban processing.
type ProjectInfo struct {
	ID   string
	Name string
	Path string
}

// ProjectStateProvider abstracts access to project-scoped StateManagers.
// This allows KanbanEngine to process workflows across multiple projects
// without being tightly coupled to the project package.
type ProjectStateProvider interface {
	// ListActiveProjects returns all projects that should be processed by Kanban.
	// Returns only healthy/accessible projects.
	ListActiveProjects(ctx context.Context) ([]ProjectInfo, error)

	// GetProjectStateManager returns the KanbanStateManager for a specific project.
	// Returns nil if the project doesn't exist or its StateManager doesn't support Kanban.
	GetProjectStateManager(ctx context.Context, projectID string) (KanbanStateManager, error)

	// GetProjectEventBus returns the EventBus for a specific project (for SSE events).
	// May return nil if project doesn't have an EventBus.
	GetProjectEventBus(ctx context.Context, projectID string) EventPublisher
}

// EventPublisher defines the interface for publishing events.
// Compatible with *events.EventBus.
type EventPublisher interface {
	Publish(event events.Event)
}

// SingleProjectProvider wraps a single StateManager for backwards compatibility.
// Used when running without multi-project support (legacy mode).
type SingleProjectProvider struct {
	stateManager KanbanStateManager
	eventBus     EventPublisher
	projectID    string
}

// NewSingleProjectProvider creates a provider for single-project mode.
func NewSingleProjectProvider(sm KanbanStateManager, eventBus EventPublisher) *SingleProjectProvider {
	return &SingleProjectProvider{
		stateManager: sm,
		eventBus:     eventBus,
		projectID:    "default",
	}
}

// ListActiveProjects returns the single default project.
func (p *SingleProjectProvider) ListActiveProjects(_ context.Context) ([]ProjectInfo, error) {
	if p.stateManager == nil {
		return nil, nil
	}
	return []ProjectInfo{{ID: p.projectID, Name: "Default", Path: ""}}, nil
}

// GetProjectStateManager returns the single StateManager.
func (p *SingleProjectProvider) GetProjectStateManager(_ context.Context, _ string) (KanbanStateManager, error) {
	return p.stateManager, nil
}

// GetProjectEventBus returns the single EventBus.
func (p *SingleProjectProvider) GetProjectEventBus(_ context.Context, _ string) EventPublisher {
	return p.eventBus
}
