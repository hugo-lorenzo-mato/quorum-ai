import { CheckCircle2, Circle, Loader2, XCircle } from 'lucide-react';

const STATUS_MAP = {
  completed: { Icon: CheckCircle2, classes: 'bg-gradient-to-br from-status-success-bg to-status-success-bg/40 text-status-success ring-1 ring-status-success/20' },
  running:   { Icon: Loader2,      classes: 'bg-gradient-to-br from-status-running-bg to-status-running-bg/40 text-status-running ring-1 ring-status-running/20 shadow-sm shadow-status-running/10', iconClass: 'animate-spin' },
  failed:    { Icon: XCircle,      classes: 'bg-gradient-to-br from-status-error-bg to-status-error-bg/40 text-status-error ring-1 ring-status-error/20' },
  pending:   { Icon: Circle,       classes: 'bg-muted/50 text-muted-foreground ring-1 ring-border/30' },
};

export default function MiniFlowNode({ label, status = 'pending', subtitle, icon: CustomIcon }) {
  const config = STATUS_MAP[status] || STATUS_MAP.pending;
  const StatusIcon = CustomIcon || config.Icon;

  return (
    <div className="flex flex-col items-center gap-1">
      <div
        className={`
          inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium
          transition-all duration-300
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
