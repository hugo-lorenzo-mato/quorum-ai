import { CheckCircle2, Loader2, Circle } from 'lucide-react';
import MiniFlowNode from './MiniFlowNode';
import FlowConnector from './FlowConnector';
import LoopArrow from './LoopArrow';

function ConfigBadge({ label, value }) {
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-gradient-to-r from-muted to-muted/60 text-[10px] font-medium text-muted-foreground ring-1 ring-border/30">
      {label}: <span className="text-foreground">{value}</span>
    </span>
  );
}

function scoreColor(score, threshold) {
  if (score == null) return '';
  if (score >= threshold) return 'text-status-success bg-status-success-bg';
  return 'text-status-error bg-status-error-bg';
}

function RoundsTable({ rounds, threshold }) {
  if (!rounds || rounds.length === 0) return null;

  return (
    <div className="mt-4">
      <table className="w-full text-xs">
        <thead>
          <tr className="text-muted-foreground border-b border-border/50">
            <th className="text-left py-1.5 font-medium">Round</th>
            <th className="text-left py-1.5 font-medium">Agents</th>
            <th className="text-left py-1.5 font-medium">Score</th>
            <th className="text-left py-1.5 font-medium">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border/30">
          {rounds.map((r) => (
            <tr key={r.round} className="transition-colors hover:bg-muted/30">
              <td className="py-2 tabular-nums font-medium">{r.round}</td>
              <td className="py-2 text-muted-foreground">{r.agents.join(', ') || '-'}</td>
              <td className="py-2 tabular-nums">
                {r.score != null ? (
                  <span className={`inline-flex px-1.5 py-0.5 rounded-full text-[10px] font-medium ${scoreColor(r.score, threshold)}`}>
                    {Math.round(r.score * 100)}%
                  </span>
                ) : '-'}
              </td>
              <td className="py-2">
                {r.status === 'completed' && <CheckCircle2 className="w-3.5 h-3.5 text-status-success inline" />}
                {r.status === 'running' && <Loader2 className="w-3.5 h-3.5 text-status-running animate-spin inline" />}
                {r.status === 'pending' && <Circle className="w-3.5 h-3.5 text-muted-foreground inline" />}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function PhaseDetailAnalyze({ analyze }) {
  const {
    threshold, minRounds, maxRounds, stagnationThreshold,
    consensusEnabled, moderatorAgent, rounds, currentRound, synthesisStatus,
  } = analyze;

  // Build the flow diagram nodes from rounds
  const flowNodes = [];
  const maxToShow = Math.min((currentRound || 1) + 1, maxRounds || 5);

  for (let i = 1; i <= maxToShow; i++) {
    const roundData = rounds.find((r) => r.round === i);
    const vStatus = roundData
      ? (roundData.status === 'completed' ? 'completed' : 'running')
      : (i <= (currentRound || 0) ? 'completed' : 'pending');

    flowNodes.push({
      type: 'version',
      label: `V${i}`,
      status: vStatus,
      subtitle: roundData ? `${roundData.agents.length} ag` : '',
    });

    if (consensusEnabled) {
      const modStatus = roundData?.score != null ? 'completed' : (i === currentRound ? 'running' : 'pending');
      const scoreLabel = roundData?.score != null ? `${Math.round(roundData.score * 100)}%` : '';
      flowNodes.push({
        type: 'moderator',
        label: 'Mod',
        status: modStatus,
        subtitle: scoreLabel ? `R${i} ${scoreLabel}` : `R${i}`,
      });
    }
  }

  // Add synthesize node at the end
  flowNodes.push({
    type: 'synthesize',
    label: 'Synth',
    status: synthesisStatus,
    subtitle: '',
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Analyze Phase</h4>
        <div className="flex items-center gap-1.5 flex-wrap">
          {consensusEnabled && threshold > 0 && (
            <ConfigBadge label="Threshold" value={`${Math.round(threshold * 100)}%`} />
          )}
          {minRounds > 0 && maxRounds > 0 && (
            <ConfigBadge label="Rounds" value={`${minRounds}–${maxRounds}`} />
          )}
          {stagnationThreshold > 0 && (
            <ConfigBadge label="Stagnation" value={`${Math.round(stagnationThreshold * 100)}%`} />
          )}
          {moderatorAgent && (
            <ConfigBadge label="Moderator" value={moderatorAgent} />
          )}
        </div>
      </div>

      {/* Flow diagram */}
      <div className="relative">
        <div className="flex items-center gap-1 overflow-x-auto py-4 px-2">
          {flowNodes.map((node, idx) => (
            <div key={`${node.type}-${idx}`} className="flex items-center">
              <MiniFlowNode
                label={node.label}
                status={node.status}
                subtitle={node.subtitle}
              />
              {idx < flowNodes.length - 1 && (
                <FlowConnector
                  completed={node.status === 'completed'}
                  nextRunning={flowNodes[idx + 1]?.status === 'running'}
                />
              )}
            </div>
          ))}
        </div>

        {/* Loop arrow beneath the moderator nodes when consensus hasn't been reached */}
        {consensusEnabled && currentRound > 0 && currentRound < maxRounds && (
          <div className="flex justify-center -mt-1">
            <LoopArrow
              width={160}
              active={analyze.status === 'running'}
              label={`< ${Math.round(threshold * 100)}%`}
            />
          </div>
        )}
      </div>

      {/* Rounds table */}
      <RoundsTable rounds={rounds} threshold={threshold} />
    </div>
  );
}
