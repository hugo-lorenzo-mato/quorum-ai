import { useMemo, useState, useEffect } from 'react';
import {
  Activity,
  ChevronDown,
  ChevronUp,
  Loader2,
  CheckCircle2,
  XCircle,
  Wrench,
  Brain,
  Clock,
  Timer,
} from 'lucide-react';

const AGENT_COLORS = {
  claude: { bg: 'bg-orange-500/10', text: 'text-orange-500', border: 'border-orange-500/20' },
  gemini: { bg: 'bg-blue-500/10', text: 'text-blue-500', border: 'border-blue-500/20' },
  codex: { bg: 'bg-green-500/10', text: 'text-green-500', border: 'border-green-500/20' },
  copilot: { bg: 'bg-purple-500/10', text: 'text-purple-500', border: 'border-purple-500/20' },
  default: { bg: 'bg-primary/10', text: 'text-primary', border: 'border-primary/20' },
};

const EVENT_ICONS = {
  started: Loader2,
  thinking: Brain,
  tool_use: Wrench,
  progress: Activity,
  completed: CheckCircle2,
  error: XCircle,
};

function getAgentColor(agentName) {
  const name = (agentName || '').toLowerCase();
  for (const key of Object.keys(AGENT_COLORS)) {
    if (name.includes(key)) return AGENT_COLORS[key];
  }
  return AGENT_COLORS.default;
}

// Format elapsed time like TUI (e.g., "45s", "2m30s")
// If durationMs is provided (agent completed), use fixed duration instead of calculating from current time
function formatElapsed(startTime, durationMs = null) {
  // Use fixed duration for completed agents
  if (durationMs !== null && durationMs !== undefined) {
    const elapsed = Math.floor(durationMs / 1000);
    if (elapsed < 60) return `${elapsed}s`;
    const mins = Math.floor(elapsed / 60);
    const secs = elapsed % 60;
    return `${mins}m${secs.toString().padStart(2, '0')}s`;
  }

  // Calculate live elapsed time for active agents
  if (!startTime) return '';
  const elapsed = Math.floor((Date.now() - new Date(startTime).getTime()) / 1000);
  if (elapsed < 60) return `${elapsed}s`;
  const mins = Math.floor(elapsed / 60);
  const secs = elapsed % 60;
  return `${mins}m${secs.toString().padStart(2, '0')}s`;
}

// Estimate progress based on elapsed time (like TUI - assumes 2 min = 100%)
function estimateProgress(startTime, isDone) {
  if (isDone) return 100;
  if (!startTime) return 0;
  const elapsed = (Date.now() - new Date(startTime).getTime()) / 1000;
  const expectedDuration = 120; // 2 minutes
  const pct = Math.min(95, Math.floor((elapsed / expectedDuration) * 100));
  return pct;
}

// Get activity icon based on event kind
function getActivityIcon(eventKind) {
  switch (eventKind) {
    case 'tool_use': return '🔧';
    case 'thinking': return '💭';
    case 'progress': return '◐';
    case 'started': return '▶';
    case 'completed': return '✓';
    case 'error': return '✗';
    default: return '◐';
  }
}

// TUI-style progress bar for a single agent
function AgentProgressBar({ agent }) {
  const color = getAgentColor(agent.name);
  const isActive = ['started', 'thinking', 'tool_use', 'progress'].includes(agent.status);
  const isDone = agent.status === 'completed';
  const isError = agent.status === 'error';

  // Use startedAt for progress calculation, fall back to timestamp for backwards compatibility
  const startTime = agent.startedAt || agent.timestamp;
  const progress = estimateProgress(startTime, isDone);
  const filledBars = Math.floor(progress / 10);

  const activityIcon = getActivityIcon(agent.status);
  // For completed agents, use fixed durationMs; for active agents, calculate live elapsed
  const elapsed = startTime ? formatElapsed(startTime, agent.durationMs) : '';

  return (
    <div className="flex items-center gap-3 py-1.5 font-mono text-xs">
      {/* Agent name */}
      <span className={`w-16 truncate font-semibold ${color.text}`}>
        {agent.name}
      </span>

      {/* Progress bar */}
      <div className="flex items-center gap-0.5">
        <span className="text-muted-foreground">[</span>
        {[...Array(10)].map((_, i) => (
          <span
            key={i}
            className={
              i < filledBars
                ? isDone
                  ? 'text-success'
                  : isError
                  ? 'text-error'
                  : color.text
                : 'text-muted-foreground/30'
            }
          >
            {i < filledBars ? '▓' : '░'}
          </span>
        ))}
        <span className="text-muted-foreground">]</span>
      </div>

      {/* Activity icon and message */}
      <div className="flex-1 flex items-center gap-1.5 min-w-0">
        <span className={isActive ? 'text-warning' : isDone ? 'text-success' : isError ? 'text-error' : 'text-muted-foreground'}>
          {activityIcon}
        </span>
        {/* Show phase/role if available (e.g., "[moderator] thinking...") */}
        {agent.data?.phase && isActive && (
          <span className="text-muted-foreground text-xs">[{agent.data.phase}]</span>
        )}
        <span className="text-muted-foreground truncate">
          {agent.message || (isDone ? 'done' : isActive ? 'processing...' : isError ? 'failed' : 'idle')}
        </span>
      </div>

      {/* Elapsed time */}
      {elapsed && (
        <span className="text-muted-foreground w-16 text-right">{elapsed}</span>
      )}
    </div>
  );
}

