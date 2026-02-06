import { Minus, Plus } from 'lucide-react';
import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function NumberInputSetting({
  label,
  description,
  tooltip,
  value,
  onChange,
  disabled = false,
  error,
  required = false,
  min,
  max,
  step = 1,
  id,
  showStepper = true,
}) {
  const inputId = id || `number-${label?.replace(/\s+/g, '-').toLowerCase()}`;

  const handleIncrement = () => {
    let newValue = (value || 0) + step;
    // Fix floating point precision
    const precision = (step.toString().split('.')[1] || '').length;
    if (precision > 0) {
      newValue = parseFloat(newValue.toFixed(precision));
    }

    if (max === undefined || newValue <= max) {
      onChange(newValue);
    }
  };

  const handleDecrement = () => {
    let newValue = (value || 0) - step;
    // Fix floating point precision
    const precision = (step.toString().split('.')[1] || '').length;
    if (precision > 0) {
      newValue = parseFloat(newValue.toFixed(precision));
    }

    if (min === undefined || newValue >= min) {
      onChange(newValue);
    }
  };

  const handleChange = (e) => {
    const val = e.target.value;
    if (val === '') {
      onChange(null);
    } else {
      const num = parseFloat(val);
      if (!isNaN(num)) {
        onChange(num);
      }
    }
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
        {showStepper && (
          <button
            type="button"
            onClick={handleDecrement}
            disabled={disabled || (min !== undefined && (value || 0) <= min)}
            className="p-2 rounded-lg border border-input bg-background hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            aria-label="Decrease"
          >
            <Minus className="w-4 h-4" />
          </button>
        )}

        <input
          id={inputId}
          type="number"
          value={value ?? ''}
          onChange={handleChange}
          disabled={disabled}
          min={min}
          max={max}
          step={step}
          className={`
            flex-1 h-10 px-3 text-center
            border rounded-lg bg-background text-foreground
            transition-colors
            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
            disabled:opacity-50 disabled:cursor-not-allowed
            [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none
            ${error ? 'border-error' : 'border-input hover:border-muted-foreground'}
          `}
          aria-invalid={!!error}
          aria-describedby={error ? `${inputId}-error` : undefined}
        />

        {showStepper && (
          <button
            type="button"
            onClick={handleIncrement}
            disabled={disabled || (max !== undefined && (value || 0) >= max)}
            className="p-2 rounded-lg border border-input bg-background hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            aria-label="Increase"
          >
            <Plus className="w-4 h-4" />
          </button>
        )}
      </div>

      {(min !== undefined || max !== undefined) && (
        <p className="text-xs text-muted-foreground mt-1">
          {min !== undefined && max !== undefined
            ? `Range: ${min} - ${max}`
            : min !== undefined
            ? `Minimum: ${min}`
            : `Maximum: ${max}`}
        </p>
      )}

      <ValidationMessage error={error} />
    </div>
  );
}
