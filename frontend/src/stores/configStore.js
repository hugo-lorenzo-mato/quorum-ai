import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { configApi } from '../lib/configApi';

// Get value at path
function getAtPath(obj, path) {
  return path.split('.').reduce((curr, key) => curr?.[key], obj);
}

// Set value at path
function setAtPath(obj, path, value) {
  const result = { ...obj };
  const keys = path.split('.');
  let current = result;

  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i];
    current[key] = { ...current[key] };
    current = current[key];
  }

  current[keys[keys.length - 1]] = value;
  return result;
}

// Initial state
const initialState = {
  // Server state
  config: null,
  etag: null,
  lastModified: null,
  source: null,

  // Local state
  localChanges: {},
  isDirty: false,

  // UI state
  isLoading: false,
  isSaving: false,
  isValidating: false,
  error: null,
  validationErrors: {},

  // Conflict state
  hasConflict: false,
  conflictConfig: null,
  conflictEtag: null,

  // Metadata
  schema: null,
  enums: null,
  agents: null,
};

export const useConfigStore = create(
  subscribeWithSelector((set, get) => ({
    ...initialState,

    // =========================================================================
    // ACTIONS
    // =========================================================================

    /**
     * Load configuration from server
     */
    loadConfig: async () => {
      const { etag } = get();
      set({ isLoading: true, error: null });

      try {
        const result = await configApi.get(etag);

        // Handle 304 Not Modified
        if (result.notModified) {
          set({ isLoading: false });
          return;
        }

        set({
          config: result.data.config,
          etag: result.data._meta?.etag || result.etag,
          lastModified: result.data._meta?.last_modified,
          source: result.data._meta?.source,
          localChanges: {},
          isDirty: false,
          isLoading: false,
          hasConflict: false,
          conflictConfig: null,
          conflictEtag: null,
        });
      } catch (err) {
        set({
          error: err.message,
          isLoading: false,
        });
        throw err;
      }
    },

    /**
     * Load schema, enums, and agents metadata
     */
    loadMetadata: async () => {
      try {
        const [schemaResult, enumsResult, agentsResult] = await Promise.all([
          configApi.getSchema(),
          configApi.getEnums(),
          configApi.getAgents(),
        ]);

        set({
          schema: schemaResult.data,
          enums: enumsResult.data,
          agents: agentsResult.data,
        });
      } catch (err) {
        console.error('Failed to load config metadata:', err);
      }
    },

    /**
     * Update a single field locally
     */
    setField: (path, value) => {
      const { config, localChanges } = get();

      // Get original value
      const originalValue = getAtPath(config, path);

      // Check if value is different from original
      const isDifferent = JSON.stringify(value) !== JSON.stringify(originalValue);

      let newChanges;
      if (isDifferent) {
        newChanges = setAtPath(localChanges, path, value);
      } else {
        // Remove from changes if reverting to original
        newChanges = { ...localChanges };
        const keys = path.split('.');
        let current = newChanges;
        for (let i = 0; i < keys.length - 1; i++) {
          if (!current[keys[i]]) break;
          current = current[keys[i]];
        }
        delete current[keys[keys.length - 1]];
      }

      // Check if any changes remain
      const isDirty = Object.keys(newChanges).length > 0 &&
        JSON.stringify(newChanges) !== '{}';

      set({
        localChanges: newChanges,
        isDirty,
      });

      // Clear validation error for this field
      const { validationErrors } = get();
      if (validationErrors[path]) {
        const newErrors = { ...validationErrors };
        delete newErrors[path];
        set({ validationErrors: newErrors });
      }
    },

    /**
     * Get effective value for a field (local change or server value)
     */
    getFieldValue: (path) => {
      const { config, localChanges } = get();
      const localValue = getAtPath(localChanges, path);
      if (localValue !== undefined) return localValue;
      return getAtPath(config, path);
    },

    /**
     * Check if a field has local changes
     */
    isFieldDirty: (path) => {
      const { localChanges } = get();
      return getAtPath(localChanges, path) !== undefined;
    },

    /**
     * Validate current changes without saving
     */
    validateChanges: async () => {
      const { localChanges } = get();
      if (Object.keys(localChanges).length === 0) return true;

      set({ isValidating: true, validationErrors: {} });

      try {
        const result = await configApi.validate(localChanges);

        if (!result.data.valid) {
          const errors = {};
          for (const err of result.data.errors) {
            errors[err.field] = err.message;
          }
          set({ validationErrors: errors, isValidating: false });
          return false;
        }

        set({ isValidating: false });
        return true;
      } catch (err) {
        set({
          error: err.message,
          isValidating: false,
        });
        return false;
      }
    },

    /**
     * Save changes to server
     */
    saveChanges: async () => {
      const { localChanges, etag, isDirty } = get();
      if (!isDirty) return;

      set({ isSaving: true, error: null });

      try {
        const result = await configApi.update(localChanges, etag);

        set({
          config: result.data.config,
          etag: result.data._meta?.etag || result.etag,
          lastModified: result.data._meta?.last_modified,
          localChanges: {},
          isDirty: false,
          isSaving: false,
          validationErrors: {},
          hasConflict: false,
        });

        return true;
      } catch (err) {
        if (err.code === 'CONFLICT') {
          set({
            hasConflict: true,
            conflictConfig: err.currentConfig,
            conflictEtag: err.currentEtag,
            isSaving: false,
          });
          return false;
        }

        if (err.code === 'VALIDATION_ERROR') {
          const errors = {};
          for (const e of err.errors) {
            errors[e.field] = e.message;
          }
          set({
            validationErrors: errors,
            isSaving: false,
          });
          return false;
        }

        set({
          error: err.message,
          isSaving: false,
        });
        return false;
      }
    },

    /**
     * Force save, ignoring conflicts
     */
    forceSave: async () => {
      const { localChanges, etag } = get();

      set({ isSaving: true, error: null, hasConflict: false });

      try {
        const result = await configApi.update(localChanges, etag, true);

        set({
          config: result.data.config,
          etag: result.data._meta?.etag || result.etag,
          lastModified: result.data._meta?.last_modified,
          localChanges: {},
          isDirty: false,
          isSaving: false,
          validationErrors: {},
          hasConflict: false,
          conflictConfig: null,
          conflictEtag: null,
        });

        return true;
      } catch (err) {
        set({
          error: err.message,
          isSaving: false,
        });
        return false;
      }
    },

    /**
     * Accept server version on conflict
     */
    acceptServerVersion: () => {
      const { conflictConfig, conflictEtag } = get();

      set({
        config: conflictConfig,
        etag: conflictEtag,
        localChanges: {},
        isDirty: false,
        hasConflict: false,
        conflictConfig: null,
        conflictEtag: null,
      });
    },

    /**
     * Discard local changes
     */
    discardChanges: () => {
      set({
        localChanges: {},
        isDirty: false,
        validationErrors: {},
      });
    },

    /**
     * Reset configuration to defaults
     */
    resetToDefaults: async (section) => {
      set({ isLoading: true, error: null });

      try {
        const result = await configApi.reset(section);

        set({
          config: result.data.config,
          etag: result.data._meta?.etag || result.etag,
          lastModified: result.data._meta?.last_modified,
          source: 'default',
          localChanges: {},
          isDirty: false,
          isLoading: false,
          validationErrors: {},
        });

        return true;
      } catch (err) {
        set({
          error: err.message,
          isLoading: false,
        });
        return false;
      }
    },

    /**
     * Clear error
     */
    clearError: () => set({ error: null }),

    /**
     * Clear validation error for specific field
     */
    clearFieldError: (path) => {
      const { validationErrors } = get();
      if (validationErrors[path]) {
        const newErrors = { ...validationErrors };
        delete newErrors[path];
        set({ validationErrors: newErrors });
      }
    },
  }))
);

// =========================================================================
// SELECTORS
// =========================================================================

export const selectConfig = (state) => state.config;
export const selectEtag = (state) => state.etag;
export const selectIsDirty = (state) => state.isDirty;
export const selectIsLoading = (state) => state.isLoading;
export const selectIsSaving = (state) => state.isSaving;
export const selectError = (state) => state.error;
export const selectValidationErrors = (state) => state.validationErrors;
export const selectHasConflict = (state) => state.hasConflict;
export const selectSchema = (state) => state.schema;
export const selectEnums = (state) => state.enums;
export const selectAgents = (state) => state.agents;

// Field-level selectors
export const selectFieldValue = (path) => (state) => {
  const localValue = getAtPath(state.localChanges, path);
  if (localValue !== undefined) return localValue;
  return getAtPath(state.config, path);
};

export const selectFieldError = (path) => (state) => state.validationErrors[path];

export const selectIsFieldDirty = (path) => (state) => {
  return getAtPath(state.localChanges, path) !== undefined;
};

// Export for use in hooks
export { getAtPath };
