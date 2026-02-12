import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useConfigStore } from '../stores/configStore';
import useProjectStore from '../stores/projectStore';
import { Search, ArrowLeft, ChevronRight, X, Settings as SettingsIcon, GitBranch as GitIcon, Terminal as AdvancedIcon, Archive as SnapshotsIcon, Workflow as WorkflowIcon, Bot as AgentsIcon, ListOrdered as PhasesIcon, Ticket as IssuesIcon } from 'lucide-react';
import { ConfirmDialog } from '../components/config/ConfirmDialog';
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
  SnapshotsTab,
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
  {
    id: 'snapshots',
    label: 'Snapshots & Restore',
    group: 'System',
    icon: SnapshotsIcon,
    component: SnapshotsTab,
    keywords: ['snapshot', 'backup', 'restore', 'import', 'export', 'validate', 'registry']
  },
  // Project Group
  { 
    id: 'workflow', 
    label: 'Workflow Defaults', 
    group: 'Project',
    icon: WorkflowIcon, 
    component: WorkflowTab,
    keywords: ['timeout', 'retry', 'dry run', 'state', 'database', 'backend'] 
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
    keywords: ['github', 'gitlab', 'issue', 'ticket', 'label', 'assignee', 'prompt']
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
    snapshots: [],
  };

  const relevantKeys = tabMappings[tabId] || [];
  return dirtyKeys.some(key => relevantKeys.includes(key));
};

export default function Settings({
  title = 'Settings',
  description = 'Manage configuration for the selected project',
  showProjectBanner = true,
}) {
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
  const projectConfigMode = useConfigStore((state) => state.projectConfigMode);
  const configSource = useConfigStore((state) => state.source);

  const navigate = useNavigate();
  const currentProjectId = useProjectStore((state) => state.currentProjectId);
  const updateProject = useProjectStore((state) => state.updateProject);
  const [isSwitchingConfig, setIsSwitchingConfig] = useState(false);
  const [showSwitchToCustomConfirm, setShowSwitchToCustomConfirm] = useState(false);
  const [showSwitchToGlobalConfirm, setShowSwitchToGlobalConfirm] = useState(false);

  // Load config and metadata on mount
  useEffect(() => {
    loadConfig();
    loadMetadata();
  }, [loadConfig, loadMetadata]);

  const handleSwitchToCustomConfig = async () => {
    if (!currentProjectId) return;
    setIsSwitchingConfig(true);
    const updated = await updateProject(currentProjectId, { config_mode: 'custom' });
    setIsSwitchingConfig(false);
    if (updated) {
      await loadConfig();
      await loadMetadata();
    }
  };

  const handleSwitchToGlobalConfig = async () => {
    if (!currentProjectId) return;
    setIsSwitchingConfig(true);
    const updated = await updateProject(currentProjectId, { config_mode: 'inherit_global' });
    setIsSwitchingConfig(false);
    if (updated) {
      await loadConfig();
      await loadMetadata();
    }
  };

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
    <div className="flex flex-col h-[calc(100vh-3.5rem)] bg-background overflow-hidden">{/* 3.5rem = h-14 del header */}
      {/* Header */}
      <header className="flex-none px-6 py-4 border-b border-border bg-card/50 backdrop-blur-sm z-10">
        <div className={`flex items-center justify-between gap-4 ${mobileView === 'content' ? 'hidden md:flex' : 'flex'}`}>
          <div className={showMobileSearch ? 'hidden md:block' : 'block'}>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">{title}</h1>
            <p className="hidden md:block text-sm text-muted-foreground mt-1">
              {description}
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

      {showProjectBanner && projectConfigMode === 'inherit_global' && (
        <div className="flex-none px-6 py-3 border-b border-border bg-muted/10">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                This project inherits the global configuration.
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Editing is disabled here. Update global defaults, or switch this project to a custom config to override settings.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => navigate('/settings/global')}
                className="px-3 py-1.5 text-xs font-medium rounded-lg border border-border bg-background hover:bg-accent transition-colors"
              >
                Edit Global Defaults
              </button>
              <button
                type="button"
                onClick={() => setShowSwitchToCustomConfirm(true)}
                disabled={isSwitchingConfig || !currentProjectId}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 disabled:pointer-events-none"
              >
                {isSwitchingConfig ? 'Switching...' : 'Switch To Custom Config'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showProjectBanner && projectConfigMode === 'custom' && (
        <div className="flex-none px-6 py-3 border-b border-border bg-muted/10">
          <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">
                This project uses a custom configuration.
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                Changes you make here apply only to this project. You can switch back to global defaults at any time.
              </p>
              {configSource === 'default' && (
                <p className="text-xs text-warning mt-1">
                  Project config file not found. You are currently seeing defaults. Switching modes can recreate or remove the project config file.
                </p>
              )}
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => navigate('/settings/global')}
                className="px-3 py-1.5 text-xs font-medium rounded-lg border border-border bg-background hover:bg-accent transition-colors"
              >
                Edit Global Defaults
              </button>
              <button
                type="button"
                onClick={() => setShowSwitchToGlobalConfirm(true)}
                disabled={isSwitchingConfig || !currentProjectId}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-warning text-black hover:bg-warning/90 transition-colors disabled:opacity-50 disabled:pointer-events-none"
              >
                {isSwitchingConfig ? 'Switching...' : 'Switch To Global Config'}
              </button>
            </div>
          </div>
        </div>
      )}

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
        <main className={`flex-1 overflow-y-auto bg-background p-3 sm:p-8 ${mobileView === 'content' ? 'block' : 'hidden md:block'}`}>
          <div className="space-y-6 pb-24">
            {isLoading && !config ? (
              <div className="flex items-center justify-center py-12" role="status" aria-live="polite">
                <Loader2 className="w-8 h-8 animate-spin text-primary" />
                <span className="sr-only">Loading settings</span>
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
              No settings found matching &quot;{searchQuery}&quot;
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

      {/* Switch mode confirmations */}
      <ConfirmDialog
        isOpen={showSwitchToCustomConfirm}
        onClose={() => setShowSwitchToCustomConfirm(false)}
        onConfirm={handleSwitchToCustomConfig}
        title="Switch to custom config?"
        message="This will create a project config file at .quorum/config.yaml by copying the current global defaults. After switching, edits here apply only to this project and global changes will not automatically apply. You can switch back later, which will delete the project config file."
        confirmText="Switch to Custom"
        cancelText="Cancel"
        variant="warning"
      />

      <ConfirmDialog
        isOpen={showSwitchToGlobalConfirm}
        onClose={() => setShowSwitchToGlobalConfirm(false)}
        onConfirm={handleSwitchToGlobalConfig}
        title="Switch back to global defaults?"
        message="This will delete the project config file at .quorum/config.yaml and the project will inherit the global configuration. Any project-specific settings will be lost."
        confirmText="Switch to Global"
        cancelText="Cancel"
        variant="danger"
      />

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
