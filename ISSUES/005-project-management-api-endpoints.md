# Project Management API Endpoints

## Summary

Implement REST API endpoints for managing the project registry, enabling the WebUI to list, register, configure, and validate projects.

## Context

Multi-project support requires HTTP endpoints to:
1. Discover registered projects
2. Register new projects
3. Configure project metadata (name, color)
4. Validate project health
5. Set default project for legacy endpoint compatibility

## Implementation Details

### Files to Create

- `internal/api/projects.go` - Project API handlers
- `internal/api/projects_test.go` - Handler tests

### Files to Modify

- `internal/api/server.go` - Add registry field, register routes

### API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/projects` | List all registered projects |
| POST | `/api/v1/projects` | Register new project |
| GET | `/api/v1/projects/{projectId}` | Get project details |
| PUT | `/api/v1/projects/{projectId}` | Update project metadata |
| DELETE | `/api/v1/projects/{projectId}` | Remove project from registry |
| POST | `/api/v1/projects/{projectId}/default` | Set as default project |
| POST | `/api/v1/projects/{projectId}/validate` | Validate project health |

### Request/Response Models

**ListProjectsResponse**
```json
{
  "projects": [
    {
      "id": "proj-abc123",
      "path": "/home/user/projects/alpha",
      "name": "Project Alpha",
      "status": "healthy",
      "color": "#4A90D9",
      "last_accessed": "2025-01-15T10:30:00Z",
      "created_at": "2025-01-10T00:00:00Z",
      "is_default": true
    }
  ],
  "total": 1
}
```

**AddProjectRequest**
```json
{
  "path": "/absolute/path/to/project",
  "name": "Custom Project Name",
  "color": "#FF5733"
}
```

### Error Handling

| Scenario | Status | Message |
|----------|--------|---------|
| Invalid path | 400 | "path is required" |
| Not a Quorum project | 400 | "not a valid Quorum project..." |
| Project not found | 404 | "project not found" |
| Already registered | 409 | "project already registered" |
| Directory inaccessible | 503 | "project directory not accessible" |

## Acceptance Criteria

- [ ] GET /api/v1/projects returns all projects
- [ ] POST /api/v1/projects validates input and registers project
- [ ] GET /api/v1/projects/{id} returns project details
- [ ] PUT /api/v1/projects/{id} updates metadata
- [ ] DELETE /api/v1/projects/{id} removes from registry
- [ ] POST /api/v1/projects/{id}/default sets default
- [ ] POST /api/v1/projects/{id}/validate checks health
- [ ] Proper HTTP status codes for all scenarios
- [ ] JSON responses match specification
- [ ] Project pool is evicted when project removed
- [ ] Error responses are descriptive and helpful
- [ ] All handlers are well-tested
- [ ] Logging provides operational visibility

## Testing Strategy

1. **Unit Tests**:
   - List, get, create, update, delete operations
   - Error cases (not found, conflict, invalid input)
   - Metadata updates
   - Default project changes
   - Health validation

2. **Integration Tests**:
   - Full workflow: register → list → get → update → remove
   - Multiple projects management
   - Pool eviction on removal

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- All error cases handled properly
- Clear and helpful error messages
- Proper logging at INFO/ERROR levels
