import { ListTodo, GitMerge } from 'lucide-react';
import MiniFlowNode from './MiniFlowNode';
import FlowConnector from './FlowConnector';

export default function PhaseDetailPlan({ plan }) {
  const { multiPlanEnabled, planSynthAgent, status } = plan;

  if (!multiPlanEnabled) {
    return (
      <div className="space-y-3">
        <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Plan Phase</h4>
        <div className="flex items-center justify-center py-4">
          <MiniFlowNode label="Plan" status={status} icon={ListTodo} subtitle="Single plan" />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Plan Phase (Multi-Agent)</h4>
      <div className="flex items-center justify-center gap-1 py-4">
        <MiniFlowNode label="Plan 1" status={status} icon={ListTodo} />
        <div className="flex flex-col items-center gap-1">
          <FlowConnector completed={status === 'completed'} />
        </div>
        <MiniFlowNode label="Plan 2" status={status} icon={ListTodo} />
        <FlowConnector completed={status === 'completed'} />
        <MiniFlowNode
          label="Consolidate"
          status={status}
          icon={GitMerge}
          subtitle={planSynthAgent || 'synthesizer'}
        />
      </div>
    </div>
  );
}
