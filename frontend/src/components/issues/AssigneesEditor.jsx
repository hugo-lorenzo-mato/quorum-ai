import { useState } from 'react';
import { Users, X, Plus, User } from 'lucide-react';

/**
 * Inline editor for managing issue assignees.
 * Shows user chips with add/remove functionality.
 */
export default function AssigneesEditor({
  assignees = [],
  onChange,
  disabled = false,
}) {
  const [isAdding, setIsAdding] = useState(false);
  const [newAssignee, setNewAssignee] = useState('');

  const handleAdd = () => {
    const trimmed = newAssignee.trim().replace(/^@/, ''); // Remove @ prefix if present
    if (trimmed && !assignees.includes(trimmed)) {
      onChange([...assignees, trimmed]);
    }
    setNewAssignee('');
    setIsAdding(false);
  };

  const handleRemove = (assigneeToRemove) => {
    onChange(assignees.filter(a => a !== assigneeToRemove));
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAdd();
    } else if (e.key === 'Escape') {
      setNewAssignee('');
      setIsAdding(false);
    }
  };

  return (
    <div className="space-y-2">
      <label className="flex items-center gap-2 text-sm font-medium text-foreground">
        <Users className="w-4 h-4 text-muted-foreground" />
        Assignees
      </label>

      <div className="flex flex-wrap gap-2">
        {assignees.map((assignee) => (
          <span
            key={assignee}
            className="inline-flex items-center gap-1.5 px-2.5 py-1 text-sm bg-muted text-foreground rounded-full"
          >
            <User className="w-3 h-3 text-muted-foreground" />
            @{assignee}
            {!disabled && (
              <button
                onClick={() => handleRemove(assignee)}
                className="p-0.5 rounded-full hover:bg-foreground/10 transition-colors"
                aria-label={`Remove ${assignee}`}
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
                value={newAssignee}
                onChange={(e) => setNewAssignee(e.target.value)}
                onKeyDown={handleKeyDown}
                onBlur={handleAdd}
                placeholder="@username"
                autoFocus
                className="w-28 px-2 py-1 text-sm bg-muted border border-border rounded-l-full focus:outline-none focus:ring-2 focus:ring-primary/50"
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

        {/* Empty state */}
        {assignees.length === 0 && !isAdding && disabled && (
          <span className="text-sm text-muted-foreground">
            No assignees
          </span>
        )}
      </div>
    </div>
  );
}
