import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useKanbanStore, KANBAN_COLUMNS } from '../stores';

// Column component
function KanbanColumn({ column, workflows }) {
  const {
    moveWorkflow,
    setDraggedWorkflow,
    clearDraggedWorkflow,
    engine,
    draggedWorkflow,
  } = useKanbanStore();
  const navigate = useNavigate();
  const dragDepth = useRef(0);
  const [isOver, setIsOver] = useState(false);

  const canDrop = Boolean(draggedWorkflow && draggedWorkflow.kanban_column !== column.id);

  useEffect(() => {
    const reset = () => {
      dragDepth.current = 0;
      setIsOver(false);
    };

    window.addEventListener('dragend', reset);
    window.addEventListener('drop', reset);
    return () => {
      window.removeEventListener('dragend', reset);
      window.removeEventListener('drop', reset);
    };
  }, []);

  const handleDragOver = (e) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = canDrop ? 'move' : 'none';
  };

  const handleDrop = (e) => {
    e.preventDefault();
    dragDepth.current = 0;
    setIsOver(false);

    if (!canDrop) {
      clearDraggedWorkflow();
      return;
    }

    const workflowId = e.dataTransfer.getData('text/plain');
    if (workflowId) {
      moveWorkflow(workflowId, column.id);
    }
    clearDraggedWorkflow();
  };

  const handleDragEnter = (e) => {
    e.preventDefault();
    if (!canDrop) return;
    dragDepth.current += 1;
    setIsOver(true);
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
    if (!canDrop) return;
    dragDepth.current = Math.max(0, dragDepth.current - 1);
    if (dragDepth.current === 0) setIsOver(false);
  };

  // Column accent colors (subtle, consistent across themes)
  const getColumnAccent = () => {
    switch (column.id) {
      case 'refinement':
        return { dot: 'bg-yellow-500', tint: 'bg-yellow-500/10', ring: 'ring-yellow-500/20' };
      case 'todo':
        return { dot: 'bg-blue-500', tint: 'bg-blue-500/10', ring: 'ring-blue-500/20' };
      case 'in_progress':
        return { dot: 'bg-purple-500', tint: 'bg-purple-500/10', ring: 'ring-purple-500/20' };
      case 'to_verify':
        return { dot: 'bg-orange-500', tint: 'bg-orange-500/10', ring: 'ring-orange-500/20' };
      case 'done':
        return { dot: 'bg-green-500', tint: 'bg-green-500/10', ring: 'ring-green-500/20' };
      default:
        return { dot: 'bg-muted-foreground', tint: 'bg-muted', ring: 'ring-ring/20' };
    }
  };

  const accent = getColumnAccent();

  return (
    <div
      className={`flex flex-col min-w-[280px] max-w-[320px] rounded-xl border border-border bg-card/50 glass ${
        isOver && canDrop ? `ring-2 ${accent.ring}` : ''
      }`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
    >
      {/* Column header */}
      <div className={`p-4 rounded-t-xl border-b border-border ${accent.tint}`}>
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-2.5 min-w-0">
            <span className={`mt-1.5 h-2.5 w-2.5 rounded-full ${accent.dot}`} aria-hidden="true" />
            <div className="min-w-0">
              <h3 className="font-semibold text-foreground text-sm truncate">{column.name}</h3>
              <p className="text-xs text-muted-foreground truncate">{column.description}</p>
            </div>
          </div>
          <span className="shrink-0 bg-muted text-muted-foreground text-xs px-2 py-0.5 rounded-full">
            {workflows.length}
          </span>
        </div>
      </div>

      {/* Workflow cards */}
      <div className="flex-1 p-3 space-y-2 overflow-y-auto max-h-[calc(100vh-320px)]">
        {workflows.map((workflow) => (
          <KanbanCard
            key={workflow.id}
            workflow={workflow}
            isExecuting={engine.currentWorkflowId === workflow.id}
            onDragStart={() => setDraggedWorkflow(workflow)}
            onDragEnd={() => clearDraggedWorkflow()}
            onClick={() => navigate(`/workflows/${workflow.id}`)}
          />
        ))}
        {workflows.length === 0 && (
          <div className="text-muted-foreground text-sm text-center py-8">
            No workflows
          </div>
        )}
      </div>
    </div>
  );
}

