import { useMemo } from 'react';
import {
  Activity,
  Bot,
  ChevronDown,
  ChevronUp,
  Loader2,
  CheckCircle2,
  XCircle,
  Wrench,
  Brain,
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

function AgentBadge({ name, status, message }) {
  const color = getAgentColor(name);
  const isActive = ['started', 'thinking', 'tool_use', 'progress'].includes(status);

  return (
    <div
      className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium border ${color.bg} ${color.text} ${color.border}`}
    >
      <Bot className="w-3 h-3" />
      <span className="truncate max-w-[120px]">{name}</span>
      {isActive && <Loader2 className="w-3 h-3 animate-spin" />}
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

export default function AgentActivity({ workflowId, activity = [], activeAgents = [], expanded, onToggle }) {
  const hasActivity = activity.length > 0 || activeAgents.length > 0;

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
          {activeAgents.length > 0 && (
            <div className="p-4 border-b border-border bg-accent/20">
              <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide mb-2">
                Active Agents
              </p>
              <div className="flex flex-wrap gap-2">
                {activeAgents.map((agent) => (
                  <AgentBadge
                    key={agent.name}
                    name={agent.name}
                    status={agent.status}
                    message={agent.message}
                  />
                ))}
              </div>
            </div>
          )}

          <div className="max-h-64 overflow-y-auto">
            {activity.length > 0 ? (
              <div className="p-2 space-y-0.5">
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
