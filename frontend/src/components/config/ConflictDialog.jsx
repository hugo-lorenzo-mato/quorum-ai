import { useConfigStore } from '../../stores/configStore';

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-full bg-yellow-100 dark:bg-yellow-900 flex items-center justify-center">
            <svg className="w-6 h-6 text-yellow-600 dark:text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
              Configuration Conflict
            </h3>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              The configuration was modified elsewhere
            </p>
          </div>
        </div>

        <p className="text-gray-600 dark:text-gray-300 mb-6">
          The configuration file has been modified by another process (CLI or another browser tab).
          How would you like to proceed?
        </p>

        <div className="space-y-3">
          <button
            onClick={handleReload}
            className="w-full px-4 py-3 text-left rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
          >
            <div className="font-medium text-gray-900 dark:text-white">
              Reload Latest
            </div>
            <div className="text-sm text-gray-500 dark:text-gray-400">
              Discard your changes and load the latest configuration
            </div>
          </button>

          <button
            onClick={handleOverwrite}
            className="w-full px-4 py-3 text-left rounded-lg border border-red-200 dark:border-red-800 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
          >
            <div className="font-medium text-red-700 dark:text-red-300">
              Overwrite
            </div>
            <div className="text-sm text-red-600 dark:text-red-400">
              Force save your changes, discarding external modifications
            </div>
          </button>

          <button
            onClick={handleCancel}
            className="w-full px-4 py-2 text-center text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

export default ConflictDialog;
