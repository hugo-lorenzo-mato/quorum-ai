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

function MobileActionSheet({ isOpen, onClose, workflow, onMoveTo }) {
  if (!isOpen || !workflow) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center md:hidden">
      <div className="absolute inset-0 bg-background/60 backdrop-blur-md animate-fade-in" onClick={onClose} />
      <div className="relative w-full bg-card rounded-t-3xl border-t border-border/20 p-8 animate-fade-up max-w-md mx-auto">
        <div className="w-10 h-1 bg-muted rounded-full mx-auto mb-8 opacity-20" />
        <h3 className="text-lg font-bold text-foreground tracking-tight mb-6">{workflow.title || workflow.id}</h3>
        <div className="space-y-2">
          {KANBAN_COLUMNS.map(col => {
             const isCurrent = workflow.kanban_column === col.id;
             const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
             return (
               <button key={col.id} onClick={() => { if (!isCurrent) { onMoveTo(col.id); onClose(); } }} disabled={isCurrent} className={`w-full flex items-center justify-between p-4 rounded-xl border transition-all ${isCurrent ? 'bg-muted/30 border-transparent opacity-30 grayscale' : 'bg-background/50 border-border/40 hover:border-primary/20 active:scale-[0.98]'}`}>
                 <div className="flex items-center gap-3"><div className={`w-1.5 h-1.5 rounded-full ${accent.dot}`} /><span className="font-bold text-sm text-foreground/80">{col.name}</span></div>
                 {isCurrent && <Badge variant="outline" className="text-[8px] uppercase font-bold opacity-40">Active</Badge>}
               </button>
             );
          })}
        </div>
        <Button variant="ghost" onClick={onClose} className="w-full mt-8 h-12 rounded-xl text-xs font-bold text-muted-foreground/40">Cancel</Button>
      </div>
    </div>
  );
}

function KanbanColumn({ column, workflows, isMobile, onOpenMobileMenu }) {
  const { moveWorkflow, setDraggedWorkflow, clearDraggedWorkflow, engine, draggedWorkflow } = useKanbanStore();
  const navigate = useNavigate(); const dragDepth = useRef(0); const [isOver, setIsOver] = useState(false);
  const canDrop = Boolean(draggedWorkflow && draggedWorkflow.kanban_column !== column.id);
  useEffect(() => { const reset = () => { dragDepth.current = 0; setIsOver(false); }; window.addEventListener('dragend', reset); window.addEventListener('drop', reset); return () => { window.removeEventListener('dragend', reset); window.removeEventListener('drop', reset); }; }, []);
  const handleDragOver = (e) => { e.preventDefault(); e.dataTransfer.dropEffect = canDrop ? 'move' : 'none'; };
  const handleDrop = (e) => { e.preventDefault(); dragDepth.current = 0; setIsOver(false); if (!canDrop) { clearDraggedWorkflow(); return; } const id = e.dataTransfer.getData('text/plain'); if (id) moveWorkflow(id, column.id); clearDraggedWorkflow(); };
  const handleDragEnter = (e) => { e.preventDefault(); if (!canDrop) return; dragDepth.current += 1; setIsOver(true); };
  const handleDragLeave = (e) => { e.preventDefault(); if (!canDrop) return; dragDepth.current = Math.max(0, dragDepth.current - 1); if (dragDepth.current === 0) setIsOver(false); };
  const accent = KANBAN_COLUMN_COLORS[column.id] || KANBAN_COLUMN_COLORS.default;

  return (
    <div onDragOver={handleDragOver} onDrop={handleDrop} onDragEnter={handleDragEnter} onDragLeave={handleDragLeave} className={`flex flex-col ${isMobile ? 'w-[85vw] flex-shrink-0 snap-center h-full mx-2 first:ml-4 last:mr-4' : 'min-w-[300px] max-w-[340px] flex-1'} h-full transition-all duration-500 rounded-3xl border border-border/30 bg-card/5 backdrop-blur-xl overflow-hidden ${isOver && canDrop ? 'ring-1 ring-primary/20 bg-primary/[0.01]' : ''}`}>
      <div className={`flex items-center justify-between px-5 py-4 border-b border-border/20 bg-card/30 border-t-2 ${accent.border}`}>
        <div className="flex items-center gap-2.5">
          <div className={`w-1.5 h-1.5 rounded-full ${accent.dot} opacity-40`} />
          <h3 className="font-bold text-foreground/60 text-[10px] uppercase tracking-widest">{column.name}</h3>
        </div>
        <span className="text-[10px] font-bold text-muted-foreground/40">{workflows.length}</span>
      </div>
      <div className="flex-1 p-3 space-y-3 overflow-y-auto min-h-0 scrollbar-none">
        {workflows.map((w) => <KanbanCard key={w.id} workflow={w} isExecuting={engine.currentWorkflowId === w.id} onDragStart={() => setDraggedWorkflow(w)} onDragEnd={() => clearDraggedWorkflow()} onClick={() => navigate(`/workflows/${w.id}`)} onMoveTo={(t) => moveWorkflow(w.id, t)} onOpenMenu={() => onOpenMobileMenu(w)} />)}
        {workflows.length === 0 && (
          <div className={`flex flex-col items-center justify-center py-16 rounded-2xl border border-dashed transition-all duration-700 ${isOver && canDrop ? 'border-primary/20 bg-primary/[0.01]' : 'border-border/10 bg-transparent opacity-10'}`}>
             <Clock className="w-8 h-8 mb-2" />
             <span className="text-[9px] font-bold uppercase tracking-widest">Idle</span>
          </div>
        )}
      </div>
    </div>
  );
}

