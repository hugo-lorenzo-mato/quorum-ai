import { describe, it, expect, vi, beforeEach } from 'vitest';
import useWorkflowStore from '../workflowStore';

// Mock the API module
vi.mock('../../lib/api', () => ({
  workflowApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    getActive: vi.fn(),
  },
}));

// Mock the dependent stores
vi.mock('../agentStore', () => ({
  default: {
    getState: () => ({
      loadPersistedEvents: vi.fn(),
    }),
  },
}));

vi.mock('../taskStore', () => ({
  default: {
    getState: () => ({
      loadPersistedTasks: vi.fn(),
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
    it('creates workflow with single-agent config', async () => {
      const mockWorkflow = {
        id: 'wf-123',
        prompt: 'Test prompt',
        title: 'Test',
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      };

      workflowApi.create.mockResolvedValue(mockWorkflow);

      const result = await useWorkflowStore.getState().createWorkflow('Test prompt', {
        title: 'Test',
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(workflowApi.create).toHaveBeenCalledWith('Test prompt', {
        title: 'Test',
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(result).toEqual(mockWorkflow);
      const state = useWorkflowStore.getState();
      expect(state.workflows).toHaveLength(1);
      expect(state.workflows[0]).toEqual(mockWorkflow);
      expect(state.workflows[0].config.execution_mode).toBe('single_agent');
      expect(state.activeWorkflow).toEqual(mockWorkflow);
      expect(state.loading).toBe(false);
    });

    it('creates workflow with default multi-agent config (no config)', async () => {
      const mockWorkflow = {
        id: 'wf-456',
        prompt: 'Test prompt',
      };

      workflowApi.create.mockResolvedValue(mockWorkflow);

      const result = await useWorkflowStore.getState().createWorkflow('Test prompt');

      expect(workflowApi.create).toHaveBeenCalledWith('Test prompt', {});
      expect(result).toEqual(mockWorkflow);
    });

    it('creates workflow with title only (no config)', async () => {
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
        config: { execution_mode: 'single_agent' },
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
});
