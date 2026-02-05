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

// Bottom Sheet Component - Refined
function MobileActionSheet({ isOpen, onClose, workflow, onMoveTo }) {
  if (!isOpen || !workflow) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center md:hidden">
      <div className="absolute inset-0 bg-background/80 backdrop-blur-md animate-fade-in" onClick={onClose} />
      <div className="relative w-full bg-card rounded-t-[2.5rem] shadow-2xl border-t border-border/40 p-10 animate-fade-up max-w-md mx-auto">
        <div className="w-12 h-1 bg-muted rounded-full mx-auto mb-10 opacity-40" />
        <div className="mb-10 space-y-2">
          <p className="text-[10px] font-bold uppercase tracking-[0.2em] text-primary/60">Node Transition</p>
          <h3 className="text-2xl font-bold text-foreground tracking-tight leading-tight">{workflow.title || workflow.id}</h3>
        </div>
        <div className="space-y-4">
          <p className="text-[10px] font-bold text-muted-foreground/40 uppercase tracking-widest px-1">Select Target Phase</p>
          <div className="grid grid-cols-1 gap-2.5">
            {KANBAN_COLUMNS.map(col => {
               const isCurrent = workflow.kanban_column === col.id;
               const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
               return (
                 <button key={col.id} onClick={() => { if (!isCurrent) { onMoveTo(col.id); onClose(); } }} disabled={isCurrent} className={`w-full flex items-center justify-between p-5 rounded-2xl border transition-all duration-300 ${isCurrent ? 'bg-muted/30 border-transparent opacity-30 grayscale cursor-default' : 'bg-background/50 border-border/40 hover:border-primary/20 hover:bg-primary/[0.01] active:scale-[0.98]'}`}>
                   <div className="flex items-center gap-4">
                      <div className={`w-2 h-2 rounded-full ${accent.dot} shadow-sm`} />
                      <span className="font-bold text-foreground/80 tracking-tight">{col.name}</span>
                   </div>
                   {isCurrent && <Badge variant="outline" className="text-[9px] uppercase font-bold tracking-tighter opacity-40">Current</Badge>}
                 </button>
               );
            })}
          </div>
        </div>
        <Button variant="ghost" onClick={onClose} className="w-full mt-10 h-14 rounded-2xl font-bold text-muted-foreground/40 hover:text-foreground">Dismiss Protocol</Button>
      </div>
    </div>
  );
}

// Column component - Refined
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
  const bgClass = isMobile ? accent.tint : 'bg-card/20';

  return (
    <div onDragOver={handleDragOver} onDrop={handleDrop} onDragEnter={handleDragEnter} onDragLeave={handleDragLeave} className={`flex flex-col ${isMobile ? 'w-[85vw] flex-shrink-0 snap-center h-full mx-2 first:ml-4 last:mr-4' : 'min-w-[320px] max-w-[360px] flex-1'} h-full transition-all duration-500 rounded-3xl border border-border/40 ${bgClass} backdrop-blur-xl overflow-hidden ${isOver && canDrop ? 'ring-2 ring-primary/20 bg-primary/[0.02] scale-[1.01]' : ''}`}>
      <div className={`flex items-center justify-between px-6 py-5 border-b border-border/30 bg-card/30 border-t-2 ${accent.border}`}>
        <div className="flex items-center gap-3">
          <div className={`w-1.5 h-1.5 rounded-full ${accent.dot} opacity-60 shadow-sm`} />
          <h3 className="font-bold text-foreground/60 text-[10px] uppercase tracking-[0.25em]">{column.name}</h3>
        </div>
        <Badge variant="secondary" className="px-2 py-0 bg-background/40 border-border/20 text-[10px] font-bold">{workflows.length}</Badge>
      </div>
      <div className="flex-1 p-4 space-y-4 overflow-y-auto min-h-0 scrollbar-none">
        {workflows.map((w) => <KanbanCard key={w.id} workflow={w} isExecuting={engine.currentWorkflowId === w.id} onDragStart={() => setDraggedWorkflow(w)} onDragEnd={() => clearDraggedWorkflow()} onClick={() => navigate(`/workflows/${w.id}`)} onMoveTo={(t) => moveWorkflow(w.id, t)} onOpenMenu={() => onOpenMobileMenu(w)} />)}
        {workflows.length === 0 && (
          <div className={`flex flex-col items-center justify-center py-20 rounded-2xl border-2 border-dashed transition-all duration-700 ${isOver && canDrop ? 'border-primary/20 bg-primary/[0.01]' : 'border-border/10 bg-transparent text-muted-foreground/10'}`}>
             {isOver && canDrop ? <Inbox className="w-12 h-12 text-primary/20 mb-4 animate-bounce" />
             : <div className="flex flex-col items-center opacity-30"><Clock className="w-10 h-10 mb-3" /><span className="text-[10px] font-bold uppercase tracking-widest">Awaiting Pulse</span></div>}
          </div>
        )}
      </div>
    </div>
  );
}

