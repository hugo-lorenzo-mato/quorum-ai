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
    const error = await response.json().catch(() => ({ message: response.statusText }));
    throw new Error(error.message || 'Request failed');
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
    body: JSON.stringify({ prompt, config }),
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

  sendMessage: (sessionId, content) => request(`/chat/sessions/${sessionId}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  }),
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
