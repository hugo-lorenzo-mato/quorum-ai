import { create } from 'zustand';

const useTaskStore = create((set, get) => ({
  // State - keyed by workflowId
  tasksByWorkflow: {},
  selectedTaskId: null,
  taskProgress: {},

  // Actions
  setTasks: (workflowId, tasks) => {
    const { tasksByWorkflow } = get();
    set({
      tasksByWorkflow: {
        ...tasksByWorkflow,
        [workflowId]: tasks.reduce((acc, task) => {
          acc[task.id] = task;
          return acc;
        }, {}),
      },
    });
  },

  // Load persisted tasks from API response (on page reload)
  loadPersistedTasks: (workflowId, tasks) => {
    if (!tasks || tasks.length === 0) return;

    const { tasksByWorkflow } = get();
    const tasksMap = {};

    for (const task of tasks) {
      tasksMap[task.id] = {
        id: task.id,
        name: task.name,
        phase: task.phase,
        status: task.status,
        cli: task.cli,
        model: task.model,
        tokens_in: task.tokens_in,
        tokens_out: task.tokens_out,
        error: task.error,
        started_at: task.started_at,
        completed_at: task.completed_at,
      };
    }

    set({
      tasksByWorkflow: {
        ...tasksByWorkflow,
        [workflowId]: tasksMap,
      },
    });
  },

  selectTask: (taskId) => {
    set({ selectedTaskId: taskId });
  },

  getTasksForWorkflow: (workflowId) => {
    const { tasksByWorkflow } = get();
    const tasks = tasksByWorkflow[workflowId] || {};
    return Object.values(tasks);
  },

  getTask: (workflowId, taskId) => {
    const { tasksByWorkflow } = get();
    return tasksByWorkflow[workflowId]?.[taskId];
  },

  // SSE event handlers
  handleTaskCreated: (data) => {
    const { tasksByWorkflow } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = {
      id: data.task_id,
      name: data.name,
      phase: data.phase,
      agent: data.agent,
      model: data.model,
      status: 'pending',
      created_at: data.timestamp,
    };
    set({
      tasksByWorkflow: {
        ...tasksByWorkflow,
        [data.workflow_id]: { ...workflowTasks, [task.id]: task },
      },
    });
  },

  handleTaskStarted: (data) => {
    const { tasksByWorkflow } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = workflowTasks[data.task_id];
    if (task) {
      set({
        tasksByWorkflow: {
          ...tasksByWorkflow,
          [data.workflow_id]: {
            ...workflowTasks,
            [data.task_id]: {
              ...task,
              status: 'running',
              worktree_path: data.worktree_path,
              started_at: data.timestamp,
            },
          },
        },
      });
    }
  },

  handleTaskProgress: (data) => {
    const { taskProgress } = get();
    set({
      taskProgress: {
        ...taskProgress,
        [data.task_id]: {
          progress: data.progress,
          tokens_in: data.tokens_in,
          tokens_out: data.tokens_out,
          message: data.message,
          timestamp: data.timestamp,
        },
      },
    });
  },

  handleTaskCompleted: (data) => {
    const { tasksByWorkflow, taskProgress } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = workflowTasks[data.task_id];
    if (task) {
      set({
        tasksByWorkflow: {
          ...tasksByWorkflow,
          [data.workflow_id]: {
            ...workflowTasks,
            [data.task_id]: {
              ...task,
              status: 'completed',
              duration: data.duration,
              tokens_in: data.tokens_in,
              tokens_out: data.tokens_out,
              completed_at: data.timestamp,
            },
          },
        },
        taskProgress: {
          ...taskProgress,
          [data.task_id]: undefined,
        },
      });
    }
  },

  handleTaskFailed: (data) => {
    const { tasksByWorkflow, taskProgress } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = workflowTasks[data.task_id];
    if (task) {
      set({
        tasksByWorkflow: {
          ...tasksByWorkflow,
          [data.workflow_id]: {
            ...workflowTasks,
            [data.task_id]: {
              ...task,
              status: 'failed',
              error: data.error,
              retryable: data.retryable,
              failed_at: data.timestamp,
            },
          },
        },
        taskProgress: {
          ...taskProgress,
          [data.task_id]: undefined,
        },
      });
    }
  },

  handleTaskSkipped: (data) => {
    const { tasksByWorkflow } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = workflowTasks[data.task_id];
    if (task) {
      set({
        tasksByWorkflow: {
          ...tasksByWorkflow,
          [data.workflow_id]: {
            ...workflowTasks,
            [data.task_id]: {
              ...task,
              status: 'skipped',
              skip_reason: data.reason,
              skipped_at: data.timestamp,
            },
          },
        },
      });
    }
  },

  handleTaskRetry: (data) => {
    const { tasksByWorkflow } = get();
    const workflowTasks = tasksByWorkflow[data.workflow_id] || {};
    const task = workflowTasks[data.task_id];
    if (task) {
      set({
        tasksByWorkflow: {
          ...tasksByWorkflow,
          [data.workflow_id]: {
            ...workflowTasks,
            [data.task_id]: {
              ...task,
              status: 'retrying',
              attempt_num: data.attempt_num,
              max_attempts: data.max_attempts,
              last_error: data.error,
            },
          },
        },
      });
    }
  },

  clearTasks: (workflowId) => {
    if (workflowId) {
      const { tasksByWorkflow } = get();
      const { [workflowId]: _removed, ...rest } = tasksByWorkflow;
      set({ tasksByWorkflow: rest });
    } else {
      set({ tasksByWorkflow: {}, taskProgress: {} });
    }
  },
}));

export default useTaskStore;
