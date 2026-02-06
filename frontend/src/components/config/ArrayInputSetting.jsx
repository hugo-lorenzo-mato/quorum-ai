import { useState } from 'react';
import { Plus, X } from 'lucide-react';
import { InfoTooltip } from './Tooltip';
import { ValidationMessage } from './ValidationMessage';

export function ArrayInputSetting({
  label,
  description,
  tooltip,
  value = [],
  onChange,
  disabled = false,
  error,
  required = false,
  placeholder = 'Add item...',
  id,
  maxItems,
  suggestions = [],
}) {
  const inputId = id || `array-${label?.replace(/\s+/g, '-').toLowerCase()}`;
  const [newItem, setNewItem] = useState('');

  // Ensure items is always an array
  const items = Array.isArray(value) ? value : [];

  const handleAdd = () => {
    if (newItem.trim() && !items.includes(newItem.trim())) {
      onChange([...items, newItem.trim()]);
      setNewItem('');
    }
  };

  const handleRemove = (index) => {
    const newValue = [...items];
    newValue.splice(index, 1);
    onChange(newValue);
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAdd();
    }
  };

  const canAdd = !disabled && (!maxItems || items.length < maxItems);

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
        {maxItems && (
          <span className="text-xs text-muted-foreground">
            {items.length} / {maxItems}
          </span>
        )}
      </div>

      {description && (
        <p className="text-xs text-muted-foreground mb-2">{description}</p>
      )}

      {/* Current items */}
      {items.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-2">
          {items.map((item, index) => (
            <span
              key={index}
              className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-muted text-sm"
            >
              <span className="font-mono">{item}</span>
              {!disabled && (
                <button
                  type="button"
                  onClick={() => handleRemove(index)}
                  className="p-0.5 hover:bg-muted-foreground/20 rounded transition-colors"
                  aria-label={`Remove ${item}`}
                >
                  <X className="w-3 h-3" />
                </button>
              )}
            </span>
          ))}
        </div>
      )}

      {/* Add new item */}
      <div className="flex items-center gap-2">
        <input
          id={inputId}
          type="text"
          value={newItem}
          onChange={(e) => setNewItem(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={!canAdd}
          placeholder={placeholder}
          list={suggestions.length > 0 ? `${inputId}-suggestions` : undefined}
          className={`
            flex-1 h-10 px-3
            border rounded-lg bg-background text-foreground
            transition-colors
            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
            disabled:opacity-50 disabled:cursor-not-allowed
            placeholder:text-muted-foreground
            ${error ? 'border-error' : 'border-input hover:border-muted-foreground'}
          `}
        />
        {suggestions.length > 0 && (
          <datalist id={`${inputId}-suggestions`}>
            {suggestions.map((s) => (
              <option key={s} value={s} />
            ))}
          </datalist>
        )}
        <button
          type="button"
          onClick={handleAdd}
          disabled={!canAdd || !newItem.trim()}
          className="p-2 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          aria-label="Add item"
        >
          <Plus className="w-5 h-5" />
        </button>
      </div>

      <ValidationMessage error={error} />
    </div>
  );
}
