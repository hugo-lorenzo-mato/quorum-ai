/**
 * API Contract Schemas
 *
 * JSON Schema definitions for validating API responses.
 * These schemas should match the Go DTO types in internal/api/config_types.go
 */

// Meta schema used in config responses
export const configMetaSchema = {
  type: 'object',
  required: ['etag', 'source'],
  properties: {
    etag: { type: 'string' },
    last_modified: { type: 'string' },
    source: { type: 'string', enum: ['file', 'default'] },
  },
  additionalProperties: false,
};

// Log configuration
export const logConfigSchema = {
  type: 'object',
  required: ['level', 'format'],
  properties: {
    level: { type: 'string' },
    format: { type: 'string' },
  },
  additionalProperties: false,
};

// Trace configuration
export const traceConfigSchema = {
  type: 'object',
  required: ['mode', 'dir', 'schema_version', 'redact', 'max_bytes', 'total_max_bytes', 'max_files'],
  properties: {
    mode: { type: 'string' },
    dir: { type: 'string' },
    schema_version: { type: 'integer' },
    redact: { type: 'boolean' },
    redact_patterns: { type: 'array', items: { type: 'string' } },
    redact_allowlist: { type: 'array', items: { type: 'string' } },
    max_bytes: { type: 'integer' },
    total_max_bytes: { type: 'integer' },
    max_files: { type: 'integer' },
    include_phases: { type: 'array', items: { type: 'string' } },
  },
  additionalProperties: false,
};

// Heartbeat configuration
export const heartbeatConfigSchema = {
  type: 'object',
  required: ['enabled', 'interval', 'stale_threshold', 'check_interval', 'auto_resume', 'max_resumes'],
  properties: {
    enabled: { type: 'boolean' },
    interval: { type: 'string' },
    stale_threshold: { type: 'string' },
    check_interval: { type: 'string' },
    auto_resume: { type: 'boolean' },
    max_resumes: { type: 'integer' },
  },
  additionalProperties: false,
};

// Workflow configuration
export const workflowConfigSchema = {
  type: 'object',
  required: ['timeout', 'max_retries', 'dry_run', 'heartbeat'],
  properties: {
    timeout: { type: 'string' },
    max_retries: { type: 'integer' },
    dry_run: { type: 'boolean' },
    deny_tools: { type: 'array', items: { type: 'string' } },
    heartbeat: heartbeatConfigSchema,
  },
  additionalProperties: false,
};

// Agent configuration
export const agentConfigSchema = {
  type: 'object',
  required: ['enabled', 'model'],
  properties: {
    enabled: { type: 'boolean' },
    model: { type: 'string' },
    reasoning_effort: { type: 'string' },
    timeout: { type: 'string' },
    phases: {
      type: 'object',
      properties: {
        analyze: { type: 'boolean' },
        plan: { type: 'boolean' },
        execute: { type: 'boolean' },
      },
      additionalProperties: false,
    },
    context_window: { type: 'integer' },
    max_output: { type: 'integer' },
  },
  additionalProperties: false,
};

// Agents configuration (map of agent name to config)
export const agentsConfigSchema = {
  type: 'object',
  properties: {
    default: { type: 'string' },
    claude: agentConfigSchema,
    codex: agentConfigSchema,
    gemini: agentConfigSchema,
    copilot: agentConfigSchema,
  },
  required: ['default'],
  additionalProperties: false,
};

// Chat configuration
export const chatConfigSchema = {
  type: 'object',
  required: ['timeout', 'progress_interval', 'editor'],
  properties: {
    timeout: { type: 'string' },
    progress_interval: { type: 'string' },
    editor: { type: 'string' },
  },
  additionalProperties: false,
};

// Report configuration
export const reportConfigSchema = {
  type: 'object',
  required: ['enabled', 'base_dir', 'use_utc', 'include_raw'],
  properties: {
    enabled: { type: 'boolean' },
    base_dir: { type: 'string' },
    use_utc: { type: 'boolean' },
    include_raw: { type: 'boolean' },
  },
  additionalProperties: false,
};

// Git configuration
export const gitConfigSchema = {
  type: 'object',
  required: ['auto_commit'],
  properties: {
    auto_commit: { type: 'boolean' },
    push: { type: 'boolean' },
    worktree: {
      type: 'object',
      properties: {
        enabled: { type: 'boolean' },
        dir: { type: 'string' },
        base_branch: { type: 'string' },
        delete_after: { type: 'boolean' },
      },
      additionalProperties: false,
    },
  },
  additionalProperties: false,
};

