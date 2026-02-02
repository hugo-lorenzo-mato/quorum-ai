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
  const { updateIssue, workflowId: storeWorkflowId } = useIssuesStore();

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
      const response = await workflowApi.createSingleIssue(wfId, {
        title: issue.title,
        body: issue.body,
        labels: issue.labels || [],
        assignees: issue.assignees || [],
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
  }, [issue, wfId, notifySuccess, notifyError]);

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
    <div className="flex flex-col h-full">
      {/* View Mode Toggle + Create Button */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-card shrink-0">
        <div className="flex items-center gap-2">
          {issue.task_id && (
            <span className="px-2 py-0.5 text-xs font-medium rounded bg-muted text-muted-foreground">
              {issue.task_id}
            </span>
          )}
          {/* Show created issue link */}
          {createdIssue && (
            <a
              href={createdIssue.url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-xs text-success hover:underline"
            >
              <CheckCircle2 className="w-3 h-3" />
              #{createdIssue.number}
              <ExternalLink className="w-3 h-3" />
            </a>
          )}
        </div>

        <div className="flex items-center gap-2">
          {/* Create This Issue Button */}
          <Button
            size="sm"
            variant={createdIssue ? 'secondary' : 'default'}
            onClick={handleCreateSingleIssue}
            disabled={creating || !issue.title}
            className="gap-1.5"
          >
            {creating ? (
              <>
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                Creating...
              </>
            ) : createdIssue ? (
              <>
                <CheckCircle2 className="w-3.5 h-3.5" />
                Created
              </>
            ) : (
              <>
                <Send className="w-3.5 h-3.5" />
                Create This Issue
              </>
            )}
          </Button>

          {/* View Mode Toggle */}
          <div className="flex items-center gap-1 bg-muted rounded-lg p-0.5">
            <button
              onClick={() => onToggleView?.()}
              className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition-colors ${
                isEditMode
                  ? 'bg-card text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <Edit3 className="w-4 h-4" />
              Edit
            </button>
            <button
              onClick={() => onToggleView?.()}
              className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-md transition-colors ${
                !isEditMode
                  ? 'bg-card text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
            >
              <Eye className="w-4 h-4" />
              Preview
            </button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {isEditMode ? (
          <div className="p-4 space-y-6">
            {/* Title */}
            <div className="space-y-2">
              <label className="text-sm font-medium text-foreground">
                Title
              </label>
              <Input
                value={issue.title || ''}
                onChange={handleTitleChange}
                placeholder="Issue title"
                className="text-base"
              />
            </div>

            {/* Labels and Assignees */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <LabelsEditor
                labels={issue.labels || []}
                onChange={handleLabelsChange}
              />
              <AssigneesEditor
                assignees={issue.assignees || []}
                onChange={handleAssigneesChange}
              />
            </div>

            {/* Body - Monaco Editor */}
            <div className="space-y-2">
              <label className="text-sm font-medium text-foreground">
                Body (Markdown)
              </label>
              <div className="border border-border rounded-lg overflow-hidden">
                <Editor
                  height="400px"
                  language="markdown"
                  theme={monacoTheme}
                  value={issue.body || ''}
                  onChange={handleBodyChange}
                  options={{
                    minimap: { enabled: false },
                    wordWrap: 'on',
                    lineNumbers: 'off',
                    folding: false,
                    fontSize: 14,
                    padding: { top: 16, bottom: 16 },
                    scrollBeyondLastLine: false,
                    renderLineHighlight: 'none',
                    hideCursorInOverviewRuler: true,
                    overviewRulerBorder: false,
                  }}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                Supports GitHub-flavored Markdown
              </p>
            </div>
          </div>
        ) : (
          <div className="p-4">
            {/* Preview Mode */}
            <div className="space-y-4">
              <h1 className="text-2xl font-bold text-foreground">
                {issue.title || 'Untitled Issue'}
              </h1>

              {/* Labels */}
              {issue.labels?.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {issue.labels.map((label) => (
                    <span
                      key={label}
                      className="inline-flex items-center gap-1 px-2.5 py-1 text-sm bg-primary/10 text-primary rounded-full"
                    >
                      {label}
                    </span>
                  ))}
                </div>
              )}

              {/* Assignees */}
              {issue.assignees?.length > 0 && (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <span>Assigned to:</span>
                  {issue.assignees.map((a) => `@${a}`).join(', ')}
                </div>
              )}

              {/* Body */}
              <div className="border-t border-border pt-4">
                <MarkdownViewer markdown={issue.body || '*No content*'} />
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
