import { create } from 'zustand';
import { persist } from 'zustand/middleware';

/**
 * Issues Editor Store
 * Manages state for the issues generation and editing workflow.
 */
const useIssuesStore = create(
  persist(
    (set, get) => ({
      // ─────────────────────────────────────────────────────────────
      // State
      // ─────────────────────────────────────────────────────────────

      workflowId: null,
      workflowTitle: '',

      // Issues data
      originalIssues: [],     // From API, for comparison
      editedIssues: [],       // User's edits
      selectedIssueId: null,

      // UI State
      viewMode: 'edit',       // 'edit' | 'preview'

      // Generation state
      generating: false,
      generationMode: null,   // 'fast' | 'ai'
      generationProgress: 0,  // Number of issues generated
      generationTotal: 0,     // Total issues to generate
      generatedIssues: [],    // Issues generated so far (for streaming UI)

      // Submission state
      submitting: false,
      error: null,

      // AI generation info (for debugging)
      aiUsed: false,
      aiErrors: [],

      // ─────────────────────────────────────────────────────────────
      // Actions
      // ─────────────────────────────────────────────────────────────

      /**
       * Set workflow context
       */
      setWorkflow: (id, title) => set({
        workflowId: id,
        workflowTitle: title
      }),

      /**
       * Load issues from API response
       * @param {Array} issues - Issues to load
       * @param {Object} aiInfo - Optional AI generation info
       */
      loadIssues: (issues, aiInfo = {}) => {
        const issuesWithIds = issues.map((issue, idx) => ({
          ...issue,
          _localId: issue.task_id || (issue.is_main_issue ? 'main' : `issue-${idx}`),
          _modified: false,
        }));

        // Sort issues by task_id numerically (00-consolidated, 01-task, 02-task, etc.)
        const extractNumber = (s) => {
          const match = String(s || '').match(/\d+/);
          return match ? Number(match[0]) : Number.MAX_SAFE_INTEGER;
        };

        issuesWithIds.sort((a, b) => {
          // Sort by task_id/localId numerically
          const aNum = extractNumber(a.task_id || a._localId);
          const bNum = extractNumber(b.task_id || b._localId);
          return aNum - bNum;
        });

        set({
          originalIssues: JSON.parse(JSON.stringify(issuesWithIds)),
          editedIssues: issuesWithIds,
          selectedIssueId: issuesWithIds[0]?._localId || null,
          generating: false,
          generationProgress: 0,
          generatedIssues: [],
          aiUsed: aiInfo.ai_used || false,
          aiErrors: aiInfo.ai_errors || [],
        });
      },

      /**
       * Select an issue for editing
       */
      selectIssue: (id) => set({ selectedIssueId: id }),

      /**
       * Get currently selected issue
       */
      getSelectedIssue: () => {
        const { editedIssues, selectedIssueId } = get();
        return editedIssues.find(i => i._localId === selectedIssueId) || null;
      },

      /**
       * Update a specific issue
       */
      updateIssue: (id, updates) => {
        const { editedIssues } = get();
        set({
          editedIssues: editedIssues.map(issue =>
            issue._localId === id
              ? { ...issue, ...updates, _modified: true }
              : issue
          ),
        });
      },

      /**
       * Check if a specific issue has unsaved changes
       */
      hasUnsavedChanges: (id) => {
        const { originalIssues, editedIssues } = get();
        const original = originalIssues.find(i => i._localId === id);
        const edited = editedIssues.find(i => i._localId === id);
        if (!original || !edited) return false;

        return (
          original.title !== edited.title ||
          original.body !== edited.body ||
          JSON.stringify(original.labels) !== JSON.stringify(edited.labels) ||
          JSON.stringify(original.assignees) !== JSON.stringify(edited.assignees)
        );
      },

      /**
       * Check if any issue has unsaved changes
       */
      hasAnyUnsavedChanges: () => {
        const { editedIssues } = get();
        return editedIssues.some(issue => get().hasUnsavedChanges(issue._localId));
      },

      /**
       * Toggle view mode between edit and preview
       */
      setViewMode: (mode) => set({ viewMode: mode }),
      toggleViewMode: () => set(state => ({
        viewMode: state.viewMode === 'edit' ? 'preview' : 'edit'
      })),

      // ─────────────────────────────────────────────────────────────
      // Generation Actions
      // ─────────────────────────────────────────────────────────────

      /**
       * Start generation process
       */
      startGeneration: (mode, totalIssues = 0) => set({
        generating: true,
        generationMode: mode,
        generationProgress: 0,
        generationTotal: totalIssues,
        generatedIssues: [],
        error: null,
      }),

      /**
       * Update generation progress (for streaming UI)
       */
      updateGenerationProgress: (progress, issue = null) => {
        const { generatedIssues } = get();
        set({
          generationProgress: progress,
          generatedIssues: issue
            ? [...generatedIssues, issue]
            : generatedIssues,
        });
      },

      /**
       * Complete generation
       */
      finishGeneration: (issues) => {
        get().loadIssues(issues);
      },

      /**
       * Cancel generation
       */
      cancelGeneration: () => set({
        generating: false,
        generationMode: null,
        generationProgress: 0,
        generationTotal: 0,
        generatedIssues: [],
      }),

      // ─────────────────────────────────────────────────────────────
      // Submission Actions
      // ─────────────────────────────────────────────────────────────

      /**
       * Start submission
       */
      setSubmitting: (submitting) => set({ submitting }),

      /**
       * Set error
       */
      setError: (error) => set({ error }),

      /**
       * Clear error
       */
      clearError: () => set({ error: null }),

      /**
       * Get issues ready for submission (mapped to API format)
       */
      getIssuesForSubmission: () => {
        const { editedIssues } = get();
        return editedIssues.map(issue => ({
          title: issue.title,
          body: issue.body,
          labels: issue.labels || [],
          assignees: issue.assignees || [],
          is_main_issue: issue.is_main_issue || false,
          task_id: issue.task_id || null,
        }));
      },

      /**
       * Reset draft to original
       */
      resetToOriginal: () => {
        const { originalIssues } = get();
        set({
          editedIssues: JSON.parse(JSON.stringify(originalIssues)),
        });
      },

      /**
       * Reset entire store
       */
      reset: () => set({
        workflowId: null,
        workflowTitle: '',
        originalIssues: [],
        editedIssues: [],
        selectedIssueId: null,
        viewMode: 'edit',
        generating: false,
        generationMode: null,
        generationProgress: 0,
        generationTotal: 0,
        generatedIssues: [],
        submitting: false,
        error: null,
        aiUsed: false,
        aiErrors: [],
      }),
    }),
    {
      name: 'quorum-issues-editor',
      // Only persist draft data, not UI state
      partialize: (state) => ({
        workflowId: state.workflowId,
        workflowTitle: state.workflowTitle,
        originalIssues: state.originalIssues,
        editedIssues: state.editedIssues,
        selectedIssueId: state.selectedIssueId,
      }),
    }
  )
);

export default useIssuesStore;