// Card component - Refined
function KanbanCard({ workflow, isExecuting, onDragStart, onDragEnd, onClick, onOpenMenu }) {
  const handleDragStart = (e) => { e.dataTransfer.setData('text/plain', workflow.id); e.dataTransfer.effectAllowed = 'move'; onDragStart(); };
  const statusColor = getStatusColor(workflow.status);
  const cardStyle = { borderLeftColor: statusColor.borderStrip ? `var(--${statusColor.borderStrip.split('-')[1]})` : 'transparent', borderLeftWidth: '2px' };

  return (
    <div draggable onDragStart={handleDragStart} onDragEnd={onDragEnd} onClick={onClick} className={`group relative flex flex-col gap-5 rounded-2xl border border-border/40 bg-card/60 p-5 shadow-sm transition-all duration-500 hover:shadow-[0_15px_40px_rgba(0,0,0,0.04)] hover:-translate-y-1 cursor-grab active:cursor-grabbing ${isExecuting ? 'ring-1 ring-primary/20 shadow-primary/5' : 'hover:border-primary/20'}`} style={cardStyle} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onClick?.(); }}>
      {isExecuting && <div className="absolute top-4 right-4 flex items-center gap-2 text-[9px] font-bold text-primary/80 bg-primary/5 px-2.5 py-1 rounded-lg border border-primary/10 animate-fade-in"><Zap className="w-3 h-3 fill-current animate-pulse" /> PROCESSING</div>}
      <div className="space-y-2.5">
         <h4 className="font-bold text-foreground text-[15px] leading-snug line-clamp-2 pr-12 group-hover:text-primary transition-colors duration-300">{workflow.title || "Untitled Execution"}</h4>
         <p className="text-muted-foreground/40 text-[11px] line-clamp-2 leading-relaxed italic border-l border-border/40 pl-3 font-medium">&ldquo;{workflow.prompt}&rdquo;</p>
      </div>
      <div className="flex items-center justify-between pt-4 mt-1 border-t border-border/30">
        <div className="flex items-center gap-4 text-muted-foreground/40">
            {workflow.pr_url && <a href={workflow.pr_url} target="_blank" rel="noopener noreferrer" onClick={(e) => e.stopPropagation()} className="flex items-center gap-2 text-[10px] font-bold hover:text-primary transition-colors duration-300"><GitPullRequest className="w-3.5 h-3.5 opacity-60" /><span>#{workflow.pr_number}</span></a>}
            {workflow.task_count > 0 && <div className="flex items-center gap-2 text-[10px] font-bold" title={`${workflow.task_count} sub-tasks`}><ListTodo className="w-3.5 h-3.5 opacity-60" /><span>{workflow.task_count}</span></div>}
        </div>
        <Badge variant="secondary" className={`text-[9px] py-0 font-bold uppercase tracking-widest border-transparent opacity-70 ${statusColor.bg} ${statusColor.text}`}>{workflow.status || 'pending'}</Badge>
      </div>
      <button onClick={(e) => { e.stopPropagation(); onOpenMenu(); }} className="absolute top-4 right-4 md:hidden p-2 text-muted-foreground/20 hover:text-foreground hover:bg-accent rounded-xl transition-all"><MoreVertical className="w-4 h-4" /></button>
    </div>
  );
}

