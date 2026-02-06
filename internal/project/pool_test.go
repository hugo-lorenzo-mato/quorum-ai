package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockRegistry implements Registry for testing
type mockRegistry struct {
	projects map[string]*Project
	mu       sync.RWMutex
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		projects: make(map[string]*Project),
	}
}

func (m *mockRegistry) ListProjects(ctx context.Context) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	projects := make([]*Project, 0, len(m.projects))
	for _, p := range m.projects {
		projects = append(projects, p.Clone())
	}
	return projects, nil
}

func (m *mockRegistry) GetProject(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.projects[id]; ok {
		return p.Clone(), nil
	}
	return nil, ErrProjectNotFound
}

func (m *mockRegistry) GetProjectByPath(ctx context.Context, path string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.projects {
		if p.Path == path {
			return p.Clone(), nil
		}
	}
	return nil, ErrProjectNotFound
}

func (m *mockRegistry) AddProject(ctx context.Context, path string, opts *AddProjectOptions) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := generateProjectID()
	p := &Project{
		ID:        id,
		Path:      path,
		Name:      filepath.Base(path),
		Status:    StatusHealthy,
		CreatedAt: time.Now(),
	}
	m.projects[id] = p
	return p.Clone(), nil
}

func (m *mockRegistry) RemoveProject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.projects, id)
	return nil
}

func (m *mockRegistry) UpdateProject(ctx context.Context, project *Project) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.projects[project.ID] = project.Clone()
	return nil
}

func (m *mockRegistry) ValidateProject(ctx context.Context, id string) error {
	return nil
}

func (m *mockRegistry) ValidateAll(ctx context.Context) error {
	return nil
}

func (m *mockRegistry) GetDefaultProject(ctx context.Context) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.projects {
		return p.Clone(), nil
	}
	return nil, ErrNoDefaultProject
}

func (m *mockRegistry) SetDefaultProject(ctx context.Context, id string) error {
	return nil
}

func (m *mockRegistry) TouchProject(ctx context.Context, id string) error {
	return nil
}

func (m *mockRegistry) Reload() error {
	return nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func createPoolTestProject(t *testing.T, baseDir, name string) string {
	t.Helper()

	projectDir := filepath.Join(baseDir, name)
	quorumDir := filepath.Join(projectDir, ".quorum")

	if err := os.MkdirAll(filepath.Join(quorumDir, "state"), 0o750); err != nil {
		t.Fatalf("failed to create test project: %v", err)
	}

	// Create a minimal config
	configPath := filepath.Join(quorumDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("version: 1\n"), 0o640); err != nil {
		t.Fatalf("failed to create config: %v", err)
	}

	return projectDir
}

func TestNewStatePool(t *testing.T) {
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}

	if pool.Size() != 0 {
		t.Errorf("expected 0 contexts, got %d", pool.Size())
	}

	if pool.IsClosed() {
		t.Error("expected pool to not be closed")
	}
}

func TestStatePoolWithOptions(t *testing.T) {
	registry := newMockRegistry()
	pool := NewStatePool(registry,
		WithMaxActiveContexts(10),
		WithMinActiveContexts(3),
		WithEvictionGracePeriod(10*time.Minute),
		WithPoolEventBufferSize(50),
	)
	defer pool.Close()

	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
}

func TestStatePoolGetContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-getcontext-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry, WithMaxActiveContexts(5))
	defer pool.Close()

	ctx := context.Background()

	// First access - miss
	pc, err := pool.GetContext(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetContext failed: %v", err)
	}

	if pc == nil {
		t.Fatal("expected non-nil context")
	}

	if pc.ID != project.ID {
		t.Errorf("expected ID %s, got %s", project.ID, pc.ID)
	}

	// Second access - hit
	pc2, err := pool.GetContext(ctx, project.ID)
	if err != nil {
		t.Fatalf("second GetContext failed: %v", err)
	}

	if pc != pc2 {
		t.Error("expected same context on second access")
	}

	// Check metrics
	metrics := pool.GetMetrics()
	if metrics.TotalMisses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.TotalMisses)
	}
	if metrics.TotalHits != 1 {
		t.Errorf("expected 1 hit, got %d", metrics.TotalHits)
	}
}

func TestStatePoolLRUEviction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-eviction-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 3 project directories
	dirs := make([]string, 3)
	for i := 0; i < 3; i++ {
		dirs[i] = createPoolTestProject(t, tmpDir, fmt.Sprintf("proj%d", i))
	}

	registry := newMockRegistry()
	var projectIDs []string
	for _, dir := range dirs {
		p, _ := registry.AddProject(context.Background(), dir, nil)
		projectIDs = append(projectIDs, p.ID)
	}

	// Pool with max 2 contexts, no grace period
	pool := NewStatePool(registry,
		WithMaxActiveContexts(2),
		WithMinActiveContexts(0),
		WithEvictionGracePeriod(0),
	)
	defer pool.Close()

	ctx := context.Background()

	// Load first two projects
	_, err = pool.GetContext(ctx, projectIDs[0])
	if err != nil {
		t.Fatalf("failed to load project 0: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	_, err = pool.GetContext(ctx, projectIDs[1])
	if err != nil {
		t.Fatalf("failed to load project 1: %v", err)
	}

	if pool.Size() != 2 {
		t.Errorf("expected 2 contexts, got %d", pool.Size())
	}

	time.Sleep(10 * time.Millisecond)

	// Load third project - should trigger eviction of first
	_, err = pool.GetContext(ctx, projectIDs[2])
	if err != nil {
		t.Fatalf("failed to load project 2: %v", err)
	}

	// Should still have 2 contexts
	if pool.Size() != 2 {
		t.Errorf("expected 2 contexts after eviction, got %d", pool.Size())
	}

	// Project 0 should be evicted
	if pool.IsLoaded(projectIDs[0]) {
		t.Error("expected project 0 to be evicted")
	}

	metrics := pool.GetMetrics()
	if metrics.TotalEvictions != 1 {
		t.Errorf("expected 1 eviction, got %d", metrics.TotalEvictions)
	}
}

func TestStatePoolConcurrentAccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-concurrent-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	var successCount int64

	// 10 concurrent accesses
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.GetContext(ctx, project.ID)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 10 {
		t.Errorf("expected 10 successful accesses, got %d", successCount)
	}

	// Should only create one context
	if pool.Size() != 1 {
		t.Errorf("expected 1 context, got %d", pool.Size())
	}
}

