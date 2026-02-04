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
        className="w-full px-3 py-2 rounded-lg border border-transparent hover:border-border hover:bg-accent/50 transition-all duration-200 flex items-center gap-3 group"
      >
        {currentProject ? (
          <>
            <div
              className="w-8 h-8 rounded-md flex items-center justify-center shadow-sm transition-transform group-hover:scale-105"
              style={{ backgroundColor: currentProject.color || '#4A90D9' }}
            >
              <span className="text-xs font-bold text-white uppercase">
                {currentProject.name.substring(0, 2)}
              </span>
            </div>
            <div className="flex-1 min-w-0 text-left">
              <div className="text-sm font-medium truncate text-foreground/90 group-hover:text-foreground">
                {currentProject.name}
              </div>
              <div className="text-[10px] text-muted-foreground truncate opacity-70 group-hover:opacity-100">
                {currentProject.path}
              </div>
            </div>
          </>
        ) : (
          <>
            <div className="w-8 h-8 rounded-md bg-muted flex items-center justify-center">
              <Folder className="w-4 h-4 text-muted-foreground" />
            </div>
            <span className="text-sm text-muted-foreground">Select project...</span>
          </>
        )}
        <ChevronDown
          className={`w-4 h-4 text-muted-foreground/50 transition-transform duration-200 group-hover:text-foreground ${
            isOpen ? 'rotate-180' : ''
          }`}
        />
      </button>

      {/* Dropdown */}
      {isOpen && (
        <div className="absolute left-0 right-0 mt-2 p-1 bg-popover/95 backdrop-blur-sm border border-border rounded-xl shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
          {error && (
            <div className="px-3 py-2 text-xs text-red-500 flex items-center gap-1">
              <AlertCircle className="w-3 h-3" />
              {error}
            </div>
          )}

          <div className="max-h-[280px] overflow-y-auto custom-scrollbar">
            {projects.length === 0 && !loading ? (
              <div className="px-3 py-4 text-sm text-muted-foreground text-center">
                No projects registered
              </div>
            ) : (
              projects.map((project) => (
                <div
                  key={project.id}
                  className="group/item flex items-center gap-1 px-1 mb-0.5 last:mb-0"
                >
                  <button
                    onClick={() => handleSelectProject(project.id)}
                    className={`flex-1 px-2 py-2 flex items-center gap-3 rounded-lg transition-colors text-left ${
                      project.id === currentProjectId
                        ? 'bg-accent/80 text-accent-foreground'
                        : 'hover:bg-accent/50 text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    <div
                      className="w-2 h-2 rounded-full flex-shrink-0"
                      style={{ backgroundColor: project.color || '#4A90D9' }}
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1">
                        <span className="text-sm font-medium truncate">{project.name}</span>
                        {project.is_default && (
                          <Star className="w-3 h-3 text-yellow-500 fill-yellow-500" />
                        )}
                      </div>
                      <div className="text-[10px] opacity-70 truncate">
                        {project.path}
                      </div>
                    </div>
                    {project.id === currentProjectId && (
                      <Check className="w-3.5 h-3.5 text-primary flex-shrink-0" />
                    )}
                  </button>

                  {/* Set as default button */}
                  {!project.is_default && (
                    <button
                      onClick={(e) => handleSetDefault(e, project.id)}
                      className="p-1.5 rounded-md opacity-0 group-hover/item:opacity-100 hover:bg-accent transition-all text-muted-foreground hover:text-yellow-500"
                      title="Set as default"
                    >
                      <Star className="w-3.5 h-3.5" />
                    </button>
                  )}
                </div>
              ))
            )}
          </div>

          {/* Add project hint */}
          <div className="px-3 py-2 text-[10px] text-muted-foreground border-t border-border/50 mt-1 bg-muted/20 rounded-b-lg">
            Use CLI to add: <code className="font-mono text-primary">quorum add .</code>
          </div>
        </div>
      )}
    </div>
  );
}
