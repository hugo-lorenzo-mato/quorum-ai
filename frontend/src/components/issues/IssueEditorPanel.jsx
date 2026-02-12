import { useState, useCallback } from 'react';
import { Edit3, Eye, FileText, Send, Loader2, CheckCircle2, ExternalLink } from 'lucide-react';
import Editor from '@monaco-editor/react';
import { useUIStore } from '../../stores';
import useIssuesStore from '../../stores/issuesStore';
import { workflowApi } from '../../lib/api';
import MarkdownViewer from '../MarkdownViewer';
import LabelsEditor from './LabelsEditor';
import AssigneesEditor from './AssigneesEditor';
import { Input } from '../ui/Input';
import { Button } from '../ui/Button';

/**
 * Main editing panel for a single issue.
 * Includes title, labels, assignees, and body (Monaco editor or preview).
 */
export default function IssueEditorPanel({
  issue,
  viewMode = 'edit',
  onToggleView,
  workflowId,
}) {
  const { theme, notifySuccess, notifyError } = useUIStore();
  const { updateIssue, setIssueFilePath, workflowId: storeWorkflowId } = useIssuesStore();

  // State for creating single issue
  const [creating, setCreating] = useState(false);
  const [createdIssue, setCreatedIssue] = useState(null);

  // Use workflowId from props or store
  const wfId = workflowId || storeWorkflowId;

  // Handle creating this single issue
  const handleCreateSingleIssue = useCallback(async () => {
    if (!issue || !wfId) return;

    try {
      setCreating(true);
      const saveResponse = await workflowApi.saveIssuesFiles(wfId, [{
        title: issue.title,
        body: issue.body,
        labels: issue.labels || [],
        assignees: issue.assignees || [],
        is_main_issue: issue.is_main_issue || false,
        task_id: issue.task_id || null,
        file_path: issue.file_path || null,
      }]);
      if (!saveResponse.success) {
        throw new Error(saveResponse.message || 'Failed to save issue file');
      }

      const savedIssue = saveResponse.issues?.[0];
      if (savedIssue?.file_path) {
        setIssueFilePath(issue._localId, savedIssue.file_path);
      }

      const response = await workflowApi.createSingleIssue(wfId, {
        title: issue.title,
        body: issue.body,
        labels: issue.labels || [],
        assignees: issue.assignees || [],
        is_main_issue: issue.is_main_issue || false,
        task_id: issue.task_id || null,
        file_path: savedIssue?.file_path || issue.file_path || null,
      });

      if (response.success) {
        setCreatedIssue(response.issue);
        notifySuccess(`Issue #${response.issue.number} created successfully`);
      } else {
        throw new Error(response.error || 'Failed to create issue');
      }
    } catch (err) {
      console.error('Failed to create issue:', err);
      notifyError(err.message || 'Failed to create issue');
    } finally {
      setCreating(false);
    }
  }, [issue, wfId, setIssueFilePath, notifySuccess, notifyError]);

  // Handle field updates
  const handleTitleChange = useCallback((e) => {
    if (issue) {
      updateIssue(issue._localId, { title: e.target.value });
    }
  }, [issue, updateIssue]);

  const handleBodyChange = useCallback((value) => {
    if (issue) {
      updateIssue(issue._localId, { body: value || '' });
    }
  }, [issue, updateIssue]);

  const handleLabelsChange = useCallback((labels) => {
    if (issue) {
      updateIssue(issue._localId, { labels });
    }
  }, [issue, updateIssue]);

  const handleAssigneesChange = useCallback((assignees) => {
    if (issue) {
      updateIssue(issue._localId, { assignees });
    }
  }, [issue, updateIssue]);

  // Empty state
  if (!issue) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4 text-muted-foreground">
        <FileText className="w-16 h-16 opacity-30" />
        <p>Select an issue to edit</p>
      </div>
    );
  }

  const isEditMode = viewMode === 'edit';
  const monacoTheme = theme === 'light' ? 'light' : 'vs-dark';

  return (
    <div className="flex flex-col h-full w-full min-h-0 bg-background overflow-hidden">
      {/* Header Toolbar - Fixed height */}
      <div className="flex-none flex items-center justify-between px-4 sm:px-6 py-3 border-b border-border bg-card/50 backdrop-blur-sm z-10">
        <div className="flex items-center gap-3 overflow-hidden">
          {issue.task_id && (
            <span className="shrink-0 px-2 py-0.5 text-xs font-medium rounded-md bg-secondary text-secondary-foreground">
              {issue.task_id}
            </span>
          )}
          {/* Show created issue link */}
          {createdIssue && (
            <a
              href={createdIssue.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-xs text-success hover:underline font-medium bg-success/10 px-2 py-0.5 rounded-full truncate"
            >
              <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">#{createdIssue.number}</span>
              <ExternalLink className="w-3 h-3 shrink-0" />
            </a>
          )}
        </div>

        <div className="flex items-center gap-2 sm:gap-3 shrink-0">
          {/* View Mode Toggle */}
          <div className="flex items-center gap-1 bg-secondary rounded-lg p-1">
            <button
              onClick={() => onToggleView?.()}
              className={`flex items-center gap-1.5 px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium rounded-md transition-all ${
                isEditMode
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <Edit3 className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Edit</span>
            </button>
            <button
              onClick={() => onToggleView?.()}
              className={`flex items-center gap-1.5 px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium rounded-md transition-all ${
                !isEditMode
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <Eye className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Preview</span>
            </button>
          </div>

          <div className="h-6 w-px bg-border hidden sm:block" />

          {/* Create This Issue Button */}
          <Button
            size="sm"
            variant={createdIssue ? 'outline' : 'default'}
            onClick={handleCreateSingleIssue}
            disabled={creating || !issue.title}
            className={`gap-2 shadow-sm ${createdIssue ? 'border-success/50 text-success hover:bg-success/5 hover:text-success' : ''}`}
          >
            {creating ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span className="hidden sm:inline">Publishing...</span>
              </>
            ) : createdIssue ? (
              <>
                <CheckCircle2 className="w-4 h-4" />
                <span className="hidden sm:inline">Published</span>
              </>
            ) : (
              <>
                <Send className="w-4 h-4" />
                <span className="hidden sm:inline">Publish</span>
                <span className="sm:hidden">Send</span>
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Content Area - Flex Grow with Overflow */}
      <div className="flex-1 overflow-hidden flex flex-col min-h-0 w-full">
        {isEditMode ? (
          <div className="flex-1 flex flex-col min-h-0 h-full">
            {/* Meta Data Section - Ultra Compact */}
            <div className="flex-none px-6 py-4 border-b border-border bg-background z-10 space-y-3">
              {/* Title - Clean & Prominent */}
              <Input
                value={issue.title || ''}
                onChange={handleTitleChange}
                placeholder="Issue title..."
                className="text-xl sm:text-2xl font-bold border-none shadow-none bg-transparent p-0 focus-visible:ring-0 placeholder:opacity-20 h-auto"
              />

              {/* Compact Meta Row */}
              <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
                <div className="flex-none">
                  <LabelsEditor
                    labels={issue.labels || []}
                    onChange={handleLabelsChange}
                    compact
                  />
                </div>
                <div className="hidden sm:block w-px h-4 bg-border/60" />
                <div className="flex-none">
                  <AssigneesEditor
                    assignees={issue.assignees || []}
                    onChange={handleAssigneesChange}
                    compact
                  />
                </div>
              </div>
            </div>

            {/* Editor Section */}
            <div className="flex-1 flex flex-col min-h-0 relative">
              <div className="px-6 py-1 flex items-center justify-between bg-muted/10 border-b border-border/40 shrink-0">
                 <span className="text-[9px] font-bold text-muted-foreground/60 uppercase tracking-widest">
                    Markdown Description
                 </span>
                 <span className="text-[9px] text-muted-foreground/40 italic">
                    GFM Enabled
                 </span>
              </div>
              <div className="flex-1 relative min-h-0">
                <Editor
                  height="100%"
                  language="markdown"
                  theme={monacoTheme}
                  value={issue.body || ''}
                  onChange={handleBodyChange}
                  loading={<div className="flex items-center justify-center h-full text-muted-foreground"><Loader2 className="w-6 h-6 animate-spin" /></div>}
                  options={{
                    minimap: { enabled: false },
                    wordWrap: 'on',
                    lineNumbers: 'off',
                    folding: false,
                    fontSize: 14,
                    fontFamily: "'JetBrains Mono', monospace",
                    padding: { top: 24, bottom: 24 },
                    scrollBeyondLastLine: false,
                    renderLineHighlight: 'none',
                    hideCursorInOverviewRuler: true,
                    overviewRulerBorder: false,
                    scrollbar: {
                      vertical: 'visible',
                      horizontal: 'hidden',
                      useShadows: false,
                      verticalScrollbarSize: 10
                    },
                    automaticLayout: true
                  }}
                />
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 overflow-y-auto p-4 sm:p-8 scroll-smooth pb-20">
            {/* Preview Mode */}
            <div className="max-w-4xl mx-auto space-y-6">
              <div className="pb-6 border-b border-border space-y-4">
                <h1 className="text-2xl sm:text-3xl font-bold text-foreground leading-tight">
                  {issue.title || 'Untitled Issue'}
                </h1>

                <div className="flex flex-wrap items-center gap-3">
                  {/* Labels */}
                  {issue.labels?.length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {issue.labels.map((label) => (
                        <span
                          key={label}
                          className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary/10 text-primary ring-1 ring-inset ring-primary/20"
                        >
                          {label}
                        </span>
                      ))}
                    </div>
                  )}
                  
                  {/* Assignees */}
                  {issue.assignees?.length > 0 && (
                     <div className="flex items-center -space-x-2">
                        {issue.assignees.map((a) => (
                           <div key={a} className="w-6 h-6 rounded-full bg-secondary ring-2 ring-background flex items-center justify-center text-[10px] font-bold uppercase ring-offset-2 ring-offset-background" title={a}>
                              {a.substring(0,2)}
                           </div>
                        ))}
                     </div>
                  )}
                </div>
              </div>

              {/* Body */}
              <div className="prose prose-sm dark:prose-invert max-w-none break-words">
                <MarkdownViewer markdown={issue.body || '*No content*'} />
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
