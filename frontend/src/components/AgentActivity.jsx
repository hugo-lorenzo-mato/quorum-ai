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

// Format token counts compactly: 1500 → "1.5K", 1200000 → "1.2M"
function formatTokens(count) {
  if (count == null) return '';
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

// Summarize tool args: prioritize common keys, truncate to maxLen
function _summarizeToolArgs(args) {
  if (!args || typeof args !== 'object') return '';
  const priorityKeys = ['command', 'path', 'file_path', 'pattern', 'query', 'url'];
  for (const key of priorityKeys) {
    if (args[key]) {
      const val = String(args[key]);
      return val.length > 60 ? val.slice(0, 57) + '...' : val;
    }
  }
  // Fallback: first value
  const firstKey = Object.keys(args)[0];
  if (firstKey) {
    const val = String(args[firstKey]);
    return val.length > 60 ? val.slice(0, 57) + '...' : val;
  }
  return '';
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
  // Only use durationMs if agent is actually done (completed or error)
  const shouldUseDuration = (isDone || isError) && agent.durationMs != null;
  const elapsed = startTime ? formatElapsed(startTime, shouldUseDuration ? agent.durationMs : null) : '';

  const data = agent.data || {};

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
        {data.phase && isActive && (
          <span className="text-muted-foreground text-xs">[{data.phase}]</span>
        )}
        <span className="text-muted-foreground truncate">
          {agent.message || (isDone ? 'done' : isActive ? 'processing...' : isError ? 'failed' : 'idle')}
        </span>
        {/* Show tool name for tool_use status */}
        {agent.status === 'tool_use' && data.tool && (
          <span className="font-mono text-foreground/70 truncate" data-testid="progress-tool">
            {data.tool}
          </span>
        )}
        {/* Show command for Codex-style data */}
        {data.command && agent.status === 'tool_use' && (
          <span className="font-mono text-muted-foreground truncate max-w-[200px]" data-testid="progress-command">
            {data.command.length > 50 ? data.command.slice(0, 47) + '...' : data.command}
          </span>
        )}
      </div>

      {/* Token usage for completed agents */}
      {isDone && data.tokens_in != null && (
        <span className="text-muted-foreground font-mono whitespace-nowrap" data-testid="progress-tokens">
          {formatTokens(data.tokens_in)}in/{formatTokens(data.tokens_out)}out
        </span>
      )}

      {/* Elapsed time */}
      {elapsed && (
        <span className="text-muted-foreground w-16 text-right">{elapsed}</span>
      )}
    </div>
  );
}

function ActivityEntry({ entry }) {
  const [expanded, setExpanded] = useState(false);
  const color = getAgentColor(entry.agent);
  const Icon = EVENT_ICONS[entry.eventKind] || Activity;
  const isActive = ['started', 'thinking', 'tool_use', 'progress'].includes(entry.eventKind);

  const data = entry.data || {};
  const hasDetail = !!(
    data.tool || data.args || data.command || data.thinking_text ||
    data.reasoning_text || data.exit_code != null || data.tokens_in != null ||
    data.result || data.text
  );

  const timeStr = useMemo(() => {
    if (!entry.timestamp) return '';
    const date = new Date(entry.timestamp);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }, [entry.timestamp]);

  return (
    <button
      type="button"
      className={`w-full text-left rounded-lg hover:bg-accent/30 transition-colors animate-fade-in ${hasDetail ? 'cursor-pointer' : 'cursor-default'}`}
      onClick={hasDetail ? () => setExpanded(e => !e) : undefined}
      disabled={!hasDetail}
      aria-expanded={hasDetail ? expanded : undefined}
      data-testid="activity-entry"
    >
      <div className="flex items-start gap-3 py-2 px-3">
        <div className={`p-1.5 rounded-lg ${color.bg} mt-0.5`}>
          <Icon className={`w-3.5 h-3.5 ${color.text} ${isActive && entry.eventKind !== 'progress' ? 'animate-pulse' : ''}`} />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className={`text-xs font-medium ${color.text}`}>{entry.agent}</span>
            {data.phase && (
              <span className="text-xs text-muted-foreground">[{data.phase}]</span>
            )}
            <span className="text-xs text-muted-foreground">{entry.eventKind}</span>
          </div>
          {entry.message && (
            <p className="text-xs text-muted-foreground truncate mt-0.5">{entry.message}</p>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-xs text-muted-foreground whitespace-nowrap">{timeStr}</span>
          {hasDetail && (
            <ChevronDown
              className={`w-3 h-3 text-muted-foreground transition-transform ${expanded ? 'rotate-180' : ''}`}
              data-testid="expand-chevron"
            />
          )}
        </div>
      </div>

      {/* Expanded detail panel */}
      {expanded && hasDetail && (
        <div className="ml-9 pb-2 px-3 space-y-1 animate-fade-in" data-testid="entry-detail">
          {data.tool && (
            <div className="flex items-center gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0">Tool</span>
              <span className="font-mono text-foreground">{data.tool}</span>
            </div>
          )}
          {data.args && (
            <div className="flex items-start gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0 mt-0.5">Args</span>
              <pre className="font-mono text-foreground/80 bg-accent/20 rounded px-2 py-1 max-h-32 overflow-auto text-[11px] whitespace-pre-wrap break-all">
                {typeof data.args === 'string' ? data.args : JSON.stringify(data.args, null, 2)}
              </pre>
            </div>
          )}
          {data.command && (
            <div className="flex items-center gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0">Cmd</span>
              <code className="font-mono text-foreground/80">{data.command}</code>
            </div>
          )}
          {(data.thinking_text || data.reasoning_text) && (
            <div className="flex items-start gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0 mt-0.5">Think</span>
              <span className="italic text-muted-foreground line-clamp-3">
                {data.thinking_text || data.reasoning_text}
              </span>
            </div>
          )}
          {data.result && (
            <div className="flex items-start gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0 mt-0.5">Result</span>
              <pre className="font-mono text-foreground/80 bg-accent/20 rounded px-2 py-1 max-h-32 overflow-auto text-[11px] whitespace-pre-wrap break-all">
                {data.result}
              </pre>
            </div>
          )}
          {data.text && (
            <div className="flex items-start gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0 mt-0.5">Text</span>
              <span className="text-muted-foreground line-clamp-3">{data.text}</span>
            </div>
          )}
          {data.exit_code != null && (
            <div className="flex items-center gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0">Exit</span>
              <span
                className={`font-mono px-1.5 py-0.5 rounded text-[11px] ${
                  data.exit_code === 0
                    ? 'bg-success/10 text-success'
                    : 'bg-error/10 text-error'
                }`}
                data-testid="exit-code-badge"
              >
                {data.exit_code}
              </span>
            </div>
          )}
          {data.tokens_in != null && (
            <div className="flex items-center gap-2 text-xs">
              <span className="w-14 text-muted-foreground shrink-0">Tokens</span>
              <span className="font-mono text-muted-foreground">
                {formatTokens(data.tokens_in)} in / {formatTokens(data.tokens_out)} out
              </span>
            </div>
          )}
        </div>
      )}
    </button>
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
