/**
 * Contract Tests
 *
 * These tests validate that our API schemas are correct and can validate
 * realistic response data. They serve as the "contract" between frontend and backend.
 *
 * If these tests fail after a backend change, it means the frontend needs to be updated
 * to match the new API contract.
 */

import { describe, it, expect } from 'vitest';
import { validateSchema, assertSchema, formatValidationErrors } from '../validator';
import {
  configMetaSchema,
  logConfigSchema,
  workflowConfigSchema,
  configResponseWithMetaSchema,
  workflowResponseSchema,
  workflowListResponseSchema,
  taskResponseSchema,
  errorResponseSchema,
  enumsResponseSchema,
  kanbanBoardResponseSchema,
} from '../schemas';

describe('Contract Schemas', () => {
  describe('configMetaSchema', () => {
    it('validates valid config meta', () => {
      const validMeta = {
        etag: '"abc123"',
        source: 'file',
        last_modified: '2024-01-15T10:30:00Z',
      };
      const { valid, errors } = validateSchema(configMetaSchema, validMeta);
      expect(valid).toBe(true);
      expect(errors).toBeNull();
    });

    it('validates meta with only required fields', () => {
      const minimalMeta = {
        etag: '"abc123"',
        source: 'default',
      };
      const { valid } = validateSchema(configMetaSchema, minimalMeta);
      expect(valid).toBe(true);
    });

    it('rejects invalid source value', () => {
      const invalidMeta = {
        etag: '"abc123"',
        source: 'invalid',
      };
      const { valid, errors } = validateSchema(configMetaSchema, invalidMeta);
      expect(valid).toBe(false);
      expect(errors).toBeDefined();
      expect(errors[0].keyword).toBe('enum');
    });

    it('rejects missing required fields', () => {
      const incompleteMeta = {
        etag: '"abc123"',
      };
      const { valid, errors } = validateSchema(configMetaSchema, incompleteMeta);
      expect(valid).toBe(false);
      expect(errors[0].keyword).toBe('required');
    });
  });

  describe('logConfigSchema', () => {
    it('validates valid log config', () => {
      const validLog = {
        level: 'info',
        format: 'auto',
      };
      const { valid } = validateSchema(logConfigSchema, validLog);
      expect(valid).toBe(true);
    });

    it('rejects missing required fields', () => {
      const invalidLog = {
        level: 'info',
      };
      const { valid, errors } = validateSchema(logConfigSchema, invalidLog);
      expect(valid).toBe(false);
      expect(errors[0].message).toContain('format');
    });
  });

  describe('workflowConfigSchema', () => {
    it('validates valid workflow config', () => {
      const validWorkflow = {
        timeout: '30m',
        max_retries: 3,
        dry_run: false,
        sandbox: true,
        deny_tools: ['rm', 'sudo'],
        heartbeat: {
          enabled: true,
          interval: '30s',
          stale_threshold: '5m',
          check_interval: '1m',
          auto_resume: true,
          max_resumes: 3,
        },
      };
      const { valid } = validateSchema(workflowConfigSchema, validWorkflow);
      expect(valid).toBe(true);
    });
  });

  describe('workflowResponseSchema', () => {
    it('validates valid workflow response', () => {
      const validWorkflow = {
        id: 'wf_123',
        status: 'running',
        prompt: 'Fix the bug in login',
        title: 'Fix login bug',
        current_phase: 'analyze',
        created_at: '2024-01-15T10:30:00Z',
        updated_at: '2024-01-15T10:35:00Z',
      };
      const { valid } = validateSchema(workflowResponseSchema, validWorkflow);
      expect(valid).toBe(true);
    });

    it('validates minimal workflow response', () => {
      const minimalWorkflow = {
        id: 'wf_123',
        status: 'pending',
        prompt: 'Do something',
        created_at: '2024-01-15T10:30:00Z',
      };
      const { valid } = validateSchema(workflowResponseSchema, minimalWorkflow);
      expect(valid).toBe(true);
    });

    it('validates all workflow statuses', () => {
      const statuses = ['pending', 'running', 'paused', 'completed', 'failed', 'cancelled'];
      for (const status of statuses) {
        const workflow = {
          id: 'wf_123',
          status,
          prompt: 'Test',
          created_at: '2024-01-15T10:30:00Z',
        };
        const { valid } = validateSchema(workflowResponseSchema, workflow);
        expect(valid).toBe(true);
      }
    });

    it('rejects invalid status', () => {
      const invalidWorkflow = {
        id: 'wf_123',
        status: 'invalid_status',
        prompt: 'Test',
        created_at: '2024-01-15T10:30:00Z',
      };
      const { valid, errors } = validateSchema(workflowResponseSchema, invalidWorkflow);
      expect(valid).toBe(false);
      expect(errors[0].keyword).toBe('enum');
    });
  });

  describe('workflowListResponseSchema', () => {
    it('validates workflow list', () => {
      const workflows = [
        {
          id: 'wf_1',
          status: 'completed',
          prompt: 'Task 1',
          created_at: '2024-01-15T10:30:00Z',
        },
        {
          id: 'wf_2',
          status: 'running',
          prompt: 'Task 2',
          created_at: '2024-01-15T11:00:00Z',
        },
      ];
      const { valid } = validateSchema(workflowListResponseSchema, workflows);
      expect(valid).toBe(true);
    });

    it('validates empty list', () => {
      const { valid } = validateSchema(workflowListResponseSchema, []);
      expect(valid).toBe(true);
    });
  });

  describe('taskResponseSchema', () => {
    it('validates valid task response', () => {
      const validTask = {
        id: 'task_1',
        title: 'Implement feature',
        description: 'Add new feature to the app',
        status: 'completed',
        priority: 'high',
        agent: 'claude',
        started_at: '2024-01-15T10:30:00Z',
        completed_at: '2024-01-15T10:45:00Z',
        files_modified: ['src/app.js', 'src/utils.js'],
      };
      const { valid } = validateSchema(taskResponseSchema, validTask);
      expect(valid).toBe(true);
    });

    it('validates all task statuses', () => {
      const statuses = ['pending', 'running', 'completed', 'failed', 'skipped'];
      for (const status of statuses) {
        const task = {
          id: 'task_1',
          title: 'Test task',
          status,
        };
        const { valid } = validateSchema(taskResponseSchema, task);
        expect(valid).toBe(true);
      }
    });
  });

  describe('errorResponseSchema', () => {
    it('validates error response', () => {
      const error = {
        error: 'Not found',
        message: 'Workflow not found',
        code: 'WORKFLOW_NOT_FOUND',
      };
      const { valid } = validateSchema(errorResponseSchema, error);
      expect(valid).toBe(true);
    });

    it('validates minimal error response', () => {
      const error = {
        error: 'Something went wrong',
      };
      const { valid } = validateSchema(errorResponseSchema, error);
      expect(valid).toBe(true);
    });
  });

  describe('enumsResponseSchema', () => {
    it('validates enums response', () => {
      const enums = {
        log_levels: ['debug', 'info', 'warn', 'error'],
        log_formats: ['auto', 'text', 'json'],
        trace_modes: ['none', 'local', 'remote'],
      };
      const { valid } = validateSchema(enumsResponseSchema, enums);
      expect(valid).toBe(true);
    });
  });

  describe('kanbanBoardResponseSchema', () => {
    it('validates kanban board response', () => {
      const board = {
        columns: {
          pending: {
            id: 'pending',
            title: 'Pending',
            workflows: [],
          },
          running: {
            id: 'running',
            title: 'Running',
            workflows: [],
          },
        },
        engine: {
          enabled: true,
          circuit_breaker_open: false,
        },
      };
      const { valid } = validateSchema(kanbanBoardResponseSchema, board);
      expect(valid).toBe(true);
    });
  });
});

