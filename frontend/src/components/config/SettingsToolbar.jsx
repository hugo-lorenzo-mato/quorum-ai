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
      className={`fixed bottom-0 right-0 z-50 border-t border-border bg-background/80 glass shadow-lg ${
        sidebarOpen ? 'left-64' : 'left-16'
      }`}
    >
      <div className="px-6 py-3 flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="w-2 h-2 rounded-full bg-warning animate-pulse" />
          <span className="text-sm text-muted-foreground">
            You have unsaved changes
          </span>
          {hasErrors && (
            <span className="text-sm text-error">
              ({errorCount} validation error{errorCount > 1 ? 's' : ''})
            </span>
          )}
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={discardChanges}
            disabled={isLoading || isSaving}
            className="px-4 py-2 text-sm font-medium text-foreground bg-muted hover:bg-muted/80 rounded-lg transition-colors disabled:opacity-50 disabled:pointer-events-none"
            type="button"
          >
            Discard
          </button>
          <button
            onClick={saveChanges}
            disabled={isLoading || isSaving || hasErrors}
            className="px-4 py-2 text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 rounded-lg transition-colors flex items-center gap-2 disabled:opacity-50 disabled:pointer-events-none"
            type="button"
          >
            {isSaving ? (
              <>
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Saving...
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
