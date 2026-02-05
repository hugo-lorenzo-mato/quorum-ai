import { useEffect, useState, useCallback } from 'react';
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
  FolderKanban,
  LayoutDashboard,
} from 'lucide-react';

function getWorkflowTitle(workflow) {
  if (workflow.title) return workflow.title;
  if (workflow.prompt) {
    const firstLine = workflow.prompt.split('\n')[0].trim();
    const cleaned = firstLine.replace(/^(analyze|analiza|implement|implementa|create|crea|fix|arregla|update|actualiza|add|añade|you are|eres)\s+/i, '');
    return cleaned.substring(0, 80) || workflow.prompt.substring(0, 80);
  }
  return workflow.id;
}

function BentoCard({ children, className = '' }) {
  return (
    <div className={`group relative rounded-2xl border border-border/30 bg-card/20 backdrop-blur-sm p-5 transition-all duration-500 hover:shadow-soft hover:border-primary/10 animate-fade-up ${className}`}>
      {children}
    </div>
  );
}

function StatCard({ title, value, subtitle, icon: Icon, color = 'primary', className = '', to }) {
  const colorClasses = {
    primary: 'text-primary bg-primary/5',
    success: 'text-success bg-success/5',
    warning: 'text-warning bg-warning/5',
    error: 'text-error bg-error/5',
    info: 'text-info bg-info/5',
  };

  const content = (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className={`p-2 rounded-xl ${colorClasses[color]} border border-current/5`}>
          <Icon className="w-4 h-4" />
        </div>
        {to && <ArrowUpRight className="w-3.5 h-3.5 text-muted-foreground/30 group-hover:text-primary transition-colors" />}
      </div>
      <div>
        <p className="text-2xl font-semibold tracking-tight text-foreground">{value}</p>
        <p className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground/60">{title}</p>
      </div>
      {subtitle && (
        <p className="text-[10px] text-muted-foreground/40 font-medium truncate">{subtitle}</p>
      )}
    </div>
  );

  const wrapperClass = `group relative rounded-2xl border border-border/30 bg-card/10 backdrop-blur-sm p-5 transition-all duration-500 hover:border-primary/20 hover:-translate-y-0.5 animate-fade-up ${className}`;

  if (to) {
    return (
      <Link to={to} className={`${wrapperClass} cursor-pointer`}>
        {content}
      </Link>
    );
  }

  return <div className={wrapperClass}>{content}</div>;
}

function MetricRow({ icon: Icon, label, value, progress, color = 'primary' }) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-[11px] font-medium">
        <div className="flex items-center gap-2 text-muted-foreground/70">
          <Icon className="w-3 h-3 opacity-50" />
          <span>{label}</span>
        </div>
        <span className="text-foreground/80 font-mono">{value}</span>
      </div>
      {progress !== undefined && (
        <div className="h-1 w-full bg-muted/20 rounded-full overflow-hidden">
          <div 
            className={`h-full rounded-full transition-all duration-1000 ${color === 'warning' ? 'bg-warning/60' : 'bg-primary/60'}`}
            style={{ width: `${progress}%` }}
          />
        </div>
      )}
    </div>
  );
}

function SystemResources({ data, loading, onRefresh, timeAgo }) {
  const [isExpanded, setIsExpanded] = useState(false);
  useEffect(() => {
    const checkWidth = () => { if (window.innerWidth >= 768) setIsExpanded(true); };
    checkWidth();
    window.addEventListener('resize', checkWidth);
    return () => window.removeEventListener('resize', checkWidth);
  }, []);

  if (loading && !data) {
    return <BentoCard className="md:col-span-2 h-64 animate-pulse bg-muted/5" />;
  }

  const { system, resources } = data || {};
  const uptime = resources?.process_uptime ? resources.process_uptime / 1e9 : 0;
  const heapMB = resources?.heap_alloc_mb || 0;

  return (
    <BentoCard className="md:col-span-2">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          <Server className="w-3.5 h-3.5 text-primary/50" />
          <h3 className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60">System Core</h3>
        </div>
        <button onClick={onRefresh} className="text-muted-foreground/30 hover:text-primary transition-colors">
          <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-x-10 gap-y-6">
        <div className="space-y-4">
          <MetricRow icon={MemoryStick} label="Engine Heap" value={`${heapMB.toFixed(0)}MB`} progress={Math.min(heapMB/5, 100)} />
          <MetricRow icon={Layers} label="Goroutines" value={resources?.goroutines?.toString() || '0'} />
          <MetricRow icon={Timer} label="Uptime" value={formatUptime(uptime)} />
        </div>
        <div className="space-y-4">
          <MetricRow icon={Cpu} label="CPU Load" value={`${system?.cpu_percent?.toFixed(0)}%`} progress={system?.cpu_percent} color="warning" />
          <MetricRow icon={MemoryStick} label="System RAM" value={`${(system?.mem_used_mb/1024).toFixed(1)}GB`} progress={system?.mem_percent} color="warning" />
          <MetricRow icon={HardDrive} label="Storage" value={`${system?.disk_used_gb}GB`} progress={system?.disk_percent} color="warning" />
        </div>
      </div>
    </BentoCard>
  );
}

