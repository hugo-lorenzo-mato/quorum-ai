import { useState, useEffect, useMemo, useRef } from 'react';
import { X, Pencil } from 'lucide-react';
import TurndownService from 'turndown';
import VoiceInputButton from './VoiceInputButton';
import { getModelsForAgent, getReasoningLevels, supportsReasoning, useEnums } from '../lib/agents';

function formatBlueprintTimeout(seconds) {
  const s = Number(seconds || 0);
  if (!Number.isFinite(s) || s <= 0) return '';
  const whole = Math.floor(s);
  if (whole % 3600 === 0) return `${whole / 3600}h`;
  if (whole % 60 === 0) return `${whole / 60}m`;
  return `${whole}s`;
}

function parseDurationToSeconds(input) {
  const raw = (input || '').trim();
  if (!raw) return 0;

  // Allow plain integer => seconds
  if (/^\d+$/.test(raw)) return parseInt(raw, 10);

  const match = raw.match(/^(\d+(?:\.\d+)?)([hms])$/i);
  if (!match) return Number.NaN;

  const value = Number.parseFloat(match[1]);
  const unit = match[2].toLowerCase();
  if (!Number.isFinite(value) || value < 0) return Number.NaN;

  let seconds = value;
  if (unit === 'h') seconds *= 3600;
  else if (unit === 'm') seconds *= 60;
  else if (unit !== 's') return Number.NaN;

  return Math.round(seconds);
}

/**
 * EditWorkflowModal - Clean modal for editing workflow title and prompt
 */
