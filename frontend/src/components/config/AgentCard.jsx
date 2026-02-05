import { useMemo, useState } from 'react';
import { Bot, Check, ChevronDown, ChevronUp } from 'lucide-react';
import { useConfigField } from '../../hooks/useConfigField';
import { useConfigStore } from '../../stores/configStore';
import { TextInputSetting, ToggleSetting, SelectSetting } from './index';
import { AGENT_INFO, PHASE_MODEL_KEYS, PHASE_LABELS, supportsReasoning } from '../../lib/agents';

function normalizeBoolMap(value) {
  if (!value || typeof value !== 'object') return {};
  const entries = Object.entries(value).filter(([, v]) => v === true);
  return Object.fromEntries(entries);
}

function normalizeStringMap(value) {
  if (!value || typeof value !== 'object') return {};
  const entries = Object.entries(value).filter(
    ([, v]) => typeof v === 'string' && v.trim() !== ''
  );
  return Object.fromEntries(entries);
}

export function AgentCard({ agentKey }) {
  const info = AGENT_INFO[agentKey] || { name: agentKey, description: '' };
  const prefix = `agents.${agentKey}`;

  const enabled = useConfigField(`${prefix}.enabled`);
  const path = useConfigField(`${prefix}.path`);
  const model = useConfigField(`${prefix}.model`);
  const phases = useConfigField(`${prefix}.phases`);
  const reasoningEffort = useConfigField(`${prefix}.reasoning_effort`);
  const phaseModels = useConfigField(`${prefix}.phase_models`);
  const reasoningEffortPhases = useConfigField(`${prefix}.reasoning_effort_phases`);

  const phaseKeys = useConfigStore((state) => state.enums?.phase_model_keys) || PHASE_MODEL_KEYS;
  const reasoningEfforts = useConfigStore((state) => state.enums?.reasoning_efforts);
  const agentsMetadata = useConfigStore((state) => state.agents) || [];

  const [showOverrides, setShowOverrides] = useState(false);

  // Get available models for this agent
  const agentMeta = agentsMetadata.find((a) => a.name === agentKey);
  const availableModels = useMemo(() => {
    if (!agentMeta?.models?.length) return [];
    return agentMeta.models.map((m) => ({ value: m, label: m }));
  }, [agentMeta]);

  // Check if this agent supports reasoning effort
  const hasReasoningEffort = agentMeta?.hasReasoningEffort === true || supportsReasoning(agentKey);
  const reasoningEffortOptions = useMemo(() => {
    return (reasoningEfforts || []).map((e) => ({ value: e, label: e.charAt(0).toUpperCase() + e.slice(1) }));
  }, [reasoningEfforts]);

  const rawPhases = useMemo(
    () => (phases.value && typeof phases.value === 'object' ? phases.value : {}),
    [phases.value]
  );
  const cleanPhases = useMemo(() => normalizeBoolMap(rawPhases), [rawPhases]);
  const isRestricted = Object.keys(rawPhases).length > 0;
  const isEnabled = !!enabled.value;
  const isEditingDisabled = enabled.disabled || !isEnabled;

  const isPhaseEnabled = (phase) => {
    if (!isRestricted) return true; // empty map => enabled for all phases (backward-compatible)
    return cleanPhases[phase] === true;
  };

  const enabledPhaseCount = phaseKeys.filter((k) => isPhaseEnabled(k)).length;

  const togglePhase = (phase) => {
    if (isEditingDisabled) return;
    const next = isRestricted ? { ...cleanPhases } : Object.fromEntries(phaseKeys.map((k) => [k, true]));

    if (next[phase]) {
      delete next[phase];
    } else {
      next[phase] = true;
    }

    // Prevent "no enabled phases" (use the agent toggle for that)
    const nextEnabledCount = phaseKeys.filter((k) => next[k] === true).length;
    if (nextEnabledCount === 0) return;

    // If all phases are enabled, store empty map (means "all")
    if (nextEnabledCount === phaseKeys.length) {
      phases.onChange({});
      return;
    }

    phases.onChange(next);
  };

  const rawPhaseModels = useMemo(
    () => (phaseModels.value && typeof phaseModels.value === 'object' ? phaseModels.value : {}),
    [phaseModels.value]
  );
  const cleanPhaseModels = useMemo(() => normalizeStringMap(rawPhaseModels), [rawPhaseModels]);

  const rawEffortPhases = useMemo(
    () =>
      reasoningEffortPhases.value && typeof reasoningEffortPhases.value === 'object'
        ? reasoningEffortPhases.value
        : {},
    [reasoningEffortPhases.value]
  );
  const cleanEffortPhases = useMemo(() => normalizeStringMap(rawEffortPhases), [rawEffortPhases]);

  const overridePhases = useMemo(() => {
    const keys = new Set(Object.keys(cleanPhaseModels));
    if (hasReasoningEffort) {
      for (const k of Object.keys(cleanEffortPhases)) keys.add(k);
    }
    return phaseKeys.filter((k) => keys.has(k));
  }, [cleanPhaseModels, cleanEffortPhases, hasReasoningEffort, phaseKeys]);

  const setPhaseModelOverride = (phase, nextValue) => {
    if (isEditingDisabled) return;
    const trimmed = typeof nextValue === 'string' ? nextValue.trim() : '';

    const next = { ...cleanPhaseModels };
    if (!trimmed || trimmed === (model.value || '').trim()) {
      delete next[phase];
    } else {
      next[phase] = trimmed;
    }
    phaseModels.onChange(next);
  };

  const setPhaseEffortOverride = (phase, nextValue) => {
    if (isEditingDisabled || !hasReasoningEffort) return;
    const trimmed = typeof nextValue === 'string' ? nextValue.trim() : '';

    const next = { ...cleanEffortPhases };
    if (!trimmed || trimmed === (reasoningEffort.value || '').trim()) {
      delete next[phase];
    } else {
      next[phase] = trimmed;
    }
    reasoningEffortPhases.onChange(next);
  };

  const clearOverridesForPhase = (phase) => {
    if (isEditingDisabled) return;
    if (cleanPhaseModels[phase]) {
      const next = { ...cleanPhaseModels };
      delete next[phase];
      phaseModels.onChange(next);
    }
    if (hasReasoningEffort && cleanEffortPhases[phase]) {
      const next = { ...cleanEffortPhases };
      delete next[phase];
      reasoningEffortPhases.onChange(next);
    }
  };

  const clearAllOverrides = () => {
    if (isEditingDisabled) return;
    phaseModels.onChange({});
    if (hasReasoningEffort) {
      reasoningEffortPhases.onChange({});
    }
  };

  return (
    <div
      className={`rounded-xl border p-6 transition-colors ${
        isEnabled ? 'border-border bg-card' : 'border-border/70 bg-muted border-dashed'
      }`}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3 min-w-0">
          <div className="p-2 rounded-lg bg-muted border border-border">
            <Bot className="w-5 h-5 text-muted-foreground" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="font-semibold text-foreground">{info.name}</h3>
              <span
                className={`text-xs font-medium px-2 py-0.5 rounded-md border ${
                  isEnabled
                    ? 'bg-success/10 text-success border-success/20'
                    : 'bg-background text-muted-foreground border-border'
                }`}
              >
                {isEnabled ? 'Enabled' : 'Disabled'}
              </span>
            </div>
            {info.description && (
              <p className="text-sm text-muted-foreground mt-1">{info.description}</p>
            )}
          </div>
        </div>

        <ToggleSetting
          label={`Enable ${info.name}`}
          checked={enabled.value}
          onChange={enabled.onChange}
          error={enabled.error}
          disabled={enabled.disabled}
          compact
        />
      </div>

      {!isEnabled && (
        <p className="mt-3 text-sm text-muted-foreground">
          This agent is disabled. Enable it to edit its settings.
        </p>
      )}

      <div className={isEnabled ? '' : 'opacity-70'}>
        {/* Basic settings */}
        <div className={`mt-4 grid gap-4 ${hasReasoningEffort ? 'md:grid-cols-3' : 'md:grid-cols-2'}`}>
          <TextInputSetting
            label="CLI path"
            tooltip="Command to invoke the agent CLI (e.g. 'claude', 'codex', 'gemini', 'copilot')."
            id={`agent-${agentKey}-path`}
            placeholder={agentKey}
            value={path.value}
            onChange={path.onChange}
            error={path.error}
            disabled={path.disabled || !isEnabled}
          />

          <TextInputSetting
            label="Default model"
            tooltip="Used when no per-phase model override is set."
            id={`agent-${agentKey}-model`}
            placeholder="e.g. gpt-5.3-codex"
            value={model.value}
            onChange={model.onChange}
            error={model.error}
            disabled={model.disabled || !isEnabled}
            suggestions={availableModels.map((m) => m.value)}
          />

          {hasReasoningEffort && (
            <SelectSetting
              label="Reasoning effort"
              tooltip="Controls reasoning depth. Higher effort = better quality but slower and more expensive."
              id={`agent-${agentKey}-effort`}
              value={reasoningEffort.value}
              onChange={reasoningEffort.onChange}
              options={[
                { value: '', label: 'Auto (default)' },
                ...reasoningEffortOptions,
              ]}
              error={reasoningEffort.error}
              disabled={reasoningEffort.disabled || !isEnabled}
              placeholder={null}
            />
          )}
        </div>

        {/* Phase selection */}
        <div className="mt-4 pt-4 border-t border-border">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">Use in phases</p>
              <p className="text-xs text-muted-foreground mt-1">
                {isRestricted
                  ? `${enabledPhaseCount} selected · This list is an allowlist`
                  : 'Default: enabled for all phases'}
              </p>
            </div>
            <button
              type="button"
              onClick={() => !isEditingDisabled && phases.onChange({})}
              disabled={phases.disabled || !isRestricted || !isEnabled}
              className="text-xs font-medium text-muted-foreground hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
            >
              Reset to all
            </button>
          </div>

          <div className="mt-3 flex flex-wrap gap-1 p-1 rounded-lg bg-secondary border border-border">
            {phaseKeys.map((phase) => {
              const active = isPhaseEnabled(phase);
              const label = PHASE_LABELS[phase] || phase;
              return (
                <button
                  key={phase}
                  type="button"
                  onClick={() => togglePhase(phase)}
                  disabled={phases.disabled || !isEnabled}
                  aria-pressed={active}
                  className={`inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                    active
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
                  } disabled:opacity-50 disabled:pointer-events-none`}
                  title={phase}
                >
                  {active && <Check className="w-3 h-3" aria-hidden="true" />}
                  {label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Per-phase overrides */}
        <div className="mt-4 pt-4 border-t border-border">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">Per-phase overrides</p>
              <p className="text-xs text-muted-foreground mt-1">
                Defaults apply everywhere unless you override a specific phase.
              </p>
              <p className="text-xs text-muted-foreground mt-1">
                {overridePhases.length === 0
                  ? 'No overrides set'
                  : `Overrides: ${overridePhases.map((p) => PHASE_LABELS[p] || p).join(', ')}`}
              </p>
            </div>
            <button
              type="button"
              onClick={() => setShowOverrides((v) => !v)}
              disabled={!isEnabled}
              className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
            >
              {showOverrides ? 'Hide' : 'Edit'}
              {showOverrides ? (
                <ChevronUp className="w-3 h-3" aria-hidden="true" />
              ) : (
                <ChevronDown className="w-3 h-3" aria-hidden="true" />
              )}
            </button>
          </div>

          {showOverrides && (
            <div className="mt-3 space-y-2">
              {phaseKeys.map((phase) => {
                const phaseAllowed = isPhaseEnabled(phase);
                const label = PHASE_LABELS[phase] || phase;
                const modelOverride = cleanPhaseModels[phase] || '';
                const effortOverride = hasReasoningEffort ? cleanEffortPhases[phase] || '' : '';
                const hasAnyOverride = !!modelOverride || (!!effortOverride && hasReasoningEffort);

                return (
                  <div
                    key={phase}
                    className={`flex flex-col gap-2 rounded-lg border border-border p-3 md:flex-row md:items-center ${
                      hasAnyOverride ? 'bg-accent/20' : 'bg-transparent'
                    } ${phaseAllowed ? '' : 'opacity-60'}`}
                  >
                    <div className="flex items-center justify-between gap-2 md:w-40">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-foreground">{label}</p>
                        <p className="text-xs text-muted-foreground">
                          {phaseAllowed ? 'Enabled' : 'Not enabled for this agent'}
                        </p>
                      </div>
                      {hasAnyOverride && (
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-md border bg-background text-foreground border-border">
                          Override
                        </span>
                      )}
                    </div>

                    <div className="flex-1 grid gap-2 md:grid-cols-2">
                      <div className="relative">
                        <input
                          type="text"
                          value={modelOverride}
                          onChange={(e) => setPhaseModelOverride(phase, e.target.value)}
                          disabled={isEditingDisabled || !phaseAllowed}
                          placeholder={`Default (${model.value || '—'})`}
                          list={`models-${agentKey}`}
                          className={`
                            w-full h-10 px-3
                            border rounded-lg bg-background text-foreground
                            transition-colors
                            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
                            disabled:opacity-50 disabled:cursor-not-allowed
                            placeholder:text-muted-foreground
                            border-input hover:border-muted-foreground
                          `}
                        />
                      </div>

                      {hasReasoningEffort ? (
                        <select
                          value={effortOverride}
                          onChange={(e) => setPhaseEffortOverride(phase, e.target.value)}
                          disabled={isEditingDisabled || !phaseAllowed}
                          className={`
                            w-full h-10 px-3 appearance-none
                            border rounded-lg bg-background text-foreground
                            transition-colors cursor-pointer
                            focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
                            disabled:opacity-50 disabled:cursor-not-allowed
                            border-input hover:border-muted-foreground
                          `}
                        >
                          <option value="">
                            Default ({reasoningEffort.value || 'auto'})
                          </option>
                          {reasoningEffortOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </select>
                      ) : (
                        <div />
                      )}
                    </div>

                    <div className="flex items-center justify-end gap-2 md:w-24">
                      {hasAnyOverride && (
                        <button
                          type="button"
                          onClick={() => clearOverridesForPhase(phase)}
                          disabled={isEditingDisabled}
                          className="text-xs font-medium text-muted-foreground hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                        >
                          Clear
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}

              <div className="flex items-center justify-between pt-2">
                <p className="text-xs text-muted-foreground">
                  Tip: leave empty to inherit defaults.
                </p>
                <button
                  type="button"
                  onClick={clearAllOverrides}
                  disabled={isEditingDisabled || overridePhases.length === 0}
                  className="text-xs font-medium text-muted-foreground hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
                >
                  Clear all
                </button>
              </div>

              {/* Suggestions */}
              <datalist id={`models-${agentKey}`}>
                {availableModels.map((m) => (
                  <option key={m.value} value={m.value} />
                ))}
              </datalist>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default AgentCard;
