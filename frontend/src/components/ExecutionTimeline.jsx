import { useMemo, useState } from 'react';
import {
  Activity,
  ListChecks,
  Workflow as WorkflowIcon,
  GitBranch,
  Loader2,
  RefreshCw,
  Filter,
  AlertTriangle,
} from 'lucide-react';

const FILTERS = [
  { id: 'phases_tasks', label: 'Phases + Tasks' },
  { id: 'agents', label: 'Agents' },
  { id: 'workflow', label: 'Workflow' },
  { id: 'all', label: 'All' },
];

function iconForEntry(entry) {
  switch (entry.kind) {
    case 'agent':
      return Activity;
    case 'phase':
      return GitBranch;
    case 'task':
      return ListChecks;
    case 'log':
      return AlertTriangle;
    case 'workflow':
    default:
      return WorkflowIcon;
  }
}

function timeStr(ts) {
  if (!ts) return '';
  try {
    const date = new Date(ts);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch {
    return '';
  }
}

function formatTokens(count) {
  if (count == null) return '';
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}K`;
  return String(count);
}

function shouldShowEntry(filterId, entry) {
  if (filterId === 'all') return true;
  if (filterId === 'agents') return entry.kind === 'agent';
  if (filterId === 'workflow') return entry.kind === 'workflow';
  // phases_tasks default
  // Include config provenance even in the default view (low-noise, high-value).
  if (entry.event === 'config_loaded') return true;
  if (entry.kind === 'log') return true;
  return entry.kind === 'phase' || entry.kind === 'task';
}

export default function ExecutionTimeline({
  entries = [],
  status,
  defaultFilter = 'phases_tasks',
  onRefresh,
  connectionMode,
}) {
  const [filterId, setFilterId] = useState(defaultFilter);

  const filtered = useMemo(
    () => (entries || []).filter((e) => shouldShowEntry(filterId, e)),
    [entries, filterId]
  );

  const showEmptyRunningHint = filtered.length === 0 && ['running', 'cancelling'].includes(String(status || '').toLowerCase());

  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden animate-fade-up">
      <div className="flex items-center justify-between p-4 border-b border-border/60">
        <div className="flex items-center gap-3 min-w-0">
          <div className="p-2 rounded-lg bg-info/10">
            <Activity className="w-4 h-4 text-info" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-semibold text-foreground">Execution Timeline</h3>
              {connectionMode && (
                <span className="text-[10px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground font-mono uppercase tracking-wide">
                  {connectionMode}
                </span>
              )}
            </div>
            <p className="text-xs text-muted-foreground">
              {filtered.length} event{filtered.length !== 1 ? 's' : ''} shown
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 shrink-0">
          {onRefresh && (
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-border bg-background text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
              title="Refresh workflow state"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              Refresh
            </button>
          )}
        </div>
      </div>

      <div className="flex items-center gap-1 p-2 border-b border-border/40 bg-accent/10 overflow-x-auto no-scrollbar">
        <div className="flex items-center gap-1 text-xs text-muted-foreground px-2">
          <Filter className="w-3.5 h-3.5" />
          <span className="whitespace-nowrap">Filter</span>
        </div>
        {FILTERS.map((f) => (
          <button
            key={f.id}
            type="button"
            onClick={() => setFilterId(f.id)}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap ${
              filterId === f.id
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
            }`}
          >
            {f.label}
          </button>
        ))}
      </div>

      <div className="max-h-64 overflow-y-auto">
        {showEmptyRunningHint ? (
          <div className="p-6 text-center text-sm text-muted-foreground">
            <div className="inline-flex items-center gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>Workflow is {String(status).toLowerCase()}. Waiting for events…</span>
            </div>
          </div>
        ) : filtered.length > 0 ? (
          <div className="p-2 space-y-0.5">
            {filtered.slice(0, 80).map((entry) => {
              const Icon = iconForEntry(entry);
              const ts = timeStr(entry.ts);
              return (
                <div
                  key={entry.id}
                  className="flex items-start gap-3 py-2 px-3 rounded-lg hover:bg-accent/30 transition-colors animate-fade-in"
                >
                  <div className="p-1.5 rounded-lg bg-muted/50 mt-0.5">
                    <Icon className="w-3.5 h-3.5 text-muted-foreground" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-xs font-medium text-foreground truncate">{entry.title || entry.event}</span>
                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground font-mono uppercase tracking-wide shrink-0">
                        {entry.kind}
                      </span>
                    </div>
                    {entry.message ? (
                      <p className="text-xs text-muted-foreground truncate mt-0.5">{entry.message}</p>
                    ) : null}
                    {entry.kind === 'agent' && entry.event === 'completed' && entry.data?.tokens_in != null && (
                      <span className="inline-block mt-0.5 font-mono text-[10px] text-muted-foreground bg-muted/50 px-1.5 py-0.5 rounded" data-testid="timeline-tokens">
                        {formatTokens(entry.data.tokens_in)}in/{formatTokens(entry.data.tokens_out)}out
                      </span>
                    )}
                  </div>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">{ts}</span>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="p-6 text-center text-sm text-muted-foreground">
            No events for this filter yet
          </div>
        )}
      </div>
    </div>
  );
}
