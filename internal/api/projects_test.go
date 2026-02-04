package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
)

// mockRegistry implements project.Registry for testing.
type mockRegistry struct {
	projects       []*project.Project
	defaultID      string
	addError       error
	removeError    error
	updateError    error
	validateError  error
	getDefaultErr  error
	setDefaultErr  error
}

func (m *mockRegistry) ListProjects(ctx context.Context) ([]*project.Project, error) {
	result := make([]*project.Project, len(m.projects))
	for i, p := range m.projects {
		result[i] = p.Clone()
	}
	return result, nil
}

func (m *mockRegistry) GetProject(ctx context.Context, id string) (*project.Project, error) {
	for _, p := range m.projects {
		if p.ID == id {
			return p.Clone(), nil
		}
	}
	return nil, project.ErrProjectNotFound
}

func (m *mockRegistry) GetProjectByPath(ctx context.Context, path string) (*project.Project, error) {
	for _, p := range m.projects {
		if p.Path == path {
			return p.Clone(), nil
		}
	}
	return nil, project.ErrProjectNotFound
}

func (m *mockRegistry) AddProject(ctx context.Context, path string, opts *project.AddProjectOptions) (*project.Project, error) {
	if m.addError != nil {
		return nil, m.addError
	}
	p := &project.Project{
		ID:           "proj-new",
		Path:         path,
		Name:         opts.Name,
		Color:        opts.Color,
		Status:       project.StatusHealthy,
		LastAccessed: time.Now(),
		CreatedAt:    time.Now(),
	}
	if p.Name == "" {
		p.Name = "New Project"
	}
	m.projects = append(m.projects, p)
	return p.Clone(), nil
}

func (m *mockRegistry) RemoveProject(ctx context.Context, id string) error {
	if m.removeError != nil {
		return m.removeError
	}
	for i, p := range m.projects {
		if p.ID == id {
			m.projects = append(m.projects[:i], m.projects[i+1:]...)
			return nil
		}
	}
	return project.ErrProjectNotFound
}

func (m *mockRegistry) UpdateProject(ctx context.Context, p *project.Project) error {
	if m.updateError != nil {
		return m.updateError
	}
	for i, existing := range m.projects {
		if existing.ID == p.ID {
			m.projects[i] = p.Clone()
			return nil
		}
	}
	return project.ErrProjectNotFound
}

func (m *mockRegistry) ValidateProject(ctx context.Context, id string) error {
	if m.validateError != nil {
		return m.validateError
	}
	for _, p := range m.projects {
		if p.ID == id {
			return nil
		}
	}
	return project.ErrProjectNotFound
}

func (m *mockRegistry) ValidateAll(ctx context.Context) error {
	return nil
}

func (m *mockRegistry) GetDefaultProject(ctx context.Context) (*project.Project, error) {
	if m.getDefaultErr != nil {
		return nil, m.getDefaultErr
	}
	if m.defaultID == "" && len(m.projects) > 0 {
		return m.projects[0].Clone(), nil
	}
	for _, p := range m.projects {
		if p.ID == m.defaultID {
			return p.Clone(), nil
		}
	}
	return nil, project.ErrNoDefaultProject
}

func (m *mockRegistry) SetDefaultProject(ctx context.Context, id string) error {
	if m.setDefaultErr != nil {
		return m.setDefaultErr
	}
	for _, p := range m.projects {
		if p.ID == id {
			m.defaultID = id
			return nil
		}
	}
	return project.ErrProjectNotFound
}

func (m *mockRegistry) TouchProject(ctx context.Context, id string) error {
	for _, p := range m.projects {
		if p.ID == id {
			p.LastAccessed = time.Now()
			return nil
		}
	}
	return project.ErrProjectNotFound
}

func (m *mockRegistry) Reload() error {
	return nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func setupProjectsTest() (*mockRegistry, *ProjectsHandler, chi.Router) {
	reg := &mockRegistry{
		projects: []*project.Project{
			{
				ID:           "proj-1",
				Path:         "/home/user/project1",
				Name:         "Project One",
				Status:       project.StatusHealthy,
				Color:        "#4A90D9",
				LastAccessed: time.Now(),
				CreatedAt:    time.Now().Add(-24 * time.Hour),
			},
			{
				ID:           "proj-2",
				Path:         "/home/user/project2",
				Name:         "Project Two",
				Status:       project.StatusDegraded,
				StatusMessage: "Config not found",
				Color:        "#7B68EE",
				LastAccessed: time.Now(),
				CreatedAt:    time.Now().Add(-48 * time.Hour),
			},
		},
		defaultID: "proj-1",
	}

	handler := NewProjectsHandler(reg)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		handler.RegisterRoutes(r)
	})

	return reg, handler, r
}

func TestProjectsHandler_ListProjects(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response []ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 projects, got %d", len(response))
	}

	// Check first project is marked as default
	found := false
	for _, p := range response {
		if p.ID == "proj-1" && p.IsDefault {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected proj-1 to be marked as default")
	}
}

func TestProjectsHandler_GetProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "proj-1" {
		t.Errorf("expected ID proj-1, got %s", response.ID)
	}
	if response.Name != "Project One" {
		t.Errorf("expected name 'Project One', got %s", response.Name)
	}
}

func TestProjectsHandler_GetProject_NotFound(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestProjectsHandler_CreateProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	body := CreateProjectRequest{
		Path: "/home/user/newproject",
		Name: "New Project",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "New Project" {
		t.Errorf("expected name 'New Project', got %s", response.Name)
	}
}

func TestProjectsHandler_CreateProject_MissingPath(t *testing.T) {
	_, _, r := setupProjectsTest()

	body := CreateProjectRequest{
		Name: "No Path",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestProjectsHandler_UpdateProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	newName := "Updated Name"
	body := UpdateProjectRequest{
		Name: &newName,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/proj-1", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", response.Name)
	}
}

func TestProjectsHandler_DeleteProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/proj-2", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestProjectsHandler_DeleteProject_NotFound(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestProjectsHandler_ValidateProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/validate", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "proj-1" {
		t.Errorf("expected ID proj-1, got %s", response.ID)
	}
}

func TestProjectsHandler_GetDefaultProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/default", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "proj-1" {
		t.Errorf("expected default project ID proj-1, got %s", response.ID)
	}
	if !response.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestProjectsHandler_SetDefaultProject(t *testing.T) {
	_, _, r := setupProjectsTest()

	body := SetDefaultProjectRequest{
		ID: "proj-2",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/default", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var response ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ID != "proj-2" {
		t.Errorf("expected ID proj-2, got %s", response.ID)
	}
	if !response.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestProjectsHandler_SetDefaultProject_NotFound(t *testing.T) {
	_, _, r := setupProjectsTest()

	body := SetDefaultProjectRequest{
		ID: "nonexistent",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/default", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
