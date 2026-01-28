import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export default function ToggleSetting({
  label,
  description,
  helperText,
  tooltip,
  checked,
  onChange,
  disabled = false,
  error,
  dangerLevel, // 'warning' | 'danger'
  id,
  compact = false,
}) {
  const inputId = id || `toggle-${label?.replace(/\s+/g, '-').toLowerCase()}`;

  const dangerStyles = {
    warning: 'border-warning/50 bg-warning/5',
    danger: 'border-error/50 bg-error/5',
  };

  const switchButton = (
    <button
      id={inputId}
      role="switch"
      aria-checked={checked}
      onClick={() => !disabled && onChange(!checked)}
      disabled={disabled}
      aria-label={label || 'Toggle'}
      className={`
        relative w-11 h-6 rounded-full transition-colors border
        focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:ring-offset-background
        disabled:cursor-not-allowed
        ${checked ? 'bg-primary border-primary/20' : 'bg-muted border-border'}
      `}
    >
      <span
        className={`
          absolute top-1 left-1 w-4 h-4 rounded-full bg-background border border-border
          transition-transform shadow-sm
          ${checked ? 'translate-x-5' : ''}
        `}
      />
    </button>
  );

  if (compact) {
    return switchButton;
  }

  return (
    <div
      className={`flex items-center justify-between py-3 ${
        dangerLevel ? `px-3 -mx-3 rounded-lg ${dangerStyles[dangerLevel]}` : ''
      }`}
    >
      <div className="flex-1">
        <div className="flex items-center gap-2">
          <label
            htmlFor={inputId}
            className={`text-sm font-medium ${
              disabled ? 'text-muted-foreground' : 'text-foreground'
            }`}
          >
            {label}
          </label>
          {tooltip && <InfoTooltip content={tooltip} />}
        </div>
        {description && (
          <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
        )}
        {helperText && (
          <p className="text-xs text-muted-foreground mt-1">{helperText}</p>
        )}
        <ValidationMessage error={error} />
      </div>

      {switchButton}
    </div>
  );
}
