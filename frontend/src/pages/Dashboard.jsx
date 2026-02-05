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

// Bento Grid Card Component - Refined
function BentoCard({ children, className = '' }) {
  return (
    <div className={`group relative rounded-2xl border border-border/40 bg-card/40 backdrop-blur-md p-6 transition-all duration-500 hover:border-primary/20 hover:shadow-[0_8px_30px_rgb(0,0,0,0.02)] animate-fade-up ${className}`}>
      {children}
    </div>
  );
}

// Compact Stat Card - Refined
function StatCard({ title, value, subtitle, icon: Icon, color = 'primary', className = '', to }) {
  const colorClasses = {
    primary: 'text-primary bg-primary/5 border-primary/10',
    success: 'text-success bg-success/5 border-success/10',
    warning: 'text-warning bg-warning/5 border-warning/10',
    error: 'text-error bg-error/5 border-error/10',
    info: 'text-info bg-info/5 border-info/10',
  };

  const content = (
    <div className="flex flex-col h-full justify-between gap-4">
      <div className="flex items-start justify-between">
        <div className={`inline-flex p-2.5 rounded-xl ${colorClasses[color]} border shadow-sm transition-transform group-hover:scale-110 duration-500`}>
          <Icon className="w-4 h-4" />
        </div>
        {to && <ArrowUpRight className="w-4 h-4 text-muted-foreground/30 group-hover:text-primary transition-colors" />}
      </div>
      
      <div className="space-y-1">
        <p className="text-3xl font-semibold tracking-tight text-foreground">{value}</p>
        <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/80">{title}</p>
      </div>
      
      {subtitle && (
        <p className="text-xs text-muted-foreground/60 font-medium truncate">{subtitle}</p>
      )}
    </div>
  );

  const wrapperClass = `group relative rounded-2xl border border-border/40 bg-card/40 backdrop-blur-md p-5 transition-all duration-500 hover:shadow-[0_8px_30px_rgb(0,0,0,0.04)] hover:border-primary/20 hover:-translate-y-1 animate-fade-up cursor-pointer ${className}`;

  if (to) {
    return (
      <Link to={to} className={wrapperClass}>
        {content}
      </Link>
    );
  }

  return (
    <div className={wrapperClass.replace('cursor-pointer', '')}>
      {content}
    </div>
  );
}

// Progress Bar Component - Refined
function ProgressBar({ value, max, color = 'primary', size = 'sm' }) {
  const percent = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  const heightClass = size === 'sm' ? 'h-1' : 'h-1.5';

  const colorClasses = {
    primary: 'bg-primary',
    success: 'bg-success',
    warning: 'bg-warning',
    error: 'bg-error',
    info: 'bg-info',
  };

  const getAutoColor = () => {
    if (percent < 60) return colorClasses.success;
    if (percent < 80) return colorClasses.warning;
    return colorClasses.error;
  };

  return (
    <div className={`w-full ${heightClass} bg-muted/30 rounded-full overflow-hidden`}>
      <div
        className={`${heightClass} ${color === 'auto' ? getAutoColor() : colorClasses[color]} rounded-full transition-all duration-700 ease-in-out`}
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
    warnings.forEach((w) => lines.push(`${(w?.Level || w?.level || 'WARNING').toUpperCase()}: ${w?.Message || w?.message || 'Unknown'}`));
  }
  if (trendWarnings.length > 0) {
    if (lines.length > 0) lines.push('');
    lines.push('Trend:');
    trendWarnings.forEach((m) => lines.push(m || 'Unknown'));
  }
  return lines.join('\n');
}

// Metric Row Component - Refined
function MetricRow({ icon: Icon, label, value, subtext, progress, color = 'primary' }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2 min-w-0">
          <Icon className="w-3.5 h-3.5 text-muted-foreground/50 shrink-0" />
          <span className="text-xs font-medium text-muted-foreground truncate">{label}</span>
        </div>
        <span className="text-xs font-mono font-semibold text-foreground shrink-0">{value}</span>
      </div>
      {progress !== undefined && (
        <ProgressBar value={progress} max={100} color={color} size="sm" />
      )}
      {subtext && (
        <p className="text-[10px] text-muted-foreground/50 font-medium pl-5 truncate">{subtext}</p>
      )}
    </div>
  );
}

