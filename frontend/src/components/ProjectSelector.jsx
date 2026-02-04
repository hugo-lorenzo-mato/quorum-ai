import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, FolderKanban, Check, Star, AlertCircle, Folder, Settings } from 'lucide-react';
import { useProjectStore } from '../stores';

/**
 * Project selector dropdown for multi-project support.
 * Displays in the sidebar and allows users to switch between projects.
 */
export default function ProjectSelector({ collapsed = false, onProjectSelected }) {
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
    if (onProjectSelected) {
      onProjectSelected(projectId);
    }
  };

  const handleSetDefault = (e, projectId) => {
    e.stopPropagation();
    setDefaultProject(projectId);
  };

  const ProjectList = () => (
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
  );

  const DropdownContainer = ({ children, className = '' }) => (
    <div className={`absolute p-1 bg-popover/95 backdrop-blur-sm border border-border rounded-xl shadow-xl z-[100] animate-in fade-in zoom-in-95 duration-100 ${className}`}>
      {error && (
        <div className="px-3 py-2 text-xs text-red-500 flex items-center gap-1">
          <AlertCircle className="w-3 h-3" />
          {error}
        </div>
      )}
      {children}
      {/* Manage projects link */}
      <div className="border-t border-border/50 mt-1 pt-1">
        <Link
          to="/projects"
          onClick={() => setIsOpen(false)}
          className="w-full px-3 py-2 text-sm text-muted-foreground hover:text-foreground hover:bg-accent/50 rounded-lg flex items-center gap-2 transition-colors"
        >
          <Settings className="w-4 h-4" />
          Manage Projects
        </Link>
      </div>
    </div>
  );

  // Collapsed view - just show an icon
  if (collapsed) {
    return (
      <div className="project-selector relative">
        <button
          onClick={() => setIsOpen(!isOpen)}
          className="w-full p-2 rounded-lg hover:bg-accent/50 transition-colors flex items-center justify-center group relative"
          title={currentProject?.name || 'Select project'}
        >
          <div className={`relative flex items-center justify-center w-8 h-8 rounded-md transition-all ${isOpen ? 'bg-accent text-accent-foreground' : 'text-muted-foreground group-hover:text-foreground'}`}>
             {currentProject ? (
               <div
                 className="w-full h-full rounded-md flex items-center justify-center text-[10px] font-bold text-white uppercase"
                 style={{ backgroundColor: currentProject.color || '#4A90D9' }}
               >
                 {currentProject.name.substring(0, 2)}
               </div>
             ) : (
               <FolderKanban className="w-5 h-5" />
             )}
          </div>
        </button>

        {/* Dropdown - Pop out to right */}
        {isOpen && (
          <DropdownContainer className="left-full top-0 ml-2 w-72 origin-top-left">
            <ProjectList />
          </DropdownContainer>
        )}
      </div>
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

      {/* Dropdown - Slightly wider than parent to feel expansive */}
      {isOpen && (
        <DropdownContainer className="left-0 mt-2 w-72 origin-top-left">
          <ProjectList />
        </DropdownContainer>
      )}
    </div>
  );
}
