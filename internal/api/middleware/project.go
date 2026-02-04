// Package middleware provides HTTP middleware for the Quorum AI API.
package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// projectContextKey is the context key for storing ProjectContext.
	projectContextKey contextKey = "projectContext"
	// projectIDKey is the context key for storing the project ID.
	projectIDKey contextKey = "projectID"
)

// ProjectContext defines the interface for project-scoped resources.
// This interface is implemented by the project.ProjectContext type.
type ProjectContext interface {
	// ProjectID returns the project identifier.
	ProjectID() string
	// ProjectRoot returns the project root directory path.
	ProjectRoot() string
	// IsClosed returns whether the context has been closed.
	IsClosed() bool
	// Touch updates the last accessed timestamp.
	Touch()
}

// ProjectRegistry defines the interface for project registration and lookup.
type ProjectRegistry interface {
	// GetDefaultProject returns the default project ID, or empty if not set.
	GetDefaultProject() string
	// Exists checks if a project with the given ID exists.
	Exists(id string) bool
}

// ProjectStatePool defines the interface for managing project contexts.
type ProjectStatePool interface {
	// GetContext returns a ProjectContext for the given project ID.
	// Returns an error if the project doesn't exist or context creation fails.
	GetContext(ctx context.Context, projectID string) (ProjectContext, error)
}

// GetProjectContext retrieves the ProjectContext from the request context.
// Returns nil if no context is set.
func GetProjectContext(ctx context.Context) ProjectContext {
	pc, _ := ctx.Value(projectContextKey).(ProjectContext)
	return pc
}

// GetProjectID retrieves the project ID from the request context.
// Returns empty string if no project ID is set.
func GetProjectID(ctx context.Context) string {
	id, _ := ctx.Value(projectIDKey).(string)
	return id
}

// WithProjectContext adds a ProjectContext to the request context.
func WithProjectContext(ctx context.Context, pc ProjectContext) context.Context {
	ctx = context.WithValue(ctx, projectContextKey, pc)
	if pc != nil {
		ctx = context.WithValue(ctx, projectIDKey, pc.ProjectID())
	}
	return ctx
}

// withProjectID adds just the project ID to the request context.
func withProjectID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, projectIDKey, id)
}

// ProjectContextMiddleware creates middleware that extracts projectID from URL
// and loads the corresponding ProjectContext from the pool.
//
// It expects the URL to contain a {projectID} parameter.
//
// Error responses:
//   - 400 Bad Request: projectID missing from URL
//   - 404 Not Found: project doesn't exist in registry
//   - 503 Service Unavailable: project context could not be loaded
func ProjectContextMiddleware(pool ProjectStatePool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			projectID := chi.URLParam(r, "projectID")
			if projectID == "" {
				logger.Warn("project context middleware: projectID missing from URL",
					"path", r.URL.Path,
					"method", r.Method,
				)
				http.Error(w, `{"error": "projectID is required"}`, http.StatusBadRequest)
				return
			}

			// Store projectID in context immediately for logging/debugging
			ctx := withProjectID(r.Context(), projectID)

			// Load context from pool
			pc, err := pool.GetContext(ctx, projectID)
			if err != nil {
				logger.Warn("project context middleware: failed to get project context",
					"project_id", projectID,
					"error", err,
					"path", r.URL.Path,
				)
				// Determine error type for appropriate status code
				// If pool returns error, we assume it's either not found or unavailable
				// The pool implementation should return typed errors for distinction
				http.Error(w, fmt.Sprintf(`{"error": "project not found or unavailable: %s"}`, projectID), http.StatusNotFound)
				return
			}

			if pc.IsClosed() {
				logger.Warn("project context middleware: project context is closed",
					"project_id", projectID,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error": "project context is unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			// Update last accessed time
			pc.Touch()

			// Add to request context
			ctx = WithProjectContext(ctx, pc)

			logger.Debug("project context middleware: loaded project context",
				"project_id", projectID,
				"project_root", pc.ProjectRoot(),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DefaultProjectMiddleware creates middleware for legacy endpoints that don't
// include projectID in the URL. It uses the registry's default project.
//
// This middleware adds deprecation headers to encourage migration to
// project-scoped endpoints.
//
// Error responses:
//   - 503 Service Unavailable: no default project configured
//   - 503 Service Unavailable: default project context could not be loaded
func DefaultProjectMiddleware(registry ProjectRegistry, pool ProjectStatePool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defaultID := registry.GetDefaultProject()
			if defaultID == "" {
				logger.Warn("default project middleware: no default project configured",
					"path", r.URL.Path,
					"method", r.Method,
				)
				http.Error(w, `{"error": "no default project configured, please specify a project"}`, http.StatusServiceUnavailable)
				return
			}

			// Add deprecation headers
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Sunset", "2026-06-01")
			w.Header().Set("Link", fmt.Sprintf(`</api/v1/projects/%s%s>; rel="successor-version"`, defaultID, r.URL.Path))

			// Store projectID in context
			ctx := withProjectID(r.Context(), defaultID)

			// Load context from pool
			pc, err := pool.GetContext(ctx, defaultID)
			if err != nil {
				logger.Warn("default project middleware: failed to get default project context",
					"project_id", defaultID,
					"error", err,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error": "default project unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			if pc.IsClosed() {
				logger.Warn("default project middleware: default project context is closed",
					"project_id", defaultID,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error": "default project context is unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			// Update last accessed time
			pc.Touch()

			// Add to request context
			ctx = WithProjectContext(ctx, pc)

			logger.Debug("default project middleware: using default project",
				"project_id", defaultID,
				"project_root", pc.ProjectRoot(),
				"deprecated", true,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireProjectContext creates middleware that validates a ProjectContext
// exists in the request context and is usable.
//
// This should be used after ProjectContextMiddleware or DefaultProjectMiddleware.
//
// Error responses:
//   - 500 Internal Server Error: no project context in request (middleware ordering issue)
//   - 503 Service Unavailable: project context is closed
func RequireProjectContext(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pc := GetProjectContext(r.Context())
			if pc == nil {
				logger.Error("require project context: no project context in request",
					"path", r.URL.Path,
					"method", r.Method,
					"hint", "ensure ProjectContextMiddleware or DefaultProjectMiddleware is applied before RequireProjectContext",
				)
				http.Error(w, `{"error": "internal error: project context not initialized"}`, http.StatusInternalServerError)
				return
			}

			if pc.IsClosed() {
				projectID := GetProjectID(r.Context())
				logger.Warn("require project context: project context is closed",
					"project_id", projectID,
					"path", r.URL.Path,
				)
				http.Error(w, `{"error": "project context is unavailable"}`, http.StatusServiceUnavailable)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
