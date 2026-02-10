import { describe, it, expect, beforeEach, vi } from 'vitest';
import useUIStore from '../uiStore';

function resetStore() {
  try {
    window.localStorage.removeItem('quorum-ui-store');
  } catch {
    // ignore
  }
  useUIStore.setState({
    sidebarOpen: true,
    theme: 'system',
    currentPage: 'dashboard',
    notifications: [],
    sseConnected: false,
    connectionMode: 'disconnected',
    retrySSEFn: null,
  });
  document.documentElement.className = '';
}

describe('uiStore', () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
    vi.useRealTimers();

    // jsdom doesn't implement matchMedia by default.
    window.matchMedia = window.matchMedia || (() => ({ matches: false, addEventListener() {}, removeEventListener() {} }));
  });

  it('toggleSidebar flips sidebarOpen', () => {
    expect(useUIStore.getState().sidebarOpen).toBe(true);
    useUIStore.getState().toggleSidebar();
    expect(useUIStore.getState().sidebarOpen).toBe(false);
  });

  it('setTheme adds dark class for dark-like themes and clears previous classes', () => {
    useUIStore.getState().setTheme('nord');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
    expect(document.documentElement.classList.contains('nord')).toBe(true);

    useUIStore.getState().setTheme('light');
    expect(document.documentElement.classList.contains('dark')).toBe(false);
    expect(document.documentElement.classList.contains('nord')).toBe(false);
  });

  it('setTheme(system) respects prefers-color-scheme', () => {
    window.matchMedia = () => ({ matches: true, addEventListener() {}, removeEventListener() {} });
    useUIStore.getState().setTheme('system');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('addNotification auto-dismisses non-error notifications', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-10T00:00:00.000Z'));

    const id = useUIStore.getState().addNotification({ type: 'success', message: 'ok' });
    expect(useUIStore.getState().notifications).toHaveLength(1);

    vi.advanceTimersByTime(5000);
    expect(useUIStore.getState().notifications.find((n) => n.id === id)).toBeUndefined();

    vi.useRealTimers();
  });

  it('notifyError does not auto-dismiss', () => {
    vi.useFakeTimers();
    const id = useUIStore.getState().notifyError('bad');
    vi.advanceTimersByTime(6000);
    expect(useUIStore.getState().notifications.find((n) => n.id === id)).toBeTruthy();
    vi.useRealTimers();
  });
});

