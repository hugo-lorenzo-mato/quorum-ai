import { create } from 'zustand';
import { persist } from 'zustand/middleware';

const useUIStore = create(
  persist(
    (set, get) => ({
      // State
      sidebarOpen: true,
      theme: 'system',
      currentPage: 'dashboard',
      notifications: [],
      sseConnected: false,
      connectionMode: 'disconnected', // 'sse', 'polling', 'disconnected'
      retrySSEFn: null, // Function to retry SSE connection

      // Sidebar
      toggleSidebar: () => {
        set(state => ({ sidebarOpen: !state.sidebarOpen }));
      },

      setSidebarOpen: (open) => {
        set({ sidebarOpen: open });
      },

      // Theme (supports: light, dark, midnight, sepia, high-contrast, dracula, nord, ocean, system)
      setTheme: (theme) => {
        set({ theme });
        const root = document.documentElement;
        // Remove all theme classes
        root.classList.remove('dark', 'midnight', 'sepia', 'high-contrast', 'dracula', 'nord', 'ocean');

        // Themes that should behave as "dark" for Tailwind's built-in `dark:` variant.
        const darkThemes = new Set(['dark', 'midnight', 'dracula', 'nord', 'ocean', 'high-contrast']);

        if (theme === 'system') {
          const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
          if (prefersDark) root.classList.add('dark');
        } else if (theme !== 'light') {
          // Add the theme class, and also add `.dark` for dark-like themes so `dark:` works consistently.
          if (darkThemes.has(theme)) {
            root.classList.add('dark');
            if (theme !== 'dark') root.classList.add(theme);
          } else {
            root.classList.add(theme);
          }
        }
      },

      // Navigation
      setCurrentPage: (page) => {
        set({ currentPage: page });
      },

      // Notifications
      addNotification: (notification) => {
        const { notifications } = get();
        const id = Date.now().toString();
        const newNotification = {
          id,
          timestamp: new Date().toISOString(),
          ...notification,
        };
        set({ notifications: [...notifications, newNotification] });

        // Auto-dismiss after 5 seconds for non-error notifications
        if (notification.type !== 'error') {
          setTimeout(() => {
            get().removeNotification(id);
          }, 5000);
        }

        return id;
      },

      removeNotification: (id) => {
        const { notifications } = get();
        set({ notifications: notifications.filter(n => n.id !== id) });
      },

      clearNotifications: () => {
        set({ notifications: [] });
      },

      // SSE Connection status
      setSSEConnected: (connected) => {
        set({ sseConnected: connected });
      },

      setConnectionMode: (mode) => {
        set({ connectionMode: mode });
      },

      setRetrySSEFn: (fn) => {
        set({ retrySSEFn: fn });
      },

      // Notification helpers
      notifySuccess: (message) => {
        return get().addNotification({ type: 'success', message });
      },

      notifyError: (message) => {
        return get().addNotification({ type: 'error', message });
      },

      notifyWarning: (message) => {
        return get().addNotification({ type: 'warning', message });
      },

      notifyInfo: (message) => {
        return get().addNotification({ type: 'info', message });
      },
    }),
    {
      name: 'quorum-ui-store',
      partialize: (state) => ({
        sidebarOpen: state.sidebarOpen,
        theme: state.theme,
      }),
    }
  )
);

export default useUIStore;
