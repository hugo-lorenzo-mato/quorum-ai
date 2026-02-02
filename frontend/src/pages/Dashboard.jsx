import { useEffect, useState, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useWorkflowStore } from '../stores';
import { getStatusColor } from '../lib/theme';
import FAB from '../components/FAB';
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

// Compact Stats Bar Component
function StatsBar({ total, completed, running, failed }) {
  const stats = [
    { label: 'Total', value: total, icon: GitBranch, color: 'text-primary' },
    { label: 'Completed', value: completed, icon: CheckCircle2, color: 'text-success' },
    { label: 'Running', value: running, icon: Activity, color: 'text-info' },
    { label: 'Failed', value: failed, icon: XCircle, color: 'text-error' },
  ];

  return (
    <div className="flex flex-wrap items-center gap-2 p-3 rounded-xl border border-border bg-card animate-fade-up">
      {stats.map((stat, index) => (
        <div key={stat.label} className="flex items-center gap-1.5">
          {index > 0 && <span className="hidden sm:block text-border mx-1">|</span>}
          <stat.icon className={`w-4 h-4 ${stat.color}`} />
          <span className="text-sm font-medium text-foreground">{stat.value}</span>
          <span className="text-xs text-muted-foreground hidden sm:inline">{stat.label}</span>
        </div>
      ))}
      {total > 0 && (
        <>
          <span className="hidden md:block text-border mx-1">|</span>
          <span className="hidden md:inline text-xs text-muted-foreground">
            {Math.round((completed / Math.max(total, 1)) * 100)}% success rate
          </span>
        </>
      )}
    </div>
  );
}

