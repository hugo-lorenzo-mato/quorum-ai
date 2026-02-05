import { useEffect, useState, useMemo } from 'react';
import { useConfigStore } from '../stores/configStore';
import { Search, ArrowLeft, ChevronRight, X, Settings as SettingsIcon, GitBranch as GitIcon, Terminal as AdvancedIcon, Workflow as WorkflowIcon, Bot as AgentsIcon, ListOrdered as PhasesIcon, Ticket as IssuesIcon, Sparkles, Info, ShieldCheck, Sliders, Globe } from 'lucide-react';
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
  { id: 'general', label: 'General', group: 'System', icon: SettingsIcon, component: GeneralTab, description: 'Application behaviors and global preferences', keywords: ['log', 'logging', 'chat', 'report', 'markdown', 'output'] },
  { id: 'git', label: 'Git Integration', group: 'System', icon: GitIcon, component: GitTab, description: 'Repository management and automation settings', keywords: ['commit', 'push', 'pr', 'pull request', 'merge', 'github', 'branch'] },
  { id: 'advanced', label: 'Advanced', group: 'System', icon: AdvancedIcon, component: AdvancedTab, description: 'System diagnostics and low-level flags', keywords: ['trace', 'debug', 'server', 'port', 'host', 'reset', 'danger'] },
  { id: 'workflow', label: 'Workflow Defaults', group: 'Project', icon: WorkflowIcon, component: WorkflowTab, description: 'Global execution parameters and safety', keywords: ['timeout', 'retry', 'dry run', 'sandbox', 'state', 'database', 'backend'] },
  { id: 'agents', label: 'Agents & Models', group: 'Project', icon: AgentsIcon, component: AgentsTab, description: 'Provider keys and model orchestration', keywords: ['model', 'temperature', 'provider', 'token', 'context'] },
  { id: 'phases', label: 'Execution Phases', group: 'Project', icon: PhasesIcon, component: PhasesTab, description: 'Orchestration step sequence management', keywords: ['step', 'order', 'execution'] },
  { id: 'issues', label: 'Issue Generation', group: 'Project', icon: IssuesIcon, component: IssuesTab, description: 'Ticketing system export configuration', keywords: ['github', 'gitlab', 'issue', 'ticket', 'label', 'assignee', 'template'] },
];