export default function EditWorkflowModal({ isOpen, onClose, workflow, onSave, canEditPrompt = true }) {
  const [title, setTitle] = useState('');
  const [prompt, setPrompt] = useState('');
  const [timeoutOverride, setTimeoutOverride] = useState('');
  const [executionMode, setExecutionMode] = useState('multi_agent');
  const [singleAgentName, setSingleAgentName] = useState('claude');
  const [singleAgentModel, setSingleAgentModel] = useState('');
  const [singleAgentReasoningEffort, setSingleAgentReasoningEffort] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const titleRef = useRef(null);
  const titleInputId = 'edit-workflow-title';
  const promptInputId = 'edit-workflow-prompt';

  // Subscribe for enums updates (models/reasoning)
  useEnums();

  const canEditConfig = workflow?.status !== 'running' && workflow?.status !== 'cancelling';

  const AGENT_OPTIONS = [
    { value: 'claude', label: 'Claude' },
    { value: 'gemini', label: 'Gemini' },
    { value: 'codex', label: 'Codex' },
  ];

  const turndown = useMemo(() => {
    const service = new TurndownService({
      codeBlockStyle: 'fenced',
      emDelimiter: '*',
      bulletListMarker: '-',
    });
    service.keep(['kbd']);
    return service;
  }, []);

  // Sync values when modal opens
  useEffect(() => {
    if (isOpen && workflow) {
      setTitle(workflow.title || '');
      setPrompt(workflow.prompt || '');
      setTimeoutOverride(formatBlueprintTimeout(workflow.blueprint?.timeout_seconds));
      const rawMode = workflow.blueprint?.execution_mode;
      const mode = rawMode === 'single_agent' ? 'single_agent' : rawMode === 'interactive' ? 'interactive' : 'multi_agent';
      setExecutionMode(mode);
      setSingleAgentName(workflow.blueprint?.single_agent_name || 'claude');
      setSingleAgentModel(workflow.blueprint?.single_agent_model || '');
      setSingleAgentReasoningEffort(workflow.blueprint?.single_agent_reasoning_effort || '');
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
    // Only validate prompt if it can be edited
    if (canEditPrompt && !prompt.trim()) {
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
      // Only include prompt changes if prompt editing is allowed
      // Preserve prompt formatting exactly as entered (do NOT trim),
      // but still use trim() only for validation and change detection.
      if (canEditPrompt && prompt !== (workflow.prompt || '')) {
        updates.prompt = prompt;
      }

      // Allow editing blueprint overrides only when workflow is not running.
      if (canEditConfig) {
        const origRaw = workflow.blueprint?.execution_mode;
        const originalMode = origRaw === 'single_agent' ? 'single_agent' : origRaw === 'interactive' ? 'interactive' : 'multi_agent';
        const nextMode = executionMode;

        const effectiveSingleAgentName = AGENT_OPTIONS.some((a) => a.value === singleAgentName)
          ? singleAgentName
          : (AGENT_OPTIONS[0]?.value || singleAgentName);

        const modelOptions = getModelsForAgent(effectiveSingleAgentName);
        const effectiveSingleAgentModel = modelOptions.some((m) => m.value === singleAgentModel)
          ? singleAgentModel
          : '';

        const reasoningLevels = getReasoningLevels(effectiveSingleAgentName, effectiveSingleAgentModel || undefined);
        const agentSupportsReasoning = supportsReasoning(effectiveSingleAgentName);
        const effectiveSingleAgentReasoningEffort = agentSupportsReasoning && reasoningLevels.some((r) => r.value === singleAgentReasoningEffort)
          ? singleAgentReasoningEffort
          : '';

        const originalAgent = workflow.blueprint?.single_agent_name || '';
        const originalModel = workflow.blueprint?.single_agent_model || '';
        const originalEffort = workflow.blueprint?.single_agent_reasoning_effort || '';

        const configChanged = (() => {
          if (originalMode !== nextMode) return true;
          if (nextMode !== 'single_agent') return false;
          return (
            originalAgent !== effectiveSingleAgentName ||
            originalModel !== effectiveSingleAgentModel ||
            originalEffort !== effectiveSingleAgentReasoningEffort
          );
        })();

        if (configChanged) {
          if (nextMode === 'single_agent') {
            updates.blueprint = {
              execution_mode: 'single_agent',
              single_agent_name: effectiveSingleAgentName,
              single_agent_model: effectiveSingleAgentModel,
              single_agent_reasoning_effort: effectiveSingleAgentReasoningEffort,
            };
          } else {
            updates.blueprint = { execution_mode: nextMode };
          }
        }

        // Workflow-level timeout override (blueprint.timeout_seconds).
        // Empty clears override and falls back to global workflow.timeout.
        const originalTimeoutSeconds = Number(workflow.blueprint?.timeout_seconds || 0);
        const nextTimeoutSeconds = parseDurationToSeconds(timeoutOverride);
        if (Number.isNaN(nextTimeoutSeconds) || nextTimeoutSeconds < 0) {
          setError("Invalid workflow timeout override. Use e.g. '16h', '30m', '45s', or a number of seconds.");
          setSaving(false);
          return;
        }
        if (nextTimeoutSeconds !== originalTimeoutSeconds) {
          updates.blueprint = {
            ...(updates.blueprint || {}),
            timeout_seconds: nextTimeoutSeconds,
          };
        }
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

  const handlePromptPaste = (e) => {
    const clipboard = e.clipboardData;
    if (!clipboard) return;

    const plain = clipboard.getData('text/plain') || '';
    const html = clipboard.getData('text/html') || '';

    const htmlSuggestsStructure = /<\s*(p|br|li|ul|ol|pre|code|h[1-6]|blockquote|table)\b/i.test(html);
    const plainHasNewlines = plain.includes('\n');

    let textToInsert = plain;
    if (html && htmlSuggestsStructure && !plainHasNewlines) {
      try {
        textToInsert = turndown.turndown(html);
      } catch {
        textToInsert = plain;
      }
    }

    if (!textToInsert) return;

    // Insert preserving selection, avoiding React/DOM mismatch.
    e.preventDefault();
    const target = e.target;
    if (!(target instanceof HTMLTextAreaElement)) return;

    const start = target.selectionStart ?? 0;
    const end = target.selectionEnd ?? start;
    const currentValue = target.value ?? '';

    const nextValue = currentValue.slice(0, start) + textToInsert + currentValue.slice(end);
    setPrompt(nextValue);

    // Restore caret position after React updates value.
    const nextPos = start + textToInsert.length;
    requestAnimationFrame(() => {
      try {
        target.selectionStart = nextPos;
        target.selectionEnd = nextPos;
      } catch {
        // ignore
      }
    });
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
      <div className="relative w-full max-w-4xl mx-4 bg-card border border-border rounded-xl shadow-2xl animate-fade-up">
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
        <div className="p-4 space-y-4 overflow-y-auto max-h-[calc(100vh-14rem)]" onKeyDown={handleKeyDown}>
          {/* Title field */}
          <div>
            <label htmlFor={titleInputId} className="block text-sm font-medium text-foreground mb-1.5">
              Title
              <span className="text-muted-foreground font-normal ml-1">(optional)</span>
            </label>
            <input
              ref={titleRef}
              id={titleInputId}
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Give your workflow a descriptive name..."
              className="w-full px-3 py-2 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-shadow"
            />
          </div>

          {/* Prompt field */}
          <div>
            <label htmlFor={promptInputId} className="block text-sm font-medium text-foreground mb-1.5">
              Prompt
              {!canEditPrompt && (
                <span className="text-muted-foreground font-normal ml-1">(locked after execution)</span>
              )}
            </label>
            <div className="relative">
              <textarea
                id={promptInputId}
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                onPaste={handlePromptPaste}
                placeholder="Describe what you want the AI agents to accomplish..."
                rows={8}
                spellCheck={false}
                disabled={!canEditPrompt}
                className={`w-full px-3 py-2 pr-12 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background resize-y min-h-[120px] max-h-[500px] transition-shadow font-mono text-sm leading-6 ${!canEditPrompt ? 'opacity-60 cursor-not-allowed' : ''}`}
              />
              {canEditPrompt && (
                <VoiceInputButton
                  onTranscript={(text) => setPrompt((prev) => (prev ? prev + ' ' + text : text))}
                  disabled={saving}
                  className="absolute top-2 right-2"
                />
              )}
            </div>
            <p className="mt-1 text-xs text-muted-foreground text-right">
              {prompt.length.toLocaleString()} characters
            </p>
          </div>

          {canEditConfig && (
            <div className="p-3 rounded-lg border border-border bg-muted/20">
              <p className="text-sm font-medium text-foreground mb-2">Blueprint overrides</p>

              <div className="mb-4">
                <label className="block text-sm font-medium text-foreground mb-1.5">
                  Workflow timeout <span className="text-muted-foreground font-normal">(optional)</span>
                </label>
                <input
                  type="text"
                  value={timeoutOverride}
                  onChange={(e) => setTimeoutOverride(e.target.value)}
                  placeholder="e.g., 16h (leave empty to use global Settings)"
                  className="w-full px-3 py-2 rounded-lg border border-input bg-background text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background transition-shadow text-sm"
                />
                <p className="mt-1 text-xs text-muted-foreground">
                  Format: <code className="font-mono">16h</code>, <code className="font-mono">30m</code>, <code className="font-mono">45s</code>, or seconds (e.g., <code className="font-mono">3600</code>). Empty clears the override.
                </p>
              </div>

              <div className="space-y-2">
                <label className={`flex items-start gap-3 p-3 border rounded-lg cursor-pointer transition-all ${
                  executionMode === 'multi_agent'
                    ? 'border-primary bg-primary/5'
                    : 'border-border hover:bg-muted/50'
                }`}
                >
                  <input
                    type="radio"
                    name="editExecutionMode"
                    value="multi_agent"
                    checked={executionMode === 'multi_agent'}
                    onChange={() => setExecutionMode('multi_agent')}
                    className="mt-0.5 w-4 h-4 text-primary"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-foreground text-sm">Multi-Agent Consensus</div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      Multiple agents analyze and iterate to reach agreement
                    </div>
                  </div>
                </label>

                <label className={`flex items-start gap-3 p-3 border rounded-lg cursor-pointer transition-all ${
                  executionMode === 'interactive'
                    ? 'border-primary bg-primary/5'
                    : 'border-border hover:bg-muted/50'
                }`}
                >
                  <input
                    type="radio"
                    name="editExecutionMode"
                    value="interactive"
                    checked={executionMode === 'interactive'}
                    onChange={() => setExecutionMode('interactive')}
                    className="mt-0.5 w-4 h-4 text-primary"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-foreground text-sm">Interactive (Supervised)</div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      Pause between phases for review, feedback, and task editing
                    </div>
                  </div>
                </label>

                <label className={`flex items-start gap-3 p-3 border rounded-lg cursor-pointer transition-all ${
                  executionMode === 'single_agent'
                    ? 'border-primary bg-primary/5'
                    : 'border-border hover:bg-muted/50'
                }`}
                >
                  <input
                    type="radio"
                    name="editExecutionMode"
                    value="single_agent"
                    checked={executionMode === 'single_agent'}
                    onChange={() => setExecutionMode('single_agent')}
                    className="mt-0.5 w-4 h-4 text-primary"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-foreground text-sm">Single Agent</div>
                    <div className="text-xs text-muted-foreground mt-0.5">
                      One agent handles everything without iteration
                    </div>
                  </div>
                </label>
              </div>

              {executionMode === 'single_agent' && (
                <div className="mt-3 space-y-3">
                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      Agent
                    </label>
                    <select
                      value={singleAgentName}
                      onChange={(e) => {
                        setSingleAgentName(e.target.value);
                        setSingleAgentModel('');
                        setSingleAgentReasoningEffort('');
                      }}
                      className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                    >
                      {AGENT_OPTIONS.map((agent) => (
                        <option key={agent.value} value={agent.value}>
                          {agent.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground mb-1.5">
                      Model <span className="text-muted-foreground font-normal">(optional)</span>
                    </label>
                    <select
                      value={singleAgentModel}
                      onChange={(e) => setSingleAgentModel(e.target.value)}
                      className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                    >
                      {getModelsForAgent(singleAgentName).map((model) => (
                        <option key={`${singleAgentName}-${model.value || 'default'}`} value={model.value}>
                          {model.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  {supportsReasoning(singleAgentName) && (
                    <div>
                      <label className="block text-sm font-medium text-foreground mb-1.5">
                        Reasoning effort <span className="text-muted-foreground font-normal">(optional)</span>
                      </label>
                      <select
                        value={singleAgentReasoningEffort}
                        onChange={(e) => setSingleAgentReasoningEffort(e.target.value)}
                        className="w-full px-3 py-2 border border-input rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background"
                      >
                        <option value="">Default</option>
                        {getReasoningLevels(singleAgentName, singleAgentModel || undefined).map((level) => (
                          <option key={level.value} value={level.value}>
                            {level.label}
                          </option>
                        ))}
                      </select>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

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
              disabled={saving || (canEditPrompt && !prompt.trim())}
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
