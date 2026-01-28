import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function SliderSetting({
  label,
  description,
  tooltip,
  value,
  onChange,
  disabled = false,
  error,
  required = false,
  min = 0,
  max = 1,
  step = 0.01,
  id,
  formatValue = (v) => (v * 100).toFixed(0) + '%',
}) {
  const inputId = id || `slider-${label?.replace(/\s+/g, '-').toLowerCase()}`;

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
        <span className="text-sm font-medium text-foreground tabular-nums">
          {formatValue(value ?? min)}
        </span>
      </div>

      {description && (
        <p className="text-xs text-muted-foreground mb-2">{description}</p>
      )}

      <input
        id={inputId}
        type="range"
        value={value ?? min}
        onChange={(e) => onChange(parseFloat(e.target.value))}
        disabled={disabled}
        min={min}
        max={max}
        step={step}
        className={`
          w-full h-2 rounded-full appearance-none cursor-pointer
          bg-muted
          [&::-webkit-slider-thumb]:appearance-none
          [&::-webkit-slider-thumb]:w-4
          [&::-webkit-slider-thumb]:h-4
          [&::-webkit-slider-thumb]:rounded-full
          [&::-webkit-slider-thumb]:bg-primary
          [&::-webkit-slider-thumb]:shadow-md
          [&::-webkit-slider-thumb]:transition-transform
          [&::-webkit-slider-thumb]:hover:scale-110
          [&::-moz-range-thumb]:w-4
          [&::-moz-range-thumb]:h-4
          [&::-moz-range-thumb]:rounded-full
          [&::-moz-range-thumb]:bg-primary
          [&::-moz-range-thumb]:border-0
          [&::-moz-range-thumb]:shadow-md
          disabled:opacity-50 disabled:cursor-not-allowed
        `}
        aria-invalid={!!error}
      />

      <div className="flex justify-between text-xs text-muted-foreground mt-1">
        <span>{formatValue(min)}</span>
        <span>{formatValue(max)}</span>
      </div>

      <ValidationMessage error={error} />
    </div>
  );
}
