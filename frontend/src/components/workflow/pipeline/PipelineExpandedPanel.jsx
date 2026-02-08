import PhaseDetailRefine from './PhaseDetailRefine';
import PhaseDetailAnalyze from './PhaseDetailAnalyze';
import PhaseDetailPlan from './PhaseDetailPlan';
import PhaseDetailExecute from './PhaseDetailExecute';

export default function PipelineExpandedPanel({ expandedPhase, pipelineState }) {
  const { phases } = pipelineState;

  return (
    <div
      className={`
        overflow-hidden transition-all duration-300 ease-in-out
        ${expandedPhase ? 'max-h-[500px] opacity-100 mt-3' : 'max-h-0 opacity-0'}
      `}
    >
      <div className="p-4 rounded-xl border border-border bg-card">
        {expandedPhase === 'refine' && <PhaseDetailRefine refine={phases.refine} />}
        {expandedPhase === 'analyze' && <PhaseDetailAnalyze analyze={phases.analyze} />}
        {expandedPhase === 'plan' && <PhaseDetailPlan plan={phases.plan} />}
        {expandedPhase === 'execute' && <PhaseDetailExecute execute={phases.execute} />}
      </div>
    </div>
  );
}
