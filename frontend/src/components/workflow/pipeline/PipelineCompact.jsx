import { Sparkles, MessageSquare, ListTodo, Play, GitMerge } from 'lucide-react';
import PhaseChip from './PhaseChip';
import FlowConnector from './FlowConnector';

function scoreColorClass(score, threshold, warningThreshold) {
  if (score == null) return 'text-muted-foreground';
  if (score >= threshold) return 'text-status-success';
  if (warningThreshold > 0 && score >= warningThreshold) return 'text-status-paused';
  return 'text-status-error';
}

function scoreBgClass(score, threshold, warningThreshold) {
  if (score == null) return 'bg-muted/50';
  if (score >= threshold) return 'bg-status-success-bg';
  if (warningThreshold > 0 && score >= warningThreshold) return 'bg-status-paused-bg';
  return 'bg-status-error-bg';
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
          <FlowConnector completed={refine.status === 'completed'} nextRunning={analyze.status === 'running'} />
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
            <span className={`inline-flex items-center px-1 rounded-full ${scoreBgClass(analyze.consensusScore, analyze.threshold, analyze.warningThreshold)} ${scoreColorClass(analyze.consensusScore, analyze.threshold, analyze.warningThreshold)}`}>
              {analyze.consensusScore != null ? `${Math.round(analyze.consensusScore * 100)}%` : ''}
            </span>
          </span>
        )}
      </PhaseChip>

      <FlowConnector completed={analyze.status === 'completed'} nextRunning={plan.status === 'running'} />

      {/* Plan */}
      <PhaseChip
        label="Plan"
        status={plan.status}
        icon={ListTodo}
        isExpanded={expandedPhase === 'plan'}
        onClick={() => onPhaseClick?.('plan')}
      >
        {plan.multiPlanEnabled && (
          <span className="inline-flex items-center px-1 py-0.5 rounded-full bg-muted/50 ring-1 ring-border/30">
            <GitMerge className="w-2.5 h-2.5" />
          </span>
        )}
      </PhaseChip>

      <FlowConnector completed={plan.status === 'completed'} nextRunning={execute.status === 'running'} />

      {/* Execute */}
      <PhaseChip
        label="Execute"
        status={execute.status}
        icon={Play}
        isExpanded={expandedPhase === 'execute'}
        onClick={() => onPhaseClick?.('execute')}
      >
        {execute.taskCount > 0 && (
          <span className="inline-flex items-center px-1.5 rounded-full bg-muted/50 text-[10px] tabular-nums ring-1 ring-border/30">
            {execute.taskCount}t
          </span>
        )}
      </PhaseChip>
    </div>
  );
}
