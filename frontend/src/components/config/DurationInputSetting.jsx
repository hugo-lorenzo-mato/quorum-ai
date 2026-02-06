import { useState, useEffect } from 'react';
import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

// Parse duration string to { value, unit }
function parseDuration(str) {
  if (!str) return { value: '', unit: 'h' };

  const match = str.match(/^(\d+(?:\.\d+)?)(h|m|s)$/);
  if (match) {
    return { value: match[1], unit: match[2] };
  }
  return { value: str, unit: 'h' };
}

// Format { value, unit } to duration string
function formatDuration(value, unit) {
  if (!value) return '';
  return `${value}${unit}`;
}

export function DurationInputSetting({
  label,
  description,
  tooltip,
  value,
  onChange,
  disabled = false,
  error,
  required = false,
  id,
  units = ['h', 'm', 's'],
}) {
  const inputId = id || `duration-${label?.replace(/\s+/g, '-').toLowerCase()}`;
  const [parsed, setParsed] = useState(() => parseDuration(value));

  useEffect(() => {
    setParsed(parseDuration(value));
  }, [value]);

  const handleValueChange = (newValue) => {
    setParsed((prev) => ({ ...prev, value: newValue }));
    onChange(formatDuration(newValue, parsed.unit));
  };

  const handleUnitChange = (newUnit) => {
    setParsed((prev) => ({ ...prev, unit: newUnit }));
    onChange(formatDuration(parsed.value, newUnit));
  };

  const unitLabels = {
    h: 'hours',
    m: 'minutes',
    s: 'seconds',
  };

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

      <div className="flex items-center gap-2">
        <input
          id={inputId}
          type="number"
          value={parsed.value}
          onChange={(e) => handleValueChange(e.target.value)}
          disabled={disabled}
          min="0"
          placeholder="0"
          className={`
            flex-1 min-w-[60px] h-10 px-3
            border rounded-lg bg-background text-foreground
            transition-colors
            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
            disabled:opacity-50 disabled:cursor-not-allowed
            [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none
            ${error ? 'border-error' : 'border-input hover:border-muted-foreground'}
          `}
          aria-invalid={!!error}
        />

        <div className="flex rounded-lg border border-input overflow-hidden flex-shrink-0">
          {units.map((unit) => (
            <button
              key={unit}
              type="button"
              onClick={() => handleUnitChange(unit)}
              disabled={disabled}
              className={`
                px-2 sm:px-3 h-10 text-sm font-medium transition-colors
                ${
                  parsed.unit === unit
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-background text-foreground hover:bg-muted'
                }
                disabled:opacity-50 disabled:cursor-not-allowed
              `}
              aria-label={unitLabels[unit]}
            >
              {unit}
            </button>
          ))}
        </div>
      </div>

      <ValidationMessage error={error} />
    </div>
  );
}
