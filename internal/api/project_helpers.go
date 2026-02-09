package api

import (
	"context"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/api/middleware"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// GetStateManagerFromContext extracts the project-scoped StateManager from the context.
// If a ProjectContext is available and has a StateManager, it returns that.
// Otherwise, it returns the provided fallback StateManager.
// This function is used by components that need to operate on project-specific state
// but may also be called in contexts without a project (e.g., TUI/CLI mode).
func GetStateManagerFromContext(ctx context.Context, fallback core.StateManager) core.StateManager {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.StateManager != nil {
			return projectCtx.StateManager
		}
	}
	return fallback
}

// GetEventBusFromContext extracts the project-scoped EventBus from the context.
// If a ProjectContext is available and has an EventBus, it returns that.
// Otherwise, it returns the provided fallback EventBus.
func GetEventBusFromContext(ctx context.Context, fallback *events.EventBus) *events.EventBus {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.EventBus != nil {
			return projectCtx.EventBus
		}
	}
	return fallback
}

// GetProjectRootFromContext extracts the project root directory from the context.
// If a ProjectContext is available, returns its root path.
// Otherwise, returns an empty string (caller should use fallback behavior).
func GetProjectRootFromContext(ctx context.Context) string {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		return pc.ProjectRoot()
	}
	return ""
}

// getProjectStateManager extracts the project-scoped StateManager from request context.
// If a ProjectContext is available and has a StateManager, it returns that.
// Otherwise, it falls back to the server's global StateManager.
func (s *Server) getProjectStateManager(ctx context.Context) core.StateManager {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		// Type assert to get the concrete ProjectContext with StateManager
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.StateManager != nil {
			return projectCtx.StateManager
		}
	}
	// Fallback to global state manager
	return s.stateManager
}

// getProjectEventBus extracts the project-scoped EventBus from request context.
// If a ProjectContext is available and has an EventBus, it returns that.
// Otherwise, it falls back to the server's global EventBus.
func (s *Server) getProjectEventBus(ctx context.Context) *events.EventBus {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.EventBus != nil {
			return projectCtx.EventBus
		}
	}
	// Fallback to global event bus
	return s.eventBus
}

// getProjectConfigLoader extracts the project-scoped ConfigLoader from request context.
// If a ProjectContext is available and has a ConfigLoader, it returns that.
// Otherwise, it falls back to the server's global ConfigLoader.
func (s *Server) getProjectConfigLoader(ctx context.Context) *config.Loader {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.ConfigLoader != nil {
			return projectCtx.ConfigLoader
		}
	}
	// Fallback to global config loader
	return s.configLoader
}

// getProjectChatStore extracts the project-scoped ChatStore from request context.
// If a ProjectContext is available and has a ChatStore, it returns that.
// Otherwise, it falls back to the server's global ChatStore.
func (s *Server) getProjectChatStore(ctx context.Context) core.ChatStore {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.ChatStore != nil {
			return projectCtx.ChatStore
		}
	}
	// Fallback to global chat store
	return s.chatStore
}

// getProjectRootPath extracts the project root directory path from request context.
// If a ProjectContext is available, returns its root path.
// Otherwise, falls back to the server root (legacy mode) when set.
func (s *Server) getProjectRootPath(ctx context.Context) string {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		return pc.ProjectRoot()
	}
	if s.root != "" {
		return s.root
	}
	return ""
}

// getProjectAttachmentStore extracts the project-scoped attachment Store from request context.
// If a ProjectContext is available and has an Attachments store, it returns that.
// Otherwise, it falls back to the server's global attachment store.
func (s *Server) getProjectAttachmentStore(ctx context.Context) *attachments.Store {
	pc := middleware.GetProjectContext(ctx)
	if pc != nil {
		if projectCtx, ok := pc.(*project.ProjectContext); ok && projectCtx.Attachments != nil {
			return projectCtx.Attachments
		}
	}
	return s.attachments
}

// getProjectID extracts the project ID from request context.
// Returns empty string if no project context is set.
func getProjectID(ctx context.Context) string {
	return middleware.GetProjectID(ctx)
}
