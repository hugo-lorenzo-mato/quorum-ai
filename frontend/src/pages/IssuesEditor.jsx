import { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useWorkflowStore } from '../stores';
import useIssuesStore from '../stores/issuesStore';
import {
  ArrowLeft,
  FileText,
  Edit3,
  Eye,
  Save,
  Download,
  Send,
  Loader2,
  AlertCircle,
  CheckCircle2,
  Tag,
  Users,
} from 'lucide-react';

// Import components (will be created)
import IssuesSidebar from '../components/issues/IssuesSidebar';
import IssueEditorPanel from '../components/issues/IssueEditorPanel';
import IssuesActionBar from '../components/issues/IssuesActionBar';
import GenerationLoadingOverlay from '../components/issues/GenerationLoadingOverlay';

/**
 * Issues Editor Page
 * Dedicated view for editing and managing generated issues before submission.
 */
export default function IssuesEditor() {
  const { id: workflowId } = useParams();
  const navigate = useNavigate();

  // Workflow store
  const { workflows, fetchWorkflows } = useWorkflowStore();
  const workflow = workflows.find(w => w.id === workflowId);

  // Issues store
  const {
    editedIssues,
    selectedIssueId,
    viewMode,
    generating,
    generationProgress,
    generationTotal,
    generatedIssues,
    submitting,
    error,
    aiUsed,
    aiErrors,
    selectIssue,
    setViewMode,
    hasUnsavedChanges,
    hasAnyUnsavedChanges,
    setWorkflow,
  } = useIssuesStore();

  // Mobile tab state
  const [mobileTab, setMobileTab] = useState('list'); // 'list' | 'editor'

  // Load workflow data if not available
  useEffect(() => {
    if (!workflow && workflowId) {
      fetchWorkflows();
    }
  }, [workflow, workflowId, fetchWorkflows]);

  // Set workflow context in store
  useEffect(() => {
    if (workflow) {
      setWorkflow(workflowId, workflow.title || workflow.id);
    }
  }, [workflow, workflowId, setWorkflow]);

  // Handle back navigation with unsaved changes warning
  const handleBack = () => {
    if (hasAnyUnsavedChanges()) {
      if (window.confirm('You have unsaved changes. Are you sure you want to leave?')) {
        navigate(`/workflows/${workflowId}`);
      }
    } else {
      navigate(`/workflows/${workflowId}`);
    }
  };

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e) => {
      // Cmd/Ctrl + S to save draft
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        // Save draft logic
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Get selected issue
  const selectedIssue = editedIssues.find(i => i._localId === selectedIssueId);

  // Show loading overlay if generating
  if (generating) {
    return (
      <GenerationLoadingOverlay
        progress={generationProgress}
        total={generationTotal}
        generatedIssues={generatedIssues}
        onCancel={() => {
          useIssuesStore.getState().cancelGeneration();
          navigate(`/workflows/${workflowId}`);
        }}
      />
    );
  }

  // If no issues loaded, show empty state or redirect
  if (!editedIssues.length && !generating) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <FileText className="w-16 h-16 text-muted-foreground/30" />
        <h2 className="text-xl font-semibold text-foreground">No Issues Generated</h2>
        <p className="text-muted-foreground text-center max-w-md">
          Generate issues from the workflow detail page first.
        </p>
        <Link
          to={`/workflows/${workflowId}`}
          className="inline-flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Workflow
        </Link>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)]">
      {/* Header */}
      <header className="flex items-center justify-between px-4 py-3 border-b border-border bg-card shrink-0">
        <div className="flex items-center gap-3">
          <button
            onClick={handleBack}
            className="p-2 rounded-lg hover:bg-muted transition-colors"
            aria-label="Back to workflow"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div>
            <h1 className="text-lg font-semibold text-foreground">Issues Editor</h1>
            <p className="text-sm text-muted-foreground">
              {workflow?.title || workflowId}
            </p>
          </div>
        </div>

        {/* Status indicators */}
        <div className="flex items-center gap-3">
          {/* AI Status */}
          {aiErrors && aiErrors.length > 0 ? (
            <span
              className="flex items-center gap-1.5 text-xs text-warning cursor-help"
              title={`AI errors: ${aiErrors.join(', ')}`}
            >
              <AlertCircle className="w-3.5 h-3.5" />
              AI fallback
            </span>
          ) : aiUsed ? (
            <span className="flex items-center gap-1.5 text-xs text-success">
              <CheckCircle2 className="w-3.5 h-3.5" />
              AI enhanced
            </span>
          ) : null}

          {/* Save status */}
          {hasAnyUnsavedChanges() ? (
            <span className="flex items-center gap-1.5 text-sm text-warning">
              <AlertCircle className="w-4 h-4" />
              Unsaved changes
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-sm text-success">
              <CheckCircle2 className="w-4 h-4" />
              Saved
            </span>
          )}
        </div>
      </header>

      {/* Mobile Tab Bar */}
      <div className="md:hidden flex border-b border-border bg-card shrink-0">
        <button
          onClick={() => setMobileTab('list')}
          className={`flex-1 py-3 text-sm font-medium text-center border-b-2 transition-colors ${
            mobileTab === 'list'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Issues ({editedIssues.length})
        </button>
        <button
          onClick={() => setMobileTab('editor')}
          className={`flex-1 py-3 text-sm font-medium text-center border-b-2 transition-colors ${
            mobileTab === 'editor'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Editor
        </button>
      </div>

      {/* Main Content */}
      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar - Hidden on mobile when editor tab is active */}
        <div className={`${
          mobileTab === 'list' ? 'flex' : 'hidden'
        } md:flex w-full md:w-80 border-r border-border bg-card shrink-0`}>
          <IssuesSidebar
            issues={editedIssues}
            selectedId={selectedIssueId}
            onSelect={(id) => {
              selectIssue(id);
              setMobileTab('editor'); // Switch to editor on mobile after selection
            }}
            hasUnsavedChanges={hasUnsavedChanges}
          />
        </div>

        {/* Editor Panel - Hidden on mobile when list tab is active */}
        <div className={`${
          mobileTab === 'editor' ? 'flex' : 'hidden'
        } md:flex flex-1 flex-col overflow-hidden`}>
          <IssueEditorPanel
            issue={selectedIssue}
            viewMode={viewMode}
            onToggleView={() => setViewMode(viewMode === 'edit' ? 'preview' : 'edit')}
            workflowId={workflowId}
          />
        </div>
      </div>

      {/* Action Bar */}
      <IssuesActionBar
        issueCount={editedIssues.length}
        hasUnsavedChanges={hasAnyUnsavedChanges()}
        submitting={submitting}
        error={error}
        workflowId={workflowId}
      />
    </div>
  );
}
