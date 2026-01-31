import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useKanbanStore, KANBAN_COLUMNS } from '../stores';

// Column component
function KanbanColumn({ column, workflows, onDrop, isDropTarget }) {
  const { moveWorkflow, setDraggedWorkflow, clearDraggedWorkflow, engine } = useKanbanStore();
  const navigate = useNavigate();

  const handleDragOver = (e) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  const handleDrop = (e) => {
    e.preventDefault();
    const workflowId = e.dataTransfer.getData('text/plain');
    if (workflowId) {
      moveWorkflow(workflowId, column.id);
    }
    clearDraggedWorkflow();
  };

  const handleDragEnter = (e) => {
    e.preventDefault();
  };

  const handleDragLeave = (e) => {
    e.preventDefault();
  };

  // Determine column header color
  const getColumnColor = () => {
    switch (column.id) {
      case 'refinement': return 'bg-yellow-500';
      case 'todo': return 'bg-blue-500';
      case 'in_progress': return 'bg-purple-500';
      case 'to_verify': return 'bg-orange-500';
      case 'done': return 'bg-green-500';
      default: return 'bg-gray-500';
    }
  };

  return (
    <div
      className={`flex flex-col min-w-[280px] max-w-[320px] bg-slate-800 rounded-lg ${
        isDropTarget ? 'ring-2 ring-blue-400' : ''
      }`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
    >
      {/* Column header */}
      <div className={`${getColumnColor()} px-3 py-2 rounded-t-lg flex items-center justify-between`}>
        <div>
          <h3 className="font-semibold text-white">{column.name}</h3>
          <p className="text-xs text-white/70">{column.description}</p>
        </div>
        <span className="bg-white/20 text-white text-xs px-2 py-1 rounded-full">
          {workflows.length}
        </span>
      </div>

      {/* Workflow cards */}
      <div className="flex-1 p-2 space-y-2 overflow-y-auto max-h-[calc(100vh-240px)]">
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
          <div className="text-slate-500 text-sm text-center py-4">
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

  // Status badge colors
  const getStatusColor = () => {
    switch (workflow.status) {
      case 'running': return 'bg-blue-500';
      case 'completed': return 'bg-green-500';
      case 'failed': return 'bg-red-500';
      case 'paused': return 'bg-yellow-500';
      default: return 'bg-slate-500';
    }
  };

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onClick={onClick}
      className={`bg-slate-700 rounded-lg p-3 cursor-pointer hover:bg-slate-600 transition-colors ${
        isExecuting ? 'ring-2 ring-purple-400 animate-pulse' : ''
      }`}
    >
      {/* Title */}
      <h4 className="font-medium text-white text-sm mb-1 line-clamp-2">
        {workflow.title || workflow.id}
      </h4>

      {/* Prompt preview */}
      <p className="text-slate-400 text-xs line-clamp-2 mb-2">
        {workflow.prompt}
      </p>

      {/* Metadata row */}
      <div className="flex items-center justify-between text-xs">
        <span className={`${getStatusColor()} text-white px-2 py-0.5 rounded`}>
          {workflow.status}
        </span>
        {workflow.pr_url && (
          <a
            href={workflow.pr_url}
            target="_blank"
            rel="noopener noreferrer"
            onClick={(e) => e.stopPropagation()}
            className="text-blue-400 hover:text-blue-300"
          >
            PR #{workflow.pr_number}
          </a>
        )}
        {workflow.kanban_execution_count > 0 && (
          <span className="text-slate-500">
            Run {workflow.kanban_execution_count}x
          </span>
        )}
      </div>

      {/* Error indicator */}
      {workflow.kanban_last_error && (
        <div className="mt-2 text-xs text-red-400 line-clamp-1" title={workflow.kanban_last_error}>
          {workflow.kanban_last_error}
        </div>
      )}

      {/* Task count */}
      {workflow.task_count > 0 && (
        <div className="mt-2 text-xs text-slate-500">
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
    error,
    clearError,
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
    <div className="flex items-center gap-4 bg-slate-800 rounded-lg px-4 py-3">
      {/* Engine toggle */}
      <div className="flex items-center gap-3">
        <span className="text-sm text-slate-300">Kanban Engine</span>
        <button
          onClick={handleToggle}
          disabled={isToggling || engine.circuitBreakerOpen}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            engine.enabled ? 'bg-green-500' : 'bg-slate-600'
          } ${isToggling || engine.circuitBreakerOpen ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              engine.enabled ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
      </div>

      {/* Status indicators */}
      <div className="flex items-center gap-2 text-sm">
        {engine.currentWorkflowId && (
          <span className="flex items-center gap-1 text-purple-400">
            <span className="w-2 h-2 bg-purple-400 rounded-full animate-pulse" />
            Executing
          </span>
        )}
        {engine.circuitBreakerOpen && (
          <span className="flex items-center gap-2 text-red-400">
            <span className="w-2 h-2 bg-red-400 rounded-full" />
            Circuit Breaker Open ({engine.consecutiveFailures} failures)
            <button
              onClick={handleReset}
              className="text-xs bg-red-500/20 hover:bg-red-500/30 px-2 py-1 rounded"
            >
              Reset
            </button>
          </span>
        )}
      </div>

      {/* Error display */}
      {error && (
        <div className="flex items-center gap-2 text-red-400 text-sm">
          <span>{error}</span>
          <button onClick={clearError} className="text-red-300 hover:text-red-200">
            &times;
          </button>
        </div>
      )}
    </div>
  );
}

// Main Kanban page
export default function Kanban() {
  const { columns, fetchBoard, loading, draggedWorkflow } = useKanbanStore();

  useEffect(() => {
    fetchBoard();
  }, [fetchBoard]);

  if (loading && Object.values(columns).every(col => col.length === 0)) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-slate-400">Loading board...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col gap-4 p-4">
      {/* Header with engine controls */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-white">Kanban Board</h1>
        <EngineControls />
      </div>

      {/* Columns */}
      <div className="flex-1 flex gap-4 overflow-x-auto pb-4">
        {KANBAN_COLUMNS.map((column) => (
          <KanbanColumn
            key={column.id}
            column={column}
            workflows={columns[column.id] || []}
            isDropTarget={draggedWorkflow && draggedWorkflow.kanban_column !== column.id}
          />
        ))}
      </div>
    </div>
  );
}
