package project

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Default pool configuration
const (
	DefaultMaxActiveContexts   = 5
	DefaultMinActiveContexts   = 2
	DefaultEvictionGracePeriod = 5 * time.Minute
	DefaultEventBufferSize     = 100
)

// PoolMetrics provides observability into pool behavior
type PoolMetrics struct {
	ActiveContexts int     `json:"active_contexts"`
	TotalHits      int64   `json:"total_hits"`
	TotalMisses    int64   `json:"total_misses"`
	TotalEvictions int64   `json:"total_evictions"`
	TotalErrors    int64   `json:"total_errors"`
	HitRate        float64 `json:"hit_rate"`
}

// poolOptions holds pool configuration
type poolOptions struct {
	logger              *slog.Logger
	maxActiveContexts   int
	minActiveContexts   int
	evictionGracePeriod time.Duration
	eventBufferSize     int
}

// PoolOption configures a StatePool
type PoolOption func(*poolOptions)

// WithPoolLogger sets the logger for the pool
func WithPoolLogger(logger *slog.Logger) PoolOption {
	return func(o *poolOptions) {
		o.logger = logger
	}
}

// WithMaxActiveContexts sets the maximum number of active contexts
func WithMaxActiveContexts(maxActive int) PoolOption {
	return func(o *poolOptions) {
		if maxActive > 0 {
			o.maxActiveContexts = maxActive
		}
	}
}

// WithMinActiveContexts sets the minimum number of contexts to keep
func WithMinActiveContexts(minActive int) PoolOption {
	return func(o *poolOptions) {
		if minActive >= 0 {
			o.minActiveContexts = minActive
		}
	}
}

// WithEvictionGracePeriod sets the grace period before eviction
func WithEvictionGracePeriod(d time.Duration) PoolOption {
	return func(o *poolOptions) {
		if d >= 0 {
			o.evictionGracePeriod = d
		}
	}
}

// WithPoolEventBufferSize sets the event buffer size for new contexts
func WithPoolEventBufferSize(size int) PoolOption {
	return func(o *poolOptions) {
		if size > 0 {
			o.eventBufferSize = size
		}
	}
}

// poolEntry wraps a ProjectContext with management metadata
type poolEntry struct {
	context      *ProjectContext
	lastAccessed time.Time
	accessCount  int64
	mu           sync.Mutex
}

// StatePool manages multiple ProjectContext instances with LRU eviction
type StatePool struct {
	registry    Registry
	contexts    map[string]*poolEntry
	accessOrder []string
	mu          sync.RWMutex
	opts        *poolOptions
	logger      *slog.Logger
	closed      bool

	// Metrics (accessed atomically)
	hits      int64
	misses    int64
	evictions int64
	errors    int64
}

// NewStatePool creates a new project state pool
func NewStatePool(registry Registry, opts ...PoolOption) *StatePool {
	// Apply default options
	options := &poolOptions{
		logger:              slog.Default(),
		maxActiveContexts:   DefaultMaxActiveContexts,
		minActiveContexts:   DefaultMinActiveContexts,
		evictionGracePeriod: DefaultEvictionGracePeriod,
		eventBufferSize:     DefaultEventBufferSize,
	}

	for _, opt := range opts {
		opt(options)
	}

	// Ensure min <= max
	if options.minActiveContexts > options.maxActiveContexts {
		options.minActiveContexts = options.maxActiveContexts
	}

	p := &StatePool{
		registry:    registry,
		contexts:    make(map[string]*poolEntry),
		accessOrder: make([]string, 0),
		opts:        options,
		logger:      options.logger.With("component", "state_pool"),
	}

	p.logger.Info("state pool initialized",
		"max_active", options.maxActiveContexts,
		"min_active", options.minActiveContexts,
		"eviction_grace", options.evictionGracePeriod,
		"state_backend", "sqlite")

	return p
}

