import { CheckCircle2, Circle, Loader2, XCircle, MinusCircle, X } from 'lucide-react';

const STATUS_CONFIG = {
  completed: {
    Icon: CheckCircle2,
    classes: 'bg-gradient-to-r from-status-success-bg to-status-success-bg/50 text-status-success ring-1 ring-status-success/20',
  },
  running: {
    Icon: Loader2,
    classes: 'bg-gradient-to-r from-status-running-bg to-status-running-bg/50 text-status-running ring-1 ring-status-running/20 shadow-sm shadow-status-running/10 animate-pulse-glow',
    iconClass: 'animate-spin',
  },
  failed: {
    Icon: XCircle,
    classes: 'bg-gradient-to-r from-status-error-bg to-status-error-bg/50 text-status-error ring-1 ring-status-error/20',
  },
  skipped: {
    Icon: MinusCircle,
    classes: 'bg-muted/40 text-muted-foreground/60 ring-1 ring-border/30',
  },
  disabled: {
    Icon: X,
    classes: 'bg-muted/30 text-muted-foreground/40',
  },
  pending: {
    Icon: Circle,
    classes: 'bg-muted/50 text-muted-foreground ring-1 ring-border/30',
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
        inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium
        cursor-pointer hover:-translate-y-0.5 hover:shadow-sm transition-all duration-300
        ${config.classes}
        ${isExpanded ? 'ring-2 ring-primary/30 shadow-md' : ''}
      `}
    >
      {PhaseIcon && <PhaseIcon className="w-3 h-3" />}
      <StatusIcon className={`w-3 h-3 ${config.iconClass || ''}`} />
      {label}
      {children}
    </button>
  );
}
