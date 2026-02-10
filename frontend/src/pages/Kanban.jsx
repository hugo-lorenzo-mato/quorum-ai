import { useEffect, useRef, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useKanbanStore, KANBAN_COLUMNS } from '../stores';
import { getStatusColor, KANBAN_COLUMN_COLORS } from '../lib/theme';
import { 
  Search, 
  MoreVertical, 
  GitPullRequest, 
  ListTodo, 
  Play, 
  Inbox,
  Zap,
  CheckCircle2,
  Clock,
  AlertCircle,
  Sparkles
} from 'lucide-react';

// Bottom Sheet Component for Mobile Actions
function MobileActionSheet({ isOpen, onClose, workflow, onMoveTo }) {
  if (!isOpen || !workflow) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center md:hidden">
      {/* Backdrop */}
      <button
        type="button"
        className="absolute inset-0 bg-black/60 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
        aria-label="Close"
      />
      
      {/* Sheet Content */}
      <div className="relative w-full bg-gradient-to-b from-card to-card/95 rounded-t-3xl shadow-2xl border-t border-border/50 p-6 animate-fade-up max-w-md mx-auto">
        <div className="w-12 h-1.5 bg-muted/50 rounded-full mx-auto mb-8" />
        
        <div className="mb-6">
          <h3 className="text-lg font-semibold text-foreground mb-2">Move Workflow</h3>
          <p className="text-sm text-muted-foreground line-clamp-2 leading-relaxed">
            {workflow.title || workflow.id}
          </p>
        </div>

        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider mb-3">Select Status</p>
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
                 className={`w-full flex items-center gap-3 p-3.5 rounded-xl border transition-all ${
                   isCurrent 
                     ? 'bg-secondary/50 border-transparent opacity-50 cursor-default' 
                     : 'bg-background/50 border-border/50 hover:bg-accent hover:border-primary/20 active:scale-[0.98]'
                 }`}
               >
                 <span className={`w-2.5 h-2.5 rounded-full ${accent.dot} shadow-sm`} />
                 <span className="font-medium text-foreground">{col.name}</span>
                 {isCurrent && <span className="ml-auto text-xs text-muted-foreground">(Current)</span>}
               </button>
             );
          })}
        </div>

        <button 
          onClick={onClose}
          className="w-full mt-6 p-3 rounded-xl font-medium text-muted-foreground hover:bg-secondary/50 transition-colors"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// Column Icons Mapping
const COLUMN_ICONS = {
  refinement: Sparkles,
  todo: Clock,
  in_progress: Zap,
  to_verify: AlertCircle,
  done: CheckCircle2,
};

