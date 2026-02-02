import { Link, useLocation } from 'react-router-dom';
import { useUIStore } from '../stores';
import {
  LayoutDashboard,
  GitBranch,
  MessageSquare,
  Settings,
  PanelLeftClose,
  PanelLeft,
  Sun,
  Moon,
  Monitor,
  Sparkles,
  Wifi,
  WifiOff,
  RefreshCw,
  KanbanSquare,
  ChevronRight,
} from 'lucide-react';

const navItems = [
  { path: '/', label: 'Dashboard', icon: LayoutDashboard },
  { path: '/workflows', label: 'Workflows', icon: GitBranch },
  { path: '/kanban', label: 'Kanban', icon: KanbanSquare },
  { path: '/chat', label: 'Chat', icon: MessageSquare },
  { path: '/settings', label: 'Settings', icon: Settings },
];

function Breadcrumbs() {
  const location = useLocation();
  const pathnames = location.pathname.split('/').filter((x) => x);

  const breadcrumbs = [
    { name: 'Home', path: '/' },
    ...pathnames.map((name, index) => {
      const routeTo = `/${pathnames.slice(0, index + 1).join('/')}`;
      const isLast = index === pathnames.length - 1;
      
      // Find label in navItems if possible
      const navItem = navItems.find((item) => item.path === routeTo);
      let label = navItem ? navItem.label : name;

      // Special case: if name looks like an ID, maybe shorten or label it
      if (name.length > 20) {
        label = `${name.substring(0, 8)}...`;
      }
      
      // Capitalize first letter if it's a generic path
      if (!navItem && name.length <= 20) {
        label = name.charAt(0).toUpperCase() + name.slice(1);
      }

      return { name: label, path: routeTo, isLast };
    }),
  ];

  return (
    <nav className="hidden sm:flex items-center text-sm text-muted-foreground">
      {breadcrumbs.map((crumb, index) => (
        <div key={crumb.path} className="flex items-center">
          {index > 0 && <ChevronRight className="w-4 h-4 mx-1 text-muted-foreground/50" />}
          {crumb.isLast ? (
            <span className="font-medium text-foreground">{crumb.name}</span>
          ) : (
            <Link
              to={crumb.path}
              className="hover:text-foreground hover:underline transition-colors"
            >
              {crumb.name}
            </Link>
          )}
        </div>
      ))}
    </nav>
  );
}

function ThemeSwitcher() {
  const { theme, setTheme } = useUIStore();

  const themes = [
    { value: 'light', icon: Sun, label: 'Light' },
    { value: 'dark', icon: Moon, label: 'Dark' },
    { value: 'system', icon: Monitor, label: 'System' },
  ];

  return (
    <div className="flex items-center gap-1 p-1 rounded-lg bg-secondary">
      {themes.map(({ value, icon: Icon, label }) => (
        <button
          key={value}
          onClick={() => setTheme(value)}
          className={`p-1.5 rounded-md transition-all ${
            theme === value
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'
          }`}
          title={label}
        >
          <Icon className="w-4 h-4" />
        </button>
      ))}
    </div>
  );
}

function ConnectionStatus({ compact = false }) {
  const { connectionMode, retrySSEFn } = useUIStore();

  const getStatusConfig = () => {
    switch (connectionMode) {
      case 'sse':
        return {
          icon: Wifi,
          label: 'Live',
          color: 'text-green-500',
          bgColor: 'bg-green-500/10',
          borderColor: 'border-green-500/20',
        };
      case 'polling':
        return {
          icon: RefreshCw,
          label: 'Polling',
          color: 'text-yellow-500',
          bgColor: 'bg-yellow-500/10',
          borderColor: 'border-yellow-500/20',
          animate: true,
        };
      case 'disconnected':
      default:
        return {
          icon: WifiOff,
          label: 'Offline',
          color: 'text-red-500',
          bgColor: 'bg-red-500/10',
          borderColor: 'border-red-500/20',
        };
    }
  };

  const config = getStatusConfig();
  const Icon = config.icon;

  if (compact) {
    return (
      <div className={`p-1.5 rounded-md ${config.bgColor} ${config.color}`} title={config.label}>
        <Icon className={`w-4 h-4 ${config.animate ? 'animate-spin' : ''}`} />
      </div>
    );
  }

  return (
    <div
      className={`flex items-center gap-2 px-2.5 py-1.5 rounded-lg border text-xs font-medium ${config.bgColor} ${config.borderColor} ${config.color}`}
    >
      <Icon className={`w-3.5 h-3.5 ${config.animate ? 'animate-spin' : ''}`} />
      <span>{config.label}</span>
      {connectionMode !== 'sse' && retrySSEFn && (
        <button
          onClick={retrySSEFn}
          className="ml-1 p-0.5 rounded hover:bg-white/10 transition-colors"
          title="Retry connection"
        >
          <RefreshCw className="w-3 h-3" />
        </button>
      )}
    </div>
  );
}

