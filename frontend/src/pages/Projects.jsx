import { useState, useEffect, useRef } from 'react';
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
  Plus
} from 'lucide-react';
import { useProjectStore } from '../stores';
import ColorPicker, { PROJECT_COLORS } from '../components/ui/ColorPicker';
import { ConfirmDialog } from '../components/config/ConfirmDialog';

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
      <span className="text-xs font-medium capitalize">{config.label}</span>
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
      <span className="ml-2 text-xs text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity hidden sm:inline">
        
      </span>
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

  return (
    <div
      className={`relative group flex flex-col h-full rounded-xl border transition-all duration-200 overflow-hidden ${
        isSelected
          ? 'border-primary bg-primary/5 ring-1 ring-primary/20 shadow-md'
          : 'border-border bg-card hover:border-primary/30 hover:shadow-lg'
      }`}
    >
      {/* Header / Main Click Area */}
      <div 
        className="flex-1 p-5 cursor-pointer"
        onClick={() => onSelect(project.id)}
      >
        <div className="flex items-start justify-between gap-3 mb-3">
          {/* Color & Icon */}
          <div className="flex-shrink-0" onClick={(e) => e.stopPropagation()}>
             <ColorPicker
                value={project.color || PROJECT_COLORS[0]}
                onChange={handleColorChange}
              />
          </div>

          {/* Menu */}
          <div className="relative" ref={menuRef} onClick={(e) => e.stopPropagation()}>
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
        <div className="space-y-1 mb-4">
          <div className="flex items-center gap-2">
            <InlineEdit
              value={project.name}
              onSave={handleNameSave}
              placeholder="Project name"
              className="font-semibold text-lg text-foreground truncate"
            />
            {isDefault && (
              <Star className="w-4 h-4 fill-yellow-500 text-yellow-500 flex-shrink-0" title="Default Project" />
            )}
          </div>
          
          <div className="flex items-center gap-1.5 text-muted-foreground text-xs font-mono" onClick={(e) => e.stopPropagation()}>
            <Folder className="w-3.5 h-3.5 flex-shrink-0" />
            <InlineEdit
              value={project.path}
              onSave={handlePathSave}
              placeholder="/path/to/project"
              className="truncate"
              inputClassName="font-mono text-xs py-0 h-6"
            />
          </div>
        </div>

        {/* Status */}
         <div className="flex items-center justify-between mt-auto">
             <StatusBadge status={project.status} />
             
             {isSelected && (
              <span className="flex items-center gap-1 text-xs font-medium text-primary bg-primary/10 px-2 py-0.5 rounded-full">
                <Check className="w-3 h-3" /> Active
              </span>
             )}
         </div>

        {/* Status Message */}
        {project.status_message && (
          <div className="mt-3 p-2 rounded bg-yellow-500/10 text-yellow-700 text-xs flex items-start gap-1.5">
            <AlertCircle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
            <span className="line-clamp-2">{project.status_message}</span>
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Add project form component - Card Style
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
        className="flex flex-col items-center justify-center h-full min-h-[200px] p-6 rounded-xl border-2 border-dashed border-border hover:border-primary/50 hover:bg-primary/5 transition-all group text-muted-foreground hover:text-primary"
      >
        <div className="w-12 h-12 rounded-full bg-muted group-hover:bg-primary/10 flex items-center justify-center mb-3 transition-colors">
          <Plus className="w-6 h-6" />
        </div>
        <span className="font-medium">Add New Project</span>
      </button>
    );
  }

  return (
    <div className="h-full min-h-[200px] p-5 rounded-xl border border-primary/30 bg-primary/5 flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-semibold text-foreground flex items-center gap-2">
          <FolderPlus className="w-4 h-4" /> New Project
        </h3>
        <button
          onClick={() => {
            setIsExpanded(false);
            setError(null);
          }}
          className="text-muted-foreground hover:text-foreground"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      <form onSubmit={handleSubmit} className="flex-1 flex flex-col gap-3">
        <div>
           <label className="text-xs font-medium ml-1">Path <span className="text-destructive">*</span></label>
           <input
            type="text"
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            placeholder="/path/to/project"
            autoFocus
            className="w-full px-3 py-2 rounded-lg border border-input bg-background text-sm font-mono focus:ring-2 focus:ring-primary/20 outline-none"
          />
        </div>
       
        <div className="relative">
          <label className="text-xs font-medium ml-1">Name (Optional)</label>
          <input
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder={autoGeneratedName || "My Project"}
            className="w-full px-3 py-2 rounded-lg border border-input bg-background text-sm focus:ring-2 focus:ring-primary/20 outline-none"
          />
           {!newName && autoGeneratedName && (
              <span className="absolute right-3 top-7 text-xs text-muted-foreground/50 pointer-events-none">
                Auto: {autoGeneratedName}
              </span>
            )}
        </div>

        {error && (
          <p className="text-xs text-destructive flex items-center gap-1">
            <AlertCircle className="w-3 h-3" /> {error}
          </p>
        )}

        <div className="mt-auto pt-2 flex justify-end gap-2">
           <button
             type="button"
             onClick={() => setIsExpanded(false)}
             className="px-3 py-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
           >
             Cancel
           </button>
           <button
             type="submit"
             disabled={creating || !newPath.trim()}
             className="px-3 py-1.5 rounded-md bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50 flex items-center gap-1.5"
           >
             {creating ? <RefreshCw className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />}
             Add
           </button>
        </div>
      </form>
    </div>
  );
}

