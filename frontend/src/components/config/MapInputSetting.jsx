import { useState } from 'react';
import { Plus, X } from 'lucide-react';
import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function MapInputSetting({
  label,
  description,
  tooltip,
  value = {},
  onChange,
  disabled = false,
  error,
  required = false,
  keyOptions, // For select-based keys
  valueOptions, // For select-based values
  keyPlaceholder = 'Key',
  valuePlaceholder = 'Value',
  id,
}) {
  const inputId = id || `map-${label?.replace(/\s+/g, '-').toLowerCase()}`;
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  const handleAdd = () => {
    if (newKey && newValue && !value[newKey]) {
      onChange({ ...value, [newKey]: newValue });
      setNewKey('');
      setNewValue('');
    }
  };

  const handleRemove = (key) => {
    const newMap = { ...value };
    delete newMap[key];
    onChange(newMap);
  };

  const handleValueChange = (key, newVal) => {
    onChange({ ...value, [key]: newVal });
  };

  const entries = Object.entries(value);

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

      {/* Existing entries */}
      {entries.length > 0 && (
        <div className="space-y-2 mb-2">
          {entries.map(([key, val]) => (
            <div key={key} className="flex items-center gap-2">
              <span className="px-2 py-1 bg-muted rounded text-sm font-mono min-w-[80px]">
                {key}
              </span>
              <span className="text-muted-foreground">=</span>
              {valueOptions ? (
                <select
                  value={val}
                  onChange={(e) => handleValueChange(key, e.target.value)}
                  disabled={disabled}
                  className="flex-1 h-8 px-2 border rounded bg-background text-foreground text-sm"
                >
                  {valueOptions.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  value={val}
                  onChange={(e) => handleValueChange(key, e.target.value)}
                  disabled={disabled}
                  className="flex-1 h-8 px-2 border rounded bg-background text-foreground text-sm"
                />
              )}
              {!disabled && (
                <button
                  type="button"
                  onClick={() => handleRemove(key)}
                  className="p-1 text-muted-foreground hover:text-error transition-colors"
                  aria-label={`Remove ${key}`}
                >
                  <X className="w-4 h-4" />
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Add new entry */}
      <div className="flex items-center gap-2">
        {keyOptions ? (
          <select
            id={inputId}
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            disabled={disabled}
            className="w-32 h-10 px-2 border rounded-lg bg-background text-foreground"
          >
            <option value="">{keyPlaceholder}</option>
            {keyOptions
              .filter((opt) => !value[opt.value])
              .map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
          </select>
        ) : (
          <input
            id={inputId}
            type="text"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            disabled={disabled}
            placeholder={keyPlaceholder}
            className="w-32 h-10 px-3 border rounded-lg bg-background text-foreground"
          />
        )}

        {valueOptions ? (
          <select
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            disabled={disabled}
            className="flex-1 h-10 px-2 border rounded-lg bg-background text-foreground"
          >
            <option value="">{valuePlaceholder}</option>
            {valueOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        ) : (
          <input
            type="text"
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            disabled={disabled}
            placeholder={valuePlaceholder}
            className="flex-1 h-10 px-3 border rounded-lg bg-background text-foreground"
          />
        )}

        <button
          type="button"
          onClick={handleAdd}
          disabled={disabled || !newKey || !newValue}
          className="p-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          aria-label="Add entry"
        >
          <Plus className="w-5 h-5" />
        </button>
      </div>

      <ValidationMessage error={error} />
    </div>
  );
}
