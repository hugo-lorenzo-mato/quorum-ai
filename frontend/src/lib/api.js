const API_BASE = '/api/v1';

async function request(endpoint, options = {}) {
  const url = `${API_BASE}${endpoint}`;
  const config = {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  };

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
}

// Workflow API
export const workflowApi = {
  list: () => request('/workflows/'),

  get: (id) => request(`/workflows/${id}/`),

  getActive: () => request('/workflows/active'),

  create: (prompt, config = {}) => request('/workflows/', {
    method: 'POST',
    body: JSON.stringify({ prompt, title: config.title, config }),
  }),

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

  delete: (id) => request(`/workflows/${id}/`, { method: 'DELETE' }),

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

export default {
  workflow: workflowApi,
  chat: chatApi,
  config: configApi,
  files: fileApi,
  health: healthApi,
};
