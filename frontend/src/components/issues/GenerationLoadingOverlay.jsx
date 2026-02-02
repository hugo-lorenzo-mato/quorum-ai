import { useMemo } from 'react';
import { FileText, Tag, Users, X, Loader2 } from 'lucide-react';
import { Button } from '../ui/Button';

/**
 * Skeleton component for loading states
 */
function Skeleton({ className = '' }) {
  return (
    <div className={`animate-pulse bg-muted/50 rounded ${className}`} />
  );
}

/**
 * Skeleton issue item in sidebar
 */
function SkeletonIssueItem({ filled = false, issue = null }) {
  if (filled && issue) {
    return (
      <div className="p-3 rounded-lg border border-border bg-card animate-fade-in">
        <div className="flex items-center gap-2 mb-1">
          <span className={`px-1.5 py-0.5 text-[10px] font-semibold rounded ${
            issue.is_main_issue
              ? 'bg-primary/10 text-primary'
              : 'bg-muted text-muted-foreground'
          }`}>
            {issue.is_main_issue ? 'MAIN' : 'SUB'}
          </span>
        </div>
        <p className="text-sm font-medium text-foreground line-clamp-2">
          {issue.title}
        </p>
      </div>
    );
  }

  return (
    <div className="p-3 rounded-lg border border-border/50 bg-card/50">
      <div className="flex items-center gap-2 mb-2">
        <Skeleton className="w-10 h-4" />
      </div>
      <Skeleton className="h-4 w-3/4 mb-1" />
      <Skeleton className="h-4 w-1/2" />
    </div>
  );
}

/**
 * Skeleton preview panel
 */
function SkeletonPreview({ issue = null }) {
  if (issue) {
    return (
      <div className="p-6 animate-fade-in">
        <h2 className="text-xl font-semibold text-foreground mb-4">
          {issue.title}
        </h2>

        <div className="flex flex-wrap gap-2 mb-4">
          {(issue.labels || []).map((label, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium bg-primary/10 text-primary rounded-full"
            >
              <Tag className="w-3 h-3" />
              {label}
            </span>
          ))}
        </div>

        {issue.assignees?.length > 0 && (
          <div className="flex items-center gap-2 mb-4">
            <Users className="w-4 h-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">
              {issue.assignees.join(', ')}
            </span>
          </div>
        )}

        <div className="prose prose-sm max-w-none text-foreground">
          <p className="text-muted-foreground line-clamp-6">
            {issue.body?.substring(0, 500)}...
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      <Skeleton className="h-7 w-2/3 mb-4" />

      <div className="flex gap-2 mb-4">
        <Skeleton className="h-6 w-16 rounded-full" />
        <Skeleton className="h-6 w-20 rounded-full" />
        <Skeleton className="h-6 w-14 rounded-full" />
      </div>

      <div className="flex items-center gap-2 mb-6">
        <Skeleton className="h-4 w-4 rounded-full" />
        <Skeleton className="h-4 w-32" />
      </div>

      <div className="space-y-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
        <Skeleton className="h-4 w-3/4" />
      </div>
    </div>
  );
}

/**
 * Full-screen loading overlay with skeleton + streaming effect.
 * Shows a preview of the editor layout with skeletons that fill in as issues are generated.
 */
export default function GenerationLoadingOverlay({
  progress = 0,
  total = 0,
  generatedIssues = [],
  onCancel,
}) {
  // Create placeholder slots for issues
  const issueSlots = useMemo(() => {
    const slots = [];
    const generated = generatedIssues || [];

    // Add all generated issues
    for (let i = 0; i < generated.length; i++) {
      slots.push({ filled: true, issue: generated[i] });
    }

    // Add remaining skeleton slots
    const remaining = Math.max(0, (total || 10) - generated.length);
    for (let i = 0; i < remaining; i++) {
      slots.push({ filled: false, issue: null });
    }

    return slots;
  }, [generatedIssues, total]);

  // Currently previewing the last generated issue
  const previewIssue = generatedIssues.length > 0
    ? generatedIssues[generatedIssues.length - 1]
    : null;

  return (
    <div className="fixed inset-0 z-50 bg-background/95 backdrop-blur-sm animate-fade-in">
      {/* Layout mimics editor */}
      <div className="flex flex-col h-full">
        {/* Header */}
        <header className="flex items-center justify-between px-4 py-3 border-b border-border bg-card shrink-0">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-primary/10">
              <Loader2 className="w-5 h-5 text-primary animate-spin" />
            </div>
            <div>
              <h1 className="text-lg font-semibold text-foreground">
                Generating Issues
              </h1>
              <p className="text-sm text-muted-foreground">
                {progress} of {total || '?'} issues generated
              </p>
            </div>
          </div>

          <Button
            variant="ghost"
            size="sm"
            onClick={onCancel}
            className="gap-2"
          >
            <X className="w-4 h-4" />
            Cancel
          </Button>
        </header>

        {/* Main content */}
        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar with issue skeletons */}
          <div className="w-80 border-r border-border bg-card p-4 overflow-y-auto">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-foreground">
                Issues
              </h2>
              <span className="text-xs text-muted-foreground">
                {generatedIssues.length} / {total || '?'}
              </span>
            </div>

            <div className="space-y-2">
              {issueSlots.map((slot, i) => (
                <SkeletonIssueItem
                  key={i}
                  filled={slot.filled}
                  issue={slot.issue}
                />
              ))}
            </div>
          </div>

          {/* Preview panel */}
          <div className="flex-1 overflow-y-auto bg-background">
            <SkeletonPreview issue={previewIssue} />
          </div>
        </div>

        {/* Progress footer */}
        <footer className="px-4 py-3 border-t border-border bg-card shrink-0">
          <div className="flex items-center justify-between gap-4">
            <div className="flex-1">
              {/* Progress bar */}
              <div className="h-2 bg-muted rounded-full overflow-hidden">
                <div
                  className="h-full bg-primary rounded-full transition-all duration-500 ease-out"
                  style={{
                    width: total > 0 ? `${(progress / total) * 100}%` : '10%'
                  }}
                />
              </div>
            </div>

            <span className="text-sm text-muted-foreground whitespace-nowrap">
              {total > 0
                ? `${Math.round((progress / total) * 100)}% complete`
                : 'Analyzing workflow...'
              }
            </span>
          </div>
        </footer>
      </div>
    </div>
  );
}
