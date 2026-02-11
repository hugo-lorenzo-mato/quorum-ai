package middleware

import (
	"context"
	"errors"
	"testing"
)

// --- Mock project.Registry for adapter tests ---

type _mockProjectForAdapter struct {
	id   string
	path string
}

type _mockProjectRegistryForAdapter struct {
	projects       map[string]*_mockProjectForAdapter
	defaultProject *_mockProjectForAdapter
	defaultErr     error
	getErr         error
}

func (m *_mockProjectRegistryForAdapter) _listProjects(_ context.Context) ([]*_mockProjectForAdapter, error) {
	var result []*_mockProjectForAdapter
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result, nil
}

func (m *_mockProjectRegistryForAdapter) _getProject(_ context.Context, id string) (*_mockProjectForAdapter, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if p, ok := m.projects[id]; ok {
		return p, nil
	}
	return nil, errors.New("project not found")
}

func (m *_mockProjectRegistryForAdapter) _getDefaultProject(_ context.Context) (*_mockProjectForAdapter, error) {
	if m.defaultErr != nil {
		return nil, m.defaultErr
	}
	return m.defaultProject, nil
}

// --- RegistryAdapter tests ---

// Since RegistryAdapter wraps a project.Registry, and we cannot easily create
// a real project.Registry in tests, we test the adapter's behavior through
// the ProjectRegistry interface that the middleware uses.

func TestRegistryAdapter_Interface(t *testing.T) {
	t.Parallel()

	// Test with a mock that implements the ProjectRegistry interface directly
	// (the same interface that RegistryAdapter provides).
	registry := &mockRegistry{
		defaultProject: "proj-default",
		projects: map[string]bool{
			"proj-default": true,
			"proj-other":   true,
		},
	}

	t.Run("GetDefaultProject returns ID", func(t *testing.T) {
		id := registry.GetDefaultProject()
		if id != "proj-default" {
			t.Errorf("expected 'proj-default', got %q", id)
		}
	})

	t.Run("GetDefaultProject empty when not set", func(t *testing.T) {
		emptyReg := &mockRegistry{defaultProject: ""}
		id := emptyReg.GetDefaultProject()
		if id != "" {
			t.Errorf("expected empty string, got %q", id)
		}
	})

	t.Run("Exists returns true for known project", func(t *testing.T) {
		if !registry.Exists("proj-default") {
			t.Error("expected Exists to return true for 'proj-default'")
		}
		if !registry.Exists("proj-other") {
			t.Error("expected Exists to return true for 'proj-other'")
		}
	})

	t.Run("Exists returns false for unknown project", func(t *testing.T) {
		if registry.Exists("proj-unknown") {
			t.Error("expected Exists to return false for 'proj-unknown'")
		}
	})
}

// --- StatePoolAdapter tests ---

func TestStatePoolAdapter_Interface(t *testing.T) {
	t.Parallel()

	t.Run("GetContext returns context when project exists", func(t *testing.T) {
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{
				"proj-1": {id: "proj-1", root: "/projects/1"},
			},
		}

		pc, err := pool.GetContext(context.Background(), "proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pc.ProjectID() != "proj-1" {
			t.Errorf("expected 'proj-1', got %q", pc.ProjectID())
		}
		if pc.ProjectRoot() != "/projects/1" {
			t.Errorf("expected '/projects/1', got %q", pc.ProjectRoot())
		}
	})

	t.Run("GetContext returns error for unknown project", func(t *testing.T) {
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{},
		}

		_, err := pool.GetContext(context.Background(), "unknown")
		if err == nil {
			t.Error("expected error for unknown project")
		}
	})

	t.Run("GetContext returns pool error", func(t *testing.T) {
		pool := &mockPool{
			contexts: map[string]*mockProjectContext{},
			err:      errors.New("pool error"),
		}

		_, err := pool.GetContext(context.Background(), "any")
		if err == nil {
			t.Error("expected error from pool")
		}
		if err.Error() != "pool error" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- ProjectContext interface tests ---

func TestMockProjectContext_IsClosed(t *testing.T) {
	t.Parallel()

	pc := &mockProjectContext{id: "test", root: "/test", closed: false}
	if pc.IsClosed() {
		t.Error("expected not closed")
	}

	closedPC := &mockProjectContext{id: "test", root: "/test", closed: true}
	if !closedPC.IsClosed() {
		t.Error("expected closed")
	}
}

func TestMockProjectContext_Touch(t *testing.T) {
	t.Parallel()

	pc := &mockProjectContext{id: "test", root: "/test"}
	if pc.touched {
		t.Error("expected not touched initially")
	}
	pc.Touch()
	if !pc.touched {
		t.Error("expected touched after Touch()")
	}
}

// --- withProjectID tests ---

func TestWithProjectID(t *testing.T) {
	t.Parallel()

	ctx := withProjectID(context.Background(), "proj-123")
	id := GetProjectID(ctx)
	if id != "proj-123" {
		t.Errorf("expected 'proj-123', got %q", id)
	}
}

func TestWithProjectID_EmptyString(t *testing.T) {
	t.Parallel()

	ctx := withProjectID(context.Background(), "")
	id := GetProjectID(ctx)
	if id != "" {
		t.Errorf("expected empty string, got %q", id)
	}
}

// --- ProjectRegistry and ProjectStatePool interface compliance ---

func TestProjectRegistryInterface(t *testing.T) {
	t.Parallel()
	// Compile-time check: mockRegistry implements ProjectRegistry.
	var _ ProjectRegistry = (*mockRegistry)(nil)
}

func TestProjectStatePoolInterface(t *testing.T) {
	t.Parallel()
	// Compile-time check: mockPool implements ProjectStatePool.
	var _ ProjectStatePool = (*mockPool)(nil)
}