const getTabDirty = (id, local) => {
  const keys = Object.keys(local); if (keys.length === 0) return false;
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

  const handleSearch = (e) => {
    const q = e.target.value; setSearchQuery(q);
    if (window.innerWidth >= 768 && q) {
      const match = TABS.filter(t => t.label.toLowerCase().includes(q.toLowerCase()) || (t.keywords && t.keywords.some(k => k.includes(q.toLowerCase()))));
      if (match.length > 0 && !match.find(t => t.id === activeTab)) setActiveTab(match[0].id);
    }
  };

  const handleTabClick = (id) => { setActiveTab(id); setMobileView('content'); };
  const activeTabData = TABS.find((t) => t.id === activeTab);
  const ActiveComponent = activeTabData?.component;
  const groups = ['System', 'Project'];

  return (
    <div className="relative min-h-full space-y-10 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header Settings */}
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-10">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]"><Sliders className="h-3 w-3 opacity-70" /> Logic Control</div>
            <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">Engine <span className="text-primary/80">Parameters</span></h1>
            <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">Fine-tune the neural fabric of Quorum AI. Manage provider keys, security constraints, and integration protocols.</p>
          </div>
          <div className="hidden lg:flex shrink-0"><div className="p-8 rounded-[2.5rem] bg-background/40 border border-border/60 shadow-inner backdrop-blur-md"><SettingsIcon className="w-16 h-16 text-primary/20 animate-[spin_20s_linear_infinite]" /></div></div>
        </div>
      </div>

      <div className="flex flex-col md:flex-row gap-10 items-start">
        <aside className="hidden md:flex flex-col w-72 lg:w-80 space-y-10 sticky top-24 z-20">
          <div className="relative group"><Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Search settings..." value={searchQuery} onChange={handleSearch} className="h-12 pl-12 pr-6 rounded-2xl border-border/40 bg-card/30 backdrop-blur-md shadow-sm transition-all" /></div>
          <nav className="space-y-10">
            {groups.map(g => {
              const gTabs = filteredTabs.filter(t => t.group === g); if (!gTabs.length) return null;
              return (
                <div key={g} className="space-y-4">
                  <h3 className="px-3 text-[10px] font-bold text-muted-foreground/40 uppercase tracking-[0.3em]">{g} Fabric</h3>
                  <div className="space-y-2">
                    {gTabs.map((t) => (
                      <button key={t.id} onClick={() => setActiveTab(t.id)} className={`w-full flex items-center justify-between p-4 rounded-2xl transition-all duration-500 border ${activeTab === t.id ? 'bg-primary/5 border-primary/20 text-primary shadow-sm scale-[1.02] z-10' : 'text-muted-foreground/60 border-transparent hover:bg-accent/40 hover:text-foreground'}`}>
                        <div className="flex items-center gap-4"><div className={`p-2 rounded-xl transition-colors duration-500 ${activeTab === t.id ? 'bg-primary/10' : 'bg-muted/30 group-hover:bg-muted/50'}`}><t.icon className="w-4 h-4" /></div><span className="text-sm font-bold tracking-tight">{t.label}</span></div>
                        {getTabDirty(t.id, localChanges) && <span className={`w-1.5 h-1.5 rounded-full ${activeTab === t.id ? 'bg-primary' : 'bg-warning animate-pulse'}`} />}
                      </button>
                    ))}
                  </div>
                </div>
              );
            })}
          </nav>
        </aside>

        <main className={`flex-1 w-full min-w-0 ${mobileView === 'content' ? 'block' : 'hidden md:block'} animate-fade-up`}>
          <div className="bg-card/30 backdrop-blur-2xl border border-border/40 rounded-[2.5rem] overflow-hidden shadow-[0_20px_50px_rgba(0,0,0,0.05)]">
            {isLoading && !config ? <div className="flex flex-col items-center justify-center py-32 gap-6"><Loader2 className="w-12 h-12 animate-spin text-primary/60" /><p className="text-[10px] font-bold uppercase tracking-[0.3em] text-muted-foreground/40">Synchronizing Parameters</p></div>
            : error ? <div className="p-12 text-center space-y-8"><div className="p-6 bg-destructive/5 border border-destructive/10 rounded-full inline-flex text-destructive/60 shadow-inner"><Info className="w-10 h-10" /></div><div className="space-y-2"><h3 className="text-xl font-bold text-foreground/80">Protocol Error</h3><p className="text-sm text-muted-foreground/40 max-w-xs mx-auto leading-relaxed">{error}</p></div><Button onClick={loadConfig} variant="outline" className="rounded-2xl font-bold px-10 border-border/60">Restore Connection</Button></div>
            : !ActiveComponent ? <div className="flex flex-col items-center justify-center py-32 opacity-20"><SettingsIcon className="w-20 h-16 mb-6" /><p className="font-bold uppercase tracking-widest text-xs">Awaiting Category Selection</p></div>
            : <div className="animate-fade-in">
                 <header className="p-8 md:p-12 border-b border-border/30 bg-primary/[0.01] flex items-start justify-between gap-8">
                    <div className="flex items-center gap-6 min-w-0">
                       <button onClick={() => setMobileView('menu')} className="md:hidden p-3 -ml-3 rounded-2xl hover:bg-accent text-muted-foreground/60"><ArrowLeft className="w-6 h-6" /></button>
                       <div className="p-4 rounded-3xl bg-background border border-border/60 shadow-sm text-primary/60 shrink-0"><activeTabData.icon className="w-7 h-7" /></div>
                       <div className="min-w-0 space-y-1.5"><h2 className="text-2xl md:text-3xl font-bold text-foreground tracking-tighter leading-none">{activeTabData.label}</h2><p className="text-sm text-muted-foreground/60 font-medium truncate">{activeTabData.description}</p></div>
                    </div>
                    {getTabDirty(activeTab, localChanges) && <Badge variant="secondary" className="bg-warning/5 text-warning/70 border-warning/10 font-bold uppercase tracking-widest text-[9px] py-1 px-3 shadow-inner">Changes Pending</Badge>}
                 </header>
                 <div className="p-8 md:p-12 selection:bg-primary/10"><ActiveComponent /></div>
              </div>}
          </div>
        </main>

        <div className={`md:hidden flex-1 space-y-10 animate-fade-in ${mobileView === 'menu' ? 'block' : 'hidden'}`}>
           <div className="relative"><Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/30" /><Input placeholder="Search engine parameters..." value={searchQuery} onChange={handleSearch} className="h-14 pl-12 pr-6 rounded-2xl bg-card/20 backdrop-blur-md border-border/40 shadow-sm" /></div>
          <div className="space-y-10">
            {groups.map(g => {
              const gTabs = filteredTabs.filter(t => t.group === g); if (!gTabs.length) return null;
              return (
                <div key={g} className="space-y-4">
                  <h3 className="px-3 text-[10px] font-bold text-muted-foreground/40 uppercase tracking-[0.3em]">{g} Core</h3>
                  <div className="grid grid-cols-1 gap-3">
                    {gTabs.map((t) => (
                      <button key={t.id} onClick={() => handleTabClick(t.id)} className="w-full flex items-center justify-between p-5 rounded-[1.5rem] border border-border/40 bg-card/40 backdrop-blur-md shadow-sm active:scale-[0.98] transition-all">
                        <div className="flex items-center gap-5"><div className="p-3 rounded-2xl bg-background border border-border/60 text-primary/60 shadow-inner"><t.icon className="w-6 h-6" /></div><div className="text-left space-y-1"><span className="font-bold text-foreground tracking-tight block">{t.label}</span><span className="text-[10px] text-muted-foreground/40 font-medium line-clamp-1">{t.description}</span></div></div>
                        <div className="flex items-center gap-3">{getTabDirty(t.id, localChanges) && <span className="w-2 h-2 bg-warning rounded-full shadow-[0_0_10px_rgba(var(--color-warning),0.4)]" />}<ChevronRight className="w-5 h-5 text-muted-foreground/20" /></div>
                      </button>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
      <SettingsToolbar /><ConflictDialog />
    </div>
  );
}

function Loader2({ className }) { return <svg className={className} xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56" /></svg>; }