export default function Layout({ children }) {
  const location = useLocation();
  const { sidebarOpen, toggleSidebar } = useUIStore();

  return (
    <div className="min-h-screen bg-background">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={toggleSidebar}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 flex flex-col border-r border-border bg-card/50 glass transition-all duration-300 ease-in-out md:translate-x-0 ${
          sidebarOpen ? 'w-64 translate-x-0' : 'w-16 -translate-x-full md:translate-x-0'
        }`}
      >
        {/* Logo */}
        <div className="flex items-center justify-between h-14 px-4 border-b border-border">
          <Link to="/" className="flex items-center gap-2">
            <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-primary">
              <Sparkles className="w-4 h-4 text-primary-foreground" />
            </div>
            {sidebarOpen && (
              <span className="font-semibold text-foreground animate-fade-in">
                Quorum AI
              </span>
            )}
          </Link>
          <button
            onClick={toggleSidebar}
            className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          >
            {sidebarOpen ? (
              <PanelLeftClose className="w-4 h-4" />
            ) : (
              <PanelLeft className="w-4 h-4" />
            )}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-2 space-y-1 overflow-y-auto">
          {navItems.map((item) => {
            const isActive = location.pathname === item.path ||
              (item.path !== '/' && location.pathname.startsWith(item.path));
            const Icon = item.icon;

            return (
              <Link
                key={item.path}
                to={item.path}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-all ${
                  isActive
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
                } ${!sidebarOpen ? 'justify-center' : ''}`}
                title={!sidebarOpen ? item.label : undefined}
              >
                <Icon className="w-5 h-5 flex-shrink-0" />
                {sidebarOpen && (
                  <span className="animate-fade-in">{item.label}</span>
                )}
              </Link>
            );
          })}
        </nav>

        {/* Bottom section */}
        <div className="p-3 border-t border-border space-y-3">
          {sidebarOpen ? (
            <>
              <ConnectionStatus />
              <ThemeSwitcher />
            </>
          ) : (
            <div className="flex flex-col items-center gap-2">
              <ConnectionStatus compact />
            </div>
          )}
        </div>
      </aside>

      {/* Main content */}
      <main
        className={`min-h-screen transition-all duration-300 ease-in-out pl-0 ${
          sidebarOpen ? 'md:pl-64' : 'md:pl-16'
        }`}
      >
        {/* Top bar */}
        <header className="sticky top-0 z-40 h-14 border-b border-border bg-background/80 glass">
          <div className="flex items-center justify-between h-full px-3 sm:px-6">
            <div className="flex items-center gap-3 sm:gap-4">
              {/* Mobile menu button */}
              <button
                onClick={toggleSidebar}
                className="md:hidden p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                aria-label="Toggle sidebar"
              >
                <PanelLeft className="w-5 h-5" />
              </button>
              
              {/* Breadcrumbs (Desktop) / Title (Mobile) */}
              <div className="hidden sm:block">
                <Breadcrumbs />
              </div>
              <h1 className="sm:hidden text-xs font-medium text-muted-foreground">
                {navItems.find(item =>
                  location.pathname === item.path ||
                  (item.path !== '/' && location.pathname.startsWith(item.path))
                )?.label || 'Quorum AI'}
              </h1>
            </div>
            <div className="flex items-center gap-3">
              {/* Additional header actions can go here */}
            </div>
          </div>
        </header>

        {/* Page content */}
        <div className="p-3 sm:p-6">
          {children}
        </div>
      </main>
    </div>
  );
}
