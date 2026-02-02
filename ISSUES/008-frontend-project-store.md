# Frontend Project Store Implementation

## Summary

Implement a Zustand store for managing registered projects, current project selection, and project operations (add, remove, update) with localStorage persistence and event notifications for other components.

## Context

The Quorum AI WebUI needs centralized project management in the frontend. The store will:

1. Maintain list of registered projects from the API
2. Track currently selected project
3. Persist selection to localStorage for session continuity
4. Dispatch events when project changes so other stores can react
5. Provide actions for adding, removing, and updating projects

## Implementation Details

### Files to Create

- `frontend/src/stores/projectStore.js` - Project state management store
- `frontend/src/lib/projectApi.js` - Project API client
- `frontend/src/hooks/useProjectChange.js` - Hook for project change reactions
- `frontend/src/stores/__tests__/projectStore.test.js` - Store tests

### Files to Modify

- `frontend/src/lib/api.js` - Add project API URL helpers

### Store State

```javascript
{
  // Project list
  projects: [],                    // All registered projects
  projectsLoading: false,          // Loading state
  projectsError: null,             // Load error message
  lastFetched: null,               // Timestamp of last fetch

  // Current selection
  currentProjectId: null,          // Selected project ID

  // Operations
  addingProject: false,            // Add operation state
  removingProject: null,           // ID of project being removed
  operationError: null,            // Operation error message
}
```

### Key Actions

- **loadProjects()**: Fetch projects from API, auto-select default or first
- **selectProject(id)**: Change current project, dispatch event
- **addProject(path, name, color)**: Register new project
- **removeProject(id)**: Remove project, switch current if needed
- **updateProject(id, updates)**: Update metadata
- **validateProject(id)**: Check project health
- **setDefaultProject(id)**: Set default project
- **refreshProjects()**: Reload from API

### Event System

Custom events dispatch when project changes:
```javascript
window.dispatchEvent(new CustomEvent('quorum:project-changed', {
  detail: { projectId, project }
}));
```

## Acceptance Criteria

- [ ] Project API client created with all endpoints
- [ ] Store manages complete project list
- [ ] Current selection persisted to localStorage
- [ ] selectProject dispatches change events
- [ ] addProject adds to local state immediately
- [ ] removeProject switches current if it was removed
- [ ] Loading and error states managed properly
- [ ] Project change hook enables other stores to react
- [ ] All unit tests pass with high coverage
- [ ] Console logging aids debugging
- [ ] No API calls for unneeded operations

## Testing Strategy

1. **Unit Tests**:
   - Load projects and auto-selection
   - Project selection and event dispatch
   - Add/remove/update operations
   - Error handling for all operations
   - localStorage persistence and restoration
   - Computed values (getCurrentProject, getDefaultProject, etc.)

2. **Integration Tests**:
   - Full workflow: load → select → add → remove
   - Event dispatch and listener behavior
   - Error recovery and retry

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- Clear error messages for user feedback
- Proper async error handling
- Console logs for debugging
