import { useEffect, useState, useMemo } from 'react';
import { useConfigStore } from '../stores/configStore';
import { Search, ArrowLeft, ChevronRight, X, Settings as SettingsIcon, GitBranch as GitIcon, Terminal as AdvancedIcon, Workflow as WorkflowIcon, Bot as AgentsIcon, ListOrdered as PhasesIcon, Ticket as IssuesIcon } from 'lucide-react';
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

const TABS = [
  // System Group
  { 
    id: 'general', 
    label: 'General', 
    group: 'System',
    icon: SettingsIcon, 
    component: GeneralTab,
    keywords: ['log', 'logging', 'chat', 'report', 'markdown', 'output'] 
  },
  {
    id: 'git',
    label: 'Git Integration',
    group: 'System',
    icon: GitIcon,
    component: GitTab,
    keywords: ['commit', 'push', 'pr', 'pull request', 'merge', 'github', 'branch']
  },
  {
    id: 'advanced',
    label: 'Advanced',
    group: 'System',
    icon: AdvancedIcon,
    component: AdvancedTab,
    keywords: ['trace', 'debug', 'server', 'port', 'host', 'reset', 'danger']
  },
  // Project Group
  { 
    id: 'workflow', 
    label: 'Workflow Defaults', 
    group: 'Project',
    icon: WorkflowIcon, 
    component: WorkflowTab,
    keywords: ['timeout', 'retry', 'dry run', 'sandbox', 'state', 'database', 'backend'] 
  },
  { 
    id: 'agents', 
    label: 'Agents & Models', 
    group: 'Project',
    icon: AgentsIcon, 
    component: AgentsTab,
    keywords: ['model', 'temperature', 'provider', 'token', 'context'] 
  },
  { 
    id: 'phases', 
    label: 'Execution Phases', 
    group: 'Project',
    icon: PhasesIcon, 
    component: PhasesTab,
    keywords: ['step', 'order', 'execution'] 
  },
  {
    id: 'issues',
    label: 'Issue Generation',
    group: 'Project',
    icon: IssuesIcon,
    component: IssuesTab,
    keywords: ['github', 'gitlab', 'issue', 'ticket', 'label', 'assignee', 'template']
  },
];

