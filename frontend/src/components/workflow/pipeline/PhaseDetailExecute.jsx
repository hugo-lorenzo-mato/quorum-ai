import { Play } from 'lucide-react';
import MiniFlowNode from './MiniFlowNode';

export default function PhaseDetailExecute({ execute }) {
  const { status, taskCount } = execute;

  return (
    <div className="space-y-3">
      <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Execute Phase</h4>
      <div className="flex items-center justify-center py-4">
        <MiniFlowNode
          label="Execute"
          status={status}
          icon={Play}
          subtitle={taskCount > 0 ? `${taskCount} task${taskCount !== 1 ? 's' : ''}` : ''}
        />
      </div>
    </div>
  );
}
