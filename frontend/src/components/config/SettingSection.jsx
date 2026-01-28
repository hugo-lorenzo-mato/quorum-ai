export function SettingSection({ title, description, children, className = '' }) {
  return (
    <div className={`p-6 rounded-xl border border-border bg-card ${className}`}>
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
    </div>
  );
}

export function SettingRow({ children, className = '' }) {
  return (
    <div className={`py-3 border-b border-border last:border-0 ${className}`}>
      {children}
    </div>
  );
}
