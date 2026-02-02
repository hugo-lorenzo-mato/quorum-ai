# ProjectStatePool - Context Management and Caching

## Summary

Implement a `ProjectStatePool` that manages multiple `ProjectContext` instances, providing lazy initialization, intelligent caching, LRU eviction with memory constraints, and running workflow protection to enable efficient multi-project support.

## Context

Each `ProjectContext` consumes approximately 8MB of memory (StateManager, EventBus, ConfigLoader, ChatStore, Attachments, and context overhead). Without active management, a server handling many projects would consume unbounded memory. The pool solves this by:

- **Lazy Initialization**: Contexts created only when first accessed
- **Caching**: Active contexts kept in memory for fast repeated access
- **LRU Eviction**: When at capacity, least-recently-used contexts are evicted
- **Protection**: Contexts with running workflows are never evicted
- **Metrics**: Observable pool behavior for monitoring

## Implementation Details

### Files to Create

- `internal/project/pool.go` - ProjectStatePool implementation
- `internal/project/pool_test.go` - Comprehensive unit tests

### Key Features

1. **Lazy Context Loading**
   - Contexts created on-demand, not up-front
   - Fast repeated access via caching
   - Automatic state machine initialization

2. **LRU Eviction**
   - Maximum active contexts limit (default: 5)
   - Minimum active contexts threshold (default: 2)
   - Eviction grace period to prevent thrashing (default: 5 min)
   - Running workflows protect contexts from eviction

3. **Cache Hit Tracking**
   - Total hits and misses
   - Hit rate calculation
   - Eviction count monitoring
   - Error tracking for observability

4. **Thread-Safe Concurrent Access**
   - Read lock for cache hits (fast path)
   - Write lock for cache misses
   - Atomic counters for metrics
   - No race conditions under load

### Pool Configuration

```go
pool := NewStatePool(registry,
    WithMaxActiveContexts(10),
    WithMinActiveContexts(2),
    WithEvictionGracePeriod(5*time.Minute),
    WithStateBackend("sqlite"),
    WithPoolEventBufferSize(100),
)
```

### Core Methods

- **GetContext(ctx, projectID)**: Get or create context for project
- **EvictProject(ctx, projectID)**: Manually evict a specific project
- **Close()**: Shutdown pool and all contexts
- **GetMetrics()**: Get performance metrics
- **GetActiveProjects()**: List currently loaded projects
- **ValidateAll(ctx)**: Check health of all loaded contexts
- **Cleanup(ctx)**: Remove contexts for deleted projects
- **Preload(ctx, projectIDs)**: Pre-warm contexts

## Acceptance Criteria

- [ ] `internal/project/pool.go` implements `StatePool`
- [ ] Lazy context creation on first access
- [ ] Context caching with correct LRU ordering
- [ ] LRU eviction when at capacity
- [ ] Grace period prevents eviction of recently accessed contexts
- [ ] Running workflows prevent eviction
- [ ] Metrics track hits, misses, evictions, errors
- [ ] Thread-safe concurrent access
- [ ] GetContext handles unknown projects with error
- [ ] Close properly cleans up all contexts
- [ ] Manual eviction works correctly
- [ ] ValidateAll checks all loaded contexts
- [ ] No race conditions under concurrent load
- [ ] Unit tests cover all scenarios
- [ ] Proper logging at operational checkpoints

## Testing Strategy

1. **Unit Tests**:
   - Pool creation with default and custom options
   - GetContext cache hits and misses
   - LRU eviction mechanics
   - Grace period enforcement
   - Running workflow protection
   - Concurrent access patterns
   - Metrics calculation
   - Error handling

2. **Load Tests**:
   - High-volume concurrent access to same project
   - Concurrent access to different projects
   - Eviction under memory pressure

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No race conditions detected (use race detector)
- Metrics accurately reflect pool behavior
- Proper error handling for edge cases