function KanbanCard({ workflow, isExecuting, onDragStart, onDragEnd, onClick, onOpenMenu }) {
  const handleDragStart = (e) => { e.dataTransfer.setData('text/plain', workflow.id); e.dataTransfer.effectAllowed = 'move'; onDragStart(); };
  const statusColor = getStatusColor(workflow.status);
  const cardStyle = { borderLeftColor: statusColor.borderStrip ? `var(--${statusColor.borderStrip.split('-')[1]})` : 'transparent', borderLeftWidth: '2px' };

  return (
    <div draggable onDragStart={handleDragStart} onDragEnd={onDragEnd} onClick={onClick} className={`group relative flex flex-col gap-4 rounded-xl border border-border/30 bg-card/60 p-4 shadow-sm transition-all duration-500 hover:shadow-soft hover:-translate-y-0.5 cursor-grab active:cursor-grabbing ${isExecuting ? 'ring-1 ring-primary/20 shadow-primary/5' : 'hover:border-primary/20'}`} style={cardStyle} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onClick?.(); }}>
      {isExecuting && <div className="absolute top-3 right-3 flex items-center gap-1.5 text-[8px] font-bold text-primary/60 bg-primary/5 px-2 py-0.5 rounded-lg border border-primary/10 animate-fade-in"><Zap className="w-2.5 h-2.5 fill-current animate-pulse" /> RUNNING</div>}
      <div className="space-y-1.5">
         <h4 className="font-bold text-foreground/90 text-sm leading-tight line-clamp-2 pr-10 group-hover:text-primary transition-colors">{workflow.title || "Untitled Execution"}</h4>
         <p className="text-muted-foreground/40 text-[10px] line-clamp-2 italic border-l border-border/40 pl-2">&ldquo;{workflow.prompt}&rdquo;</p>
      </div>
      <div className="flex items-center justify-between pt-3 border-t border-border/20">
        <div className="flex items-center gap-3 text-muted-foreground/30">
            {workflow.pr_url && <a href={workflow.pr_url} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()} className="flex items-center gap-1.5 text-[10px] font-bold hover:text-primary transition-all"><GitPullRequest className="w-3 h-3 opacity-60" /><span>#{workflow.pr_number}</span></a>}
            {workflow.task_count > 0 && <div className="flex items-center gap-1.5 text-[10px] font-bold" title={`${workflow.task_count} sub-tasks`}><ListTodo className="w-3 h-3 opacity-60" /><span>{workflow.task_count}</span></div>}
        </div>
        <Badge variant="outline" className={`text-[8px] py-0 font-bold uppercase tracking-widest border-current opacity-40 ${statusColor.text}`}>{workflow.status || 'pending'}</Badge>
      </div>
      <button onClick={(e) => { e.stopPropagation(); onOpenMenu(); }} className="absolute top-3 right-3 md:hidden p-1 text-muted-foreground/20 hover:text-foreground"><MoreVertical className="w-4 h-4" /></button>
    </div>
  );
}

function EngineControls() {
  const { engine, enableEngine, disableEngine, resetCircuitBreaker } = useKanbanStore(); const [isToggling, setIsToggling] = useState(false);
  const handleToggle = async () => { setIsToggling(true); try { if (engine.enabled) await disableEngine(); else await enableEngine(); } finally { setIsToggling(false); } };
  return (
    <div className="flex items-center gap-4 rounded-xl border border-border/20 bg-card/20 backdrop-blur-xl px-4 py-2 shrink-0">
      <div className="flex items-center gap-3">
        <div className={`p-1.5 rounded-lg border transition-all ${engine.enabled ? 'bg-success/5 border-success/20 text-success/60' : 'bg-muted/20 border-border/20 text-muted-foreground/20'}`}><Cpu className={`w-4 h-4 ${engine.enabled ? 'animate-pulse' : ''}`} /></div>
        <div className="hidden lg:block"><p className="text-[9px] font-bold uppercase tracking-widest text-foreground/60">Neural Engine</p></div>
        <button onClick={handleToggle} disabled={isToggling || engine.circuitBreakerOpen} className={`relative inline-flex h-5 w-10 items-center rounded-full transition-all duration-500 ${engine.enabled ? 'bg-success shadow-lg shadow-success/10' : 'bg-muted/30'} ${isToggling || engine.circuitBreakerOpen ? 'opacity-30' : 'cursor-pointer hover:scale-105'}`}><span className={`inline-block h-3.5 w-3.5 transform rounded-full bg-background shadow-sm transition-transform duration-500 ${engine.enabled ? 'translate-x-5.5' : 'translate-x-1'}`} /></button>
      </div>
      {engine.circuitBreakerOpen && <button onClick={() => resetCircuitBreaker()} className="px-2 py-1 text-[8px] font-bold uppercase tracking-widest bg-error/10 text-error rounded-md hover:bg-error/20 transition-all">Reset Fault</button>}
    </div>
  );
}

