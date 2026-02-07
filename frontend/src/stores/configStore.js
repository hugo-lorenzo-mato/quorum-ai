import { createContext, useContext, createElement } from 'react';
import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { configApi, globalConfigApi } from '../lib/configApi';

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

const baseInitialState = {
  // Server state
  config: null,
  etag: null,
  lastModified: null,
  source: null,

  // Meta
  scope: null, // "global" | "project"
  projectConfigMode: null, // "inherit_global" | "custom"
  isReadOnly: false,

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

function createConfigStore(api, opts) {
  const options = {
    supportsSectionReset: true,
    readOnlyFromMeta: false,
    ...opts,
  };

  return create(
    subscribeWithSelector((set, get) => ({
      ...baseInitialState,

      // =========================================================================
      // ACTIONS
      // =========================================================================

      loadConfig: async () => {
        const { etag } = get();
        set({ isLoading: true, error: null });

        try {
          const result = await api.get(etag);

          // Handle 304 Not Modified
          if (result.notModified) {
            set({ isLoading: false });
            return;
          }

          const meta = result.data?._meta || {};
          const projectConfigMode = meta.project_config_mode || null;

          set({
            config: result.data.config,
            etag: meta.etag || result.etag,
            lastModified: meta.last_modified,
            source: meta.source,
            scope: meta.scope || null,
            projectConfigMode,
            isReadOnly: options.readOnlyFromMeta && projectConfigMode === 'inherit_global',

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

      loadMetadata: async () => {
        try {
          const [schemaResult, enumsResult, agentsResult] = await Promise.all([
            api.getSchema(),
            api.getEnums(),
            api.getAgents(),
          ]);

          set({
            schema: schemaResult.data,
            enums: enumsResult.data,
            agents: agentsResult.data,
          });
        } catch (err) {
          // Non-fatal (UI can still render using fallbacks)
          console.error('Failed to load config metadata:', err);
        }
      },

      setField: (path, value) => {
        const { config, localChanges, isReadOnly } = get();
        if (isReadOnly) return;

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
          const stack = [];
          let current = newChanges;
          for (let i = 0; i < keys.length - 1; i++) {
            if (!current[keys[i]] || typeof current[keys[i]] !== 'object') {
              current = null;
              break;
            }
            stack.push([current, keys[i]]);
            current = current[keys[i]];
          }

          if (current) {
            delete current[keys[keys.length - 1]];

            // Prune empty objects created by the deletion
            for (let i = stack.length - 1; i >= 0; i--) {
              const [parent, key] = stack[i];
              const child = parent[key];
              if (
                child &&
                typeof child === 'object' &&
                !Array.isArray(child) &&
                Object.keys(child).length === 0
              ) {
                delete parent[key];
              }
            }
          }
        }

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

      getFieldValue: (path) => {
        const { config, localChanges } = get();
        const localValue = getAtPath(localChanges, path);
        if (localValue !== undefined) return localValue;
        return getAtPath(config, path);
      },

      isFieldDirty: (path) => {
        const { localChanges } = get();
        return getAtPath(localChanges, path) !== undefined;
      },

      validateChanges: async () => {
        const { localChanges, isReadOnly } = get();
        if (isReadOnly) return false;
        if (Object.keys(localChanges).length === 0) return true;

        set({ isValidating: true, validationErrors: {} });

        try {
          const result = await api.validate(localChanges);

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

      saveChanges: async () => {
        const { localChanges, etag, isDirty, isReadOnly } = get();
        if (isReadOnly) return false;
        if (!isDirty) return true;

        set({ error: null });

        const isValid = await get().validateChanges();
        if (!isValid) {
          return false;
        }

        set({ isSaving: true });

        try {
          const result = await api.update(localChanges, etag);
          const meta = result.data?._meta || {};
          const projectConfigMode = meta.project_config_mode || null;

          set({
            config: result.data.config,
            etag: meta.etag || result.etag,
            lastModified: meta.last_modified,
            source: meta.source,
            scope: meta.scope || null,
            projectConfigMode,
            isReadOnly: options.readOnlyFromMeta && projectConfigMode === 'inherit_global',

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

      forceSave: async () => {
        const { localChanges, etag, isReadOnly } = get();
        if (isReadOnly) return false;

        set({ isSaving: true, error: null, hasConflict: false });

        try {
          const result = await api.update(localChanges, etag, true);
          const meta = result.data?._meta || {};
          const projectConfigMode = meta.project_config_mode || null;

          set({
            config: result.data.config,
            etag: meta.etag || result.etag,
            lastModified: meta.last_modified,
            source: meta.source,
            scope: meta.scope || null,
            projectConfigMode,
            isReadOnly: options.readOnlyFromMeta && projectConfigMode === 'inherit_global',

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

      discardChanges: () => {
        const { isReadOnly } = get();
        if (isReadOnly) return;
        set({
          localChanges: {},
          isDirty: false,
          validationErrors: {},
        });
      },

      resetToDefaults: async (section) => {
        const { isReadOnly } = get();
        if (isReadOnly) return false;

        set({ isLoading: true, error: null });

        try {
          const result = await api.reset(options.supportsSectionReset ? section : undefined);
          const meta = result.data?._meta || {};
          const projectConfigMode = meta.project_config_mode || null;

          set({
            config: result.data.config,
            etag: meta.etag || result.etag,
            lastModified: meta.last_modified,
            source: meta.source,
            scope: meta.scope || null,
            projectConfigMode,
            isReadOnly: options.readOnlyFromMeta && projectConfigMode === 'inherit_global',

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

      clearError: () => set({ error: null }),

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
}

export const projectConfigStore = createConfigStore(configApi, {
  supportsSectionReset: true,
  readOnlyFromMeta: true,
});

export const globalConfigStore = createConfigStore(globalConfigApi, {
  supportsSectionReset: false,
  readOnlyFromMeta: false,
});

const ConfigStoreContext = createContext(projectConfigStore);

export function ConfigStoreProvider({ store, children }) {
  return createElement(ConfigStoreContext.Provider, { value: store }, children);
}

// useConfigStore is context-aware (defaults to projectConfigStore), but keeps legacy
// Zustand store methods (getState/setState/subscribe) wired to the project store.
export function useConfigStore(selector) {
  const store = useContext(ConfigStoreContext);
  return store(selector);
}

// Legacy, non-React usage (always the project-scoped store).
useConfigStore.getState = projectConfigStore.getState;
useConfigStore.setState = projectConfigStore.setState;
useConfigStore.subscribe = projectConfigStore.subscribe;
useConfigStore.destroy = projectConfigStore.destroy;

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
