import { useEffect, useState, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useWorkflowStore } from '../stores';
import { promptPresets } from '../data/promptPresets';
import { systemPromptsApi } from '../lib/api';
import { getStatusColor } from '../lib/theme';
import FAB from '../components/FAB';
import Logo from '../components/Logo';
import HeroSection from '../components/HeroSection';
import {
  GitBranch,
  CheckCircle2,
  XCircle,
  Zap,
  Clock,
  ArrowUpRight,
  Activity,
  Cpu,
  HardDrive,
  Timer,
  Server,
  RefreshCw,
  MemoryStick,
  Gauge,
  Box,
  Layers,
  MonitorDot,
  Thermometer,
  ChevronDown,
  ChevronUp,
  FileText,
  FileCode2,
  FolderKanban,
} from 'lucide-react';

// Get workflow display title
function getWorkflowTitle(workflow) {
  if (workflow.title) return workflow.title;
  if (workflow.prompt) {
    const firstLine = workflow.prompt.split('\n')[0].trim();
    const cleaned = firstLine.replace(/^(analyze|analiza|implement|implementa|create|crea|fix|arregla|update|actualiza|add|añade|you are|eres)\s+/i, '');
    return cleaned.substring(0, 80) || workflow.prompt.substring(0, 80);
  }
  return workflow.id;
}

// Bento Grid Card Component
function BentoCard({ children, className = '' }) {
  return (
    <div className={`group relative rounded-xl border border-border bg-card p-4 transition-all hover:border-muted-foreground/30 hover:shadow-lg animate-fade-up ${className}`}>
      {children}
    </div>
  );
}

