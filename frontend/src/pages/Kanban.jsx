import { useEffect, useRef, useState, useMemo } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useKanbanStore, KANBAN_COLUMNS } from '../stores';
import { getStatusColor, KANBAN_COLUMN_COLORS } from '../lib/theme';
import { 
  Search, 
  ChevronLeft, 
  ChevronRight, 
  X, 
  ChevronDown, 
  GitPullRequest, 
  ListTodo, 
  Play, 
  AlertCircle, 
  Clock, 
  Inbox, 
  FolderKanban, 
  Sparkles, 
  Info,
  Settings,
  Cpu,
  Zap,
  LayoutDashboard,
  MoreVertical
} from 'lucide-react';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';
import { Button } from '../components/ui/Button';

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
      <div className="relative w-full bg-card rounded-t-3xl shadow-xl border-t border-border p-6 animate-fade-up max-w-md mx-auto">
        <div className="w-12 h-1.5 bg-muted rounded-full mx-auto mb-8" />
        
        <div className="mb-8">
          <div className="flex items-center gap-2 mb-1">
             <p className="text-[10px] font-black uppercase tracking-[0.2em] text-primary">Workflow Transition</p>
          </div>
          <h3 className="text-xl font-bold text-foreground tracking-tight">
            {workflow.title || workflow.id}
          </h3>
        </div>

        <div className="space-y-3">
          <p className="text-[10px] font-bold text-muted-foreground uppercase tracking-widest mb-2 px-1">Select Target Phase</p>
          <div className="grid grid-cols-1 gap-2">
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
                   className={`w-full flex items-center justify-between p-4 rounded-2xl border transition-all ${
                     isCurrent 
                       ? 'bg-secondary/30 border-transparent opacity-40 grayscale cursor-default' 
                       : 'bg-background border-border hover:border-primary/30 hover:bg-accent/50 active:scale-[0.98]'
                   }`}
                 >
                   <div className="flex items-center gap-3">
                      <div className={`w-2.5 h-2.5 rounded-full ${accent.dot} shadow-sm`} />
                      <span className="font-bold text-foreground">{col.name}</span>
                   </div>
                   {isCurrent && <Badge variant="outline" className="text-[9px] uppercase tracking-tighter">Active</Badge>}
                 </button>
               );
            })}
          </div>
        </div>

        <Button 
          variant="ghost"
          onClick={onClose}
          className="w-full mt-8 h-12 rounded-2xl font-bold text-muted-foreground hover:text-foreground"
        >
          Dismiss
        </Button>
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

  // Accent color for the column header
  const accent = KANBAN_COLUMN_COLORS[column.id] || KANBAN_COLUMN_COLORS.default;

  // On mobile, use the column's tint color for better visual distinction
  const bgClass = isMobile ? accent.tint : 'bg-card/20';

  return (
    <div
      className={`flex flex-col ${isMobile ? 'w-[85vw] flex-shrink-0 snap-center h-full mx-2 first:ml-4 last:mr-4' : 'min-w-[300px] max-w-[340px] flex-1'} h-full transition-all duration-300 rounded-2xl border border-border/50 ${bgClass} backdrop-blur-md overflow-hidden ${
        isOver && canDrop ? 'ring-2 ring-primary/20 bg-primary/5 scale-[1.01]' : ''
      }`}
      onDragOver={handleDragOver}
      onDrop={handleDrop}
      onDragEnter={handleDragEnter}
      onDragLeave={handleDragLeave}
    >
      {/* Column header - Modern & Bold */}
      <div className={`flex items-center justify-between p-4 border-b border-border/40 bg-card/30 border-t-4 ${accent.border}`}>
        <div className="flex items-center gap-3">
          <div className={`w-2 h-2 rounded-full ${accent.dot} shadow-[0_0_8px_rgba(var(--color-primary),0.4)]`} />
          <h3 className="font-black text-foreground text-[11px] uppercase tracking-[0.2em]">{column.name}</h3>
        </div>
        <Badge variant="secondary" className="px-1.5 py-0 bg-background/50 border-border/40 text-[10px] font-black">
            {workflows.length}
        </Badge>
      </div>

      {/* Workflow cards container */}
      <div className="flex-1 p-3 space-y-3 overflow-y-auto min-h-0 scrollbar-thin scrollbar-thumb-muted">
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
        
        {/* Empty State */}
        {workflows.length === 0 && (
          <div className={`flex flex-col items-center justify-center py-16 rounded-xl border-2 border-dashed transition-all duration-500 ${
            isOver && canDrop 
              ? 'border-primary/40 bg-primary/5 scale-[0.98]' 
              : 'border-border/20 bg-transparent text-muted-foreground/20'
          }`}>
             {isOver && canDrop ? (
                 <Inbox className="w-10 h-10 text-primary/40 mb-3 animate-bounce" />
             ) : (
                 <div className="flex flex-col items-center opacity-40">
                    <Clock className="w-8 h-8 mb-2" />
                    <span className="text-[10px] font-black uppercase tracking-widest">Awaiting Items</span>
                 </div>
             )}
          </div>
        )}
      </div>
    </div>
  );
}

