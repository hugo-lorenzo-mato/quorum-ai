import { describe, it, expect, vi, beforeEach } from 'vitest';
import { workflowApi } from '../api';
import useProjectStore from '../../stores/projectStore';

describe('workflowApi', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn();
    // Ensure test isolation: api.js will append ?project=... if set.
    useProjectStore.setState({ currentProjectId: null });
  });

  describe('create', () => {
    it('sends blueprint in request body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'Test',
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            prompt: 'Test prompt',
            title: 'Test',
            blueprint: {
              execution_mode: 'single_agent',
              single_agent_name: 'claude',
            },
          }),
        })
      );
    });

    it('omits blueprint when empty object', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {});

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({ prompt: 'Test prompt' });
      expect(callBody.blueprint).toBeUndefined();
      expect(callBody.title).toBeUndefined();
    });

    it('omits blueprint when not provided', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt');

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({ prompt: 'Test prompt' });
      expect(callBody.blueprint).toBeUndefined();
    });

    it('sends title without blueprint', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'My Workflow',
      });

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({
        prompt: 'Test prompt',
        title: 'My Workflow',
      });
      expect(callBody.blueprint).toBeUndefined();
    });

    it('sends files when provided', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        files: ['file1.txt', 'file2.txt'],
      });

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody.files).toEqual(['file1.txt', 'file2.txt']);
    });

    it('omits files when empty array', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        files: [],
      });

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody.files).toBeUndefined();
    });

    it('sends all options together', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'Complete Workflow',
        files: ['doc.pdf'],
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
          single_agent_model: 'claude-3-opus',
        },
      });

      const callBody = JSON.parse(globalThis.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({
        prompt: 'Test prompt',
        title: 'Complete Workflow',
        files: ['doc.pdf'],
        blueprint: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
          single_agent_model: 'claude-3-opus',
        },
      });
    });

    it('throws error on non-ok response', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
        json: () => Promise.resolve({
          error: "single_agent_name required when execution_mode is 'single_agent'",
        }),
      });

      await expect(workflowApi.create('Test', {
        blueprint: { execution_mode: 'single_agent' },
      })).rejects.toThrow("single_agent_name required when execution_mode is 'single_agent'");
    });

    it('handles validation error format', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: false,
        status: 400,
        json: () => Promise.resolve({
          message: 'Validation failed',
        }),
      });

      await expect(workflowApi.create('Test')).rejects.toThrow('Validation failed');
    });

    it('handles network errors gracefully', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.reject(new Error('Invalid JSON')),
      });

      await expect(workflowApi.create('Test')).rejects.toThrow('Internal Server Error');
    });
  });

  describe('replan', () => {
    it('posts without a body when context is empty', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ ok: true }),
      });

      await workflowApi.replan('wf-1', '');

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/replan',
        expect.objectContaining({ method: 'POST' })
      );
      expect(globalThis.fetch.mock.calls[0][1].body).toBeUndefined();
    });

    it('includes context in the JSON body when provided', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ ok: true }),
      });

      await workflowApi.replan('wf-1', 'more context');

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/replan',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ context: 'more context' }),
        })
      );
    });
  });

  describe('review', () => {
    it('maps continueUnattended to continue_unattended in request body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ ok: true }),
      });

      await workflowApi.review('wf-1', {
        action: 'approve',
        feedback: 'looks good',
        phase: 'plan',
        continueUnattended: true,
      });

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/review',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({
            action: 'approve',
            feedback: 'looks good',
            phase: 'plan',
            continue_unattended: true,
          }),
        })
      );
    });
  });

  describe('switchInteractive', () => {
    it('posts to /switch-interactive without a body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ ok: true }),
      });

      await workflowApi.switchInteractive('wf-1');

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/switch-interactive',
        expect.objectContaining({ method: 'POST' })
      );
      expect(globalThis.fetch.mock.calls[0][1].body).toBeUndefined();
    });

    it('appends the project query parameter when a project is selected', async () => {
      useProjectStore.setState({ currentProjectId: 'proj 1' });

      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ ok: true }),
      });

      await workflowApi.switchInteractive('wf-1');

      expect(globalThis.fetch.mock.calls[0][0]).toBe(
        '/api/v1/workflows/wf-1/switch-interactive?project=proj%201'
      );
    });
  });

  describe('task mutations', () => {
    it('createTask posts JSON body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 't1' }),
      });

      await workflowApi.createTask('wf-1', { name: 'x' });

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/tasks/',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ name: 'x' }),
        })
      );
    });

    it('updateTask patches JSON body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 't1' }),
      });

      await workflowApi.updateTask('wf-1', 't1', { name: 'y' });

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/tasks/t1',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ name: 'y' }),
        })
      );
    });

    it('deleteTask sends DELETE without a body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 204,
        json: () => Promise.resolve({}),
      });

      await workflowApi.deleteTask('wf-1', 't1');

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/tasks/t1',
        expect.objectContaining({ method: 'DELETE' })
      );
      expect(globalThis.fetch.mock.calls[0][1].body).toBeUndefined();
    });

    it('reorderTasks puts task_order JSON body', async () => {
      globalThis.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve([{ id: 't2' }, { id: 't1' }]),
      });

      await workflowApi.reorderTasks('wf-1', ['t2', 't1']);

      expect(globalThis.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/wf-1/tasks/reorder',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ task_order: ['t2', 't1'] }),
        })
      );
    });
  });
});