function WorkflowItem({ workflow }) {
  const statusColor = getStatusColor(workflow.status);
  return (
    <Link to={`/workflows/${workflow.id}`} className="group flex items-center justify-between p-3 rounded-xl transition-all hover:bg-primary/[0.02]">
      <div className="flex items-center gap-3 min-w-0">
        <div className={`w-1.5 h-1.5 rounded-full ${statusColor.dot} opacity-40 group-hover:opacity-100 group-hover:scale-125 transition-all`} />
        <p className="text-xs font-medium text-foreground/70 truncate group-hover:text-primary transition-colors">{getWorkflowTitle(workflow)}</p>
      </div>
      <ChevronRight className="w-3 h-3 text-muted-foreground/20 group-hover:text-primary group-hover:translate-x-0.5 transition-all" />
    </Link>
  );
}

function ActiveWorkflowBanner({ workflow }) {
  if (!workflow) return null;
  return (
    <div className="relative overflow-hidden rounded-2xl border border-primary/10 bg-primary/[0.01] px-5 py-4 animate-fade-up">
      <div className="relative flex items-center justify-between gap-4">
        <div className="flex items-center gap-4 min-w-0">
          <div className="p-2 rounded-xl bg-primary/5 text-primary/60"><Zap className="w-4 h-4" /></div>
          <div className="min-w-0">
            <p className="text-[9px] font-bold uppercase tracking-widest text-primary/50">Engine Active: {workflow.current_phase}</p>
            <p className="text-sm font-semibold text-foreground truncate">{getWorkflowTitle(workflow)}</p>
          </div>
        </div>
        <Link to={`/workflows/${workflow.id}`} className="shrink-0 text-xs font-bold text-primary hover:underline">Manage Cycle</Link>
      </div>
    </div>
  );
}

function formatElapsed(startTime) {
  if (!startTime) return '';
  const elapsed = Math.floor((Date.now() - new Date(startTime).getTime()) / 1000);
  return elapsed < 60 ? `${elapsed}s` : `${Math.floor(elapsed / 60)}m`;
}

function useProjects() {
  const [projects, setProjects] = useState([]);
  const [loading, setLoading] = useState(true);
  const fetchProjects = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/projects/');
      if (response.ok) setProjects(await response.json() || []);
    } catch (e) { console.error(e); } finally { setLoading(false); }
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
    } catch (e) { console.error(e); } finally { setLoading(false); }
  }, []);
  useEffect(() => { fetchResources(); const i = setInterval(fetchResources, 30000); return () => clearInterval(i); }, [fetchResources]);
  useEffect(() => {
    const update = () => {
      if (!lastUpdate) return;
      const s = Math.floor((Date.now() - lastUpdate) / 1000);
      setTimeAgo(s < 5 ? 'now' : `${s}s ago`);
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
  const recentWorkflows = [...workflows].sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at)).slice(0, 5);

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header - Ultra Minimalist */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary">
            <div className="w-1 h-1 rounded-full bg-current" />
            <span className="text-[10px] font-bold uppercase tracking-widest opacity-70">Control Deck</span>
          </div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Orchestrator <span className="text-muted-foreground/40 font-medium">Dashboard</span></h1>
        </div>
        <div className="flex items-center gap-3">
           <Link to="/workflows/new" className="px-5 py-2 rounded-xl bg-primary text-primary-foreground text-xs font-bold hover:bg-primary/90 transition-all shadow-sm">Initialize Cycle</Link>
           <button onClick={refreshSystem} className="p-2 rounded-xl border border-border/40 hover:bg-accent transition-all text-muted-foreground/60"><RefreshCw className={`w-4 h-4 ${systemLoading ? 'animate-spin' : ''}`} /></button>
        </div>
      </div>

      {activeWorkflow && activeWorkflow.status === 'running' && <ActiveWorkflowBanner workflow={activeWorkflow} />}

      <div className="grid grid-cols-2 lg:grid-cols-5 gap-4">
        <StatCard title="Nodes" value={projects.length} subtitle="Environments" icon={FolderKanban} to="/projects" />
        <StatCard title="Cycles" value={workflows.length} subtitle="Executions" icon={GitBranch} to="/workflows" />
        <StatCard title="Reliability" value={`${Math.round((completedCount / Math.max(workflows.length, 1)) * 100)}%`} icon={CheckCircle2} color="success" to="/workflows?status=completed" />
        <StatCard title="Active" value={runningCount} icon={Activity} color="info" to="/workflows?status=running" />
        <StatCard title="Faults" value={failedCount} icon={XCircle} color="error" to="/workflows?status=failed" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
        <SystemResources data={systemData} loading={systemLoading} onRefresh={refreshSystem} timeAgo={systemTimeAgo} />
        <BentoCard className="flex flex-col">
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/60">Execution Stream</h3>
            <Link to="/workflows" className="text-[10px] font-bold text-primary/60 hover:underline">History</Link>
          </div>
          <div className="space-y-1 flex-1">
            {recentWorkflows.length > 0 ? recentWorkflows.map((w) => <WorkflowItem key={w.id} workflow={w} />) : <p className="text-xs text-muted-foreground/40 py-8 text-center italic">No data streams detected</p>}
          </div>
        </BentoCard>
      </div>

      <FAB onClick={() => navigate('/workflows/new')} icon={Zap} label="New Workflow" />
    </div>
  );
}