function EngineControls() {
  const { engine, enableEngine, disableEngine, resetCircuitBreaker } = useKanbanStore(); const [isToggling, setIsToggling] = useState(false);
  const handleToggle = async () => { setIsToggling(true); try { if (engine.enabled) await disableEngine(); else await enableEngine(); } finally { setIsToggling(false); } };
  return (
    <div className="flex items-center justify-between gap-6 rounded-2xl border border-border/40 bg-card/30 backdrop-blur-xl px-6 py-3 shrink-0 shadow-sm">
      <div className="flex items-center gap-4">
        <div className={`p-2.5 rounded-xl border transition-all duration-700 ${engine.enabled ? 'bg-success/5 border-success/20 text-success/70' : 'bg-muted/30 border-border/40 text-muted-foreground/30'}`}><Cpu className={`w-4.5 h-4.5 ${engine.enabled ? 'animate-pulse' : ''}`} /></div>
        <div className="hidden sm:block"><p className="text-[10px] font-bold uppercase tracking-[0.2em] text-foreground/80">Neural Engine</p><p className="text-[9px] font-semibold text-muted-foreground/40 leading-none mt-1">{engine.enabled ? 'MONITORING PULSE' : 'AWAITING WAKE'}</p></div>
        <button onClick={handleToggle} disabled={isToggling || engine.circuitBreakerOpen} className={`relative inline-flex h-6 w-12 items-center rounded-full transition-all duration-500 ${engine.enabled ? 'bg-success shadow-[0_0_15px_rgba(34,197,94,0.2)]' : 'bg-muted/40'} ${isToggling || engine.circuitBreakerOpen ? 'opacity-30 cursor-not-allowed' : 'cursor-pointer hover:scale-105'}`}><span className={`inline-block h-4.5 w-4.5 transform rounded-full bg-background shadow-sm transition-transform duration-500 ease-in-out ${engine.enabled ? 'translate-x-6.5' : 'translate-x-1'}`} /></button>
      </div>
      <div className="hidden xl:flex items-center gap-4 border-l border-border/30 pl-6 ml-2">
        {engine.currentWorkflowId && <div className="flex items-center gap-3 px-4 py-1.5 rounded-xl bg-primary/5 text-primary/60 border border-primary/10 animate-fade-in shadow-inner"><span className="w-1.5 h-1.5 bg-primary rounded-full animate-pulse" /><span className="text-[9px] font-bold uppercase tracking-[0.3em]">Processing: {engine.currentWorkflowId.substring(0, 8)}</span></div>}
        {engine.circuitBreakerOpen && <div className="flex items-center gap-4 px-4 py-1.5 rounded-xl bg-error/5 text-error/60 border border-error/10 animate-shake shadow-inner"><AlertCircle className="w-4 h-4" /><span className="text-[9px] font-bold uppercase tracking-widest">EMERGENCY HALT: {engine.consecutiveFailures} FAULTS</span><Button variant="ghost" size="sm" onClick={() => resetCircuitBreaker()} className="h-7 px-3 text-[9px] font-bold uppercase tracking-widest hover:bg-error/10 text-error">System Reset</Button></div>}
      </div>
    </div>
  );
}

function KanbanFilters({ filter, setFilter }) {
  return (
    <div className="relative w-full sm:w-80 group">
      <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" />
      <Input placeholder="Filter neural board..." value={filter} onChange={(e) => setFilter(e.target.value)} className="h-12 pl-12 pr-6 rounded-2xl border-border/40 bg-card/30 backdrop-blur-md shadow-sm transition-all" />
    </div>
  );
}

function MobileColumnNav({ columns, activeIndex, onChange }) {
  return (
    <div className="md:hidden flex items-center justify-center bg-background/80 backdrop-blur-xl py-4 border-b border-border/30 mb-6 sticky top-14 z-20">
      <div className="flex gap-3 items-center">
        {columns.map((col, idx) => {
          const accent = KANBAN_COLUMN_COLORS[col.id] || KANBAN_COLUMN_COLORS.default;
          return <button key={col.id} onClick={() => onChange(idx)} className={`h-1 rounded-full transition-all duration-700 ${idx === activeIndex ? `w-14 ${accent.dot} shadow-[0_0_10px_rgba(var(--color-primary),0.3)] opacity-80` : 'w-6 bg-muted/40 hover:bg-muted-foreground/20'}`} title={col.name} />;
        })}
      </div>
    </div>
  );
}

