/**
 * Unified Card Components with Vercel-inspired design
 * 
 * Base card system used across the application:
 * - WorkflowCard
 * - ProjectCard  
 * - TemplateCard
 * - KanbanCard
 * 
 * All cards share:
 * - Consistent spacing and typography
 * - Gradient backgrounds
 * - Smooth hover effects
 * - Border and shadow treatments
 * - Backdrop blur effects
 */

import { cn } from '../../lib/utils';

/**
 * Base Card Container
 * Foundation for all card types with Vercel aesthetics
 */
export function CardBase({ 
  children, 
  onClick, 
  className,
  variant = 'default', // 'default' | 'selected' | 'executing' | 'completed' | 'failed'
  accentColor, // 'primary' | 'blue' | 'emerald' | 'rose' | 'amber' | 'violet'
  hoverable = true,
  ...props 
}) {
  const variantStyles = {
    default: 'border-border/40 bg-card hover:border-border/80 shadow-sm',
    selected: 'border-primary/40 bg-primary/5 ring-2 ring-primary/20 shadow-md',
    executing: 'border-status-running/30 bg-gradient-to-br from-status-running-bg/30 via-card to-card shadow-lg shadow-status-running/5 ring-1 ring-status-running/10',
    completed: 'border-border/40 bg-gradient-to-br from-card via-card to-status-success-bg/10 hover:border-status-success/30',
    failed: 'border-border/40 bg-gradient-to-br from-card via-card to-status-error-bg/10 hover:border-status-error/30',
  };

  const accentStyles = {
    primary: 'border-t-primary',
    blue: 'border-t-status-running',
    emerald: 'border-t-status-success',
    rose: 'border-t-status-error',
    amber: 'border-t-status-warning',
    violet: 'border-t-violet-500',
  };

  const hoverStyles = hoverable 
    ? 'hover:shadow-premium active:scale-[0.99] transition-all duration-300' 
    : '';

  return (
    <div
      onClick={onClick}
      className={cn(
        'relative flex flex-col rounded-xl border transition-all backdrop-blur-sm',
        accentColor && 'border-t-2',
        accentColor && accentStyles[accentColor],
        variantStyles[variant],
        hoverStyles,
        onClick && 'cursor-pointer',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * Card Header
 * Top section with title and primary info
 */
export function CardHeader({ children, className, ...props }) {
  return (
    <div 
      className={cn('flex items-start justify-between gap-3 p-4 pb-3', className)}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * Card Title
 * Main heading with proper typography
 */
export function CardTitle({ children, className, ...props }) {
  return (
    <h3 
      className={cn(
        'text-base font-semibold text-foreground leading-snug line-clamp-2 group-hover:text-primary transition-colors',
        className
      )}
      {...props}
    >
      {children}
    </h3>
  );
}

/**
 * Card Description
 * Subtitle or secondary text
 */
export function CardDescription({ children, className, ...props }) {
  return (
    <p 
      className={cn(
        'text-sm text-muted-foreground leading-relaxed line-clamp-2 mt-1.5',
        className
      )}
      {...props}
    >
      {children}
    </p>
  );
}

/**
 * Card Content
 * Main content area
 */
export function CardContent({ children, className, ...props }) {
  return (
    <div 
      className={cn('flex-1 px-4 pb-3', className)}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * Card Footer
 * Bottom section with metadata and actions
 */
export function CardFooter({ children, className, divided = true, ...props }) {
  return (
    <div 
      className={cn(
        'flex items-center justify-between gap-3 px-4 py-3 mt-auto',
        divided && 'border-t border-border/30',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * Card Metadata
 * Display small pieces of info (tags, counts, etc.)
 */
export function CardMeta({ children, className, ...props }) {
  return (
    <div 
      className={cn('flex items-center gap-3 text-xs text-muted-foreground flex-wrap', className)}
      {...props}
    >
      {children}
    </div>
  );
}

/**
 * Card Meta Item
 * Individual metadata element with optional icon
 */
export function CardMetaItem({ icon: Icon, children, className, interactive = false, ...props }) {
  const Component = interactive ? 'button' : 'div';
  
  return (
    <Component
      className={cn(
        'flex items-center gap-1.5 text-xs',
        interactive && 'px-2 py-1 rounded-md hover:bg-muted/50 hover:text-foreground transition-colors',
        className
      )}
      {...props}
    >
      {Icon && <Icon className="w-3.5 h-3.5" />}
      <span className="font-medium">{children}</span>
    </Component>
  );
}

/**
 * Card Badge
 * Status indicator or label
 */
export function CardBadge({ 
  children, 
  className,
  variant = 'default', // 'default' | 'primary' | 'success' | 'warning' | 'error' | 'info'
  ...props 
}) {
  const variantStyles = {
    default: 'bg-muted/50 text-muted-foreground border-border/40',
    primary: 'bg-primary/10 text-primary border-primary/20',
    success: 'bg-status-success-bg text-status-success border-status-success/20',
    warning: 'bg-status-warning-bg text-status-warning border-status-warning/20',
    error: 'bg-status-error-bg text-status-error border-status-error/20',
    info: 'bg-status-running-bg text-status-running border-status-running/20',
  };

  return (
    <span 
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-semibold uppercase tracking-wide border',
        variantStyles[variant],
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
}

/**
 * Card Action Button
 * Secondary action button in card
 */
export function CardAction({ 
  children, 
  icon: Icon,
  className,
  variant = 'ghost', // 'ghost' | 'primary' | 'destructive'
  ...props 
}) {
  const variantStyles = {
    ghost: 'hover:bg-accent hover:text-foreground',
    primary: 'bg-primary text-primary-foreground hover:bg-primary/90',
    destructive: 'hover:bg-destructive/10 hover:text-destructive hover:border-destructive/50',
  };

  return (
    <button
      className={cn(
        'inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm font-medium border border-border transition-colors',
        variantStyles[variant],
        className
      )}
      {...props}
    >
      {Icon && <Icon className="w-4 h-4" />}
      {children}
    </button>
  );
}

/**
 * Card Icon Container
 * Wrapper for icons with consistent styling
 */
export function CardIcon({ icon: Icon, className, color = 'default', ...props }) {
  const colorStyles = {
    default: 'bg-muted/50 border-border/50 text-muted-foreground',
    primary: 'bg-primary/10 border-primary/20 text-primary',
    blue: 'bg-status-running-bg border-status-running/20 text-status-running',
    emerald: 'bg-status-success-bg border-status-success/20 text-status-success',
    rose: 'bg-status-error-bg border-status-error/20 text-status-error',
    amber: 'bg-status-warning-bg border-status-warning/20 text-status-warning',
    violet: 'bg-violet-500/10 border-violet-500/20 text-violet-500',
  };

  return (
    <div 
      className={cn(
        'p-2 rounded-lg border',
        colorStyles[color],
        className
      )}
      {...props}
    >
      <Icon className="w-4 h-4" />
    </div>
  );
}

/**
 * Card Floating Badge
 * Positioned badge (top-right corner typical)
 */
export function CardFloatingBadge({ children, className, ...props }) {
  return (
    <div 
      className={cn(
        'absolute -top-2 -right-2 flex items-center gap-1.5 text-[10px] font-bold backdrop-blur-sm px-2.5 py-1.5 rounded-full border shadow-sm',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}
