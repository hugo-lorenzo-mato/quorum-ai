import { describe, it, expect, vi, beforeEach } from 'vitest';
import useChatStore from '../chatStore';

vi.mock('../../lib/api', () => ({
  chatApi: {
    listSessions: vi.fn(),
    createSession: vi.fn(),
    deleteSession: vi.fn(),
    updateSession: vi.fn(),
    getMessages: vi.fn(),
    sendMessage: vi.fn(),
    uploadAttachments: vi.fn(),
  },
}));

import { chatApi } from '../../lib/api';

function resetStore() {
  try {
    window.localStorage.removeItem('quorum-chat-store');
  } catch {
    // ignore
  }

  // Do not replace the whole store state (would remove actions).
  useChatStore.setState({
    sessions: [],
    activeSessionId: null,
    messages: {},
    loading: false,
    sending: false,
    error: null,
    sidebarCollapsed: true,
    currentModel: '',
    attachments: [],
  });
}

describe('chatStore', () => {
  beforeEach(() => {
    resetStore();
    vi.clearAllMocks();
  });

  it('fetchSessions loads sessions list', async () => {
    chatApi.listSessions.mockResolvedValue([{ id: 's1' }]);
    const p = useChatStore.getState().fetchSessions();
    expect(useChatStore.getState().loading).toBe(true);
    await p;
    expect(useChatStore.getState().loading).toBe(false);
    expect(useChatStore.getState().sessions).toHaveLength(1);
  });

  it('createSession sets activeSessionId and initializes messages bucket', async () => {
    chatApi.createSession.mockResolvedValue({ id: 's1', agent: 'claude' });
    const session = await useChatStore.getState().createSession('claude');
    expect(session.id).toBe('s1');
    expect(useChatStore.getState().activeSessionId).toBe('s1');
    expect(useChatStore.getState().messages['s1']).toEqual([]);
  });

  it('selectSession fetches messages when not cached', async () => {
    useChatStore.setState({ sessions: [{ id: 's1' }] });
    chatApi.getMessages.mockResolvedValue([{ id: 'm1' }]);
    await useChatStore.getState().selectSession('s1');
    expect(chatApi.getMessages).toHaveBeenCalledWith('s1');
    expect(useChatStore.getState().messages['s1']).toHaveLength(1);
  });

  it('deleteSession removes session and message bucket', async () => {
    useChatStore.setState({
      sessions: [{ id: 's1' }, { id: 's2' }],
      activeSessionId: 's1',
      messages: { s1: [{ id: 'm1' }], s2: [] },
    });
    chatApi.deleteSession.mockResolvedValue({});

    const ok = await useChatStore.getState().deleteSession('s1');
    expect(ok).toBe(true);
    expect(useChatStore.getState().sessions.map(s => s.id)).toEqual(['s2']);
    expect(useChatStore.getState().activeSessionId).toBeNull();
    expect(useChatStore.getState().messages.s1).toBeUndefined();
  });

  it('updateSession merges updates into sessions list', async () => {
    useChatStore.setState({ sessions: [{ id: 's1', title: 'old' }] });
    chatApi.updateSession.mockResolvedValue({ title: 'new' });
    const res = await useChatStore.getState().updateSession('s1', { title: 'new' });
    expect(res.title).toBe('new');
    expect(useChatStore.getState().sessions[0].title).toBe('new');
  });

  it('sendMessage errors when there is no active session', async () => {
    const res = await useChatStore.getState().sendMessage('hi');
    expect(res).toBeNull();
    expect(useChatStore.getState().error).toBe('No active session');
  });

  it('sendMessage adds optimistic user message and appends assistant response', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-10T00:00:00.000Z'));

    useChatStore.setState({
      activeSessionId: 's1',
      messages: { s1: [] },
      currentAgent: 'claude',
      currentModel: '',
      currentReasoningEffort: 'medium',
      attachments: ['a.txt'],
    });

    chatApi.sendMessage.mockResolvedValue({
      id: 'm2',
      content: 'hello',
      timestamp: '2026-02-10T00:00:01.000Z',
      agent: 'claude',
    });

    const res = await useChatStore.getState().sendMessage('hi');
    expect(res.id).toBe('m2');

    const msgs = useChatStore.getState().messages.s1;
    expect(msgs).toHaveLength(2);
    expect(msgs[0].role).toBe('user');
    expect(msgs[1].role).toBe('assistant');
    expect(useChatStore.getState().sending).toBe(false);
    expect(useChatStore.getState().attachments).toEqual([]); // reset after send

    vi.useRealTimers();
  });

  it('sendMessage removes optimistic message on failure', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-10T00:00:00.000Z'));

    useChatStore.setState({ activeSessionId: 's1', messages: { s1: [] } });
    chatApi.sendMessage.mockRejectedValue(new Error('nope'));

    const res = await useChatStore.getState().sendMessage('hi');
    expect(res).toBeNull();
    expect(useChatStore.getState().messages.s1).toHaveLength(0);
    expect(useChatStore.getState().error).toBe('nope');
    expect(useChatStore.getState().sending).toBe(false);

    vi.useRealTimers();
  });

  it('uploadAttachments dedupes returned paths and persists into attachments list', async () => {
    useChatStore.setState({ activeSessionId: 's1', attachments: ['a.txt'] });
    chatApi.uploadAttachments.mockResolvedValue([{ path: 'a.txt' }, { path: 'b.txt' }, { path: '' }]);

    const res = await useChatStore.getState().uploadAttachments([{ name: 'x' }]);
    expect(res).toHaveLength(3);
    expect(useChatStore.getState().attachments).toEqual(['a.txt', 'b.txt']);
  });

  it('uploadAttachments errors when there is no active session', async () => {
    const res = await useChatStore.getState().uploadAttachments([{ name: 'x' }]);
    expect(res).toEqual([]);
    expect(useChatStore.getState().error).toBe('No active session');
  });
});

