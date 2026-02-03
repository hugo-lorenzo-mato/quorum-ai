import { useEffect, useRef, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useKanbanStore, KANBAN_COLUMNS } from '../stores';
import { getStatusColor, KANBAN_COLUMN_COLORS } from '../lib/theme';
import { Search, ChevronLeft, ChevronRight, X, ChevronDown } from 'lucide-react';

// Bottom Sheet Component for Mobile Actions
function MobileActionSheet({ isOpen, onClose, workflow, onMoveTo }) {
  if (!isOpen || !workflow) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center md:hidden">
      {/* Backdrop */}
      <div 
        className="absolute inset-0 bg-black/60 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
      />
      
      {/* Sheet Content */}
      <div className="relative w-full bg-card rounded-t-2xl shadow-xl border-t border-border p-4 animate-fade-up max-w-md mx-auto">
        <div className="w-12 h-1.5 bg-muted rounded-full mx-auto mb-6" />
        
        <div className="mb-6">
          <h3 className="text-lg font-semibold text-foreground mb-1">Move Workflow</h3>
          <p className="text-sm text-muted-foreground line-clamp-1">
            {workflow.title || workflow.id}
          </p>
        </div>

        <div className="space-y-2.5">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-2">Select Status</p>
          {KANBAN_COLUMNS.map(col => {
             const isCurrent = workflow.kanban_column === col.id;
             const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
             
             return (
               <button
                 key={col.id}
                 onClick={() => {
                   if (!isCurrent) {
                     onMoveTo(col.id);
                     onClose();
                   }
                 }}
                 disabled={isCurrent}
                 className={`w-full flex items-center gap-3 p-3 rounded-xl border transition-all ${
                   isCurrent 
                     ? 'bg-secondary/50 border-transparent opacity-50 cursor-default' 
                     : 'bg-background border-border hover:bg-accent active:scale-[0.98]'
                 }`}
               >
                 <span className={`w-3 h-3 rounded-full ${accent.dot}`} />
                 <span className="font-medium text-foreground">{col.name}</span>
                 {isCurrent && <span className="ml-auto text-xs text-muted-foreground">(Current)</span>}
               </button>
             );
          })}
        </div>

        <button 
          onClick={onClose}
          className="w-full mt-6 p-3 rounded-xl font-medium text-muted-foreground hover:bg-secondary transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// Column component
function KanbanColumn({ column, workflows, isMobile, onOpenMobileMenu }) {
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

  const accent = KANBAN_COLUMN_COLORS[column.id] || KANBAN_COLUMN_COLORS.default;

  return (
    <div
      className={`flex flex-col ${isMobile ? 'w-[85vw] flex-shrink-0 snap-center h-full mx-2 first:ml-4 last:mr-4' : 'min-w-[280px] max-w-[320px]'} rounded-xl border border-border bg-card/60 backdrop-blur-xl dark:bg-card/40 transition-colors ${
        isOver && canDrop ? `ring-2 ring-dashed ${accent.ring} bg-accent/50` : ''
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
          <span className="shrink-0 bg-muted text-muted-foreground text-xs px-2 py-0.5 rounded-full font-mono">
            {workflows.length}
          </span>
        </div>
      </div>

      {/* Workflow cards */}
      <div className="flex-1 p-3 space-y-2 overflow-y-auto max-h-[calc(100vh-320px)] scrollbar-thin">
        {workflows.map((workflow) => (
          <KanbanCard
            key={workflow.id}
            workflow={workflow}
            isExecuting={engine.currentWorkflowId === workflow.id}
            onDragStart={() => setDraggedWorkflow(workflow)}
            onDragEnd={() => clearDraggedWorkflow()}
            onClick={() => navigate(`/workflows/${workflow.id}`)}
            onMoveTo={(targetCol) => moveWorkflow(workflow.id, targetCol)}
            onOpenMenu={() => onOpenMobileMenu(workflow)}
          />
        ))}
        {workflows.length === 0 && (
          <div className="flex flex-col items-center justify-center py-8 text-muted-foreground/50 border-2 border-dashed border-border/50 rounded-lg m-1">
             <span className="text-xs">Empty</span>
          </div>
        )}
      </div>
    </div>
  );
}

// Card component
function KanbanCard({ workflow, isExecuting, onDragStart, onDragEnd, onClick, onOpenMenu }) {
  const handleDragStart = (e) => {
    e.dataTransfer.setData('text/plain', workflow.id);
    e.dataTransfer.effectAllowed = 'move';
    onDragStart();
  };

  const statusColor = getStatusColor(workflow.status);

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onClick={onClick}
      className={`group relative overflow-hidden rounded-xl border border-border bg-card p-3 shadow-sm shadow-black/5 cursor-grab active:cursor-grabbing transition-all hover:bg-accent hover:border-muted-foreground/30 hover:shadow-md hover:-translate-y-0.5 active:translate-y-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/30 dark:bg-secondary dark:border-white/10 dark:shadow-md dark:shadow-black/60 dark:ring-1 dark:ring-white/10 ${
        isExecuting ? 'ring-2 ring-info/30 dark:ring-info/40' : ''
      } before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:content-[''] before:bg-gradient-to-b before:from-white/0 before:to-transparent dark:before:from-white/10`}
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
      <p className="text-muted-foreground text-xs line-clamp-2 mb-3 font-mono text-[10px] leading-tight opacity-80">
        {workflow.prompt}
      </p>

      {/* Metadata row */}
      <div className="flex items-center justify-between gap-2 text-xs">
        {/* Status Badge - Interactive Trigger for Mobile */}
        <button 
          onClick={(e) => {
            // Only stop propagation on mobile to open menu
            if (window.innerWidth < 768) {
              e.stopPropagation();
              onOpenMenu();
            }
          }}
          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full font-medium transition-transform active:scale-95 ${statusColor.bg} ${statusColor.text}`}
        >
          {workflow.status || 'pending'}
          <ChevronDown className="w-3 h-3 md:hidden opacity-50" />
        </button>
        
        {/* Desktop: Details */}
        <div className="hidden md:flex items-center gap-2 text-muted-foreground">
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
          className="mt-2 rounded-lg border border-error/20 bg-error/5 px-2 py-1 text-xs text-error line-clamp-2 font-mono"
          title={workflow.kanban_last_error}
        >
          {workflow.kanban_last_error}
        </div>
      )}

      {/* Task count */}
      {workflow.task_count > 0 && (
        <div className="mt-2 text-xs text-muted-foreground font-mono">
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

// Filter Component
function KanbanFilters({ filter, setFilter }) {
  return (
    <div className="relative">
      <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <input
        type="text"
        placeholder="Filter workflows..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="h-9 w-full sm:w-64 pl-9 pr-4 rounded-lg border border-border bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring/20 transition-all"
      />
    </div>
  );
}

// Mobile Column Navigator
function MobileColumnNav({ columns, activeIndex, onChange }) {
  return (
    <div className="md:hidden flex items-center justify-between bg-card/50 backdrop-blur-md p-2 rounded-lg border border-border mb-4">
      <button
        onClick={() => onChange(Math.max(0, activeIndex - 1))}
        disabled={activeIndex === 0}
        className="p-2 rounded-md hover:bg-accent disabled:opacity-30 transition-colors"
      >
        <ChevronLeft className="w-5 h-5" />
      </button>
      
      <div className="flex gap-1.5 overflow-x-auto scrollbar-none px-2">
        {columns.map((col, idx) => {
          const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
          const isActive = idx === activeIndex;
          
          return (
            <button
              key={col.id}
              onClick={() => onChange(idx)}
              className={`h-2 transition-all rounded-full ${
                isActive 
                  ? `w-8 ${accent.dot.replace('bg-', 'bg-')}` // Use the dot color class
                  : 'w-2 bg-muted-foreground/30'
              }`}
              title={col.name}
            />
          );
        })}
      </div>
      
      <button
        onClick={() => onChange(Math.min(columns.length - 1, activeIndex + 1))}
        disabled={activeIndex === columns.length - 1}
        className="p-2 rounded-md hover:bg-accent disabled:opacity-30 transition-colors"
      >
        <ChevronRight className="w-5 h-5" />
      </button>
    </div>
  );
}

// Main Kanban page
export default function Kanban() {
  const {
    columns,
    fetchBoard,
    moveWorkflow,
    loading,
    error,
    clearError,
  } = useKanbanStore();

  const [filter, setFilter] = useState('');
  const [activeColIndex, setActiveColIndex] = useState(0);
  
  // Mobile Menu State
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [selectedWorkflow, setSelectedWorkflow] = useState(null);
  
  const scrollRef = useRef(null);

  useEffect(() => {
    fetchBoard();
  }, [fetchBoard]);

  const handleOpenMobileMenu = (workflow) => {
    setSelectedWorkflow(workflow);
    setMobileMenuOpen(true);
  };

  const handleMoveFromSheet = (targetColumnId) => {
    if (selectedWorkflow) {
      moveWorkflow(selectedWorkflow.id, targetColumnId);
    }
  };

  // Handle scroll to update active index
  const handleScroll = (e) => {
    const container = e.target;
    const scrollLeft = container.scrollLeft;
    const width = container.clientWidth;
    const totalWidth = container.scrollWidth;
    const progress = (scrollLeft + width / 2) / totalWidth;
    const index = Math.floor(progress * KANBAN_COLUMNS.length);
    if (index !== activeColIndex && index >= 0 && index < KANBAN_COLUMNS.length) {
      setActiveColIndex(index);
    }
  };

  const handleNavClick = (index) => {
    setActiveColIndex(index);
    if (scrollRef.current) {
        const container = scrollRef.current;
        // Scroll to the child element
        const child = container.children[index];
        if (child) {
            child.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' });
        }
    }
  };

  // Filter workflows
  const filteredColumns = useMemo(() => {
    if (!filter) return columns;
    const lowerFilter = filter.toLowerCase();
    
    const newColumns = {};
    Object.keys(columns).forEach(key => {
      newColumns[key] = columns[key].filter(w => 
        (w.title && w.title.toLowerCase().includes(lowerFilter)) ||
        (w.id && w.id.toLowerCase().includes(lowerFilter)) ||
        (w.prompt && w.prompt.toLowerCase().includes(lowerFilter))
      );
    });
    return newColumns;
  }, [columns, filter]);

  if (loading && Object.values(columns).every(col => col.length === 0)) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-muted-foreground animate-pulse">Loading board...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in flex flex-col h-[calc(100vh-8rem)]">
      {/* Header */}
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between shrink-0">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Kanban</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Visualize and manage workflow execution
          </p>
        </div>
        <div className="flex flex-col sm:flex-row sm:items-center gap-3">
          <KanbanFilters filter={filter} setFilter={setFilter} />
          <EngineControls />
        </div>
      </div>

      {error && (
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-4 bg-error/10 border border-error/20 rounded-xl shrink-0">
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

      {/* Mobile Column Navigation */}
      <MobileColumnNav 
        columns={KANBAN_COLUMNS} 
        activeIndex={activeColIndex} 
        onChange={handleNavClick} 
      />

      {/* Columns Container */}
      <div className="flex-1 min-h-0">
        {/* Desktop View */}
        <div className="hidden md:flex gap-4 overflow-x-auto pb-4 h-full">
          {KANBAN_COLUMNS.map((column) => (
            <KanbanColumn
              key={column.id}
              column={column}
              workflows={filteredColumns[column.id] || []}
              onOpenMobileMenu={handleOpenMobileMenu}
            />
          ))}
        </div>

        {/* Mobile View - Scroll Snap Carousel */}
        <div 
            ref={scrollRef}
            onScroll={handleScroll}
            className="md:hidden flex overflow-x-auto snap-x snap-mandatory h-full pb-4 scrollbar-none"
        >
          {KANBAN_COLUMNS.map((column) => (
            <KanbanColumn
              key={column.id}
              column={column}
              workflows={filteredColumns[column.id] || []}
              isMobile={true}
              onOpenMobileMenu={handleOpenMobileMenu}
            />
          ))}
        </div>
      </div>

      {/* Mobile Action Sheet */}
      <MobileActionSheet 
        isOpen={mobileMenuOpen}
        onClose={() => setMobileMenuOpen(false)}
        workflow={selectedWorkflow}
        onMoveTo={handleMoveFromSheet}
      />
    </div>
  );
}