// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// ProjectResponse represents a project in API responses.
type ProjectResponse struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	LastAccessed  time.Time `json:"last_accessed"`
	Status        string    `json:"status"`
	StatusMessage string    `json:"status_message,omitempty"`
	Color         string    `json:"color,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	IsDefault     bool      `json:"is_default"`
}

// CreateProjectRequest is the request body for creating a project.
type CreateProjectRequest struct {
	Path  string `json:"path"`
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

// UpdateProjectRequest is the request body for updating a project.
type UpdateProjectRequest struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
	Path  *string `json:"path,omitempty"`
}

// SetDefaultProjectRequest is the request body for setting the default project.
type SetDefaultProjectRequest struct {
	ID string `json:"id"`
}

// ProjectsHandler handles project management endpoints.
type ProjectsHandler struct {
	registry project.Registry
}

// NewProjectsHandler creates a new projects handler.
func NewProjectsHandler(registry project.Registry) *ProjectsHandler {
	return &ProjectsHandler{
		registry: registry,
	}
}

// RegisterRoutes registers project routes on the given router.
func (h *ProjectsHandler) RegisterRoutes(r chi.Router) {
	r.Route("/projects", func(r chi.Router) {
		r.Get("/", h.handleListProjects)
		r.Post("/", h.handleCreateProject)
		r.Get("/default", h.handleGetDefaultProject)
		r.Put("/default", h.handleSetDefaultProject)

		r.Route("/{projectID}", func(r chi.Router) {
			r.Get("/", h.handleGetProject)
			r.Put("/", h.handleUpdateProject)
			r.Delete("/", h.handleDeleteProject)
			r.Post("/validate", h.handleValidateProject)
		})
	})
}

// handleListProjects returns all registered projects.
// GET /api/v1/projects
func (h *ProjectsHandler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Reload registry to pick up changes from CLI or other processes
	if err := h.registry.Reload(); err != nil {
		// Log but don't fail - we can still serve stale data
		// In production this would use a proper logger
	}

	projects, err := h.registry.ListProjects(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	// Get default project ID for is_default flag
	defaultProject, _ := h.registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	response := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		response[i] = projectToResponse(p, defaultID)
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetProject returns a specific project by ID.
// GET /api/v1/projects/{projectID}
func (h *ProjectsHandler) handleGetProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "projectID")

	if projectID == "" {
		respondError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	p, err := h.registry.GetProject(ctx, projectID)
	if err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	// Get default project ID for is_default flag
	defaultProject, _ := h.registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	respondJSON(w, http.StatusOK, projectToResponse(p, defaultID))
}

// handleCreateProject registers a new project.
// POST /api/v1/projects
func (h *ProjectsHandler) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	opts := &project.AddProjectOptions{
		Name:  req.Name,
		Color: req.Color,
	}

	p, err := h.registry.AddProject(ctx, req.Path, opts)
	if err != nil {
		switch err {
		case project.ErrProjectAlreadyExists:
			respondError(w, http.StatusConflict, "project already exists")
		case project.ErrNotQuorumProject:
			respondError(w, http.StatusBadRequest, "path is not a valid Quorum project (missing .quorum directory)")
		case project.ErrInvalidPath:
			respondError(w, http.StatusBadRequest, "invalid path")
		default:
			respondError(w, http.StatusInternalServerError, "failed to add project")
		}
		return
	}

	// Get default project ID for is_default flag
	defaultProject, _ := h.registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	respondJSON(w, http.StatusCreated, projectToResponse(p, defaultID))
}

// handleUpdateProject updates project metadata.
// PUT /api/v1/projects/{projectID}
func (h *ProjectsHandler) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "projectID")

	if projectID == "" {
		respondError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	var req UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing project
	p, err := h.registry.GetProject(ctx, projectID)
	if err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	// Update fields
	updated := false
	if req.Name != nil {
		p.Name = *req.Name
		updated = true
	}
	if req.Color != nil {
		p.Color = *req.Color
		updated = true
	}
	if req.Path != nil && *req.Path != "" {
		// Validate the new path
		newPath := *req.Path

		// Convert to absolute path
		absPath, err := filepath.Abs(newPath)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid path format")
			return
		}

		// Check if path exists and is a directory
		fileInfo, err := os.Stat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				respondError(w, http.StatusBadRequest, "path does not exist")
				return
			}
			respondError(w, http.StatusBadRequest, "cannot access path")
			return
		}
		if !fileInfo.IsDir() {
			respondError(w, http.StatusBadRequest, "path is not a directory")
			return
		}

		// Check if another project already uses this path (only if path changed)
		if absPath != p.Path {
			existing, _ := h.registry.GetProjectByPath(ctx, absPath)
			if existing != nil && existing.ID != p.ID {
				respondError(w, http.StatusConflict, "another project is already registered at this path")
				return
			}
		}

		p.Path = absPath
		updated = true
	}

	if !updated {
		respondError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	if err := h.registry.UpdateProject(ctx, p); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update project")
		return
	}

	// Re-validate project status after path change
	if req.Path != nil {
		_ = h.registry.ValidateProject(ctx, p.ID)
		// Refresh project data after validation
		p, _ = h.registry.GetProject(ctx, p.ID)
	}

	// Get default project ID for is_default flag
	defaultProject, _ := h.registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	respondJSON(w, http.StatusOK, projectToResponse(p, defaultID))
}

// handleDeleteProject removes a project from the registry.
// DELETE /api/v1/projects/{projectID}
func (h *ProjectsHandler) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "projectID")

	if projectID == "" {
		respondError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	err := h.registry.RemoveProject(ctx, projectID)
	if err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to remove project")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleValidateProject validates a project's status.
// POST /api/v1/projects/{projectID}/validate
func (h *ProjectsHandler) handleValidateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := chi.URLParam(r, "projectID")

	if projectID == "" {
		respondError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	err := h.registry.ValidateProject(ctx, projectID)
	if err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		// Validation errors are returned but project may still exist
		// Re-fetch project to return current status
	}

	// Get updated project
	p, err := h.registry.GetProject(ctx, projectID)
	if err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	// Get default project ID for is_default flag
	defaultProject, _ := h.registry.GetDefaultProject(ctx)
	defaultID := ""
	if defaultProject != nil {
		defaultID = defaultProject.ID
	}

	respondJSON(w, http.StatusOK, projectToResponse(p, defaultID))
}

// handleGetDefaultProject returns the default project.
// GET /api/v1/projects/default
func (h *ProjectsHandler) handleGetDefaultProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p, err := h.registry.GetDefaultProject(ctx)
	if err != nil {
		if err == project.ErrNoDefaultProject {
			respondError(w, http.StatusNotFound, "no default project configured")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get default project")
		return
	}

	respondJSON(w, http.StatusOK, projectToResponse(p, p.ID))
}

// handleSetDefaultProject sets the default project.
// PUT /api/v1/projects/default
func (h *ProjectsHandler) handleSetDefaultProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req SetDefaultProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		respondError(w, http.StatusBadRequest, "project ID is required")
		return
	}

	if err := h.registry.SetDefaultProject(ctx, req.ID); err != nil {
		if err == project.ErrProjectNotFound {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to set default project")
		return
	}

	// Get updated project
	p, err := h.registry.GetProject(ctx, req.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	respondJSON(w, http.StatusOK, projectToResponse(p, req.ID))
}

// projectToResponse converts a Project to a ProjectResponse.
func projectToResponse(p *project.Project, defaultID string) ProjectResponse {
	return ProjectResponse{
		ID:            p.ID,
		Path:          p.Path,
		Name:          p.Name,
		LastAccessed:  p.LastAccessed,
		Status:        string(p.Status),
		StatusMessage: p.StatusMessage,
		Color:         p.Color,
		CreatedAt:     p.CreatedAt,
		IsDefault:     p.ID == defaultID,
	}
}
