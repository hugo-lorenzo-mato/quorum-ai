import { useState } from 'react';
import { Play, RefreshCw, FastForward, Loader2, AlertCircle, RotateCcw } from 'lucide-react';
import useWorkflowStore from '../../stores/workflowStore';
import ReplanModal from './ReplanModal';
import PhaseStepper from './PhaseStepper';

export default function PhaseControls({ workflow }) {
  const [isReplanModalOpen, setReplanModalOpen] = useState(false);
  const {
    analyzeWorkflow,
    planWorkflow,
    replanWorkflow,
    executeWorkflow,
    startWorkflow,
    resumeWorkflow,
    loading,
    error,
  } = useWorkflowStore();

  const { id, status, current_phase } = workflow;

  // Determine which buttons to show based on workflow state
  const isPending = status === 'pending';
  const isRunning = status === 'running';
  const isPaused = status === 'paused';
  const isFailed = status === 'failed';
  const isCompleted = status === 'completed';
  const isFullyCompleted = isCompleted && (!current_phase || current_phase === 'done');

  // Phase-specific conditions
  const canRunAll = isPending;
  const canAnalyze = isPending;
  const canPlan = isCompleted && current_phase === 'plan';
  const canReplan = isCompleted && ['plan', 'execute', 'done'].includes(current_phase);
  const canExecute = isCompleted && current_phase === 'execute';

  // Resume/Retry conditions
  const canResume = isPaused;
  const canRetry = isFailed;

  const handleAnalyze = async () => {
    await analyzeWorkflow(id);
  };

  const handlePlan = async () => {
    await planWorkflow(id);
  };

  const handleReplan = async (context) => {
    await replanWorkflow(id, context);
    setReplanModalOpen(false);
  };

  const handleExecute = async () => {
    await executeWorkflow(id);
  };

  const handleRunAll = async () => {
    await startWorkflow(id);
  };

  const handleResume = async () => {
    await resumeWorkflow(id);
  };

  const handleRetry = async () => {
    // Retry uses startWorkflow which detects failed state and resumes
    await startWorkflow(id);
  };

  return (
    <div className="space-y-4">
      {/* Phase Progress Stepper */}
      <PhaseStepper workflow={workflow} />

      {/* Action Buttons */}
      <div className="flex flex-wrap gap-2">
        {/* Run All button - only for pending workflows */}
        {canRunAll && (
          <button
            onClick={handleRunAll}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <FastForward className="w-4 h-4" />}
            Run All Phases
          </button>
        )}

        {/* Analyze button - for pending workflows wanting step-by-step */}
        {canAnalyze && (
          <button
            onClick={handleAnalyze}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-info/10 text-info hover:bg-info/20 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Analyze Only
          </button>
        )}

        {/* Plan button */}
        {canPlan && (
          <button
            onClick={handlePlan}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-info/10 text-info hover:bg-info/20 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Generate Plan
          </button>
        )}

        {/* Replan button */}
        {canReplan && (
          <button
            onClick={() => setReplanModalOpen(true)}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-warning/10 text-warning hover:bg-warning/20 disabled:opacity-50 transition-all"
          >
            <RefreshCw className="w-4 h-4" />
            Replan
          </button>
        )}

        {/* Execute button */}
        {canExecute && (
          <button
            onClick={handleExecute}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-success/10 text-success hover:bg-success/20 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Execute Plan
          </button>
        )}

        {/* Resume button - for paused workflows */}
        {canResume && (
          <button
            onClick={handleResume}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Resume
          </button>
        )}

        {/* Retry button - for failed workflows */}
        {canRetry && (
          <button
            onClick={handleRetry}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RotateCcw className="w-4 h-4" />}
            Retry
          </button>
        )}

        {/* Running indicator */}
        {isRunning && (
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-info/10 text-info">
            <Loader2 className="w-4 h-4 animate-spin" />
            {current_phase ? `Running ${current_phase}...` : 'Running...'}
          </div>
        )}

        {/* Paused indicator */}
        {isPaused && (
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-warning/10 text-warning">
            Paused at {current_phase || 'unknown phase'}
          </div>
        )}

        {/* Completed indicator */}
        {isFullyCompleted && (
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-success/10 text-success">
            All phases completed
          </div>
        )}
      </div>

      {/* Error Display */}
      {error && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-destructive/10 text-destructive text-sm">
          <AlertCircle className="w-4 h-4" />
          {error}
        </div>
      )}

      {/* Replan Modal */}
      <ReplanModal
        isOpen={isReplanModalOpen}
        onClose={() => setReplanModalOpen(false)}
        onSubmit={handleReplan}
        loading={loading}
      />
    </div>
  );
}
