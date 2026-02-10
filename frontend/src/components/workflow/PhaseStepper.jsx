import { CheckCircle2, Circle, Loader2, Eye } from 'lucide-react';

const PHASES = [
  { id: 'analyze', label: 'Analyze' },
  { id: 'plan', label: 'Plan' },
  { id: 'execute', label: 'Execute' },
];

function getPhaseStatus(phaseId, currentPhase, workflowStatus) {
  const phaseOrder = { analyze: 0, plan: 1, execute: 2 };
  const currentOrder = phaseOrder[currentPhase] ?? -1;
  const thisOrder = phaseOrder[phaseId];

  // If workflow is awaiting review and this is the current phase
  if (workflowStatus === 'awaiting_review' && currentPhase === phaseId) {
    return 'review';
  }

  // If workflow is running and this is the current phase
  if (workflowStatus === 'running' && currentPhase === phaseId) {
    return 'running';
  }

  // If this phase is before current phase, it's completed
  if (currentOrder > thisOrder) {
    return 'completed';
  }

  // If workflow completed and current_phase is empty or 'done' (all done)
  if (workflowStatus === 'completed' && (currentPhase === '' || currentPhase === 'done')) {
    return 'completed';
  }

  // If workflow completed and current phase matches, it's ready to run
  if (workflowStatus === 'completed' && currentPhase === phaseId) {
    return 'ready';
  }

  // Pending
  return 'pending';
}

export default function PhaseStepper({ workflow, compact = false }) {
  const { status, current_phase } = workflow;

  if (compact) {
    return (
      <div className="flex items-center gap-1">
        {PHASES.map((phase, index) => {
          const phaseStatus = getPhaseStatus(phase.id, current_phase, status);

          return (
            <div key={phase.id} className="flex items-center">
              {/* Compact phase indicator */}
              <div
                className={`
                  flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium
                  ${phaseStatus === 'completed' ? 'bg-success/10 text-success' : ''}
                  ${phaseStatus === 'running' ? 'bg-info/10 text-info' : ''}
                  ${phaseStatus === 'review' ? 'bg-warning/10 text-warning' : ''}
                  ${phaseStatus === 'ready' ? 'bg-primary/10 text-primary' : ''}
                  ${phaseStatus === 'pending' ? 'bg-muted text-muted-foreground' : ''}
                `}
              >
                {phaseStatus === 'completed' && <CheckCircle2 className="w-3 h-3" />}
                {phaseStatus === 'running' && <Loader2 className="w-3 h-3 animate-spin" />}
                {phaseStatus === 'review' && <Eye className="w-3 h-3" />}
                {phaseStatus === 'ready' && <Circle className="w-3 h-3" />}
                {phaseStatus === 'pending' && <Circle className="w-3 h-3" />}
                {phase.label}
              </div>

              {/* Connector */}
              {index < PHASES.length - 1 && (
                <div
                  className={`
                    w-4 h-0.5 mx-0.5
                    ${phaseStatus === 'completed' ? 'bg-success' : 'bg-muted'}
                  `}
                />
              )}
            </div>
          );
        })}
      </div>
    );
  }

  // Full version
  return (
    <div className="flex items-center gap-2 py-3">
      {PHASES.map((phase, index) => {
        const phaseStatus = getPhaseStatus(phase.id, current_phase, status);

        return (
          <div key={phase.id} className="flex items-center">
            {/* Phase indicator */}
            <div className="flex flex-col items-center">
              <div
                className={`
                  w-8 h-8 rounded-full flex items-center justify-center
                  ${phaseStatus === 'completed' ? 'bg-success/20 text-success' : ''}
                  ${phaseStatus === 'running' ? 'bg-info/20 text-info' : ''}
                  ${phaseStatus === 'review' ? 'bg-warning/20 text-warning' : ''}
                  ${phaseStatus === 'ready' ? 'bg-primary/20 text-primary' : ''}
                  ${phaseStatus === 'pending' ? 'bg-muted text-muted-foreground' : ''}
                `}
              >
                {phaseStatus === 'completed' && <CheckCircle2 className="w-4 h-4" />}
                {phaseStatus === 'running' && <Loader2 className="w-4 h-4 animate-spin" />}
                {phaseStatus === 'review' && <Eye className="w-4 h-4" />}
                {phaseStatus === 'ready' && <Circle className="w-4 h-4" />}
                {phaseStatus === 'pending' && <Circle className="w-4 h-4" />}
              </div>
              <span className="text-xs mt-1 text-muted-foreground">{phase.label}</span>
            </div>

            {/* Connector line */}
            {index < PHASES.length - 1 && (
              <div
                className={`
                  h-0.5 w-12 mx-2
                  ${phaseStatus === 'completed' ? 'bg-success' : 'bg-muted'}
                `}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