function ActivityEntry({ entry }) {
  const color = getAgentColor(entry.agent);
  const Icon = EVENT_ICONS[entry.eventKind] || Activity;
  const isActive = ['started', 'thinking', 'tool_use', 'progress'].includes(entry.eventKind);

  const timeStr = useMemo(() => {
    if (!entry.timestamp) return '';
    const date = new Date(entry.timestamp);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }, [entry.timestamp]);

  return (
    <div className="flex items-start gap-3 py-2 px-3 rounded-lg hover:bg-accent/30 transition-colors animate-fade-in">
      <div className={`p-1.5 rounded-lg ${color.bg} mt-0.5`}>
        <Icon className={`w-3.5 h-3.5 ${color.text} ${isActive && entry.eventKind !== 'progress' ? 'animate-pulse' : ''}`} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className={`text-xs font-medium ${color.text}`}>{entry.agent}</span>
          {entry.data?.phase && (
            <span className="text-xs text-muted-foreground">[{entry.data.phase}]</span>
          )}
          <span className="text-xs text-muted-foreground">{entry.eventKind}</span>
        </div>
        {entry.message && (
          <p className="text-xs text-muted-foreground truncate mt-0.5">{entry.message}</p>
        )}
      </div>
      <span className="text-xs text-muted-foreground whitespace-nowrap">{timeStr}</span>
    </div>
  );
}

