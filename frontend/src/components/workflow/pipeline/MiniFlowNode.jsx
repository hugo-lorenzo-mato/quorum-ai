import { CheckCircle2, Circle, Loader2, XCircle } from 'lucide-react';

const STATUS_MAP = {
  completed: { Icon: CheckCircle2, classes: 'border-status-success/40 bg-status-success-bg text-status-success' },
  running:   { Icon: Loader2,      classes: 'border-status-running/40 bg-status-running-bg text-status-running', iconClass: 'animate-spin' },
  failed:    { Icon: XCircle,      classes: 'border-status-error/40 bg-status-error-bg text-status-error' },
  pending:   { Icon: Circle,       classes: 'border-border bg-muted text-muted-foreground' },
};

export default function MiniFlowNode({ label, status = 'pending', subtitle, icon: CustomIcon }) {
  const config = STATUS_MAP[status] || STATUS_MAP.pending;
  const StatusIcon = CustomIcon || config.Icon;

  return (
    <div className="flex flex-col items-center gap-1">
      <div
        className={`
          inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border text-xs font-medium
          ${config.classes}
        `}
      >
        <StatusIcon className={`w-3 h-3 ${config.iconClass || ''}`} />
        {label}
      </div>
      {subtitle && (
        <span className="text-[10px] text-muted-foreground">{subtitle}</span>
      )}
    </div>
  );
}
