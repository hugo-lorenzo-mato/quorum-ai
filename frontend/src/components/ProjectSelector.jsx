import { useEffect, useState } from 'react';
import { ChevronDown, FolderKanban, Check, Plus, Star, AlertCircle, Folder } from 'lucide-react';
import { useProjectStore } from '../stores';

/**
 * Project selector dropdown for multi-project support.
 * Displays in the sidebar and allows users to switch between projects.
 */
export default function ProjectSelector({ collapsed = false }) {
  const [isOpen, setIsOpen] = useState(false);
  const {
    projects,
    currentProjectId,
    loading,
    error,
    fetchProjects,
    selectProject,
    setDefaultProject,
    getCurrentProject,
  } = useProjectStore();

  const currentProject = getCurrentProject();

  useEffect(() => {
    fetchProjects();
  }, [fetchProjects]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (isOpen && !e.target.closest('.project-selector')) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen]);

  const handleSelectProject = (projectId) => {
    selectProject(projectId);
    setIsOpen(false);
  };

  const handleSetDefault = (e, projectId) => {
    e.stopPropagation();
    setDefaultProject(projectId);
  };

  const getStatusColor = (status) => {
    switch (status) {
      case 'healthy':
        return 'text-green-500';
      case 'degraded':
        return 'text-yellow-500';
      case 'unavailable':
        return 'text-red-500';
      default:
        return 'text-muted-foreground';
    }
  };

  // Collapsed view - just show an icon
  if (collapsed) {
    return (
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="project-selector relative w-full p-2 rounded-lg bg-secondary hover:bg-accent transition-colors flex items-center justify-center"
        title={currentProject?.name || 'Select project'}
      >
        <FolderKanban className="w-5 h-5 text-primary" />
        {currentProject && (
          <div
            className="absolute bottom-0 right-0 w-2.5 h-2.5 rounded-full border-2 border-background"
            style={{ backgroundColor: currentProject.color || '#4A90D9' }}
          />
        )}

        {/* Dropdown */}
        {isOpen && (
          <div className="absolute left-full ml-2 top-0 w-56 py-1 bg-popover border border-border rounded-lg shadow-lg z-50">
            <div className="px-3 py-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Projects
            </div>
            {projects.map((project) => (
              <button
                key={project.id}
                onClick={() => handleSelectProject(project.id)}
                className="w-full px-3 py-2 flex items-center gap-2 hover:bg-accent transition-colors text-left"
              >
                <div
                  className="w-3 h-3 rounded-full flex-shrink-0"
                  style={{ backgroundColor: project.color || '#4A90D9' }}
                />
                <span className="flex-1 truncate text-sm">{project.name}</span>
                {project.id === currentProjectId && (
                  <Check className="w-4 h-4 text-primary" />
                )}
              </button>
            ))}
          </div>
        )}
      </button>
    );
  }

  // Expanded view
  return (
    <div className="project-selector relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        disabled={loading}
        className="w-full px-3 py-2 rounded-lg bg-secondary hover:bg-accent transition-colors flex items-center gap-3"
      >
        {currentProject ? (
          <>
            <div
              className="w-3 h-3 rounded-full flex-shrink-0"
              style={{ backgroundColor: currentProject.color || '#4A90D9' }}
            />
            <div className="flex-1 min-w-0 text-left">
              <div className="text-sm font-medium truncate">{currentProject.name}</div>
              <div className="text-xs text-muted-foreground truncate">
                {currentProject.path}
              </div>
            </div>
          </>
        ) : (
          <>
            <Folder className="w-4 h-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">Select project...</span>
          </>
        )}
        <ChevronDown
          className={`w-4 h-4 text-muted-foreground transition-transform ${isOpen ? 'rotate-180' : ''}`}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div className="absolute left-0 right-0 mt-1 py-1 bg-popover border border-border rounded-lg shadow-lg z-50">
          {error && (
            <div className="px-3 py-2 text-xs text-red-500 flex items-center gap-1">
              <AlertCircle className="w-3 h-3" />
              {error}
            </div>
          )}

          {projects.length === 0 && !loading ? (
            <div className="px-3 py-4 text-sm text-muted-foreground text-center">
              No projects registered
            </div>
          ) : (
            projects.map((project) => (
              <div
                key={project.id}
                className="group flex items-center gap-1 px-1"
              >
                <button
                  onClick={() => handleSelectProject(project.id)}
                  className="flex-1 px-2 py-2 flex items-center gap-2 hover:bg-accent rounded-md transition-colors text-left"
                >
                  <div
                    className="w-3 h-3 rounded-full flex-shrink-0"
                    style={{ backgroundColor: project.color || '#4A90D9' }}
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-1">
                      <span className="text-sm truncate">{project.name}</span>
                      {project.is_default && (
                        <Star className="w-3 h-3 text-yellow-500 fill-yellow-500" />
                      )}
                    </div>
                    <div className="text-xs text-muted-foreground truncate">
                      {project.path}
                    </div>
                  </div>
                  {project.id === currentProjectId && (
                    <Check className="w-4 h-4 text-primary flex-shrink-0" />
                  )}
                </button>

                {/* Set as default button */}
                {!project.is_default && (
                  <button
                    onClick={(e) => handleSetDefault(e, project.id)}
                    className="p-1.5 rounded-md opacity-0 group-hover:opacity-100 hover:bg-accent transition-all"
                    title="Set as default"
                  >
                    <Star className="w-3.5 h-3.5 text-muted-foreground" />
                  </button>
                )}
              </div>
            ))
          )}

          {/* Add project hint */}
          <div className="px-3 py-2 text-xs text-muted-foreground border-t border-border mt-1">
            Use the CLI to add projects:
            <code className="ml-1 px-1 py-0.5 bg-muted rounded text-xs">quorum add .</code>
          </div>
        </div>
      )}
    </div>
  );
}
