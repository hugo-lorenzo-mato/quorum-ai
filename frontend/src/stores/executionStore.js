import { create } from 'zustand';
import { persist } from 'zustand/middleware';

const STORE_NAME = 'quorum-execution-store-v1';
const MAX_TIMELINE_ENTRIES = 500;
const ACTIVE_AGENT_STATUSES = new Set(['started', 'thinking', 'tool_use', 'progress']);

export function buildWorkflowKey(projectId, workflowId) {
  const proj = projectId || 'default';
  return `${proj}:${workflowId}`;
}

function toIsoTimestamp(ts) {
  if (!ts) return null;
  if (typeof ts === 'string') return ts;
  try {
    return new Date(ts).toISOString();
  } catch {
    return null;
  }
}

function safeStr(v) {
  if (v == null) return '';
  return String(v);
}

// Fast, deterministic hash for stable ids (not crypto).
function hashStringFNV1a(str) {
  let h = 0x811c9dc5;
  for (let i = 0; i < str.length; i += 1) {
    h ^= str.charCodeAt(i);
    // 32-bit FNV_prime = 16777619
    h = (h + (h << 1) + (h << 4) + (h << 7) + (h << 8) + (h << 24)) >>> 0;
  }
  return h.toString(16).padStart(8, '0');
}

function makeSyntheticId(entry) {
  const base = [
    entry.kind,
    entry.event,
    entry.agent || '',
    entry.phase || '',
    entry.taskId || '',
    entry.ts || '',
    entry.title || '',
    entry.message || '',
  ].join('|');
  return `syn_${hashStringFNV1a(base)}`;
}

function normalizeTimelineEntry(raw) {
  if (!raw) return null;
  const entry = {
    id: raw.id || null,
    ts: toIsoTimestamp(raw.ts || raw.timestamp) || null,
    kind: raw.kind || null,
    event: raw.event || null,
    title: raw.title || '',
    message: raw.message || '',
    agent: raw.agent || null,
    phase: raw.phase || null,
    taskId: raw.taskId || null,
    data: raw.data || undefined,
    executionId: raw.executionId ?? null,
  };
  if (!entry.ts || !entry.kind || !entry.event) return null;
  if (!entry.id) entry.id = makeSyntheticId(entry);
  return entry;
}

function mergeTimeline(existing, incoming) {
  if (!incoming || incoming.length === 0) return existing || [];
  const byId = new Map();

  for (const e of existing || []) {
    if (e?.id) byId.set(e.id, e);
  }
  for (const raw of incoming) {
    const e = normalizeTimelineEntry(raw);
    if (!e) continue;
    const prev = byId.get(e.id);
    byId.set(e.id, prev ? { ...prev, ...e } : e);
  }

  const merged = Array.from(byId.values());
  merged.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime());
  return merged.slice(0, MAX_TIMELINE_ENTRIES);
}

function updateAgentStatusFromEvent(existing, eventKind, payload) {
  if (eventKind === 'chunk') return existing || {};
  const ts = toIsoTimestamp(payload?.timestamp) || toIsoTimestamp(payload?.ts) || null;
  const data = payload?.data;

  const isStartEvent = eventKind === 'started';
  const isEndEvent = eventKind === 'completed' || eventKind === 'error';

  let startedAt = existing?.startedAt || null;
  if (isStartEvent) startedAt = ts;
  if (!startedAt && data?.started_at) startedAt = toIsoTimestamp(data.started_at);
  if (!startedAt) startedAt = ts;

  const completedAt = isEndEvent ? ts : (existing?.completedAt || null);

  let durationMs = existing?.durationMs ?? null;
  if (isEndEvent) {
    if (data?.duration_ms != null) durationMs = data.duration_ms;
    else if (startedAt && ts) {
      const d = new Date(ts).getTime() - new Date(startedAt).getTime();
      if (!Number.isNaN(d) && d >= 0) durationMs = d;
    }
  }

  return {
    status: eventKind,
    message: payload?.message,
    data,
    timestamp: ts,
    startedAt,
    completedAt,
    durationMs,
  };
}

