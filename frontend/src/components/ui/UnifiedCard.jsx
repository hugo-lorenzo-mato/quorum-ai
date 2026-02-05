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

import { cn } from '../lib/utils';

/**
 * Base Card Container
 * Foundation for all card types with Vercel aesthetics
 */
export function CardBase({ 
  children, 
  onClick, 
  className,
  variant = 'default', // 'default' | 'selected' | 'executing' | 'completed' | 'failed'
  hoverable = true,
  ...props 
}) {
  const variantStyles = {
    default: 'border-border/50 bg-gradient-to-br from-card via-card to-card hover:border-primary/30',
    selected: 'border-primary/40 bg-primary/5 ring-2 ring-primary/20',
    executing: 'border-blue-500/40 bg-gradient-to-br from-blue-500/5 via-card to-card shadow-lg shadow-blue-500/10 ring-1 ring-blue-500/20',
    completed: 'border-border/50 bg-gradient-to-br from-card via-card to-emerald-500/5 hover:border-emerald-500/30',
    failed: 'border-border/50 bg-gradient-to-br from-card via-card to-rose-500/5 hover:border-rose-500/30',
  };

  const hoverStyles = hoverable 
    ? 'hover:shadow-lg hover:-translate-y-0.5 active:scale-[0.98]' 
    : '';

  return (
    <div
      onClick={onClick}
      className={cn(
        'relative flex flex-col rounded-xl border transition-all duration-300 backdrop-blur-sm shadow-sm',
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
    success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20',
    warning: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20',
    error: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20',
    info: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20',
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
 * Card Accent Line
 * Top decorative line indicating status/category
 */
export function CardAccent({ className, color = 'primary', ...props }) {
  const colorMap = {
    primary: 'bg-gradient-to-r from-transparent via-primary to-transparent',
    blue: 'bg-gradient-to-r from-transparent via-blue-500 to-transparent',
    emerald: 'bg-gradient-to-r from-transparent via-emerald-500 to-transparent',
    rose: 'bg-gradient-to-r from-transparent via-rose-500 to-transparent',
    amber: 'bg-gradient-to-r from-transparent via-amber-500 to-transparent',
    violet: 'bg-gradient-to-r from-transparent via-violet-500 to-transparent',
  };

  return (
    <div 
      className={cn(
        'absolute top-0 left-4 right-4 h-0.5 rounded-full',
        colorMap[color] || colorMap.primary,
        className
      )}
      {...props}
    />
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
    blue: 'bg-blue-500/10 border-blue-500/20 text-blue-500',
    emerald: 'bg-emerald-500/10 border-emerald-500/20 text-emerald-500',
    rose: 'bg-rose-500/10 border-rose-500/20 text-rose-500',
    amber: 'bg-amber-500/10 border-amber-500/20 text-amber-500',
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
