import { useEffect, useState, useCallback, useMemo } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useWorkflowStore } from '../stores';
import { getStatusColor } from '../lib/theme';
import FAB from '../components/FAB';
import Logo from '../components/Logo';
import { Badge } from '../components/ui/Badge';
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
  FolderKanban,
  LayoutDashboard,
  Sparkles,
  ChevronRight,
  Info
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
    <div className={`group relative rounded-xl border border-border bg-card/50 backdrop-blur-sm p-5 transition-all duration-300 hover:shadow-xl hover:border-primary/30 animate-fade-up ${className}`}>
      {children}
    </div>
  );
}

// Compact Stat Card
function StatCard({ title, value, subtitle, icon: Icon, color = 'primary', className = '', to }) {
  const colorClasses = {
    primary: 'text-primary bg-primary/10 border-primary/20',
    success: 'text-success bg-success/10 border-success/20',
    warning: 'text-warning bg-warning/10 border-warning/20',
    error: 'text-error bg-error/10 border-error/20',
    info: 'text-info bg-info/10 border-info/20',
  };

  const content = (
    <div className="flex items-start justify-between gap-3">
      <div className="space-y-3 flex-1 min-w-0">
        <div className={`inline-flex p-2 rounded-lg ${colorClasses[color]} border shadow-sm`}>
          <Icon className="w-4 h-4" />
        </div>
        <div>
          <p className="text-2xl font-bold text-foreground tracking-tight">{value}</p>
          <p className="text-[10px] uppercase font-black tracking-widest text-muted-foreground opacity-70 group-hover:opacity-100 transition-opacity">{title}</p>
        </div>
        {subtitle && (
          <p className="text-xs text-muted-foreground line-clamp-1">{subtitle}</p>
        )}
      </div>
      {to && <ArrowUpRight className="w-4 h-4 text-muted-foreground/50 group-hover:text-primary transition-colors mt-1" />}
    </div>
  );

  if (to) {
    return (
      <Link to={to} className={`group relative rounded-xl border border-border bg-card/50 backdrop-blur-sm p-5 transition-all duration-300 hover:shadow-xl hover:border-primary/30 hover:-translate-y-1 animate-fade-up cursor-pointer ${className}`}>
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

// System Resources Card
function SystemResources({ data, loading, onRefresh, timeAgo }) {
  const [isExpanded, setIsExpanded] = useState(false);

  // Auto-expand on desktop
  useEffect(() => {
    const checkWidth = () => {
      if (window.innerWidth >= 768) {
        setIsExpanded(true);
      }
    };
    checkWidth();
    window.addEventListener('resize', checkWidth);
    return () => window.removeEventListener('resize', checkWidth);
  }, []);

  if (loading && !data) {
    return (
      <BentoCard className="md:col-span-2">
        <div className="animate-pulse flex flex-col h-full">
          <div className="h-4 bg-muted rounded w-32 mb-6" />
          <div className="grid grid-cols-1 md:grid-cols-2 gap-8 flex-1">
            <div className="space-y-4">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="h-10 bg-muted rounded" />
              ))}
            </div>
            <div className="space-y-4">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="h-10 bg-muted rounded" />
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
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-2">
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
      <div 
        className="flex items-center justify-between mb-1 md:mb-6 cursor-pointer md:cursor-default py-1 md:py-0"
        onClick={() => window.innerWidth < 768 && setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2 md:gap-0 flex-1">
          <h3 className="text-sm font-bold uppercase tracking-widest text-muted-foreground flex items-center gap-2">
            <Server className="w-4 h-4" />
            Health Metrics
          </h3>
          {/* Mobile summary when collapsed */}
          {!isExpanded && (
            <span className="md:hidden text-xs text-muted-foreground ml-2 truncate">
              CPU: {system.cpu_percent?.toFixed(0)}% · RAM: {(system.mem_used_mb / 1024)?.toFixed(1)}GB
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {timeAgo && (
            <span className="text-[10px] font-bold text-muted-foreground uppercase tracking-wider hidden sm:inline">{timeAgo}</span>
          )}
          <button
            onClick={(e) => {
              e.stopPropagation();
              onRefresh();
            }}
            className="p-1.5 rounded-lg hover:bg-accent transition-colors text-muted-foreground hover:text-foreground z-10"
            title="Refresh"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          {/* Mobile Chevron */}
          <div className="md:hidden text-muted-foreground ml-1">
            {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </div>
        </div>
      </div>

      <div className={`${isExpanded ? 'block animate-fade-in' : 'hidden md:block'}`}> 
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-10 gap-y-6 mt-4 md:mt-0">
          {/* Quorum Process Section */}
          <div className="space-y-4">
            <h4 className="text-[10px] font-black text-primary flex items-center gap-1.5 uppercase tracking-[0.2em] opacity-80">
              <Box className="w-3 h-3" />
              Engine Stats
            </h4>

            <MetricRow
              icon={MemoryStick}
              label="Heap Memory"
              value={`${heapMB.toFixed(1)} MB`}
              progress={Math.min(heapMB / 512 * 100, 100)}
              color="primary"
            />

            <MetricRow
              icon={Layers}
              label="Goroutines"
              value={goroutines.toString()}
            />

            <MetricRow
              icon={Timer}
              label="Process Uptime"
              value={formatUptime(uptime)}
            />
          </div>

          {/* Machine Section */}
          <div className="space-y-4">
            <h4 className="text-[10px] font-black text-warning flex items-center gap-1.5 uppercase tracking-[0.2em] opacity-80">
              <MonitorDot className="w-3 h-3" />
              Infrastructure
            </h4>

            <MetricRow
              icon={Cpu}
              label="CPU Usage"
              value={`${system.cpu_percent?.toFixed(1) || 0}%`}
              subtext={`${system.cpu_cores || 0} Cores / ${system.cpu_threads || 0} Threads`}
              progress={system.cpu_percent || 0}
              color="warning"
            />

            <MetricRow
              icon={MemoryStick}
              label="System RAM"
              value={`${(system.mem_used_mb / 1024)?.toFixed(1) || 0} / ${(system.mem_total_mb / 1024)?.toFixed(1) || 0} GB`}
              progress={system.mem_percent || 0}
              color="warning"
            />

            <MetricRow
              icon={HardDrive}
              label="Disk Space"
              value={`${system.disk_used_gb?.toFixed(0) || 0} / ${system.disk_total_gb?.toFixed(0) || 0} GB`}
              progress={system.disk_percent || 0}
              color="warning"
            />
          </div>
        </div>

        {/* Hardware Info Section */}
        <div className="mt-6 pt-4 border-t border-border/50">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
            <div className="space-y-2">
               <h5 className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/60">Processor</h5>
               <div className="flex items-center gap-2 text-muted-foreground font-medium">
                  <Cpu className="w-3.5 h-3.5" />
                  <span className="truncate" title={system.cpu_model || 'Unknown'}>
                    {system.cpu_model || 'Unknown CPU'}
                  </span>
               </div>
            </div>
            <div className="space-y-2">
               <h5 className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/60">GPU Accelerators</h5>
               {gpus.length > 0 ? (
                  gpus.map((gpu, i) => (
                    <div key={i} className="flex items-center gap-2 text-muted-foreground font-medium">
                      <MonitorDot className="w-3.5 h-3.5" />
                      <span className="truncate" title={gpu.name}>
                        {gpu.name}
                        {gpu.util_valid && ` · ${gpu.util_percent?.toFixed(0)}%`}
                        {gpu.temp_valid && (
                          <span className="inline-flex items-center gap-0.5 ml-1.5 px-1.5 py-0.5 rounded bg-orange-500/10 text-orange-500 text-[10px]">
                            <Thermometer className="w-2.5 h-2.5" />
                            {gpu.temp_c?.toFixed(0)}°C
                          </span>
                        )}
                      </span>
                    </div>
                  ))
                ) : (
                  <div className="flex items-center gap-2 text-muted-foreground/50 font-medium">
                    <MonitorDot className="w-3.5 h-3.5" />
                    <span>No GPU detected</span>
                  </div>
                )}
            </div>
          </div>
        </div>

        {/* Status warning */}
        {data.status && data.status !== 'healthy' && (
          <div
            className={`mt-4 text-[10px] font-bold uppercase tracking-widest px-3 py-2 rounded-lg cursor-help flex items-center gap-2 ${ 
              data.status === 'critical'
                ? 'bg-error/10 text-error border border-error/20'
                : 'bg-warning/10 text-warning border border-warning/20'
            }`}
            title={statusTooltip || undefined}
          >
            <AlertCircle className="w-3.5 h-3.5" />
            System Status: {data.status}
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
      className="group flex items-center gap-3 p-3 rounded-xl transition-all hover:bg-accent/50 border border-transparent hover:border-border/50"
    >
      <div className={`p-2 rounded-lg ${statusColor.bg} border ${statusColor.border} shadow-sm group-hover:scale-110 transition-transform`}>
        <StatusIcon className={`w-4 h-4 ${statusColor.text}`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-bold text-foreground truncate group-hover:text-primary transition-colors">
          {getWorkflowTitle(workflow)}
        </p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-[10px] text-muted-foreground font-mono bg-muted/50 px-1.5 py-0.5 rounded border border-border/30">
            {workflow.id.substring(0, 8)}
          </span>
          <span className="text-[10px] font-bold text-muted-foreground/60 uppercase tracking-tight">
            · {workflow.task_count || 0} tasks
          </span>
        </div>
      </div>
      <ChevronRight className="w-4 h-4 text-muted-foreground/30 group-hover:text-primary transition-colors" />
    </Link>
  );
}

// Active Workflow Banner
function ActiveWorkflowBanner({ workflow }) {
  if (!workflow) return null;

  return (
    <div className="relative overflow-hidden rounded-2xl border border-primary/20 bg-primary/5 p-5 animate-fade-up shadow-sm">
      <div className="absolute top-0 right-0 p-1">
         <div className="w-24 h-24 bg-primary/10 rounded-full blur-2xl -mr-12 -mt-12" />
      </div>
      
      <div className="relative flex items-center justify-between gap-4">
        <div className="flex items-center gap-4 min-w-0">
          <div className="relative flex-shrink-0">
            <div className="p-3 rounded-2xl bg-primary/10 border border-primary/20 shadow-sm">
              <Zap className="w-5 h-5 text-primary" />
            </div>
            <span className="absolute -top-1 -right-1 w-3 h-3 bg-primary rounded-full border-2 border-background animate-pulse" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
               <p className="text-[10px] font-black uppercase tracking-[0.2em] text-primary">Active Execution</p>
               <Badge variant="secondary" className="text-[9px] py-0 bg-primary/10 text-primary border-transparent">
                  {workflow.current_phase}
               </Badge>
            </div>
            <p className="text-base font-bold text-foreground truncate mt-0.5">
              {getWorkflowTitle(workflow)}
            </p>
          </div>
        </div>
        <Link
          to={`/workflows/${workflow.id}`}
          className="flex-shrink-0 flex items-center gap-2 px-4 py-2 rounded-xl bg-primary text-primary-foreground text-xs font-bold hover:bg-primary/90 transition-all shadow-lg shadow-primary/20"
        >
          Manage
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
        <div className="absolute inset-0 bg-primary/20 blur-2xl rounded-full" />
        <div className="relative p-6 rounded-3xl bg-background border border-border shadow-sm">
          <Logo className="w-16 h-16" />
        </div>
      </div>
      <h3 className="text-xl font-bold text-foreground mb-2">Initialize Your First Workflow</h3>
      <p className="text-sm text-muted-foreground mb-8 max-w-sm mx-auto leading-relaxed">
        Quorum AI is ready to orchestrate your multi-agent development cycles. Start by choosing a template or creating a custom workflow.
      </p>
      <div className="flex flex-col sm:flex-row items-center gap-4">
        <Link
          to="/templates"
          className="w-full sm:w-auto px-6 py-2.5 rounded-xl border border-border bg-card/50 text-foreground text-sm font-bold hover:bg-accent transition-all flex items-center justify-center gap-2 shadow-sm"
        >
          <FileText className="w-4 h-4" />
          Browse Templates
        </Link>
        <Link
          to="/workflows/new"
          className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-bold hover:bg-primary/90 transition-all flex items-center justify-center gap-2 shadow-lg shadow-primary/30"
        >
          <Zap className="w-4 h-4" />
          New Workflow
        </Link>
      </div>
    </div>
  );
}

// Loading Skeleton
function LoadingSkeleton() {
  return (
    <div className="space-y-8 animate-pulse">
      <div className="h-48 bg-muted/30 rounded-3xl w-full" />
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="h-32 rounded-2xl bg-muted/20" />
        ))}
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="md:col-span-2 h-80 rounded-2xl bg-muted/20" />
        <div className="h-80 rounded-2xl bg-muted/20" />
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
    return (
      <div className="relative min-h-full">
         <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
         <LoadingSkeleton />
      </div>
    );
  }

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      {/* Background Pattern - Global Consistency */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header - Unified Style */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/30 backdrop-blur-md p-8 sm:p-12 shadow-sm">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-8">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
              <LayoutDashboard className="h-3 w-3" />
              Command Center
            </div>
            <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
              Quorum <span className="text-primary">Dashboard</span>
            </h1>
            <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
              Real-time monitoring of your multi-agent ecosystem. Orchestrate, analyze, and scale your AI-driven development workflows.
            </p>
          </div>

          <div className="flex flex-col gap-3 shrink-0">
             <Link
                to="/workflows/new"
                className="flex items-center justify-center gap-2 px-6 py-3 rounded-2xl bg-primary text-primary-foreground text-sm font-bold hover:bg-primary/90 transition-all shadow-xl shadow-primary/20"
              >
                <Zap className="w-4 h-4" />
                Start Workflow
              </Link>
              <button 
                onClick={refreshSystem}
                className="flex items-center justify-center gap-2 px-6 py-3 rounded-2xl bg-card border border-border text-muted-foreground text-sm font-bold hover:text-foreground hover:bg-accent transition-all shadow-sm"
              >
                <RefreshCw className={`w-4 h-4 ${systemLoading ? 'animate-spin' : ''}`} />
                Sync Engine
              </button>
          </div>
        </div>
      </div>

      {/* Active Workflow Banner */}
      {activeWorkflow && activeWorkflow.status === 'running' && (
        <ActiveWorkflowBanner workflow={activeWorkflow} />
      )}

      {/* Stats Grid - Consistent with /projects & /templates cards */}
      <div className="flex overflow-x-auto pb-4 -mx-3 px-3 sm:mx-0 sm:px-0 gap-4 snap-x md:grid md:grid-cols-2 lg:grid-cols-5 md:gap-6 md:overflow-visible md:pb-0 scrollbar-none">
        <StatCard
          title="Workspaces"
          value={projects.length}
          subtitle={`${healthyProjectsCount} healthy environments`}
          icon={FolderKanban}
          color="primary"
          to="/projects"
          className="min-w-[200px] md:min-w-0 snap-center"
        />
        <StatCard
          title="Total Workflows"
          value={workflows.length}
          subtitle="All execution history"
          icon={GitBranch}
          color="primary"
          to="/workflows"
          className="min-w-[200px] md:min-w-0 snap-center"
        />
        <StatCard
          title="Success Rate"
          value={completedCount}
          subtitle={`${Math.round((completedCount / Math.max(workflows.length, 1)) * 100)}% overall success`}
          icon={CheckCircle2}
          color="success"
          to="/workflows?status=completed"
          className="min-w-[200px] md:min-w-0 snap-center"
        />
        <StatCard
          title="Active Runs"
          value={runningCount}
          subtitle="Currently processing"
          icon={Activity}
          color="info"
          to="/workflows?status=running"
          className="min-w-[200px] md:min-w-0 snap-center"
        />
        <StatCard
          title="Failed Tasks"
          value={failedCount}
          subtitle="Attention required"
          icon={XCircle}
          color="error"
          to="/workflows?status=failed"
          className="min-w-[200px] md:min-w-0 snap-center"
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        {/* System Resources (Metric heavy) */}
        <SystemResources
          data={systemData}
          loading={systemLoading}
          onRefresh={refreshSystem}
          timeAgo={systemTimeAgo}
        />

        {/* Recent Workflows (Activity heavy) */}
        <BentoCard className="flex flex-col">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
               <div className="p-2 rounded-lg bg-secondary text-foreground">
                  <Activity className="w-4 h-4" />
               </div>
               <div>
                  <h2 className="text-sm font-bold uppercase tracking-widest text-muted-foreground">Recent Activity</h2>
                  <p className="text-[10px] text-muted-foreground/60 font-bold">LATEST EXECUTIONS</p>
               </div>
            </div>
            <Link
              to="/workflows"
              className="p-2 rounded-lg hover:bg-accent text-muted-foreground hover:text-primary transition-colors"
              title="View all workflows"
            >
              <ArrowUpRight className="w-4 h-4" />
            </Link>
          </div>

          {recentWorkflows.length > 0 ? (
            <div className="space-y-2 flex-1">
              {recentWorkflows.map((workflow) => (
                <WorkflowItem key={workflow.id} workflow={workflow} />
              ))}
            </div>
          ) : (
            <EmptyState />
          )}
          
          <div className="mt-6 pt-4 border-t border-border/50">
             <Link 
               to="/kanban" 
               className="flex items-center justify-between p-3 rounded-xl bg-primary/5 border border-primary/10 hover:bg-primary/10 transition-colors group"
             >
                <div className="flex items-center gap-3">
                   <div className="p-1.5 rounded-lg bg-primary/10 text-primary">
                      <FolderKanban className="w-4 h-4" />
                   </div>
                   <span className="text-xs font-bold text-foreground group-hover:text-primary">Open Kanban Board</span>
                </div>
                <ChevronRight className="w-4 h-4 text-muted-foreground/40 group-hover:text-primary" />
             </Link>
          </div>
        </BentoCard>
      </div>

      {/* Mobile FAB */}
      <FAB onClick={() => navigate('/workflows/new')} icon={Zap} label="New Workflow" />
    </div>
  );
}