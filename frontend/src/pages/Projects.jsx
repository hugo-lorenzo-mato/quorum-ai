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
  let name = baseName.charAt(0).toUpperCase() + baseName.slice(1);
  return name.replace(/[-_]/g, ' ');
}

function StatusBadge({ status }) {
  const config = {
    healthy: { color: 'bg-green-500', textColor: 'text-green-600', label: 'Healthy' },
    degraded: { color: 'bg-yellow-500', textColor: 'text-yellow-600', label: 'Degraded' },
    offline: { color: 'bg-red-500', textColor: 'text-red-600', label: 'Offline' },
    initializing: { color: 'bg-blue-500', textColor: 'text-blue-600', label: 'Initializing' },
  }[status] || { color: 'bg-gray-500', textColor: 'text-gray-600', label: status };
  return (
    <div className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-opacity-5 ${config.textColor} border border-current opacity-70`} style={{ backgroundColor: `${config.color}10` }}>
      <div className={`w-1.5 h-1.5 rounded-full ${config.color}`} />
      <span className="text-[9px] font-bold uppercase tracking-widest">{config.label}</span>
    </div>
  );
}

function InlineEdit({ value, onSave, placeholder, className = '', inputClassName = '' }) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(value);
  const inputRef = useRef(null);
  useEffect(() => { if (isEditing && inputRef.current) { inputRef.current.focus(); inputRef.current.select(); } }, [isEditing]);
  useEffect(() => { setEditValue(value); }, [value]);
  const handleSave = () => { const trimmed = editValue.trim(); if (trimmed && trimmed !== value) onSave(trimmed); setIsEditing(false); };
  const handleKeyDown = (e) => { if (e.key === 'Enter') { e.preventDefault(); handleSave(); } else if (e.key === 'Escape') { setEditValue(value); setIsEditing(false); } };
  if (isEditing) return <input ref={inputRef} type="text" value={editValue} onChange={(e) => setEditValue(e.target.value)} onBlur={handleSave} onKeyDown={handleKeyDown} placeholder={placeholder} className={`px-2 py-1 rounded-lg border border-primary/30 bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-primary/10 w-full min-w-0 ${inputClassName}`} onClick={(e) => e.stopPropagation()} />;
  return <div onClick={(e) => { e.stopPropagation(); setIsEditing(true); }} className={`text-left hover:bg-primary/5 rounded-lg px-2 py-1 -mx-2 transition-colors group max-w-full truncate min-w-0 cursor-text ${className}`} title="Click to edit">{value || <span className="text-muted-foreground/40 italic">{placeholder}</span>}</div>;
}

function ProjectCard({ project, isDefault, isSelected, onUpdate, onDelete, onSetDefault, onValidate, onSelect }) {
  const [showMenu, setShowMenu] = useState(false); const menuRef = useRef(null);
  useEffect(() => { const handleClickOutside = (e) => { if (menuRef.current && !menuRef.current.contains(e.target)) setShowMenu(false); }; document.addEventListener('mousedown', handleClickOutside); return () => document.removeEventListener('mousedown', handleClickOutside); }, []);
  const handleNameSave = (name) => { if (name !== project.name) onUpdate(project.id, { name }); };
  const handlePathSave = (path) => { if (path !== project.path) onUpdate(project.id, { path }); };
  const handleColorChange = (color) => { if (color !== project.color) onUpdate(project.id, { color }); };
  const borderStyle = { borderLeftColor: project.color || '#71717a', borderLeftWidth: '2px' };

  return (
    <div className={`group flex flex-col h-full rounded-[1.5rem] border border-border/40 bg-card/40 backdrop-blur-md transition-all duration-500 hover:shadow-[0_20px_50px_rgba(0,0,0,0.04)] hover:-translate-y-1 overflow-hidden ${isSelected ? 'ring-1 ring-primary/20 shadow-md border-primary/20' : ''}`} style={borderStyle}>
      <div className="flex-1 p-6 space-y-5">
        <div className="flex items-start justify-between gap-3">
          <div className="p-2.5 rounded-xl bg-background border border-border/60 shadow-sm group-hover:border-primary/30 transition-all duration-500">
             <ColorPicker value={project.color || PROJECT_COLORS[0]} onChange={handleColorChange} />
          </div>
          <div className="relative" ref={menuRef}>
            <button onClick={() => setShowMenu(!showMenu)} className="p-2 rounded-xl text-muted-foreground/30 hover:bg-accent hover:text-foreground transition-all duration-300"><MoreVertical className="w-4 h-4" /></button>
            {showMenu && <div className="absolute right-0 top-full mt-2 w-56 rounded-2xl border border-border/40 bg-popover/90 backdrop-blur-xl shadow-2xl z-20 py-2 text-sm animate-in fade-in zoom-in-95 duration-200">
                 {!isSelected && <button onClick={() => { onSelect(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2.5 hover:bg-primary/5 flex items-center gap-3 font-semibold text-foreground/80"><FolderOpen className="w-4 h-4 opacity-40" /> Open Workspace</button>}
                <button onClick={() => { onValidate(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2.5 hover:bg-primary/5 flex items-center gap-3 font-semibold text-foreground/80"><RefreshCw className="w-4 h-4 opacity-40" /> Verify Health</button>
                {!isDefault && <button onClick={() => { onSetDefault(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2.5 hover:bg-primary/5 flex items-center gap-3 font-semibold text-foreground/80"><Star className="w-4 h-4 opacity-40" /> Set Primary</button>}
                <div className="h-px bg-border/30 my-2" />
                <button onClick={() => { onDelete(project.id); setShowMenu(false); }} className="w-full text-left px-4 py-2.5 hover:bg-destructive/5 text-destructive/80 flex items-center gap-3 font-bold"><Trash2 className="w-4 h-4 opacity-40" /> Purge Entry</button>
              </div>}
          </div>
        </div>
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <InlineEdit value={project.name} onSave={handleNameSave} placeholder="Project name" className="font-bold text-lg text-foreground truncate group-hover:text-primary transition-colors duration-300" />
            {isDefault && <Star className="w-3.5 h-3.5 fill-yellow-500/40 text-yellow-500/60" />}
          </div>
          <div className="flex items-center gap-2 text-muted-foreground/40 text-[10px] font-mono font-bold uppercase tracking-tight">
            <Folder className="w-3 h-3 opacity-40" />
            <InlineEdit value={project.path} onSave={handlePathSave} placeholder="/path/to/project" className="truncate opacity-70 group-hover:opacity-100" inputClassName="font-mono text-[10px] py-0 h-6" />
          </div>
        </div>
        <div className="flex items-center justify-between pt-2">
           <StatusBadge status={project.status} />
           {isSelected && <Badge variant="secondary" className="text-[9px] px-2 py-0 font-black uppercase tracking-widest bg-primary/5 text-primary/60 border-primary/10">Active</Badge>}
        </div>
        {project.status_message && <div className="mt-4 p-3 rounded-xl bg-yellow-500/[0.03] border border-yellow-500/10 text-yellow-700/70 dark:text-yellow-400/60 text-[10px] leading-relaxed flex items-start gap-2.5 font-medium"><AlertCircle className="w-3.5 h-3.5 mt-0.5 shrink-0 opacity-60" /><span className="line-clamp-2">{project.status_message}</span></div>}
      </div>
      <div className="px-6 py-4 mt-auto flex gap-3 border-t border-border/30 bg-primary/[0.01]">
        <Button variant="outline" size="sm" onClick={() => onSelect(project.id)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest border-border/60 h-9">Open</Button>
        <Button size="sm" onClick={() => onValidate(project.id)} className="flex-1 rounded-xl text-[10px] font-bold uppercase tracking-widest h-9 shadow-lg shadow-primary/10">Validate</Button>
      </div>
    </div>
  );
}

function AddProjectCard({ onAdd }) {
  const [isExpanded, setIsExpanded] = useState(false); const [newPath, setNewPath] = useState(''); const [newName, setNewName] = useState(''); const [creating, setCreating] = useState(false); const [error, setError] = useState(null);
  const autoName = generateNameFromPath(newPath);
  const handleSubmit = async (e) => { e.preventDefault(); if (!newPath.trim()) return; setCreating(true); setError(null); try { await onAdd(newPath.trim(), { name: newName.trim() || undefined }); setNewPath(''); setNewName(''); setIsExpanded(false); } catch (err) { setError(err.message || 'Purge failed'); } finally { setCreating(false); } };
  if (!isExpanded) return <button onClick={() => setIsExpanded(true)} className="flex flex-col items-center justify-center h-full min-h-[240px] p-8 rounded-[2.5rem] border-2 border-dashed border-border/20 hover:border-primary/20 hover:bg-primary/[0.01] transition-all duration-500 group text-muted-foreground/30 hover:text-primary/60 bg-card/5 shadow-inner"><div className="w-14 h-14 rounded-3xl bg-muted/10 group-hover:bg-primary/5 flex items-center justify-center mb-4 transition-all duration-500"><Plus className="w-7 h-7" /></div><span className="text-sm font-bold uppercase tracking-widest">Deploy Workspace</span></button>;
  return (
    <div className="h-full min-h-[240px] p-8 rounded-[2.5rem] border border-primary/20 bg-primary/[0.02] backdrop-blur-xl flex flex-col animate-fade-in shadow-2xl">
      <div className="flex items-center justify-between mb-8"><h3 className="font-bold text-foreground tracking-tight flex items-center gap-3"><FolderPlus className="w-5 h-5 text-primary/60" /> New Architecture</h3><button onClick={() => { setIsExpanded(false); setError(null); }} className="p-1.5 rounded-lg hover:bg-primary/5 text-muted-foreground/30 hover:text-foreground transition-all"><X className="w-4 h-4" /></button></div>
      <form onSubmit={handleSubmit} className="flex-1 flex flex-col gap-6">
        <div className="space-y-2"><label className="text-[9px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40 ml-1">Physical Path</label><input type="text" value={newPath} onChange={(e) => setNewPath(e.target.value)} placeholder="/home/dev/project" autoFocus className="w-full h-11 px-4 rounded-xl border border-border/40 bg-background/50 text-sm font-mono focus:ring-2 focus:ring-primary/10 outline-none transition-all shadow-inner" /></div>
        <div className="relative space-y-2"><label className="text-[9px] font-bold uppercase tracking-[0.2em] text-muted-foreground/40 ml-1">Logical Name</label><input type="text" value={newName} onChange={(e) => setNewName(e.target.value)} placeholder={autoName || "Neural Node"} className="w-full h-11 px-4 rounded-xl border border-border/40 bg-background/50 text-sm font-bold focus:ring-2 focus:ring-primary/10 outline-none transition-all shadow-inner" />{!newName && autoName && <span className="absolute right-4 top-10 text-[9px] text-muted-foreground/20 font-bold uppercase pointer-events-none">Auto: {autoName}</span>}</div>
        {error && <p className="text-[10px] text-destructive/70 flex items-center gap-2 font-bold bg-destructive/5 p-3 rounded-xl border border-destructive/10 animate-shake"><AlertCircle className="w-3.5 h-3.5" /> {error}</p>}
        <div className="mt-auto pt-6 flex justify-end gap-3"><Button type="button" variant="ghost" size="sm" onClick={() => setIsExpanded(false)} className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40">Abort</Button><Button type="submit" size="sm" disabled={creating || !newPath.trim()} className="px-6 h-10 rounded-xl text-[10px] font-bold uppercase tracking-widest shadow-xl shadow-primary/10">{creating ? <RefreshCw className="w-3.5 h-3.5 animate-spin mr-2" /> : <Plus className="w-3.5 h-3.5 mr-2" />}Deploy</Button></div>
      </form>
    </div>
  );
}

export default function Projects() {
  const { projects, currentProjectId, defaultProjectId, error, fetchProjects, createProject, updateProject, deleteProject, setDefaultProject, validateProject, selectProject, clearError } = useProjectStore();
  const [deleteConfirmId, setDeleteConfirmId] = useState(null); const [searchQuery, setSearchQuery] = useState('');
  useEffect(() => { fetchProjects(); }, [fetchProjects]);
  const handleUpdate = async (id, data) => { try { await updateProject(id, data); } catch {} };
  const handleDelete = async (id) => { try { await deleteProject(id); setDeleteConfirmId(null); } catch {} };
  const projectToDelete = projects.find((p) => p.id === deleteConfirmId);
  const filteredProjects = useMemo(() => projects.filter(p => { const q = searchQuery.toLowerCase(); return p.name?.toLowerCase().includes(q) || p.path?.toLowerCase().includes(q); }), [projects, searchQuery]);

  return (
    <div className="relative min-h-full space-y-10 pb-12 animate-fade-in">
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />
      <div className="relative overflow-hidden rounded-[2.5rem] border border-border/40 bg-card/20 backdrop-blur-xl p-8 md:p-12 shadow-[0_8px_30px_rgb(0,0,0,0.02)]">
        <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/[0.03] rounded-full blur-[120px] -translate-y-1/2 translate-x-1/4 pointer-events-none" />
        <div className="relative z-10 flex flex-col md:flex-row md:items-center justify-between gap-10">
          <div className="max-w-2xl space-y-6">
            <div className="inline-flex items-center gap-2.5 px-4 py-1.5 rounded-full bg-primary/5 border border-primary/10 text-primary text-[10px] font-bold uppercase tracking-[0.2em]"><Layers className="h-3 w-3 opacity-70" /> Workspace Clusters</div>
            <h1 className="text-4xl md:text-5xl font-bold text-foreground tracking-tight leading-[1.1]">Active <span className="text-primary/80">Environments</span></h1>
            <p className="text-base md:text-lg text-muted-foreground font-medium leading-relaxed max-w-lg">Manage your isolated node clusters. Scale development environments with dedicated telemetry and context management.</p>
          </div>
          <div className="hidden lg:flex shrink-0"><div className="p-8 rounded-[2.5rem] bg-background/40 border border-border/60 shadow-inner backdrop-blur-md group hover:border-primary/20 transition-all duration-500"><LayoutDashboard className="w-16 h-16 text-primary/20 group-hover:text-primary/40 transition-colors duration-500" /></div></div>
        </div>
      </div>

      <div className="sticky top-14 z-30 flex flex-col gap-6 bg-background/80 backdrop-blur-xl py-6 border-b border-border/30">
        <div className="flex flex-col md:flex-row gap-6 md:items-center justify-between">
          <div className="relative flex-1 max-w-md group"><Search className="absolute left-4 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground/30 group-focus-within:text-primary/60 transition-colors" /><Input placeholder="Filter managed environments..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} className="h-12 pl-12 pr-6 bg-card/30 border-border/40 rounded-2xl shadow-sm focus-visible:ring-primary/10 transition-all" />{searchQuery && <button onClick={() => setSearchQuery('')} className="absolute right-4 top-1/2 -translate-y-1/2 text-muted-foreground/30 hover:text-foreground"><X className="w-4 h-4" /></button>}</div>
          <div className="hidden sm:flex items-center gap-3 text-[10px] font-bold uppercase tracking-widest text-muted-foreground/40 bg-muted/10 px-4 py-2 rounded-xl border border-border/30"><Info className="h-3.5 w-3.5" /><span>{projects.length} Nodes Registered</span></div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-8">
        {!searchQuery && <AddProjectCard onAdd={createProject} />}
        {filteredProjects.map((p) => <ProjectCard key={p.id} project={p} isDefault={p.id === defaultProjectId} isSelected={p.id === currentProjectId} onUpdate={handleUpdate} onDelete={setDeleteConfirmId} onSetDefault={setDefaultProject} onValidate={validateProject} onSelect={selectProject} />)}
        {filteredProjects.length === 0 && searchQuery && <div className="col-span-full flex flex-col items-center justify-center py-32 text-center space-y-8 rounded-[3rem] border border-dashed border-border/30 bg-muted/[0.02]"><div className="p-8 rounded-[2rem] bg-muted/10 border border-border/20 text-muted-foreground/20"><Search className="h-16 w-16" /></div><div className="space-y-2"><h3 className="text-2xl font-bold text-foreground/80 tracking-tight">Node Not Located</h3><p className="text-muted-foreground/40 max-w-xs mx-auto font-medium">Verify workspace identity and retry search protocol.</p></div><Button variant="outline" onClick={() => setSearchQuery('')} className="rounded-2xl font-bold px-10 border-border/60">Reset Search</Button></div>}
      </div>

      <div className="mt-16 pt-16 border-t border-border/30">
          <h3 className="text-[10px] font-black text-muted-foreground/40 uppercase tracking-[0.3em] mb-8 flex items-center gap-3"><div className="w-2 h-2 rounded-full bg-primary/20" /> Protocol Reference</h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
             {[ { cmd: 'quorum add .', sub: 'Register current terminal directory cluster.' }, { cmd: 'quorum project list', sub: 'Audit all active node clusters and status.' }, { cmd: 'quorum project default', sub: 'Set global session priority for terminal node.' } ].map((c, i) => (
               <div key={i} className="group p-6 rounded-[2rem] bg-muted/5 border border-border/30 hover:border-primary/20 transition-all hover:bg-muted/10">
                 <div className="flex items-center gap-3 mb-3"><div className="w-1.5 h-1.5 rounded-full bg-primary/40 group-hover:scale-150 transition-transform" /><code className="text-primary/80 font-mono text-sm font-bold">{c.cmd}</code></div>
                 <p className="text-muted-foreground/40 text-[11px] font-medium leading-relaxed">{c.sub}</p>
               </div>
             ))}
          </div>
      </div>

      <ConfirmDialog isOpen={deleteConfirmId !== null} onClose={() => setDeleteConfirmId(null)} onConfirm={() => handleDelete(deleteConfirmId)} title="Purge Workspace Context?" message={projectToDelete ? `Confirming the permanent decoupling of "${projectToDelete.name}". This action will not affect physical disk data, only the management index.` : ''} confirmText="Purge Context" variant="danger" />
    </div>
  );
}
