import { useState, useCallback } from 'react';
import { workflowApi, configApi } from '../../lib/api';
import { useUIStore } from '../../stores';
import {
  FileText,
  ExternalLink,
  Loader2,
  Eye,
  Send,
  CheckCircle2,
  AlertCircle,
  Tag,
  Users,
  ChevronDown,
  ChevronUp,
  RefreshCw,
} from 'lucide-react';

function IssuePreviewCard({ issue, index }) {
  const [expanded, setExpanded] = useState(index === 0);

  return (
    <div className="border border-border rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-3 bg-background hover:bg-accent/30 transition-colors text-left"
      >
        <div className="flex items-center gap-2 min-w-0">
          {issue.is_main_issue ? (
            <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-primary/10 text-primary">
              MAIN
            </span>
          ) : (
            <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-muted text-muted-foreground">
              SUB
            </span>
          )}
          <span className="text-sm font-medium text-foreground truncate">
            {issue.title}
          </span>
        </div>
        {expanded ? (
          <ChevronUp className="w-4 h-4 text-muted-foreground shrink-0" />
        ) : (
          <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
        )}
      </button>

      {expanded && (
        <div className="p-3 border-t border-border space-y-3">
          {/* Labels */}
          {issue.labels && issue.labels.length > 0 && (
            <div className="flex items-center gap-2 flex-wrap">
              <Tag className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
              {issue.labels.map((label, i) => (
                <span
                  key={i}
                  className="px-2 py-0.5 rounded-full bg-muted text-xs text-muted-foreground"
                >
                  {label}
                </span>
              ))}
            </div>
          )}

          {/* Assignees */}
          {issue.assignees && issue.assignees.length > 0 && (
            <div className="flex items-center gap-2">
              <Users className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
              <span className="text-xs text-muted-foreground">
                {issue.assignees.join(', ')}
              </span>
            </div>
          )}

          {/* Body preview */}
          <div className="text-sm text-muted-foreground">
            <p className="line-clamp-6 whitespace-pre-wrap font-mono text-xs bg-muted/50 p-2 rounded">
              {issue.body || 'No body content'}
            </p>
          </div>

          {issue.task_id && (
            <p className="text-xs text-muted-foreground">
              Task: <span className="font-mono">{issue.task_id}</span>
            </p>
          )}
        </div>
      )}
    </div>
  );
}

function CreatedIssueCard({ issue }) {
  return (
    <a
      href={issue.url}
      target="_blank"
      rel="noopener noreferrer"
      className="block p-3 border border-border rounded-lg hover:border-primary/30 hover:bg-accent/30 transition-colors"
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <CheckCircle2 className="w-4 h-4 text-success shrink-0" />
          <span className="text-sm font-medium text-foreground truncate">
            #{issue.number} {issue.title}
          </span>
        </div>
        <ExternalLink className="w-4 h-4 text-muted-foreground shrink-0" />
      </div>
      {issue.labels && issue.labels.length > 0 && (
        <div className="flex items-center gap-1.5 mt-2 flex-wrap">
          {issue.labels.map((label, i) => (
            <span
              key={i}
              className="px-2 py-0.5 rounded-full bg-muted text-[10px] text-muted-foreground"
            >
              {label}
            </span>
          ))}
        </div>
      )}
    </a>
  );
}

