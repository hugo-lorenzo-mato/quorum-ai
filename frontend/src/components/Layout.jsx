import { useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useUIStore } from '../stores';
import MobileBottomNav from './MobileBottomNav';
import Notifications from './Notifications';
import Logo from './Logo';
import ProjectSelector from './ProjectSelector';
import {
  LayoutDashboard,
  GitBranch,
  MessageSquare,
  Settings,
  PanelLeftClose,
  PanelLeft,
  Menu,
  Sun,
  Moon,
  Wifi,
  WifiOff,
  RefreshCw,
  KanbanSquare,
  ChevronRight,
  Droplets,
  Snowflake,
  Ghost,
  MoonStar,
  Github,
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
    { name: 'Home', path: '/', fullName: 'Home' },
    ...pathnames.map((name, index) => {
      const routeTo = `/${pathnames.slice(0, index + 1).join('/')}`;
      const isLast = index === pathnames.length - 1;

      // Find label in navItems if possible
      const navItem = navItems.find((item) => item.path === routeTo);
      let label = navItem ? navItem.label : name;
      const fullName = label;

      // Special case: if name looks like an ID, shorten for display but keep full for tooltip
      if (!navItem && name.length > 50) {
        label = `${name.substring(0, 20)}...`;
      }

      // Capitalize first letter if it's a generic path
      if (!navItem && name.length <= 50) {
        label = name.charAt(0).toUpperCase() + name.slice(1);
      }

      return { name: label, fullName, path: routeTo, isLast };
    }),
  ];

  return (
    <nav className="flex items-center text-sm text-muted-foreground overflow-hidden">
      {breadcrumbs.map((crumb, index) => (
        <div key={crumb.path} className="flex items-center min-w-0">
          {index > 0 && <ChevronRight className="w-4 h-4 mx-1 text-muted-foreground/50 flex-shrink-0" />}
          {crumb.isLast ? (
            <span
              className="font-medium text-foreground truncate max-w-[200px] sm:max-w-[300px] md:max-w-[400px]"
              title={crumb.fullName}
            >
              {crumb.name}
            </span>
          ) : (
            <Link
              to={crumb.path}
              className="hover:text-foreground hover:underline transition-colors truncate shrink-0"
              title={crumb.fullName}
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
    { value: 'midnight', icon: MoonStar, label: 'Midnight' },
    { value: 'dracula', icon: Ghost, label: 'Dracula' },
    { value: 'nord', icon: Snowflake, label: 'Nord' },
    { value: 'ocean', icon: Droplets, label: 'Ocean' },
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
  const { sidebarOpen, toggleSidebar, setSidebarOpen } = useUIStore();
  
  // Check if we're on the chat or settings page - they need full control of their layout
  const isFullLayoutPage = location.pathname === '/chat' || location.pathname === '/settings';

  // Close sidebar when resizing to mobile viewport (only triggers on actual resize)
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth < 768) {
        setSidebarOpen(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
    };
  }, [setSidebarOpen]);

  // Close sidebar on route change on mobile
  useEffect(() => {
    if (window.innerWidth < 768) {
      setSidebarOpen(false);
    }
    window.scrollTo(0, 0);
  }, [location.pathname, setSidebarOpen]);

  return (
    <div className="min-h-screen bg-background pb-16 md:pb-0">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-[55] md:hidden"
          onClick={toggleSidebar}
          aria-hidden="true"
        />
      )}

      {/* Sidebar - Hidden on mobile, visible on desktop */}
      <aside
        className={`fixed inset-y-0 left-0 z-[60] flex flex-col border-r border-border bg-background/95 backdrop-blur-xl md:bg-card/50 md:glass transition-all duration-300 ease-in-out md:translate-x-0 ${
          sidebarOpen ? 'w-64 translate-x-0' : 'w-16 -translate-x-full md:translate-x-0'
        }`}
      >
        {/* Logo */}
        <div className={`flex items-center h-14 border-b border-border ${sidebarOpen ? 'justify-between px-4' : 'justify-center px-2'}`}>
          <Link to="/" className="flex items-center gap-3 group">
            <div className="flex items-center justify-center w-8 h-8 rounded-lg bg-gradient-to-br from-primary/10 to-primary/5 shadow-sm transition-transform group-hover:scale-105 border border-primary/20">
              <Logo className="w-5 h-5" />
            </div>
            {sidebarOpen && (
              <span className="font-bold text-foreground tracking-tight animate-fade-in text-lg">
                Quorum AI
              </span>
            )}
          </Link>
          
          {/* Toggle Button - Only visible in sidebar when expanded */}
          {sidebarOpen && (
            <button
              onClick={toggleSidebar}
              className="p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
              title="Collapse sidebar"
            >
              <PanelLeftClose className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Project Selector */}
        <div className="px-2 py-2 border-b border-border">
          <ProjectSelector 
            collapsed={!sidebarOpen} 
            onProjectSelected={() => {
              if (window.innerWidth < 768) {
                setSidebarOpen(false);
              }
            }}
          />
        </div>

        {/* Navigation - Hidden on mobile as it's in the bottom bar */}
        <nav className="hidden md:block flex-1 p-2 space-y-1 overflow-y-auto">
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
            <div className="flex items-center gap-3 sm:gap-4 flex-1 min-w-0">
              
              {/* Mobile Menu Toggle */}
              <button
                onClick={toggleSidebar}
                className="md:hidden p-2 -ml-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                title="Toggle menu"
              >
                <Menu className="w-5 h-5" />
              </button>

              {/* Expand Toggle - Only visible when sidebar is collapsed */}
              {!sidebarOpen && (
                <button
                  onClick={toggleSidebar}
                  className="hidden md:flex p-2 -ml-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                  title="Expand sidebar"
                >
                  <PanelLeft className="w-5 h-5" />
                </button>
              )}

              {/* Breadcrumbs */}
              <div className="flex-1 min-w-0">
                <Breadcrumbs />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <a
                href="https://github.com/hugo-lorenzo-mato/quorum-ai"
                target="_blank"
                rel="noopener noreferrer"
                className="p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
                title="View on GitHub"
              >
                <Github className="w-5 h-5" />
              </a>
            </div>
          </div>
        </header>

        {/* Page content */}
        <div className={isFullLayoutPage ? '' : 'px-3 pt-3 pb-0 sm:p-6'}>
          {children}
        </div>
      </main>
      
      {/* Mobile Bottom Navigation */}
      <MobileBottomNav />

      <Notifications />
    </div>
  );
}
