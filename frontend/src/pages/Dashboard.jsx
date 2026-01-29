import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useWorkflowStore } from '../stores';
import {
  GitBranch,
  CheckCircle2,
  XCircle,
  Zap,
  Clock,
  ArrowUpRight,
  Activity,
  TrendingUp,
} from 'lucide-react';

// Get workflow display title
function getWorkflowTitle(workflow) {
  if (workflow.title) return workflow.title;
  if (workflow.prompt) {
    // Extract first meaningful line, skip generic prefixes
    const firstLine = workflow.prompt.split('\n')[0].trim();
    const cleaned = firstLine.replace(/^(analyze|analiza|implement|implementa|create|crea|fix|arregla|update|actualiza|add|añade|you are|eres)\s+/i, '');
    return cleaned.substring(0, 80) || workflow.prompt.substring(0, 80);
  }
  return workflow.id;
}

// Bento Grid Card Component
function BentoCard({ children, className = '', span = 1 }) {
  const spanClasses = {
    1: '',
    2: 'md:col-span-2',
    3: 'md:col-span-3',
  };

  return (
    <div
      className={`group relative rounded-xl border border-border bg-card p-6 transition-all hover:border-muted-foreground/30 hover:shadow-lg animate-fade-up ${spanClasses[span]} ${className}`}
    >
      {children}
    </div>
  );
}

// Stat Card for Bento Grid
function StatCard({ title, value, subtitle, icon: Icon, trend, color = 'primary' }) {
  const colorClasses = {
    primary: 'bg-primary/10 text-primary',
    success: 'bg-success/10 text-success',
    warning: 'bg-warning/10 text-warning',
    error: 'bg-error/10 text-error',
    info: 'bg-info/10 text-info',
  };

  return (
    <BentoCard>
      <div className="flex items-start justify-between">
        <div className="space-y-2">
          <p className="text-sm font-medium text-muted-foreground">{title}</p>
          <p className="text-3xl font-semibold text-foreground tracking-tight">{value}</p>
          {subtitle && (
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          )}
          {trend && (
            <div className="flex items-center gap-1 text-sm text-success">
              <TrendingUp className="w-3 h-3" />
              <span>{trend}</span>
            </div>
          )}
        </div>
        <div className={`p-3 rounded-xl ${colorClasses[color]}`}>
          <Icon className="w-5 h-5" />
        </div>
      </div>
    </BentoCard>
  );
}

// Recent Workflow Item
function WorkflowItem({ workflow }) {
  const statusConfig = {
    pending: { color: 'text-muted-foreground', bg: 'bg-muted', icon: Clock },
    running: { color: 'text-info', bg: 'bg-info/10', icon: Activity },
    completed: { color: 'text-success', bg: 'bg-success/10', icon: CheckCircle2 },
    failed: { color: 'text-error', bg: 'bg-error/10', icon: XCircle },
  };

  const config = statusConfig[workflow.status] || statusConfig.pending;
  const StatusIcon = config.icon;

  return (
    <Link
      to={`/workflows/${workflow.id}`}
      className="group flex items-center gap-4 p-3 -mx-3 rounded-lg transition-colors hover:bg-accent"
    >
      <div className={`p-2 rounded-lg ${config.bg}`}>
        <StatusIcon className={`w-4 h-4 ${config.color}`} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground truncate">
          {getWorkflowTitle(workflow)}
        </p>
        <p className="text-xs text-muted-foreground">
          {workflow.current_phase || 'Pending'} · {workflow.task_count || 0} tasks
        </p>
      </div>
      <ArrowUpRight className="w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
    </Link>
  );
}

// Active Workflow Banner
function ActiveWorkflowBanner({ workflow }) {
  if (!workflow) return null;

  return (
    <BentoCard span={3} className="bg-gradient-to-r from-info/5 to-primary/5 border-info/20">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="relative">
            <div className="p-3 rounded-xl bg-info/10">
              <Zap className="w-5 h-5 text-info" />
            </div>
            <span className="absolute -top-1 -right-1 w-3 h-3 bg-info rounded-full animate-pulse" />
          </div>
          <div>
            <p className="text-sm font-medium text-muted-foreground">Active Workflow</p>
            <p className="text-lg font-semibold text-foreground">
              {getWorkflowTitle(workflow)}
            </p>
            <p className="text-sm text-muted-foreground mt-1">
              Phase: {workflow.current_phase} · {workflow.task_count || 0} tasks
            </p>
          </div>
        </div>
        <Link
          to={`/workflows/${workflow.id}`}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          View Details
          <ArrowUpRight className="w-4 h-4" />
        </Link>
      </div>
    </BentoCard>
  );
}

// Empty State
function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="p-4 rounded-2xl bg-muted mb-4">
        <GitBranch className="w-8 h-8 text-muted-foreground" />
      </div>
      <h3 className="text-lg font-semibold text-foreground mb-2">No workflows yet</h3>
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
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      {[...Array(6)].map((_, i) => (
        <div
          key={i}
          className="h-32 rounded-xl bg-muted animate-pulse"
        />
      ))}
    </div>
  );
}

export default function Dashboard() {
  const { workflows, activeWorkflow, fetchWorkflows, fetchActiveWorkflow, loading } = useWorkflowStore();

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
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Dashboard</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Monitor your AI workflows and tasks
          </p>
        </div>
        <Link
          to="/workflows/new"
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Zap className="w-4 h-4" />
          New Workflow
        </Link>
      </div>

      {/* Active Workflow Banner */}
      {activeWorkflow && <ActiveWorkflowBanner workflow={activeWorkflow} />}

      {/* Bento Grid Stats */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Total Workflows"
          value={workflows.length}
          subtitle="All time"
          icon={GitBranch}
          color="primary"
        />
        <StatCard
          title="Completed"
          value={completedCount}
          subtitle={`${Math.round((completedCount / Math.max(workflows.length, 1)) * 100)}% success rate`}
          icon={CheckCircle2}
          color="success"
        />
        <StatCard
          title="Running"
          value={runningCount}
          subtitle="Active now"
          icon={Activity}
          color="info"
        />
        <StatCard
          title="Failed"
          value={failedCount}
          subtitle="Needs attention"
          icon={XCircle}
          color="error"
        />
      </div>

      {/* Recent Workflows */}
      <BentoCard span={3}>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-foreground">Recent Workflows</h2>
            <p className="text-sm text-muted-foreground">Your latest workflow activity</p>
          </div>
          <Link
            to="/workflows"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
          >
            View all
            <ArrowUpRight className="w-4 h-4" />
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
  );
}
