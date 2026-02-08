import { describe, it, expect, beforeEach } from 'vitest';
import useExecutionStore from '../executionStore';

const STORE_KEY = 'quorum-execution-store-v1';

function resetStore() {
  try {
    window.localStorage.removeItem(STORE_KEY);
  } catch {
    // ignore
  }
  // Do not replace the whole store state (would remove actions).
  useExecutionStore.setState({
    timelineByWorkflow: {},
    currentAgentsByWorkflow: {},
    metaByWorkflow: {},
  });
}

describe('executionStore', () => {
  beforeEach(() => {
    resetStore();
  });

  it('ingests agent_event SSE and tracks active agent status', () => {
    useExecutionStore.getState().ingestSSEEvent(
      'agent_event',
      {
        workflow_id: 'wf-1',
        event_kind: 'thinking',
        agent: 'claude',
        message: 'Thinking...',
        data: { phase: 'analyze' },
        timestamp: '2026-02-07T00:00:00.000Z',
      },
      'proj-1'
    );

    const key = 'proj-1:wf-1';
    const state = useExecutionStore.getState();
    expect(state.timelineByWorkflow[key]).toHaveLength(1);
    expect(state.timelineByWorkflow[key][0].kind).toBe('agent');
    expect(state.timelineByWorkflow[key][0].event).toBe('thinking');

    const agents = state.currentAgentsByWorkflow[key];
    expect(agents).toBeTruthy();
    expect(agents.claude.status).toBe('thinking');
  });

  it('hydrateFromWorkflowResponse merges and does not overwrite local timeline when backend has fewer events', () => {
    const key = 'proj-1:wf-1';

    // Local SSE event (newest)
    useExecutionStore.getState().ingestSSEEvent(
      'agent_event',
      {
        workflow_id: 'wf-1',
        event_kind: 'tool_use',
        agent: 'claude',
        message: 'Using tool',
        data: { tool: 'read_file' },
        timestamp: '2026-02-07T00:00:05.000Z',
      },
      'proj-1'
    );

    // Backend snapshot with no agent_events yet should not wipe
    useExecutionStore.getState().hydrateFromWorkflowResponse(
      { id: 'wf-1', execution_id: 1, agent_events: [] },
      'proj-1'
    );

    const state = useExecutionStore.getState();
    expect(state.metaByWorkflow[key].executionId).toBe(1);
    expect(state.timelineByWorkflow[key]).toHaveLength(1);
    expect(state.timelineByWorkflow[key][0].event).toBe('tool_use');
  });

  it('clears timeline when execution_id changes (no history required)', () => {
    const key = 'proj-1:wf-1';

    // Set execution_id=1
    useExecutionStore.getState().hydrateFromWorkflowResponse(
      { id: 'wf-1', execution_id: 1, agent_events: [] },
      'proj-1'
    );

    // Add local event under exec 1
    useExecutionStore.getState().ingestSSEEvent(
      'agent_event',
      {
        workflow_id: 'wf-1',
        event_kind: 'started',
        agent: 'claude',
        message: 'Starting',
        timestamp: '2026-02-07T00:00:00.000Z',
      },
      'proj-1'
    );
    expect(useExecutionStore.getState().timelineByWorkflow[key]).toHaveLength(1);

    // New execution_id=2 should clear local timeline
    useExecutionStore.getState().hydrateFromWorkflowResponse(
      { id: 'wf-1', execution_id: 2, agent_events: [] },
      'proj-1'
    );

    const state = useExecutionStore.getState();
    expect(state.metaByWorkflow[key].executionId).toBe(2);
    expect(state.timelineByWorkflow[key]).toHaveLength(0);
    expect(state.currentAgentsByWorkflow[key] || {}).toEqual({});
  });

  it('dedupes persisted agent_events by synthetic id', () => {
    const key = 'proj-1:wf-1';
    const workflow = {
      id: 'wf-1',
      execution_id: 1,
      agent_events: [
        {
          id: 'evt-1',
          event_kind: 'started',
          agent: 'claude',
          message: 'Start',
          timestamp: '2026-02-07T00:00:00.000Z',
          data: {},
          execution_id: 1,
        },
      ],
    };

    useExecutionStore.getState().hydrateFromWorkflowResponse(workflow, 'proj-1');
    useExecutionStore.getState().hydrateFromWorkflowResponse(workflow, 'proj-1');

    const state = useExecutionStore.getState();
    expect(state.timelineByWorkflow[key]).toHaveLength(1);
    // ID is now a synthetic hash (not the backend-assigned evt-1) so that
    // hydrated events merge with identical events that arrived via SSE.
    expect(state.timelineByWorkflow[key][0].id).toMatch(/^syn_/);
  });

  it('dedupes SSE event with identical hydrated event', () => {
    const key = 'proj-1:wf-1';
    const ts = '2026-02-07T00:00:01.000Z';

    // Simulate SSE event arriving first
    useExecutionStore.getState().ingestSSEEvent(
      'agent_event',
      {
        workflow_id: 'wf-1',
        event_kind: 'progress',
        agent: 'codex',
        message: 'Command completed',
        timestamp: ts,
      },
      'proj-1'
    );
    expect(useExecutionStore.getState().timelineByWorkflow[key]).toHaveLength(1);

    // Hydrate the same event from backend persistence — should merge, not duplicate
    useExecutionStore.getState().hydrateFromWorkflowResponse(
      {
        id: 'wf-1',
        execution_id: 1,
        agent_events: [
          {
            id: 'backend-id-123',
            event_kind: 'progress',
            agent: 'codex',
            message: 'Command completed',
            timestamp: ts,
            data: {},
            execution_id: 1,
          },
        ],
      },
      'proj-1'
    );

    const state = useExecutionStore.getState();
    // Must still be 1 entry, not 2 (the core deduplication fix)
    expect(state.timelineByWorkflow[key]).toHaveLength(1);
  });
});
