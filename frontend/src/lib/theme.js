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
    text: 'text-blue-600 dark:text-blue-400',
    bg: 'bg-blue-500/10',
    border: 'border-blue-500/20',
    borderStrip: 'border-blue-500',
    dot: 'bg-blue-500',
    tint: 'bg-blue-500/5',
  },
  completed: {
    text: 'text-emerald-600 dark:text-emerald-400',
    bg: 'bg-emerald-500/10',
    border: 'border-emerald-500/20',
    borderStrip: 'border-emerald-500',
    dot: 'bg-emerald-500',
    tint: 'bg-emerald-500/5',
  },
  failed: {
    text: 'text-rose-600 dark:text-rose-400',
    bg: 'bg-rose-500/10',
    border: 'border-rose-500/20',
    borderStrip: 'border-rose-500',
    dot: 'bg-rose-500',
    tint: 'bg-rose-500/5',
  },
  paused: {
    text: 'text-amber-600 dark:text-amber-400',
    bg: 'bg-amber-500/10',
    border: 'border-amber-500/20',
    borderStrip: 'border-amber-500',
    dot: 'bg-amber-500',
    tint: 'bg-amber-500/5',
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