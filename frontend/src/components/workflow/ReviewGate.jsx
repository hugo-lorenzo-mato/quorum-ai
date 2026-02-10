import { useState } from 'react';
import { CheckCircle, XCircle, Loader2, MessageSquare } from 'lucide-react';
import useWorkflowStore from '../../stores/workflowStore';
import TaskEditor from './TaskEditor';

/**
 * ReviewGateInner contains the stateful form.
 * Parent ReviewGate uses a key to reset state when workflow/status changes.
 */
function ReviewGateInner({ workflow }) {
  const [feedback, setFeedback] = useState('');
  const [continueUnattended, setContinueUnattended] = useState(false);
  const { reviewWorkflow, loading } = useWorkflowStore();

  const { id, current_phase } = workflow;

  // Determine which phase just completed (the one before current_phase)
  const phaseOrder = ['analyze', 'plan', 'execute', 'done'];
  const currentIndex = phaseOrder.indexOf(current_phase);
  const completedPhase = currentIndex > 0 ? phaseOrder[currentIndex - 1] : current_phase;

  // Show task editor when plan phase completed (current_phase is 'execute')
  const showTaskEditor = current_phase === 'execute';

  const handleApprove = async () => {
    await reviewWorkflow(id, {
      action: 'approve',
      feedback: feedback.trim() || undefined,
      phase: completedPhase,
      continueUnattended,
    });
  };

  const handleReject = async () => {
    await reviewWorkflow(id, {
      action: 'reject',
      feedback: feedback.trim() || undefined,
      phase: completedPhase,
    });
  };

  return (
    <div className="rounded-lg border border-warning/30 bg-warning/5 p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center gap-2">
        <MessageSquare className="w-5 h-5 text-warning" />
        <h3 className="font-medium text-foreground">
          Review: {completedPhase} phase completed
        </h3>
      </div>

      {/* Feedback textarea */}
      <div>
        <label className="block text-sm text-muted-foreground mb-1.5">
          Feedback or guidance for the next phase (optional)
        </label>
        <textarea
          value={feedback}
          onChange={(e) => setFeedback(e.target.value)}
          placeholder="Add notes, corrections, or additional guidance..."
          rows={3}
          className="w-full px-3 py-2 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring resize-y text-sm"
        />
      </div>

      {/* Task Editor - shown after plan phase */}
      {showTaskEditor && (
        <TaskEditor workflowId={id} />
      )}

      {/* Continue unattended option */}
      <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
        <input
          type="checkbox"
          checked={continueUnattended}
          onChange={(e) => setContinueUnattended(e.target.checked)}
          className="w-4 h-4 rounded border-input"
        />
        Continue without pausing (switch to unattended mode)
      </label>

      {/* Action buttons */}
      <div className="flex gap-2">
        <button
          onClick={handleApprove}
          disabled={loading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-success/10 text-success hover:bg-success/20 disabled:opacity-50 transition-all font-medium"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <CheckCircle className="w-4 h-4" />}
          Approve & Continue
        </button>
        <button
          onClick={handleReject}
          disabled={loading}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-destructive/10 text-destructive hover:bg-destructive/20 disabled:opacity-50 transition-all font-medium"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <XCircle className="w-4 h-4" />}
          Reject & Redo
        </button>
      </div>
    </div>
  );
}

/**
 * ReviewGate displays when a workflow is in 'awaiting_review' status.
 * Uses a key to reset internal form state when workflow/status changes.
 */
export default function ReviewGate({ workflow }) {
  if (workflow.status !== 'awaiting_review') return null;
  return <ReviewGateInner key={`${workflow.id}:${workflow.status}`} workflow={workflow} />;
}
