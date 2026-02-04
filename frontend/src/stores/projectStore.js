import { create } from 'zustand';
import { persist, createJSONStorage } from 'zustand/middleware';
import { projectApi } from '../lib/api';
import useWorkflowStore from './workflowStore';
import useKanbanStore from './kanbanStore';
import useChatStore from './chatStore';
import { useConfigStore } from './configStore';

/**
 * Project store for multi-project support.
 * Manages project list, selected project, and project-related operations.
 */
const useProjectStore = create(
  persist(
    (set, get) => ({
      // State
      projects: [],
      currentProjectId: null,
      defaultProjectId: null,
      loading: false,
      error: null,

      // Computed getters
      getCurrentProject: () => {
        const { projects, currentProjectId } = get();
        return projects.find(p => p.id === currentProjectId) || null;
      },

      getDefaultProject: () => {
        const { projects, defaultProjectId } = get();
        return projects.find(p => p.id === defaultProjectId) || null;
      },

      // Actions
      fetchProjects: async () => {
        set({ loading: true, error: null });
        try {
          const projects = await projectApi.list();
          const defaultProject = projects.find(p => p.is_default);
          set({
            projects,
            defaultProjectId: defaultProject?.id || null,
            loading: false,
          });
          return projects;
        } catch (error) {
          set({ error: error.message, loading: false });
          return [];
        }
      },

      fetchProject: async (id) => {
        set({ loading: true, error: null });
        try {
          const project = await projectApi.get(id);
          const { projects } = get();
          const updated = projects.map(p => p.id === id ? project : p);
          if (!projects.find(p => p.id === id)) {
            updated.push(project);
          }
          set({ projects: updated, loading: false });
          return project;
        } catch (error) {
          set({ error: error.message, loading: false });
          return null;
        }
      },

      createProject: async (path, options = {}) => {
        set({ loading: true, error: null });
        try {
          const project = await projectApi.create(path, options);
          const { projects } = get();
          set({
            projects: [...projects, project],
            loading: false,
          });
          return project;
        } catch (error) {
          set({ error: error.message, loading: false });
          return null;
        }
      },

      updateProject: async (id, data) => {
        set({ loading: true, error: null });
        try {
          const project = await projectApi.update(id, data);
          const { projects } = get();
          const updated = projects.map(p => p.id === id ? { ...p, ...project } : p);
          set({ projects: updated, loading: false });
          return project;
        } catch (error) {
          set({ error: error.message, loading: false });
          return null;
        }
      },

      deleteProject: async (id) => {
        set({ loading: true, error: null });
        try {
          await projectApi.delete(id);
          const { projects, currentProjectId, defaultProjectId } = get();
          const newState = {
            projects: projects.filter(p => p.id !== id),
            loading: false,
          };
          // Clear selection if deleted project was selected
          if (currentProjectId === id) {
            newState.currentProjectId = null;
          }
          // Clear default if deleted project was default
          if (defaultProjectId === id) {
            newState.defaultProjectId = null;
          }
          set(newState);
          return true;
        } catch (error) {
          set({ error: error.message, loading: false });
          return false;
        }
      },

      validateProject: async (id) => {
        try {
          const project = await projectApi.validate(id);
          const { projects } = get();
          const updated = projects.map(p => p.id === id ? project : p);
          set({ projects: updated });
          return project;
        } catch (error) {
          set({ error: error.message });
          return null;
        }
      },

      setDefaultProject: async (id) => {
        set({ loading: true, error: null });
        try {
          const project = await projectApi.setDefault(id);
          const { projects } = get();
          // Update is_default flag for all projects
          const updated = projects.map(p => ({
            ...p,
            is_default: p.id === id,
          }));
          set({
            projects: updated,
            defaultProjectId: id,
            loading: false,
          });
          return project;
        } catch (error) {
          set({ error: error.message, loading: false });
          return null;
        }
      },

      /**
       * Select a project and refresh all project-scoped data.
       * This clears and reloads workflows, kanban, chat, and config from the new project.
       */
      selectProject: async (id) => {
        const { currentProjectId } = get();

        // Skip if already on this project
        if (currentProjectId === id) return;

        // Update the current project ID first (this affects API calls via query param)
        set({ currentProjectId: id });

        // Clear and refresh all project-scoped stores
        // These API calls will now include ?project={id}
        try {
          // Refresh workflow data
          useWorkflowStore.setState({
            workflows: [],
            activeWorkflow: null,
            selectedWorkflowId: null,
            tasks: {},
            error: null,
          });
          await useWorkflowStore.getState().fetchWorkflows();

          // Refresh kanban board
          useKanbanStore.setState({
            columns: {
              refinement: [],
              todo: [],
              in_progress: [],
              to_verify: [],
              done: [],
            },
            error: null,
          });
          await useKanbanStore.getState().fetchBoard();

          // Refresh chat sessions
          useChatStore.setState({
            sessions: [],
            activeSessionId: null,
            messages: {},
            error: null,
          });
          await useChatStore.getState().fetchSessions();

          // Refresh configuration (clear etag to force full reload)
          useConfigStore.setState({
            config: null,
            etag: null,
            localChanges: {},
            isDirty: false,
            error: null,
          });
          await useConfigStore.getState().loadConfig();
        } catch (error) {
          console.error('Error refreshing project data:', error);
          set({ error: error.message });
        }
      },

      // Initialize: fetch projects and auto-select default
      initialize: async () => {
        const projects = await get().fetchProjects();
        const { currentProjectId, defaultProjectId } = get();

        // If no project selected, select the default or first available
        if (!currentProjectId && projects.length > 0) {
          const selectedId = defaultProjectId || projects[0]?.id;
          if (selectedId) {
            set({ currentProjectId: selectedId });
          }
        }

        return projects;
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: 'quorum-project-store',
      storage: createJSONStorage(() => localStorage),
      // Only persist the selected project ID, not the full project list
      partialize: (state) => ({
        currentProjectId: state.currentProjectId,
      }),
    }
  )
);

export default useProjectStore;
