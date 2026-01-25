import { create } from 'zustand';
import { workflowApi } from '../lib/api';

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

  clearError: () => set({ error: null }),
}));

export default useWorkflowStore;
