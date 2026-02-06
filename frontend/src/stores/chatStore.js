import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { chatApi } from '../lib/api';
import { DEFAULT_AGENT, DEFAULT_REASONING } from '../lib/agents';

const useChatStore = create(
  persist(
    (set, get) => ({
  // State
  sessions: [],
  activeSessionId: null,
  messages: {},
  loading: false,
  sending: false,
  error: null,
  sidebarCollapsed: true,

  // Per-message options (reset after send)
  currentAgent: DEFAULT_AGENT,
  currentModel: '',
  currentReasoningEffort: DEFAULT_REASONING,
  attachments: [],

  // Actions
  fetchSessions: async () => {
    set({ loading: true, error: null });
    try {
      const sessions = await chatApi.listSessions();
      set({ sessions, loading: false });
    } catch (error) {
      set({ error: error.message, loading: false });
    }
  },

  createSession: async (agent = DEFAULT_AGENT) => {
    set({ loading: true, error: null });
    try {
      const session = await chatApi.createSession(agent);
      const { sessions } = get();
      set({
        sessions: [...sessions, session],
        activeSessionId: session.id,
        messages: { ...get().messages, [session.id]: [] },
        loading: false,
      });
      return session;
    } catch (error) {
      set({ error: error.message, loading: false });
      return null;
    }
  },

  selectSession: async (sessionId) => {
    set({ activeSessionId: sessionId, error: null });
    const { messages } = get();
    if (!messages[sessionId]) {
      await get().fetchMessages(sessionId);
    }
  },

  deleteSession: async (sessionId) => {
    try {
      await chatApi.deleteSession(sessionId);
      const { sessions, activeSessionId, messages } = get();
      const { [sessionId]: _removed, ...remainingMessages } = messages;
      set({
        sessions: sessions.filter(s => s.id !== sessionId),
        activeSessionId: activeSessionId === sessionId ? null : activeSessionId,
        messages: remainingMessages,
      });
      return true;
    } catch (error) {
      set({ error: error.message });
      return false;
    }
  },

  updateSession: async (sessionId, updates) => {
    try {
      const updatedSession = await chatApi.updateSession(sessionId, updates);
      const { sessions } = get();
      set({
        sessions: sessions.map(s => s.id === sessionId ? { ...s, ...updatedSession } : s),
      });
      return updatedSession;
    } catch (error) {
      set({ error: error.message });
      return null;
    }
  },

  fetchMessages: async (sessionId) => {
    try {
      const messageList = await chatApi.getMessages(sessionId);
      const { messages } = get();
      set({ messages: { ...messages, [sessionId]: messageList } });
      return messageList;
    } catch (error) {
      set({ error: error.message });
      return [];
    }
  },

  sendMessage: async (content) => {
    const {
      activeSessionId, messages, currentAgent, currentModel,
      currentReasoningEffort, attachments,
    } = get();
    if (!activeSessionId) {
      set({ error: 'No active session' });
      return null;
    }

    set({ sending: true, error: null });

    // Optimistically add user message
    const userMessage = {
      id: `temp-${Date.now()}`,
      role: 'user',
      content,
      timestamp: new Date().toISOString(),
    };
    const sessionMessages = messages[activeSessionId] || [];
    set({
      messages: {
        ...messages,
        [activeSessionId]: [...sessionMessages, userMessage],
      },
    });

    try {
      // Send with per-message options
      const response = await chatApi.sendMessage(activeSessionId, content, {
        agent: currentAgent,
        model: currentModel || undefined,
        reasoningEffort: currentReasoningEffort,
        attachments: attachments,
      });

      // Add assistant response
      const assistantMessage = {
        id: response.id || `msg-${Date.now()}`,
        role: 'assistant',
        content: response.content,
        timestamp: response.timestamp || new Date().toISOString(),
        agent: response.agent,
      };

      const { messages: currentMessages } = get();
      const currentSessionMessages = currentMessages[activeSessionId] || [];

      set({
        messages: {
          ...currentMessages,
          [activeSessionId]: [...currentSessionMessages, assistantMessage],
        },
        sending: false,
        // Reset attachments after send (keep agent and model for convenience)
        attachments: [],
      });

      return response;
    } catch (error) {
      // Remove optimistic message on error
      const { messages: currentMessages } = get();
      const currentSessionMessages = currentMessages[activeSessionId] || [];
      set({
        messages: {
          ...currentMessages,
          [activeSessionId]: currentSessionMessages.filter(m => m.id !== userMessage.id),
        },
        error: error.message,
        sending: false,
      });
      return null;
    }
  },

  getActiveMessages: () => {
    const { activeSessionId, messages } = get();
    return messages[activeSessionId] || [];
  },

  getActiveSession: () => {
    const { activeSessionId, sessions } = get();
    return sessions.find(s => s.id === activeSessionId);
  },

  clearError: () => set({ error: null }),

  uploadAttachments: async (fileList) => {
    const { activeSessionId, attachments } = get();
    if (!activeSessionId) {
      set({ error: 'No active session' });
      return [];
    }
    if (!fileList || fileList.length === 0) return [];

    try {
      const uploaded = await chatApi.uploadAttachments(activeSessionId, fileList);
      const uploadedPaths = (uploaded || []).map((a) => a.path).filter(Boolean);
      const next = [...attachments];
      for (const p of uploadedPaths) {
        if (!next.includes(p)) next.push(p);
      }
      set({ attachments: next });
      return uploaded || [];
    } catch (error) {
      set({ error: error.message });
      return [];
    }
  },

  // Per-message option setters
  setCurrentAgent: (agent) => set({ currentAgent: agent, currentModel: '' }),
  setCurrentModel: (model) => set({ currentModel: model }),
  setCurrentReasoningEffort: (effort) => set({ currentReasoningEffort: effort }),
  addAttachment: (path) => {
    const { attachments } = get();
    if (!attachments.includes(path)) {
      set({ attachments: [...attachments, path] });
    }
  },
  removeAttachment: (path) => {
    const { attachments } = get();
    set({ attachments: attachments.filter(a => a !== path) });
  },
  clearAttachments: () => set({ attachments: [] }),
  resetMessageOptions: () => set({
    currentModel: '',
    currentReasoningEffort: DEFAULT_REASONING,
    attachments: [],
  }),

  // Sidebar controls
  toggleSidebar: () => set(state => ({ sidebarCollapsed: !state.sidebarCollapsed })),
  setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
})),
{
  name: 'quorum-chat-store',
  partialize: (state) => ({
    sidebarCollapsed: state.sidebarCollapsed,
  }),
}
);

export default useChatStore;
