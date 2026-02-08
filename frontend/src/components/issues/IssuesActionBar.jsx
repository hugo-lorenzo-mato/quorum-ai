import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Download, Save, Send, Loader2, AlertCircle } from 'lucide-react';
import { Button } from '../ui/Button';
import { workflowApi } from '../../lib/api';
import useIssuesStore from '../../stores/issuesStore';
import { useUIStore } from '../../stores';

/**
 * Bottom action bar with Create Issues / Save Draft / Export buttons.
 */
export default function IssuesActionBar({
  issueCount = 0,
  hasUnsavedChanges = false,
  submitting = false,
  error = null,
  workflowId,
}) {
  const navigate = useNavigate();
  const { notifySuccess, notifyError } = useUIStore();
  const {
    getIssuesForSubmission,
    applySavedIssues,
    setSubmitting,
    setError,
    clearError,
    reset,
    publishingProgress,
    publishingTotal,
  } = useIssuesStore();

  const [exporting, setExporting] = useState(false);

  // Handle issue submission to GitHub/GitLab
  const handleCreateIssues = async () => {
    try {
      setSubmitting(true);
      clearError();

      const issues = getIssuesForSubmission();

      const saveResponse = await workflowApi.saveIssuesFiles(workflowId, issues);
      if (!saveResponse.success) {
        throw new Error(saveResponse.message || 'Failed to save issue files');
      }
      const issuesToSubmit = saveResponse.issues || issues;
      applySavedIssues(issuesToSubmit);

      const response = await workflowApi.generateIssues(workflowId, {
        dryRun: false,
        linkIssues: true,
        issues: issuesToSubmit,
      });

      if (response.success) {
        const mainCount = response.main_issue ? 1 : 0;
        const subCount = response.sub_issues?.length || 0;
        notifySuccess(`Created ${mainCount + subCount} issue${mainCount + subCount !== 1 ? 's' : ''} successfully`);
        reset();
        navigate(`/workflows/${workflowId}`);
      } else {
        throw new Error(response.message || 'Failed to create issues');
      }
    } catch (err) {
      console.error('Failed to create issues:', err);
      setError(err.message || 'Failed to create issues');
      notifyError(err.message || 'Failed to create issues');
    } finally {
      setSubmitting(false);
    }
  };

  // Handle save draft (just persists to localStorage via store)
  const handleSaveDraft = () => {
    const saveDraft = async () => {
      try {
        const issues = getIssuesForSubmission();
        const response = await workflowApi.saveIssuesFiles(workflowId, issues);
        if (!response.success) {
          throw new Error(response.message || 'Failed to save issue files');
        }
        if (response.issues) {
          applySavedIssues(response.issues);
        }
        notifySuccess('Draft saved');
      } catch (err) {
        console.error('Failed to save draft:', err);
        notifyError(err.message || 'Failed to save draft');
      }
    };

    saveDraft();
  };

  // Handle export as markdown files
  const handleExport = async () => {
    try {
      setExporting(true);
      const issues = getIssuesForSubmission();

      // Create markdown content
      const mainIssue = issues.find(i => i.is_main_issue);
      const subIssues = issues.filter(i => !i.is_main_issue);

      let content = '';

      if (mainIssue) {
        content += `# ${mainIssue.title}\n\n`;
        if (mainIssue.labels?.length) {
          content += `**Labels:** ${mainIssue.labels.join(', ')}\n`;
        }
        if (mainIssue.assignees?.length) {
          content += `**Assignees:** ${mainIssue.assignees.map(a => `@${a}`).join(', ')}\n`;
        }
        content += `\n${mainIssue.body}\n\n---\n\n`;
      }

      subIssues.forEach((issue, i) => {
        content += `## ${i + 1}. ${issue.title}\n\n`;
        if (issue.task_id) {
          content += `**Task:** ${issue.task_id}\n`;
        }
        if (issue.labels?.length) {
          content += `**Labels:** ${issue.labels.join(', ')}\n`;
        }
        if (issue.assignees?.length) {
          content += `**Assignees:** ${issue.assignees.map(a => `@${a}`).join(', ')}\n`;
        }
        content += `\n${issue.body}\n\n---\n\n`;
      });

      // Create download
      const blob = new Blob([content], { type: 'text/markdown' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `issues-${workflowId}.md`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      notifySuccess('Issues exported as Markdown');
    } catch (err) {
      console.error('Export failed:', err);
      notifyError('Failed to export issues');
    } finally {
      setExporting(false);
    }
  };

  return (
    <footer className="flex items-center justify-between px-4 py-3 border-t border-border bg-card shrink-0">
      {/* Left side - status */}
      <div className="flex items-center gap-3">
        {error ? (
          <span className="flex items-center gap-1.5 text-sm text-destructive">
            <AlertCircle className="w-4 h-4" />
            {error}
          </span>
        ) : (
          <span className="text-sm text-muted-foreground">
            {issueCount} issue{issueCount !== 1 ? 's' : ''} ready
            {hasUnsavedChanges && ' (modified)'}
          </span>
        )}
      </div>

      {/* Right side - actions */}
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleExport}
          disabled={exporting || submitting || issueCount === 0}
          className="gap-2"
        >
          {exporting ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Download className="w-4 h-4" />
          )}
          Export
        </Button>

        <Button
          variant="secondary"
          size="sm"
          onClick={handleSaveDraft}
          disabled={submitting || !hasUnsavedChanges}
          className="gap-2"
        >
          <Save className="w-4 h-4" />
          Save Draft
        </Button>

        <Button
          variant="default"
          size="sm"
          onClick={handleCreateIssues}
          disabled={submitting || issueCount === 0}
          className="gap-2"
        >
          {submitting ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              Creating{publishingTotal > 0 ? ` (${publishingProgress}/${publishingTotal})` : '...'}
            </>
          ) : (
            <>
              <Send className="w-4 h-4" />
              Create Issues
            </>
          )}
        </Button>
      </div>
    </footer>
  );
}