// Card component
function KanbanCard({ workflow, isExecuting, onDragStart, onDragEnd, onClick }) {
  const handleDragStart = (e) => {
    e.dataTransfer.setData('text/plain', workflow.id);
    e.dataTransfer.effectAllowed = 'move';
    onDragStart();
  };

  const statusConfig = {
    pending: 'bg-muted text-muted-foreground',
    running: 'bg-info/10 text-info',
    completed: 'bg-success/10 text-success',
    failed: 'bg-error/10 text-error',
    paused: 'bg-warning/10 text-warning',
  };

  const status = workflow.status || 'pending';
  const statusClasses = statusConfig[status] || statusConfig.pending;

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onClick={onClick}
      className={`group relative rounded-xl border border-border bg-background p-3 shadow-sm cursor-pointer transition-all hover:bg-accent/30 hover:border-muted-foreground/30 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30 ${
        isExecuting ? 'ring-2 ring-info/30' : ''
      }`}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onClick?.();
      }}
    >
      {isExecuting && (
        <div className="absolute top-3 right-3 flex items-center gap-1 text-xs text-info">
          <span className="w-1.5 h-1.5 bg-info rounded-full animate-pulse" aria-hidden="true" />
          <span>Executing</span>
        </div>
      )}

      {/* Title */}
      <h4 className="font-medium text-foreground text-sm mb-1 line-clamp-2 pr-16">
        {workflow.title || workflow.id}
      </h4>

      {/* Prompt preview */}
      <p className="text-muted-foreground text-xs line-clamp-2 mb-3">
        {workflow.prompt}
      </p>

      {/* Metadata row */}
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className={`inline-flex items-center px-2 py-0.5 rounded-full font-medium ${statusClasses}`}>
          {status}
        </span>
        <div className="flex items-center gap-2 text-muted-foreground">
          {workflow.kanban_execution_count > 0 && (
            <span>
              Run {workflow.kanban_execution_count}x
            </span>
          )}
          {workflow.pr_url && (
            <a
              href={workflow.pr_url}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(e) => e.stopPropagation()}
              className="text-info hover:text-info/80 transition-colors"
            >
              PR #{workflow.pr_number}
            </a>
          )}
        </div>
      </div>

      {/* Error indicator */}
      {workflow.kanban_last_error && (
        <div
          className="mt-2 rounded-lg border border-error/20 bg-error/5 px-2 py-1 text-xs text-error line-clamp-2"
          title={workflow.kanban_last_error}
        >
          {workflow.kanban_last_error}
        </div>
      )}

      {/* Task count */}
      {workflow.task_count > 0 && (
        <div className="mt-2 text-xs text-muted-foreground">
          {workflow.task_count} tasks
        </div>
      )}
    </div>
  );
}

// Engine controls component
function EngineControls() {
  const {
    engine,
    enableEngine,
    disableEngine,
    resetCircuitBreaker,
  } = useKanbanStore();

  const [isToggling, setIsToggling] = useState(false);

  const handleToggle = async () => {
    setIsToggling(true);
    try {
      if (engine.enabled) {
        await disableEngine();
      } else {
        await enableEngine();
      }
    } finally {
      setIsToggling(false);
    }
  };

  const handleReset = async () => {
    await resetCircuitBreaker();
  };

  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4 rounded-xl border border-border bg-card/50 glass px-4 py-3">
      {/* Engine toggle */}
      <div className="flex items-center justify-between gap-3 sm:justify-start">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-foreground">Kanban engine</span>
          {engine.enabled && (
            <span className="text-xs bg-success/10 text-success px-2 py-0.5 rounded-full">
              Enabled
            </span>
          )}
        </div>
        <button
          onClick={handleToggle}
          disabled={isToggling || engine.circuitBreakerOpen}
          role="switch"
          aria-checked={engine.enabled}
          aria-label="Toggle Kanban engine"
          className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
            engine.enabled ? 'bg-success/80' : 'bg-muted'
          } ${isToggling || engine.circuitBreakerOpen ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-background shadow-sm transition-transform ${
              engine.enabled ? 'translate-x-4' : 'translate-x-0.5'
            }`}
          />
        </button>
      </div>

      {/* Status indicators */}
      <div className="flex flex-wrap items-center gap-2 text-sm">
        {engine.currentWorkflowId && (
          <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-info/10 text-info text-xs font-medium">
            <span className="w-1.5 h-1.5 bg-info rounded-full animate-pulse" aria-hidden="true" />
            Executing
          </span>
        )}
        {engine.circuitBreakerOpen && (
          <span className="inline-flex items-center gap-2 px-2 py-1 rounded-full bg-error/10 text-error text-xs font-medium">
            <span className="w-1.5 h-1.5 bg-error rounded-full" aria-hidden="true" />
            Circuit breaker open ({engine.consecutiveFailures} failures)
            <button
              onClick={handleReset}
              className="text-xs px-2 py-0.5 rounded-md bg-error/10 hover:bg-error/20 transition-colors"
              type="button"
            >
              Reset
            </button>
          </span>
        )}
      </div>
    </div>
  );
}

// Main Kanban page
export default function Kanban() {
  const {
    columns,
    fetchBoard,
    loading,
    error,
    clearError,
  } = useKanbanStore();

  useEffect(() => {
    fetchBoard();
  }, [fetchBoard]);

  if (loading && Object.values(columns).every(col => col.length === 0)) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-muted-foreground">Loading board...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Kanban</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Visualize and manage workflow execution
          </p>
        </div>
        <EngineControls />
      </div>

      {error && (
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-4 bg-error/10 border border-error/20 rounded-xl">
          <p className="text-error text-sm">{error}</p>
          <div className="flex items-center gap-2">
            <button
              onClick={fetchBoard}
              className="px-3 py-1.5 rounded-lg bg-background text-foreground text-sm font-medium hover:bg-accent transition-colors"
              type="button"
            >
              Retry
            </button>
            <button
              onClick={clearError}
              className="px-3 py-1.5 rounded-lg text-error hover:bg-error/10 transition-colors"
              type="button"
              aria-label="Dismiss error"
              title="Dismiss"
            >
              &times;
            </button>
          </div>
        </div>
      )}

      {/* Columns */}
      <div className="flex gap-4 overflow-x-auto pb-4">
        {KANBAN_COLUMNS.map((column) => (
          <KanbanColumn
            key={column.id}
            column={column}
            workflows={columns[column.id] || []}
          />
        ))}
      </div>
    </div>
  );
}