export default function IssuesPanel({ workflow }) {
  const notifyInfo = useUIStore((s) => s.notifyInfo);
  const notifyError = useUIStore((s) => s.notifyError);

  const [loading, setLoading] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewIssues, setPreviewIssues] = useState(null);
  const [createdIssues, setCreatedIssues] = useState(null);
  const [error, setError] = useState(null);
  const [issuesEnabled, setIssuesEnabled] = useState(null);

  // Check if issues are enabled
  const checkIssuesConfig = useCallback(async () => {
    try {
      const config = await configApi.getIssuesConfig();
      setIssuesEnabled(config.enabled);
      return config.enabled;
    } catch (err) {
      setIssuesEnabled(false);
      return false;
    }
  }, []);

  // Preview issues (dry run)
  const handlePreview = useCallback(async () => {
    setPreviewLoading(true);
    setError(null);

    try {
      const enabled = await checkIssuesConfig();
      if (!enabled) {
        setError('Issue generation is disabled in settings');
        return;
      }

      const response = await workflowApi.previewIssues(workflow.id);
      setPreviewIssues(response.preview_issues || []);
      setCreatedIssues(null);
    } catch (err) {
      setError(err.message || 'Failed to preview issues');
      notifyError(err.message || 'Failed to preview issues');
    } finally {
      setPreviewLoading(false);
    }
  }, [workflow.id, checkIssuesConfig, notifyError]);

  // Generate issues
  const handleGenerate = useCallback(async (options = {}) => {
    setLoading(true);
    setError(null);

    try {
      const enabled = await checkIssuesConfig();
      if (!enabled) {
        setError('Issue generation is disabled in settings');
        return;
      }

      const response = await workflowApi.generateIssues(workflow.id, options);

      const created = [];
      if (response.main_issue) {
        created.push(response.main_issue);
      }
      if (response.sub_issues) {
        created.push(...response.sub_issues);
      }

      setCreatedIssues(created);
      setPreviewIssues(null);
      notifyInfo(response.message || `Created ${created.length} issue(s)`);

      if (response.errors && response.errors.length > 0) {
        setError(`Warnings: ${response.errors.join(', ')}`);
      }
    } catch (err) {
      setError(err.message || 'Failed to generate issues');
      notifyError(err.message || 'Failed to generate issues');
    } finally {
      setLoading(false);
    }
  }, [workflow.id, checkIssuesConfig, notifyInfo, notifyError]);

  // Check if workflow can generate issues (has artifacts)
  const canGenerateIssues = workflow.status === 'completed' && workflow.current_phase === 'done';
  const hasAnalysis = workflow.current_phase && ['plan', 'execute', 'done'].includes(workflow.current_phase);

  if (!hasAnalysis) {
    return (
      <div className="p-4 rounded-xl border border-border bg-card">
        <div className="flex items-center gap-2 mb-2">
          <FileText className="w-4 h-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">Issue Generation</h3>
        </div>
        <p className="text-sm text-muted-foreground">
          Complete the analyze phase to generate issues from workflow artifacts.
        </p>
      </div>
    );
  }

  return (
    <div className="p-4 rounded-xl border border-border bg-card">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <FileText className="w-4 h-4 text-muted-foreground" />
          <h3 className="text-sm font-semibold text-foreground">Issue Generation</h3>
        </div>
        <div className="flex items-center gap-2">
          {/* Preview Button */}
          <button
            onClick={handlePreview}
            disabled={previewLoading || loading}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-secondary text-secondary-foreground text-xs font-medium hover:bg-secondary/80 disabled:opacity-50 transition-colors"
          >
            {previewLoading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Eye className="w-3.5 h-3.5" />
            )}
            Preview
          </button>

          {/* Generate Button */}
          <button
            onClick={() => handleGenerate()}
            disabled={loading || previewLoading || !canGenerateIssues}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
            title={!canGenerateIssues ? 'Complete workflow execution first' : 'Generate issues in GitHub/GitLab'}
          >
            {loading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Send className="w-3.5 h-3.5" />
            )}
            Generate
          </button>
        </div>
      </div>

      {/* Description */}
      <p className="text-xs text-muted-foreground mb-3">
        Generate GitHub/GitLab issues from workflow analysis and task specifications.
      </p>

      {/* Error */}
      {error && (
        <div className="flex items-start gap-2 p-2 rounded-lg bg-destructive/10 border border-destructive/20 mb-3">
          <AlertCircle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
          <p className="text-xs text-destructive">{error}</p>
        </div>
      )}

      {/* Issues Disabled Warning */}
      {issuesEnabled === false && (
        <div className="flex items-start gap-2 p-2 rounded-lg bg-warning/10 border border-warning/20 mb-3">
          <AlertCircle className="w-4 h-4 text-warning shrink-0 mt-0.5" />
          <p className="text-xs text-warning">
            Issue generation is disabled. Enable it in Settings → Issues.
          </p>
        </div>
      )}

      {/* Preview Results */}
      {previewIssues && previewIssues.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
              Preview ({previewIssues.length} issues)
            </p>
            <button
              onClick={handlePreview}
              disabled={previewLoading}
              className="p-1 rounded hover:bg-accent transition-colors"
              title="Refresh preview"
            >
              <RefreshCw className={`w-3.5 h-3.5 text-muted-foreground ${previewLoading ? 'animate-spin' : ''}`} />
            </button>
          </div>
          <div className="space-y-2 max-h-[40vh] overflow-y-auto">
            {previewIssues.map((issue, index) => (
              <IssuePreviewCard key={index} issue={issue} index={index} />
            ))}
          </div>
          <button
            onClick={() => handleGenerate()}
            disabled={loading}
            className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-lg bg-success/10 text-success text-sm font-medium hover:bg-success/20 disabled:opacity-50 transition-colors"
          >
            {loading ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Send className="w-4 h-4" />
            )}
            Create {previewIssues.length} Issue{previewIssues.length !== 1 ? 's' : ''}
          </button>
        </div>
      )}

      {/* Created Issues */}
      {createdIssues && createdIssues.length > 0 && (
        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
            Created Issues ({createdIssues.length})
          </p>
          <div className="space-y-2 max-h-[40vh] overflow-y-auto">
            {createdIssues.map((issue, index) => (
              <CreatedIssueCard key={index} issue={issue} />
            ))}
          </div>
        </div>
      )}

      {/* Empty State */}
      {!previewIssues && !createdIssues && !error && (
        <div className="text-center py-4">
          <p className="text-xs text-muted-foreground">
            Click Preview to see what issues will be created.
          </p>
        </div>
      )}
    </div>
  );
}
