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
    <div className="border border-border rounded-xl overflow-hidden bg-background shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-4 hover:bg-accent/30 active:bg-accent/50 transition-colors text-left"
      >
        <div className="flex items-center gap-3 min-w-0">
          {issue.is_main_issue ? (
            <span className="px-2 py-0.5 rounded-md text-[10px] font-bold bg-primary text-primary-foreground tracking-wider uppercase">
              MAIN
            </span>
          ) : (
            <span className="px-2 py-0.5 rounded-md text-[10px] font-bold bg-muted text-muted-foreground tracking-wider uppercase">
              SUB
            </span>
          )}
          <span className="text-sm font-semibold text-foreground truncate">
            {issue.title}
          </span>
        </div>
        <div className="p-1 rounded-full bg-muted/50">
          {expanded ? (
            <ChevronUp className="w-4 h-4 text-muted-foreground shrink-0" />
          ) : (
            <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
          )}
        </div>
      </button>

      {expanded && (
        <div className="p-4 border-t border-border space-y-4 animate-fade-in bg-card/30">
          {/* Labels & Assignees */}
          <div className="flex flex-col gap-2">
            {issue.labels && issue.labels.length > 0 && (
              <div className="flex items-center gap-2 flex-wrap">
                <Tag className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <div className="flex gap-1 flex-wrap">
                  {issue.labels.map((label, i) => (
                    <span
                      key={i}
                      className="px-2 py-0.5 rounded-md bg-accent text-[11px] font-medium text-accent-foreground"
                    >
                      {label}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {issue.assignees && issue.assignees.length > 0 && (
              <div className="flex items-center gap-2">
                <Users className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                <span className="text-xs font-medium text-foreground">
                  {issue.assignees.join(', ')}
                </span>
              </div>
            )}
          </div>

          {/* Body preview */}
          <div className="relative">
            <div className="absolute top-2 right-2 text-[10px] font-mono text-muted-foreground/50 uppercase">Preview</div>
            <p className="line-clamp-10 whitespace-pre-wrap font-mono text-[13px] leading-relaxed bg-muted/50 p-3 rounded-lg border border-border/50 text-foreground/80">
              {issue.body || 'No body content'}
            </p>
          </div>

          {issue.task_id && (
            <div className="flex items-center gap-2 pt-2 border-t border-border/30">
              <span className="text-[10px] text-muted-foreground uppercase font-bold tracking-tight">Task ID:</span>
              <span className="text-[11px] font-mono bg-primary/5 text-primary px-1.5 rounded">{issue.task_id}</span>
            </div>
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
      className="block p-4 border border-border rounded-xl bg-background hover:border-primary/30 hover:bg-accent/30 active:scale-[0.98] transition-all shadow-sm group"
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className="p-1.5 rounded-full bg-success/10 text-success">
            <CheckCircle2 className="w-4 h-4" />
          </div>
          <span className="text-sm font-semibold text-foreground truncate group-hover:text-primary transition-colors">
            <span className="text-muted-foreground font-mono mr-1">#{issue.number}</span>
            {issue.title}
          </span>
        </div>
        <ExternalLink className="w-4 h-4 text-muted-foreground shrink-0 group-hover:text-primary transition-colors" />
      </div>
      {issue.labels && issue.labels.length > 0 && (
        <div className="flex items-center gap-1.5 mt-3 flex-wrap pl-10">
          {issue.labels.map((label, i) => (
            <span
              key={i}
              className="px-2 py-0.5 rounded-md bg-muted text-[10px] font-medium text-muted-foreground"
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
    } catch {
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
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
        <div className="flex items-center gap-2">
          <div className="p-2 rounded-lg bg-primary/10">
            <FileText className="w-4 h-4 text-primary" />
          </div>
          <h3 className="text-sm font-semibold text-foreground tracking-tight">Issue Generation</h3>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          {/* Preview Button */}
          <button
            onClick={handlePreview}
            disabled={previewLoading || loading}
            className="flex-1 sm:flex-none flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-secondary text-secondary-foreground text-xs font-medium hover:bg-secondary/80 disabled:opacity-50 transition-colors"
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
            className="flex-1 sm:flex-none flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
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
          <div className="space-y-3 max-h-[50vh] overflow-y-auto pr-1 scrollbar-thin">
            {previewIssues.map((issue, index) => (
              <IssuePreviewCard key={index} issue={issue} index={index} />
            ))}
          </div>
          <button
            onClick={() => handleGenerate()}
            disabled={loading}
            className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl bg-primary text-primary-foreground text-sm font-bold shadow-lg shadow-primary/20 hover:bg-primary/90 active:scale-[0.98] transition-all mt-2"
          >
            {loading ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : (
              <Send className="w-5 h-5" />
            )}
            Create {previewIssues.length} Issue{previewIssues.length !== 1 ? 's' : ''} Now
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
