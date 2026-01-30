import { create } from 'zustand';
import { kanbanApi, workflowApi } from '../lib/api';

// Column order for iteration
export const KANBAN_COLUMNS = [
  'refinement',
  'todo',
  'in_progress',
  'to_verify',
  'done',
];

// Column display names
export const COLUMN_LABELS = {
  refinement: 'Refinement',
  todo: 'To Do',
  in_progress: 'In Progress',
  to_verify: 'To Verify',
  done: 'Done',
};

const useKanbanStore = create((set, get) => ({
  // Board state - workflows organized by column
  board: {
    refinement: [],
    todo: [],
    in_progress: [],
    to_verify: [],
    done: [],
  },

  // Engine state
  engineState: {
    enabled: false,
    currentWorkflowId: null,
    consecutiveFailures: 0,
    circuitBreakerOpen: false,
    lastFailureAt: null,
  },

  // UI state
  loading: false,
  error: null,
  moving: null,

  // ========== Fetch Operations ==========

  fetchBoard: async () => {
    set({ loading: true, error: null });
    try {
      const data = await kanbanApi.getBoard();
      set({
        board: data.board,
        engineState: data.engine_state,
        loading: false,
      });
    } catch (error) {
      set({
        error: error.message,
        loading: false,
      });
    }
  },

  fetchEngineState: async () => {
    try {
      const data = await kanbanApi.getEngineState();
      set({ engineState: data });
    } catch (error) {
      console.error('Failed to fetch engine state:', error);
    }
  },

  // ========== Workflow Operations ==========

  moveWorkflow: async (workflowId, targetColumn, targetPosition) => {
    const { board } = get();

    // Find current location
    let sourceColumn = null;
    let sourcePosition = null;
    let workflow = null;

    for (const column of KANBAN_COLUMNS) {
      const idx = board[column].findIndex((w) => w.workflow_id === workflowId);
      if (idx !== -1) {
        sourceColumn = column;
        sourcePosition = idx;
        workflow = board[column][idx];
        break;
      }
    }

    if (!workflow) {
      set({ error: 'Workflow not found' });
      return false;
    }

    // Optimistic update
    set({ moving: workflowId });

    const newBoard = { ...board };

    // Remove from source
    newBoard[sourceColumn] = [
      ...newBoard[sourceColumn].slice(0, sourcePosition),
      ...newBoard[sourceColumn].slice(sourcePosition + 1),
    ];

    // Update positions in source column
    newBoard[sourceColumn] = newBoard[sourceColumn].map((w, idx) => ({
      ...w,
      kanban_position: idx,
    }));

    // Insert into target
    const updatedWorkflow = {
      ...workflow,
      kanban_column: targetColumn,
      kanban_position: targetPosition,
    };

    newBoard[targetColumn] = [
      ...newBoard[targetColumn].slice(0, targetPosition),
      updatedWorkflow,
      ...newBoard[targetColumn].slice(targetPosition),
    ];

    // Update positions in target column
    newBoard[targetColumn] = newBoard[targetColumn].map((w, idx) => ({
      ...w,
      kanban_position: idx,
    }));

    set({ board: newBoard });

    // API call
    try {
      await kanbanApi.moveWorkflow(workflowId, targetColumn, targetPosition);
      set({ moving: null });
      return true;
    } catch (error) {
      // Revert on error
      set({
        board: board,
        moving: null,
        error: error.message,
      });
      return false;
    }
  },

  createWorkflow: async (title, prompt) => {
    try {
      const data = await workflowApi.create(prompt, { title });
      const newWorkflow = {
        workflow_id: data.id,
        title: data.title,
        prompt: data.prompt,
        kanban_column: 'refinement',
        kanban_position: 0,
        workflow_status: data.status,
        current_phase: data.current_phase,
        pr_url: null,
        pr_number: null,
        execution_count: 0,
        last_error: null,
        created_at: data.created_at,
        updated_at: data.updated_at,
        kanban_started_at: null,
        kanban_completed_at: null,
      };

      // Add to refinement column
      set((state) => ({
        board: {
          ...state.board,
          refinement: [newWorkflow, ...state.board.refinement],
        },
      }));

      return newWorkflow;
    } catch (error) {
      set({ error: error.message });
      return null;
    }
  },

  // ========== Engine Operations ==========

  enableEngine: async () => {
    try {
      const data = await kanbanApi.startEngine();
      set({ engineState: data });
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  disableEngine: async () => {
    try {
      const data = await kanbanApi.stopEngine();
      set({ engineState: data });
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  resetCircuitBreaker: async () => {
    try {
      const data = await kanbanApi.resetCircuitBreaker();
      set({ engineState: data });
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  // ========== SSE Event Handlers ==========

  handleWorkflowMoved: (data) => {
    const { board, moving } = get();

    // If this is our own optimistic update, ignore
    if (moving === data.workflow_id && data.user_initiated) {
      return;
    }

    // Find and move the workflow
    let workflow = null;
    for (const column of KANBAN_COLUMNS) {
      const idx = board[column].findIndex((w) => w.workflow_id === data.workflow_id);
      if (idx !== -1) {
        workflow = board[column][idx];
        break;
      }
    }

    if (!workflow) {
      // Workflow not in board, fetch fresh data
      get().fetchBoard();
      return;
    }

    // Apply the move
    get().moveWorkflowLocally(data.workflow_id, data.to_column, data.new_position);
  },

  handleExecutionStarted: (data) => {
    set((state) => ({
      engineState: {
        ...state.engineState,
        currentWorkflowId: data.workflow_id,
      },
    }));

    // Update started_at
    get().updateWorkflowInBoard(data.workflow_id, {
      kanban_started_at: data.timestamp,
    });
  },

  handleExecutionCompleted: (data) => {
    set((state) => ({
      engineState: {
        ...state.engineState,
        currentWorkflowId: null,
        consecutiveFailures: 0,
      },
    }));

    get().updateWorkflowInBoard(data.workflow_id, {
      pr_url: data.pr_url,
      pr_number: data.pr_number,
      kanban_completed_at: data.timestamp,
    });
  },

  handleExecutionFailed: (data) => {
    set((state) => ({
      engineState: {
        ...state.engineState,
        currentWorkflowId: null,
        consecutiveFailures: data.consecutive_failures,
      },
    }));

    get().updateWorkflowInBoard(data.workflow_id, {
      last_error: data.error,
      execution_count: (get().findWorkflow(data.workflow_id)?.execution_count || 0) + 1,
    });
  },

  handleEngineStateChanged: (data) => {
    set({
      engineState: {
        enabled: data.enabled,
        currentWorkflowId: data.current_workflow_id,
        circuitBreakerOpen: data.circuit_breaker_open,
        consecutiveFailures: get().engineState.consecutiveFailures,
        lastFailureAt: get().engineState.lastFailureAt,
      },
    });
  },

  handleCircuitBreakerOpened: (data) => {
    set({
      engineState: {
        ...get().engineState,
        enabled: false,
        circuitBreakerOpen: true,
        consecutiveFailures: data.consecutive_failures,
        lastFailureAt: data.last_failure_at,
      },
    });
  },

  // ========== Helper Actions ==========

  moveWorkflowLocally: (workflowId, targetColumn, targetPosition) => {
    const { board } = get();

    let sourceColumn = null;
    let workflow = null;

    for (const column of KANBAN_COLUMNS) {
      const idx = board[column].findIndex((w) => w.workflow_id === workflowId);
      if (idx !== -1) {
        sourceColumn = column;
        workflow = board[column][idx];
        break;
      }
    }

    if (!workflow) return;

    const newBoard = { ...board };

    // Remove from source
    newBoard[sourceColumn] = newBoard[sourceColumn].filter(
      (w) => w.workflow_id !== workflowId
    );

    // Insert into target
    const updatedWorkflow = {
      ...workflow,
      kanban_column: targetColumn,
      kanban_position: targetPosition,
    };

    newBoard[targetColumn] = [
      ...newBoard[targetColumn].slice(0, targetPosition),
      updatedWorkflow,
      ...newBoard[targetColumn].slice(targetPosition),
    ].map((w, idx) => ({ ...w, kanban_position: idx }));

    set({ board: newBoard });
  },

  updateWorkflowInBoard: (workflowId, updates) => {
    set((state) => {
      const newBoard = { ...state.board };

      for (const column of KANBAN_COLUMNS) {
        newBoard[column] = newBoard[column].map((w) =>
          w.workflow_id === workflowId ? { ...w, ...updates } : w
        );
      }

      return { board: newBoard };
    });
  },

  findWorkflow: (workflowId) => {
    const { board } = get();

    for (const column of KANBAN_COLUMNS) {
      const workflow = board[column].find((w) => w.workflow_id === workflowId);
      if (workflow) return workflow;
    }

    return null;
  },

  setError: (error) => {
    set({ error });
  },

  clearError: () => {
    set({ error: null });
  },
}));

export default useKanbanStore;
