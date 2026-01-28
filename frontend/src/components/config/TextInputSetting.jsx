import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function TextInputSetting({
  label,
  description,
  tooltip,
  value,
  onChange,
  suggestions,
  disabled = false,
  error,
  required = false,
  placeholder = '',
  type = 'text',
  id,
  autoComplete = 'off',
}) {
  const inputId = id || `input-${label?.replace(/\s+/g, '-').toLowerCase()}`;
  const datalistId = suggestions?.length ? `${inputId}-list` : undefined;

  return (
    <div className="py-3">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <label
            htmlFor={inputId}
            className={`text-sm font-medium ${
              disabled ? 'text-muted-foreground' : 'text-foreground'
            }`}
          >
            {label}
            {required && <span className="text-error ml-1">*</span>}
          </label>
          {tooltip && <InfoTooltip content={tooltip} />}
        </div>
      </div>

      {description && (
        <p className="text-xs text-muted-foreground mb-2">{description}</p>
      )}

      <input
        id={inputId}
        type={type}
        value={value || ''}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        placeholder={placeholder}
        autoComplete={autoComplete}
        list={datalistId}
        className={`
          w-full h-10 px-3
          border rounded-lg bg-background text-foreground
          transition-colors
          focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
          disabled:opacity-50 disabled:cursor-not-allowed
          placeholder:text-muted-foreground
          ${error ? 'border-error' : 'border-input hover:border-muted-foreground'}
        `}
        aria-invalid={!!error}
        aria-describedby={error ? `${inputId}-error` : undefined}
      />

      {datalistId && (
        <datalist id={datalistId}>
          {Array.from(new Set(suggestions)).map((opt) => (
            <option key={opt} value={opt} />
          ))}
        </datalist>
      )}

      <ValidationMessage error={error} />
    </div>
  );
}
