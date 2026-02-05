import { create } from 'zustand';

const MAX_ACTIVITY_ENTRIES = 100;

const useAgentStore = create((set, get) => ({
  // Map of workflowId -> { agentName -> current status }
  currentAgents: {},
  // Map of workflowId -> array of activity entries
  agentActivity: {},

  handleAgentEvent: (data) => {
    const { agentActivity, currentAgents } = get();
    const workflowId = data.workflow_id;
    const agent = data.agent;
    const eventKind = data.event_kind;

    // Update current agent status with proper timestamp handling
    const workflowAgents = currentAgents[workflowId] || {};
    const existingAgent = workflowAgents[agent] || {};

    // Determine timestamps based on event kind
    const isStartEvent = eventKind === 'started';
    const isEndEvent = eventKind === 'completed' || eventKind === 'error';

    // startedAt: set on first 'started' event, preserve existing, or extract from data
    let startedAt = existingAgent.startedAt;
    if (isStartEvent) {
      startedAt = data.timestamp;
    } else if (existingAgent.startedAt) {
      // Preserve existing startedAt from previous events
      startedAt = existingAgent.startedAt;
    } else if (data.data?.started_at) {
      // Some events include started_at in their data
      startedAt = data.data.started_at;
    } else {
      // Last resort: use current event timestamp
      startedAt = data.timestamp;
    }

    // completedAt: set when agent completes or errors
    const completedAt = isEndEvent ? data.timestamp : existingAgent.completedAt;

    // durationMs: use from event data if available, otherwise calculate
    let durationMs = existingAgent.durationMs;
    if (isEndEvent) {
      // Prefer duration_ms from event data if available
      if (data.data?.duration_ms != null) {
        durationMs = data.data.duration_ms;
      } else if (startedAt && data.timestamp && startedAt !== data.timestamp) {
        // Calculate from timestamps (only if they're different)
        durationMs = new Date(data.timestamp).getTime() - new Date(startedAt).getTime();
      }
    }

    const updatedAgents = {
      ...workflowAgents,
      [agent]: {
        status: eventKind,
        message: data.message,
        data: data.data,
        timestamp: data.timestamp,
        startedAt,
        completedAt,
        durationMs,
      },
    };

    // Add to activity log
    const workflowActivity = agentActivity[workflowId] || [];
    const newEntry = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      agent,
      eventKind,
      message: data.message,
      data: data.data,
      timestamp: data.timestamp,
    };

    const updatedActivity = [newEntry, ...workflowActivity].slice(0, MAX_ACTIVITY_ENTRIES);

    set({
      currentAgents: { ...currentAgents, [workflowId]: updatedAgents },
      agentActivity: { ...agentActivity, [workflowId]: updatedActivity },
    });
  },

  getAgentStatuses: (workflowId) => get().currentAgents[workflowId] || {},

  getActivityLog: (workflowId) => get().agentActivity[workflowId] || [],

  getActiveAgents: (workflowId) => {
    const agents = get().currentAgents[workflowId] || {};
    return Object.entries(agents)
      .filter(([, info]) => ['started', 'thinking', 'tool_use', 'progress'].includes(info.status))
      .map(([name, info]) => ({ name, ...info }));
  },

  clearActivity: (workflowId) => {
    const { agentActivity, currentAgents } = get();
    const updatedActivity = { ...agentActivity };
    const updatedAgents = { ...currentAgents };
    delete updatedActivity[workflowId];
    delete updatedAgents[workflowId];
    set({ agentActivity: updatedActivity, currentAgents: updatedAgents });
  },

  // Load persisted agent events from workflow API response (for page reload recovery)
  // currentExecutionId filters events to only show those from the current execution
  loadPersistedEvents: (workflowId, events, currentExecutionId) => {
    if (!events || events.length === 0) return;

    // Filter events by execution ID if provided
    // Events without execution_id are included for backwards compatibility
    const filteredEvents = currentExecutionId
      ? events.filter(e => e.execution_id === currentExecutionId || !e.execution_id)
      : events;

    if (filteredEvents.length === 0) return;

    const { agentActivity, currentAgents } = get();

    // Convert persisted events to activity entries (reverse to maintain newest-first order)
    const activityEntries = filteredEvents.map(event => ({
      id: event.id || `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      agent: event.agent,
      eventKind: event.event_kind,
      message: event.message,
      data: event.data,
      timestamp: event.timestamp,
    })).reverse().slice(0, MAX_ACTIVITY_ENTRIES);

    // Rebuild current agent statuses from events with proper timestamp handling
    // First pass: find startedAt timestamp for each agent
    const agentStartTimes = {};
    for (const event of filteredEvents) {
      if (event.event_kind === 'started' && !agentStartTimes[event.agent]) {
        agentStartTimes[event.agent] = event.timestamp;
      }
    }

    // Second pass: build agent statuses
    const agentStatuses = {};
    for (const event of filteredEvents) {
      // Events are in chronological order, so we process them sequentially
      const existing = agentStatuses[event.agent] || {};
      const eventKind = event.event_kind;
      const isEndEvent = eventKind === 'completed' || eventKind === 'error';

      // Use the startedAt from first pass, or event timestamp as fallback
      const startedAt = agentStartTimes[event.agent] || event.timestamp;

      // Track completedAt from completion events
      const completedAt = isEndEvent ? event.timestamp : existing.completedAt;

      // Calculate durationMs when we have both timestamps
      let durationMs = existing.durationMs;
      if (isEndEvent && startedAt && event.timestamp) {
        durationMs = new Date(event.timestamp).getTime() - new Date(startedAt).getTime();
      }

      agentStatuses[event.agent] = {
        status: eventKind,
        message: event.message,
        data: event.data,
        timestamp: event.timestamp,
        startedAt,
        completedAt,
        durationMs,
      };
    }

    set({
      currentAgents: { ...currentAgents, [workflowId]: agentStatuses },
      agentActivity: { ...agentActivity, [workflowId]: activityEntries },
    });
  },
}));

export default useAgentStore;
