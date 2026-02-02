# Frontend Store Restructuring for Multi-Project Support

## Summary

Restructure all Zustand stores (workflowStore, taskStore, chatStore, configStore, kanbanStore, engineStore) to organize state by project ID, using a factory pattern to create project-scoped stores.

## Context

Current stores are global singletons holding state for a single project. For multi-project support, each store needs to maintain separate state for each project. This is achieved through:

1. **Factory Pattern**: `createProjectStore(projectId)` factory function
2. **State Nesting**: `byProject: { [projectId]: { workflowState } }`
3. **Backward Compatibility**: Wrapper functions for legacy endpoints
4. **Auto-Cleanup**: Remove unused project state after eviction

## Implementation Details

### Files to Create

- `frontend/src/stores/createProjectStore.js` - Store factory pattern
- `frontend/src/stores/projectWorkflowStore.js` - Project-scoped workflow store
- `frontend/src/stores/projectTaskStore.js` - Project-scoped task store
- `frontend/src/stores/projectChatStore.js` - Project-scoped chat store
- `frontend/src/stores/projectConfigStore.js` - Project-scoped config store
- `frontend/src/stores/projectKanbanStore.js` - Project-scoped kanban store
- `frontend/src/stores/projectEngineStore.js` - Project-scoped engine store

### Files to Modify

- `frontend/src/stores/workflowStore.js` - Use factory for project scoping
- `frontend/src/stores/taskStore.js` - Use factory for project scoping
- `frontend/src/stores/chatStore.js` - Use factory for project scoping
- `frontend/src/stores/configStore.js` - Use factory for project scoping
- `frontend/src/stores/kanbanStore.js` - Use factory for project scoping
- `frontend/src/stores/engineStore.js` - Use factory for project scoping
- `frontend/src/App.jsx` - Initialize stores for current project

### Store Factory Pattern

```javascript
// createProjectStore.js
export function createProjectStore(projectId) {
  return create((set, get) => ({
    byProject: {
      [projectId]: {
        // Project-specific state
      }
    },

    // Project-scoped actions
    addWorkflow(workflow) {
      set(state => ({
        byProject: {
          ...state.byProject,
          [projectId]: {
            ...state.byProject[projectId],
            workflows: [...state.byProject[projectId].workflows, workflow]
          }
        }
      }));
    },

    // Cleanup for project
    cleanupProject(id) {
      set(state => {
        const newState = { ...state.byProject };
        delete newState[id];
        return { byProject: newState };
      });
    }
  }));
}
```

### Store Usage Pattern

```javascript
// In components
function MyComponent() {
  const projectId = useProjectStore(state => state.currentProjectId);

  // Get project-scoped data
  const workflows = useWorkflowStore(
    state => state.byProject[projectId]?.workflows || []
  );

  // Or use helper hook
  const workflows = useProjectWorkflows(projectId);
}
```

### Helper Hooks

For each store, create helper hooks:

```javascript
// useProjectWorkflows.js
export function useProjectWorkflows(projectId) {
  return useWorkflowStore(
    state => state.byProject[projectId]?.workflows || [],
    (a, b) => a?.length === b?.length
  );
}

export function useProjectWorkflowsLoading(projectId) {
  return useWorkflowStore(
    state => state.byProject[projectId]?.loading || false
  );
}
```

## Acceptance Criteria

- [ ] Factory pattern implemented in createProjectStore
- [ ] All 6 stores restructured with `byProject` nesting
- [ ] Project-scoped actions created for all stores
- [ ] cleanupProject method removes unused state
- [ ] Helper hooks created for easy component access
- [ ] Backward compatibility wrappers for legacy endpoints
- [ ] Components updated to use project-scoped data
- [ ] No data leakage between projects
- [ ] Project selection triggers data refresh
- [ ] Memory cleanup when project evicted
- [ ] All unit tests pass
- [ ] Integration tests verify project isolation

## Testing Strategy

1. **Unit Tests**:
   - Factory creates separate stores
   - State isolation between projects
   - Actions properly update project state
   - Cleanup removes project state
   - Helper hooks return correct data

2. **Integration Tests**:
   - Switch projects and verify state changes
   - Add data to project A, switch to B, verify isolation
   - Project eviction triggers cleanup
   - Backward compatibility wrappers work

3. **Manual Testing**:
   - Multiple projects open in tabs
   - Project selection refreshes UI correctly
   - No data from other projects visible

## Definition of Done

- All acceptance criteria met
- Unit test coverage >80%
- No state leakage between projects
- Memory properly cleaned up
- Clear error handling for missing project