// Column component with Vercel-style design
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
  const ColumnIcon = COLUMN_ICONS[column.id] || Clock;

  return (
    <div
      className={`flex flex-col ${
        isMobile 
          ? 'w-[88vw] flex-shrink-0 snap-center h-full mx-2 first:ml-4 last:mr-4' 
          : 'flex-1 min-w-[340px] max-w-[420px]'
      } h-full transition-all duration-300 rounded-2xl border border-border/40 bg-gradient-to-b from-card/80 to-card/40 backdrop-blur-xl shadow-sm hover:shadow-md ${
        isOver && canDrop ? 'ring-2 ring-primary/30 border-primary/30 shadow-lg' : ''
      }`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
    >
      {/* Column header - Enhanced Vercel style */}
      <div className="flex items-center justify-between px-5 py-4 border-b border-border/30 bg-gradient-to-r from-transparent via-muted/5 to-transparent">
        <div className="flex items-center gap-3">
          <div className={`p-2 rounded-lg ${accent.tint} border border-border/50`}>
            <ColumnIcon className={`w-4 h-4 ${accent.dot.replace('bg-', 'text-')}`} />
          </div>
          <div>
            <h3 className="font-semibold text-foreground text-base tracking-tight">{column.name}</h3>
            <p className="text-xs text-muted-foreground mt-0.5">{workflows.length} workflow{workflows.length !== 1 ? 's' : ''}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <span className={`w-2 h-2 rounded-full ${accent.dot} shadow-sm`} />
        </div>
      </div>

      {/* Workflow cards container */}
      <div className="flex-1 px-3 py-4 space-y-3 overflow-y-auto min-h-0 scrollbar-thin">
        {workflows.map((workflow) => (
          <KanbanCard
            key={workflow.id}
            workflow={workflow}
            isExecuting={engine.currentWorkflowId === workflow.id}
            columnAccent={accent}
            onDragStart={() => setDraggedWorkflow(workflow)}
            onDragEnd={() => clearDraggedWorkflow()}
            onClick={() => navigate(`/workflows/${workflow.id}`)}
            onMoveTo={(targetCol) => moveWorkflow(workflow.id, targetCol)}
            onOpenMenu={() => onOpenMobileMenu(workflow)}
          />
        ))}
        
        {/* Empty State - Elegant */}
        {workflows.length === 0 && (
          <div className={`flex flex-col items-center justify-center py-16 rounded-xl border-2 border-dashed transition-all ${
            isOver && canDrop 
              ? 'border-primary/50 bg-primary/5 scale-[0.98]' 
              : 'border-border/30 bg-muted/5'
          }`}>
             {isOver && canDrop ? (
                <>
                  <Inbox className="w-10 h-10 text-primary/60 mb-3 animate-bounce" />
                  <span className="text-sm font-medium text-primary/70">Drop here</span>
                </>
             ) : (
                <>
                  <Inbox className="w-10 h-10 text-muted-foreground/30 mb-3" />
                  <span className="text-sm font-medium text-muted-foreground/50">No workflows</span>
                </>
             )}
          </div>
        )}
      </div>
    </div>
  );
}

