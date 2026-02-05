import { useEffect, useState, useMemo } from 'react';
import { useConfigStore } from '../stores/configStore';
import { Search, ArrowLeft, ChevronRight, X, Settings as SettingsIcon, GitBranch as GitIcon, Terminal as AdvancedIcon, Workflow as WorkflowIcon, Bot as AgentsIcon, ListOrdered as PhasesIcon, Ticket as IssuesIcon, Sliders, Info, RefreshCw, Loader2 } from 'lucide-react';
import {
  SettingsToolbar,
  ConflictDialog,
  GeneralTab,
  WorkflowTab,
  AgentsTab,
  PhasesTab,
  GitTab,
  IssuesTab,
  AdvancedTab,
} from '../components/config';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Badge } from '../components/ui/Badge';

const TABS = [
  { id: 'general', label: 'General', group: 'System', icon: SettingsIcon, component: GeneralTab, description: 'App behaviors and global preferences', keywords: ['log', 'logging', 'chat', 'report', 'markdown', 'output'] },
  { id: 'git', label: 'Git Integration', group: 'System', icon: GitIcon, component: GitTab, description: 'Repository management and automation', keywords: ['commit', 'push', 'pr', 'pull request', 'merge', 'github', 'branch'] },
  { id: 'advanced', label: 'Advanced', group: 'System', icon: AdvancedIcon, component: AdvancedTab, description: 'System diagnostics and tracing', keywords: ['trace', 'debug', 'server', 'port', 'host', 'reset', 'danger'] },
  { id: 'workflow', label: 'Workflow Defaults', group: 'Project', icon: WorkflowIcon, component: WorkflowTab, description: 'Execution parameters and safety', keywords: ['timeout', 'retry', 'dry run', 'sandbox', 'state', 'database', 'backend'] },
  { id: 'agents', label: 'Agents & Models', group: 'Project', icon: AgentsIcon, component: AgentsTab, description: 'Provider keys and model orchestration', keywords: ['model', 'temperature', 'provider', 'token', 'context'] },
  { id: 'phases', label: 'Execution Phases', group: 'Project', icon: PhasesIcon, component: PhasesTab, description: 'Orchestration step sequence', keywords: ['step', 'order', 'execution'] },
  { id: 'issues', label: 'Issue Generation', group: 'Project', icon: IssuesIcon, component: IssuesTab, description: 'Ticketing system export configuration', keywords: ['github', 'gitlab', 'issue', 'ticket', 'label', 'assignee', 'template'] },
];

const getTabDirty = (id, local) => {
  const keys = Object.keys(local);
  const map = { general: ['log', 'chat', 'report'], workflow: ['workflow', 'state'], agents: ['agents'], phases: ['phases'], git: ['git', 'github'], issues: ['issues'], advanced: ['trace', 'server'] };
  return keys.some(k => (map[id] || []).includes(k));
};

