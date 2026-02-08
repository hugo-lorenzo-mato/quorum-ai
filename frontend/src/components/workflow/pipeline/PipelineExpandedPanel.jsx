import PhaseDetailRefine from './PhaseDetailRefine';
import PhaseDetailAnalyze from './PhaseDetailAnalyze';
import PhaseDetailPlan from './PhaseDetailPlan';
import PhaseDetailExecute from './PhaseDetailExecute';

const PHASE_ACCENT = {
  refine: 'from-transparent via-violet-500/60 to-transparent',
  analyze: 'from-transparent via-status-running/60 to-transparent',
  plan: 'from-transparent via-amber-500/60 to-transparent',
  execute: 'from-transparent via-status-success/60 to-transparent',
};

export default function PipelineExpandedPanel({ expandedPhase, pipelineState }) {
  const { phases } = pipelineState;

  return (
    <div
      className={`
        overflow-hidden transition-all duration-300 ease-in-out
        ${expandedPhase ? 'max-h-[500px] opacity-100 mt-3' : 'max-h-0 opacity-0'}
      `}
    >
      <div className="relative rounded-xl border border-border/60 bg-card/80 backdrop-blur-sm shadow-premium animate-fade-up">
        {/* Decorative top accent line */}
        <div className={`absolute inset-x-0 top-0 h-0.5 rounded-t-xl bg-gradient-to-r ${PHASE_ACCENT[expandedPhase] || PHASE_ACCENT.analyze}`} />
        <div className="p-4">
          {expandedPhase === 'refine' && <PhaseDetailRefine refine={phases.refine} />}
          {expandedPhase === 'analyze' && <PhaseDetailAnalyze analyze={phases.analyze} />}
          {expandedPhase === 'plan' && <PhaseDetailPlan plan={phases.plan} />}
          {expandedPhase === 'execute' && <PhaseDetailExecute execute={phases.execute} />}
        </div>
      </div>
    </div>
  );
}
