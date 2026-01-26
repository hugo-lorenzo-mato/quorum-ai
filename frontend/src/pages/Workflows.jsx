import { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useWorkflowStore, useTaskStore } from '../stores';
import {
  GitBranch,
  Plus,
  Play,
  Pause,
  StopCircle,
  CheckCircle2,
  XCircle,
  Clock,
  Activity,
  ArrowLeft,
  ChevronRight,
  Loader2,
  Zap,
} from 'lucide-react';

function StatusBadge({ status }) {
  const config = {
    pending: { color: 'bg-muted text-muted-foreground', icon: Clock },
    running: { color: 'bg-info/10 text-info', icon: Activity },
    completed: { color: 'bg-success/10 text-success', icon: CheckCircle2 },
    failed: { color: 'bg-error/10 text-error', icon: XCircle },
    paused: { color: 'bg-warning/10 text-warning', icon: Pause },
  };

  const { color, icon: Icon } = config[status] || config.pending;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${color}`}>
      <Icon className="w-3 h-3" />
      {status}
    </span>
  );
}

function WorkflowCard({ workflow, onClick }) {
  return (
    <button
      onClick={onClick}
      className="w-full text-left p-4 rounded-xl border border-border bg-card hover:border-muted-foreground/30 hover:shadow-md transition-all group"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex-1 min-w-0">
          <p className="text-sm font-medium text-foreground truncate">
            {workflow.prompt?.substring(0, 60) || 'Untitled workflow'}...
          </p>
          <p className="text-xs text-muted-foreground mt-1">{workflow.id}</p>
        </div>
        <StatusBadge status={workflow.status} />
      </div>
      <div className="flex items-center gap-4 text-xs text-muted-foreground">
        <span>Phase: {workflow.current_phase || 'N/A'}</span>
        <span>Tasks: {workflow.task_count || 0}</span>
      </div>
      <ChevronRight className="absolute right-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity" />
    </button>
  );
}

function TaskItem({ task }) {
  const config = {
    pending: { color: 'text-muted-foreground', bg: 'bg-muted' },
    running: { color: 'text-info', bg: 'bg-info/10' },
    completed: { color: 'text-success', bg: 'bg-success/10' },
    failed: { color: 'text-error', bg: 'bg-error/10' },
  };

  const { color, bg } = config[task.status] || config.pending;

  return (
    <div className="flex items-center gap-3 p-3 rounded-lg border border-border bg-card">
      <div className={`p-2 rounded-lg ${bg}`}>
        {task.status === 'running' ? (
          <Loader2 className={`w-4 h-4 ${color} animate-spin`} />
        ) : task.status === 'completed' ? (
          <CheckCircle2 className={`w-4 h-4 ${color}`} />
        ) : task.status === 'failed' ? (
          <XCircle className={`w-4 h-4 ${color}`} />
        ) : (
          <Clock className={`w-4 h-4 ${color}`} />
        )}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium text-foreground truncate">{task.name || task.id}</p>
        <p className="text-xs text-muted-foreground">{task.type || 'Task'}</p>
      </div>
      <StatusBadge status={task.status} />
    </div>
  );
}

function WorkflowDetail({ workflow, tasks, onBack }) {
  const { startWorkflow, pauseWorkflow, stopWorkflow, error, clearError } = useWorkflowStore();

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex items-center gap-4">
        <button
          onClick={onBack}
          className="p-2 rounded-lg hover:bg-accent transition-colors"
        >
          <ArrowLeft className="w-5 h-5 text-muted-foreground" />
        </button>
        <div className="flex-1">
          <h1 className="text-xl font-semibold text-foreground">Workflow Details</h1>
          <p className="text-sm text-muted-foreground">{workflow.id}</p>
        </div>
        <div className="flex items-center gap-2">
          {workflow.status === 'pending' && (
            <button
              onClick={() => startWorkflow(workflow.id)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
            >
              <Play className="w-4 h-4" />
              Start
            </button>
          )}
          {workflow.status === 'running' && (
            <>
              <button
                onClick={() => pauseWorkflow(workflow.id)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
              >
                <Pause className="w-4 h-4" />
                Pause
              </button>
              <button
                onClick={() => stopWorkflow(workflow.id)}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-destructive text-destructive-foreground text-sm font-medium hover:bg-destructive/90 transition-colors"
              >
                <StopCircle className="w-4 h-4" />
                Stop
              </button>
            </>
          )}
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="p-4 rounded-lg bg-warning/10 border border-warning/20 flex items-start justify-between">
          <p className="text-sm text-warning">{error}</p>
          <button onClick={clearError} className="text-warning hover:text-warning/80 text-sm">
            Dismiss
          </button>
        </div>
      )}

      {/* Info Card */}
      <div className="p-6 rounded-xl border border-border bg-card">
        <div className="flex items-start justify-between mb-4">
          <div>
            <h2 className="text-lg font-semibold text-foreground mb-2">
              {workflow.prompt?.substring(0, 100) || 'Untitled workflow'}
            </h2>
            <StatusBadge status={workflow.status} />
          </div>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4 pt-4 border-t border-border">
          <div>
            <p className="text-xs text-muted-foreground">Phase</p>
            <p className="text-sm font-medium text-foreground">{workflow.current_phase || 'N/A'}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Tasks</p>
            <p className="text-sm font-medium text-foreground">{tasks.length}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Created</p>
            <p className="text-sm font-medium text-foreground">
              {new Date(workflow.created_at).toLocaleDateString()}
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Updated</p>
            <p className="text-sm font-medium text-foreground">
              {new Date(workflow.updated_at).toLocaleDateString()}
            </p>
          </div>
        </div>
      </div>

      {/* Tasks */}
      <div>
        <h3 className="text-lg font-semibold text-foreground mb-4">Tasks ({tasks.length})</h3>
        {tasks.length > 0 ? (
          <div className="space-y-2">
            {tasks.map((task) => (
              <TaskItem key={task.id} task={task} />
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground">
            <p className="text-sm">No tasks yet</p>
          </div>
        )}
      </div>
    </div>
  );
}

function NewWorkflowForm({ onSubmit, onCancel, loading }) {
  const [prompt, setPrompt] = useState('');

  const handleSubmit = (e) => {
    e.preventDefault();
    if (prompt.trim()) onSubmit(prompt.trim());
  };

  return (
    <div className="max-w-2xl mx-auto p-6 rounded-xl border border-border bg-card animate-fade-up">
      <h2 className="text-xl font-semibold text-foreground mb-4">Create New Workflow</h2>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Workflow Prompt
          </label>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Describe what you want the AI agents to accomplish..."
            rows={4}
            className="w-full px-4 py-3 border border-input rounded-lg bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background resize-none"
          />
        </div>
        <div className="flex gap-3">
          <button
            type="submit"
            disabled={loading || !prompt.trim()}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Zap className="w-4 h-4" />}
            Create Workflow
          </button>
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2.5 rounded-lg bg-secondary text-secondary-foreground text-sm font-medium hover:bg-secondary/80 transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}

export default function Workflows() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { workflows, loading, fetchWorkflows, createWorkflow } = useWorkflowStore();
  const { getTasksForWorkflow, setTasks } = useTaskStore();
  const [showNewForm, setShowNewForm] = useState(false);

  useEffect(() => {
    fetchWorkflows();
  }, [fetchWorkflows]);

  // Fetch tasks for the selected workflow
  const fetchTasks = async (workflowId) => {
    try {
      const { workflowApi } = await import('../lib/api');
      const taskList = await workflowApi.getTasks(workflowId);
      setTasks(workflowId, taskList);
    } catch (error) {
      console.error('Failed to fetch tasks:', error);
    }
  };

  useEffect(() => {
    if (id && id !== 'new') {
      fetchTasks(id);
    }
  }, [id]);

  const selectedWorkflow = workflows.find(w => w.id === id);
  const workflowTasks = id ? getTasksForWorkflow(id) : [];

  const handleCreate = async (prompt) => {
    const workflow = await createWorkflow(prompt);
    if (workflow) {
      setShowNewForm(false);
      navigate(`/workflows/${workflow.id}`);
    }
  };

  // Show new workflow form
  if (id === 'new' || showNewForm) {
    return (
      <NewWorkflowForm
        onSubmit={handleCreate}
        onCancel={() => {
          setShowNewForm(false);
          if (id === 'new') navigate('/workflows');
        }}
        loading={loading}
      />
    );
  }

  // Show workflow detail
  if (selectedWorkflow) {
    return (
      <WorkflowDetail
        workflow={selectedWorkflow}
        tasks={workflowTasks}
        onBack={() => navigate('/workflows')}
      />
    );
  }

  // Show workflow list
  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Workflows</h1>
          <p className="text-sm text-muted-foreground mt-1">Manage your AI automation workflows</p>
        </div>
        <Link
          to="/workflows/new"
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Workflow
        </Link>
      </div>

      {loading && workflows.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="h-32 rounded-xl bg-muted animate-pulse" />
          ))}
        </div>
      ) : workflows.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {workflows.map((workflow) => (
            <WorkflowCard
              key={workflow.id}
              workflow={workflow}
              onClick={() => navigate(`/workflows/${workflow.id}`)}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-16">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-muted flex items-center justify-center">
            <GitBranch className="w-8 h-8 text-muted-foreground" />
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">No workflows yet</h3>
          <p className="text-sm text-muted-foreground mb-4">Create your first workflow to get started</p>
          <Link
            to="/workflows/new"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90 transition-colors"
          >
            <Plus className="w-4 h-4" />
            Create Workflow
          </Link>
        </div>
      )}
    </div>
  );
}
