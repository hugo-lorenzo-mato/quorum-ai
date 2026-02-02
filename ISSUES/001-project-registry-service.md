# Project Registry Service Implementation

## Summary

Implement a complete `ProjectRegistry` service that tracks all Quorum projects registered on the user's machine. This is foundational for multi-project support, enabling the system to discover, validate, and manage multiple projects with independent state.

## Context

Currently, Quorum operates on a single-project paradigm where the working directory determines the project context. The ProjectRegistry provides centralized project management with:
- Fast lookup of registered projects by ID
- Persistence across server restarts
- Project metadata (name, path, status, last accessed)
- Health validation for project accessibility
- Foundation for ProjectStatePool lazy loading

## Implementation Details

### Files to Create

- `internal/project/types.go` - Type definitions (Project, RegistryConfig, ProjectStatus)
- `internal/project/errors.go` - Project-specific error types
- `internal/project/registry.go` - Core registry implementation
- `internal/project/registry_test.go` - Comprehensive unit tests

### Key Features

1. **CRUD Operations**
   - Add projects from filesystem paths
   - List all registered projects
   - Get project by ID or path
   - Update project metadata
   - Remove projects from registry

2. **Project Health Validation**
   - Detect healthy/degraded/offline/initializing status
   - Periodic validation with automatic status updates
   - Handle edge cases (deleted dirs, permission changes, corrupted configs)

3. **Project ID Generation**
   - Cryptographically random IDs that don't expose paths
   - Format: `proj-{12 hex chars}`

4. **Persistence**
   - YAML file at `~/.config/quorum/projects.yaml`
   - Atomic save with backup
   - Thread-safe concurrent access

5. **Default Project**
   - Support for legacy endpoints compatibility
   - Automatic fallback logic

### Registry File Format

```yaml
version: 1
default_project: "proj-abc123"
projects:
  - id: "proj-abc123"
    path: "/home/user/projects/alpha"
    name: "Project Alpha"
    last_accessed: "2025-01-15T10:30:00Z"
    status: "healthy"
    color: "#4A90D9"
    created_at: "2025-01-10T00:00:00Z"
```

## Acceptance Criteria

- [ ] All CRUD operations work correctly
- [ ] Project validation detects all status states
- [ ] Project IDs are cryptographically random
- [ ] Registry persists to `~/.config/quorum/projects.yaml`
- [ ] Atomic save with backup prevents corruption
- [ ] Default project tracking works
- [ ] All unit tests pass (>80% coverage)
- [ ] Concurrent access is thread-safe
- [ ] Proper logging for operational visibility

## Error Handling

| Scenario | Status Code | Message |
|----------|-------------|---------|
| Invalid path | 400 | "Path must be absolute" |
| Not a Quorum project | 400 | "Not a valid quorum project" |
| Project not found | 404 | "Project not found" |
| Already registered | 409 | "Project already registered" |
| Directory inaccessible | 503 | "Project directory not accessible" |

## Testing Strategy

1. **Unit tests** for:
   - CRUD operations
   - Project validation
   - Default project management
   - Persistence
   - Concurrency

2. **Integration tests** with:
   - Real filesystem operations
   - Registry persistence
   - Error conditions

## Dependencies

None - this is the foundation for multi-project support

## Blocks

- task-2: ProjectContext Encapsulation
- task-3: ProjectStatePool
- task-5: Project Management API Endpoints
