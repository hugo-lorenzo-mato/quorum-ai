import { useState, useEffect, useRef } from 'react';
import { X, Pencil } from 'lucide-react';

/**
 * EditWorkflowModal - Clean modal for editing workflow title and prompt
 */
export default function EditWorkflowModal({ isOpen, onClose, workflow, onSave }) {
  const [title, setTitle] = useState('');
  const [prompt, setPrompt] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const titleRef = useRef(null);

  // Sync values when modal opens
  useEffect(() => {
    if (isOpen && workflow) {
      setTitle(workflow.title || '');
      setPrompt(workflow.prompt || '');
      setError(null);
      // Focus title input after a short delay for animation
      setTimeout(() => titleRef.current?.focus(), 100);
    }
  }, [isOpen, workflow]);

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose]);

  const handleSave = async () => {
    if (!prompt.trim()) {
      setError('Prompt is required');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      const updates = {};
      if (title.trim() !== (workflow.title || '')) {
        updates.title = title.trim();
      }
      if (prompt.trim() !== (workflow.prompt || '')) {
        updates.prompt = prompt.trim();
      }

      if (Object.keys(updates).length > 0) {
        await onSave(updates);
      }
      onClose();
    } catch (err) {
      setError(err.message || 'Failed to save changes');
    } finally {
      setSaving(false);
    }
  };

  const handleKeyDown = (e) => {
    // Cmd/Ctrl + Enter to save
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSave();
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-background/80 backdrop-blur-sm animate-fade-in"
        onClick={onClose}
      />

      {/* Modal */}
      <div className="relative w-full max-w-2xl mx-4 bg-card border border-border rounded-xl shadow-2xl animate-fade-up">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          <div className="flex items-center gap-2">
            <Pencil className="w-4 h-4 text-muted-foreground" />
            <h2 className="text-lg font-semibold text-foreground">Edit Workflow</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-4" onKeyDown={handleKeyDown}>
          {/* Title field */}
          <div>
            <label className="block text-sm font-medium text-foreground mb-1.5">
              Title
              <span className="text-muted-foreground font-normal ml-1">(optional)</span>
            </label>
            <input
              ref={titleRef}
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Give your workflow a descriptive name..."
              className="w-full px-3 py-2 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-shadow"
            />
          </div>

          {/* Prompt field */}
          <div>
            <label className="block text-sm font-medium text-foreground mb-1.5">
              Prompt
            </label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Describe what you want the AI agents to accomplish..."
              rows={8}
              className="w-full px-3 py-2 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background resize-none transition-shadow"
            />
            <p className="mt-1 text-xs text-muted-foreground text-right">
              {prompt.length.toLocaleString()} characters
            </p>
          </div>

          {/* Error */}
          {error && (
            <p className="text-sm text-error">{error}</p>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between p-4 border-t border-border bg-muted/30 rounded-b-xl">
          <p className="text-xs text-muted-foreground">
            <kbd className="px-1.5 py-0.5 rounded bg-muted border border-border text-[10px] font-mono">⌘</kbd>
            <span className="mx-0.5">+</span>
            <kbd className="px-1.5 py-0.5 rounded bg-muted border border-border text-[10px] font-mono">Enter</kbd>
            <span className="ml-1.5">to save</span>
          </p>
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              disabled={saving}
              className="px-4 py-2 rounded-lg text-sm font-medium text-foreground hover:bg-accent transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving || !prompt.trim()}
              className="px-4 py-2 rounded-lg text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
