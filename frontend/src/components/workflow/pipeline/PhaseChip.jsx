import { CheckCircle2, Circle, Loader2, XCircle, MinusCircle, X } from 'lucide-react';

const STATUS_CONFIG = {
  completed: {
    Icon: CheckCircle2,
    classes: 'bg-status-success-bg text-status-success',
  },
  running: {
    Icon: Loader2,
    classes: 'bg-status-running-bg text-status-running',
    iconClass: 'animate-spin',
  },
  failed: {
    Icon: XCircle,
    classes: 'bg-status-error-bg text-status-error',
  },
  skipped: {
    Icon: MinusCircle,
    classes: 'bg-muted text-muted-foreground opacity-60',
  },
  disabled: {
    Icon: X,
    classes: 'bg-muted text-muted-foreground opacity-40',
  },
  pending: {
    Icon: Circle,
    classes: 'bg-muted text-muted-foreground',
  },
};

export default function PhaseChip({ label, status, icon: PhaseIcon, isExpanded, onClick, children }) {
  const config = STATUS_CONFIG[status] || STATUS_CONFIG.pending;
  const StatusIcon = config.Icon;

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium
        cursor-pointer hover:bg-accent/50 transition-all
        ${config.classes}
        ${isExpanded ? 'ring-2 ring-primary/50' : ''}
      `}
    >
      {PhaseIcon && <PhaseIcon className="w-3 h-3" />}
      <StatusIcon className={`w-3 h-3 ${config.iconClass || ''}`} />
      {label}
      {children}
    </button>
  );
}
