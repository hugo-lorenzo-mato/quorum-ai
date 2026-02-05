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
  Layers
} from 'lucide-react';
import { useProjectStore } from '../stores';
import ColorPicker, { PROJECT_COLORS } from '../components/ui/ColorPicker';
import { ConfirmDialog } from '../components/config/ConfirmDialog';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Input } from '../components/ui/Input';

/**
 * Generate a project name from a path (same logic as backend).
 */
function generateNameFromPath(path) {
  if (!path) return '';
  const baseName = path.split('/').pop() || path.split('\\').pop() || '';
  if (!baseName) return '';

  let name = baseName.charAt(0).toUpperCase() + baseName.slice(1);
  name = name.replace(/[-_]/g, ' ');
  return name;
}

/**
 * Status badge component for project health.
 */
function StatusBadge({ status }) {
  const config = {
    healthy: { color: 'bg-green-500', textColor: 'text-green-600', label: 'Healthy' },
    degraded: { color: 'bg-yellow-500', textColor: 'text-yellow-600', label: 'Degraded' },
    offline: { color: 'bg-red-500', textColor: 'text-red-600', label: 'Offline' },
    initializing: { color: 'bg-blue-500', textColor: 'text-blue-600', label: 'Initializing' },
  }[status] || { color: 'bg-gray-500', textColor: 'text-gray-600', label: status };

  return (
    <div className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-opacity-10 ${config.textColor}`} style={{ backgroundColor: `${config.color}20` }}>
      <div className={`w-1.5 h-1.5 rounded-full ${config.color}`} />
      <span className="text-[10px] font-bold uppercase tracking-wide">{config.label}</span>
    </div>
  );
}

/**
 * Inline editable text field component.
 */
function InlineEdit({ value, onSave, placeholder, className = '', inputClassName = '' }) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(value);
  const inputRef = useRef(null);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  useEffect(() => {
    setEditValue(value);
  }, [value]);

  const handleSave = () => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== value) {
      onSave(trimmed);
    }
    setIsEditing(false);
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSave();
    } else if (e.key === 'Escape') {
      setEditValue(value);
      setIsEditing(false);
    }
  };

  if (isEditing) {
    return (
      <input
        ref={inputRef}
        type="text"
        value={editValue}
        onChange={(e) => setEditValue(e.target.value)}
        onBlur={handleSave}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        className={`px-2 py-1 rounded-md border border-primary bg-background text-foreground focus:outline-none focus:ring-2 focus:ring-ring w-full min-w-0 ${inputClassName}`}
        onClick={(e) => e.stopPropagation()}
      />
    );
  }

  return (
    <div
      onClick={(e) => {
        e.stopPropagation();
        setIsEditing(true);
      }}
      className={`text-left hover:bg-accent/50 rounded-md px-2 py-1 -mx-2 -my-1 transition-colors group max-w-full truncate min-w-0 cursor-text ${className}`}
      title="Click to edit"
    >
      {value || <span className="text-muted-foreground italic">{placeholder}</span>}
    </div>
  );
}

/**
 * Project card component.
 */
function ProjectCard({
  project,
  isDefault,
  isSelected,
  onUpdate,
  onDelete,
  onSetDefault,
  onValidate,
  onSelect,
}) {
  const [showMenu, setShowMenu] = useState(false);
  const menuRef = useRef(null);

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (menuRef.current && !menuRef.current.contains(event.target)) {
        setShowMenu(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleNameSave = (name) => {
    if (name !== project.name) {
      onUpdate(project.id, { name });
    }
  };

  const handlePathSave = (path) => {
    if (path !== project.path) {
      onUpdate(project.id, { path });
    }
  };

  const handleColorChange = (color) => {
    if (color !== project.color) {
      onUpdate(project.id, { color });
    }
  };

  // Convert tailwind color class or hex to border class
  const borderStyle = {
    borderLeftColor: project.color || '#71717a',
    borderLeftWidth: '4px'
  };

  return (
    <div
      className={`group flex flex-col h-full rounded-xl border border-border bg-card/50 backdrop-blur-sm transition-all duration-300 hover:shadow-xl hover:-translate-y-1 overflow-hidden ${
        isSelected ? 'ring-1 ring-primary/20 shadow-md' : ''
      }`}
      style={borderStyle}
    >
      {/* Header Area */}
      <div className="flex-1 p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          {/* Color & Icon */}
          <div className="flex-shrink-0 p-2 rounded-lg bg-background border border-border shadow-sm group-hover:border-primary/30 transition-colors">
             <ColorPicker
                value={project.color || PROJECT_COLORS[0]}
                onChange={handleColorChange}
              />
          </div>

          {/* Menu */}
          <div className="relative" ref={menuRef}>
            <button
              onClick={() => setShowMenu(!showMenu)}
              className="p-1.5 rounded-lg text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
            >
              <MoreVertical className="w-4 h-4" />
            </button>

            {showMenu && (
              <div className="absolute right-0 top-full mt-1 w-48 rounded-lg border border-border bg-popover shadow-xl z-10 py-1 text-sm animate-in fade-in zoom-in-95 duration-100">
                 {!isSelected && (
                  <button
                    onClick={() => {
                      onSelect(project.id);
                      setShowMenu(false);
                    }}
                    className="w-full text-left px-3 py-2 hover:bg-accent flex items-center gap-2"
                  >
                    <FolderOpen className="w-4 h-4" /> Open
                  </button>
                )}
                <button
                  onClick={() => {
                    onValidate(project.id);
                    setShowMenu(false);
                  }}
                  className="w-full text-left px-3 py-2 hover:bg-accent flex items-center gap-2"
                >
                  <RefreshCw className="w-4 h-4" /> Validate
                </button>
                {!isDefault && (
                  <button
                    onClick={() => {
                      onSetDefault(project.id);
                      setShowMenu(false);
                    }}
                    className="w-full text-left px-3 py-2 hover:bg-accent flex items-center gap-2"
                  >
                    <Star className="w-4 h-4" /> Set as Default
                  </button>
                )}
                <div className="h-px bg-border my-1" />
                <button
                  onClick={() => {
                    onDelete(project.id);
                    setShowMenu(false);
                  }}
                  className="w-full text-left px-3 py-2 hover:bg-red-50 text-red-600 flex items-center gap-2"
                >
                  <Trash2 className="w-4 h-4" /> Remove
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Title & Path */}
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <InlineEdit
              value={project.name}
              onSave={handleNameSave}
              placeholder="Project name"
              className="font-bold text-lg text-foreground truncate group-hover:text-primary transition-colors"
            />
            {isDefault && (
              <Star className="w-4 h-4 fill-yellow-500 text-yellow-500 flex-shrink-0" title="Default Project" />
            )}
          </div>
          
          <div className="flex items-center gap-1.5 text-muted-foreground text-xs font-mono">
            <Folder className="w-3.5 h-3.5 flex-shrink-0" />
            <InlineEdit
              value={project.path}
              onSave={handlePathSave}
              placeholder="/path/to/project"
              className="truncate opacity-70 group-hover:opacity-100 transition-opacity"
              inputClassName="font-mono text-xs py-0 h-6"
            />
          </div>
        </div>

        {/* Status Indicators */}
        <div className="flex items-center justify-between pt-2">
           <StatusBadge status={project.status} />
           {isSelected && (
              <Badge variant="secondary" className="text-[9px] uppercase tracking-widest font-black bg-primary/10 text-primary border-primary/20">
                Active
              </Badge>
           )}
        </div>

        {/* Status Message */}
        {project.status_message && (
          <div className="mt-3 p-2 rounded-lg bg-yellow-500/5 border border-yellow-500/10 text-yellow-700 dark:text-yellow-400 text-[10px] leading-relaxed flex items-start gap-1.5">
            <AlertCircle className="w-3 h-3 mt-0.5 flex-shrink-0" />
            <span className="line-clamp-2">{project.status_message}</span>
          </div>
        )}
      </div>

      {/* Footer Actions */}
      <div className="p-4 pt-0 mt-auto flex gap-2">
        <Button 
          variant="outline" 
          size="sm"
          onClick={() => onSelect(project.id)}
          className="flex-1 rounded-lg text-xs font-semibold h-9"
        >
          Open
        </Button>
        <Button 
          size="sm"
          onClick={() => onValidate(project.id)}
          className="flex-1 rounded-lg text-xs font-bold h-9 shadow-sm shadow-primary/10"
        >
          Validate
          <ChevronRight className="ml-1 h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}

/**
 * Add project card component.
 */
function AddProjectCard({ onAdd }) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [newPath, setNewPath] = useState('');
  const [newName, setNewName] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);

  const autoGeneratedName = generateNameFromPath(newPath);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!newPath.trim()) return;

    setCreating(true);
    setError(null);

    try {
      await onAdd(newPath.trim(), { name: newName.trim() || undefined });
      setNewPath('');
      setNewName('');
      setIsExpanded(false);
    } catch (err) {
      setError(err.message || 'Failed to add project');
    } finally {
      setCreating(false);
    }
  };

  if (!isExpanded) {
    return (
      <button
        onClick={() => setIsExpanded(true)}
        className="flex flex-col items-center justify-center h-full min-h-[220px] p-6 rounded-xl border-2 border-dashed border-border hover:border-primary/50 hover:bg-primary/5 transition-all group text-muted-foreground hover:text-primary bg-card/20"
      >
        <div className="w-12 h-12 rounded-full bg-muted group-hover:bg-primary/10 flex items-center justify-center mb-3 transition-colors">
          <Plus className="w-6 h-6" />
        </div>
        <span className="font-bold tracking-tight">Add New Project</span>
        <p className="text-xs mt-1 opacity-60">Register existing directory</p>
      </button>
    );
  }

  return (
    <div className="h-full min-h-[220px] p-5 rounded-xl border border-primary/30 bg-primary/5 backdrop-blur-sm flex flex-col animate-fade-in shadow-lg">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-bold text-foreground flex items-center gap-2">
          <FolderPlus className="w-4 h-4 text-primary" /> 
          New Project
        </h3>
        <button
          onClick={() => {
            setIsExpanded(false);
            setError(null);
          }}
          className="p-1 rounded-md hover:bg-primary/10 text-muted-foreground hover:text-foreground transition-colors"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      <form onSubmit={handleSubmit} className="flex-1 flex flex-col gap-3">
        <div className="space-y-1">
           <label className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground ml-1">Path</label>
           <input
            type="text"
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            placeholder="/path/to/project"
            autoFocus
            className="w-full px-3 py-2 rounded-lg border border-input bg-background text-sm font-mono focus:ring-2 focus:ring-primary/20 outline-none transition-shadow shadow-sm"
          />
        </div>
       
        <div className="relative space-y-1">
          <label className="text-[10px] font-bold uppercase tracking-widest text-muted-foreground ml-1">Name (Optional)</label>
          <input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder={autoGeneratedName || "My Project"}
            className="w-full px-3 py-2 rounded-lg border border-input bg-background text-sm focus:ring-2 focus:ring-primary/20 outline-none transition-shadow shadow-sm"
          />
           {!newName && autoGeneratedName && (
              <span className="absolute right-3 top-8 text-[10px] text-muted-foreground/40 pointer-events-none font-medium">
                Auto: {autoGeneratedName}
              </span>
            )}
        </div>

        {error && (
          <p className="text-[11px] text-destructive flex items-center gap-1.5 font-medium bg-destructive/10 p-2 rounded-md border border-destructive/20 animate-shake">
            <AlertCircle className="w-3.5 h-3.5" /> {error}
          </p>
        )}

        <div className="mt-auto pt-4 flex justify-end gap-2">
           <Button
             type="button"
             variant="ghost"
             size="sm"
             onClick={() => setIsExpanded(false)}
             className="text-xs font-bold"
           >
             Cancel
           </Button>
           <Button
             type="submit"
             size="sm"
             disabled={creating || !newPath.trim()}
             className="px-4 rounded-lg text-xs font-bold shadow-sm shadow-primary/20"
           >
             {creating ? <RefreshCw className="w-3 h-3 animate-spin mr-1.5" /> : <Plus className="w-3 h-3 mr-1.5" />}
             Register
           </Button>
        </div>
      </form>
    </div>
  );
}

/**
 * Projects page component.
 */
export default function Projects() {
  const {
    projects,
    currentProjectId,
    defaultProjectId,
    error,
    fetchProjects,
    createProject,
    updateProject,
    deleteProject,
    setDefaultProject,
    validateProject,
    selectProject,
    clearError,
  } = useProjectStore();

  const [deleteConfirmId, setDeleteConfirmId] = useState(null);
  const [searchQuery, setSearchQuery] = useState('');
  
  // Fetch projects on mount
  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  const handleUpdateProject = async (id, data) => {
    try {
      await updateProject(id, data);
    } catch {
      // Error handled by store
    }
  };

  const handleDeleteProject = async (id) => {
    try {
      await deleteProject(id);
      setDeleteConfirmId(null);
    } catch {
      // Error handled by store
    }
  };

  const projectToDelete = projects.find((p) => p.id === deleteConfirmId);

  // Filter projects
  const filteredProjects = useMemo(() => {
    return projects.filter(p => {
      const q = searchQuery.toLowerCase();
      return p.name?.toLowerCase().includes(q) || p.path?.toLowerCase().includes(q);
    });
  }, [projects, searchQuery]);

  return (
    <div className="relative min-h-full space-y-8 pb-12 animate-fade-in">
      {/* Background Pattern - Consistent across app */}
      <div className="absolute inset-0 bg-dot-pattern pointer-events-none -z-10" />

      {/* Hero Header - Unified style with Templates */}
      <div className="relative overflow-hidden rounded-3xl border border-border bg-card/30 backdrop-blur-md p-8 sm:p-12 shadow-sm">
        <div className="absolute top-0 right-0 -translate-y-1/2 translate-x-1/3 w-96 h-96 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        <div className="absolute bottom-0 left-0 translate-y-1/2 -translate-x-1/3 w-64 h-64 bg-primary/5 rounded-full blur-3xl pointer-events-none" />
        
        <div className="relative z-10 max-w-2xl space-y-4">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20 text-primary text-[10px] font-bold uppercase tracking-widest">
            <Layers className="h-3 w-3" />
            Workspace Manager
          </div>
          <h1 className="text-4xl sm:text-5xl font-black text-foreground tracking-tight leading-tight">
            Active <span className="text-primary">Projects</span>
          </h1>
          <p className="text-lg text-muted-foreground leading-relaxed max-w-xl">
            Manage your isolated development environments. Switch between projects to maintain dedicated context, agents, and workflows.
          </p>
        </div>
      </div>

      {/* Control Bar - Unified style with Templates */}
      <div className="sticky top-14 z-30 flex flex-col gap-4 bg-background/80 backdrop-blur-md py-4 border-b border-border/50">
        <div className="flex flex-col md:flex-row gap-4 md:items-center justify-between">
          {/* Search */}
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="Filter projects by name or path..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-11 bg-background border-border rounded-xl shadow-sm focus-visible:ring-primary/20"
            />
            {searchQuery && (
              <button 
                onClick={() => setSearchQuery('')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              >
                <X className="h-4 w-4" />
              </button>
            )}
          </div>

          {/* Stats */}
          <div className="hidden sm:flex items-center gap-2 text-xs font-medium text-muted-foreground bg-muted/50 px-3 py-2 rounded-lg border border-border/50">
            <Info className="h-3.5 w-3.5" />
            <span>{projects.length} Registered Workspaces</span>
          </div>
        </div>
      </div>

      {/* Global Error Display */}
      {error && (
        <div className="p-4 rounded-xl bg-destructive/5 text-destructive flex items-center justify-between border border-destructive/20 animate-fade-in shadow-sm">
          <div className="flex items-center gap-3">
            <AlertCircle className="w-5 h-5" />
            <span className="text-sm font-medium">{error}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={clearError} className="text-xs font-bold hover:bg-destructive/10">
            Dismiss
          </Button>
        </div>
      )}

      {/* Projects Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {/* Add New Project Card - Always visible unless filtered to zero result */}
        {!searchQuery && <AddProjectCard onAdd={createProject} />}

        {/* Project Cards */}
        {filteredProjects.map((project) => (
          <ProjectCard
            key={project.id}
            project={project}
            isDefault={project.id === defaultProjectId}
            isSelected={project.id === currentProjectId}
            onUpdate={handleUpdateProject}
            onDelete={setDeleteConfirmId}
            onSetDefault={setDefaultProject}
            onValidate={validateProject}
            onSelect={selectProject}
          />
        ))}

        {/* Empty State for Search */}
        {filteredProjects.length === 0 && searchQuery && (
           <div className="col-span-full flex flex-col items-center justify-center py-20 text-center space-y-4 rounded-3xl border border-dashed border-border bg-muted/5">
              <div className="p-4 rounded-full bg-muted border border-border">
                <Search className="h-8 w-8 text-muted-foreground/30" />
              </div>
              <div className="space-y-1">
                <h3 className="text-xl font-bold text-foreground">No projects found</h3>
                <p className="text-muted-foreground max-w-xs mx-auto">
                  We couldn't find any workspace matching &quot;{searchQuery}&quot;.
                </p>
              </div>
              <Button variant="outline" onClick={() => setSearchQuery('')} className="rounded-xl">
                Clear search
              </Button>
           </div>
        )}
      </div>

       {/* CLI Quick Reference Footer */}
      <div className="mt-12 pt-12 border-t border-border/50">
          <h3 className="text-xs font-bold text-muted-foreground uppercase tracking-widest mb-6 flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-primary/40" />
            CLI Quick Reference
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-6">
             <div className="group p-4 rounded-2xl bg-muted/30 border border-border/50 hover:border-primary/30 transition-all hover:bg-muted/50">
               <div className="flex items-center gap-2 mb-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                  <code className="text-primary font-mono text-sm font-bold">quorum add .</code>
               </div>
               <span className="text-muted-foreground text-xs leading-relaxed">Instantly register and switch to the current terminal directory.</span>
             </div>
             
             <div className="group p-4 rounded-2xl bg-muted/30 border border-border/50 hover:border-primary/30 transition-all hover:bg-muted/50">
               <div className="flex items-center gap-2 mb-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                  <code className="text-primary font-mono text-sm font-bold">quorum project list</code>
               </div>
               <span className="text-muted-foreground text-xs leading-relaxed">View a detailed list of all managed environments and their status.</span>
             </div>

             <div className="group p-4 rounded-2xl bg-muted/30 border border-border/50 hover:border-primary/30 transition-all hover:bg-muted/50">
               <div className="flex items-center gap-2 mb-2">
                  <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                  <code className="text-primary font-mono text-sm font-bold">quorum project default</code>
               </div>
               <span className="text-muted-foreground text-xs leading-relaxed">Set your preferred workspace for the current terminal session.</span>
             </div>
          </div>
      </div>

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={deleteConfirmId !== null}
        onClose={() => setDeleteConfirmId(null)}
        onConfirm={() => handleDeleteProject(deleteConfirmId)}
        title="Remove Project"
        message={
          projectToDelete
            ? `Are you sure you want to remove "${projectToDelete.name}"? This action only removes the entry from Quorum AI, your project files will remain untouched.`
            : ''
        }
        confirmText="Remove Project"
        variant="danger"
      />
    </div>
  );
}