// GetContext retrieves or creates a context for the given project
func (p *StatePool) GetContext(ctx context.Context, projectID string) (*ProjectContext, error) {
	// Fast path: check if already cached (read lock)
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("pool is closed")
	}

	if entry, ok := p.contexts[projectID]; ok {
		p.mu.RUnlock()

		// Update access time
		entry.mu.Lock()
		entry.lastAccessed = time.Now()
		entry.accessCount++
		entry.mu.Unlock()

		// Update LRU order
		p.updateAccessOrder(projectID)

		// Record hit
		atomic.AddInt64(&p.hits, 1)

		entry.context.Touch()
		return entry.context, nil
	}
	p.mu.RUnlock()

	// Slow path: need to create context (write lock)
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	// Double-check after acquiring write lock
	if entry, ok := p.contexts[projectID]; ok {
		entry.lastAccessed = time.Now()
		entry.accessCount++
		atomic.AddInt64(&p.hits, 1)
		entry.context.Touch()
		return entry.context, nil
	}

	// Record miss
	atomic.AddInt64(&p.misses, 1)

	// Get project info from registry
	project, err := p.registry.GetProject(ctx, projectID)
	if err != nil {
		atomic.AddInt64(&p.errors, 1)
		return nil, fmt.Errorf("project not found in registry: %w", err)
	}

	// Validate project before creating context
	if err := p.registry.ValidateProject(ctx, projectID); err != nil {
		p.logger.Warn("project validation failed, attempting to create anyway",
			"project_id", projectID, "error", err)
	}

	// Evict if at capacity
	if len(p.contexts) >= p.opts.maxActiveContexts {
		if err := p.evictLRULocked(); err != nil {
			p.logger.Warn("eviction failed", "error", err)
			// Continue anyway - we might still have room or can exceed temporarily
		}
	}

	// Create new context
	pc, err := NewProjectContext(projectID, project.Path,
		WithContextLogger(p.logger),
		WithEventBufferSize(p.opts.eventBufferSize),
	)
	if err != nil {
		atomic.AddInt64(&p.errors, 1)
		return nil, fmt.Errorf("creating project context: %w", err)
	}

	// Add to pool
	entry := &poolEntry{
		context:      pc,
		lastAccessed: time.Now(),
		accessCount:  1,
	}
	p.contexts[projectID] = entry
	p.accessOrder = append(p.accessOrder, projectID)

	// Update last accessed in registry
	_ = p.registry.TouchProject(ctx, projectID)

	p.logger.Info("created project context",
		"project_id", projectID,
		"project_name", project.Name,
		"path", project.Path,
		"active_contexts", len(p.contexts))

	return pc, nil
}

// updateAccessOrder moves the project to the end of the access order (most recent)
func (p *StatePool) updateAccessOrder(projectID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find and remove from current position
	for i, id := range p.accessOrder {
		if id == projectID {
			p.accessOrder = append(p.accessOrder[:i], p.accessOrder[i+1:]...)
			break
		}
	}

	// Add to end (most recently used)
	p.accessOrder = append(p.accessOrder, projectID)
}

// evictLRULocked evicts the least recently used context
// Caller must hold write lock
func (p *StatePool) evictLRULocked() error {
	if len(p.accessOrder) == 0 {
		return fmt.Errorf("no contexts to evict")
	}

	// Don't evict below minimum
	if len(p.contexts) <= p.opts.minActiveContexts {
		p.logger.Debug("at minimum active contexts, skipping eviction",
			"active", len(p.contexts),
			"min", p.opts.minActiveContexts)
		return nil
	}

	now := time.Now()

	// Iterate from oldest (start of list) to newest
	for i := 0; i < len(p.accessOrder); i++ {
		projectID := p.accessOrder[i]
		entry, ok := p.contexts[projectID]
		if !ok {
			// Entry missing, clean up access order
			p.accessOrder = append(p.accessOrder[:i], p.accessOrder[i+1:]...)
			i--
			continue
		}

		// Check grace period
		if now.Sub(entry.lastAccessed) < p.opts.evictionGracePeriod {
			continue // Too recently used
		}

		// Check for running workflows
		hasRunning, err := entry.context.HasRunningWorkflows(context.Background())
		if err != nil {
			p.logger.Warn("error checking running workflows",
				"project_id", projectID, "error", err)
			continue
		}
		if hasRunning {
			p.logger.Debug("skipping eviction due to running workflows",
				"project_id", projectID)
			continue
		}

		// Found a candidate - evict it
		return p.evictProjectLocked(projectID, i)
	}

	return fmt.Errorf("no eligible contexts for eviction (all have running workflows or are in grace period)")
}

// evictProjectLocked evicts a specific project
// Caller must hold write lock
func (p *StatePool) evictProjectLocked(projectID string, orderIndex int) error {
	entry, ok := p.contexts[projectID]
	if !ok {
		return fmt.Errorf("project not in pool: %s", projectID)
	}

	p.logger.Info("evicting project context",
		"project_id", projectID,
		"last_accessed", entry.lastAccessed,
		"access_count", entry.accessCount)

	// Close the context
	if err := entry.context.Close(); err != nil {
		p.logger.Error("error closing evicted context",
			"project_id", projectID, "error", err)
		// Continue with removal even if close fails
	}

	// Remove from pool
	delete(p.contexts, projectID)

	// Remove from access order
	if orderIndex >= 0 && orderIndex < len(p.accessOrder) {
		p.accessOrder = append(p.accessOrder[:orderIndex], p.accessOrder[orderIndex+1:]...)
	} else {
		// Find and remove
		for i, id := range p.accessOrder {
			if id == projectID {
				p.accessOrder = append(p.accessOrder[:i], p.accessOrder[i+1:]...)
				break
			}
		}
	}

	atomic.AddInt64(&p.evictions, 1)

	p.logger.Info("project context evicted",
		"project_id", projectID,
		"remaining_contexts", len(p.contexts))

	return nil
}

