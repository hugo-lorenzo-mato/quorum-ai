import { create } from 'zustand';
import { kanbanApi } from '../lib/api';

// Column definitions with display names
export const KANBAN_COLUMNS = [
  { id: 'refinement', name: 'Refinement', description: 'Workflows being refined' },
  { id: 'todo', name: 'To Do', description: 'Ready for execution' },
  { id: 'in_progress', name: 'In Progress', description: 'Currently executing' },
  { id: 'to_verify', name: 'To Verify', description: 'Completed, pending review' },
  { id: 'done', name: 'Done', description: 'Verified and complete' },
];

const useKanbanStore = create((set, get) => ({
  // State
  columns: {
    refinement: [],
    todo: [],
    in_progress: [],
    to_verify: [],
    done: [],
  },
  engine: {
    enabled: false,
    currentWorkflowId: null,
    consecutiveFailures: 0,
    circuitBreakerOpen: false,
    lastFailureAt: null,
  },
  loading: false,
  error: null,
  draggedWorkflow: null,

  // Actions
  fetchBoard: async () => {
    set({ loading: true, error: null });
    try {
      const board = await kanbanApi.getBoard();
      set({
        columns: board.columns,
        engine: board.engine,
        loading: false,
      });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  fetchEngineState: async () => {
    try {
      const engine = await kanbanApi.getEngineState();
      set({ engine });
    } catch (error) {
      set({ error: error.message });
    }
  },

  moveWorkflow: async (workflowId, toColumn, position = 0) => {
    const { columns } = get();

    // Find current column
    let fromColumn = null;
    let workflow = null;
    for (const [col, workflows] of Object.entries(columns)) {
      const found = workflows.find(w => w.id === workflowId);
      if (found) {
        fromColumn = col;
        workflow = found;
        break;
      }
    }

    if (!workflow || !fromColumn) {
      set({ error: 'Workflow not found' });
      return false;
    }

    // Optimistic update
    const newColumns = { ...columns };
    newColumns[fromColumn] = newColumns[fromColumn].filter(w => w.id !== workflowId);
    const updatedWorkflow = { ...workflow, kanban_column: toColumn, kanban_position: position };
    newColumns[toColumn] = [...newColumns[toColumn]];
    newColumns[toColumn].splice(position, 0, updatedWorkflow);
    set({ columns: newColumns });

    try {
      await kanbanApi.moveWorkflow(workflowId, toColumn, position);
      return true;
    } catch (error) {
      // Revert on error
      set({ columns, error: error.message });
      return false;
    }
  },

  enableEngine: async () => {
    try {
      await kanbanApi.enableEngine();
      set(state => ({
        engine: { ...state.engine, enabled: true },
      }));
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  disableEngine: async () => {
    try {
      await kanbanApi.disableEngine();
      set(state => ({
        engine: { ...state.engine, enabled: false },
      }));
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  resetCircuitBreaker: async () => {
    try {
      await kanbanApi.resetCircuitBreaker();
      set(state => ({
        engine: {
          ...state.engine,
          circuitBreakerOpen: false,
          consecutiveFailures: 0,
        },
      }));
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  // SSE event handlers
  handleWorkflowMoved: (data) => {
    const { columns } = get();
    const newColumns = { ...columns };

    // Remove from old column
    if (data.from_column && newColumns[data.from_column]) {
      newColumns[data.from_column] = newColumns[data.from_column].filter(
        w => w.id !== data.workflow_id
      );
    }

    // Find workflow in any column if from_column not specified
    let workflow = null;
    for (const [col, workflows] of Object.entries(newColumns)) {
      const found = workflows.find(w => w.id === data.workflow_id);
      if (found) {
        workflow = found;
        newColumns[col] = workflows.filter(w => w.id !== data.workflow_id);
        break;
      }
    }

    // Add to new column if we found the workflow
    if (workflow && data.to_column && newColumns[data.to_column]) {
      const updatedWorkflow = {
        ...workflow,
        kanban_column: data.to_column,
        kanban_position: data.new_position,
      };
      newColumns[data.to_column] = [...newColumns[data.to_column]];
      const pos = Math.min(data.new_position, newColumns[data.to_column].length);
      newColumns[data.to_column].splice(pos, 0, updatedWorkflow);
    }

    set({ columns: newColumns });
  },

  handleExecutionStarted: (data) => {
    set(state => ({
      engine: {
        ...state.engine,
        currentWorkflowId: data.workflow_id,
      },
    }));
  },

  handleExecutionCompleted: (data) => {
    const { columns } = get();
    const newColumns = { ...columns };

    // Find workflow and update PR info
    for (const [col, workflows] of Object.entries(newColumns)) {
      const idx = workflows.findIndex(w => w.id === data.workflow_id);
      if (idx !== -1) {
        newColumns[col] = [...workflows];
        newColumns[col][idx] = {
          ...newColumns[col][idx],
          pr_url: data.pr_url,
          pr_number: data.pr_number,
        };
        break;
      }
    }

    set(state => ({
      columns: newColumns,
      engine: { ...state.engine, currentWorkflowId: null },
    }));
  },

  handleExecutionFailed: (data) => {
    set(state => ({
      engine: {
        ...state.engine,
        currentWorkflowId: null,
        consecutiveFailures: data.consecutive_failures,
      },
    }));
  },

  handleEngineStateChanged: (data) => {
    set(state => ({
      engine: {
        ...state.engine,
        enabled: data.enabled,
        currentWorkflowId: data.current_workflow_id,
        circuitBreakerOpen: data.circuit_breaker_open,
      },
    }));
  },

  handleCircuitBreakerOpened: (data) => {
    set(state => ({
      engine: {
        ...state.engine,
        circuitBreakerOpen: true,
        consecutiveFailures: data.consecutive_failures,
        lastFailureAt: data.last_failure_at,
        enabled: false,
      },
    }));
  },

  // UI state
  setDraggedWorkflow: (workflow) => set({ draggedWorkflow: workflow }),
  clearDraggedWorkflow: () => set({ draggedWorkflow: null }),
  clearError: () => set({ error: null }),
}));

export default useKanbanStore;
