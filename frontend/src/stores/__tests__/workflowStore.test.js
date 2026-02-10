import { describe, it, expect, vi, beforeEach } from 'vitest';
import useWorkflowStore from '../workflowStore';

const loadPersistedEvents = vi.fn();
const loadPersistedTasks = vi.fn();

// Mock the API module
vi.mock('../../lib/api', () => ({
  workflowApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    getActive: vi.fn(),
    activate: vi.fn(),
    update: vi.fn(),
    run: vi.fn(),
    pause: vi.fn(),
    resume: vi.fn(),
    cancel: vi.fn(),
    forceStop: vi.fn(),
    analyze: vi.fn(),
    plan: vi.fn(),
    replan: vi.fn(),
    execute: vi.fn(),
    delete: vi.fn(),
    getTasks: vi.fn(),
  },
}));

// Mock the dependent stores
vi.mock('../agentStore', () => ({
  default: {
    getState: () => ({
      loadPersistedEvents,
    }),
  },
}));

vi.mock('../taskStore', () => ({
  default: {
    getState: () => ({
      loadPersistedTasks,
    }),
  },
}));

import { workflowApi } from '../../lib/api';

describe('workflowStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    useWorkflowStore.setState({
      workflows: [],
      activeWorkflow: null,
      selectedWorkflowId: null,
      tasks: {},
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  describe('createWorkflow', () => {
    it('creates workflow with single-agent blueprint', async () => {
      const mockWorkflow = {
        id: 'wf-123',
        prompt: 'Test prompt',
        title: 'Test',
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      };

      workflowApi.create.mockResolvedValue(mockWorkflow);

      const result = await useWorkflowStore.getState().createWorkflow('Test prompt', {
        title: 'Test',
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(workflowApi.create).toHaveBeenCalledWith('Test prompt', {
        title: 'Test',
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(result).toEqual(mockWorkflow);
      const state = useWorkflowStore.getState();
      expect(state.workflows).toHaveLength(1);
      expect(state.workflows[0]).toEqual(mockWorkflow);
      expect(state.workflows[0].blueprint.execution_mode).toBe('single_agent');
      expect(state.activeWorkflow).toEqual(mockWorkflow);
      expect(state.loading).toBe(false);
    });

    it('creates workflow with default multi-agent mode (no blueprint)', async () => {
      const mockWorkflow = {
        id: 'wf-456',
        prompt: 'Test prompt',
      };

      workflowApi.create.mockResolvedValue(mockWorkflow);

      const result = await useWorkflowStore.getState().createWorkflow('Test prompt');

      expect(workflowApi.create).toHaveBeenCalledWith('Test prompt', {});
      expect(result).toEqual(mockWorkflow);
    });

    it('creates workflow with title only (no blueprint)', async () => {
      const mockWorkflow = {
        id: 'wf-789',
        prompt: 'Test prompt',
        title: 'My Workflow',
      };

      workflowApi.create.mockResolvedValue(mockWorkflow);

      await useWorkflowStore.getState().createWorkflow('Test prompt', {
        title: 'My Workflow',
      });

      expect(workflowApi.create).toHaveBeenCalledWith('Test prompt', {
        title: 'My Workflow',
      });
    });

    it('handles API errors correctly', async () => {
      const errorMessage = "single_agent_name: required when execution_mode is 'single_agent'";
      workflowApi.create.mockRejectedValue(new Error(errorMessage));

      const result = await useWorkflowStore.getState().createWorkflow('Test prompt', {
        blueprint: { execution_mode: 'single_agent' },
      });

      expect(result).toBeNull();
      const state = useWorkflowStore.getState();
      expect(state.error).toBe(errorMessage);
      expect(state.loading).toBe(false);
      expect(state.workflows).toHaveLength(0);
    });

    it('sets loading state during API call', async () => {
      let resolvePromise;
      workflowApi.create.mockImplementation(() => new Promise((resolve) => {
        resolvePromise = resolve;
      }));

      const createPromise = useWorkflowStore.getState().createWorkflow('Test');

      expect(useWorkflowStore.getState().loading).toBe(true);
      expect(useWorkflowStore.getState().error).toBeNull();

      resolvePromise({ id: 'wf-123', prompt: 'Test' });
      await createPromise;

      expect(useWorkflowStore.getState().loading).toBe(false);
    });

    it('sets selectedWorkflowId after creation', async () => {
      const mockWorkflow = { id: 'wf-new', prompt: 'Test' };
      workflowApi.create.mockResolvedValue(mockWorkflow);

      await useWorkflowStore.getState().createWorkflow('Test');

      expect(useWorkflowStore.getState().selectedWorkflowId).toBe('wf-new');
    });
  });

  describe('fetchWorkflows', () => {
    it('loads workflows list and clears loading state', async () => {
      workflowApi.list.mockResolvedValue([{ id: 'wf-1' }, { id: 'wf-2' }]);

      const p = useWorkflowStore.getState().fetchWorkflows();
      expect(useWorkflowStore.getState().loading).toBe(true);
      await p;

      const state = useWorkflowStore.getState();
      expect(state.workflows).toHaveLength(2);
      expect(state.loading).toBe(false);
      expect(state.error).toBeNull();
    });

    it('stores error when API fails', async () => {
      workflowApi.list.mockRejectedValue(new Error('boom'));

      await useWorkflowStore.getState().fetchWorkflows();
      const state = useWorkflowStore.getState();
      expect(state.loading).toBe(false);
      expect(state.error).toBe('boom');
    });
  });

  describe('fetchActiveWorkflow', () => {
    it('hydrates persisted agent events and tasks when active workflow has them', async () => {
      workflowApi.getActive.mockResolvedValue({
        id: 'wf-active',
        execution_id: 7,
        agent_events: [{ id: 'evt-1' }],
        tasks: [{ id: 't-1' }],
      });

      await useWorkflowStore.getState().fetchActiveWorkflow();

      expect(useWorkflowStore.getState().activeWorkflow.id).toBe('wf-active');
      expect(loadPersistedEvents).toHaveBeenCalledWith('wf-active', [{ id: 'evt-1' }], 7);
      expect(loadPersistedTasks).toHaveBeenCalledWith('wf-active', [{ id: 't-1' }]);
    });

    it('does not set error when backend says there is no active workflow', async () => {
      workflowApi.getActive.mockRejectedValue(new Error('not found'));

      await useWorkflowStore.getState().fetchActiveWorkflow();
      expect(useWorkflowStore.getState().error).toBeNull();
    });

    it('sets error for other failures', async () => {
      workflowApi.getActive.mockRejectedValue(new Error('db down'));

      await useWorkflowStore.getState().fetchActiveWorkflow();
      expect(useWorkflowStore.getState().error).toBe('db down');
    });
  });

  describe('fetchWorkflow', () => {
    it('updates existing workflow entry and returns workflow', async () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', title: 'old' }] });
      workflowApi.get.mockResolvedValue({ id: 'wf-1', title: 'new', execution_id: 1, agent_events: [], tasks: [] });

      const wf = await useWorkflowStore.getState().fetchWorkflow('wf-1');
      expect(wf.title).toBe('new');
      expect(useWorkflowStore.getState().workflows.find(w => w.id === 'wf-1').title).toBe('new');
      expect(useWorkflowStore.getState().loading).toBe(false);
    });

    it('supports silent refresh without toggling loading state', async () => {
      useWorkflowStore.setState({ loading: false, workflows: [] });
      workflowApi.get.mockResolvedValue({ id: 'wf-2', agent_events: [], tasks: [] });

      await useWorkflowStore.getState().fetchWorkflow('wf-2', { silent: true });
      expect(useWorkflowStore.getState().loading).toBe(false);
      expect(useWorkflowStore.getState().workflows.find(w => w.id === 'wf-2')).toBeTruthy();
    });

    it('sets error and returns null on failure', async () => {
      workflowApi.get.mockRejectedValue(new Error('nope'));
      const wf = await useWorkflowStore.getState().fetchWorkflow('wf-x');
      expect(wf).toBeNull();
      expect(useWorkflowStore.getState().error).toBe('nope');
      expect(useWorkflowStore.getState().loading).toBe(false);
    });
  });

  describe('updateWorkflow', () => {
    it('updates workflows list and activeWorkflow when ids match', async () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', title: 'old' }], activeWorkflow: { id: 'wf-1', title: 'old' } });
      workflowApi.update.mockResolvedValue({ title: 'new' });

      const res = await useWorkflowStore.getState().updateWorkflow('wf-1', { title: 'new' });
      expect(res.title).toBe('new');
      expect(useWorkflowStore.getState().workflows[0].title).toBe('new');
      expect(useWorkflowStore.getState().activeWorkflow.title).toBe('new');
    });
  });

  describe('control actions', () => {
    it('startWorkflow sets status running and updates phase from backend', async () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', status: 'pending', currentPhase: 'plan' }], activeWorkflow: { id: 'wf-1', status: 'pending', currentPhase: 'plan' } });
      workflowApi.run.mockResolvedValue({ status: 'running', current_phase: 'analyze' });

      const res = await useWorkflowStore.getState().startWorkflow('wf-1');
      expect(res.status).toBe('running');
      expect(useWorkflowStore.getState().workflows[0].status).toBe('running');
      expect(useWorkflowStore.getState().activeWorkflow.status).toBe('running');
    });

    it('pauseWorkflow marks workflow as paused', async () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', status: 'running' }], activeWorkflow: { id: 'wf-1', status: 'running' } });
      workflowApi.pause.mockResolvedValue({ ok: true });

      await useWorkflowStore.getState().pauseWorkflow('wf-1');
      expect(useWorkflowStore.getState().workflows[0].status).toBe('paused');
      expect(useWorkflowStore.getState().activeWorkflow.status).toBe('paused');
    });

    it('resumeWorkflow marks workflow as running', async () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', status: 'paused', currentPhase: 'plan' }], activeWorkflow: { id: 'wf-1', status: 'paused', currentPhase: 'plan' } });
      workflowApi.resume.mockResolvedValue({ status: 'running', current_phase: 'execute' });

      await useWorkflowStore.getState().resumeWorkflow('wf-1');
      expect(useWorkflowStore.getState().workflows[0].status).toBe('running');
      expect(useWorkflowStore.getState().activeWorkflow.status).toBe('running');
    });

    it('stopWorkflow transitions to cancelling with stable timestamp', async () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date('2026-02-10T00:00:00.000Z'));
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', status: 'running' }], activeWorkflow: { id: 'wf-1', status: 'running' } });
      workflowApi.cancel.mockResolvedValue({ ok: true });

      await useWorkflowStore.getState().stopWorkflow('wf-1');
      expect(useWorkflowStore.getState().workflows[0].status).toBe('cancelling');
      expect(useWorkflowStore.getState().workflows[0].updated_at).toBe('2026-02-10T00:00:00.000Z');
      expect(useWorkflowStore.getState().activeWorkflow.status).toBe('cancelling');

      vi.useRealTimers();
    });
  });

  describe('fetchTasks', () => {
    it('stores tasks list under workflowId key', async () => {
      workflowApi.getTasks.mockResolvedValue([{ id: 't1' }]);
      const tasks = await useWorkflowStore.getState().fetchTasks('wf-1');
      expect(tasks).toHaveLength(1);
      expect(useWorkflowStore.getState().tasks['wf-1']).toHaveLength(1);
    });
  });

  describe('SSE handlers', () => {
    it('handleWorkflowStarted creates/updates workflow and activates it', () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', title: 'keep-me', created_at: 'old' }] });
      useWorkflowStore.getState().handleWorkflowStarted({
        workflow_id: 'wf-1',
        prompt: 'p',
        timestamp: '2026-02-10T00:00:01Z',
      });

      const state = useWorkflowStore.getState();
      expect(state.activeWorkflow.id).toBe('wf-1');
      expect(state.activeWorkflow.title).toBe('keep-me');
      expect(state.activeWorkflow.status).toBe('running');
    });

    it('handleWorkflowFailed sets aborted when error_code=CANCELLED', () => {
      useWorkflowStore.setState({ workflows: [{ id: 'wf-1', status: 'running' }], activeWorkflow: { id: 'wf-1', status: 'running' } });
      useWorkflowStore.getState().handleWorkflowFailed({
        workflow_id: 'wf-1',
        error_code: 'CANCELLED',
        error: 'cancelled',
        timestamp: '2026-02-10T00:00:02Z',
      });

      const state = useWorkflowStore.getState();
      expect(state.workflows[0].status).toBe('aborted');
      expect(state.activeWorkflow.status).toBe('aborted');
    });
  });
});
