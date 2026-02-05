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
  // System Group
  { 
    id: 'general', 
    label: 'General', 
    group: 'System',
    icon: SettingsIcon, 
    component: GeneralTab,
    description: 'Global app behavior, logging, and chat preferences',
    keywords: ['log', 'logging', 'chat', 'report', 'markdown', 'output'] 
  },
  {
    id: 'git',
    label: 'Git Integration',
    group: 'System',
    icon: GitIcon,
    component: GitTab,
    description: 'Repository management, branches, and PR settings',
    keywords: ['commit', 'push', 'pr', 'pull request', 'merge', 'github', 'branch']
  },
  {
    id: 'advanced',
    label: 'Advanced',
    group: 'System',
    icon: AdvancedIcon,
    component: AdvancedTab,
    description: 'System diagnostics, trace tracing, and low-level flags',
    keywords: ['trace', 'debug', 'server', 'port', 'host', 'reset', 'danger']
  },
  // Project Group
  { 
    id: 'workflow', 
    label: 'Workflow Defaults', 
    group: 'Project',
    icon: WorkflowIcon, 
    component: WorkflowTab,
    description: 'Global execution parameters and safety constraints',
    keywords: ['timeout', 'retry', 'dry run', 'sandbox', 'state', 'database', 'backend'] 
  },
  { 
    id: 'agents', 
    label: 'Agents & Models', 
    group: 'Project',
    icon: AgentsIcon, 
    component: AgentsTab,
    description: 'Provider keys and default model configurations',
    keywords: ['model', 'temperature', 'provider', 'token', 'context'] 
  },
  { 
    id: 'phases', 
    label: 'Execution Phases', 
    group: 'Project',
    icon: PhasesIcon, 
    component: PhasesTab,
    description: 'Manage the sequence of AI orchestration steps',
    keywords: ['step', 'order', 'execution'] 
  },
  {
    id: 'issues',
    label: 'Issue Generation',
    group: 'Project',
    icon: IssuesIcon,
    component: IssuesTab,
    description: 'Configuration for ticketing system exports',
    keywords: ['github', 'gitlab', 'issue', 'ticket', 'label', 'assignee', 'template']
  },
];

const getTabDirty = (tabId, localChanges) => {
  const dirtyKeys = Object.keys(localChanges);
  if (dirtyKeys.length === 0) return false;

  const tabMappings = {
    general: ['log', 'chat', 'report'],
    workflow: ['workflow', 'state'],
    agents: ['agents'],
    phases: ['phases'],
    git: ['git', 'github'],
    issues: ['issues'],
    advanced: ['trace', 'server'],
  };

  const relevantKeys = tabMappings[tabId] || [];
  return dirtyKeys.some(key => relevantKeys.includes(key));
};

