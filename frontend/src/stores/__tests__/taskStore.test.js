import { describe, it, expect, beforeEach } from 'vitest';
import useTaskStore from '../taskStore';

function resetStore() {
  useTaskStore.setState({
    tasksByWorkflow: {},
    selectedTaskId: null,
    taskProgress: {},
  });
}

describe('taskStore', () => {
  beforeEach(() => {
    resetStore();
  });

  it('setTasks indexes tasks by id under workflow key', () => {
    useTaskStore.getState().setTasks('wf-1', [{ id: 't1', name: 'a' }, { id: 't2', name: 'b' }]);
    expect(Object.keys(useTaskStore.getState().tasksByWorkflow['wf-1']).sort()).toEqual(['t1', 't2']);
  });

  it('loadPersistedTasks ignores empty list and maps known fields', () => {
    useTaskStore.getState().loadPersistedTasks('wf-1', []);
    expect(useTaskStore.getState().tasksByWorkflow['wf-1']).toBeUndefined();

    useTaskStore.getState().loadPersistedTasks('wf-1', [{ id: 't1', name: 'n', phase: 'plan', status: 'pending' }]);
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.phase).toBe('plan');
  });

  it('SSE handlers update task state and progress', () => {
    useTaskStore.getState().handleTaskCreated({
      workflow_id: 'wf-1',
      task_id: 't1',
      name: 'Task',
      phase: 'plan',
      agent: 'claude',
      model: 'm',
      timestamp: '2026-02-10T00:00:00Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('pending');

    useTaskStore.getState().handleTaskStarted({
      workflow_id: 'wf-1',
      task_id: 't1',
      worktree_path: '/tmp/w',
      timestamp: '2026-02-10T00:00:01Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('running');

    useTaskStore.getState().handleTaskProgress({
      task_id: 't1',
      progress: 50,
      tokens_in: 1,
      tokens_out: 2,
      message: 'half',
      timestamp: '2026-02-10T00:00:02Z',
    });
    expect(useTaskStore.getState().taskProgress.t1.progress).toBe(50);

    useTaskStore.getState().handleTaskCompleted({
      workflow_id: 'wf-1',
      task_id: 't1',
      duration: 3,
      tokens_in: 1,
      tokens_out: 2,
      timestamp: '2026-02-10T00:00:03Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('completed');
    expect(useTaskStore.getState().taskProgress.t1).toBeUndefined();
  });

  it('handleTaskFailed, handleTaskSkipped, handleTaskRetry update status and clear progress', () => {
    useTaskStore.getState().handleTaskCreated({
      workflow_id: 'wf-1',
      task_id: 't1',
      name: 'Task',
      phase: 'execute',
      agent: 'claude',
      model: 'm',
      timestamp: '2026-02-10T00:00:00Z',
    });

    useTaskStore.getState().handleTaskProgress({
      task_id: 't1',
      progress: 1,
      tokens_in: 0,
      tokens_out: 0,
      message: 'x',
      timestamp: '2026-02-10T00:00:01Z',
    });

    useTaskStore.getState().handleTaskFailed({
      workflow_id: 'wf-1',
      task_id: 't1',
      error: 'nope',
      retryable: true,
      timestamp: '2026-02-10T00:00:02Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('failed');
    expect(useTaskStore.getState().taskProgress.t1).toBeUndefined();

    useTaskStore.getState().handleTaskSkipped({
      workflow_id: 'wf-1',
      task_id: 't1',
      reason: 'skip',
      timestamp: '2026-02-10T00:00:03Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('skipped');

    useTaskStore.getState().handleTaskRetry({
      workflow_id: 'wf-1',
      task_id: 't1',
      attempt_num: 2,
      max_attempts: 3,
      error: 'retry',
      timestamp: '2026-02-10T00:00:04Z',
    });
    expect(useTaskStore.getState().tasksByWorkflow['wf-1'].t1.status).toBe('retrying');
  });

  it('clearTasks removes one workflow or resets all', () => {
    useTaskStore.setState({ tasksByWorkflow: { a: { t: {} }, b: { t: {} } }, taskProgress: { t: {} } });
    useTaskStore.getState().clearTasks('a');
    expect(useTaskStore.getState().tasksByWorkflow.a).toBeUndefined();
    expect(useTaskStore.getState().tasksByWorkflow.b).toBeTruthy();

    useTaskStore.getState().clearTasks();
    expect(useTaskStore.getState().tasksByWorkflow).toEqual({});
    expect(useTaskStore.getState().taskProgress).toEqual({});
  });
});

