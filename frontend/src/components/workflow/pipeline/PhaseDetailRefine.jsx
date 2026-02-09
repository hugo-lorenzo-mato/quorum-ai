import { Sparkles } from 'lucide-react';
import MiniFlowNode from './MiniFlowNode';

export default function PhaseDetailRefine({ refine }) {
  return (
    <div className="space-y-3">
      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Refine Phase</h4>
      <div className="flex items-center justify-center py-4">
        <MiniFlowNode
          label={refine.agentName || 'Refiner'}
          status={refine.status}
          icon={Sparkles}
          subtitle="Prompt refinement"
        />
      </div>
    </div>
  );
}
