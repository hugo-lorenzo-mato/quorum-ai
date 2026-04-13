import { Clock, Activity, StopCircle, CheckCircle2, XCircle, Pause } from 'lucide-react';
import { getStatusColor } from '../lib/theme';

export default function StatusBadge({ status }) {
  const { bg, text, border } = getStatusColor(status);
  
  const iconMap = {
    pending: Clock,
    running: Activity,
    cancelling: StopCircle,
    aborted: StopCircle,
    completed: CheckCircle2,
    failed: XCircle,
    paused: Pause,
  };
  
  const StatusIcon = iconMap[status] || Clock;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider border ${bg} ${text} ${border} shadow-sm transition-all`}>
      <StatusIcon className="w-3 h-3" strokeWidth={3} />
      {status}
    </span>
  );
}
