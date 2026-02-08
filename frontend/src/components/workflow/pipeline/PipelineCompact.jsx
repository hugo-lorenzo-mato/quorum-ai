import { Sparkles, MessageSquare, ListTodo, Play, GitMerge } from 'lucide-react';
import PhaseChip from './PhaseChip';
import FlowConnector from './FlowConnector';

function scoreColorClass(score, threshold, warningThreshold) {
  if (score == null) return 'text-muted-foreground';
  if (score >= threshold) return 'text-status-success';
  if (warningThreshold > 0 && score >= warningThreshold) return 'text-status-paused';
  return 'text-status-error';
}

export default function PipelineCompact({ pipelineState, expandedPhase, onPhaseClick }) {
  const { refine, analyze, plan, execute } = pipelineState.phases;

  return (
    <div className="flex items-center gap-1">
      {/* Refine (optional) */}
      {refine.enabled && (
        <>
          <PhaseChip
            label="Refine"
            status={refine.status}
            icon={Sparkles}
            isExpanded={expandedPhase === 'refine'}
            onClick={() => onPhaseClick?.('refine')}
          />
          <FlowConnector completed={refine.status === 'completed'} />
        </>
      )}

      {/* Analyze */}
      <PhaseChip
        label="Analyze"
        status={analyze.status}
        icon={MessageSquare}
        isExpanded={expandedPhase === 'analyze'}
        onClick={() => onPhaseClick?.('analyze')}
      >
        {analyze.status === 'running' && analyze.consensusEnabled && (
          <span className="text-[10px] ml-1 tabular-nums">
            R{analyze.currentRound || 1}/{analyze.maxRounds}{' '}
            <span className={scoreColorClass(analyze.consensusScore, analyze.threshold, analyze.warningThreshold)}>
              {analyze.consensusScore != null ? `${Math.round(analyze.consensusScore * 100)}%` : ''}
            </span>
          </span>
        )}
      </PhaseChip>

      <FlowConnector completed={analyze.status === 'completed'} />

      {/* Plan */}
      <PhaseChip
        label="Plan"
        status={plan.status}
        icon={ListTodo}
        isExpanded={expandedPhase === 'plan'}
        onClick={() => onPhaseClick?.('plan')}
      >
        {plan.multiPlanEnabled && <GitMerge className="w-2.5 h-2.5 ml-0.5" />}
      </PhaseChip>

      <FlowConnector completed={plan.status === 'completed'} />

      {/* Execute */}
      <PhaseChip
        label="Execute"
        status={execute.status}
        icon={Play}
        isExpanded={expandedPhase === 'execute'}
        onClick={() => onPhaseClick?.('execute')}
      >
        {execute.taskCount > 0 && (
          <span className="text-[10px] ml-1 tabular-nums">{execute.taskCount}t</span>
        )}
      </PhaseChip>
    </div>
  );
}