export default function Kanban() {
  const { columns, fetchBoard, moveWorkflow, loading, error, clearError } = useKanbanStore();
  const [filter, setFilter] = useState(''); const [activeColIndex, setActiveColIndex] = useState(0); const [mobileMenuOpen, setMobileMenuOpen] = useState(false); const [selectedWorkflow, setSelectedWorkflow] = useState(null); const scrollRef = useRef(null);
  useEffect(() => { fetchBoard(); }, [fetchBoard]);
  const handleOpenMobileMenu = (w) => { setSelectedWorkflow(w); setMobileMenuOpen(true); };
  const handleMove = (id) => { if (selectedWorkflow) moveWorkflow(selectedWorkflow.id, id); };
  const handleScroll = (e) => { const c = e.target; const index = Math.floor((c.scrollLeft + c.clientWidth / 2) / c.scrollWidth * KANBAN_COLUMNS.length); if (index !== activeColIndex && index >= 0 && index < KANBAN_COLUMNS.length) setActiveColIndex(index); };
  const handleNavClick = (i) => { setActiveColIndex(i); if (scrollRef.current) { const child = scrollRef.current.children[i]; if (child) child.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'center' }); } };
  const filteredColumns = useMemo(() => {
    if (!filter) return columns; const f = filter.toLowerCase(); const res = {};
    Object.keys(columns).forEach(k => { res[k] = columns[k].filter(w => (w.title && w.title.toLowerCase().includes(f)) || (w.id && w.id.toLowerCase().includes(f)) || (w.prompt && w.prompt.toLowerCase().includes(f))); });
    return res;
  }, [columns, filter]);

  if (loading && Object.values(columns).every(col => col.length === 0)) return <div className="relative min-h-full"><div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" /><div className="flex flex-col items-center justify-center py-60 gap-6 animate-fade-in"><Loader2 className="w-12 h-12 animate-spin text-primary/40" /><p className="text-[10px] font-bold uppercase tracking-[0.3em] text-muted-foreground/40">Visualizing Neural Board</p></div></div>;

  return (
    <div className="relative min-h-full space-y-10 pb-12 animate-fade-in flex flex-col">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        <div className="relative z-10 flex flex-col lg:flex-row lg:items-center justify-between gap-10">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]"><FolderKanban className="h-3 w-3 opacity-70" /> Structural Logic</div>
            <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">Process <span className="text-primary/80">Architecture Board</span></h1>
            <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">Monitor the operational flow of your automated cycles. Refine, audit, and transition blueprints across the execution pipeline.</p>
          </div>
          <div className="hidden lg:flex shrink-0"><div className="p-8 rounded-[2.5rem] bg-background/40 border border-border/60 shadow-inner backdrop-blur-md group hover:border-primary/20 transition-all duration-500"><LayoutDashboard className="w-16 h-16 text-primary/20 group-hover:text-primary/40 transition-colors duration-500" /></div></div>
        </div>
      </div>

      <div className="sticky top-14 z-30 flex flex-col gap-6 bg-background/80 backdrop-blur-xl py-6 border-b border-border/30">
        <div className="flex flex-col md:flex-row gap-6 md:items-center justify-between">
          <div className="flex items-center gap-6 flex-1"><KanbanFilters filter={filter} setFilter={setFilter} /><div className="hidden lg:flex items-center gap-3 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 bg-muted/10 px-4 py-2 rounded-xl border border-border/30 backdrop-blur-sm"><Info className="h-3.5 w-3.5" /><span>{Object.values(columns).flat().length} BLUEPRINTS AUDITED</span></div></div>
          <EngineControls />
        </div>
      </div>

      {error && <div className="flex items-center justify-between gap-6 p-6 bg-error/[0.02] border border-error/20 rounded-3xl animate-shake shadow-sm"><div className="flex items-center gap-4"><AlertCircle className="w-6 h-6 text-error/60" /><p className="text-error/80 text-sm font-bold tracking-tight">{error}</p></div><div className="flex items-center gap-3"><Button variant="outline" size="sm" onClick={fetchBoard} className="rounded-xl font-bold bg-background h-10 px-6 border-border/60">Reconnect System</Button><Button variant="ghost" size="icon" onClick={clearError} className="h-10 w-10 text-error/40 hover:bg-error/10 rounded-xl"><X className="w-5 h-5" /></Button></div></div>}

      <MobileColumnNav columns={KANBAN_COLUMNS} activeIndex={activeColIndex} onChange={handleNavClick} />

      <div className="flex-1 min-h-[65vh]">
        <div className="hidden md:flex gap-8 overflow-x-auto pb-12 h-full scrollbar-thin scrollbar-thumb-muted/20">{KANBAN_COLUMNS.map((col) => <KanbanColumn key={col.id} column={col} workflows={filteredColumns[col.id] || []} onOpenMobileMenu={handleOpenMobileMenu} />)}</div>
        <div ref={scrollRef} onScroll={handleScroll} className="md:hidden flex overflow-x-auto snap-x snap-mandatory h-full pb-12 scrollbar-none">{KANBAN_COLUMNS.map((col) => <KanbanColumn key={col.id} column={col} workflows={filteredColumns[col.id] || []} isMobile={true} onOpenMobileMenu={handleOpenMobileMenu} />)}</div>
      </div>

      <MobileActionSheet isOpen={mobileMenuOpen} onClose={() => setMobileMenuOpen(false)} workflow={selectedWorkflow} onMoveTo={handleMove} />
    </div>
  );
}

function Loader2({ className }) { return <svg className={className} xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 12a9 9 0 1 1-6.219-8.56" /></svg>; }