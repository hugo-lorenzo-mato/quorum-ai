import { useState } from 'react';
import { X, RefreshCw, Loader2 } from 'lucide-react';

export default function ReplanModal({ isOpen, onClose, onSubmit, loading }) {
  const [context, setContext] = useState('');

  if (!isOpen) return null;

  const handleSubmit = (e) => {
    e.preventDefault();
    onSubmit(context);
  };

  const handleClose = () => {
    setContext('');
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <button
        type="button"
        className="absolute inset-0 bg-black/50"
        onClick={handleClose}
        aria-label="Close modal"
      />

      {/* Modal */}
      <div className="relative w-full max-w-lg mx-4 bg-card rounded-xl shadow-lg border border-border">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          <h3 className="text-lg font-semibold text-foreground">Replan Workflow</h3>
          <button
            onClick={handleClose}
            className="p-1 rounded-lg hover:bg-muted transition-colors"
          >
            <X className="w-5 h-5 text-muted-foreground" />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit}>
          <div className="p-4 space-y-4">
            <p className="text-sm text-muted-foreground">
              Clear the existing plan and regenerate tasks. Optionally provide additional
              context to influence the new plan.
            </p>

            <div>
              <label
                htmlFor="replan-context"
                className="block text-sm font-medium text-foreground mb-1"
              >
                Additional Context (optional)
              </label>
              <textarea
                id="replan-context"
                value={context}
                onChange={(e) => setContext(e.target.value)}
                rows={4}
                className="w-full px-3 py-2 rounded-lg border border-border bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary resize-none"
                placeholder="e.g., Focus on performance optimization, avoid adding new dependencies..."
              />
            </div>

            <div className="text-xs text-muted-foreground">
              <strong>Examples:</strong>
              <ul className="mt-1 list-disc list-inside space-y-1">
                <li>Focus on security aspects</li>
                <li>Prioritize test coverage</li>
                <li>Use existing patterns from the codebase</li>
                <li>Avoid breaking changes</li>
              </ul>
            </div>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-end gap-2 p-4 border-t border-border">
            <button
              type="button"
              onClick={handleClose}
              className="px-4 py-2 rounded-lg text-muted-foreground hover:bg-muted transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-warning text-warning-foreground hover:bg-warning/90 disabled:opacity-50 transition-all"
            >
              {loading ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <RefreshCw className="w-4 h-4" />
              )}
              Replan
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
