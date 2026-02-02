# Project Context Middleware for API

## Summary

Implement middleware that injects project-scoped context into HTTP requests, enabling handlers to access project-specific resources via the ProjectStatePool.

## Context

With multi-project support, all API handlers must be aware of which project they're operating on. The middleware:

1. Extracts `projectID` from URL path parameters
2. Loads `ProjectContext` from `ProjectStatePool`
3. Stores context in request context for handler access
4. Handles errors (project not found, context creation failed)
5. Supports fallback to default project for legacy endpoints

## Implementation Details

### Files to Create

- `internal/api/middleware/project.go` - Project context middleware
- `internal/api/middleware/project_test.go` - Middleware tests

### Files to Modify

- `internal/api/server.go` - Apply middleware to routes

### Middleware Components

1. **ProjectContextMiddleware**
   - Extracts projectID from `{projectID}` URL parameter
   - Loads context from pool
   - Returns 400 if projectID missing
   - Returns 404 if project not found
   - Returns 503 if project inaccessible

2. **DefaultProjectMiddleware**
   - For legacy endpoints without projectID in URL
   - Looks up default project from registry
   - Returns 503 if no default project set
   - Adds deprecation headers to responses

3. **RequireProjectContext**
   - Validates project context exists
   - Returns 500 if missing
   - Returns 503 if context closed

4. **Helper Functions**
   - `GetProjectContext(ctx)`: Retrieve context from request
   - `GetProjectID(ctx)`: Retrieve project ID from request
   - `WithProjectContext(ctx, pc)`: Add context to request

### Route Structure

```go
// Project-scoped endpoints
r.Route("/projects/{projectID}", func(r chi.Router) {
    r.Use(ProjectContextMiddleware(pool))
    r.Use(RequireProjectContext)

    r.Route("/workflows", func(r chi.Router) {
        r.Get("/", handleListWorkflows)
        // ... project-scoped endpoints
    })
})

// Legacy endpoints (backward compatibility)
r.Route("/workflows", func(r chi.Router) {
    r.Use(DefaultProjectMiddleware(registry, pool))
    r.Use(RequireProjectContext)

    r.Get("/", handleListWorkflows)
    // ... legacy endpoints
})
```

## Acceptance Criteria

- [ ] ProjectContextMiddleware extracts projectID from URL
- [ ] ProjectContextMiddleware loads context from pool
- [ ] ProjectContextMiddleware returns proper error codes
- [ ] DefaultProjectMiddleware uses default project
- [ ] DefaultProjectMiddleware sets deprecation headers
- [ ] RequireProjectContext validates context presence
- [ ] Helper functions retrieve values correctly
- [ ] Handlers can access project context
- [ ] Legacy endpoints work with default project
- [ ] Project validation happens before passing to handler
- [ ] Proper error handling for closed contexts
- [ ] All unit tests pass

## Testing Strategy

1. **Unit Tests**:
   - ProjectContextMiddleware with valid/invalid projectID
   - DefaultProjectMiddleware with/without default project
   - RequireProjectContext with/without context
   - Helper functions for context access
   - Error cases and status codes

2. **Integration Tests**:
   - Project-scoped endpoint receives correct context
   - Legacy endpoint uses default project
   - Invalid project returns 404
   - No project set returns 503

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No context leakage between requests
- Proper error codes for all failure scenarios
- Clear error messages for debugging
