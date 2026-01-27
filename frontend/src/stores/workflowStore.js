import { create } from 'zustand';
import { workflowApi } from '../lib/api';
import useAgentStore from './agentStore';

const useWorkflowStore = create((set, get) => ({
  // State
  workflows: [],
  activeWorkflow: null,
  selectedWorkflowId: null,
  tasks: {},
  loading: false,
  error: null,

  // Actions
  fetchWorkflows: async () => {
    set({ loading: true, error: null });
    try {
      const workflows = await workflowApi.list();
      set({ workflows, loading: false });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  fetchActiveWorkflow: async () => {
    try {
      const activeWorkflow = await workflowApi.getActive();
      set({ activeWorkflow });

      // Load persisted agent events for page reload recovery
      if (activeWorkflow?.agent_events && activeWorkflow.agent_events.length > 0) {
        useAgentStore.getState().loadPersistedEvents(activeWorkflow.id, activeWorkflow.agent_events);
      }
    } catch (error) {
      // No active workflow is not an error
      if (!error.message.includes('not found')) {
        set({ error: error.message });
      }
    }
  },

  fetchWorkflow: async (id) => {
    set({ loading: true, error: null });
    try {
      const workflow = await workflowApi.get(id);
      const { workflows } = get();
      const updated = workflows.map(w => w.id === id ? workflow : w);
      if (!workflows.find(w => w.id === id)) {
        updated.push(workflow);
      }
      set({ workflows: updated, loading: false });

      // Load persisted agent events for page reload recovery
      if (workflow.agent_events && workflow.agent_events.length > 0) {
        useAgentStore.getState().loadPersistedEvents(id, workflow.agent_events);
      }

      return workflow;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  createWorkflow: async (prompt, config = {}) => {
    set({ loading: true, error: null });
    try {
      const workflow = await workflowApi.create(prompt, config);
      const { workflows } = get();
      set({
        workflows: [...workflows, workflow],
        activeWorkflow: workflow,
        selectedWorkflowId: workflow.id,
        loading: false,
      });
      return workflow;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  activateWorkflow: async (id) => {
    try {
      const workflow = await workflowApi.activate(id);
      set({ activeWorkflow: workflow });
      return workflow;
    } catch (error) {
      set({ error: error.message });
      return null;
    }
  },

  updateWorkflow: async (id, data) => {
    try {
      const workflow = await workflowApi.update(id, data);
      const { workflows, activeWorkflow } = get();
      const updated = workflows.map(w => w.id === id ? { ...w, ...workflow } : w);
      set({ workflows: updated });
      if (activeWorkflow?.id === id) {
        set({ activeWorkflow: { ...activeWorkflow, ...workflow } });
      }
      return workflow;
    } catch (error) {
      set({ error: error.message });
      throw error;
    }
  },

  // Workflow control actions
  startWorkflow: async (id) => {
    set({ loading: true, error: null });
    try {
      const result = await workflowApi.run(id);
      // Update local state immediately with the response from backend
      // Backend now returns status: "running" for run/resume operations
      const { workflows, activeWorkflow } = get();
      const updated = workflows.map(w =>
        w.id === id ? { ...w, status: result.status || 'running', currentPhase: result.current_phase || w.currentPhase } : w
      );
      set({
        workflows: updated,
        activeWorkflow: activeWorkflow?.id === id
          ? { ...activeWorkflow, status: result.status || 'running', currentPhase: result.current_phase || activeWorkflow.currentPhase }
          : activeWorkflow,
        loading: false,
      });
      return result;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  pauseWorkflow: async (id) => {
    set({ loading: true, error: null });
    try {
      const result = await workflowApi.pause(id);
      // Update local state immediately for responsive UI
      const { workflows, activeWorkflow } = get();
      const updated = workflows.map(w =>
        w.id === id ? { ...w, status: 'paused' } : w
      );
      set({
        workflows: updated,
        loading: false,
      });
      if (activeWorkflow?.id === id) {
        set({ activeWorkflow: { ...activeWorkflow, status: 'paused' } });
      }
      return result;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  resumeWorkflow: async (id) => {
    set({ loading: true, error: null });
    try {
      const result = await workflowApi.resume(id);
      // Update local state immediately with the response from backend
      const { workflows, activeWorkflow } = get();
      const updated = workflows.map(w =>
        w.id === id ? { ...w, status: result.status || 'running', currentPhase: result.current_phase || w.currentPhase } : w
      );
      set({
        workflows: updated,
        activeWorkflow: activeWorkflow?.id === id
          ? { ...activeWorkflow, status: result.status || 'running', currentPhase: result.current_phase || activeWorkflow.currentPhase }
          : activeWorkflow,
        loading: false,
      });
      return result;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  stopWorkflow: async (id) => {
    set({ loading: true, error: null });
    try {
      const result = await workflowApi.cancel(id);
      // The backend cancels the workflow asynchronously.
      // Real-time status updates will come via SSE events.
      set({ loading: false });
      return result;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  selectWorkflow: (id) => {
    set({ selectedWorkflowId: id });
  },

  fetchTasks: async (workflowId) => {
    try {
      const taskList = await workflowApi.getTasks(workflowId);
      const { tasks } = get();
      set({ tasks: { ...tasks, [workflowId]: taskList } });
      return taskList;
    } catch (error) {
      set({ error: error.message });
      return [];
    }
  },

  // SSE event handlers
  handleWorkflowStarted: (data) => {
    const { workflows } = get();
    const workflow = {
      id: data.workflow_id,
      status: 'running',
      prompt: data.prompt,
      created_at: data.timestamp,
      updated_at: data.timestamp,
    };
    set({
      workflows: [...workflows.filter(w => w.id !== workflow.id), workflow],
      activeWorkflow: workflow,
    });
  },

  handleWorkflowStateUpdated: (data) => {
    const { workflows, activeWorkflow } = get();
    const updated = workflows.map(w => {
      if (w.id === data.workflow_id) {
        return {
          ...w,
          current_phase: data.phase,
          task_count: data.total_tasks,
          updated_at: data.timestamp,
        };
      }
      return w;
    });
    set({ workflows: updated });
    if (activeWorkflow?.id === data.workflow_id) {
      set({
        activeWorkflow: {
          ...activeWorkflow,
          current_phase: data.phase,
          task_count: data.total_tasks,
        },
      });
    }
  },

  handleWorkflowCompleted: (data) => {
    const { workflows, activeWorkflow } = get();
    const updated = workflows.map(w => {
      if (w.id === data.workflow_id) {
        return { ...w, status: 'completed', updated_at: data.timestamp };
      }
      return w;
    });
    set({ workflows: updated });
    if (activeWorkflow?.id === data.workflow_id) {
      set({ activeWorkflow: { ...activeWorkflow, status: 'completed' } });
    }
  },

  handleWorkflowFailed: (data) => {
    const { workflows, activeWorkflow } = get();
    const updated = workflows.map(w => {
      if (w.id === data.workflow_id) {
        return { ...w, status: 'failed', error: data.error, updated_at: data.timestamp };
      }
      return w;
    });
    set({ workflows: updated });
    if (activeWorkflow?.id === data.workflow_id) {
      set({ activeWorkflow: { ...activeWorkflow, status: 'failed', error: data.error } });
    }
  },

  handleWorkflowPaused: (data) => {
    const { workflows, activeWorkflow } = get();
    const updated = workflows.map(w => {
      if (w.id === data.workflow_id) {
        return { ...w, status: 'paused', updated_at: data.timestamp };
      }
      return w;
    });
    set({ workflows: updated });
    if (activeWorkflow?.id === data.workflow_id) {
      set({ activeWorkflow: { ...activeWorkflow, status: 'paused' } });
    }
  },

  handleWorkflowResumed: (data) => {
    const { workflows, activeWorkflow } = get();
    const updated = workflows.map(w => {
      if (w.id === data.workflow_id) {
        return { ...w, status: 'running', updated_at: data.timestamp };
      }
      return w;
    });
    set({ workflows: updated });
    if (activeWorkflow?.id === data.workflow_id) {
      set({ activeWorkflow: { ...activeWorkflow, status: 'running' } });
    }
  },

  clearError: () => set({ error: null }),

  // Bulk update for polling fallback
  setWorkflows: (workflows) => set({ workflows }),
}));

export default useWorkflowStore;