// Compact Stat Card
function StatCard({ title, value, subtitle, icon: Icon, color = 'primary', className = '', to }) {
  const colorClasses = {
    primary: 'bg-primary/10 text-primary',
    success: 'bg-success/10 text-success',
    warning: 'bg-warning/10 text-warning',
    error: 'bg-error/10 text-error',
    info: 'bg-info/10 text-info',
  };

  const content = (
    <div className="flex items-start gap-3">
      <div className={`p-2 rounded-lg ${colorClasses[color]}`}>
        <Icon className="w-4 h-4" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-xl font-mono font-semibold text-foreground">{value}</p>
        <p className="text-xs font-medium text-muted-foreground truncate">{title}</p>
        {subtitle && (
          <p className="text-[10px] text-muted-foreground mt-0.5 truncate">{subtitle}</p>
        )}
      </div>
      {to && <ArrowUpRight className="w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />}
    </div>
  );

  if (to) {
    return (
      <Link to={to} className={`group relative rounded-xl border border-border bg-card p-4 transition-all hover:border-primary/50 hover:shadow-lg hover:bg-accent/30 animate-fade-up cursor-pointer ${className}`}>
        {content}
      </Link>
    );
  }

  return (
    <BentoCard className={className}>
      {content}
    </BentoCard>
  );
}

// Progress Bar Component
function ProgressBar({ value, max, color = 'primary', size = 'sm' }) {
  const percent = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  const heightClass = size === 'sm' ? 'h-1.5' : 'h-2';

  const colorClasses = {
    primary: 'bg-primary',
    success: 'bg-success',
    warning: 'bg-warning',
    error: 'bg-error',
    info: 'bg-info',
  };

  // Dynamic color based on percentage
  const getAutoColor = () => {
    if (percent < 60) return colorClasses.success;
    if (percent < 80) return colorClasses.warning;
    return colorClasses.error;
  };

  return (
    <div className={`w-full ${heightClass} bg-muted rounded-full overflow-hidden`}>
      <div
        className={`${heightClass} ${color === 'auto' ? getAutoColor() : colorClasses[color]} rounded-full transition-all duration-500`}
        style={{ width: `${percent}%` }}
      />
    </div>
  );
}

// Format uptime duration
function formatUptime(seconds) {
  if (!seconds || seconds < 0) return '0s';

  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m`;
  return `${Math.floor(seconds)}s`;
}

function buildStatusTooltip(data) {
  if (!data) return '';
  const lines = [];
  const warnings = Array.isArray(data.warnings) ? data.warnings : [];
  const trendWarnings = Array.isArray(data.trend?.Warnings) ? data.trend.Warnings : [];

  if (warnings.length > 0) {
    lines.push('Warnings:');
    warnings.forEach((warning) => {
      const levelRaw = warning?.Level || warning?.level;
      const level = levelRaw ? String(levelRaw).toUpperCase() : 'WARNING';
      const message = warning?.Message || warning?.message || 'Unknown warning';
      lines.push(`${level}: ${message}`);
    });
  }

  if (trendWarnings.length > 0) {
    if (lines.length > 0) lines.push('');
    lines.push('Trend:');
    trendWarnings.forEach((message) => {
      lines.push(message || 'Unknown trend warning');
    });
  }

  return lines.join('\n');
}

// Metric Row Component for compact display
function MetricRow({ icon: Icon, label, value, subtext, progress, color = 'primary' }) {
  return (
    <div className="flex items-center gap-3">
      <Icon className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline justify-between gap-2">
          <span className="text-xs text-muted-foreground">{label}</span>
          <span className="text-sm font-mono font-medium text-foreground">{value}</span>
        </div>
        {progress !== undefined && (
          <ProgressBar value={progress} max={100} color={color} size="sm" />
        )}
        {subtext && (
          <p className="text-[10px] text-muted-foreground mt-0.5">{subtext}</p>
        )}
      </div>
    </div>
  );
}

// System Resources Card - Complete view with Process, Machine, and Hardware info
function SystemResources({ data, loading, onRefresh, timeAgo }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [isMobile, setIsMobile] = useState(() => window.innerWidth < 768);

  // Auto-expand on desktop
  useEffect(() => {
    const updateLayout = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      if (!mobile) {
        setIsExpanded(true);
      }
    };
    updateLayout();
    window.addEventListener('resize', updateLayout);
    return () => window.removeEventListener('resize', updateLayout);
  }, []);

  if (loading && !data) {
    return (
      <BentoCard className="md:col-span-2">
        <div className="animate-pulse">
          <div className="h-4 bg-muted rounded w-32 mb-4" />
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-3">
              {[...Array(4)].map((_, i) => (
                <div key={i} className="h-8 bg-muted rounded" />
              ))}
            </div>
            <div className="space-y-3">
              {[...Array(4)].map((_, i) => (
                <div key={i} className="h-8 bg-muted rounded" />
              ))}
            </div>
          </div>
        </div>
      </BentoCard>
    );
  }

  if (!data?.system) {
    return (
      <BentoCard className="md:col-span-2">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-foreground flex items-center gap-2">
            <Server className="w-4 h-4 text-muted-foreground" />
            System Resources
          </h3>
        </div>
        <p className="text-xs text-muted-foreground">Unable to load system metrics</p>
      </BentoCard>
    );
  }

  const { system, resources } = data;
  const uptime = resources?.process_uptime ? resources.process_uptime / 1e9 : 0;
  const heapMB = resources?.heap_alloc_mb || 0;
  const goroutines = resources?.goroutines || 0;
  const statusTooltip = buildStatusTooltip(data);

  // Format load average
  const loadAvg = system.load_avg_1 !== undefined
    ? `${system.load_avg_1?.toFixed(2)} / ${system.load_avg_5?.toFixed(2)} / ${system.load_avg_15?.toFixed(2)}`
    : 'n/a';

  // GPU info
  const gpus = system.gpu_infos || [];

  return (
    <BentoCard className="md:col-span-2 transition-all duration-300">
      {/* Header */}
      <div className="flex items-center justify-between mb-1 md:mb-4 py-1 md:py-0">
        <button
          type="button"
          disabled={!isMobile}
          onClick={() => setIsExpanded((v) => !v)}
          aria-expanded={isExpanded}
          aria-label="Toggle system resources"
          className="flex items-center gap-2 md:gap-0 flex-1 min-w-0 text-left bg-transparent border-0 p-0 appearance-none disabled:cursor-default"
        >
          <h3 className="text-sm font-medium text-foreground flex items-center gap-2">
            <Server className="w-4 h-4 text-muted-foreground" />
            System Resources
          </h3>
          {/* Mobile summary when collapsed */}
          {!isExpanded && (
            <span className="md:hidden text-xs text-muted-foreground ml-2 truncate">
              CPU: {system.cpu_percent?.toFixed(0)}% · RAM: {(system.mem_used_mb / 1024)?.toFixed(1)}GB
            </span>
          )}
        </button>

        <div className="flex items-center gap-2">
          {timeAgo && (
            <span className="text-[10px] text-muted-foreground hidden sm:inline">{timeAgo}</span>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              onRefresh();
            }}
            className="p-1.5 rounded-lg hover:bg-muted transition-colors text-muted-foreground hover:text-foreground z-10"
            title="Refresh"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          {/* Mobile Chevron */}
          {isMobile && (
            <button
              type="button"
              onClick={() => setIsExpanded((v) => !v)}
              className="md:hidden text-muted-foreground ml-1 bg-transparent border-0 p-0 appearance-none"
              aria-label="Toggle system resources"
            >
              {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </button>
          )}
        </div>
      </div>

      <div className={`${isExpanded ? 'block animate-fade-in' : 'hidden md:block'}`}> 
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4 mt-4 md:mt-0">
          {/* Quorum Process Section */}
          <div className="space-y-3">
            <h4 className="text-xs font-semibold text-info flex items-center gap-1.5 uppercase tracking-wide">
              <Box className="w-3 h-3" />
              Quorum Process
            </h4>

            <MetricRow
              icon={MemoryStick}
              label="Heap Memory"
              value={`${heapMB.toFixed(1)} MB`}
              progress={Math.min(heapMB / 512 * 100, 100)}
              color="auto"
            />

            <MetricRow
              icon={Layers}
              label="Goroutines"
              value={goroutines.toString()}
            />

            <MetricRow
              icon={Timer}
              label="Uptime"
              value={formatUptime(uptime)}
            />
          </div>

          {/* Machine Section */}
          <div className="space-y-3">
            <h4 className="text-xs font-semibold text-warning flex items-center gap-1.5 uppercase tracking-wide">
              <MonitorDot className="w-3 h-3" />
              Machine
            </h4>

            <MetricRow
              icon={Cpu}
              label="CPU"
              value={`${system.cpu_percent?.toFixed(1) || 0}%`}
              subtext={`${system.cpu_cores || 0}C / ${system.cpu_threads || 0}T`}
              progress={system.cpu_percent || 0}
              color="auto"
            />

            <MetricRow
              icon={MemoryStick}
              label="RAM"
              value={`${(system.mem_used_mb / 1024)?.toFixed(1) || 0} / ${(system.mem_total_mb / 1024)?.toFixed(1) || 0} GB`}
              progress={system.mem_percent || 0}
              color="auto"
            />

            <MetricRow
              icon={HardDrive}
              label="Disk"
              value={`${system.disk_used_gb?.toFixed(0) || 0} / ${system.disk_total_gb?.toFixed(0) || 0} GB`}
              progress={system.disk_percent || 0}
              color="auto"
            />

            <MetricRow
              icon={Gauge}
              label="Load Avg"
              value={loadAvg}
            />
          </div>
        </div>

        {/* Hardware Info Section */}
        <div className="mt-4 pt-3 border-t border-border">
          <h4 className="text-xs font-semibold text-muted-foreground mb-2 uppercase tracking-wide">
            Hardware
          </h4>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs">
            <div className="flex items-center gap-2 text-muted-foreground">
              <Cpu className="w-3 h-3" />
              <span className="truncate" title={system.cpu_model || 'Unknown'}>
                {system.cpu_model || 'Unknown CPU'}
              </span>
            </div>
            <div className="flex items-center gap-2 text-muted-foreground">
              <MemoryStick className="w-3 h-3" />
              <span>{(system.mem_total_mb / 1024)?.toFixed(1) || 0} GB RAM</span>
            </div>
            {gpus.length > 0 ? (
              gpus.map((gpu, i) => (
                <div key={i} className="flex items-center gap-2 text-muted-foreground md:col-span-2">
                  <MonitorDot className="w-3 h-3" />
                  <span className="truncate" title={gpu.name}>
                    {gpu.name}
                    {gpu.util_valid && ` · ${gpu.util_percent?.toFixed(0)}%`}
                    {gpu.mem_valid && ` · ${(gpu.mem_used_mb / 1024)?.toFixed(1)}/${(gpu.mem_total_mb / 1024)?.toFixed(1)} GB`}
                    {gpu.temp_valid && (
                      <span className="inline-flex items-center gap-0.5 ml-1">
                        <Thermometer className="w-2.5 h-2.5" />
                        {gpu.temp_c?.toFixed(0)}°C
                      </span>
                    )}
                  </span>
                </div>
              ))
            ) : (
              <div className="flex items-center gap-2 text-muted-foreground">
                <MonitorDot className="w-3 h-3" />
                <span>No GPU detected</span>
              </div>
            )}
          </div>
        </div>

        {/* Status warning */}
        {data.status && data.status !== 'healthy' && (
          <div
            className={`mt-3 text-xs px-2 py-1 rounded cursor-help ${ 
              data.status === 'critical'
                ? 'bg-error/10 text-error'
                : 'bg-warning/10 text-warning'
            }`}
            title={statusTooltip || undefined}
          >
            Status: {data.status}
          </div>
        )}
      </div>
    </BentoCard>
  );
}

// Recent Workflow Item
function WorkflowItem({ workflow }) {
  const statusConfig = {
    pending: { icon: Clock },
    running: { icon: Activity },
    completed: { icon: CheckCircle2 },
    failed: { icon: XCircle },
  };

  const config = statusConfig[workflow.status] || statusConfig.pending;
  const StatusIcon = config.icon;
  const statusColor = getStatusColor(workflow.status);

  return (
    <Link
      to={`/workflows/${workflow.id}`}
      className="group flex items-center gap-3 p-2.5 -mx-2.5 rounded-lg transition-colors hover:bg-accent"
    >
      <div className={`p-1.5 rounded-lg ${statusColor.bg}`}>
        <StatusIcon className={`w-3.5 h-3.5 ${statusColor.text}`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground truncate">
          {getWorkflowTitle(workflow)}
        </p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-muted-foreground font-mono bg-muted/50 px-1.5 rounded">
            {workflow.id.substring(0, 8)}
          </span>
          <span className="text-xs text-muted-foreground">
            · {workflow.task_count || 0} tasks
          </span>
        </div>
      </div>
      <div className={`text-xs font-medium px-2 py-0.5 rounded-full ${statusColor.bg} ${statusColor.text}`}>
        {workflow.status}
      </div>
    </Link>
  );
}

// Active Workflow Banner
function ActiveWorkflowBanner({ workflow }) {
  if (!workflow) return null;

  return (
    <div className="rounded-xl border border-info/20 bg-gradient-to-r from-info/5 to-primary/5 p-4 animate-fade-up">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="relative flex-shrink-0">
            <div className="p-2.5 rounded-xl bg-info/10">
              <Zap className="w-4 h-4 text-info" />
            </div>
            <span className="absolute -top-0.5 -right-0.5 w-2.5 h-2.5 bg-info rounded-full animate-pulse" />
          </div>
          <div className="min-w-0">
            <p className="text-xs font-medium text-muted-foreground">Active Workflow</p>
            <p className="text-sm font-semibold text-foreground truncate">
              {getWorkflowTitle(workflow)}
            </p>
            <p className="text-xs text-muted-foreground mt-0.5">
              Phase: {workflow.current_phase} · {workflow.task_count || 0} tasks
            </p>
          </div>
        </div>
        <Link
          to={`/workflows/${workflow.id}`}
          className="flex-shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 transition-colors"
        >
          <span className="hidden sm:inline">View</span>
          <ArrowUpRight className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}
// Empty State with Large Logo
function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="relative mb-6">
        <div className="absolute inset-0 bg-primary/20 blur-xl rounded-full" />
        <div className="relative p-6 rounded-3xl bg-card border border-border shadow-sm">
          <Logo className="w-16 h-16" />
        </div>
      </div>
      <h3 className="text-lg font-semibold text-foreground mb-2">Welcome to Quorum AI</h3>
      <p className="text-sm text-muted-foreground mb-6 max-w-sm mx-auto leading-relaxed">
        Your intelligent consensus engine is ready. Create your first workflow to orchestrate AI agents.
      </p>
      <div className="flex flex-col sm:flex-row items-center gap-3">
        <Link
          to="/workflows/new"
          className="w-full sm:w-auto px-5 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-all flex items-center justify-center gap-2 shadow-lg shadow-primary/20"
        >
          <Zap className="w-4 h-4" />
          Start Workflow
        </Link>
        <Link
          to="/prompts"
          className="w-full sm:w-auto px-5 py-2.5 rounded-xl border border-border text-foreground text-sm font-medium hover:bg-accent transition-all flex items-center justify-center gap-2"
        >
          <FileText className="w-4 h-4" />
          Browse Prompts
        </Link>
      </div>
    </div>
  );
}

// Loading Skeleton
function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="h-20 bg-muted/50 rounded-xl animate-pulse w-1/3 mb-8" />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="h-24 rounded-xl bg-muted/50 animate-pulse" />
        ))}
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="md:col-span-2 h-64 rounded-xl bg-muted/50 animate-pulse" />
        <div className="h-64 rounded-xl bg-muted/50 animate-pulse" />
      </div>
    </div>
  );
}
function useProjects() {
  const [projects, setProjects] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchProjects = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/projects/');
      if (response.ok) {
        const result = await response.json();
        setProjects(result || []);
      }
    } catch (error) {
      console.error('Failed to fetch projects:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  return { projects, loading, refresh: fetchProjects };
}

function useSystemPrompts() {
  const [count, setCount] = useState(0);

  useEffect(() => {
    systemPromptsApi.list()
      .then((data) => setCount(Array.isArray(data) ? data.length : 0))
      .catch(() => {});
  }, []);

  return count;
}

function useSystemResources() {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState(null);
  const [timeAgo, setTimeAgo] = useState('');

  const fetchResources = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetch('/health/deep');
      if (response.ok) {
        const result = await response.json();
        setData(result);
        setLastUpdate(Date.now());
      }
    } catch (error) {
      console.error('Failed to fetch system resources:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  // Auto-fetch every 30 seconds
  useEffect(() => {
    fetchResources();
    const interval = setInterval(fetchResources, 30000);
    return () => clearInterval(interval);
  }, [fetchResources]);

  // Update "time ago" text every 5 seconds
  useEffect(() => {
    const updateTimeAgo = () => {
      if (!lastUpdate) {
        setTimeAgo('');
        return;
      }
      const seconds = Math.floor((Date.now() - lastUpdate) / 1000);
      if (seconds < 5) {
        setTimeAgo('just now');
      } else if (seconds < 60) {
        setTimeAgo(`${seconds}s ago`);
      } else {
        const minutes = Math.floor(seconds / 60);
        setTimeAgo(`${minutes}m ago`);
      }
    };

    updateTimeAgo();
    const interval = setInterval(updateTimeAgo, 5000);
    return () => clearInterval(interval);
  }, [lastUpdate]);

  return { data, loading, refresh: fetchResources, timeAgo };
}

export default function Dashboard() {
  const { workflows, activeWorkflow, fetchWorkflows, fetchActiveWorkflow, loading } = useWorkflowStore();
  const { data: systemData, loading: systemLoading, refresh: refreshSystem, timeAgo: systemTimeAgo } = useSystemResources();
  const { projects } = useProjects();
  const systemPromptsCount = useSystemPrompts();
  const navigate = useNavigate();

  useEffect(() => {
    fetchWorkflows();
    fetchActiveWorkflow();
  }, [fetchWorkflows, fetchActiveWorkflow]);

  const completedCount = workflows.filter(w => w.status === 'completed').length;
  const runningCount = workflows.filter(w => w.status === 'running').length;
  const failedCount = workflows.filter(w => w.status === 'failed').length;
  const healthyProjectsCount = projects.filter(p => p.status === 'healthy').length;

  const recentWorkflows = [...workflows]
    .sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
    .slice(0, 5);

  if (loading && workflows.length === 0) {
    return <LoadingSkeleton />;
  }

  return (
    <div className="space-y-6 animate-fade-in pb-10">
      {/* Brand Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-border/50 pb-6">
        <div className="flex items-center gap-4">
          <div className="p-3 bg-gradient-to-br from-primary/10 to-primary/5 rounded-2xl border border-primary/10 shadow-sm">
             <Logo className="w-10 h-10" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">
              Quorum AI
            </h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              Consensus Engine Dashboard
            </p>
          </div>
        </div>
        
        <div className="flex items-center gap-3">
          <Link
            to="/workflows/new"
            className="hidden md:flex items-center gap-2 px-4 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-all shadow-sm hover:shadow-md"
          >
            <Zap className="w-4 h-4" />
            New Workflow
          </Link>
        </div>
      </div>

      {/* Hero Section with Animated Flow Diagram */}
      {workflows.length === 0 && <HeroSection />}

      {/* Active Workflow Banner */}
      {activeWorkflow && activeWorkflow.status === 'running' && (
        <ActiveWorkflowBanner workflow={activeWorkflow} />
      )}

      {/* Stats Grid - Mobile Carousel, Desktop Grid */}
      <div className="flex overflow-x-auto pb-4 -mx-3 px-3 sm:mx-0 sm:px-0 gap-3 snap-x md:grid md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7 md:gap-4 md:overflow-visible md:pb-0 scrollbar-none md:scrollbar-default">
        <StatCard
          title="Projects"
          value={projects.length}
          subtitle={`${healthyProjectsCount} healthy`}
          icon={FolderKanban}
          color="primary"
          to="/projects"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="Prompts"
          value={promptPresets.length}
          subtitle="Presets"
          icon={FileText}
          color="primary"
          to="/prompts"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="System Prompts"
          value={systemPromptsCount}
          subtitle="Embedded"
          icon={FileCode2}
          color="primary"
          to="/system-prompts"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="Workflows"
          value={workflows.length}
          subtitle="All time"
          icon={GitBranch}
          color="primary"
          to="/workflows"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="Completed"
          value={completedCount}
          subtitle={`${Math.round((completedCount / Math.max(workflows.length, 1)) * 100)}% success`}
          icon={CheckCircle2}
          color="success"
          to="/workflows?status=completed"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="Running"
          value={runningCount}
          subtitle="Active now"
          icon={Activity}
          color="info"
          to="/workflows?status=running"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
        <StatCard
          title="Failed"
          value={failedCount}
          subtitle="Needs attention"
          icon={XCircle}
          color="error"
          to="/workflows?status=failed"
          className="min-w-[160px] md:min-w-0 snap-center h-full"
        />
      </div>

      {/* System Resources + Recent Workflows */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* System Resources */}
        <SystemResources
          data={systemData}
          loading={systemLoading}
          onRefresh={refreshSystem}
          timeAgo={systemTimeAgo}
        />

        {/* Recent Workflows */}
        <BentoCard>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-sm font-semibold text-foreground">Recent Activity</h2>
              <p className="text-xs text-muted-foreground">Latest executions</p>
            </div>
            <Link
              to="/workflows"
              className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
            >
              View all
              <ArrowUpRight className="w-3.5 h-3.5" />
            </Link>
          </div>

          {recentWorkflows.length > 0 ? (
            <div className="space-y-1">
              {recentWorkflows.map((workflow) => (
                <WorkflowItem key={workflow.id} workflow={workflow} />
              ))}
            </div>
          ) : (
            <EmptyState />
          )}
        </BentoCard>
      </div>

      {/* Mobile FAB */}
      <FAB onClick={() => navigate('/workflows/new')} icon={Zap} label="New Workflow" />
    </div>
  );
}