export default function Settings() {
  const [activeTab, setActiveTab] = useState('general');
  const [searchQuery, setSearchQuery] = useState('');
  const [mobileView, setMobileView] = useState('menu');
  const { loadConfig, loadMetadata, isLoading, error, config, localChanges } = useConfigStore();

  useEffect(() => { loadConfig(); loadMetadata(); }, [loadConfig, loadMetadata]);

  const filteredTabs = useMemo(() => {
    if (!searchQuery) return TABS;
    const q = searchQuery.toLowerCase();
    return TABS.filter(t => t.label.toLowerCase().includes(q) || t.description.toLowerCase().includes(q) || (t.keywords && t.keywords.some(k => k.includes(q))));
  }, [searchQuery]);

  const activeTabData = TABS.find((t) => t.id === activeTab);
  const ActiveComponent = activeTabData?.component;
  const groups = ['System', 'Project'];

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      <header className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary"><div className="w-1 h-1 rounded-full bg-current" /><span className="text-[10px] font-bold uppercase tracking-[0.2em] opacity-70">Engine Params</span></div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">System <span className="text-muted-foreground/40 font-medium">Settings</span></h1>
        </div>
        <div className="relative group"><Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Search parameters..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="h-10 pl-10 pr-4 bg-card/20 border-border/30 rounded-2xl text-xs shadow-sm transition-all" /></div>
      </header>

      <div className="flex flex-col md:flex-row gap-8 items-start">
        <aside className="hidden md:flex flex-col w-64 lg:w-72 space-y-8 sticky top-24 z-20">
          <nav className="space-y-8">
            {groups.map(g => {
              const gTabs = filteredTabs.filter(t => t.group === g); if (!gTabs.length) return null;
              return (
                <div key={g} className="space-y-3">
                  <h3 className="px-2 text-[10px] font-bold text-muted-foreground/40 uppercase tracking-widest">{g} Infrastructure</h3>
                  <div className="space-y-1">
                    {gTabs.map((t) => (
                      <button key={t.id} onClick={() => setActiveTab(t.id)} className={`w-full flex items-center justify-between p-3 text-xs font-bold rounded-xl transition-all duration-300 border ${activeTab === t.id ? 'bg-primary/[0.03] border-primary/20 text-primary' : 'text-muted-foreground/60 border-transparent hover:bg-accent/40'}`}>
                        <div className="flex items-center gap-3"><div className={`p-1.5 rounded-lg transition-colors ${activeTab === t.id ? 'bg-primary/10' : 'bg-muted/30 group-hover:bg-muted/50'}`}><t.icon className="w-3.5 h-3.5" /></div><span>{t.label}</span></div>
                        {getTabDirty(t.id, localChanges) && <span className="w-1 h-1 rounded-full bg-warning animate-pulse" />}
                      </button>
                    ))}
                  </div>
                </div>
              );
            })}
          </nav>
        </aside>

        <main className={`flex-1 w-full min-w-0 ${mobileView === 'content' ? 'block' : 'hidden md:block'}`}>
          <div className="bg-card/10 backdrop-blur-xl border border-border/30 rounded-3xl overflow-hidden shadow-soft">
            {isLoading && !config ? <div className="flex flex-col items-center justify-center py-32 gap-4"><Loader2 className="w-8 h-8 animate-spin text-primary/40" /><p className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40">Syncing Node</p></div>
            : error ? <div className="p-12 text-center space-y-6"><h3 className="text-lg font-bold text-foreground/80">Sync Error</h3><Button onClick={loadConfig} variant="outline" className="rounded-xl border-border/60">Retry</Button></div>
            : <div className="animate-fade-in">
                 <header className="px-8 py-6 border-b border-border/20 bg-background/20 flex items-center justify-between">
                    <div className="flex items-center gap-4">
                       <button onClick={() => setMobileView('menu')} className="md:hidden p-2 -ml-2 text-muted-foreground/60"><ArrowLeft className="w-5 h-5" /></button>
                       <div><h2 className="text-lg font-bold text-foreground tracking-tight leading-none">{activeTabLabel}</h2><p className="text-[11px] text-muted-foreground/50 font-medium mt-1">{activeTabData.description}</p></div>
                    </div>
                    {getTabDirty(activeTab, localChanges) && <Badge variant="secondary" className="bg-warning/5 text-warning/70 border-warning/10 text-[9px] font-bold">Modified</Badge>}
                 </header>
                 <div className="p-8"><ActiveComponent /></div>
              </div>}
          </div>
        </main>

        <div className={`md:hidden flex-1 space-y-8 animate-fade-in ${mobileView === 'menu' ? 'block' : 'hidden'}`}>
          {groups.map(g => {
            const gTabs = filteredTabs.filter(t => t.group === g); if (!gTabs.length) return null;
            return (
              <div key={g} className="space-y-3">
                <h3 className="px-2 text-[10px] font-bold text-muted-foreground/40 uppercase tracking-widest">{g} Core</h3>
                <div className="grid grid-cols-1 gap-2">
                  {gTabs.map((t) => (
                    <button key={t.id} onClick={() => handleTabClick(t.id)} className="w-full flex items-center justify-between p-4 rounded-2xl border border-border/30 bg-card/20 backdrop-blur-md active:scale-[0.98] transition-all">
                      <div className="flex items-center gap-4"><div className="p-2.5 rounded-xl bg-background border border-border/60 text-primary/60 shadow-inner"><t.icon className="w-5 h-5" /></div><span className="font-bold text-sm text-foreground/80">{t.label}</span></div>
                      <ChevronRight className="w-4 h-4 text-muted-foreground/20" />
                    </button>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      </div>
      <SettingsToolbar /><ConflictDialog />
    </div>
  );
}