import { useConfigStore } from '../../stores/configStore';
import { useUIStore } from '../../stores';

export function SettingsToolbar() {
  const isDirty = useConfigStore((state) => state.isDirty);
  const isLoading = useConfigStore((state) => state.isLoading);
  const isSaving = useConfigStore((state) => state.isSaving);
  const validationErrors = useConfigStore((state) => state.validationErrors);
  const saveChanges = useConfigStore((state) => state.saveChanges);
  const discardChanges = useConfigStore((state) => state.discardChanges);
  const sidebarOpen = useUIStore((state) => state.sidebarOpen);

  const errorCount = Object.keys(validationErrors).length;
  const hasErrors = errorCount > 0;

  if (!isDirty) {
    return null;
  }

  return (
    <div
      className={`fixed bottom-0 right-0 z-50 border-t border-border bg-background/80 glass shadow-lg md:${
        sidebarOpen ? 'left-64' : 'left-16'
      } left-0`}
    >
      <div className="px-3 sm:px-6 py-2 sm:py-3 flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2 sm:gap-4">
        <div className="flex items-center gap-2 sm:gap-3 min-w-0">
          <div className="w-2 h-2 flex-shrink-0 rounded-full bg-warning animate-pulse" />
          <span className="text-xs sm:text-sm text-muted-foreground truncate">
            You have unsaved changes
          </span>
          {hasErrors && (
            <span className="text-xs sm:text-sm text-error flex-shrink-0">
              ({errorCount} validation error{errorCount > 1 ? 's' : ''})
            </span>
          )}
        </div>

        <div className="flex items-center gap-2 sm:gap-3">
          <button
            onClick={discardChanges}
            disabled={isLoading || isSaving}
            className="flex-1 sm:flex-none px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium text-foreground bg-muted hover:bg-muted/80 rounded-lg transition-colors disabled:opacity-50 disabled:pointer-events-none"
            type="button"
          >
            Discard
          </button>
          <button
            onClick={saveChanges}
            disabled={isLoading || isSaving || hasErrors}
            className="flex-1 sm:flex-none px-3 sm:px-4 py-1.5 sm:py-2 text-xs sm:text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-50 disabled:pointer-events-none"
            type="button"
          >
            {isSaving ? (
              <>
                <svg className="animate-spin h-3 sm:h-4 w-3 sm:w-4" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                <span className="hidden sm:inline">Saving...</span>
                <span className="sm:hidden">Save</span>
              </>
            ) : (
              'Save Changes'
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

export default SettingsToolbar;
