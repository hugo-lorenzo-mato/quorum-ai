import { describe, it, expect, beforeEach, vi } from 'vitest';
import useAgentStore from '../agentStore';

function resetStore() {
  useAgentStore.setState({
    currentAgents: {},
    agentActivity: {},
  });
}

describe('agentStore', () => {
  beforeEach(() => {
    resetStore();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('handleAgentEvent tracks startedAt and completedAt and calculates durationMs', () => {
    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'started',
      message: 'start',
      data: {},
      timestamp: '2026-02-10T00:00:00.000Z',
    });

    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'completed',
      message: 'done',
      data: {},
      timestamp: '2026-02-10T00:00:02.000Z',
    });

    const agent = useAgentStore.getState().currentAgents['wf-1'].claude;
    expect(agent.startedAt).toBe('2026-02-10T00:00:00.000Z');
    expect(agent.completedAt).toBe('2026-02-10T00:00:02.000Z');
    expect(agent.durationMs).toBe(2000);
  });

  it('handleAgentEvent prefers duration_ms from event data', () => {
    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'gemini',
      event_kind: 'started',
      message: 'start',
      data: {},
      timestamp: '2026-02-10T00:00:00.000Z',
    });

    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'gemini',
      event_kind: 'completed',
      message: 'done',
      data: { duration_ms: 123 },
      timestamp: '2026-02-10T00:00:10.000Z',
    });

    const agent = useAgentStore.getState().currentAgents['wf-1'].gemini;
    expect(agent.durationMs).toBe(123);
  });

  it('getActiveAgents returns only agents with active statuses', () => {
    useAgentStore.setState({
      currentAgents: {
        'wf-1': {
          a: { status: 'thinking' },
          b: { status: 'completed' },
          c: { status: 'progress' },
        },
      },
    });

    const active = useAgentStore.getState().getActiveAgents('wf-1');
    expect(active.map((x) => x.name).sort()).toEqual(['a', 'c']);
  });

  it('clearActivity removes activity and status for workflow', () => {
    useAgentStore.setState({
      currentAgents: { 'wf-1': { claude: { status: 'thinking' } } },
      agentActivity: { 'wf-1': [{ id: 'x' }] },
    });

    useAgentStore.getState().clearActivity('wf-1');
    expect(useAgentStore.getState().currentAgents['wf-1']).toBeUndefined();
    expect(useAgentStore.getState().agentActivity['wf-1']).toBeUndefined();
  });

  it('loadPersistedEvents filters by execution_id and rebuilds newest-first activity', () => {
    useAgentStore.getState().loadPersistedEvents(
      'wf-1',
      [
        { id: '1', agent: 'claude', event_kind: 'started', message: 's', timestamp: '2026-02-10T00:00:00Z', data: {}, execution_id: 1 },
        { id: '2', agent: 'claude', event_kind: 'completed', message: 'c', timestamp: '2026-02-10T00:00:01Z', data: {}, execution_id: 1 },
        { id: '3', agent: 'codex', event_kind: 'started', message: 's2', timestamp: '2026-02-10T00:00:02Z', data: {}, execution_id: 2 },
      ],
      1
    );

    const log = useAgentStore.getState().getActivityLog('wf-1');
    expect(log).toHaveLength(2);
    // persisted events are reversed so newest comes first
    expect(log[0].id).toBe('2');
    expect(log[1].id).toBe('1');

    const claude = useAgentStore.getState().getAgentStatuses('wf-1').claude;
    expect(claude.status).toBe('completed');
    expect(claude.durationMs).toBe(1000);
    expect(useAgentStore.getState().getAgentStatuses('wf-1').codex).toBeUndefined();
  });

  it('handleAgentEvent uses crypto.randomUUID for stable activity ids when available', () => {
    vi.stubGlobal('crypto', { randomUUID: () => 'uuid-1' });

    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'thinking',
      message: 'hi',
      data: {},
      timestamp: '2026-02-10T00:00:00.000Z',
    });

    const log = useAgentStore.getState().getActivityLog('wf-1');
    expect(log).toHaveLength(1);
    expect(log[0].id).toBe('uuid-1');
  });

  it('handleAgentEvent falls back to crypto.getRandomValues when randomUUID is unavailable', () => {
    vi.stubGlobal('crypto', {
      getRandomValues: (buf) => {
        // Deterministic bytes for test stability.
        for (let i = 0; i < buf.length; i += 1) buf[i] = i + 1;
        return buf;
      },
    });

    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'thinking',
      message: 'hi',
      data: {},
      timestamp: '2026-02-10T00:00:00.000Z',
    });

    const log = useAgentStore.getState().getActivityLog('wf-1');
    expect(log).toHaveLength(1);
    // 12 bytes => 24 hex chars
    expect(log[0].id).toMatch(/^[0-9a-f]{24}$/);
  });

  it('handleAgentEvent falls back to timestamp+sequence when crypto is unavailable', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-02-10T00:00:00.000Z'));
    vi.stubGlobal('crypto', undefined);

    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'thinking',
      message: '1',
      data: {},
      timestamp: '2026-02-10T00:00:00.000Z',
    });
    useAgentStore.getState().handleAgentEvent({
      workflow_id: 'wf-1',
      agent: 'claude',
      event_kind: 'thinking',
      message: '2',
      data: {},
      timestamp: '2026-02-10T00:00:01.000Z',
    });

    const log = useAgentStore.getState().getActivityLog('wf-1');
    expect(log).toHaveLength(2);
    const nowPrefix = `${Date.now()}-`;
    expect(log[0].id.startsWith(nowPrefix)).toBe(true);
    expect(log[1].id.startsWith(nowPrefix)).toBe(true);

    const seq0 = Number(log[0].id.slice(nowPrefix.length));
    const seq1 = Number(log[1].id.slice(nowPrefix.length));
    expect(Number.isFinite(seq0)).toBe(true);
    expect(Number.isFinite(seq1)).toBe(true);
    // Newest-first log, so seq0 should be seq1+1.
    expect(seq0).toBe(seq1 + 1);

    vi.useRealTimers();
  });
});
