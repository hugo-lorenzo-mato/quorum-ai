export function SettingSection({ title, description, children, className = '', variant = 'default' }) {
  const variantClasses = {
    default: 'border-border bg-card',
    danger: 'border-destructive/30 bg-destructive/5',
  };

  const styleClasses = variantClasses[variant] || variantClasses.default;

  return (
    <section className={`p-6 rounded-xl border ${styleClasses} ${className}`}>
      {(title || description) && (
        <div className="mb-4">
          {title && (
            <h3 className="text-lg font-semibold text-foreground">{title}</h3>
          )}
          {description && (
            <p className="text-sm text-muted-foreground mt-1">{description}</p>
          )}
        </div>
      )}
      {children}
    </section>
  );
}

export function SettingRow({ children, className = '' }) {
  return (
    <div className={`py-3 border-b border-border last:border-0 ${className}`}>
      {children}
    </div>
  );
}