/**
 * Projects page - Dedicated section for project management.
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
  const filteredProjects = projects.filter(p => {
    const q = searchQuery.toLowerCase();
    return p.name?.toLowerCase().includes(q) || p.path?.toLowerCase().includes(q);
  });

  return (
    <div className="max-w-7xl mx-auto p-4 md:p-8 space-y-8">
      {/* Header & Controls */}
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold text-foreground tracking-tight">Projects</h1>
          <p className="text-muted-foreground mt-1 text-lg">
            Manage your workspaces and isolated environments.
          </p>
        </div>
        
        <div className="flex items-center gap-3 w-full md:w-auto">
           <div className="relative flex-1 md:w-64">
             <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
             <input 
               type="text"
               placeholder="Search projects..."
               value={searchQuery}
               onChange={(e) => setSearchQuery(e.target.value)}
               className="w-full pl-9 pr-4 py-2 rounded-lg border border-input bg-background focus:ring-2 focus:ring-primary/20 outline-none transition-shadow"
             />
           </div>
        </div>
      </div>

      {/* Global Error */}
      {error && (
        <div className="p-4 rounded-xl bg-destructive/10 text-destructive flex items-center justify-between border border-destructive/20">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-5 h-5" />
            <span>{error}</span>
          </div>
          <button onClick={clearError} className="text-sm font-medium hover:underline">
            Dismiss
          </button>
        </div>
      )}

      {/* Projects Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {/* Add New Card - Always first */}
        <AddProjectCard onAdd={createProject} />

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
      </div>

      {/* Empty State (Search Results) */}
      {filteredProjects.length === 0 && projects.length > 0 && (
         <div className="text-center py-12 text-muted-foreground">
            <Search className="w-12 h-12 mx-auto mb-3 opacity-20" />
            <p>No projects match &quot;{searchQuery}&quot;</p>
         </div>
      )}

      {/* Empty State (No Projects) */}
      {projects.length === 0 && (
        <div className="col-span-full hidden"> {/* Hidden because AddProjectCard is visible */} </div>
      )}

       {/* CLI Hint Footer */}
      <div className="mt-8 pt-8 border-t border-border">
          <h3 className="text-sm font-medium text-foreground mb-3 flex items-center gap-2">
            <div className="w-1.5 h-1.5 rounded-full bg-primary" />
            CLI Quick Reference
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
             <div className="p-3 rounded-lg bg-muted/50 border border-border">
               <code className="text-primary font-mono block mb-1">quorum add .</code>
               <span className="text-muted-foreground text-xs">Register current directory</span>
             </div>
             <div className="p-3 rounded-lg bg-muted/50 border border-border">
               <code className="text-primary font-mono block mb-1">quorum project list</code>
               <span className="text-muted-foreground text-xs">View all projects</span>
             </div>
             <div className="p-3 rounded-lg bg-muted/50 border border-border">
               <code className="text-primary font-mono block mb-1">quorum project default &lt;id&gt;</code>
               <span className="text-muted-foreground text-xs">Set default project</span>
             </div>
          </div>
      </div>

      {/* Delete Dialog */}
      <ConfirmDialog
        isOpen={deleteConfirmId !== null}
        onClose={() => setDeleteConfirmId(null)}
        onConfirm={() => handleDeleteProject(deleteConfirmId)}
        title="Remove Project"
        message={
          projectToDelete
            ? `Remove "${projectToDelete.name}" from the registry? The project files will not be deleted.`
            : ''
        }
        confirmText="Remove"
        variant="danger"
      />
    </div>
  );
}
