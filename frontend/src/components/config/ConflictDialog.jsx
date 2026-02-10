import { useConfigStore } from '../../stores/configStore';
import { AlertTriangle, X } from 'lucide-react';

export function ConflictDialog() {
  const hasConflict = useConfigStore((state) => state.hasConflict);
  const acceptServerVersion = useConfigStore((state) => state.acceptServerVersion);
  const forceSave = useConfigStore((state) => state.forceSave);

  const handleReload = () => {
    acceptServerVersion();
  };

  const handleOverwrite = () => {
    forceSave();
  };

  const handleCancel = () => {
    // Clear the conflict state without taking action
    useConfigStore.setState({ hasConflict: false, conflictConfig: null, conflictEtag: null });
  };

  if (!hasConflict) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in">
      <button
        type="button"
        className="absolute inset-0 bg-black/50"
        onClick={handleCancel}
        aria-label="Close dialog"
      />
      <div
        className="relative bg-card border border-border rounded-xl shadow-xl max-w-md w-full p-6 animate-fade-up"
        role="dialog"
        aria-modal="true"
        aria-labelledby="conflict-dialog-title"
      >
        <div className="flex items-start gap-4">
          <div className="p-2 rounded-full bg-warning/10 text-warning">
            <AlertTriangle className="w-6 h-6" />
          </div>
          <div className="flex-1">
            <h3 id="conflict-dialog-title" className="text-lg font-semibold text-foreground">
              Configuration Conflict
            </h3>
            <p className="mt-1 text-sm text-muted-foreground">
              The configuration was modified elsewhere
            </p>
          </div>
          <button
            onClick={handleCancel}
            className="p-1 text-muted-foreground hover:text-foreground transition-colors rounded"
            aria-label="Close"
            type="button"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        <p className="text-sm text-muted-foreground mt-4 mb-6">
          The configuration file has been modified by another process (CLI or another browser tab).
          How would you like to proceed?
        </p>

        <div className="flex flex-col gap-3">
          <button
            onClick={handleReload}
            className="w-full px-4 py-2.5 rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
            type="button"
          >
            Reload Latest
          </button>

          <button
            onClick={handleOverwrite}
            className="w-full px-4 py-2.5 rounded-lg bg-destructive text-destructive-foreground hover:bg-destructive/90 transition-colors"
            type="button"
          >
            Overwrite
          </button>

          <button
            onClick={handleCancel}
            className="w-full px-4 py-2.5 rounded-lg bg-muted text-foreground hover:bg-muted/80 transition-colors"
            type="button"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

export default ConflictDialog;
