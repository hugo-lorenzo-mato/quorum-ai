// Centralized theme constants and utilities

export const STATUS_COLORS = {
  pending: {
    text: 'text-muted-foreground',
    bg: 'bg-muted',
    border: 'border-muted',
    borderStrip: 'border-muted-foreground',
    dot: 'bg-muted-foreground',
    tint: 'bg-muted/50',
  },
  running: {
    text: 'text-status-running',
    bg: 'bg-status-running-bg',
    border: 'border-status-running/20',
    borderStrip: 'border-status-running',
    dot: 'bg-status-running',
    tint: 'bg-status-running-bg/50',
  },
  completed: {
    text: 'text-status-success',
    bg: 'bg-status-success-bg',
    border: 'border-status-success/20',
    borderStrip: 'border-status-success',
    dot: 'bg-status-success',
    tint: 'bg-status-success-bg/50',
  },
  failed: {
    text: 'text-status-error',
    bg: 'bg-status-error-bg',
    border: 'border-status-error/20',
    borderStrip: 'border-status-error',
    dot: 'bg-status-error',
    tint: 'bg-status-error-bg/50',
  },
  paused: {
    text: 'text-status-paused',
    bg: 'bg-status-paused-bg',
    border: 'border-status-paused/20',
    borderStrip: 'border-status-paused',
    dot: 'bg-status-paused',
    tint: 'bg-status-paused-bg/50',
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