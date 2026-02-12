import { useEffect } from 'react';
import { Zap, Sparkles, X, Clock, FileText } from 'lucide-react';
import { Button } from '../ui/Button';

/**
 * Modal for selecting issue generation mode.
 * - Quorum Only: Fast mode, direct markdown copy (~12ms)
 * - AI Enhanced: Slow mode, LLM reformats content (several minutes)
 */
export default function GenerationOptionsModal({
  isOpen,
  onClose,
  onSelect,
  loading = false,
}) {
  // Handle escape key
  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === 'Escape' && isOpen && !loading) {
        onClose();
      }
    };
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose, loading]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[120] flex items-center justify-center p-4">
      {/* Backdrop */}
      <button
        type="button"
        className="absolute inset-0 bg-background/80 backdrop-blur-sm animate-fade-in"
        onClick={() => !loading && onClose()}
        aria-label="Close modal"
        disabled={loading}
      />

      {/* Modal */}
      <div className="relative w-full max-w-lg bg-card border border-border rounded-xl shadow-2xl animate-fade-up">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          <div className="flex items-center gap-2">
            <FileText className="w-5 h-5 text-primary" />
            <h2 className="text-lg font-semibold text-foreground">Generate Issues</h2>
          </div>
          <button
            onClick={onClose}
            disabled={loading}
            className="p-1.5 rounded-lg hover:bg-muted transition-colors disabled:opacity-50"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-3">
          <p className="text-sm text-muted-foreground mb-4">
            Choose how to generate issues from your workflow artifacts.
          </p>

          {/* Quorum Only Option */}
          <button
            onClick={() => onSelect('fast')}
            disabled={loading}
            className="w-full p-4 rounded-xl border-2 border-border hover:border-primary/50 bg-card hover:bg-primary/5 transition-all text-left group disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-success/10 text-success group-hover:bg-success/20 transition-colors">
                <Zap className="w-6 h-6" />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="font-semibold text-foreground">Quorum Only</h3>
                  <span className="px-2 py-0.5 text-xs font-medium bg-success/10 text-success rounded-full">
                    Instant
                  </span>
                </div>
                <p className="text-sm text-muted-foreground">
                  Direct extraction from workflow artifacts. Uses your task specifications as-is.
                </p>
                <div className="flex items-center gap-1 mt-2 text-xs text-success">
                  <Clock className="w-3 h-3" />
                  <span>~12ms</span>
                </div>
              </div>
            </div>
          </button>

          {/* AI Enhanced Option */}
          <button
            onClick={() => onSelect('ai')}
            disabled={loading}
            className="w-full p-4 rounded-xl border-2 border-border hover:border-primary/50 bg-card hover:bg-primary/5 transition-all text-left group disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-xl bg-primary/10 text-primary group-hover:bg-primary/20 transition-colors">
                <Sparkles className="w-6 h-6" />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <h3 className="font-semibold text-foreground">AI Enhanced</h3>
                  <span className="px-2 py-0.5 text-xs font-medium bg-primary/10 text-primary rounded-full">
                    Enhanced
                  </span>
                </div>
                <p className="text-sm text-muted-foreground">
                  LLM reformats and polishes issue content. Better titles and structured bodies.
                </p>
                <div className="flex items-center gap-1 mt-2 text-xs text-primary">
                  <Clock className="w-3 h-3" />
                  <span>Several minutes</span>
                </div>
              </div>
            </div>
          </button>
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-2 p-4 border-t border-border">
          <Button
            variant="ghost"
            onClick={onClose}
            disabled={loading}
          >
            Cancel
          </Button>
        </div>
      </div>
    </div>
  );
}
