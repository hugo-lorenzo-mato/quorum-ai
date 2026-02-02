import { useEffect, useState, useMemo } from 'react';
import { useConfigStore } from '../stores/configStore';
import { Search, ArrowLeft, ChevronRight } from 'lucide-react';
import {
  SettingsToolbar,
  ConflictDialog,
  GeneralTab,
  WorkflowTab,
  AgentsTab,
  PhasesTab,
  GitTab,
  AdvancedTab,
} from '../components/config';

const TABS = [
  { 
    id: 'general', 
    label: 'General', 
    icon: SettingsIcon, 
    component: GeneralTab,
    keywords: ['log', 'logging', 'chat', 'report', 'markdown', 'output'] 
  },
  { 
    id: 'workflow', 
    label: 'Workflow', 
    icon: WorkflowIcon, 
    component: WorkflowTab,
    keywords: ['timeout', 'retry', 'dry run', 'sandbox', 'state', 'database', 'backend'] 
  },
  { 
    id: 'agents', 
    label: 'Agents', 
    icon: AgentsIcon, 
    component: AgentsTab,
    keywords: ['model', 'temperature', 'provider', 'token', 'context'] 
  },
  { 
    id: 'phases', 
    label: 'Phases', 
    icon: PhasesIcon, 
    component: PhasesTab,
    keywords: ['step', 'order', 'execution'] 
  },
  { 
    id: 'git', 
    label: 'Git', 
    icon: GitIcon, 
    component: GitTab,
    keywords: ['commit', 'push', 'pr', 'pull request', 'merge', 'github', 'branch'] 
  },
  { 
    id: 'advanced', 
    label: 'Advanced', 
    icon: AdvancedIcon, 
    component: AdvancedTab,
    keywords: ['trace', 'debug', 'server', 'port', 'host', 'reset', 'danger'] 
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
    // On mobile we just filter the list
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

  return (
    <div className="space-y-4 sm:space-y-6 pb-32 sm:pb-32 animate-fade-in">
      {/* Header */}
      <div className={`flex flex-col sm:flex-row sm:items-center justify-between gap-4 ${mobileView === 'content' ? 'hidden md:flex' : 'flex'}`}>
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-foreground tracking-tight">Settings</h1>
          <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
            Configure Quorum behavior and preferences
          </p>
        </div>
        
        {/* Search Input */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search settings..."
            value={searchQuery}
            onChange={handleSearchChange}
            className="h-9 w-full pl-9 pr-4 rounded-lg border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-all placeholder:text-muted-foreground/50"
          />
        </div>
      </div>

      {/* Mobile Content Header */}
      <div className={`md:hidden items-center gap-3 ${mobileView === 'content' ? 'flex' : 'hidden'}`}>
        <button 
          onClick={handleMobileBack}
          className="p-2 -ml-2 rounded-lg hover:bg-accent text-muted-foreground"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <h2 className="text-lg font-semibold text-foreground">{activeTabLabel}</h2>
      </div>

      {/* Tab Navigation (Desktop) */}
      <div className="hidden md:block sticky top-14 z-30 -mx-6 px-6 py-3 border-b border-border bg-background/80 glass">
        <div className="flex items-center gap-1 p-1 rounded-lg bg-secondary border border-border overflow-x-auto scrollbar-none">
          {filteredTabs.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            const isDirty = getTabDirty(tab.id, localChanges);
            
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`relative flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-md transition-all whitespace-nowrap ${
                  isActive
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
                }`}
                type="button"
                role="tab"
                aria-selected={isActive}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
                {isDirty && (
                  <span className="absolute top-1 right-1 w-1.5 h-1.5 bg-warning rounded-full animate-fade-in" />
                )}
              </button>
            );
          })}
          {filteredTabs.length === 0 && (
            <div className="px-3 py-2 text-xs text-muted-foreground">
              No matching settings found
            </div>
          )}
        </div>
      </div>

      {/* Mobile Menu List */}
      <div className={`md:hidden space-y-2 ${mobileView === 'menu' ? 'block' : 'hidden'}`}>
        {filteredTabs.map((tab) => {
          const Icon = tab.icon;
          const isDirty = getTabDirty(tab.id, localChanges);
          return (
            <button
              key={tab.id}
              onClick={() => handleTabClick(tab.id)}
              className="w-full flex items-center justify-between p-4 rounded-xl border border-border bg-card hover:bg-accent active:scale-[0.98] transition-all"
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

      {/* Content */}
      <div className={`min-h-[400px] ${mobileView === 'content' ? 'block' : 'hidden md:block'}`}>
        {isLoading && !config && (
          <div className="flex items-center justify-center py-12">
            <svg className="animate-spin h-8 w-8 text-primary" viewBox="0 0 24 24" role="status">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
            </svg>
          </div>
        )}

        {error && (
          <div className="p-4 bg-error/10 border border-error/20 rounded-xl">
            <p className="text-error text-sm">{error}</p>
            <button
              onClick={loadConfig}
              className="mt-2 text-sm text-error hover:underline"
              type="button"
            >
              Retry
            </button>
          </div>
        )}

        {!isLoading && !error && config && ActiveComponent && (
          <div className="animate-fade-in">
             <ActiveComponent />
          </div>
        )}
      </div>

      {/* Floating Toolbar */}
      <SettingsToolbar />

      {/* Conflict Dialog */}
      <ConflictDialog />
    </div>
  );
}

// Icon Components
function SettingsIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

function WorkflowIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
    </svg>
  );
}

function AgentsIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  );
}

function PhasesIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
    </svg>
  );
}

function GitIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
    </svg>
  );
}

function AdvancedIcon({ className }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
    </svg>
  );
}
