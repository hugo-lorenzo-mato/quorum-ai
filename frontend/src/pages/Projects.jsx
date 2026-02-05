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
      />
    );
  }

  return (
    <button
      onClick={() => setIsEditing(true)}
      className={`text-left hover:bg-accent/50 rounded-md px-2 py-1 -mx-2 -my-1 transition-colors group max-w-full truncate min-w-0 ${className}`}
      title="Click to edit"
    >
      {value || <span className="text-muted-foreground italic">{placeholder}</span>}
      <span className="ml-2 text-xs text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity hidden sm:inline">
        (click to edit)
      </span>
    </button>
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
  loading,
}) {
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
      className={`relative p-4 rounded-xl border transition-all duration-300 overflow-hidden ${
        isSelected
          ? 'border-primary bg-primary/5 ring-2 ring-primary/20'
          : 'border-border bg-card hover:border-primary/40 hover:shadow-xl hover:shadow-primary/5 hover:-translate-y-1 hover:scale-[1.01]'
      }`}
    >
      {/* Header row */}
      <div className="flex flex-col sm:flex-row items-start gap-4 mb-4">
        {/* Color picker */}
        <div className="flex-shrink-0 pt-1">
          <ColorPicker
            value={project.color || PROJECT_COLORS[0]}
            onChange={handleColorChange}
          />
        </div>

        {/* Project info */}
        <div className="flex-1 min-w-0 w-full">
          <div className="flex flex-wrap items-center gap-2 mb-1">
            <InlineEdit
              value={project.name}
              onSave={handleNameSave}
              placeholder="Project name"
              className="text-lg font-semibold text-foreground block truncate"
            />
            {isDefault && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-yellow-500/10 text-yellow-600 text-xs font-medium whitespace-nowrap">
                <Star className="w-3 h-3 fill-yellow-500" />
                Default
              </span>
            )}
            {isSelected && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-primary/10 text-primary text-xs font-medium whitespace-nowrap">
                <Check className="w-3 h-3" />
                Active
              </span>
            )}
          </div>
          <StatusBadge status={project.status} />
        </div>
      </div>

      {/* Path row */}
      <div className="flex items-center gap-2 mb-4 p-3 rounded-lg bg-muted/30 overflow-hidden">
        <Folder className="w-4 h-4 text-muted-foreground flex-shrink-0" />
        <InlineEdit
          value={project.path}
          onSave={handlePathSave}
          placeholder="/path/to/project"
          className="text-sm text-muted-foreground font-mono flex-1 truncate"
          inputClassName="font-mono text-sm w-full"
        />
      </div>

      {/* Actions row */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-wrap">
        {!isSelected && (
          <button
            onClick={() => onSelect(project.id)}
            className="w-full sm:w-auto px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors flex items-center justify-center gap-2"
          >
            <FolderOpen className="w-4 h-4" />
            Open Project
          </button>
        )}

        <div className="flex gap-2 w-full sm:w-auto sm:flex-1">
          {!isDefault && (
            <button
              onClick={() => onSetDefault(project.id)}
              disabled={loading}
              className="flex-1 px-3 py-2 rounded-lg text-sm font-medium border border-border hover:bg-accent transition-colors flex items-center justify-center gap-2 text-muted-foreground hover:text-foreground"
            >
              <Star className="w-4 h-4" />
              <span className="sm:hidden lg:inline">Set Default</span>
              <span className="hidden sm:inline lg:hidden">Default</span>
            </button>
          )}

          <button
            onClick={() => onValidate(project.id)}
            disabled={loading}
            className="flex-1 px-3 py-2 rounded-lg text-sm font-medium border border-border hover:bg-accent transition-colors flex items-center justify-center gap-2 text-muted-foreground hover:text-foreground"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Validate
          </button>
        </div>

        <div className="hidden sm:block sm:flex-1" />

        <button
          onClick={() => onDelete(project.id)}
          disabled={loading}
          className="w-full sm:w-auto px-3 py-2 rounded-lg text-sm font-medium border border-border hover:bg-destructive/10 hover:border-destructive/50 hover:text-destructive transition-colors flex items-center justify-center gap-2 text-muted-foreground"
        >
          <Trash2 className="w-4 h-4" />
          Remove
        </button>
      </div>

      {/* Status message if any */}
      {project.status_message && (
        <div className="mt-3 p-2 rounded-lg bg-yellow-500/10 text-yellow-700 text-sm flex items-start gap-2 break-words">
          <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span className="min-w-0">{project.status_message}</span>
        </div>
      )}
    </div>
  );
}

/**
 * Add project form component.
 */