func TestStatePoolClose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-close-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	// Close pool
	err = pool.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !pool.IsClosed() {
		t.Error("expected pool to be closed")
	}

	// GetContext should fail after close
	_, err = pool.GetContext(ctx, project.ID)
	if err == nil {
		t.Error("expected error after close")
	}

	// Second close should be no-op
	err = pool.Close()
	if err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestStatePoolEvictProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-evict-manual-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	// Manual eviction
	err = pool.EvictProject(ctx, project.ID)
	if err != nil {
		t.Errorf("EvictProject failed: %v", err)
	}

	if pool.Size() != 0 {
		t.Errorf("expected 0 contexts after eviction, got %d", pool.Size())
	}

	// Context can be reloaded
	_, err = pool.GetContext(ctx, project.ID)
	if err != nil {
		t.Errorf("GetContext after eviction failed: %v", err)
	}
}

func TestStatePoolGetContextNotFound(t *testing.T) {
	registry := newMockRegistry()
	pool := NewStatePool(registry)
	defer pool.Close()

	_, err := pool.GetContext(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent project")
	}

	metrics := pool.GetMetrics()
	if metrics.TotalErrors != 1 {
		t.Errorf("expected 1 error, got %d", metrics.TotalErrors)
	}
}

func TestStatePoolGetActiveProjects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-active-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 2 projects
	dir1 := createPoolTestProject(t, tmpDir, "proj1")
	dir2 := createPoolTestProject(t, tmpDir, "proj2")

	registry := newMockRegistry()
	p1, _ := registry.AddProject(context.Background(), dir1, nil)
	p2, _ := registry.AddProject(context.Background(), dir2, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, p1.ID)
	_, _ = pool.GetContext(ctx, p2.ID)

	active := pool.GetActiveProjects()
	if len(active) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(active))
	}
}

func TestStatePoolGetContextInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-info-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	// Not loaded yet
	_, _, loaded := pool.GetContextInfo(project.ID)
	if loaded {
		t.Error("expected project not to be loaded")
	}

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	// Now loaded
	lastAccessed, accessCount, loaded := pool.GetContextInfo(project.ID)
	if !loaded {
		t.Error("expected project to be loaded")
	}
	if lastAccessed.IsZero() {
		t.Error("expected non-zero last accessed time")
	}
	if accessCount != 1 {
		t.Errorf("expected access count 1, got %d", accessCount)
	}
}

func TestStatePoolIsLoaded(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-loaded-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	if pool.IsLoaded(project.ID) {
		t.Error("expected project not to be loaded initially")
	}

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	if !pool.IsLoaded(project.ID) {
		t.Error("expected project to be loaded after GetContext")
	}
}

func TestStatePoolValidateAll(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-validate-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	// Should succeed
	err = pool.ValidateAll(ctx)
	if err != nil {
		t.Errorf("ValidateAll failed: %v", err)
	}
}

func TestStatePoolCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-cleanup-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	_, _ = pool.GetContext(ctx, project.ID)

	// Remove from registry
	registry.RemoveProject(ctx, project.ID)

	// Cleanup should remove orphaned context
	err = pool.Cleanup(ctx)
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}

	if pool.Size() != 0 {
		t.Errorf("expected 0 contexts after cleanup, got %d", pool.Size())
	}
}

func TestStatePoolPreload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-preload-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dir1 := createPoolTestProject(t, tmpDir, "proj1")
	dir2 := createPoolTestProject(t, tmpDir, "proj2")

	registry := newMockRegistry()
	p1, _ := registry.AddProject(context.Background(), dir1, nil)
	p2, _ := registry.AddProject(context.Background(), dir2, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()
	err = pool.Preload(ctx, []string{p1.ID, p2.ID})
	if err != nil {
		t.Errorf("Preload failed: %v", err)
	}

	if pool.Size() != 2 {
		t.Errorf("expected 2 contexts after preload, got %d", pool.Size())
	}
}

func TestStatePoolMetricsHitRate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pool-hitrate-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := createPoolTestProject(t, tmpDir, "test-project")

	registry := newMockRegistry()
	project, _ := registry.AddProject(context.Background(), projectDir, nil)

	pool := NewStatePool(registry)
	defer pool.Close()

	ctx := context.Background()

	// First access - miss
	_, _ = pool.GetContext(ctx, project.ID)

	// 4 more accesses - hits
	for i := 0; i < 4; i++ {
		_, _ = pool.GetContext(ctx, project.ID)
	}

	metrics := pool.GetMetrics()
	if metrics.TotalMisses != 1 {
		t.Errorf("expected 1 miss, got %d", metrics.TotalMisses)
	}
	if metrics.TotalHits != 4 {
		t.Errorf("expected 4 hits, got %d", metrics.TotalHits)
	}

	// Hit rate should be 4/5 = 0.8
	if metrics.HitRate < 0.79 || metrics.HitRate > 0.81 {
		t.Errorf("expected hit rate ~0.8, got %f", metrics.HitRate)
	}
}
