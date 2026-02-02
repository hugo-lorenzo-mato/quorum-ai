# ProjectContext Encapsulation

## Summary

Implement a `ProjectContext` struct that encapsulates all project-specific resources (StateManager, EventBus, ConfigLoader, ChatStore, AttachmentStore) into a single, manageable unit that can be created on-demand, validated, and properly cleaned up.

## Context

The Quorum AI system currently uses a single-instance architecture where the API server holds exactly one StateManager, one EventBus, one ConfigLoader, and one root directory. For multi-project support, each project needs its own isolated set of resources:

- **StateManager**: Each project has its own `.quorum/state/state.db` database
- **EventBus**: Events should be scoped to the project they originate from
- **ConfigLoader**: Each project has its own `.quorum/config.yaml`
- **ChatStore**: Each project has its own chat history
- **AttachmentStore**: Attachments are project-specific

The `ProjectContext` is the foundational abstraction that encapsulates all of these resources, enabling proper management of multiple projects within a single server process.

## Implementation Details

### Files to Create

- `internal/project/context.go` - ProjectContext implementation
- `internal/project/context_test.go` - Comprehensive unit tests

### Key Features

1. **Resource Encapsulation**
   - StateManager for project's state database
   - EventBus for project-scoped events
   - ConfigLoader for project configuration
   - ChatStore for project chat history
   - AttachmentStore for project files

2. **Lifecycle Management**
   - Create contexts on-demand for specific projects
   - Validate contexts before use
   - Close and cleanup resources when done
   - Track creation and last access times

3. **Thread-Safe Access**
   - All public methods protected with RWMutex
   - Safe concurrent access to all resources
   - Proper synchronization for metadata updates

4. **Configuration Options**
   - Custom logger per context
   - Configurable state backend (SQLite or JSON)
   - Configurable event buffer size
   - Optional chat store (for testing)

### Constructor Pattern

```go
pc, err := NewProjectContext(projectID, projectRoot,
    WithContextLogger(logger),
    WithStateBackend("sqlite"),
    WithEventBufferSize(100),
)
defer pc.Close()
```

### Service Initializers

The context automatically initializes all required services during construction:

- State manager initialization with configurable backend
- Event bus creation with configurable buffer size
- Config loader setup for project root
- Chat store initialization (optional, logs warning if unavailable)
- Attachments store setup for project directory

### Validation and Lifecycle

- **Validate()**: Checks that project directory and all resources are still accessible
- **Touch()**: Updates last accessed timestamp for LRU eviction tracking
- **Close()**: Properly releases all resources and prevents further use
- **IsClosed()**: Returns whether context has been closed

## Acceptance Criteria

- [ ] `internal/project/context.go` implements `ProjectContext`
- [ ] Constructor validates project directory exists and has `.quorum` structure
- [ ] Constructor creates StateManager with configurable backend
- [ ] Constructor creates EventBus with configurable buffer size
- [ ] Constructor creates ConfigLoader for project root
- [ ] Constructor creates ChatStore and Attachments store
- [ ] Close() releases all resources properly
- [ ] IsClosed() returns correct state
- [ ] Validate() checks all resources are accessible
- [ ] Touch() updates LastAccessed timestamp
- [ ] All methods are thread-safe
- [ ] Unit tests cover all scenarios including error cases
- [ ] Logging provides operational visibility

## Testing Strategy

1. **Unit Tests**:
   - Creation with valid and invalid projects
   - Resource initialization verification
   - Validation of healthy and degraded projects
   - Lifecycle management (close, idempotency)
   - Concurrent access patterns
   - Configuration options

2. **Integration Tests** (with ProjectStatePool):
   - Multiple contexts created and evicted
   - Resource cleanup on eviction
   - State consistency across operations

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No race conditions under concurrent load
- Clear error messages for all failure cases
- Proper logging at INFO/WARN/ERROR levels