// Helper to determine if a tab has dirty fields
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
  
  // Mobile navigation state
  const [mobileView, setMobileView] = useState('menu'); // 'menu' | 'content'
  const [showMobileSearch, setShowMobileSearch] = useState(false);
  
  const loadConfig = useConfigStore((state) => state.loadConfig);
  const loadMetadata = useConfigStore((state) => state.loadMetadata);
  const isLoading = useConfigStore((state) => state.isLoading);
  const error = useConfigStore((state) => state.error);
  const config = useConfigStore((state) => state.config);
  const localChanges = useConfigStore((state) => state.localChanges);

  // Load config and metadata on mount
  useEffect(() => {
    loadConfig();
    loadMetadata();
  }, [loadConfig, loadMetadata]);

  // Filter tabs based on search
  const filteredTabs = useMemo(() => {
    if (!searchQuery) return TABS;
    const lowerQuery = searchQuery.toLowerCase();
    return TABS.filter(tab => 
      tab.label.toLowerCase().includes(lowerQuery) || 
      (tab.keywords && tab.keywords.some(k => k.includes(lowerQuery)))
    );
  }, [searchQuery]);

  const handleSearchChange = (e) => {
    const query = e.target.value;
    setSearchQuery(query);
    
    // Auto-switch tab if current becomes hidden (Desktop behavior)
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

  const handleMobileBack = () => {
    setMobileView('menu');
  };

  const ActiveComponent = TABS.find((t) => t.id === activeTab)?.component;
  const activeTabLabel = TABS.find((t) => t.id === activeTab)?.label;

  const groups = ['System', 'Project'];

  return (
    <div className="flex flex-col md:h-[calc(100vh-4rem)] bg-background z-0 pb-10">
      {/* Header */}
      <header className="flex-none px-6 py-4 border-b border-border bg-card/50 backdrop-blur-sm z-10">
        <div className={`flex items-center justify-between gap-4 ${mobileView === 'content' ? 'hidden md:flex' : 'flex'}`}>
          <div className={showMobileSearch ? 'hidden md:block' : 'block'}>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Settings</h1>
            <p className="hidden md:block text-sm text-muted-foreground mt-1">
              Manage your global configuration and preferences
            </p>
          </div>
          
          {/* Mobile Search Toggle */}
          <div className="flex md:hidden items-center justify-end w-full gap-2">
             {showMobileSearch ? (
               <div className="flex-1 animate-slide-in relative">
                  <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                  <input
                    type="text"
                    placeholder="Search settings..."
                    value={searchQuery}
                    onChange={handleSearchChange}
                    autoFocus
                    className="h-9 w-full pl-9 pr-8 rounded-lg border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                  />
                  <button 
                    onClick={() => {
                      setShowMobileSearch(false);
                      setSearchQuery('');
                    }}
                    className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    <X className="w-4 h-4" />
                  </button>
               </div>
             ) : (
               <button 
                 onClick={() => setShowMobileSearch(true)}
                 className="p-2 -mr-2 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground"
               >
                 <Search className="w-5 h-5" />
               </button>
             )}
          </div>

          {/* Desktop Search Input */}
          <div className="hidden md:block relative w-72">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search settings..."
              value={searchQuery}
              onChange={handleSearchChange}
              className="h-10 w-full pl-10 pr-4 rounded-lg border border-border bg-secondary/50 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-all placeholder:text-muted-foreground/50 hover:bg-secondary/80 focus:bg-background"
            />
          </div>
        </div>

        {/* Mobile Content Header */}
        <div className={`md:hidden items-center gap-3 ${mobileView === 'content' ? 'flex animate-slide-in' : 'hidden'}`}>
          <button 
            onClick={handleMobileBack}
            className="p-2 -ml-2 rounded-lg hover:bg-accent text-muted-foreground"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <h2 className="text-lg font-semibold text-foreground">{activeTabLabel}</h2>
        </div>
      </header>

      {/* Main Layout */}
      <div className="flex-1 flex overflow-hidden">
        {/* Desktop Sidebar */}
        <nav className="hidden md:block w-64 lg:w-72 border-r border-border bg-card/30 overflow-y-auto p-4 space-y-8">
          {groups.map(group => {
            const groupTabs = filteredTabs.filter(tab => tab.group === group);
            if (groupTabs.length === 0) return null;

            return (
              <div key={group} className="space-y-2">
                <h3 className="px-3 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  {group}
                </h3>
                <div className="space-y-1">
                  {groupTabs.map((tab) => {
                    const Icon = tab.icon;
                    const isActive = activeTab === tab.id;
                    const isDirty = getTabDirty(tab.id, localChanges);
                    
                    return (
                      <button
                        key={tab.id}
                        onClick={() => setActiveTab(tab.id)}
                        className={`w-full flex items-center justify-between px-3 py-2 text-sm font-medium rounded-lg transition-all ${
                          isActive
                            ? 'bg-primary text-primary-foreground shadow-md shadow-primary/20'
                            : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                        }`}
                      >
                        <div className="flex items-center gap-3">
                          <Icon className={`w-4 h-4 ${isActive ? 'text-primary-foreground' : 'text-muted-foreground'}`} />
                          {tab.label}
                        </div>
                        {isDirty && (
                          <span className={`w-2 h-2 rounded-full ${isActive ? 'bg-white' : 'bg-warning'}`} />
                        )}
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </nav>

        {/* Content Area */}
        <main className={`flex-1 overflow-y-auto bg-background p-4 sm:p-8 ${mobileView === 'content' ? 'block' : 'hidden md:block'}`}>
          <div className="max-w-4xl mx-auto space-y-6 pb-24">
            {isLoading && !config ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="w-8 h-8 animate-spin text-primary" />
              </div>
            ) : error ? (
              <div className="p-4 bg-destructive/10 border border-destructive/20 rounded-xl text-destructive flex items-center justify-between">
                <span>{error}</span>
                <button onClick={loadConfig} className="text-sm font-medium underline hover:no-underline">Retry</button>
              </div>
            ) : !ActiveComponent ? (
              <div className="text-center py-12 text-muted-foreground">
                Select a setting category to get started
              </div>
            ) : (
              <div className="animate-fade-in">
                 {/* Desktop Title for the active section */}
                 <div className="hidden md:block mb-6 pb-4 border-b border-border">
                    <h2 className="text-2xl font-semibold text-foreground">{activeTabLabel}</h2>
                    <p className="text-muted-foreground text-sm mt-1">
                      Customize settings for {activeTabLabel.toLowerCase()}
                    </p>
                 </div>
                 <ActiveComponent />
              </div>
            )}
          </div>
        </main>

        {/* Mobile Menu List */}
        <div className={`md:hidden flex-1 overflow-y-auto p-4 space-y-6 bg-background ${mobileView === 'menu' ? 'block animate-fade-in' : 'hidden'}`}>
          {filteredTabs.length === 0 && (
            <div className="text-center py-8 text-muted-foreground">
              No settings found matching "{searchQuery}"
            </div>
          )}
          
          {groups.map(group => {
            const groupTabs = filteredTabs.filter(tab => tab.group === group);
            if (groupTabs.length === 0) return null;

            return (
              <div key={group} className="space-y-2">
                <h3 className="px-1 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  {group}
                </h3>
                <div className="space-y-1">
                  {groupTabs.map((tab) => {
                    const Icon = tab.icon;
                    const isDirty = getTabDirty(tab.id, localChanges);
                    return (
                      <button
                        key={tab.id}
                        onClick={() => handleTabClick(tab.id)}
                        className="w-full flex items-center justify-between p-3 rounded-xl border border-border bg-card hover:bg-accent active:scale-[0.98] transition-all"
                      >
                        <div className="flex items-center gap-3">
                          <div className="p-2 rounded-lg bg-secondary text-foreground">
                            <Icon className="w-5 h-5" />
                          </div>
                          <span className="font-medium text-foreground">{tab.label}</span>
                          {isDirty && (
                            <span className="w-2 h-2 bg-warning rounded-full" />
                          )}
                        </div>
                        <ChevronRight className="w-5 h-5 text-muted-foreground" />
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Floating Toolbar */}
      <SettingsToolbar />

      {/* Conflict Dialog */}
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