export default function Settings() {
  const [activeTab, setActiveTab] = useState('general');
  const [searchQuery, setSearchQuery] = useState('');
  const [mobileView, setMobileView] = useState('menu');
  
  const loadConfig = useConfigStore((state) => state.loadConfig);
  const loadMetadata = useConfigStore((state) => state.loadMetadata);
  const isLoading = useConfigStore((state) => state.isLoading);
  const error = useConfigStore((state) => state.error);
  const config = useConfigStore((state) => state.config);
  const localChanges = useConfigStore((state) => state.localChanges);

  useEffect(() => {
    loadConfig();
    loadMetadata();
  }, [loadConfig, loadMetadata]);

  const filteredTabs = useMemo(() => {
    if (!searchQuery) return TABS;
    const lowerQuery = searchQuery.toLowerCase();
    return TABS.filter(tab => 
      tab.label.toLowerCase().includes(lowerQuery) || 
      tab.description.toLowerCase().includes(lowerQuery) ||
      (tab.keywords && tab.keywords.some(k => k.includes(lowerQuery)))
    );
  }, [searchQuery]);

  const handleSearchChange = (e) => {
    const query = e.target.value;
    setSearchQuery(query);
    
    if (window.innerWidth >= 768 && query) {
      const lowerQuery = query.toLowerCase();
      const matching = TABS.filter(tab => 
        tab.label.toLowerCase().includes(lowerQuery) || 
        (tab.keywords && tab.keywords.some(k => k.includes(lowerQuery)))
      );
      
      if (matching.length > 0 && !matching.find(t => t.id === activeTab)) {
        setActiveTab(matching[0].id);
      }
    }
  };

  const handleTabClick = (tabId) => {
    setActiveTab(tabId);
    setMobileView('content');
  };

  const activeTabData = TABS.find((t) => t.id === activeTab);
  const ActiveComponent = activeTabData?.component;
  const activeTabLabel = activeTabData?.label;

  const groups = ['System', 'Project'];

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      {/* Background Pattern */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/30 backdrop-blur-md p-8 sm:p-12 shadow-sm">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-8">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
              <Sliders className="h-3 w-3" />
              Environment Control
            </div>
            <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
              System <span className="text-primary">Settings</span>
            </h1>
            <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
              Configure your Quorum AI engine. Fine-tune agent behaviors, security protocols, and system integration parameters.
            </p>
          </div>

          <div className="hidden md:flex shrink-0">
             <div className="p-6 rounded-3xl bg-background/50 border border-border shadow-inner backdrop-blur-sm">
                <SettingsIcon className="w-12 h-12 text-primary/40 animate-[spin_10s_linear_infinite]" />
             </div>
          </div>
        </div>
      </div>

      {/* Main Layout - Sidebar + Content */}
      <div className="flex flex-col md:flex-row gap-8 items-start">
        
        {/* Desktop Sidebar Nav */}
        <aside className={`hidden md:flex flex-col w-72 lg:w-80 space-y-8 sticky top-24 z-20`}>
          <div className="relative">
             <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
             <Input
               placeholder="Search settings..."
               value={searchQuery}
               onChange={handleSearchChange}
               className="pl-10 h-11 bg-card/50 backdrop-blur-sm border-border rounded-xl shadow-sm focus-visible:ring-primary/20"
             />
          </div>

          <nav className="space-y-8">
            {groups.map(group => {
              const groupTabs = filteredTabs.filter(tab => tab.group === group);
              if (groupTabs.length === 0) return null;

              return (
                <div key={group} className="space-y-3">
                  <h3 className="px-2 text-[10px] font-black text-muted-foreground uppercase tracking-[0.2em] opacity-60">
                    {group} Infrastructure
                  </h3>
                  <div className="space-y-1.5">
                    {groupTabs.map((tab) => {
                      const Icon = tab.icon;
                      const isActive = activeTab === tab.id;
                      const isDirty = getTabDirty(tab.id, localChanges);
                      
                      return (
                        <button
                          key={tab.id}
                          onClick={() => setActiveTab(tab.id)}
                          className={`w-full flex items-center justify-between p-3 text-sm font-bold rounded-xl transition-all duration-300 border ${
                            isActive
                              ? 'bg-primary text-primary-foreground border-primary shadow-lg shadow-primary/20 scale-[1.02] z-10'
                              : 'text-muted-foreground bg-card/20 border-border/50 hover:border-primary/30 hover:bg-card/50 hover:text-foreground'
                          }`}
                        >
                          <div className="flex items-center gap-3">
                            <div className={`p-1.5 rounded-lg ${isActive ? 'bg-white/20' : 'bg-muted/50 group-hover:bg-muted'}`}>
                               <Icon className="w-4 h-4" />
                            </div>
                            {tab.label}
                          </div>
                          {isDirty && (
                            <span className={`w-2 h-2 rounded-full ${isActive ? 'bg-white' : 'bg-warning animate-pulse'}`} />
                          )}
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </nav>
        </aside>

        {/* Content Area */}
        <main className={`flex-1 w-full min-w-0 ${mobileView === 'content' ? 'block' : 'hidden md:block'}`}>
          <div className="bg-card/40 backdrop-blur-md border border-border rounded-3xl overflow-hidden shadow-xl animate-fade-up">
            {isLoading && !config ? (
              <div className="flex flex-col items-center justify-center py-24 gap-4">
                <Loader2 className="w-10 h-10 animate-spin text-primary" />
                <p className="text-xs font-black uppercase tracking-widest text-muted-foreground">Synchronizing State</p>
              </div>
            ) : error ? (
              <div className="p-8 text-center space-y-6">
                <div className="p-4 bg-destructive/10 border border-destructive/20 rounded-full inline-flex text-destructive">
                   <Info className="w-8 h-8" />
                </div>
                <div>
                   <h3 className="text-lg font-bold text-foreground">Connection Error</h3>
                   <p className="text-sm text-muted-foreground mt-1">{error}</p>
                </div>
                <Button onClick={loadConfig} variant="outline" className="rounded-xl font-bold">Try Reconnect</Button>
              </div>
            ) : !ActiveComponent ? (
              <div className="flex flex-col items-center justify-center py-24 text-center opacity-40">
                <SettingsIcon className="w-16 h-16 mb-4" />
                <p className="font-bold uppercase tracking-widest text-xs">Select Category</p>
              </div>
            ) : (
              <div className="animate-fade-in">
                 {/* Detail Header */}
                 <header className="p-6 md:p-8 border-b border-border bg-muted/20 flex items-start justify-between gap-4">
                    <div className="flex items-center gap-4">
                       <button 
                         onClick={() => setMobileView('menu')}
                         className="md:hidden p-2 -ml-2 rounded-xl hover:bg-accent text-muted-foreground"
                       >
                         <ArrowLeft className="w-5 h-5" />
                       </button>
                       <div className="p-3 rounded-2xl bg-background border border-border shadow-sm text-primary">
                          <activeTabData.icon className="w-6 h-6" />
                       </div>
                       <div>
                          <h2 className="text-xl md:text-2xl font-black text-foreground tracking-tight leading-none mb-1.5">{activeTabLabel}</h2>
                          <p className="text-xs md:text-sm text-muted-foreground font-medium">
                            {activeTabData.description}
                          </p>
                       </div>
                    </div>
                    {getTabDirty(activeTab, localChanges) && (
                       <Badge variant="secondary" className="bg-warning/10 text-warning border-warning/20 font-black uppercase tracking-widest text-[9px] py-1 px-2 animate-fade-in">
                          Pending Changes
                       </Badge>
                    )}
                 </header>

                 <div className="p-6 md:p-10">
                    <ActiveComponent />
                 </div>
              </div>
            )}
          </div>
        </main>

        {/* Mobile Nav Menu */}
        <div className={`md:hidden flex-1 space-y-8 animate-fade-in ${mobileView === 'menu' ? 'block' : 'hidden'}`}>
           <div className="relative">
             <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
             <Input
               placeholder="Search settings..."
               value={searchQuery}
               onChange={handleSearchChange}
               className="pl-10 h-12 bg-card/50 backdrop-blur-sm border-border rounded-2xl"
             />
          </div>

          <div className="space-y-8">
            {groups.map(group => {
              const groupTabs = filteredTabs.filter(tab => tab.group === group);
              if (groupTabs.length === 0) return null;

              return (
                <div key={group} className="space-y-3">
                  <h3 className="px-2 text-[10px] font-black text-muted-foreground uppercase tracking-[0.2em] opacity-60">
                    {group} Infrastructure
                  </h3>
                  <div className="grid grid-cols-1 gap-2">
                    {groupTabs.map((tab) => {
                      const Icon = tab.icon;
                      const isDirty = getTabDirty(tab.id, localChanges);
                      return (
                        <button
                          key={tab.id}
                          onClick={() => handleTabClick(tab.id)}
                          className="w-full flex items-center justify-between p-4 rounded-2xl border border-border bg-card/40 backdrop-blur-sm active:scale-[0.98] transition-all shadow-sm"
                        >
                          <div className="flex items-center gap-4">
                            <div className="p-2.5 rounded-xl bg-background border border-border shadow-sm text-primary">
                              <Icon className="w-5 h-5" />
                            </div>
                            <div className="text-left">
                               <span className="font-bold text-foreground block">{tab.label}</span>
                               <span className="text-[10px] text-muted-foreground font-medium line-clamp-1">{tab.description}</span>
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                             {isDirty && <span className="w-2 h-2 bg-warning rounded-full shadow-[0_0_8px_rgba(var(--color-warning),0.5)]" />}
                             <ChevronRight className="w-5 h-5 text-muted-foreground/30" />
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      <SettingsToolbar />
      <ConflictDialog />
    </div>
  );
}

function Loader2({ className }) {
  return (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
}