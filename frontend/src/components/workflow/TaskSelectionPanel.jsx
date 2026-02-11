import PropTypes from 'prop-types';
import { computeSelectionDetails } from './taskSelection';

export default function TaskSelectionPanel({
  tasks,
  selectedTaskIds,
  onChangeSelectedTaskIds,
  disabled = false,
  title = 'Select Tasks To Execute',
}) {
  const taskList = Array.isArray(tasks) ? tasks : [];
  const selected = Array.isArray(selectedTaskIds) ? selectedTaskIds : [];

  const { explicit, effective, required } = computeSelectionDetails(taskList, selected);
  const totalCount = taskList.length;
  const effectiveCount = effective.size;
  const requiredCount = required.size;
  const willSkipCount = taskList.filter(t => (t?.status || 'pending') === 'pending' && !effective.has(t.id)).length;

  const selectAll = () => {
    onChangeSelectedTaskIds(taskList.map(t => t.id).filter(Boolean));
  };

  const clearAll = () => {
    onChangeSelectedTaskIds([]);
  };

  const toggle = (taskId) => {
    if (!taskId) return;
    if (disabled) return;
    if (required.has(taskId)) return;

    const next = new Set(explicit);
    if (next.has(taskId)) next.delete(taskId);
    else next.add(taskId);
    onChangeSelectedTaskIds(Array.from(next));
  };

  return (
    <div className="rounded-lg border border-border bg-card p-3 space-y-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h4 className="text-sm font-medium text-foreground">{title}</h4>
          <p className="text-xs text-muted-foreground mt-1">
            {effectiveCount}/{totalCount} selected
            {requiredCount > 0 ? ` (${requiredCount} required)` : ''}
            {willSkipCount > 0 ? `, ${willSkipCount} will be skipped` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={selectAll}
            disabled={disabled || taskList.length === 0}
            className="px-2 py-1 rounded text-xs bg-primary/10 text-primary hover:bg-primary/20 disabled:opacity-50 transition-colors"
          >
            Select all
          </button>
          <button
            type="button"
            onClick={clearAll}
            disabled={disabled || taskList.length === 0}
            className="px-2 py-1 rounded text-xs bg-muted text-muted-foreground hover:bg-muted/70 disabled:opacity-50 transition-colors"
          >
            Clear
          </button>
        </div>
      </div>

      <div className="space-y-2 max-h-64 overflow-auto pr-1">
        {taskList.length === 0 && (
          <div className="text-xs text-muted-foreground">No tasks available.</div>
        )}

        {taskList.map((task) => {
          const id = task?.id;
          const isChecked = effective.has(id);
          const isRequired = required.has(id);

          return (
            <label
              key={id}
              className={`flex items-start gap-2 p-2 rounded-lg border transition-colors ${
                isChecked ? 'border-primary/30 bg-primary/5' : 'border-border bg-background'
              } ${disabled ? 'opacity-60' : 'hover:bg-accent/20'}`}
            >
              <input
                type="checkbox"
                checked={isChecked}
                disabled={disabled || isRequired}
                onChange={() => toggle(id)}
                aria-label={`Select task ${task?.name || id}`}
                className="mt-0.5 w-4 h-4 rounded border-input"
              />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 min-w-0">
                  {task?.cli && (
                    <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground shrink-0">
                      {task.cli}
                    </span>
                  )}
                  <span className="text-sm font-medium text-foreground truncate">
                    {task?.name || id}
                  </span>
                  {isRequired && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-warning/10 text-warning shrink-0">
                      required
                    </span>
                  )}
                </div>
                {task?.description && (
                  <p className="text-xs text-muted-foreground mt-0.5 truncate">
                    {task.description}
                  </p>
                )}
                {Array.isArray(task?.dependencies) && task.dependencies.length > 0 && (
                  <p className="text-[10px] text-muted-foreground mt-0.5 truncate">
                    Depends on: {task.dependencies.join(', ')}
                  </p>
                )}
              </div>
            </label>
          );
        })}
      </div>
    </div>
  );
}

TaskSelectionPanel.propTypes = {
  tasks: PropTypes.arrayOf(PropTypes.shape({
    id: PropTypes.string.isRequired,
    name: PropTypes.string,
    description: PropTypes.string,
    status: PropTypes.string,
    cli: PropTypes.string,
    dependencies: PropTypes.arrayOf(PropTypes.string),
  })).isRequired,
  selectedTaskIds: PropTypes.arrayOf(PropTypes.string).isRequired,
  onChangeSelectedTaskIds: PropTypes.func.isRequired,
  disabled: PropTypes.bool,
  title: PropTypes.string,
};
