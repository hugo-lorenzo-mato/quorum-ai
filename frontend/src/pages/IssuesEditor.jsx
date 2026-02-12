import { useEffect, useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useWorkflowStore, useIssuesStore } from '../stores';
import {
  ArrowLeft,
  FileText,
  AlertCircle,
  CheckCircle2,
  Plus,
} from 'lucide-react';
import FAB from '../components/FAB';

// Import components
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
  const [hasAttemptedFetch, setHasAttemptedFetch] = useState(false);

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
    createIssue,
  } = useIssuesStore();

  // Mobile tab state
  const [mobileTab, setMobileTab] = useState('list'); // 'list' | 'editor'

  // Load workflow data if not available
  useEffect(() => {
    if (!workflow && workflowId && !hasAttemptedFetch) {
      fetchWorkflows().finally(() => setHasAttemptedFetch(true));
    }
  }, [workflow, workflowId, fetchWorkflows, hasAttemptedFetch]);

  // Set workflow context in store
  useEffect(() => {
    if (workflow) {
      setWorkflow(workflowId, workflow.title || workflow.id);
    }
  }, [workflow, workflowId, setWorkflow]);

  // Handle back navigation with unsaved changes warning
  const handleBack = () => {
    // Mobile: back to list if in editor
    if (window.innerWidth < 768 && mobileTab === 'editor') {
      setMobileTab('list');
      return;
    }

    if (hasAnyUnsavedChanges()) {
      if (window.confirm('You have unsaved changes. Are you sure you want to leave?')) {
        navigate(`/workflows/${workflowId}`);
      }
    } else {
      navigate(`/workflows/${workflowId}`);
    }
  };

  const handleCreate = () => {
    createIssue();
    setMobileTab('editor');
    // Default to edit mode for new issues
    setViewMode('edit');
  };

  // Handle selecting an issue from sidebar
  const handleSelectIssue = (id) => {
    selectIssue(id);
    setMobileTab('editor');
    
    // Mobile Optimization: Auto-switch to preview mode when opening an issue
    // This allows the user to READ the issue immediately instead of seeing the code editor
    if (window.innerWidth < 768) {
      setViewMode('preview');
    }
  };

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e) => {
      // Cmd/Ctrl + S to save draft
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        // Save draft logic would go here if implemented
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Get selected issue
  const selectedIssue = editedIssues.find(i => i._localId === selectedIssueId);

  // If no issues loaded and not generating, show empty state or redirect
  if (!editedIssues.length && !generating && hasAttemptedFetch) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[calc(100vh-4rem)] gap-6 bg-background animate-fade-in">
        <div className="p-6 rounded-full bg-secondary/50">
           <FileText className="w-12 h-12 text-muted-foreground" />
        </div>
        <div className="text-center space-y-2">
           <h2 className="text-2xl font-semibold text-foreground">No Issues Generated</h2>
           <p className="text-muted-foreground max-w-md">
             Generate issues from the workflow detail page first to start editing.
           </p>
        </div>
        <Link
          to={`/workflows/${workflowId}`}
          className="inline-flex items-center gap-2 px-5 py-2.5 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-all font-medium shadow-sm hover:shadow-md"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to Workflow
        </Link>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[calc(100dvh-3.5rem)] w-full bg-background overflow-hidden relative border-0 m-0 p-0">
      {/* Loading Overlay - Rendered on top of content when generating */}
      {generating && (
        <GenerationLoadingOverlay
          workflowId={workflowId}
          progress={generationProgress}
          total={generationTotal}
          generatedIssues={generatedIssues}
          onCancel={() => {
            useIssuesStore.getState().cancelGeneration();
            navigate(`/workflows/${workflowId}`);
          }}
        />
      )}

      {/* Header */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-border bg-card shrink-0 z-20">
        <div className="flex items-center gap-4">
          <button
            onClick={handleBack}
            className="p-2 -ml-2 rounded-lg hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors"
            aria-label="Back to workflow"
          >
            <ArrowLeft className="w-5 h-5" />
          </button>
          <div className="flex flex-col">
            <h1 className="text-lg font-bold text-foreground tracking-tight leading-none">Issues Editor</h1>
            <p className="text-xs text-muted-foreground truncate max-w-[200px] md:max-w-md mt-0.5">
              {workflow?.title || workflowId}
            </p>
          </div>
        </div>

        {/* Status indicators */}
        <div className="flex items-center gap-4">
          {/* AI Status */}
          {aiErrors && aiErrors.length > 0 ? (
            <span
              className="hidden md:flex items-center gap-1.5 text-[10px] font-bold uppercase text-warning bg-warning/10 px-2 py-1 rounded-md border border-warning/20"
              title={`AI errors: ${aiErrors.join(', ')}`}
            >
              <AlertCircle className="w-3 h-3" />
              AI warnings
            </span>
          ) : aiUsed ? (
            <span className="hidden md:flex items-center gap-1.5 text-[10px] font-bold uppercase text-success bg-success/10 px-2 py-1 rounded-md border border-success/20">
              <CheckCircle2 className="w-3 h-3" />
              AI Enhanced
            </span>
          ) : null}

          {/* Save status */}
          {hasAnyUnsavedChanges() ? (
            <span className="flex items-center gap-1.5 text-xs text-warning font-semibold">
              <AlertCircle className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Unsaved changes</span>
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <CheckCircle2 className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">All saved</span>
            </span>
          )}
        </div>
      </header>

      {/* Main Content */}
      <div className="flex flex-1 overflow-hidden min-h-0 relative">
        {/* Sidebar - Hidden on mobile when editor tab is active */}
        <div className={`${
          mobileTab === 'list' ? 'flex' : 'hidden'
        } md:flex w-full md:w-80 lg:w-96 border-r border-border bg-card/30 shrink-0 z-10 transition-all flex-col h-full overflow-hidden`}>
          <IssuesSidebar
            issues={editedIssues}
            selectedId={selectedIssueId}
            onSelect={handleSelectIssue}
            onCreate={handleCreate}
            hasUnsavedChanges={hasUnsavedChanges}
          />
        </div>

        {/* Editor Panel - Hidden on mobile when list tab is active */}
        <div className={`${
          mobileTab === 'editor' ? 'flex' : 'hidden'
        } md:flex flex-1 flex-col overflow-hidden bg-background relative h-full w-full min-w-0`}>
          {/* Background Pattern - Consistent across app */}
          <div className="absolute inset-0 bg-dot-pattern pointer-events-none" />
          
          <div className="relative flex-1 flex flex-col min-h-0 z-10 h-full w-full">
             <IssueEditorPanel
               issue={selectedIssue}
               viewMode={viewMode}
               onToggleView={() => setViewMode(viewMode === 'edit' ? 'preview' : 'edit')}
               workflowId={workflowId}
             />
          </div>
        </div>
      </div>

      {/* Mobile FAB */}
      {mobileTab === 'list' && (
        <FAB 
          onClick={handleCreate} 
          icon={Plus} 
          label="New Issue" 
        />
      )}

      {/* Action Bar (Global Actions like Bulk Submit) */}
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