export default function Kanban() {
  const { columns, fetchBoard, moveWorkflow, loading, error, clearError } = useKanbanStore();
  const [filter, setFilter] = useState(''); const [activeColIndex, setActiveColIndex] = useState(0); const [mobileMenuOpen, setMobileMenuOpen] = useState(false); const [selectedWorkflow, setSelectedWorkflow] = useState(null); const scrollRef = useRef(null);
  useEffect(() => { fetchBoard(); }, [fetchBoard]);
  const handleScroll = (e) => { const c = e.target; const index = Math.floor((c.scrollLeft + c.clientWidth / 2) / c.scrollWidth * KANBAN_COLUMNS.length); if (index !== activeColIndex && index >= 0 && index < KANBAN_COLUMNS.length) setActiveColIndex(index); };
  const handleNavClick = (i) => { setActiveColIndex(i); if (scrollRef.current) { const child = scrollRef.current.children[i]; if (child) child.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' }); } };
  const filteredColumns = useMemo(() => {
    if (!filter) return columns; const f = filter.toLowerCase(); const res = {};
    Object.keys(columns).forEach(k => { res[k] = columns[k].filter(w => (w.title && w.title.toLowerCase().includes(f)) || (w.id && w.id.toLowerCase().includes(f)) || (w.prompt && w.prompt.toLowerCase().includes(f))); });
    return res;
  }, [columns, filter]);

  if (loading && Object.values(columns).every(col => col.length === 0)) return <div className="relative min-h-full"><div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" /><div className="flex items-center justify-center py-60 animate-fade-in"><Loader2 className="w-10 h-10 animate-spin text-primary/20" /></div></div>;

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in flex flex-col">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary"><div className="w-1 h-1 rounded-full bg-current" /><span className="text-[10px] font-bold uppercase tracking-widest opacity-70">Architecture Node</span></div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Operational <span className="text-muted-foreground/40 font-medium">Board</span></h1>
        </div>
        <div className="flex items-center gap-4">
           <div className="relative group"><Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Query board..." value={filter} onChange={(e) => setFilter(e.target.value)} className="h-10 pl-10 pr-4 bg-card/20 border-border/30 rounded-2xl text-xs shadow-sm transition-all" /></div>
           <EngineControls />
        </div>
      </header>

      <div className="md:hidden flex items-center justify-center bg-background/80 backdrop-blur-xl py-4 border-b border-border/10 mb-6 sticky top-14 z-20">
        <div className="flex gap-3 items-center">
          {KANBAN_COLUMNS.map((col, idx) => <button key={col.id} onClick={() => handleNavClick(idx)} className={`h-1 rounded-full transition-all duration-500 ${idx === activeColIndex ? `w-14 bg-primary/60` : 'w-6 bg-muted/40'}`} />)}
        </div>
      </div>

      <div className="flex-1 min-h-[65vh]">
        <div className="hidden md:flex gap-6 overflow-x-auto pb-8 h-full scrollbar-none">{KANBAN_COLUMNS.map((col) => <KanbanColumn key={col.id} column={col} workflows={filteredColumns[col.id] || []} onOpenMobileMenu={(w) => { setSelectedWorkflow(w); setMobileMenuOpen(true); }} />)}</div>
        <div ref={scrollRef} onScroll={handleScroll} className="md:hidden flex overflow-x-auto snap-x snap-mandatory h-full pb-8 scrollbar-none">{KANBAN_COLUMNS.map((col) => <KanbanColumn key={col.id} column={col} workflows={filteredColumns[col.id] || []} isMobile={true} onOpenMobileMenu={(w) => { setSelectedWorkflow(w); setMobileMenuOpen(true); }} />)}</div>
      </div>

      <MobileActionSheet isOpen={mobileMenuOpen} onClose={() => setMobileMenuOpen(false)} workflow={selectedWorkflow} onMoveTo={(id) => moveWorkflow(selectedWorkflow.id, id)} />
    </div>
  );
}

function Loader2({ className }) { return <svg className={className} xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56" /></svg>; }
