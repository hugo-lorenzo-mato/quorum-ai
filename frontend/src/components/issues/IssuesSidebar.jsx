import { FileText, Plus, Filter } from 'lucide-react';

/**
 * Sidebar listing all issues (main + sub).
 * Shows title, type badge, and unsaved indicator.
 */
export default function IssuesSidebar({
  issues = [],
  selectedId,
  onSelect,
  onCreate,
  hasUnsavedChanges,
}) {
  return (
    <div className="flex flex-col h-full w-full bg-card/30 border-r border-border">
      {/* Header */}
      <div className="px-4 py-4 border-b border-border shrink-0 flex items-center justify-between bg-card/50 backdrop-blur-sm">
        <div className="flex items-center gap-2.5">
          <h2 className="text-sm font-semibold text-foreground tracking-wide uppercase">
            Issues
          </h2>
          <span className="flex items-center justify-center min-w-[20px] h-5 px-1.5 text-xs font-medium rounded-full bg-secondary text-secondary-foreground">
            {issues.length}
          </span>
        </div>
        
        <div className="flex items-center gap-1">
           {/* Filter button (placeholder for future feature) */}
           <button className="p-2 rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors touch-manipulation">
              <Filter className="w-4 h-4" />
           </button>
           
           {onCreate && (
            <button
              onClick={onCreate}
              className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium bg-primary text-primary-foreground rounded-md shadow-sm hover:bg-primary/90 transition-all active:scale-95 touch-manipulation"
              title="Create New Issue"
            >
              <Plus className="w-3.5 h-3.5" />
              New
            </button>
          )}
        </div>
      </div>

      {/* Issue list */}
      <div className="flex-1 overflow-y-auto p-3 custom-scrollbar">
        <div className="space-y-2">
          {issues.map((issue) => {
            const isSelected = issue._localId === selectedId;
            const isModified = hasUnsavedChanges?.(issue._localId);

            return (
              <button
                key={issue._localId}
                onClick={() => onSelect(issue._localId)}
                className={`group w-full p-3 sm:p-4 rounded-xl text-left transition-all border relative overflow-hidden touch-manipulation active:scale-[0.99] ${
                  isSelected
                    ? 'bg-background border-primary/50 shadow-sm ring-1 ring-primary/20'
                    : 'bg-card border-transparent hover:bg-background hover:border-border/50 hover:shadow-sm'
                }`}
              >
                {/* Selection Indicator Bar */}
                {isSelected && (
                   <div className="absolute left-0 top-0 bottom-0 w-1 bg-primary" />
                )}

                <div className="flex items-start gap-3 pl-1">
                  {/* Icon */}
                  <div className={`mt-0.5 p-1.5 rounded-lg shrink-0 ${
                    isSelected ? 'bg-primary/10 text-primary' : 'bg-secondary text-muted-foreground group-hover:text-foreground'
                  }`}>
                    <FileText className="w-4 h-4" />
                  </div>

                  <div className="flex-1 min-w-0 py-0.5">
                    {/* Type badge + Task ID + modified indicator */}
                    <div className="flex items-center justify-between mb-1.5">
                      <div className="flex items-center gap-1.5">
                        <span className={`px-1.5 py-0.5 text-[10px] font-bold rounded ${
                          issue.is_main_issue
                            ? 'bg-primary/15 text-primary'
                            : 'bg-secondary/80 text-secondary-foreground'
                        }`}>
                          {issue.is_main_issue ? 'MAIN' : 'SUB'}
                        </span>
                        {issue.task_id && (
                          <span className="px-1.5 py-0.5 text-[10px] font-bold rounded bg-secondary/80 text-secondary-foreground font-mono tracking-tight">
                            {issue.task_id}
                          </span>
                        )}
                      </div>

                      {isModified && (
                        <span className="flex h-2 w-2 relative" title="Unsaved changes">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-warning opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-warning"></span>
                        </span>
                      )}
                    </div>

                    {/* Title */}
                    <p className={`text-sm font-medium line-clamp-2 leading-relaxed ${
                      isSelected ? 'text-foreground' : 'text-muted-foreground group-hover:text-foreground'
                    }`}>
                      {issue.title || 'Untitled Issue'}
                    </p>
                  </div>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