// Card component - Redesigned to match /projects and /templates
function KanbanCard({ workflow, isExecuting, onDragStart, onDragEnd, onClick, onOpenMenu }) {
  const handleDragStart = (e) => {
    e.dataTransfer.setData('text/plain', workflow.id);
    e.dataTransfer.effectAllowed = 'move';
    onDragStart();
  };

  const statusColor = getStatusColor(workflow.status);
  
  // Dynamic border color based on status
  const cardStyle = {
    borderLeftColor: statusColor.borderStrip ? `var(--${statusColor.borderStrip.split('-')[1]})` : 'transparent',
    borderLeftWidth: '4px'
  };

  return (
    <div
      draggable
      onDragStart={handleDragStart}
      onDragEnd={onDragEnd}
      onClick={onClick}
      className={`group relative flex flex-col gap-4 rounded-xl border border-border bg-card/80 p-4 shadow-sm transition-all duration-300 hover:shadow-xl hover:-translate-y-1 cursor-grab active:cursor-grabbing ${
        isExecuting 
          ? 'ring-2 ring-primary/30 shadow-primary/10' 
          : 'hover:border-primary/30'
      }`}
      style={cardStyle}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') onClick?.();
      }}
    >
      {/* Execution Indicator */}
      {isExecuting && (
        <div className="absolute top-3 right-3 flex items-center gap-1.5 text-[9px] font-black text-primary bg-primary/10 px-2 py-0.5 rounded-full border border-primary/20 animate-fade-in">
          <Zap className="w-3 h-3 fill-current animate-pulse" />
          EXECUTING
        </div>
      )}

      {/* Header & Title */}
      <div className="space-y-2">
         <h4 className="font-bold text-foreground text-sm leading-tight line-clamp-2 pr-12 group-hover:text-primary transition-colors">
            {workflow.title || "Untitled Blueprint"}
         </h4>
         <p className="text-muted-foreground text-[10px] line-clamp-2 font-mono opacity-60 leading-relaxed italic border-l border-border/50 pl-2">
            &ldquo;{workflow.prompt}&rdquo;
         </p>
      </div>

      {/* Footer - Metadata & Status */}
      <div className="flex items-center justify-between pt-3 mt-1 border-t border-border/30">
        <div className="flex items-center gap-3 text-muted-foreground">
            {/* PR Link */}
            {workflow.pr_url && (
                <a
                    href={workflow.pr_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    className="flex items-center gap-1.5 text-[10px] font-bold hover:text-primary transition-colors"
                    title={`Pull Request #${workflow.pr_number}`}
                >
                <GitPullRequest className="w-3 h-3" />
                <span>#{workflow.pr_number}</span>
                </a>
            )}

            {/* Tasks */}
            {workflow.task_count > 0 && (
                <div className="flex items-center gap-1.5 text-[10px] font-bold opacity-60" title={`${workflow.task_count} sub-tasks`}>
                    <ListTodo className="w-3 h-3" />
                    <span>{workflow.task_count}</span>
                </div>
            )}
        </div>

        {/* Status Badge */}
        <Badge variant="secondary" className={`text-[9px] py-0 font-black uppercase tracking-widest ${statusColor.bg} ${statusColor.text} border-transparent`}>
            {workflow.status || 'pending'}
        </Badge>
      </div>
      
      {/* Mobile Menu Trigger */}
      <button 
          onClick={(e) => {
              e.stopPropagation();
              onOpenMenu();
          }}
          className="absolute top-2 right-2 md:hidden p-2 text-muted-foreground/30 hover:text-foreground hover:bg-accent rounded-lg transition-all"
      >
          <MoreVertical className="w-4 h-4" />
      </button>
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
    <div
      className="flex items-center justify-between gap-4 rounded-xl border border-border bg-card/50 backdrop-blur-sm px-4 py-2 shrink-0 shadow-sm"
      title="Auto-execute workflows from Todo column"
    >
      {/* Engine toggle */}
      <div className="flex items-center gap-3">
        <div className={`p-1.5 rounded-lg border transition-colors ${engine.enabled ? 'bg-success/10 border-success/20 text-success' : 'bg-muted border-border text-muted-foreground'}`}>
           <Cpu className={`w-4 h-4 ${engine.enabled ? 'animate-pulse' : ''}`} />
        </div>
        <div>
           <p className="text-[10px] font-black uppercase tracking-widest text-foreground">Autonomous Engine</p>
           <p className="text-[9px] font-bold text-muted-foreground leading-none">{engine.enabled ? 'ACTIVE & MONITORING' : 'READY TO INITIALIZE'}</p>
        </div>
        <button
          onClick={handleToggle}
          disabled={isToggling || engine.circuitBreakerOpen}
          role="switch"
          aria-checked={engine.enabled}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-all duration-300 ${
            engine.enabled ? 'bg-success shadow-lg shadow-success/20' : 'bg-muted'
          } ${isToggling || engine.circuitBreakerOpen ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer hover:scale-105'}`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-background shadow-sm transition-transform duration-300 ${
              engine.enabled ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
      </div>

      {/* Status indicators */}
      <div className="hidden lg:flex items-center gap-3 border-l border-border/50 pl-4 ml-2">
        {engine.currentWorkflowId && (
          <div className="flex items-center gap-2 px-3 py-1 rounded-lg bg-primary/10 text-primary border border-primary/20 animate-fade-in shadow-sm">
            <span className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse" />
            <span className="text-[9px] font-black uppercase tracking-[0.2em]">Executing ID: {engine.currentWorkflowId.substring(0, 8)}</span>
          </div>
        )}
        {engine.circuitBreakerOpen && (
          <div className="flex items-center gap-3 px-3 py-1 rounded-lg bg-error/10 text-error border border-error/20 animate-shake shadow-sm">
            <AlertCircle className="w-3.5 h-3.5" />
            <span className="text-[9px] font-black uppercase tracking-widest">HALTED: {engine.consecutiveFailures} FAILURES</span>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleReset}
              className="h-6 px-2 text-[9px] font-black uppercase tracking-tighter hover:bg-error/20 text-error"
            >
              System Reset
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}

// Filter Component
function KanbanFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-72">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
      <Input
        placeholder="Filter board by blueprint, ID..."
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        className="h-11 pl-10 pr-4 rounded-xl border border-border bg-background shadow-sm focus-visible:ring-primary/20"
      />
    </div>
  );
}