describe('Validator utilities', () => {
  describe('assertSchema', () => {
    it('returns data when valid', () => {
      const data = { etag: '"abc"', source: 'file' };
      const result = assertSchema(configMetaSchema, data);
      expect(result).toEqual(data);
    });

    it('throws on invalid data', () => {
      const invalidData = { etag: '"abc"' };
      expect(() => assertSchema(configMetaSchema, invalidData)).toThrow('contract violation');
    });

    it('includes context in error message', () => {
      const invalidData = { etag: '"abc"' };
      expect(() => assertSchema(configMetaSchema, invalidData, 'GET /config')).toThrow('GET /config');
    });
  });

  describe('formatValidationErrors', () => {
    it('formats errors correctly', () => {
      const { errors } = validateSchema(configMetaSchema, { etag: '"abc"' });
      const formatted = formatValidationErrors(errors);
      expect(formatted).toContain('source');
    });

    it('handles empty errors', () => {
      const formatted = formatValidationErrors([]);
      expect(formatted).toBe('No errors');
    });

    it('handles null errors', () => {
      const formatted = formatValidationErrors(null);
      expect(formatted).toBe('No errors');
    });
  });
});

describe('Full Config Response Contract', () => {
  it('validates realistic full config response', () => {
    // This represents the actual structure returned by GET /api/v1/config/
    const fullResponse = {
      config: {
        log: { level: 'info', format: 'auto' },
        trace: {
          mode: 'local',
          dir: '.quorum/traces',
          schema_version: 1,
          redact: true,
          redact_patterns: [],
          redact_allowlist: [],
          max_bytes: 10485760,
          total_max_bytes: 104857600,
          max_files: 100,
          include_phases: ['analyze', 'plan', 'execute'],
        },
        workflow: {
          timeout: '30m',
          max_retries: 2,
          dry_run: false,
          sandbox: true,
          deny_tools: [],
          heartbeat: {
            enabled: true,
            interval: '30s',
            stale_threshold: '5m',
            check_interval: '1m',
            auto_resume: true,
            max_resumes: 3,
          },
        },
        phases: {
          analyze: {},
          plan: {},
          execute: {},
        },
        agents: {
          default: 'claude',
          claude: { enabled: true, model: 'claude-sonnet-4-20250514' },
          codex: { enabled: false, model: 'o3-mini' },
          gemini: { enabled: false, model: 'gemini-2.5-pro' },
          copilot: { enabled: false, model: 'gpt-4.1' },
        },
        state: {},
        git: {
          auto_commit: true,
          push: false,
          worktree: {
            enabled: false,
            dir: '.worktrees',
            base_branch: 'main',
            delete_after: false,
          },
        },
        github: {},
        chat: {
          timeout: '3m',
          progress_interval: '15s',
          editor: 'vim',
        },
        report: {
          enabled: true,
          base_dir: '.quorum/runs',
          use_utc: true,
          include_raw: true,
        },
        diagnostics: {},
        issues: {},
      },
      _meta: {
        etag: '"d41d8cd98f00b204e9800998ecf8427e"',
        source: 'file',
        last_modified: '2024-01-15T10:30:00Z',
      },
    };

    const { valid, errors } = validateSchema(configResponseWithMetaSchema, fullResponse);
    if (!valid) {
      console.error('Validation errors:', formatValidationErrors(errors));
    }
    expect(valid).toBe(true);
  });
});
