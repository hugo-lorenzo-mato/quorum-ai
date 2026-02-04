// Package middleware provides HTTP middleware for the Quorum AI API.
package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
)

// QueryProjectMiddleware creates middleware that extracts projectID from the
// ?project= query parameter and loads the corresponding ProjectContext.
//
// If no project is specified, it falls back to the registry's default project.
// If neither is available, the request proceeds without a project context
// (backward compatibility mode).
//
// When using the default project, deprecation headers are added to encourage
// migration to explicit project specification.
//
// Error responses:
//   - 404 Not Found: specified project doesn't exist
//   - 503 Service Unavailable: project context could not be loaded
func QueryProjectMiddleware(pool ProjectStatePool, registry ProjectRegistry, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			projectID := r.URL.Query().Get("project")
			useDefault := false

			// If no project specified, try default
			if projectID == "" {
				projectID = registry.GetDefaultProject()
				useDefault = true

				// No project and no default - proceed without context (legacy mode)
				if projectID == "" {
					logger.Debug("query project middleware: no project specified and no default, proceeding without context",
						"path", r.URL.Path,
						"method", r.Method,
					)
					next.ServeHTTP(w, r)
					return
				}
			}

			// Verify project exists
			if !registry.Exists(projectID) {
				logger.Warn("query project middleware: project not found",
					"project_id", projectID,
					"path", r.URL.Path,
					"method", r.Method,
				)
				http.Error(w, fmt.Sprintf(`{"error": "project not found: %s"}`, projectID), http.StatusNotFound)
				return
			}

			// Add deprecation headers if using default
			if useDefault {
				w.Header().Set("Deprecation", "true")
				w.Header().Set("Sunset", "2026-06-01")
				w.Header().Set("X-Project-ID", projectID)
			}

			// Store projectID in context immediately
			ctx := withProjectID(r.Context(), projectID)

			// Load context from pool
			pc, err := pool.GetContext(ctx, projectID)
			if err != nil {
				logger.Warn("query project middleware: failed to get project context",
					"project_id", projectID,
					"error", err,
					"path", r.URL.Path,
				)
				http.Error(w, fmt.Sprintf(`{"error": "project unavailable: %s"}`, projectID), http.StatusServiceUnavailable)
				return
			}

			if pc.IsClosed() {
				logger.Warn("query project middleware: project context is closed",
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

			logger.Debug("query project middleware: loaded project context",
				"project_id", projectID,
				"project_root", pc.ProjectRoot(),
				"using_default", useDefault,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