function AddProjectForm({ onAdd }) {
  const [newPath, setNewPath] = useState('');
  const [newName, setNewName] = useState('');
  const [newColor, setNewColor] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState(null);
  const [expanded, setExpanded] = useState(false);

  const autoGeneratedName = generateNameFromPath(newPath);

  const handleSubmit = async (e) => {
    e.preventDefault();

    if (!newPath.trim()) {
      setError('Path is required');
      return;
    }

    setCreating(true);
    setError(null);

    try {
      const options = {};
      if (newName.trim()) {
        options.name = newName.trim();
      }
      if (newColor) {
        options.color = newColor;
      }

      await onAdd(newPath.trim(), options);

      // Reset form
      setNewPath('');
      setNewName('');
      setNewColor('');
      setExpanded(false);
    } catch (err) {
      setError(err.message || 'Failed to add project');
    } finally {
      setCreating(false);
    }
  };

  if (!expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        className="w-full p-6 rounded-xl border-2 border-dashed border-border hover:border-primary/50 hover:bg-primary/5 transition-all flex items-center justify-center gap-3 text-muted-foreground hover:text-foreground group"
      >
        <div className="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center group-hover:bg-primary/20 transition-colors">
          <FolderPlus className="w-6 h-6 text-primary" />
        </div>
        <div className="text-left">
          <div className="font-semibold">Add New Project</div>
          <div className="text-sm text-muted-foreground">Register an existing project directory</div>
        </div>
      </button>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="p-6 rounded-xl border-2 border-primary/30 bg-primary/5">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
          <FolderPlus className="w-5 h-5 text-primary" />
          Add New Project
        </h3>
        <button
          type="button"
          onClick={() => {
            setExpanded(false);
            setError(null);
          }}
          className="p-1.5 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
        >
          <X className="w-5 h-5" />
        </button>
      </div>

      <div className="space-y-4">
        {/* Path input */}
        <div className="w-full min-w-0">
          <label className="block text-sm font-medium text-foreground mb-1.5">
            Project Path <span className="text-destructive">*</span>
          </label>
          <input
            type="text"
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            placeholder="/home/user/my-project"
            autoFocus
            className="w-full px-4 py-2.5 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring font-mono text-sm min-w-0"
          />
        </div>

        {/* Name input with preview */}
        <div className="w-full min-w-0">
          <label className="block text-sm font-medium text-foreground mb-1.5">
            Project Name{' '}
            <span className="text-muted-foreground font-normal">(optional)</span>
          </label>
          <div className="relative w-full">
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={autoGeneratedName || 'My project'}
              className="w-full px-4 py-2.5 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground/50 focus:outline-none focus:ring-2 focus:ring-ring text-sm min-w-0"
            />
            {!newName && autoGeneratedName && (
              <span className="absolute right-4 top-1/2 -translate-y-1/2 text-sm text-muted-foreground hidden sm:inline-block pointer-events-none">
                Auto: {autoGeneratedName}
              </span>
            )}
          </div>
        </div>

        {/* Color picker */}
        <div>
          <label className="block text-sm font-medium text-foreground mb-1.5">
            Color
          </label>
          <div className="flex items-center gap-3">
            <ColorPicker
              value={newColor || PROJECT_COLORS[0]}
              onChange={setNewColor}
            />
            <span className="text-sm text-muted-foreground">
              Choose a color to identify this project
            </span>
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 text-destructive text-sm">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            {error}
          </div>
        )}

        {/* Submit button */}
        <div className="flex justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={() => {
              setExpanded(false);
              setError(null);
            }}
            className="px-4 py-2 rounded-lg text-sm font-medium text-foreground hover:bg-accent transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={creating || !newPath.trim()}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {creating ? (
              <>
                <RefreshCw className="w-4 h-4 animate-spin" />
                Adding...
              </>
            ) : (
              <>
                <FolderPlus className="w-4 h-4" />
                Add Project
              </>
            )}
          </button>
        </div>
      </div>
    </form>
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
    loading,
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

  // Delete confirmation
  const [deleteConfirmId, setDeleteConfirmId] = useState(null);

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

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between px-6">
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage your registered projects. Each project has isolated workflows, chat sessions, and settings.
          </p>
        </div>
      </div>

      {/* Content container with padding */}
      <div className="px-6 space-y-6">
      {/* Global error */}
      {error && (
        <div className="p-4 rounded-xl bg-destructive/10 text-destructive flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-5 h-5" />
            <span>{error}</span>
          </div>
          <button onClick={clearError} className="text-sm hover:underline">
            Dismiss
          </button>
        </div>
      )}

      {/* Add project form */}
      <AddProjectForm onAdd={createProject} />

      {/* Projects list */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">
          Registered Projects ({projects.length})
        </h2>

        {projects.length === 0 ? (
          <div className="text-center py-12 rounded-xl border border-border bg-card">
            <Folder className="w-16 h-16 mx-auto mb-4 text-muted-foreground/30" />
            <p className="text-lg font-medium text-foreground mb-1">No projects registered</p>
            <p className="text-muted-foreground mb-4">
              Add your first project using the form above or via CLI:
            </p>
            <code className="px-4 py-2 rounded-lg bg-muted font-mono text-sm">
              quorum add /path/to/project
            </code>
          </div>
        ) : (
          <div className="grid gap-4">
            {projects.map((project) => (
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
                loading={loading}
              />
            ))}
          </div>
        )}
      </div>

      {/* CLI hint */}
      <div className="p-4 rounded-xl bg-muted/30 border border-border overflow-x-auto">
        <h3 className="text-sm font-medium text-foreground mb-2">Command Line</h3>
        <p className="text-sm text-muted-foreground mb-2">
          You can also manage projects via the CLI:
        </p>
        <div className="space-y-1 font-mono text-sm min-w-max">
          <div><code className="text-primary">quorum add .</code> - Register current directory</div>
          <div><code className="text-primary">quorum project list</code> - List all projects</div>
          <div><code className="text-primary">quorum project default &lt;id&gt;</code> - Set default project</div>
        </div>
      </div>

      {/* Delete confirmation dialog */}
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
    </div>
  );
}
