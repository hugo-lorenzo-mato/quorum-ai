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

      // Sidebar
      toggleSidebar: () => {
        set(state => ({ sidebarOpen: !state.sidebarOpen }));
      },

      setSidebarOpen: (open) => {
        set({ sidebarOpen: open });
      },

      // Theme
      setTheme: (theme) => {
        set({ theme });
        // Apply theme to document
        const root = document.documentElement;
        if (theme === 'system') {
          const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
          root.classList.toggle('dark', prefersDark);
        } else {
          root.classList.toggle('dark', theme === 'dark');
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