// System Resources Card - Refined
function SystemResources({ data, loading, onRefresh, timeAgo }) {
  const [isExpanded, setIsExpanded] = useState(false);

  useEffect(() => {
    const checkWidth = () => { if (window.innerWidth >= 768) setIsExpanded(true); };
    checkWidth();
    window.addEventListener('resize', checkWidth);
    return () => window.removeEventListener('resize', checkWidth);
  }, []);

  if (loading && !data) {
    return (
      <BentoCard className="md:col-span-2">
        <div className="animate-pulse flex flex-col h-full space-y-8">
          <div className="h-3 bg-muted/20 rounded-full w-24" />
          <div className="grid grid-cols-1 md:grid-cols-2 gap-12">
            <div className="space-y-6">{[...Array(3)].map((_, i) => <div key={i} className="h-8 bg-muted/10 rounded-xl" />)}</div>
            <div className="space-y-6">{[...Array(3)].map((_, i) => <div key={i} className="h-8 bg-muted/10 rounded-xl" />)}</div>
          </div>
        </div>
      </BentoCard>
    );
  }

  const { system, resources } = data || {};
  const uptime = resources?.process_uptime ? resources.process_uptime / 1e9 : 0;
  const heapMB = resources?.heap_alloc_mb || 0;
  const goroutines = resources?.goroutines || 0;

  return (
    <BentoCard className="md:col-span-2">
      <div 
        className="flex items-center justify-between mb-6 cursor-pointer md:cursor-default"
        onClick={() => window.innerWidth < 768 && setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2.5">
          <Server className="w-4 h-4 text-primary/60" />
          <h3 className="text-[11px] font-bold uppercase tracking-[0.15em] text-muted-foreground">Infrastructure Core</h3>
        </div>

        <div className="flex items-center gap-4">
          {timeAgo && <span className="text-[10px] font-semibold text-muted-foreground/40 uppercase tracking-tighter hidden sm:inline">Sync {timeAgo}</span>}
          <button onClick={(e) => { e.stopPropagation(); onRefresh(); }} className="p-1.5 rounded-lg hover:bg-primary/5 transition-colors text-muted-foreground/40 hover:text-primary">
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <div className="md:hidden text-muted-foreground/40">{isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}</div>
        </div>
      </div>

      <div className={`${isExpanded ? 'block' : 'hidden md:block'} animate-fade-in`}> 
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-8">
          <div className="space-y-6">
            <h4 className="text-[10px] font-bold text-primary/50 uppercase tracking-widest flex items-center gap-2">
              <div className="w-1 h-1 rounded-full bg-primary/40" /> Quorum Process
            </h4>
            <div className="space-y-5">
              <MetricRow icon={MemoryStick} label="Heap Memory" value={`${heapMB.toFixed(1)} MB`} progress={Math.min(heapMB / 512 * 100, 100)} />
              <MetricRow icon={Layers} label="Active Goroutines" value={goroutines.toString()} />
              <MetricRow icon={Timer} label="Process Uptime" value={formatUptime(uptime)} />
            </div>
          </div>

          <div className="space-y-6">
            <h4 className="text-[10px] font-bold text-warning/50 uppercase tracking-widest flex items-center gap-2">
              <div className="w-1 h-1 rounded-full bg-warning/40" /> System Resources
            </h4>
            <div className="space-y-5">
              <MetricRow icon={Cpu} label="CPU Usage" value={`${system?.cpu_percent?.toFixed(1) || 0}%`} progress={system?.cpu_percent || 0} color="warning" />
              <MetricRow icon={MemoryStick} label="Memory Pool" value={`${(system?.mem_used_mb / 1024)?.toFixed(1) || 0} / ${(system?.mem_total_mb / 1024)?.toFixed(1) || 0} GB`} progress={system?.mem_percent || 0} color="warning" />
              <MetricRow icon={HardDrive} label="Disk I/O" value={`${system?.disk_used_gb?.toFixed(0) || 0} GB`} progress={system?.disk_percent || 0} color="warning" />
            </div>
          </div>
        </div>

        <div className="mt-8 pt-6 border-t border-border/30">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-1.5">
               <h5 className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40">Processor Identity</h5>
               <p className="text-xs font-medium text-foreground/70 truncate flex items-center gap-2">
                  <Cpu className="w-3 h-3 opacity-40" /> {system?.cpu_model || 'Unknown architecture'}
               </p>
            </div>
            <div className="space-y-1.5">
               <h5 className="text-[9px] font-bold uppercase tracking-widest text-muted-foreground/40">Accelerators</h5>
               {(system?.gpu_infos || []).length > 0 ? system.gpu_infos.map((gpu, i) => (
                  <p key={i} className="text-xs font-medium text-foreground/70 truncate flex items-center gap-2">
                    <MonitorDot className="w-3 h-3 opacity-40" /> {gpu.name}
                    {gpu.temp_valid && <span className="ml-1 px-1.5 py-0.5 rounded-md bg-orange-500/5 text-orange-500/70 text-[10px] font-bold">{gpu.temp_c?.toFixed(0)}°C</span>}
                  </p>
                )) : <p className="text-xs font-medium text-muted-foreground/40 italic">No neural processing units detected</p>}
            </div>
          </div>
        </div>
      </div>
    </BentoCard>
  );
}

// Recent Workflow Item - Refined
function WorkflowItem({ workflow }) {
  const statusColor = getStatusColor(workflow.status);
  const iconMap = { pending: Clock, running: Activity, completed: CheckCircle2, failed: XCircle };
  const StatusIcon = iconMap[workflow.status] || Clock;

  return (
    <Link to={`/workflows/${workflow.id}`} className="group flex items-center gap-4 p-3 rounded-xl transition-all hover:bg-primary/[0.03] border border-transparent hover:border-primary/10">
      <div className={`p-2 rounded-lg ${statusColor.bg} border ${statusColor.border} shadow-sm group-hover:scale-105 transition-transform duration-500`}>
        <StatusIcon className={`w-3.5 h-3.5 ${statusColor.text}`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-foreground truncate transition-colors group-hover:text-primary">{getWorkflowTitle(workflow)}</p>
        <div className="flex items-center gap-2 mt-0.5 opacity-50 font-medium">
          <span className="text-[10px] font-mono">{workflow.id.substring(0, 8)}</span>
          <span className="text-[10px] uppercase tracking-tighter">· {workflow.task_count || 0} sub-tasks</span>
        </div>
      </div>
      <ChevronRight className="w-3.5 h-3.5 text-muted-foreground/20 group-hover:text-primary group-hover:translate-x-0.5 transition-all" />
    </Link>
  );
}

// Active Workflow Banner - Refined
function ActiveWorkflowBanner({ workflow }) {
  if (!workflow) return null;
  return (
    <div className="relative overflow-hidden rounded-2xl border border-primary/10 bg-primary/[0.02] p-5 animate-fade-up">
      <div className="absolute top-0 right-0 w-64 h-64 bg-primary/5 rounded-full blur-[80px] -mr-32 -mt-32 pointer-events-none" />
      <div className="relative flex items-center justify-between gap-6">
        <div className="flex items-center gap-4 min-w-0">
          <div className="relative shrink-0">
            <div className="p-3 rounded-2xl bg-primary/10 border border-primary/10 shadow-sm"><Zap className="w-5 h-5 text-primary" /></div>
            <div className="absolute -top-1 -right-1 w-3 h-3 bg-primary rounded-full border-2 border-background animate-pulse" />
          </div>
          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2">
               <span className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary/70">Engine processing</span>
               <Badge variant="secondary" className="text-[9px] py-0 bg-primary/10 text-primary border-transparent font-bold">{workflow.current_phase}</Badge>
            </div>
            <p className="text-base font-semibold text-foreground truncate">{getWorkflowTitle(workflow)}</p>
          </div>
        </div>
        <Link to={`/workflows/${workflow.id}`} className="shrink-0 flex items-center gap-2 px-5 py-2 rounded-xl bg-primary text-primary-foreground text-xs font-bold hover:bg-primary/90 transition-all shadow-md shadow-primary/10">
          Inspect Cycle <ArrowUpRight className="w-3.5 h-3.5" />
        </Link>
      </div>
    </div>
  );
}

// Empty State - Refined
function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="relative mb-8">
        <div className="absolute inset-0 bg-primary/10 blur-[40px] rounded-full" />
        <div className="relative p-8 rounded-[2rem] bg-card/50 border border-border/40 shadow-sm backdrop-blur-sm">
          <Logo className="w-16 h-16 text-primary/80" />
        </div>
      </div>
      <h3 className="text-lg font-semibold text-foreground mb-2">Systems Ready</h3>
      <p className="text-sm text-muted-foreground mb-8 max-w-xs mx-auto leading-relaxed">Your intelligent consensus engine is initialized. Choose a blueprint to begin.</p>
      <div className="flex flex-col sm:flex-row items-center gap-4">
        <Link to="/templates" className="w-full sm:w-auto px-6 py-2.5 rounded-xl border border-border/60 text-foreground text-sm font-semibold hover:bg-accent transition-all shadow-sm">Browse Blueprints</Link>
        <Link to="/workflows/new" className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-semibold hover:bg-primary/90 transition-all shadow-lg shadow-primary/10 flex items-center gap-2"><Zap className="w-4 h-4" /> New Execution</Link>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="space-y-10 animate-pulse relative min-h-screen">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="h-40 bg-muted/10 rounded-3xl w-full border border-border/20" />
      <div className="grid grid-cols-2 lg:grid-cols-5 gap-6">{[...Array(5)].map((_, i) => <div key={i} className="h-32 rounded-2xl bg-muted/10 border border-border/20" />)}</div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-8"><div className="md:col-span-2 h-80 rounded-3xl bg-muted/10 border border-border/20" /><div className="h-80 rounded-3xl bg-muted/10 border border-border/20" /></div>
    </div>
  );
}

