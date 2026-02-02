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
    <div className="space-y-2">
      <label className="flex items-center gap-2 text-sm font-medium text-foreground">
        <Tag className="w-4 h-4 text-muted-foreground" />
        Labels
      </label>

      <div className="flex flex-wrap gap-2">
        {labels.map((label) => (
          <span
            key={label}
            className="inline-flex items-center gap-1 px-2.5 py-1 text-sm bg-primary/10 text-primary rounded-full"
          >
            {label}
            {!disabled && (
              <button
                onClick={() => handleRemove(label)}
                className="p-0.5 rounded-full hover:bg-primary/20 transition-colors"
                aria-label={`Remove ${label}`}
              >
                <X className="w-3 h-3" />
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
                placeholder="Label name"
                autoFocus
                className="w-24 px-2 py-1 text-sm bg-muted border border-border rounded-l-full focus:outline-none focus:ring-2 focus:ring-primary/50"
              />
              <button
                onClick={handleAdd}
                className="px-2 py-1 text-sm bg-primary text-primary-foreground rounded-r-full hover:bg-primary/90 transition-colors"
              >
                Add
              </button>
            </div>
          ) : (
            <button
              onClick={() => setIsAdding(true)}
              className="inline-flex items-center gap-1 px-2.5 py-1 text-sm text-muted-foreground border border-dashed border-border rounded-full hover:border-primary hover:text-primary transition-colors"
            >
              <Plus className="w-3 h-3" />
              Add
            </button>
          )
        )}
      </div>
    </div>
  );
}