export default function AgentActivity({ activity = [], activeAgents = [], expanded, onToggle, workflowStartTime }) {
  const hasActivity = activity.length > 0 || activeAgents.length > 0;

  // Timer tick for updating elapsed times
  const [, setTick] = useState(0);

  useEffect(() => {
    if (!hasActivity) return;
    const interval = setInterval(() => setTick(t => t + 1), 1000);
    return () => clearInterval(interval);
  }, [hasActivity]);

  // Calculate total elapsed time
  const totalElapsed = workflowStartTime ? formatElapsed(workflowStartTime) : null;

  // Build agent progress data from activity with proper timestamp handling
  const agentProgress = useMemo(() => {
    const agents = new Map();

    // Process activity in reverse (oldest first) to build up state
    [...activity].reverse().forEach(entry => {
      const existing = agents.get(entry.agent) || {
        name: entry.agent,
        status: 'idle',
        message: '',
        timestamp: null,
        startedAt: null,
        completedAt: null,
        durationMs: null,
      };

      const eventKind = entry.eventKind;
      const isStartEvent = eventKind === 'started';
      const isEndEvent = eventKind === 'completed' || eventKind === 'error';

      // Update with latest event
      existing.status = eventKind;
      existing.message = entry.message;
      existing.data = entry.data;

      // Track startedAt from first 'started' event
      if (isStartEvent) {
        existing.startedAt = entry.timestamp;
      } else if (!existing.startedAt) {
        existing.startedAt = entry.timestamp;
      }

      // Track completedAt and calculate duration when completed
      if (isEndEvent) {
        existing.completedAt = entry.timestamp;
        if (existing.startedAt && entry.timestamp) {
          existing.durationMs = new Date(entry.timestamp).getTime() - new Date(existing.startedAt).getTime();
        }
      }

      // Keep timestamp for backwards compatibility
      if (!existing.timestamp) {
        existing.timestamp = entry.timestamp;
      }

      agents.set(entry.agent, existing);
    });

    // Also add any active agents not in activity (these come from the store with proper timestamps)
    activeAgents.forEach(agent => {
      const existing = agents.get(agent.name) || {
        name: agent.name,
        status: agent.status,
        message: agent.message,
        timestamp: agent.timestamp,
        startedAt: agent.startedAt,
        completedAt: agent.completedAt,
        durationMs: agent.durationMs,
      };
      existing.status = agent.status;
      existing.message = agent.message;
      existing.data = agent.data;
      // Use store values which have proper timestamp tracking
      if (agent.startedAt) existing.startedAt = agent.startedAt;
      if (agent.completedAt) existing.completedAt = agent.completedAt;
      if (agent.durationMs) existing.durationMs = agent.durationMs;
      if (agent.timestamp) existing.timestamp = agent.timestamp;
      agents.set(agent.name, existing);
    });

    return Array.from(agents.values());
  }, [activity, activeAgents]);

  if (!hasActivity) return null;

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden animate-fade-up">
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between p-4 hover:bg-accent/30 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-info/10">
            <Activity className="w-4 h-4 text-info" />
          </div>
          <div className="text-left">
            <h3 className="text-sm font-semibold text-foreground">Agent Activity</h3>
            <p className="text-xs text-muted-foreground">
              {activeAgents.length > 0
                ? `${activeAgents.length} active agent${activeAgents.length > 1 ? 's' : ''}`
                : `${activity.length} events`}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {/* Total elapsed time */}
          {totalElapsed && (
            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Timer className="w-3.5 h-3.5" />
              <span className="font-mono">{totalElapsed}</span>
            </div>
          )}
          {activeAgents.length > 0 && (
            <div className="flex -space-x-1">
              {activeAgents.slice(0, 3).map((agent) => (
                <div
                  key={agent.name}
                  className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium border-2 border-card ${getAgentColor(agent.name).bg} ${getAgentColor(agent.name).text}`}
                >
                  {agent.name.charAt(0).toUpperCase()}
                </div>
              ))}
              {activeAgents.length > 3 && (
                <div className="w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium border-2 border-card bg-muted text-muted-foreground">
                  +{activeAgents.length - 3}
                </div>
              )}
            </div>
          )}
          {expanded ? (
            <ChevronUp className="w-4 h-4 text-muted-foreground" />
          ) : (
            <ChevronDown className="w-4 h-4 text-muted-foreground" />
          )}
        </div>
      </button>

      {expanded && (
        <div className="border-t border-border">
          {/* TUI-style progress bars section */}
          {agentProgress.length > 0 && (
            <div className="p-4 border-b border-border bg-accent/10">
              <div className="flex items-center justify-between mb-3">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                  Agent Progress
                </p>
                {totalElapsed && (
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Clock className="w-3 h-3" />
                    <span className="font-mono">Total: {totalElapsed}</span>
                  </div>
                )}
              </div>
              <div className="space-y-0.5 bg-background/50 rounded-lg p-3">
                {agentProgress.map((agent) => (
                  <AgentProgressBar key={agent.name} agent={agent} />
                ))}
              </div>
            </div>
          )}

          {/* Activity log */}
          <div className="max-h-64 overflow-y-auto">
            {activity.length > 0 ? (
              <div className="p-2 space-y-0.5">
                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide px-3 py-2">
                  Activity Log
                </p>
                {activity.slice(0, 50).map((entry) => (
                  <ActivityEntry key={entry.id} entry={entry} />
                ))}
              </div>
            ) : (
              <div className="p-6 text-center text-sm text-muted-foreground">
                No activity yet
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export function AgentActivityCompact({ activeAgents = [] }) {
  if (activeAgents.length === 0) return null;

  return (
    <div className="flex items-center gap-2">
      <div className="flex -space-x-1">
        {activeAgents.slice(0, 3).map((agent) => (
          <div
            key={agent.name}
            className={`w-5 h-5 rounded-full flex items-center justify-center text-[10px] font-medium border border-card ${getAgentColor(agent.name).bg} ${getAgentColor(agent.name).text}`}
            title={`${agent.name}: ${agent.status}`}
          >
            {agent.name.charAt(0).toUpperCase()}
          </div>
        ))}
      </div>
      <span className="text-xs text-muted-foreground">
        {activeAgents.length} active
      </span>
    </div>
  );
}
