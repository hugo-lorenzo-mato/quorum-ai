import { useState, useRef, useEffect, useCallback } from 'react';
import { Pencil, Check, X, Loader2 } from 'lucide-react';

/**
 * EditableField - Inline editing component with Notion/Linear style UX
 *
 * Features:
 * - Click to edit with smooth transition
 * - Keyboard support: Enter to save, Escape to cancel (Cmd+Enter for multiline)
 * - Visual affordances on hover
 * - Loading and success states
 * - Auto-focus and select on edit
 */
export default function EditableField({
  value,
  onSave,
  placeholder = 'Click to edit...',
  multiline = false,
  disabled = false,
  className = '',
  inputClassName = '',
  displayClassName = '',
  maxLength,
  minRows = 2,
  maxRows = 6,
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [editValue, setEditValue] = useState(value || '');
  const [isSaving, setIsSaving] = useState(false);
  const [showSuccess, setShowSuccess] = useState(false);
  const inputRef = useRef(null);

  // Sync external value changes
  useEffect(() => {
    if (!isEditing) {
      setEditValue(value || '');
    }
  }, [value, isEditing]);

  // Auto-focus and select when entering edit mode
  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleStartEdit = useCallback(() => {
    if (disabled) return;
    setEditValue(value || '');
    setIsEditing(true);
  }, [disabled, value]);

  const handleCancel = useCallback(() => {
    setEditValue(value || '');
    setIsEditing(false);
  }, [value]);

  const handleSave = useCallback(async () => {
    const trimmed = editValue.trim();

    // Don't save if empty or unchanged
    if (!trimmed || trimmed === value) {
      handleCancel();
      return;
    }

    setIsSaving(true);
    try {
      await onSave(trimmed);
      setIsEditing(false);
      setShowSuccess(true);
      setTimeout(() => setShowSuccess(false), 1500);
    } catch (error) {
      console.error('Failed to save:', error);
      // Keep editing mode open on error
    } finally {
      setIsSaving(false);
    }
  }, [editValue, value, onSave, handleCancel]);

  const handleKeyDown = useCallback((e) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      handleCancel();
    } else if (e.key === 'Enter') {
      if (multiline) {
        // Cmd/Ctrl + Enter to save in multiline
        if (e.metaKey || e.ctrlKey) {
          e.preventDefault();
          handleSave();
        }
      } else {
        // Enter to save in single line
        e.preventDefault();
        handleSave();
      }
    }
  }, [multiline, handleCancel, handleSave]);

  const handleBlur = useCallback((e) => {
    // Don't cancel if clicking on save/cancel buttons
    if (e.relatedTarget?.closest('[data-editable-action]')) {
      return;
    }
    // Auto-save on blur if changed, otherwise cancel
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== value) {
      handleSave();
    } else {
      handleCancel();
    }
  }, [editValue, value, handleSave, handleCancel]);

  // Calculate textarea rows based on content
  const getRows = useCallback(() => {
    if (!multiline) return 1;
    const lines = (editValue || '').split('\n').length;
    return Math.min(Math.max(lines, minRows), maxRows);
  }, [multiline, editValue, minRows, maxRows]);

  // Editing mode
  if (isEditing) {
    const InputComponent = multiline ? 'textarea' : 'input';

    return (
      <div className={`relative group ${className}`}>
        <InputComponent
          ref={inputRef}
          type={multiline ? undefined : 'text'}
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={handleBlur}
          disabled={isSaving}
          maxLength={maxLength}
          rows={multiline ? getRows() : undefined}
          placeholder={placeholder}
          className={`
            w-full px-3 py-2 rounded-lg
            bg-background border-2 border-primary/50
            text-foreground placeholder:text-muted-foreground
            focus:outline-none focus:border-primary focus:ring-2 focus:ring-primary/20
            disabled:opacity-50 disabled:cursor-not-allowed
            transition-all duration-150
            ${multiline ? 'resize-none' : ''}
            ${inputClassName}
          `}
        />

        {/* Action buttons */}
        <div className="absolute right-2 top-2 flex items-center gap-1">
          {isSaving ? (
            <Loader2 className="w-4 h-4 text-primary animate-spin" />
          ) : (
            <>
              <button
                type="button"
                data-editable-action
                onClick={handleSave}
                className="p-1 rounded hover:bg-success/10 text-success transition-colors"
                title={multiline ? 'Save (Cmd+Enter)' : 'Save (Enter)'}
              >
                <Check className="w-4 h-4" />
              </button>
              <button
                type="button"
                data-editable-action
                onClick={handleCancel}
                className="p-1 rounded hover:bg-error/10 text-error transition-colors"
                title="Cancel (Escape)"
              >
                <X className="w-4 h-4" />
              </button>
            </>
          )}
        </div>

        {/* Character count for multiline */}
        {multiline && maxLength && (
          <div className="absolute right-2 bottom-2 text-xs text-muted-foreground">
            {editValue.length}/{maxLength}
          </div>
        )}

        {/* Keyboard hint */}
        <div className="mt-1 text-xs text-muted-foreground">
          {multiline ? 'Cmd+Enter to save · Escape to cancel' : 'Enter to save · Escape to cancel'}
        </div>
      </div>
    );
  }

  // Display mode
  return (
    <div
      role="button"
      tabIndex={disabled ? -1 : 0}
      onClick={handleStartEdit}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          handleStartEdit();
        }
      }}
      className={`
        group relative cursor-pointer
        rounded-lg transition-all duration-150
        ${disabled ? 'cursor-not-allowed opacity-60' : 'hover:bg-accent/50'}
        ${className}
      `}
    >
      <div className={`
        py-1 pr-8
        ${displayClassName}
        ${!value ? 'text-muted-foreground italic' : ''}
      `}>
        {value || placeholder}
      </div>

      {/* Edit indicator */}
      {!disabled && (
        <div className={`
          absolute right-0 top-1/2 -translate-y-1/2
          p-1.5 rounded-lg
          opacity-0 group-hover:opacity-100 group-focus:opacity-100
          transition-opacity duration-150
          ${showSuccess ? 'opacity-100' : ''}
        `}>
          {showSuccess ? (
            <Check className="w-4 h-4 text-success" />
          ) : (
            <Pencil className="w-3.5 h-3.5 text-muted-foreground" />
          )}
        </div>
      )}
    </div>
  );
}
