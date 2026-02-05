import { useState, useEffect, useRef, useMemo } from 'react';
import {
  FolderPlus,
  Trash2,
  Star,
  RefreshCw,
  AlertCircle,
  Folder,
  FolderOpen,
  Check,
  X,
  Search,
  MoreVertical,
  Plus,
  ChevronRight,
  Info,
  Layers,
  LayoutDashboard,
  ArrowUpRight
} from 'lucide-react';
import { useProjectStore } from '../stores';
import ColorPicker, { PROJECT_COLORS } from '../components/ui/ColorPicker';
import { ConfirmDialog } from '../components/config/ConfirmDialog';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';

function generateNameFromPath(path) {
  if (!path) return '';
  const baseName = path.split('/').pop() || path.split('\\').pop() || '';
  if (!baseName) return '';
  return baseName.charAt(0).toUpperCase() + baseName.slice(1).replace(/[-_]/g, ' ');
}

function StatusBadge({ status }) {
  const config = {
    healthy: { color: 'bg-green-500', textColor: 'text-green-600', label: 'Stable' },
    degraded: { color: 'bg-yellow-500', textColor: 'text-yellow-600', label: 'Alert' },
    offline: { color: 'bg-red-500', textColor: 'text-red-600', label: 'Fault' },
    initializing: { color: 'bg-blue-500', textColor: 'text-blue-600', label: 'Init' },
  }[status] || { color: 'bg-gray-500', textColor: 'text-gray-600', label: status };
  return (
    <div className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full ${config.textColor} opacity-70`}>
      <div className={`w-1 h-1 rounded-full ${config.color}`} />
      <span className="text-[9px] font-bold uppercase tracking-widest">{config.label}</span>
    </div>
  );
}

function InlineEdit({ value, onSave, placeholder, className = '', inputClassName = '' }) {
  const [isEditing, setIsEditing] = useState(false); const [editValue, setEditValue] = useState(value); const inputRef = useRef(null);
  useEffect(() => { if (isEditing && inputRef.current) { inputRef.current.focus(); inputRef.current.select(); } }, [isEditing]);
  useEffect(() => { setEditValue(value); }, [value]);
  const handleSave = () => { if (editValue.trim() && editValue.trim() !== value) onSave(editValue.trim()); setIsEditing(false); };
  if (isEditing) return <input ref={inputRef} type="text" value={editValue} onChange={(e) => setEditValue(e.target.value)} onBlur={handleSave} onKeyDown={(e) => { if (e.key === 'Enter') handleSave(); else if (e.key === 'Escape') { setEditValue(value); setIsEditing(false); } }} className={`px-2 py-1 rounded-lg border border-primary/20 bg-background text-xs focus:outline-none w-full ${inputClassName}`} onClick={(e) => e.stopPropagation()} />;
  return <div onClick={(e) => { e.stopPropagation(); setIsEditing(true); }} className={`text-left hover:bg-primary/[0.02] rounded-lg px-2 py-1 -mx-2 transition-colors truncate cursor-text ${className}`}>{value || <span className="text-muted-foreground/30 italic">{placeholder}</span>}</div>;
}

function ProjectCard({ project, isDefault, isSelected, onUpdate, onDelete, onSetDefault, onValidate, onSelect }) {
  const [showMenu, setShowMenu] = useState(false); const menuRef = useRef(null);
  useEffect(() => { const handleClickOutside = (e) => { if (menuRef.current && !menuRef.current.contains(e.target)) setShowMenu(false); }; document.addEventListener('mousedown', handleClickOutside); return () => document.removeEventListener('mousedown', handleClickOutside); }, []);
  const cardStyle = { borderLeftColor: project.color || '#71717a', borderLeftWidth: '2px' };

  return (
    <div className={`group flex flex-col h-full rounded-2xl border border-border/30 bg-card/10 backdrop-blur-sm transition-all duration-500 hover:shadow-soft hover:-translate-y-0.5 overflow-hidden ${isSelected ? 'ring-1 ring-primary/10 border-primary/20' : ''}`} style={cardStyle}>
      <div className="flex-1 p-6 space-y-5">
        <div className="flex items-start justify-between">
          <div className="p-2 rounded-xl bg-background border border-border/60 shadow-sm transition-all group-hover:border-primary/20">
             <ColorPicker value={project.color || PROJECT_COLORS[0]} onChange={(c) => onUpdate(project.id, { color: c })} />
          </div>
          <div className="relative" ref={menuRef}>
            <button onClick={() => setShowMenu(!showMenu)} className="p-1.5 rounded-lg text-muted-foreground/20 hover:text-foreground transition-all"><MoreVertical className="w-4 h-4" /></button>
            {showMenu && <div className="absolute right-0 top-full mt-2 w-48 rounded-xl border border-border/20 bg-popover/90 backdrop-blur-xl shadow-2xl z-20 py-1 text-[11px] animate-in fade-in zoom-in-95">
                <button onClick={() => { onValidate(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2 hover:bg-primary/5 flex items-center gap-2 font-bold uppercase tracking-widest text-foreground/60"><RefreshCw className="w-3.5 h-3.5 opacity-40" /> Verify</button>
                {!isDefault && <button onClick={() => { onSetDefault(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2 hover:bg-primary/5 flex items-center gap-2 font-bold uppercase tracking-widest text-foreground/60"><Star className="w-3.5 h-3.5 opacity-40" /> Default</button>}
                <button onClick={() => { onDelete(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2 hover:bg-destructive/5 text-destructive/60 flex items-center gap-2 font-bold uppercase tracking-widest border-t border-border/10 mt-1">Delete</button>
              </div>}
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <InlineEdit value={project.name} onSave={(n) => onUpdate(project.id, { name: n })} placeholder="Node identity" className="font-bold text-base text-foreground/90 transition-colors" />
            {isDefault && <Star className="w-3 h-3 fill-yellow-500/20 text-yellow-500/40" />}
          </div>
          <div className="flex items-center gap-2 text-muted-foreground/30 text-[9px] font-mono font-bold uppercase tracking-widest">
            <InlineEdit value={project.path} onSave={(p) => onUpdate(project.id, { path: p })} placeholder="/mnt/root" className="truncate" inputClassName="font-mono text-[9px] py-0 h-6" />
          </div>
        </div>
        <div className="flex items-center justify-between pt-1">
           <StatusBadge status={project.status} />
           {isSelected && <span className="text-[8px] font-bold uppercase tracking-wider text-primary/40">Active Node</span>}
        </div>
      </div>
      <div className="px-6 py-3 mt-auto flex gap-2 border-t border-border/20 bg-background/20">
        <Button variant="ghost" size="sm" onClick={() => onSelect(project.id)} className="flex-1 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 hover:text-primary">Connect</Button>
        <Button size="sm" onClick={() => onValidate(project.id)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest h-8 shadow-md shadow-primary/5">Audit</Button>
      </div>
    </div>
  );
}

export default function Projects() {
  const { projects, currentProjectId, defaultProjectId, error, fetchProjects, createProject, updateProject, deleteProject, setDefaultProject, validateProject, selectProject, clearError } = useProjectStore();
  const [deleteId, setDeleteId] = useState(null); const [searchQuery, setSearchQuery] = useState('');
  useEffect(() => { fetchProjects(); }, [fetchProjects]);
  const filtered = useMemo(() => projects.filter(p => !searchQuery || p.name?.toLowerCase().includes(searchQuery.toLowerCase()) || p.path?.toLowerCase().includes(searchQuery.toLowerCase())), [projects, searchQuery]);

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <header className="flex flex-col md:flex-row md:items-end justify-between gap-6 pt-4 pb-2 border-b border-border/20">
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-primary"><div className="w-1 h-1 rounded-full bg-current" /><span className="text-[10px] font-bold uppercase tracking-widest opacity-70">Architecture Hub</span></div>
          <h1 className="text-3xl font-bold tracking-tight text-foreground">Workspace <span className="text-muted-foreground/40 font-medium">Nodes</span></h1>
        </div>
        <div className="relative group"><Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Filter clusters..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="h-10 pl-10 pr-4 bg-card/20 border-border/30 rounded-2xl text-xs shadow-sm transition-all" /></div>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {!searchQuery && <button onClick={() => createProject('/home/user/project')} className="flex flex-col items-center justify-center min-h-[200px] rounded-3xl border-2 border-dashed border-border/10 hover:border-primary/10 hover:bg-primary/[0.01] transition-all duration-500 group opacity-30 hover:opacity-100"><Plus className="w-8 h-8 mb-3 text-muted-foreground/20 group-hover:text-primary/40 transition-all" /><span className="text-[10px] font-bold uppercase tracking-widest">Spawn Node</span></button>}
        {filtered.map((p) => <ProjectCard key={p.id} project={p} isDefault={p.id === defaultProjectId} isSelected={p.id === currentProjectId} onUpdate={updateProject} onDelete={setDeleteId} onSetDefault={setDefaultProject} onValidate={validateProject} onSelect={selectProject} />)}
      </div>

      <ConfirmDialog isOpen={deleteId !== null} onClose={() => setDeleteId(null)} onConfirm={() => { deleteProject(deleteId); setDeleteId(null); }} title="Purge Node?" message="Confirm irreversible decoupling of workspace telemetry." confirmText="Purge" variant="danger" />
    </div>
  );
}