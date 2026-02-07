import { useCallback, useMemo } from 'react';
import { useConfigStore, getAtPath } from '../stores/configStore';

/**
 * Hook for accessing and updating a single configuration field
 *
 * @param {string} path - Dot-notation path to the field (e.g., "log.level")
 * @returns {object} Field state and handlers
 */
export function useConfigField(path) {
  const setField = useConfigStore((state) => state.setField);
  const clearFieldError = useConfigStore((state) => state.clearFieldError);
  const isLoading = useConfigStore((state) => state.isLoading);
  const isSaving = useConfigStore((state) => state.isSaving);
  const isReadOnly = useConfigStore((state) => state.isReadOnly);

  // Get effective value (local change or server value)
  const value = useConfigStore(
    useCallback(
      (state) => {
        const localValue = getAtPath(state.localChanges, path);
        if (localValue !== undefined) return localValue;
        return getAtPath(state.config, path);
      },
      [path]
    )
  );

  // Get validation error for this field
  const error = useConfigStore(
    useCallback((state) => state.validationErrors[path], [path])
  );

  // Check if field has local changes
  const isDirty = useConfigStore(
    useCallback((state) => getAtPath(state.localChanges, path) !== undefined, [path])
  );

  const onChange = useCallback(
    (newValue) => {
      setField(path, newValue);
    },
    [path, setField]
  );

  const clearError = useCallback(() => {
    clearFieldError(path);
  }, [path, clearFieldError]);

  return useMemo(
    () => ({
      value,
      onChange,
      error,
      isDirty,
      clearError,
      disabled: isLoading || isSaving || isReadOnly,
    }),
    [value, onChange, error, isDirty, clearError, isLoading, isSaving, isReadOnly]
  );
}

/**
 * Hook for a toggle/boolean field with confirmation dialog support
 */
export function useConfigToggle(path, options = {}) {
  const field = useConfigField(path);
  const { requireConfirmation } = options;

  const onToggle = useCallback(
    (newValue) => {
      // If turning ON and requires confirmation, caller handles dialog
      if (requireConfirmation && newValue === true) {
        return { needsConfirmation: true, value: newValue };
      }
      field.onChange(newValue);
      return { needsConfirmation: false };
    },
    [field, requireConfirmation]
  );

  return {
    ...field,
    checked: field.value,
    onToggle,
  };
}

/**
 * Hook for a select field with enum options
 */
export function useConfigSelect(path, enumKey) {
  const field = useConfigField(path);
  const enums = useConfigStore((state) => state.enums);

  const options = useMemo(() => {
    if (!enums || !enumKey || !enums[enumKey]) return [];
    return enums[enumKey].map((val) => ({
      value: val,
      label: val.charAt(0).toUpperCase() + val.slice(1),
    }));
  }, [enums, enumKey]);

  return {
    ...field,
    options,
  };
}

/**
 * Hook for nested object fields (like agent config)
 */
export function useConfigSection(basePath) {
  const setField = useConfigStore((state) => state.setField);
  const config = useConfigStore((state) => state.config);
  const localChanges = useConfigStore((state) => state.localChanges);
  const validationErrors = useConfigStore((state) => state.validationErrors);
  const isLoading = useConfigStore((state) => state.isLoading);
  const isSaving = useConfigStore((state) => state.isSaving);
  const isReadOnly = useConfigStore((state) => state.isReadOnly);

  const getField = useCallback(
    (subPath) => {
      const fullPath = `${basePath}.${subPath}`;
      const localValue = getAtPath(localChanges, fullPath);
      const serverValue = getAtPath(config, fullPath);

      return {
        value: localValue !== undefined ? localValue : serverValue,
        error: validationErrors[fullPath],
        isDirty: localValue !== undefined,
        onChange: (val) => setField(fullPath, val),
        disabled: isLoading || isSaving || isReadOnly,
      };
    },
    [basePath, config, localChanges, validationErrors, setField, isLoading, isSaving, isReadOnly]
  );

  return { getField };
}