// Card component - Modern Vercel-inspired design
function KanbanCard({ workflow, isExecuting, columnAccent, onDragStart, onDragEnd, onClick, onOpenMenu }) {
  const handleDragStart = (e) => {
    e.dataTransfer.setData('text/plain', workflow.id);
    e.dataTransfer.effectAllowed = 'move';
    onDragStart();
  };

  const statusColor = getStatusColor(workflow.status);
  const isCompleted = workflow.status === 'completed';
  const isFailed = workflow.status === 'failed';

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onClick={onClick}
      className={`group relative flex flex-col gap-4 rounded-xl border transition-all cursor-pointer ${
        isExecuting 
          ? 'border-blue-500/40 bg-gradient-to-br from-blue-500/5 via-card to-card shadow-lg shadow-blue-500/10 ring-1 ring-blue-500/20' 
          : isCompleted
          ? 'border-border/50 bg-gradient-to-br from-card via-card to-emerald-500/5 hover:shadow-lg hover:border-emerald-500/30 hover:-translate-y-0.5'
          : isFailed
          ? 'border-border/50 bg-gradient-to-br from-card via-card to-rose-500/5 hover:shadow-lg hover:border-rose-500/30 hover:-translate-y-0.5'
          : 'border-border/50 bg-gradient-to-br from-card via-card to-card hover:shadow-lg hover:border-primary/30 hover:-translate-y-0.5'
      } p-4 shadow-sm backdrop-blur-sm active:cursor-grabbing active:scale-[0.98]`}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onClick?.();
      }}
    >
      {/* Top accent line */}
      <div className={`absolute top-0 left-4 right-4 h-0.5 rounded-full ${
        isExecuting 
          ? 'bg-gradient-to-r from-transparent via-blue-500 to-transparent' 
          : isCompleted
          ? 'bg-gradient-to-r from-transparent via-emerald-500 to-transparent'
          : isFailed
          ? 'bg-gradient-to-r from-transparent via-rose-500 to-transparent'
          : `bg-gradient-to-r from-transparent ${columnAccent.dot.replace('bg-', 'via-')} to-transparent`
      }`} />

      {/* Execution Indicator */}
      {isExecuting && (
        <div className="absolute -top-2 -right-2 flex items-center gap-1.5 text-[10px] font-bold text-blue-600 dark:text-blue-400 bg-blue-500/10 backdrop-blur-sm px-2.5 py-1.5 rounded-full border border-blue-500/20 shadow-sm">
          <Play className="w-3 h-3 fill-current animate-pulse" />
          EXECUTING
        </div>
      )}

      {/* Content */}
      <div className="space-y-2">
         <h4 className="font-semibold text-foreground text-base leading-snug line-clamp-2 pr-8 group-hover:text-primary transition-colors">
            {workflow.title || "Untitled Workflow"}
         </h4>
         {workflow.prompt && (
           <p className="text-muted-foreground text-sm line-clamp-2 leading-relaxed">
              {workflow.prompt}
           </p>
         )}
      </div>

      {/* Footer - Metadata & Status */}
      <div className="flex items-center justify-between pt-3 mt-auto border-t border-border/30">
        <div className="flex items-center gap-3 text-muted-foreground">
            {/* PR Link */}
            {workflow.pr_url && (
                <a
                    href={workflow.pr_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    className="flex items-center gap-1.5 text-xs hover:text-primary transition-colors px-2 py-1 rounded-md hover:bg-primary/5"
                    title={`PR #${workflow.pr_number}`}
                >
                <GitPullRequest className="w-3.5 h-3.5" />
                <span className="font-medium">#{workflow.pr_number}</span>
                </a>
            )}

            {/* Tasks */}
            {workflow.task_count > 0 && (
                <div className="flex items-center gap-1.5 text-xs px-2 py-1 rounded-md bg-muted/50" title={`${workflow.task_count} tasks`}>
                    <ListTodo className="w-3.5 h-3.5" />
                    <span className="font-medium">{workflow.task_count}</span>
                </div>
            )}
        </div>

        {/* Status Badge */}
        <div className={`flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-semibold uppercase tracking-wide border ${statusColor.bg} ${statusColor.text} ${statusColor.border}`}>
            {workflow.status || 'pending'}
        </div>
      </div>
      
      {/* Mobile Menu Trigger */}
      <button 
          onClick={(e) => {
              e.stopPropagation();
              onOpenMenu();
          }}
          className="absolute top-3 right-3 md:hidden p-2 text-muted-foreground/50 hover:text-foreground hover:bg-muted/50 rounded-lg transition-colors"
      >
          <MoreVertical className="w-4 h-4" />
      </button>
    </div>
  );
}

