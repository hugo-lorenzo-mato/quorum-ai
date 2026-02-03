const API_BASE = '/api/v1';

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const { timeout, ...restOptions } = options;

  // Create abort controller for timeout
  const controller = new AbortController();
  let timeoutId;
  if (timeout) {
    timeoutId = setTimeout(() => controller.abort(), timeout);
  }

  const config = {
    headers: {
      'Content-Type': 'application/json',
      ...restOptions.headers,
    },
    signal: controller.signal,
    ...restOptions,
  };

  try {
    const response = await fetch(url, config);

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      const message = error.message || error.error || response.statusText;
      throw new Error(message || 'Request failed');
    }

    // Handle 204 No Content
    if (response.status === 204) {
      return null;
    }

    return response.json();
  } catch (err) {
    if (err.name === 'AbortError') {
      throw new Error('Request timed out');
    }
    throw err;
  } finally {
    if (timeoutId) {
      clearTimeout(timeoutId);
    }
  }
}

// Workflow API
export const workflowApi = {
  list: () => request('/workflows/'),

  get: (id) => request(`/workflows/${id}/`),

  getActive: () => request('/workflows/active'),

  /**
   * Create a new workflow with optional configuration.
   * @param {string} prompt - The workflow prompt
   * @param {Object} options - Additional options
   * @param {string} [options.title] - Optional workflow title
   * @param {string[]} [options.files] - Optional file paths
   * @param {Object} [options.config] - Optional workflow configuration
   * @param {string} [options.config.execution_mode] - 'multi_agent' or 'single_agent'
   * @param {string} [options.config.single_agent_name] - Agent name for single-agent mode
   * @param {string} [options.config.single_agent_model] - Optional model override
   */
  create: (prompt, options = {}) => {
    const { title, files, config } = options;

    const body = { prompt };

    // Add optional fields only if they have values
    if (title) body.title = title;
    if (files && files.length > 0) body.files = files;
    if (config && Object.keys(config).length > 0) body.config = config;

    return request('/workflows/', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  update: (id, data) => request(`/workflows/${id}/`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  }),

  activate: (id) => request(`/workflows/${id}/activate`, {
    method: 'POST',
  }),

  getTasks: (workflowId) => request(`/workflows/${workflowId}/tasks/`),

  getTask: (workflowId, taskId) => request(`/workflows/${workflowId}/tasks/${taskId}`),

  run: (id) => request(`/workflows/${id}/run`, { method: 'POST' }),

  cancel: (id) => request(`/workflows/${id}/cancel`, { method: 'POST' }),

  pause: (id) => request(`/workflows/${id}/pause`, { method: 'POST' }),

  resume: (id) => request(`/workflows/${id}/resume`, { method: 'POST' }),

  // Phase-specific execution endpoints
  /**
   * Run only the analyze phase of a workflow.
   * After completion, CurrentPhase will be "plan".
   * @param {string} id - Workflow ID
   * @returns {Promise<Object>} - PhaseResponse with id, status, current_phase, message
   */
  analyze: (id) => request(`/workflows/${id}/analyze`, { method: 'POST' }),

  /**
   * Run only the plan phase of a workflow.
   * Requires completed analyze phase. After completion, CurrentPhase will be "execute".
   * @param {string} id - Workflow ID
   * @returns {Promise<Object>} - PhaseResponse with id, status, current_phase, message
   */
  plan: (id) => request(`/workflows/${id}/plan`, { method: 'POST' }),

  /**
   * Regenerate the plan with optional additional context.
   * Clears existing tasks and regenerates the plan.
   * @param {string} id - Workflow ID
   * @param {string} [context] - Optional additional context to prepend to analysis
   * @returns {Promise<Object>} - PhaseResponse with id, status, current_phase, message
   */
  replan: (id, context = '') => {
    const options = { method: 'POST' };
    if (context) {
      options.body = JSON.stringify({ context });
    }
    return request(`/workflows/${id}/replan`, options);
  },

  /**
   * Run only the execute phase of a workflow.
   * Requires completed plan phase with tasks defined.
   * @param {string} id - Workflow ID
   * @returns {Promise<Object>} - PhaseResponse with id, status, current_phase, message
   */
  execute: (id) => request(`/workflows/${id}/execute`, { method: 'POST' }),

  delete: (id) => request(`/workflows/${id}/`, { method: 'DELETE' }),

  // Issue generation
  /**
   * Generate GitHub/GitLab issues from workflow artifacts.
   * @param {string} id - Workflow ID
   * @param {Object} options - Generation options
   * @param {boolean} [options.dry_run=false] - Preview without creating
   * @param {boolean} [options.create_main_issue=true] - Create parent issue
   * @param {boolean} [options.create_sub_issues=true] - Create child issues
   * @param {boolean} [options.link_issues=true] - Link sub-issues to main
   * @param {string[]} [options.labels] - Override default labels
   * @param {string[]} [options.assignees] - Override default assignees
   */
  generateIssues: (id, options = {}) => request(`/workflows/${id}/issues`, {
    method: 'POST',
    body: JSON.stringify({
      dry_run: options.dryRun ?? false,
      create_main_issue: options.createMainIssue ?? true,
      create_sub_issues: options.createSubIssues ?? true,
      link_issues: options.linkIssues ?? true,
      labels: options.labels,
      assignees: options.assignees,
      issues: options.issues, // Frontend-edited issues
    }),
  }),

  /**
   * Preview issues without creating them.
   * @param {string} id - Workflow ID
   * @param {boolean} fast - Use fast mode (skip LLM generation for instant response)
   * @param {Object} options - Additional options
   * @param {number} [options.timeout] - Request timeout in ms (default: 30s for fast, 180s for AI)
   */
  previewIssues: (id, fast = true, options = {}) => {
    // AI mode can take 60-120 seconds, so use 3 minute timeout
    const timeout = options.timeout ?? (fast ? 30000 : 180000);
    return request(`/workflows/${id}/issues/preview${fast ? '?fast=true' : ''}`, { timeout });
  },

  /**
   * Create a single issue directly.
   * @param {string} workflowId - Workflow ID
   * @param {Object} issue - Issue data (title, body, labels, assignees)
   */
  createSingleIssue: (workflowId, issue) => request(`/workflows/${workflowId}/issues/single`, {
    method: 'POST',
    body: JSON.stringify({
      title: issue.title,
      body: issue.body,
      labels: issue.labels || [],
      assignees: issue.assignees || [],
    }),
  }),

  // Workflow attachments
  listAttachments: (id) => request(`/workflows/${id}/attachments`),

  uploadAttachments: async (id, files) => {
    const formData = new FormData();
    for (const file of files) {
      formData.append('files', file);
    }

    const response = await fetch(`${API_BASE}/workflows/${id}/attachments`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      const message = error.message || error.error || response.statusText;
      throw new Error(message || 'Upload failed');
    }

    return response.json();
  },

  deleteAttachment: (id, attachmentId) => request(`/workflows/${id}/attachments/${attachmentId}`, {
    method: 'DELETE',
  }),
};

// Chat API
export const chatApi = {
  listSessions: () => request('/chat/sessions'),

  createSession: (agent = 'claude') => request('/chat/sessions', {
    method: 'POST',
    body: JSON.stringify({ agent }),
  }),

  getSession: (id) => request(`/chat/sessions/${id}`),

  deleteSession: (id) => request(`/chat/sessions/${id}`, {
    method: 'DELETE',
  }),

  updateSession: (id, data) => request(`/chat/sessions/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  }),

  getMessages: (sessionId) => request(`/chat/sessions/${sessionId}/messages`),

  sendMessage: (sessionId, content, options = {}) => request(`/chat/sessions/${sessionId}/messages`, {
    method: 'POST',
    body: JSON.stringify({
      content,
      agent: options.agent || undefined,
      model: options.model || undefined,
      reasoning_effort: options.reasoningEffort || undefined,
      attachments: options.attachments?.length > 0 ? options.attachments : undefined,
    }),
  }),

  uploadAttachments: async (sessionId, files) => {
    const formData = new FormData();
    for (const file of files) {
      formData.append('files', file);
    }

    const response = await fetch(`${API_BASE}/chat/sessions/${sessionId}/attachments`, {
      method: 'POST',
      body: formData,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      const message = error.message || error.error || response.statusText;
      throw new Error(message || 'Upload failed');
    }

    return response.json();
  },
};

// Config API
export const configApi = {
  get: () => request('/config/'),

  update: (config) => request('/config/', {
    method: 'PATCH',
    body: JSON.stringify(config),
  }),

  getAgents: () => request('/config/agents'),

  getEnums: () => request('/config/enums'),

  getIssuesConfig: () => request('/config/issues'),
};

// Files API
export const fileApi = {
  list: (path = '') => request(`/files/?path=${encodeURIComponent(path)}`),

  getContent: (path) => request(`/files/content?path=${encodeURIComponent(path)}`),

  getTree: (path = '', maxDepth = 3) => request(`/files/tree?path=${encodeURIComponent(path)}&max_depth=${maxDepth}`),
};

// Alias for backward compatibility
export const filesApi = fileApi;

// Health API
export const healthApi = {
  check: () => fetch('/health').then(r => r.json()),
};

// Kanban API
export const kanbanApi = {
  // Get full board state
  getBoard: () => request('/kanban/board'),

  // Move workflow to a different column
  moveWorkflow: (workflowId, toColumn, position = 0) => request(`/kanban/workflows/${workflowId}/move`, {
    method: 'POST',
    body: JSON.stringify({ to_column: toColumn, position }),
  }),

  // Engine control
  getEngineState: () => request('/kanban/engine'),

  enableEngine: () => request('/kanban/engine/enable', { method: 'POST' }),

  disableEngine: () => request('/kanban/engine/disable', { method: 'POST' }),

  resetCircuitBreaker: () => request('/kanban/engine/reset-circuit-breaker', { method: 'POST' }),
};

export default {
  workflow: workflowApi,
  chat: chatApi,
  config: configApi,
  files: fileApi,
  health: healthApi,
  kanban: kanbanApi,
};