// Full config response schema
export const fullConfigResponseSchema = {
  type: 'object',
  required: ['log', 'trace', 'workflow', 'phases', 'agents', 'state', 'git', 'github', 'chat', 'report', 'diagnostics', 'issues'],
  properties: {
    log: logConfigSchema,
    trace: traceConfigSchema,
    workflow: workflowConfigSchema,
    phases: { type: 'object' }, // Complex nested structure
    agents: agentsConfigSchema,
    state: { type: 'object' },
    git: gitConfigSchema,
    github: { type: 'object' },
    chat: chatConfigSchema,
    report: reportConfigSchema,
    diagnostics: { type: 'object' },
    issues: { type: 'object' },
  },
  additionalProperties: false,
};

// Config response with metadata
export const configResponseWithMetaSchema = {
  type: 'object',
  required: ['config', '_meta'],
  properties: {
    config: fullConfigResponseSchema,
    _meta: configMetaSchema,
  },
  additionalProperties: false,
};

// Workflow response schema
export const workflowResponseSchema = {
  type: 'object',
  required: ['id', 'status', 'prompt', 'created_at'],
  properties: {
    id: { type: 'string' },
    status: { type: 'string', enum: ['pending', 'running', 'paused', 'completed', 'failed', 'cancelled'] },
    prompt: { type: 'string' },
    title: { type: 'string' },
    current_phase: { type: 'string' },
    created_at: { type: 'string' },
    updated_at: { type: 'string' },
    started_at: { type: 'string' },
    completed_at: { type: 'string' },
    config: { type: 'object' },
    error: { type: 'string' },
    artifacts: { type: 'object' },
    tasks: { type: 'array' },
    heartbeat: { type: 'object' },
  },
  additionalProperties: true, // Allow additional fields for flexibility
};

// Workflow list response
export const workflowListResponseSchema = {
  type: 'array',
  items: workflowResponseSchema,
};

// Task response schema
export const taskResponseSchema = {
  type: 'object',
  required: ['id', 'title', 'status'],
  properties: {
    id: { type: 'string' },
    title: { type: 'string' },
    description: { type: 'string' },
    status: { type: 'string', enum: ['pending', 'running', 'completed', 'failed', 'skipped'] },
    priority: { type: 'string' },
    agent: { type: 'string' },
    started_at: { type: 'string' },
    completed_at: { type: 'string' },
    error: { type: 'string' },
    output: { type: 'string' },
    files_modified: { type: 'array', items: { type: 'string' } },
  },
  additionalProperties: true,
};

// Error response schema
export const errorResponseSchema = {
  type: 'object',
  required: ['error'],
  properties: {
    error: { type: 'string' },
    message: { type: 'string' },
    code: { type: 'string' },
    details: { type: 'object' },
  },
  additionalProperties: true,
};

// Enum response schema (for /config/enums)
export const enumsResponseSchema = {
  type: 'object',
  properties: {
    log_levels: { type: 'array', items: { type: 'string' } },
    log_formats: { type: 'array', items: { type: 'string' } },
    trace_modes: { type: 'array', items: { type: 'string' } },
    issue_platforms: { type: 'array', items: { type: 'string' } },
    issue_label_schemas: { type: 'array', items: { type: 'string' } },
  },
  additionalProperties: true,
};

// Agents list response (for /config/agents)
export const agentsListResponseSchema = {
  type: 'object',
  additionalProperties: {
    type: 'object',
    properties: {
      name: { type: 'string' },
      enabled: { type: 'boolean' },
      model: { type: 'string' },
      available: { type: 'boolean' },
    },
  },
};

// Kanban board response
export const kanbanBoardResponseSchema = {
  type: 'object',
  required: ['columns', 'engine'],
  properties: {
    columns: {
      type: 'object',
      additionalProperties: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          title: { type: 'string' },
          workflows: { type: 'array' },
        },
      },
    },
    engine: {
      type: 'object',
      properties: {
        enabled: { type: 'boolean' },
        circuit_breaker_open: { type: 'boolean' },
      },
    },
  },
  additionalProperties: true,
};
