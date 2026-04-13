// Centralized theme constants and utilities

export const STATUS_COLORS = {
  pending: {
    text: 'text-muted-foreground',
    bg: 'bg-muted/80',
    border: 'border-muted-foreground/30',
    borderStrip: 'border-muted-foreground/50',
    dot: 'bg-muted-foreground/40',
    tint: 'bg-muted/40',
    shadow: 'shadow-muted/10',
  },
  running: {
    text: 'text-status-running',
    bg: 'bg-status-running/15',
    border: 'border-status-running/40',
    borderStrip: 'border-status-running',
    dot: 'bg-status-running',
    tint: 'bg-status-running/10',
    shadow: 'shadow-status-running/10',
  },
  completed: {
    text: 'text-status-success',
    bg: 'bg-status-success/15',
    border: 'border-status-success/40',
    borderStrip: 'border-status-success',
    dot: 'bg-status-success',
    tint: 'bg-status-success/10',
    shadow: 'shadow-status-success/10',
  },
  failed: {
    text: 'text-status-error',
    bg: 'bg-status-error/15',
    border: 'border-status-error/40',
    borderStrip: 'border-status-error',
    dot: 'bg-status-error',
    tint: 'bg-status-error/10',
    shadow: 'shadow-status-error/10',
  },
  skipped: {
    text: 'text-muted-foreground/70',
    bg: 'bg-muted/50',
    border: 'border-border/60',
    borderStrip: 'border-muted-foreground/60',
    dot: 'bg-muted-foreground/60',
    tint: 'bg-muted/40',
    shadow: 'shadow-muted/5',
  },
  paused: {
    text: 'text-status-paused',
    bg: 'bg-status-paused/15',
    border: 'border-status-paused/40',
    borderStrip: 'border-status-paused',
    dot: 'bg-status-paused',
    tint: 'bg-status-paused/10',
    shadow: 'shadow-status-paused/10',
  },
  cancelling: {
    text: 'text-status-paused',
    bg: 'bg-status-paused/15',
    border: 'border-status-paused/40',
    borderStrip: 'border-status-paused',
    dot: 'bg-status-paused',
    tint: 'bg-status-paused/10',
    shadow: 'shadow-status-paused/10',
  },
  aborted: {
    text: 'text-muted-foreground',
    bg: 'bg-muted/80',
    border: 'border-muted/40',
    borderStrip: 'border-muted-foreground/50',
    dot: 'bg-muted-foreground/40',
    tint: 'bg-muted/40',
    shadow: 'shadow-muted/10',
  },
};

export const KANBAN_COLUMN_COLORS = {
  refinement: {
    dot: 'bg-sky-500',
    tint: 'bg-sky-500/10',
    ring: 'ring-sky-500/20',
    border: 'border-sky-500'
  },
  todo: { 
    dot: 'bg-indigo-500', 
    tint: 'bg-indigo-500/5', 
    ring: 'ring-indigo-500/20',
    border: 'border-indigo-500'
  },
  in_progress: { 
    dot: 'bg-violet-500', 
    tint: 'bg-violet-500/5', 
    ring: 'ring-violet-500/20',
    border: 'border-violet-500'
  },
  to_verify: { 
    dot: 'bg-orange-500', 
    tint: 'bg-orange-500/5', 
    ring: 'ring-orange-500/20',
    border: 'border-orange-500'
  },
  done: { 
    dot: 'bg-emerald-500', 
    tint: 'bg-emerald-500/5', 
    ring: 'ring-emerald-500/20',
    border: 'border-emerald-500'
  },
  default: { 
    dot: 'bg-muted-foreground', 
    tint: 'bg-muted', 
    ring: 'ring-ring/20',
    border: 'border-muted-foreground'
  },
};

export function getStatusColor(status) {
  return STATUS_COLORS[status?.toLowerCase()] || STATUS_COLORS.pending;
}
