import { useMemo } from 'react';
import { Bot } from 'lucide-react';
import { useConfigField } from '../../hooks/useConfigField';
import { useConfigStore } from '../../stores/configStore';
import { TextInputSetting, ToggleSetting, SelectSetting } from './index';

const AGENT_INFO = {
  claude: {
    name: 'Claude',
    description: "Anthropic's Claude - primary agent with strong reasoning",
  },
  codex: {
    name: 'Codex',
    description: "OpenAI Codex - strong code generation (recommended for coding-heavy phases)",
  },
  gemini: {
    name: 'Gemini',
    description: "Google's Gemini - fast and efficient",
  },
  copilot: {
    name: 'Copilot',
    description: 'GitHub Copilot CLI - optimized for code tasks',
  },
};

const FALLBACK_PHASE_KEYS = ['refine', 'analyze', 'moderate', 'synthesize', 'plan', 'execute'];

const PHASE_LABELS = {
  refine: 'Refine',
  analyze: 'Analyze',
  moderate: 'Moderate',
  synthesize: 'Synthesize',
  plan: 'Plan',
  execute: 'Execute',
};

function normalizeBoolMap(value) {
  if (!value || typeof value !== 'object') return {};
  const entries = Object.entries(value).filter(([, v]) => v === true);
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

  const phaseKeys = useConfigStore((state) => state.enums?.phase_model_keys) || FALLBACK_PHASE_KEYS;
  const reasoningEfforts = useConfigStore((state) => state.enums?.reasoning_efforts) || [];
  const agentsMetadata = useConfigStore((state) => state.agents) || [];

  // Get available models for this agent
  const agentMeta = agentsMetadata.find((a) => a.name === agentKey);
  const availableModels = useMemo(() => {
    if (!agentMeta?.models?.length) return [];
    return agentMeta.models.map((m) => ({ value: m, label: m }));
  }, [agentMeta]);

  // Check if this agent supports reasoning effort
  const hasReasoningEffort = agentMeta?.hasReasoningEffort === true;
  const reasoningEffortOptions = useMemo(() => {
    return reasoningEfforts.map((e) => ({ value: e, label: e.charAt(0).toUpperCase() + e.slice(1) }));
  }, [reasoningEfforts]);

  const rawPhases = useMemo(
    () => (phases.value && typeof phases.value === 'object' ? phases.value : {}),
    [phases.value]
  );
  const cleanPhases = useMemo(() => normalizeBoolMap(rawPhases), [rawPhases]);
  const isRestricted = Object.keys(rawPhases).length > 0;
  const isEnabled = !!enabled.value;

  const isPhaseEnabled = (phase) => {
    if (!isRestricted) return true; // empty map => enabled for all phases (backward-compatible)
    return cleanPhases[phase] === true;
  };

  const enabledPhaseCount = phaseKeys.filter((k) => isPhaseEnabled(k)).length;

  const togglePhase = (phase) => {
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

      {/* Basic settings */}
      <div className={`mt-4 grid gap-4 ${hasReasoningEffort ? 'md:grid-cols-3' : 'md:grid-cols-2'}`}>
        <TextInputSetting
          label="CLI path"
          tooltip="Command to invoke the agent CLI (e.g. 'claude', 'codex', 'gemini', 'copilot')."
          placeholder={agentKey}
          value={path.value}
          onChange={path.onChange}
          error={path.error}
          disabled={path.disabled}
        />

        <SelectSetting
          label="Default model"
          tooltip="Default model used by this agent when no phase override is set."
          value={model.value}
          onChange={model.onChange}
          options={availableModels}
          error={model.error}
          disabled={model.disabled}
          placeholder="Select model..."
        />

        {hasReasoningEffort && (
          <SelectSetting
            label="Reasoning effort"
            tooltip="Controls reasoning depth. Higher effort = better quality but slower and more expensive."
            value={reasoningEffort.value}
            onChange={reasoningEffort.onChange}
            options={reasoningEffortOptions}
            error={reasoningEffort.error}
            disabled={reasoningEffort.disabled}
            placeholder="Select effort..."
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
            onClick={() => phases.onChange({})}
            disabled={phases.disabled || !isRestricted}
            className="text-xs font-medium text-muted-foreground hover:text-foreground disabled:opacity-50 disabled:pointer-events-none"
          >
            Reset to all
          </button>
        </div>

        <div className="mt-3 flex flex-wrap gap-2">
          {phaseKeys.map((phase) => {
            const active = isPhaseEnabled(phase);
            const label = PHASE_LABELS[phase] || phase;
            return (
              <button
                key={phase}
                type="button"
                onClick={() => togglePhase(phase)}
                disabled={phases.disabled}
                className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-colors ${
                  active
                    ? 'bg-background text-foreground border-border shadow-sm'
                    : 'bg-muted text-muted-foreground border-border hover:bg-accent/60 hover:text-foreground'
                } disabled:opacity-50 disabled:pointer-events-none`}
                title={phase}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export default AgentCard;
