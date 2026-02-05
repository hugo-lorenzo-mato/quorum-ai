import { ChevronDown } from 'lucide-react';
import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function SelectSetting({
  label,
  description,
  tooltip,
  value,
  onChange,
  options,
  disabled = false,
  error,
  required = false,
  placeholder = 'Select...',
  id,
}) {
  const inputId = id || `select-${label?.replace(/\s+/g, '-').toLowerCase()}`;

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

      <div className="relative">
        <select
          id={inputId}
          value={value || ''}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className={`
            w-full h-10 px-3 pr-10 appearance-none
            border rounded-lg bg-background text-foreground
            transition-colors cursor-pointer
            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
            disabled:opacity-50 disabled:cursor-not-allowed
            ${error ? 'border-error' : 'border-input hover:border-muted-foreground'}
          `}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
        >
          {placeholder && (
            <option value="" disabled hidden>
              {placeholder}
            </option>
          )}
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.label}
            </option>
          ))}
        </select>
        <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
      </div>

      <ValidationMessage error={error} />
    </div>
  );
}