// Progress Bar Component
function ProgressBar({ value, max, color = 'primary', size = 'md' }) {
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
    if (percent < 50) return colorClasses.success;
    if (percent < 75) return colorClasses.warning;
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

// System Resources Card
function SystemResources({ resources, loading, onRefresh }) {
  if (loading && !resources) {
    return (
      <div className="rounded-xl border border-border bg-card p-4 animate-pulse">
        <div className="h-4 bg-muted rounded w-32 mb-4" />
        <div className="space-y-3">
          <div className="h-8 bg-muted rounded" />
          <div className="h-8 bg-muted rounded" />
          <div className="h-8 bg-muted rounded" />
        </div>
      </div>
    );
  }

  if (!resources) {
    return (
      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-medium text-foreground flex items-center gap-2">
            <Server className="w-4 h-4 text-muted-foreground" />
            System Resources
          </h3>
        </div>
        <p className="text-xs text-muted-foreground">Unable to load system metrics</p>
      </div>
    );
  }

  const { resources: res } = resources;
  const memoryMB = res?.heap_alloc_mb || 0;
  const goroutines = res?.goroutines || 0;
  const uptime = res?.process_uptime ? res.process_uptime / 1e9 : 0; // nanoseconds to seconds
  const commandsActive = res?.commands_active || 0;

  const metrics = [
    {
      label: 'Memory',
      value: `${memoryMB.toFixed(1)} MB`,
      icon: HardDrive,
      progress: memoryMB,
      max: 512, // Assume 512MB as reference
      color: 'auto',
    },
    {
      label: 'Goroutines',
      value: goroutines,
      icon: Cpu,
      progress: goroutines,
      max: 1000,
      color: 'auto',
    },
    {
      label: 'Uptime',
      value: formatUptime(uptime),
      icon: Timer,
      showProgress: false,
    },
    {
      label: 'Active Tasks',
      value: commandsActive,
      icon: Activity,
      showProgress: false,
    },
  ];

  return (
    <div className="rounded-xl border border-border bg-card p-4 animate-fade-up">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-medium text-foreground flex items-center gap-2">
          <Server className="w-4 h-4 text-muted-foreground" />
          System Resources
        </h3>
        <button
          onClick={onRefresh}
          className="p-1.5 rounded-lg hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
          title="Refresh"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="space-y-3">
        {metrics.map((metric) => (
          <div key={metric.label} className="space-y-1">
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground flex items-center gap-1.5">
                <metric.icon className="w-3.5 h-3.5" />
                {metric.label}
              </span>
              <span className="text-xs font-medium text-foreground font-mono">
                {metric.value}
              </span>
            </div>
            {metric.showProgress !== false && metric.max && (
              <ProgressBar
                value={metric.progress}
                max={metric.max}
                color={metric.color}
                size="sm"
              />
            )}
          </div>
        ))}
      </div>

      {resources.status && resources.status !== 'healthy' && (
        <div className={`mt-3 text-xs px-2 py-1 rounded ${
          resources.status === 'critical'
            ? 'bg-error/10 text-error'
            : 'bg-warning/10 text-warning'
        }`}>
          Status: {resources.status}
        </div>
      )}
    </div>
  );
}

// Bento Grid Card Component
function BentoCard({ children, className = '' }) {
  return (
    <div className={`group relative rounded-xl border border-border bg-card p-4 md:p-6 transition-all hover:border-muted-foreground/30 hover:shadow-lg animate-fade-up ${className}`}>
      {children}
    </div>
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

// Empty State
function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-8 text-center">
      <div className="p-3 rounded-2xl bg-muted mb-3">
        <GitBranch className="w-6 h-6 text-muted-foreground" />
      </div>
      <h3 className="text-base font-semibold text-foreground mb-1">No workflows yet</h3>
      <p className="text-sm text-muted-foreground mb-4 max-w-sm">
        Create your first workflow to start automating tasks with AI agents.
      </p>
      <Link
        to="/workflows/new"
        className="px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
      >
        Create Workflow
      </Link>
    </div>
  );
}

// Loading Skeleton
function LoadingSkeleton() {
  return (
    <div className="space-y-4">
      <div className="h-12 rounded-xl bg-muted animate-pulse" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="h-40 rounded-xl bg-muted animate-pulse" />
        <div className="md:col-span-2 h-40 rounded-xl bg-muted animate-pulse" />
      </div>
    </div>
  );
}

// Custom hook for system resources
function useSystemResources() {
  const [resources, setResources] = useState(null);
  const [loading, setLoading] = useState(true);

  const fetchResources = useCallback(async () => {
    setLoading(true);
    try {
      const response = await fetch('/health/deep');
      if (response.ok) {
        const data = await response.json();
        setResources(data);
      }
    } catch (error) {
      console.error('Failed to fetch system resources:', error);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchResources();
    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchResources, 30000);
    return () => clearInterval(interval);
  }, [fetchResources]);

  return { resources, loading, refresh: fetchResources };
}

export default function Dashboard() {
  const { workflows, activeWorkflow, fetchWorkflows, fetchActiveWorkflow, loading } = useWorkflowStore();
  const { resources, loading: resourcesLoading, refresh: refreshResources } = useSystemResources();
  const navigate = useNavigate();

  useEffect(() => {
    fetchWorkflows();
    fetchActiveWorkflow();
  }, [fetchWorkflows, fetchActiveWorkflow]);

  const completedCount = workflows.filter(w => w.status === 'completed').length;
  const runningCount = workflows.filter(w => w.status === 'running').length;
  const failedCount = workflows.filter(w => w.status === 'failed').length;

  const recentWorkflows = [...workflows]
    .sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at))
    .slice(0, 5);

  if (loading && workflows.length === 0) {
    return <LoadingSkeleton />;
  }

  return (
    <div className="space-y-4 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl md:text-2xl font-semibold text-foreground tracking-tight">Dashboard</h1>
          <p className="text-xs md:text-sm text-muted-foreground mt-0.5">
            Monitor your AI workflows and system health
          </p>
        </div>
        <Link
          to="/workflows/new"
          className="hidden md:flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Zap className="w-4 h-4" />
          New Workflow
        </Link>
      </div>

      {/* Compact Stats Bar */}
      <StatsBar
        total={workflows.length}
        completed={completedCount}
        running={runningCount}
        failed={failedCount}
      />

      {/* Active Workflow Banner */}
      {activeWorkflow && activeWorkflow.status === 'running' && (
        <ActiveWorkflowBanner workflow={activeWorkflow} />
      )}

      {/* Main Grid: System Resources + Recent Workflows */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* System Resources - smaller on desktop */}
        <div className="md:col-span-1">
          <SystemResources
            resources={resources}
            loading={resourcesLoading}
            onRefresh={refreshResources}
          />
        </div>

        {/* Recent Workflows - larger on desktop */}
        <div className="md:col-span-2">
          <BentoCard>
            <div className="flex items-center justify-between mb-4">
              <div>
                <h2 className="text-base font-semibold text-foreground">Recent Workflows</h2>
                <p className="text-xs text-muted-foreground">Your latest workflow activity</p>
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
              <div className="space-y-0.5">
                {recentWorkflows.map((workflow) => (
                  <WorkflowItem key={workflow.id} workflow={workflow} />
                ))}
              </div>
            ) : (
              <EmptyState />
            )}
          </BentoCard>
        </div>
      </div>

      {/* Mobile FAB */}
      <FAB onClick={() => navigate('/workflows/new')} icon={Zap} label="New Workflow" />
    </div>
  );
}
