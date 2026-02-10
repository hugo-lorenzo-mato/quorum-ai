import { useEffect, useMemo, useState } from 'react';
import { systemPromptsApi } from '../lib/api';
import { Badge } from '../components/ui/Badge';
import { Search, FileCode2, RefreshCw, X } from 'lucide-react';

const PHASES = ['All', 'refine', 'analyze', 'plan', 'execute'];
const USED_BY = ['All', 'workflow', 'issues'];
const STATUSES = ['All', 'active', 'reserved', 'deprecated'];

function statusVariant(status) {
  switch (status) {
    case 'active':
      return 'success';
    case 'reserved':
      return 'warning';
    case 'deprecated':
      return 'destructive';
    default:
      return 'secondary';
  }
}

function SystemPromptModal({ prompt, onClose }) {
  if (!prompt) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in">
      <button
        type="button"
        className="absolute inset-0 bg-background/80 backdrop-blur-sm"
        onClick={onClose}
        aria-label="Close system prompt"
      />
      <div
        className="relative bg-card border border-border shadow-2xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col rounded-2xl animate-fade-up"
      >
        <div className="relative flex items-start justify-between p-6 border-b border-border/50 bg-muted/5">
          <div className="absolute top-0 left-6 right-6 h-0.5 rounded-full bg-gradient-to-r from-transparent via-primary to-transparent" />
          <div className="flex items-start gap-4 min-w-0">
            <div className="p-3 rounded-2xl bg-primary/10 border border-primary/20 text-primary flex-shrink-0">
              <FileCode2 className="w-7 h-7" />
            </div>
            <div className="min-w-0">
              <h2 className="text-xl font-bold text-foreground tracking-tight truncate">{prompt.title}</h2>
              <div className="flex flex-wrap items-center gap-2 mt-2">
                <Badge variant="outline" className="text-[10px] h-5 font-mono">{prompt.id}</Badge>
                <Badge variant="secondary" className="text-[10px] h-5 uppercase tracking-wider">{prompt.workflow_phase}</Badge>
                <Badge variant={statusVariant(prompt.status)} className="text-[10px] h-5 uppercase tracking-wider">{prompt.status}</Badge>
                {Array.isArray(prompt.used_by) && prompt.used_by.map((v) => (
                  <Badge key={v} variant="outline" className="text-[10px] h-5">{v}</Badge>
                ))}
              </div>
              <p className="text-xs text-muted-foreground mt-2 font-mono">
                step={prompt.step} · sha256={prompt.sha256}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-all flex-shrink-0"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6">
          <h3 className="text-xs font-bold uppercase tracking-widest text-muted-foreground mb-3">Content</h3>
          <div className="relative group">
            <div className="absolute -inset-0.5 bg-gradient-to-r from-primary/10 to-transparent rounded-xl blur opacity-20 group-hover:opacity-40 transition-opacity" />
            <pre className="relative bg-muted/30 backdrop-blur-sm rounded-xl p-5 text-sm font-mono whitespace-pre-wrap text-foreground/85 overflow-x-auto border border-border/50 shadow-inner leading-relaxed">
              {prompt.content}
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function SystemPrompts() {
  const [prompts, setPrompts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [phase, setPhase] = useState('All');
  const [usedBy, setUsedBy] = useState('All');
  const [status, setStatus] = useState('All');
  const [searchQuery, setSearchQuery] = useState('');

  const [selected, setSelected] = useState(null);
  const [selectedLoading, setSelectedLoading] = useState(false);
  const [selectedError, setSelectedError] = useState('');
  const [cache, setCache] = useState(() => new Map());

  const loadList = async () => {
    setLoading(true);
    setError('');
    try {
      const data = await systemPromptsApi.list();
      setPrompts(Array.isArray(data) ? data : []);
    } catch (e) {
      setError(e?.message || 'Failed to load system prompts');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadList();
  }, []);

  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    return (prompts || []).filter((p) => {
      const matchesPhase = phase === 'All' || p.workflow_phase === phase;
      const matchesUsedBy = usedBy === 'All' || (Array.isArray(p.used_by) && p.used_by.includes(usedBy));
      const matchesStatus = status === 'All' || p.status === status;
      const matchesSearch =
        q === '' ||
        (p.id || '').toLowerCase().includes(q) ||
        (p.title || '').toLowerCase().includes(q) ||
        (p.step || '').toLowerCase().includes(q);
      return matchesPhase && matchesUsedBy && matchesStatus && matchesSearch;
    });
  }, [prompts, phase, usedBy, status, searchQuery]);

  const openPrompt = async (id) => {
    setSelectedError('');
    if (!id) return;

    if (cache.has(id)) {
      setSelected(cache.get(id));
      return;
    }

    setSelectedLoading(true);
    try {
      const full = await systemPromptsApi.get(id);
      setCache((prev) => {
        const next = new Map(prev);
        next.set(id, full);
        return next;
      });
      setSelected(full);
    } catch (e) {
      setSelectedError(e?.message || 'Failed to load system prompt');
    } finally {
      setSelectedLoading(false);
    }
  };

  return (
    <div className="space-y-6 animate-fade-in pb-10">
      <div className="px-4 sm:px-6 space-y-8">
        <div className="flex flex-col gap-6">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 className="text-2xl font-semibold text-foreground tracking-tight">System Prompts</h1>
              <p className="text-sm text-muted-foreground mt-1">
                Embedded prompts used by the workflow engine and issue generation. Read-only.
              </p>
            </div>

            <div className="flex flex-col sm:flex-row gap-3 w-full lg:w-auto">
              <button
                type="button"
                onClick={loadList}
                className="h-9 px-3 rounded-md border border-border bg-background text-sm font-medium text-foreground hover:bg-accent transition-all flex items-center justify-center gap-2"
                title="Reload"
              >
                <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                Reload
              </button>

              <div className="relative w-full sm:w-72">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search by id, title, step..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="h-9 w-full pl-9 pr-4 rounded-md border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/10 hover:border-border/80 transition-all"
                />
              </div>
            </div>
          </div>

          {/* Filters */}
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-1 p-1 rounded-lg bg-muted/30 overflow-x-auto no-scrollbar max-w-full border border-border/50">
              {PHASES.map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setPhase(p)}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium whitespace-nowrap transition-all duration-200 ${
                    phase === p
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
                  }`}
                >
                  {p === 'All' ? 'All Phases' : p}
                  <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${
                    phase === p ? 'bg-muted text-foreground' : 'bg-muted/50 text-muted-foreground'
                  }`}>
                    {(p === 'All'
                      ? prompts.length
                      : prompts.filter((x) => x.workflow_phase === p).length) || 0}
                  </span>
                </button>
              ))}
            </div>

            <div className="flex flex-col sm:flex-row gap-3">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-muted-foreground">Used by</span>
                <select
                  value={usedBy}
                  onChange={(e) => setUsedBy(e.target.value)}
                  className="h-9 px-3 rounded-md border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/10"
                >
                  {USED_BY.map((v) => (
                    <option key={v} value={v}>{v}</option>
                  ))}
                </select>
              </div>

              <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-muted-foreground">Status</span>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value)}
                  className="h-9 px-3 rounded-md border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/10"
                >
                  {STATUSES.map((v) => (
                    <option key={v} value={v}>{v}</option>
                  ))}
                </select>
              </div>

              <div className="flex items-center gap-2 text-xs text-muted-foreground sm:ml-auto">
                <span className="text-foreground">{filtered.length}</span> shown
              </div>
            </div>
          </div>

          {error && (
            <div className="p-4 rounded-xl border border-status-error/30 bg-status-error-bg/10 text-status-error text-sm">
              {error}
            </div>
          )}

          {/* Grid */}
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {loading ? (
              [...Array(9)].map((_, i) => (
                <div key={i} className="h-40 rounded-xl border border-border bg-muted/20 animate-pulse" />
              ))
            ) : (
              filtered.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => openPrompt(p.id)}
                  className="group text-left flex flex-col rounded-xl border border-border bg-card transition-all duration-200 overflow-hidden shadow-sm hover:shadow-md hover:border-foreground/30"
                >
                  <div className="p-5 space-y-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <h3 className="text-sm font-bold text-foreground tracking-tight truncate">{p.title}</h3>
                        <p className="text-xs text-muted-foreground font-mono mt-1 truncate">{p.id}</p>
                      </div>
                      <div className="p-2 rounded-lg bg-primary/10 border border-primary/20 text-primary flex-shrink-0">
                        <FileCode2 className="w-4 h-4" />
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-2">
                      <Badge variant="secondary" className="text-[10px] h-5 uppercase tracking-wider">{p.workflow_phase}</Badge>
                      <Badge variant={statusVariant(p.status)} className="text-[10px] h-5 uppercase tracking-wider">{p.status}</Badge>
                      {Array.isArray(p.used_by) && p.used_by.map((v) => (
                        <Badge key={v} variant="outline" className="text-[10px] h-5">{v}</Badge>
                      ))}
                    </div>

                    <div className="text-xs text-muted-foreground font-mono">
                      step={p.step}
                    </div>
                  </div>
                  <div className="px-5 pb-5">
                    <div className="text-[10px] text-muted-foreground/80 font-mono truncate">
                      sha256={p.sha256}
                    </div>
                  </div>
                </button>
              ))
            )}
          </div>

          {selectedError && (
            <div className="p-4 rounded-xl border border-status-error/30 bg-status-error-bg/10 text-status-error text-sm">
              {selectedError}
            </div>
          )}

          {selectedLoading && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <RefreshCw className="w-4 h-4 animate-spin" />
              Loading prompt...
            </div>
          )}
        </div>
      </div>

      <SystemPromptModal prompt={selected} onClose={() => setSelected(null)} />
    </div>
  );
}
