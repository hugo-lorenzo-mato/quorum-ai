import PipelineCompact from './pipeline/PipelineCompact';
import PipelineExpandedPanel from './pipeline/PipelineExpandedPanel';
import usePipelineState from './hooks/usePipelineState';

export default function WorkflowPipelineLive({
  workflow,
  workflowKey,
  compact = false,
  expandedPhase: externalExpanded,
  onPhaseClick: externalOnPhaseClick,
}) {
  const pipelineState = usePipelineState(workflow, workflowKey);

  // Use external state if provided, otherwise use internal
  const expandedPhase = externalExpanded !== undefined ? externalExpanded : pipelineState.expandedPhase;
  const onPhaseClick = externalOnPhaseClick || ((phase) => {
    pipelineState.setExpandedPhase((prev) => (prev === phase ? null : phase));
  });

  if (compact) {
    return (
      <PipelineCompact
        pipelineState={pipelineState}
        expandedPhase={expandedPhase}
        onPhaseClick={onPhaseClick}
      />
    );
  }

  return (
    <div>
      <PipelineCompact
        pipelineState={pipelineState}
        expandedPhase={expandedPhase}
        onPhaseClick={onPhaseClick}
      />
      <PipelineExpandedPanel
        expandedPhase={expandedPhase}
        pipelineState={pipelineState}
      />
    </div>
  );
}