function useProjects() {
  const [projects, setProjects] = useState([]);
  const [loading, setLoading] = useState(true);
  const fetchProjects = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/projects/');
      if (response.ok) setProjects(await response.json() || []);
    } catch (error) { console.error(error); } finally { setLoading(false); }
  }, []);
  useEffect(() => { fetchProjects(); }, [fetchProjects]);
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
      if (response.ok) { setData(await response.json()); setLastUpdate(Date.now()); }
    } catch (error) { console.error(error); } finally { setLoading(false); }
  }, []);
  useEffect(() => { fetchResources(); const i = setInterval(fetchResources, 30000); return () => clearInterval(i); }, [fetchResources]);
  useEffect(() => {
    const update = () => {
      if (!lastUpdate) return;
      const s = Math.floor((Date.now() - lastUpdate) / 1000);
      setTimeAgo(s < 5 ? 'just now' : s < 60 ? `${s}s ago` : `${Math.floor(s/60)}m ago`);
    };
    update(); const i = setInterval(update, 5000); return () => clearInterval(i);
  }, [lastUpdate]);
  return { data, loading, refresh: fetchResources, timeAgo };
}

export default function Dashboard() {
  const { workflows, activeWorkflow, fetchWorkflows, fetchActiveWorkflow, loading } = useWorkflowStore();
  const { data: systemData, loading: systemLoading, refresh: refreshSystem, timeAgo: systemTimeAgo } = useSystemResources();
  const { projects } = useProjects();
  const navigate = useNavigate();

  useEffect(() => { fetchWorkflows(); fetchActiveWorkflow(); }, [fetchWorkflows, fetchActiveWorkflow]);

  const completedCount = workflows.filter(w => w.status === 'completed').length;
  const runningCount = workflows.filter(w => w.status === 'running').length;
  const failedCount = workflows.filter(w => w.status === 'failed').length;
  const healthyProjectsCount = projects.filter(p => p.status === 'healthy').length;
  const recentWorkflows = [...workflows].sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at)).slice(0, 5);

  if (loading && workflows.length === 0) return <LoadingSkeleton />;

  return (
    <div className="relative min-h-full space-y-10 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header - Refined & Compact */}
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        
        <div className="relative z-10 flex flex-col lg:flex-row lg:items-center justify-between gap-10">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]">
              <LayoutDashboard className="h-3 w-3 opacity-70" /> System Orchestrator
            </div>
            <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">
              Intelligent <span className="text-primary/80">Command Center</span>
            </h1>
            <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">
              Observe and orchestrate your multi-agent ecosystem with surgical precision. 
            </p>
          </div>

          <div className="flex flex-wrap gap-4 shrink-0">
             <Link to="/workflows/new" className="flex items-center justify-center gap-2.5 px-8 py-3.5 rounded-2xl bg-primary text-primary-foreground text-sm font-bold hover:bg-primary/90 transition-all shadow-xl shadow-primary/10">
                <Zap className="w-4 h-4" /> Start Execution
              </Link>
              <button onClick={refreshSystem} className="flex items-center justify-center gap-2.5 px-8 py-3.5 rounded-2xl bg-card/50 border border-border/60 text-muted-foreground text-sm font-bold hover:text-foreground hover:bg-accent transition-all">
                <RefreshCw className={`w-4 h-4 ${systemLoading ? 'animate-spin' : ''}`} /> Sync Metrics
              </button>
          </div>
        </div>
      </div>

      {activeWorkflow && activeWorkflow.status === 'running' && <ActiveWorkflowBanner workflow={activeWorkflow} />}

      {/* Stats Grid - Refined */}
      <div className="flex overflow-x-auto pb-4 -mx-3 px-3 sm:mx-0 sm:px-0 gap-6 snap-x md:grid md:grid-cols-2 lg:grid-cols-5 md:overflow-visible scrollbar-none">
        <StatCard title="Workspaces" value={projects.length} subtitle={`${healthyProjectsCount} healthy nodes`} icon={FolderKanban} to="/projects" className="min-w-[220px] md:min-w-0 snap-center" />
        <StatCard title="Total Cycles" value={workflows.length} subtitle="History depth" icon={GitBranch} to="/workflows" className="min-w-[220px] md:min-w-0 snap-center" />
        <StatCard title="Performance" value={`${Math.round((completedCount / Math.max(workflows.length, 1)) * 100)}%`} subtitle="Success velocity" icon={CheckCircle2} color="success" to="/workflows?status=completed" className="min-w-[220px] md:min-w-0 snap-center" />
        <StatCard title="Active Runs" value={runningCount} subtitle="Live processes" icon={Activity} color="info" to="/workflows?status=running" className="min-w-[220px] md:min-w-0 snap-center" />
        <StatCard title="Halted" value={failedCount} subtitle="Needs triage" icon={XCircle} color="error" to="/workflows?status=failed" className="min-w-[220px] md:min-w-0 snap-center" />
      </div>

      {/* Content Grid - Refined */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-10">
        <SystemResources data={systemData} loading={systemLoading} onRefresh={refreshSystem} timeAgo={systemTimeAgo} />

        <BentoCard className="flex flex-col">
          <div className="flex items-center justify-between mb-8">
            <div className="flex items-center gap-3">
               <div className="p-2.5 rounded-xl bg-primary/5 border border-primary/10 text-primary/60"><Activity className="w-4 h-4" /></div>
               <div>
                  <h2 className="text-[11px] font-bold uppercase tracking-widest text-muted-foreground">Recent Pulse</h2>
                  <p className="text-[9px] text-muted-foreground/40 font-bold uppercase tracking-tighter">Event Stream</p>
               </div>
            </div>
            <Link to="/workflows" className="p-2 rounded-lg hover:bg-primary/5 text-muted-foreground/40 hover:text-primary transition-all"><ArrowUpRight className="w-4 h-4" /></Link>
          </div>

          {recentWorkflows.length > 0 ? <div className="space-y-3 flex-1">{recentWorkflows.map((w) => <WorkflowItem key={workflow.id} workflow={w} />)}</div> : <EmptyState />}
          
          <div className="mt-8 pt-6 border-t border-border/30">
             <Link to="/kanban" className="flex items-center justify-between p-4 rounded-2xl bg-primary/[0.02] border border-primary/5 hover:border-primary/20 hover:bg-primary/[0.04] transition-all group">
                <div className="flex items-center gap-3">
                   <div className="p-2 rounded-lg bg-primary/5 text-primary/60 group-hover:scale-110 transition-transform"><FolderKanban className="w-4 h-4" /></div>
                   <span className="text-xs font-bold text-foreground/70 group-hover:text-primary transition-colors">Strategic Kanban Board</span>
                </div>
                <ChevronRight className="w-4 h-4 text-muted-foreground/20 group-hover:text-primary" />
             </Link>
          </div>
        </BentoCard>
      </div>

      <FAB onClick={() => navigate('/workflows/new')} icon={Zap} label="New Workflow" />
    </div>
  );
}
