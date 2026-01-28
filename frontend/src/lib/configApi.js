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

  // Handle 304 Not Modified
  if (response.status === 304) {
    return { notModified: true };
  }

  // Handle 412 Precondition Failed (conflict)
  if (response.status === 412) {
    const error = await response.json();
    const conflictError = new Error('Configuration conflict');
    conflictError.code = 'CONFLICT';
    conflictError.currentEtag = error.current_etag;
    conflictError.currentConfig = error.current_config;
    throw conflictError;
  }

  // Handle 422 Validation Errors
  if (response.status === 422) {
    const error = await response.json();
    const validationError = new Error('Validation failed');
    validationError.code = 'VALIDATION_ERROR';
    validationError.errors = error.errors;
    throw validationError;
  }

  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.message || error.error || response.statusText);
  }

  // Extract ETag from headers
  const etag = response.headers.get('ETag')?.replace(/"/g, '');

  const data = await response.json();
  return { data, etag };
}

export const configApi = {
  /**
   * Get full configuration with metadata
   * @param {string} etag - Optional ETag for conditional GET
   */
  get: async (etag) => {
    const headers = {};
    if (etag) {
      headers['If-None-Match'] = `"${etag}"`;
    }
    return request('/config/', { headers });
  },

  /**
   * Update configuration with ETag validation
   * @param {object} updates - Partial config updates
   * @param {string} etag - Current ETag for conflict detection
   * @param {boolean} force - Force update ignoring conflicts
   */
  update: async (updates, etag, force = false) => {
    const headers = {};
    if (etag && !force) {
      headers['If-Match'] = `"${etag}"`;
    }

    const url = force ? '/config/?force=true' : '/config/';

    return request(url, {
      method: 'PATCH',
      headers,
      body: JSON.stringify(updates),
    });
  },

  /**
   * Validate configuration without saving
   * @param {object} updates - Config updates to validate
   */
  validate: async (updates) => {
    return request('/config/validate', {
      method: 'POST',
      body: JSON.stringify(updates),
    });
  },

  /**
   * Reset configuration to defaults
   * @param {string} section - Optional section to reset
   */
  reset: async (section) => {
    const url = section ? `/config/reset?section=${section}` : '/config/reset';
    return request(url, { method: 'POST' });
  },

  /**
   * Get configuration schema
   */
  getSchema: async () => {
    return request('/config/schema');
  },

  /**
   * Get enum values
   */
  getEnums: async () => {
    return request('/config/enums');
  },

  /**
   * Get available agents
   */
  getAgents: async () => {
    return request('/config/agents');
  },
};
