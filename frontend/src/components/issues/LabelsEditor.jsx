import { useState } from 'react';
import { Tag, X, Plus } from 'lucide-react';

/**
 * Inline editor for managing issue labels.
 * Shows chips with add/remove functionality.
 */
export default function LabelsEditor({
  labels = [],
  onChange,
  disabled = false,
  compact = false,
}) {
  const [isAdding, setIsAdding] = useState(false);
  const [newLabel, setNewLabel] = useState('');

  const handleAdd = () => {
    const trimmed = newLabel.trim();
    if (trimmed && !labels.includes(trimmed)) {
      onChange([...labels, trimmed]);
    }
    setNewLabel('');
    setIsAdding(false);
  };

  const handleRemove = (labelToRemove) => {
    onChange(labels.filter(l => l !== labelToRemove));
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAdd();
    } else if (e.key === 'Escape') {
      setNewLabel('');
      setIsAdding(false);
    }
  };

  return (
    <div className={`flex items-center gap-3 ${compact ? "" : "space-y-2 flex-wrap"}`}>
      <label className={`flex items-center gap-1.5 shrink-0 ${compact ? 'text-[10px] font-bold uppercase text-muted-foreground/70 tracking-wider' : 'text-sm font-medium text-foreground'}`}>
        <Tag className={`${compact ? 'w-3 h-3' : 'w-4 h-4'} text-muted-foreground`} />
        Labels
      </label>

      <div className="flex flex-wrap gap-1.5 items-center">
        {labels.map((label) => (
          <span
            key={label}
            className={`inline-flex items-center gap-1 ${compact ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'} bg-primary/10 text-primary rounded-full font-medium`}
          >
            {label}
            {!disabled && (
              <button
                onClick={() => handleRemove(label)}
                className="p-0.5 rounded-full hover:bg-primary/20 transition-colors"
                aria-label={`Remove ${label}`}
              >
                <X className={compact ? 'w-2.5 h-2.5' : 'w-3 h-3'} />
              </button>
            )}
          </span>
        ))}

        {/* Add button or input */}
        {!disabled && (
          isAdding ? (
            <div className="inline-flex items-center">
              <input
                type="text"
                value={newLabel}
                onChange={(e) => setNewLabel(e.target.value)}
                onKeyDown={handleKeyDown}
                onBlur={handleAdd}
                placeholder="Name..."
                autoFocus
                className={`w-20 ${compact ? 'px-2 py-0.5 text-xs' : 'px-2 py-1 text-sm'} bg-muted border border-border rounded-l-full focus:outline-none focus:ring-1 focus:ring-primary/50`}
              />
              <button
                onClick={handleAdd}
                className={`${compact ? 'px-2 py-0.5 text-xs' : 'px-2 py-1 text-sm'} bg-primary text-primary-foreground rounded-r-full hover:bg-primary/90 transition-colors font-medium`}
              >
                Add
              </button>
            </div>
          ) : (
            <button
              onClick={() => setIsAdding(true)}
              className={`inline-flex items-center gap-1 ${compact ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'} text-muted-foreground border border-dashed border-border rounded-full hover:border-primary hover:text-primary transition-colors`}
            >
              <Plus className={compact ? 'w-2.5 h-2.5' : 'w-3 h-3'} />
              Add
            </button>
          )
        )}
      </div>
    </div>
  );
}
