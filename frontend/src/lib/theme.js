// Centralized theme constants and utilities

export const STATUS_COLORS = {
  pending: {
    text: 'text-muted-foreground',
    bg: 'bg-muted',
    border: 'border-muted',
    dot: 'bg-muted-foreground',
    tint: 'bg-muted/50',
  },
  running: {
    text: 'text-info',
    bg: 'bg-info/10',
    border: 'border-info/20',
    dot: 'bg-info',
    tint: 'bg-info/10',
  },
  completed: {
    text: 'text-success',
    bg: 'bg-success/10',
    border: 'border-success/20',
    dot: 'bg-success',
    tint: 'bg-success/10',
  },
  failed: {
    text: 'text-error',
    bg: 'bg-error/10',
    border: 'border-error/20',
    dot: 'bg-error',
    tint: 'bg-error/10',
  },
  paused: {
    text: 'text-warning',
    bg: 'bg-warning/10',
    border: 'border-warning/20',
    dot: 'bg-warning',
    tint: 'bg-warning/10',
  },
};

export const KANBAN_COLUMN_COLORS = {
  refinement: { dot: 'bg-yellow-500', tint: 'bg-yellow-500/10', ring: 'ring-yellow-500/20' },
  todo: { dot: 'bg-blue-500', tint: 'bg-blue-500/10', ring: 'ring-blue-500/20' },
  in_progress: { dot: 'bg-purple-500', tint: 'bg-purple-500/10', ring: 'ring-purple-500/20' },
  to_verify: { dot: 'bg-orange-500', tint: 'bg-orange-500/10', ring: 'ring-orange-500/20' },
  done: { dot: 'bg-green-500', tint: 'bg-green-500/10', ring: 'ring-green-500/20' },
  default: { dot: 'bg-muted-foreground', tint: 'bg-muted', ring: 'ring-ring/20' },
};

export function getStatusColor(status) {
  return STATUS_COLORS[status?.toLowerCase()] || STATUS_COLORS.pending;
}
