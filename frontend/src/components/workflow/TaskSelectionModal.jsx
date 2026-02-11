import { useEffect, useMemo, useState } from 'react';
import PropTypes from 'prop-types';
import { X, Play, Loader2 } from 'lucide-react';
import TaskSelectionPanel from './TaskSelectionPanel';
import { computeSelectionDetails } from './taskSelection';

export default function TaskSelectionModal({ isOpen, onClose, onConfirm, tasks, loading }) {
  const taskList = Array.isArray(tasks) ? tasks : [];
  const [selectedTaskIds, setSelectedTaskIds] = useState([]);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      setInitialized(false);
      setSelectedTaskIds([]);
      return;
    }
    if (!initialized && taskList.length > 0) {
      setSelectedTaskIds(taskList.map(t => t.id).filter(Boolean));
      setInitialized(true);
    }
  }, [isOpen, initialized, taskList]);

  const { effective } = useMemo(
    () => computeSelectionDetails(taskList, selectedTaskIds),
    [taskList, selectedTaskIds],
  );
  const canConfirm = effective.size > 0 && !loading;

  if (!isOpen) return null;

  const handleConfirm = async () => {
    if (!canConfirm) return;
    await onConfirm(selectedTaskIds);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <button
        type="button"
        className="absolute inset-0 bg-black/50"
        onClick={onClose}
        aria-label="Close modal"
      />

      <div className="relative w-full max-w-2xl mx-4 bg-card rounded-xl shadow-lg border border-border">
        <div className="flex items-center justify-between p-4 border-b border-border">
          <h3 className="text-lg font-semibold text-foreground">Execute Selection</h3>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-muted transition-colors"
          >
            <X className="w-5 h-5 text-muted-foreground" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          <p className="text-sm text-muted-foreground">
            Choose which tasks to execute. Dependencies will be auto-included and unselected tasks will be marked as skipped.
          </p>
          <TaskSelectionPanel
            tasks={taskList}
            selectedTaskIds={selectedTaskIds}
            onChangeSelectedTaskIds={setSelectedTaskIds}
            disabled={loading}
          />
        </div>

        <div className="flex items-center justify-end gap-2 p-4 border-t border-border">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-muted-foreground hover:bg-muted transition-colors"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleConfirm}
            disabled={!canConfirm}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-success/10 text-success hover:bg-success/20 disabled:opacity-50 transition-all"
          >
            {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
            Execute
          </button>
        </div>
      </div>
    </div>
  );
}

TaskSelectionModal.propTypes = {
  isOpen: PropTypes.bool.isRequired,
  onClose: PropTypes.func.isRequired,
  onConfirm: PropTypes.func.isRequired,
  tasks: PropTypes.arrayOf(PropTypes.shape({
    id: PropTypes.string.isRequired,
    name: PropTypes.string,
    description: PropTypes.string,
    status: PropTypes.string,
    cli: PropTypes.string,
    dependencies: PropTypes.arrayOf(PropTypes.string),
  })).isRequired,
  loading: PropTypes.bool,
};