// EvictProject manually evicts a specific project from the pool
func (p *StatePool) EvictProject(_ context.Context, projectID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	return p.evictProjectLocked(projectID, -1)
}

// Close shuts down the pool and all managed contexts
func (p *StatePool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.logger.Info("closing state pool", "active_contexts", len(p.contexts))

	var errs []error
	for id, entry := range p.contexts {
		if err := entry.context.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing context %s: %w", id, err))
			p.logger.Error("error closing context", "project_id", id, "error", err)
		}
	}

	p.contexts = make(map[string]*poolEntry)
	p.accessOrder = nil
	p.closed = true

	p.logger.Info("state pool closed")

	if len(errs) > 0 {
		return fmt.Errorf("errors closing pool: %v", errs)
	}
	return nil
}

// IsClosed returns whether the pool is closed
func (p *StatePool) IsClosed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.closed
}

// GetMetrics returns current pool metrics
func (p *StatePool) GetMetrics() PoolMetrics {
	p.mu.RLock()
	activeContexts := len(p.contexts)
	p.mu.RUnlock()

	hits := atomic.LoadInt64(&p.hits)
	misses := atomic.LoadInt64(&p.misses)
	evictions := atomic.LoadInt64(&p.evictions)
	errors := atomic.LoadInt64(&p.errors)

	var hitRate float64
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return PoolMetrics{
		ActiveContexts: activeContexts,
		TotalHits:      hits,
		TotalMisses:    misses,
		TotalEvictions: evictions,
		TotalErrors:    errors,
		HitRate:        hitRate,
	}
}

// GetActiveProjects returns IDs of all currently loaded projects
func (p *StatePool) GetActiveProjects() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	projects := make([]string, 0, len(p.contexts))
	for id := range p.contexts {
		projects = append(projects, id)
	}
	return projects
}

// GetContextInfo returns information about a specific context (if loaded)
func (p *StatePool) GetContextInfo(projectID string) (lastAccessed time.Time, accessCount int64, loaded bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.contexts[projectID]
	if !ok {
		return time.Time{}, 0, false
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	return entry.lastAccessed, entry.accessCount, true
}

// Size returns the number of active contexts
func (p *StatePool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.contexts)
}

// ValidateAll validates all loaded contexts and updates their status
func (p *StatePool) ValidateAll(ctx context.Context) error {
	p.mu.RLock()
	projectIDs := make([]string, 0, len(p.contexts))
	for id := range p.contexts {
		projectIDs = append(projectIDs, id)
	}
	p.mu.RUnlock()

	var lastErr error
	for _, id := range projectIDs {
		p.mu.RLock()
		entry, ok := p.contexts[id]
		p.mu.RUnlock()

		if !ok {
			continue
		}

		if err := entry.context.Validate(ctx); err != nil {
			p.logger.Warn("context validation failed",
				"project_id", id, "error", err)
			lastErr = err

			// Update registry status
			_ = p.registry.ValidateProject(ctx, id)
		}
	}

	return lastErr
}

// Cleanup removes contexts for projects that no longer exist in the registry
func (p *StatePool) Cleanup(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("pool is closed")
	}

	var toRemove []string
	for id := range p.contexts {
		_, err := p.registry.GetProject(ctx, id)
		if err != nil {
			// Project no longer in registry
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		p.logger.Info("cleaning up orphaned context", "project_id", id)
		_ = p.evictProjectLocked(id, -1)
	}

	if len(toRemove) > 0 {
		p.logger.Info("cleanup completed", "removed", len(toRemove))
	}

	return nil
}

// Preload loads contexts for the specified projects
func (p *StatePool) Preload(ctx context.Context, projectIDs []string) error {
	for _, id := range projectIDs {
		if _, err := p.GetContext(ctx, id); err != nil {
			p.logger.Warn("failed to preload project", "project_id", id, "error", err)
		}
	}
	return nil
}

// IsLoaded checks if a project context is currently loaded in the pool
func (p *StatePool) IsLoaded(projectID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.contexts[projectID]
	return ok
}
