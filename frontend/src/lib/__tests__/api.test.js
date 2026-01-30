import { describe, it, expect, vi, beforeEach } from 'vitest';
import { workflowApi } from '../api';

describe('workflowApi', () => {
  beforeEach(() => {
    global.fetch = vi.fn();
  });

  describe('create', () => {
    it('sends config in request body', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'Test',
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
        },
      });

      expect(global.fetch).toHaveBeenCalledWith(
        '/api/v1/workflows/',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            prompt: 'Test prompt',
            title: 'Test',
            config: {
              execution_mode: 'single_agent',
              single_agent_name: 'claude',
            },
          }),
        })
      );
    });

    it('omits config when empty object', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {});

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({ prompt: 'Test prompt' });
      expect(callBody.config).toBeUndefined();
      expect(callBody.title).toBeUndefined();
    });

    it('omits config when not provided', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt');

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({ prompt: 'Test prompt' });
      expect(callBody.config).toBeUndefined();
    });

    it('sends title without config', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'My Workflow',
      });

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({
        prompt: 'Test prompt',
        title: 'My Workflow',
      });
      expect(callBody.config).toBeUndefined();
    });

    it('sends files when provided', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        files: ['file1.txt', 'file2.txt'],
      });

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody.files).toEqual(['file1.txt', 'file2.txt']);
    });

    it('omits files when empty array', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        files: [],
      });

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody.files).toBeUndefined();
    });

    it('sends all options together', async () => {
      global.fetch.mockResolvedValue({
        ok: true,
        status: 200,
        json: () => Promise.resolve({ id: 'wf-123' }),
      });

      await workflowApi.create('Test prompt', {
        title: 'Complete Workflow',
        files: ['doc.pdf'],
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
          single_agent_model: 'claude-3-opus',
        },
      });

      const callBody = JSON.parse(global.fetch.mock.calls[0][1].body);
      expect(callBody).toEqual({
        prompt: 'Test prompt',
        title: 'Complete Workflow',
        files: ['doc.pdf'],
        config: {
          execution_mode: 'single_agent',
          single_agent_name: 'claude',
          single_agent_model: 'claude-3-opus',
        },
      });
    });

    it('throws error on non-ok response', async () => {
      global.fetch.mockResolvedValue({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
        json: () => Promise.resolve({
          error: "single_agent_name required when execution_mode is 'single_agent'",
        }),
      });

      await expect(workflowApi.create('Test', {
        config: { execution_mode: 'single_agent' },
      })).rejects.toThrow("single_agent_name required when execution_mode is 'single_agent'");
    });

    it('handles validation error format', async () => {
      global.fetch.mockResolvedValue({
        ok: false,
        status: 400,
        json: () => Promise.resolve({
          message: 'Validation failed',
        }),
      });

      await expect(workflowApi.create('Test')).rejects.toThrow('Validation failed');
    });

    it('handles network errors gracefully', async () => {
      global.fetch.mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.reject(new Error('Invalid JSON')),
      });

      await expect(workflowApi.create('Test')).rejects.toThrow('Internal Server Error');
    });
  });
});