function rebuildAgentStatusesFromTimeline(timeline) {
  const agents = {};
  const chronological = [...(timeline || [])]
    .filter((e) => e.kind === 'agent' && e.agent)
    .sort((a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime());

  for (const e of chronological) {
    const name = e.agent;
    const existing = agents[name] || {};
    agents[name] = updateAgentStatusFromEvent(existing, e.event, {
      timestamp: e.ts,
      message: e.message,
      data: e.data,
      ts: e.ts,
    });
  }

  return agents;
}

function entryFromSSE(eventType, data, executionId) {
  const ts = toIsoTimestamp(data?.timestamp) || null;
  if (!ts) return null;

  // Skip noisy events by default.
  if (eventType === 'workflow_state_updated') return null;
  if (eventType === 'task_progress') return null;

  if (eventType === 'config_loaded') {
    const scope = safeStr(data?.config_scope) || 'unknown';
    const mode = safeStr(data?.config_mode) || 'unknown';
    const path = safeStr(data?.config_path);
    const snapshot = safeStr(data?.snapshot_path);
    const usedExecId = data?.execution_id ?? executionId;

    const parts = [];
    if (path) parts.push(path);
    if (snapshot) parts.push(`snapshot: ${snapshot}`);
    if (data?.execution_id != null) parts.push(`exec ${safeStr(data.execution_id)}`);

    return {
      kind: 'workflow',
      event: eventType,
      title: `Config loaded · ${scope}/${mode}`,
      message: parts.join(' · '),
      ts,
      data,
      executionId: usedExecId,
    };
  }

  if (eventType === 'agent_event') {
    const eventKind = safeStr(data?.event_kind) || 'progress';
    if (eventKind === 'chunk') return null;
    const agent = safeStr(data?.agent) || 'agent';
    const message = safeStr(data?.message);
    return {
      kind: 'agent',
      event: eventKind,
      agent,
      title: `${agent} · ${eventKind}`,
      message,
      ts,
      data: data?.data,
      executionId,
    };
  }

  if (eventType === 'phase_started' || eventType === 'phase_completed') {
    const phase = safeStr(data?.phase) || 'unknown';
    const title =
      eventType === 'phase_started'
        ? `Phase started · ${phase}`
        : `Phase completed · ${phase}`;
    const message =
      eventType === 'phase_completed' && data?.duration
        ? `Duration: ${safeStr(data.duration)}`
        : '';
    return {
      kind: 'phase',
      event: eventType,
      phase,
      title,
      message,
      ts,
      data,
      executionId,
    };
  }

  if (eventType === 'log') {
    const level = safeStr(data?.level) || 'info';
    const message = safeStr(data?.message);
    if (!message) return null;
    return {
      kind: 'log',
      event: eventType,
      title: `Log [${level}]`,
      message,
      ts,
      data,
      executionId,
    };
  }

  if (eventType.startsWith('task_')) {
    const taskId = safeStr(data?.task_id);
    if (!taskId) return null;

    const name = safeStr(data?.name);
    const baseTitle = name ? `${name}` : taskId;
    let title = `${eventType.replace('task_', 'Task ')} · ${baseTitle}`;
    title = title.replace('Task created', 'Task created');
    title = title.replace('Task started', 'Task started');
    title = title.replace('Task completed', 'Task completed');
    title = title.replace('Task failed', 'Task failed');
    title = title.replace('Task skipped', 'Task skipped');
    title = title.replace('Task retry', 'Task retry');

    let message = '';
    if (eventType === 'task_failed') message = safeStr(data?.error);
    if (eventType === 'task_skipped') message = safeStr(data?.reason);
    if (eventType === 'task_completed') message = data?.duration ? `Duration: ${safeStr(data.duration)}` : '';
    if (eventType === 'task_retry') message = data?.error ? safeStr(data.error) : '';

    return {
      kind: 'task',
      event: eventType,
      taskId,
      title,
      message,
      ts,
      data,
      executionId,
    };
  }

  if (eventType.startsWith('workflow_')) {
    const title = eventType.replace('workflow_', 'Workflow ').replace(/_/g, ' ');
    let message = '';
    if (eventType === 'workflow_failed') message = safeStr(data?.error);
    if (eventType === 'workflow_completed') message = data?.duration ? `Duration: ${safeStr(data.duration)}` : '';
    if (eventType === 'workflow_paused') message = safeStr(data?.reason);
    return {
      kind: 'workflow',
      event: eventType,
      title,
      message,
      ts,
      data,
      executionId,
    };
  }

  // Unknown event type: keep a minimal trace if it has a timestamp.
  return {
    kind: 'workflow',
    event: eventType,
    title: eventType,
    message: '',
    ts,
    data,
    executionId,
  };
}

const useExecutionStore = create(
  persist(
    (set, get) => ({
      timelineByWorkflow: {},
      currentAgentsByWorkflow: {},
      metaByWorkflow: {}, // workflowKey -> { executionId }

      clearWorkflow: (workflowKey) => {
        const { timelineByWorkflow, currentAgentsByWorkflow, metaByWorkflow } = get();
        const nextTimeline = { ...timelineByWorkflow };
        const nextAgents = { ...currentAgentsByWorkflow };
        const nextMeta = { ...metaByWorkflow };
        delete nextTimeline[workflowKey];
        delete nextAgents[workflowKey];
        delete nextMeta[workflowKey];
        set({
          timelineByWorkflow: nextTimeline,
          currentAgentsByWorkflow: nextAgents,
          metaByWorkflow: nextMeta,
        });
      },

      ingestSSEEvent: (eventType, data, projectId) => {
        const workflowId = data?.workflow_id;
        if (!workflowId) return;
        const workflowKey = buildWorkflowKey(projectId, workflowId);

        const meta = get().metaByWorkflow[workflowKey] || {};
        const executionId = meta.executionId ?? null;

        const entry = entryFromSSE(eventType, data, executionId);
        if (!entry) return;

        const { timelineByWorkflow, currentAgentsByWorkflow } = get();
        const existingTimeline = timelineByWorkflow[workflowKey] || [];
        const nextTimeline = mergeTimeline(existingTimeline, [entry]);

        let nextAgentsForWorkflow = currentAgentsByWorkflow[workflowKey] || {};
        if (entry.kind === 'agent' && entry.agent) {
          const prev = nextAgentsForWorkflow[entry.agent] || {};
          const updated = updateAgentStatusFromEvent(prev, entry.event, {
            timestamp: entry.ts,
            message: entry.message,
            data: entry.data,
            ts: entry.ts,
          });
          nextAgentsForWorkflow = { ...nextAgentsForWorkflow, [entry.agent]: updated };
        }

        set({
          timelineByWorkflow: { ...timelineByWorkflow, [workflowKey]: nextTimeline },
          currentAgentsByWorkflow: { ...currentAgentsByWorkflow, [workflowKey]: nextAgentsForWorkflow },
        });
      },

      hydrateFromWorkflowResponse: (workflow, projectId) => {
        if (!workflow?.id) return;
        const workflowKey = buildWorkflowKey(projectId, workflow.id);
        const incomingExecutionId = workflow.execution_id ?? null;

        const { metaByWorkflow, timelineByWorkflow, currentAgentsByWorkflow } = get();
        const meta = metaByWorkflow[workflowKey] || {};
        const prevExecutionId = meta.executionId ?? null;

        let nextTimeline = timelineByWorkflow[workflowKey] || [];
        let nextAgentsForWorkflow = currentAgentsByWorkflow[workflowKey] || {};

        // New execution: clear local timeline/state (no historic required).
        if (incomingExecutionId != null && prevExecutionId != null && incomingExecutionId !== prevExecutionId) {
          nextTimeline = [];
          nextAgentsForWorkflow = {};
        }

        const persisted = Array.isArray(workflow.agent_events) ? workflow.agent_events : [];
        if (persisted.length > 0) {
          // NOTE: we intentionally omit the backend-assigned `id` so that
          // normalizeTimelineEntry computes a synthetic ID from the entry
          // content.  This ensures hydrated events merge correctly with
          // events that arrived earlier via the real-time SSE path (which
          // also use synthetic IDs).
          const persistedEntries = persisted.map((e) => ({
            kind: 'agent',
            event: e.event_kind,
            agent: e.agent,
            title: `${safeStr(e.agent)} · ${safeStr(e.event_kind)}`,
            message: safeStr(e.message),
            ts: toIsoTimestamp(e.timestamp),
            data: e.data,
            executionId: e.execution_id ?? incomingExecutionId ?? null,
          }));
          nextTimeline = mergeTimeline(nextTimeline, persistedEntries);
          nextAgentsForWorkflow = rebuildAgentStatusesFromTimeline(nextTimeline);
        }

        set({
          metaByWorkflow: {
            ...metaByWorkflow,
            [workflowKey]: { executionId: incomingExecutionId ?? prevExecutionId ?? null },
          },
          timelineByWorkflow: { ...timelineByWorkflow, [workflowKey]: nextTimeline },
          currentAgentsByWorkflow: { ...currentAgentsByWorkflow, [workflowKey]: nextAgentsForWorkflow },
        });
      },

      getActiveAgents: (workflowKey) => {
        const agents = get().currentAgentsByWorkflow[workflowKey] || {};
        return Object.entries(agents)
          .filter(([, info]) => ACTIVE_AGENT_STATUSES.has(info.status))
          .map(([name, info]) => ({ name, ...info }));
      },
    }),
    {
      name: STORE_NAME,
      partialize: (state) => ({
        timelineByWorkflow: state.timelineByWorkflow,
        currentAgentsByWorkflow: state.currentAgentsByWorkflow,
        metaByWorkflow: state.metaByWorkflow,
      }),
    }
  )
);

export default useExecutionStore;
