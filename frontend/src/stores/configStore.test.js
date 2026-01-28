import { renderHook, act } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { useConfigStore } from './configStore';

// Reset store before each test
beforeEach(() => {
  useConfigStore.setState({
    config: null,
    etag: null,
    localChanges: {},
    isDirty: false,
    isLoading: false,
    error: null,
    validationErrors: {},
  });
});

describe('configStore', () => {
  describe('setField', () => {
    it('tracks local changes', () => {
      const { result } = renderHook(() => useConfigStore());

      // Set initial config
      act(() => {
        useConfigStore.setState({
          config: { log: { level: 'info' } },
        });
      });

      // Make a change
      act(() => {
        result.current.setField('log.level', 'debug');
      });

      expect(result.current.localChanges).toEqual({ log: { level: 'debug' } });
      expect(result.current.isDirty).toBe(true);
    });

    it('removes change when reverting to original', () => {
      const { result } = renderHook(() => useConfigStore());

      act(() => {
        useConfigStore.setState({
          config: { log: { level: 'info' } },
          localChanges: { log: { level: 'debug' } },
          isDirty: true,
        });
      });

      // Revert to original
      act(() => {
        result.current.setField('log.level', 'info');
      });

      expect(result.current.isDirty).toBe(false);
    });
  });

  describe('getFieldValue', () => {
    it('returns local value when present', () => {
      const { result } = renderHook(() => useConfigStore());

      act(() => {
        useConfigStore.setState({
          config: { log: { level: 'info' } },
          localChanges: { log: { level: 'debug' } },
        });
      });

      expect(result.current.getFieldValue('log.level')).toBe('debug');
    });

    it('returns server value when no local change', () => {
      const { result } = renderHook(() => useConfigStore());

      act(() => {
        useConfigStore.setState({
          config: { log: { level: 'info' } },
          localChanges: {},
        });
      });

      expect(result.current.getFieldValue('log.level')).toBe('info');
    });
  });

  describe('discardChanges', () => {
    it('clears local changes and validation errors', () => {
      const { result } = renderHook(() => useConfigStore());

      act(() => {
        useConfigStore.setState({
          config: { log: { level: 'info' } },
          localChanges: { log: { level: 'debug' } },
          isDirty: true,
          validationErrors: { 'log.level': 'Invalid' },
        });
      });

      act(() => {
        result.current.discardChanges();
      });

      expect(result.current.localChanges).toEqual({});
      expect(result.current.isDirty).toBe(false);
      expect(result.current.validationErrors).toEqual({});
    });
  });
});
