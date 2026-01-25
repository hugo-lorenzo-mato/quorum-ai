import { create } from 'zustand';
import { chatApi } from '../lib/api';

const useChatStore = create((set, get) => ({
  // State
  sessions: [],
  activeSessionId: null,
  messages: {},
  loading: false,
  sending: false,
  error: null,

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

  createSession: async (agent = 'claude') => {
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
    set({ activeSessionId: sessionId });
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
    const { activeSessionId, messages } = get();
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
      const response = await chatApi.sendMessage(activeSessionId, content);

      // Add assistant response
      const assistantMessage = {
        id: response.id || `msg-${Date.now()}`,
        role: 'assistant',
        content: response.content,
        timestamp: response.timestamp || new Date().toISOString(),
      };

      const { messages: currentMessages } = get();
      const currentSessionMessages = currentMessages[activeSessionId] || [];

      set({
        messages: {
          ...currentMessages,
          [activeSessionId]: [...currentSessionMessages, assistantMessage],
        },
        sending: false,
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
}));

export default useChatStore;
