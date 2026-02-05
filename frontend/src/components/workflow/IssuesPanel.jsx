import { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowApi, configApi } from '../../lib/api';
import { useUIStore } from '../../stores';
import useIssuesStore from '../../stores/issuesStore';
import {
  FileText,
  Loader2,
  Send,
  AlertCircle,
} from 'lucide-react';
import { GenerationOptionsModal } from '../issues';

export default function IssuesPanel({ workflow }) {
  const navigate = useNavigate();
  const notifyError = useUIStore((s) => s.notifyError);

  // Issues store actions
  const {
    setWorkflow,
    loadIssues,
    startGeneration,
    updateGenerationProgress,
    cancelGeneration,
  } = useIssuesStore();

  const [loading, _setLoading] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [error, setError] = useState(null);
  const [issuesEnabled, setIssuesEnabled] = useState(null);

  // Modal state
  const [showModal, setShowModal] = useState(false);

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

  // Handle generation mode selection from modal
  const handleModeSelect = useCallback(async (mode) => {
    setShowModal(false);
    setError(null);

    const enabled = await checkIssuesConfig();
    if (!enabled) {
      setError('Issue generation is disabled in settings');
      return;
    }

    // Set workflow context
    setWorkflow(workflow.id, workflow.title || workflow.id);

    if (mode === 'fast') {
      // Fast mode: instant generation, then navigate
      setPreviewLoading(true);
      try {
        const response = await workflowApi.previewIssues(workflow.id, true);
        const issues = response.preview_issues || [];

        if (issues.length > 0) {
          loadIssues(issues, {
            ai_used: response.ai_used,
            ai_errors: response.ai_errors,
          });
          navigate(`/workflows/${workflow.id}/issues`);
        } else {
          setError('No issues generated from workflow artifacts');
        }
      } catch (err) {
        setError(err.message || 'Failed to generate issues');
        notifyError(err.message || 'Failed to generate issues');
      } finally {
        setPreviewLoading(false);
      }
    } else {
      // AI mode: show loading, start generation with streaming effect
      startGeneration('ai', 10); // Estimated 10 issues
      navigate(`/workflows/${workflow.id}/issues`);

      // Generate in background with AI
      try {
        const response = await workflowApi.previewIssues(workflow.id, false);
        const issues = response.preview_issues || [];

        // Show AI status warnings if AI was not used despite being enabled
        if (response.ai_errors && response.ai_errors.length > 0) {
          console.warn('AI generation errors:', response.ai_errors);
        }

        // Simulate streaming by revealing issues progressively
        for (let i = 0; i < issues.length; i++) {
          await new Promise(resolve => setTimeout(resolve, 200)); // 200ms delay per issue
          updateGenerationProgress(i + 1, issues[i]);
        }

        // Pass AI info to store
        useIssuesStore.getState().loadIssues(issues, {
          ai_used: response.ai_used,
          ai_errors: response.ai_errors,
        });
      } catch (err) {
        cancelGeneration();
        setError(err.message || 'Failed to generate issues');
        notifyError(err.message || 'Failed to generate issues');
        navigate(`/workflows/${workflow.id}`);
      }
    }
  }, [
    workflow.id,
    workflow.title,
    checkIssuesConfig,
    setWorkflow,
    loadIssues,
    startGeneration,
    updateGenerationProgress,
    cancelGeneration,
    navigate,
    notifyError,
  ]);

  // Check if workflow can generate issues (has artifacts)
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
        {/* Create Button - Opens modal */}
        <button
          onClick={() => setShowModal(true)}
          disabled={loading || previewLoading}
          className="flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-primary text-primary-foreground text-xs font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
        >
          {loading ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            <Send className="w-3.5 h-3.5" />
          )}
          Create
        </button>
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

      {/* Generation Options Modal */}
      <GenerationOptionsModal
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        onSelect={handleModeSelect}
        loading={previewLoading || loading}
      />
    </div>
  );
}
