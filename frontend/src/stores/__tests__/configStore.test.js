import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useConfigStore } from '../configStore';

// Mock fetch
globalThis.fetch = vi.fn();

describe('configStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    useConfigStore.setState({
      config: null,
      localChanges: {},
      etag: null,
      isLoading: false,
      error: null,
      validationErrors: {},
      hasConflict: false,
      conflictConfig: null,
      conflictEtag: null,
      isDirty: false,
    });
    vi.clearAllMocks();
  });

  describe('setField', () => {
    it('tracks local changes', () => {
      useConfigStore.setState({
        config: { log: { level: 'info' } },
      });

      useConfigStore.getState().setField('log.level', 'debug');

      const state = useConfigStore.getState();
      expect(state.localChanges.log?.level).toBe('debug');
      expect(state.isDirty).toBe(true);
    });

    it('removes change if value matches server', () => {
      useConfigStore.setState({
        config: { log: { level: 'info' } },
        localChanges: { log: { level: 'debug' } },
        isDirty: true,
      });

      useConfigStore.getState().setField('log.level', 'info');

      const state = useConfigStore.getState();
      expect(state.localChanges.log?.level).toBeUndefined();
    });
  });

  describe('getFieldValue', () => {
    it('returns local value if set', () => {
      useConfigStore.setState({
        config: { log: { level: 'info' } },
        localChanges: { log: { level: 'debug' } },
      });

      const value = useConfigStore.getState().getFieldValue('log.level');
      expect(value).toBe('debug');
    });

    it('returns server value if no local change', () => {
      useConfigStore.setState({
        config: { log: { level: 'info' } },
        localChanges: {},
      });

      const value = useConfigStore.getState().getFieldValue('log.level');
      expect(value).toBe('info');
    });
  });

  describe('isFieldDirty', () => {
    it('returns true when field has local changes', () => {
      useConfigStore.setState({
        localChanges: { log: { level: 'debug' } },
      });

      expect(useConfigStore.getState().isFieldDirty('log.level')).toBe(true);
    });

    it('returns false when field has no local changes', () => {
      useConfigStore.setState({
        localChanges: {},
      });

      expect(useConfigStore.getState().isFieldDirty('log.level')).toBe(false);
    });
  });

  describe('discardChanges', () => {
    it('clears local changes', () => {
      useConfigStore.setState({
        localChanges: { log: { level: 'debug' }, trace: { enabled: true } },
        isDirty: true,
        validationErrors: { 'log.level': 'error' },
      });

      useConfigStore.getState().discardChanges();

      const state = useConfigStore.getState();
      expect(state.localChanges).toEqual({});
      expect(state.isDirty).toBe(false);
      expect(state.validationErrors).toEqual({});
    });
  });

  describe('acceptServerVersion', () => {
    it('accepts server config and clears conflict', () => {
      useConfigStore.setState({
        config: { log: { level: 'info' } },
        localChanges: { log: { level: 'debug' } },
        isDirty: true,
        hasConflict: true,
        conflictConfig: { log: { level: 'warn' } },
        conflictEtag: '"new-etag"',
      });

      useConfigStore.getState().acceptServerVersion();

      const state = useConfigStore.getState();
      expect(state.config).toEqual({ log: { level: 'warn' } });
      expect(state.etag).toBe('"new-etag"');
      expect(state.localChanges).toEqual({});
      expect(state.isDirty).toBe(false);
      expect(state.hasConflict).toBe(false);
    });
  });
});
