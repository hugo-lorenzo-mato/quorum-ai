import { FileText, Circle } from 'lucide-react';

/**
 * Sidebar listing all issues (main + sub).
 * Shows title, type badge, and unsaved indicator.
 */
export default function IssuesSidebar({
  issues = [],
  selectedId,
  onSelect,
  hasUnsavedChanges,
}) {
  return (
    <div className="flex flex-col h-full w-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-border shrink-0">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-foreground">
            Issues
          </h2>
          <span className="text-xs text-muted-foreground bg-muted px-2 py-0.5 rounded-full">
            {issues.length}
          </span>
        </div>
      </div>

      {/* Issue list */}
      <div className="flex-1 overflow-y-auto p-2">
        <div className="space-y-1">
          {issues.map((issue) => {
            const isSelected = issue._localId === selectedId;
            const isModified = hasUnsavedChanges?.(issue._localId);

            return (
              <button
                key={issue._localId}
                onClick={() => onSelect(issue._localId)}
                className={`w-full p-3 rounded-lg text-left transition-all ${
                  isSelected
                    ? 'bg-primary/10 border border-primary/30'
                    : 'hover:bg-muted border border-transparent'
                }`}
              >
                <div className="flex items-start gap-2">
                  {/* Icon */}
                  <FileText className={`w-4 h-4 mt-0.5 shrink-0 ${
                    isSelected ? 'text-primary' : 'text-muted-foreground'
                  }`} />

                  <div className="flex-1 min-w-0">
                    {/* Task ID + modified indicator */}
                    <div className="flex items-center gap-2 mb-1">
                      {issue.task_id && (
                        <span className="px-1.5 py-0.5 text-[10px] font-medium rounded bg-muted text-muted-foreground">
                          {issue.task_id}
                        </span>
                      )}

                      {isModified && (
                        <Circle className="w-2 h-2 fill-warning text-warning" />
                      )}
                    </div>

                    {/* Title */}
                    <p className={`text-sm font-medium line-clamp-2 ${
                      isSelected ? 'text-foreground' : 'text-foreground/80'
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