// Engine controls component - Enhanced
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
    <div
      className="flex items-center justify-between gap-3 rounded-xl border border-border/50 bg-gradient-to-br from-card/80 to-card/40 backdrop-blur-xl px-4 py-3 shadow-sm shrink-0"
      title="Auto-execute workflows from Todo column"
    >
      {/* Engine toggle */}
      <div className="flex items-center gap-3">
        <Zap className={`w-4 h-4 ${engine.enabled ? 'text-emerald-500' : 'text-muted-foreground'}`} />
        <span className="text-sm font-semibold text-foreground">Auto</span>
        <button
          onClick={handleToggle}
          disabled={isToggling || engine.circuitBreakerOpen}
          role="switch"
          aria-checked={engine.enabled}
          aria-label="Toggle auto-execution of workflows"
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-all ${
            engine.enabled ? 'bg-emerald-500 shadow-sm shadow-emerald-500/20' : 'bg-muted'
          } ${isToggling || engine.circuitBreakerOpen ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer hover:scale-105'}`}
        >
          <span
            className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform ${
              engine.enabled ? 'translate-x-5' : 'translate-x-0.5'
            }`}
          />
        </button>
      </div>

      {/* Status indicators */}
      <div className="hidden sm:flex flex-wrap items-center gap-2 text-sm">
        {engine.currentWorkflowId && (
          <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400 text-xs font-semibold border border-blue-500/20">
            <span className="w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse" aria-hidden="true" />
            Executing
          </span>
        )}
        {engine.circuitBreakerOpen && (
          <span className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-rose-500/10 text-rose-600 dark:text-rose-400 text-xs font-semibold border border-rose-500/20">
            <span className="w-1.5 h-1.5 bg-rose-500 rounded-full" aria-hidden="true" />
            Circuit breaker ({engine.consecutiveFailures})
            <button
              onClick={handleReset}
              className="ml-1 text-xs px-2 py-0.5 rounded-md bg-rose-500/10 hover:bg-rose-500/20 transition-colors font-medium"
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

// Filter Component - Enhanced
function KanbanFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-72">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <input
        type="text"
        placeholder="Search workflows..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="h-10 w-full pl-10 pr-4 rounded-xl border border-border/50 bg-background/50 backdrop-blur-sm text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary/30 transition-all"
      />
    </div>
  );
}

// Mobile Column Navigator - Enhanced
function MobileColumnNav({ columns, activeIndex, onChange }) {
  return (
    <div className="md:hidden flex items-center justify-center bg-background/80 backdrop-blur-xl py-3 border-b border-border/50 shrink-0 z-20">
      <div className="flex gap-2.5 items-center px-4">
        {columns.map((col, idx) => {
          const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
          const isActive = idx === activeIndex;
          
          return (
            <button
              key={col.id}
              onClick={() => onChange(idx)}
              className={`h-1.5 rounded-full transition-all duration-300 ${
                isActive 
                  ? `w-16 ${accent.dot} shadow-sm` 
                  : 'w-8 bg-muted hover:bg-muted-foreground/30'
              }`}
              title={col.name}
              aria-label={`Go to ${col.name} column`}
            />
          );
        })}
      </div>
    </div>
  );
}

// Main Kanban page - Enhanced
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
        <div className="flex flex-col items-center gap-3">
          <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          <div className="text-muted-foreground text-sm font-medium">Loading board...</div>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-x-0 top-14 bottom-[calc(4rem+env(safe-area-inset-bottom))] md:static md:h-[calc(100vh-3.5rem)] md:inset-auto flex flex-col overflow-hidden animate-fade-in bg-background relative">
      {/* Background Pattern */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none opacity-50" />
      
      {/* Header - Enhanced Vercel style */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 px-4 py-3 shrink-0 relative z-10 border-b border-border/50 bg-card/30">
        <div className="hidden lg:block">
          <h1 className="text-3xl font-bold text-foreground tracking-tight bg-clip-text bg-gradient-to-r from-foreground to-foreground/70">
            Kanban Board
          </h1>
          <p className="text-sm text-muted-foreground mt-1.5 leading-relaxed">
            Visualize and manage workflow execution with drag & drop
          </p>
        </div>
        <div className="flex items-center gap-3 w-full lg:w-auto">
          <KanbanFilters filter={filter} setFilter={setFilter} />
          <EngineControls />
        </div>
      </div>

      {error && (
        <div className="mx-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-4 bg-gradient-to-r from-rose-500/10 to-rose-500/5 border border-rose-500/20 rounded-xl shrink-0 backdrop-blur-sm">
          <p className="text-rose-600 dark:text-rose-400 text-sm font-medium">{error}</p>
          <div className="flex items-center gap-2">
            <button
              onClick={fetchBoard}
              className="px-3 py-1.5 rounded-lg bg-background text-foreground text-sm font-medium hover:bg-accent transition-colors shadow-sm"
              type="button"
            >
              Retry
            </button>
            <button
              onClick={clearError}
              className="px-3 py-1.5 rounded-lg text-rose-600 dark:text-rose-400 hover:bg-rose-500/10 transition-colors"
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
      <div className="flex-1 min-h-0 px-4 pt-4">
        {/* Desktop View - Better spacing */}
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
            className="md:hidden flex overflow-x-auto snap-x snap-mandatory h-full pb-4 scrollbar-none -mx-4"
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