// Mobile Column Navigator
function MobileColumnNav({ columns, activeIndex, onChange }) {
  return (
    <div className="md:hidden flex items-center justify-center bg-background/80 backdrop-blur-md py-3 border-b border-border mb-4 sticky top-14 z-20">
      <div className="flex gap-2.5 items-center">
        {columns.map((col, idx) => {
          const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
          const isActive = idx === activeIndex;
          
          return (
            <button
              key={col.id}
              onClick={() => onChange(idx)}
              className={`h-1.5 rounded-full transition-all duration-500 ${
                isActive 
                  ? `w-12 ${accent.dot} shadow-[0_0_8px_rgba(var(--color-primary),0.4)]` 
                  : 'w-6 bg-muted hover:bg-muted-foreground/30'
              }`}
              title={col.name}
            />
          );
        })}
      </div>
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
      <div className="relative min-h-full">
         <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
         <div className="flex flex-col items-center justify-center h-full py-40 gap-4">
            <Loader2 className="w-10 h-10 animate-spin text-primary" />
            <p className="text-xs font-black uppercase tracking-widest text-muted-foreground">Initializing Vision</p>
         </div>
      </div>
    );
  }

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      {/* Background Pattern - Consistent across app */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header - Unified style */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/30 backdrop-blur-md p-8 sm:p-12 shadow-sm">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/10 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-8">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
              <FolderKanban className="h-3 w-3" />
              Strategic Planning
            </div>
            <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
              Process <span className="text-primary">Kanban</span>
            </h1>
            <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
              Visualize the lifecycle of your automated cycles. Drag and drop blueprints to refine, verify, and complete complex development tasks.
            </p>
          </div>

          <div className="flex shrink-0">
             <div className="p-6 rounded-3xl bg-background/50 border border-border shadow-inner backdrop-blur-sm group hover:border-primary/30 transition-all">
                <LayoutDashboard className="w-12 h-12 text-primary/40 group-hover:text-primary transition-colors" />
             </div>
          </div>
        </div>
      </div>

      {/* Control Bar - Unified Sticky Style */}
      <div className="sticky top-14 z-30 flex flex-col gap-4 bg-background/80 backdrop-blur-md py-4 border-b border-border/50">
        <div className="flex flex-col md:flex-row gap-4 md:items-center justify-between">
          <div className="flex items-center gap-4 flex-1">
             <KanbanFilters filter={filter} setFilter={setFilter} />
             <div className="hidden sm:flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-muted-foreground bg-muted/50 px-3 py-2 rounded-lg border border-border/50">
                <Info className="h-3.5 w-3.5" />
                <span>{Object.values(columns).flat().length} BLUEPRINTS TRACKED</span>
             </div>
          </div>
          <EngineControls />
        </div>
      </div>

      {error && (
        <div className="flex items-center justify-between gap-4 p-4 bg-error/5 border border-error/20 rounded-2xl animate-shake shadow-sm">
          <div className="flex items-center gap-3">
             <AlertCircle className="w-5 h-5 text-error" />
             <p className="text-error text-sm font-bold">{error}</p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={fetchBoard}
              className="rounded-xl font-bold bg-background"
            >
              System Retry
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={clearError}
              className="rounded-xl text-error hover:bg-error/10"
            >
              <X className="w-4 h-4" />
            </Button>
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
      <div className="flex-1 min-h-[60vh]">
        {/* Desktop View */}
        <div className="hidden md:flex gap-6 overflow-x-auto pb-8 h-full scrollbar-thin">
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
            className="md:hidden flex overflow-x-auto snap-x snap-mandatory h-full pb-8 scrollbar-none"
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

function Loader2({ className }) {
  return (
    <svg className={className} xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
}
