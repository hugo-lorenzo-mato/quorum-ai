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

    // Update current agent status
    const workflowAgents = currentAgents[workflowId] || {};
    const updatedAgents = {
      ...workflowAgents,
      [agent]: {
        status: eventKind,
        message: data.message,
        data: data.data,
        timestamp: data.timestamp,
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
  loadPersistedEvents: (workflowId, events) => {
    if (!events || events.length === 0) return;

    const { agentActivity, currentAgents } = get();

    // Convert persisted events to activity entries (reverse to maintain newest-first order)
    const activityEntries = events.map(event => ({
      id: event.id || `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
      agent: event.agent,
      eventKind: event.event_kind,
      message: event.message,
      data: event.data,
      timestamp: event.timestamp,
    })).reverse().slice(0, MAX_ACTIVITY_ENTRIES);

    // Rebuild current agent statuses from the most recent event per agent
    const agentStatuses = {};
    for (const event of events) {
      // Later events override earlier ones (events are in chronological order)
      agentStatuses[event.agent] = {
        status: event.event_kind,
        message: event.message,
        data: event.data,
        timestamp: event.timestamp,
      };
    }

    set({
      currentAgents: { ...currentAgents, [workflowId]: agentStatuses },
      agentActivity: { ...agentActivity, [workflowId]: activityEntries },
    });
  },
}));

export default useAgentStore;
