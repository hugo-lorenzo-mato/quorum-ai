import { useConfigField } from '../../hooks/useConfigField';
import { useEnabledAgents } from '../../lib/agents';
import { SelectSetting, NumberInputSetting } from './index';

const PHASE_INFO = {
  analyze: {
    name: 'Analyze',
    description: 'Initial analysis of the task and codebase exploration',
    icon: '🔍',
  },
  plan: {
    name: 'Plan',
    description: 'Task breakdown into executable steps',
    icon: '📋',
  },
  execute: {
    name: 'Execute',
    description: 'Implementation of planned tasks',
    icon: '⚡',
  },
  review: {
    name: 'Review',
    description: 'Code review and validation',
    icon: '✅',
  },
};

const MODE_OPTIONS = [
  { value: 'single', label: 'Single Agent' },
  { value: 'moderator', label: 'Moderated Discussion' },
];

export function PhaseCard({ phaseKey }) {
  const info = PHASE_INFO[phaseKey];
  const prefix = `phases.${phaseKey}`;

  // Correct nested paths for the config structure
  const singleAgentEnabled = useConfigField(`${prefix}.single_agent.enabled`);
  const singleAgentAgent = useConfigField(`${prefix}.single_agent.agent`);
  const moderatorEnabled = useConfigField(`${prefix}.moderator.enabled`);
  const moderatorAgent = useConfigField(`${prefix}.moderator.agent`);
  const moderatorThreshold = useConfigField(`${prefix}.moderator.threshold`);
  const moderatorMinRounds = useConfigField(`${prefix}.moderator.min_rounds`);
  const moderatorMaxRounds = useConfigField(`${prefix}.moderator.max_rounds`);

  // Derive mode from enabled flags
  const currentMode = moderatorEnabled.value ? 'moderator' : 'single';

  // Get only enabled agents
  const enabledAgents = useEnabledAgents();
  const agentOptions = enabledAgents.map((a) => ({ value: a.value, label: a.label }));

  const handleModeChange = (newMode) => {
    if (newMode === 'single') {
      // Switching to single agent mode
      singleAgentEnabled.onChange(true);
      moderatorEnabled.onChange(false);
      if (!singleAgentAgent.value) {
        singleAgentAgent.onChange(moderatorAgent.value || 'claude');
      }
    } else {
      // Switching to moderator mode
      moderatorEnabled.onChange(true);
      singleAgentEnabled.onChange(false);
      if (!moderatorAgent.value) {
        moderatorAgent.onChange(singleAgentAgent.value || 'claude');
      }
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-6">
      {/* Header */}
      <div className="flex items-center gap-3 mb-4">
        <span className="text-2xl">{info.icon}</span>
        <div>
          <h3 className="font-semibold text-foreground">
            {info.name} Phase
          </h3>
          <p className="text-sm text-muted-foreground">
            {info.description}
          </p>
        </div>
      </div>

      {/* Mode Selection */}
      <div className="space-y-4">
        <div className="flex items-center gap-1 p-1 rounded-lg bg-secondary border border-border">
          {MODE_OPTIONS.map((option) => (
            <button
              key={option.value}
              onClick={() => handleModeChange(option.value)}
              type="button"
              className={`flex-1 px-3 py-2 text-sm font-medium rounded-md transition-all ${
                currentMode === option.value
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground hover:bg-background/50'
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>

        {/* Single Agent Mode */}
        {currentMode === 'single' && (
          <SelectSetting
            label="Agent"
            tooltip="The agent that will handle this phase independently."
            value={singleAgentAgent.value || ''}
            onChange={singleAgentAgent.onChange}
            options={agentOptions}
            error={singleAgentAgent.error}
            disabled={singleAgentAgent.disabled}
          />
        )}

        {/* Moderator Mode */}
        {currentMode === 'moderator' && (
          <>
            <SelectSetting
              label="Moderator Agent"
              tooltip="Agent that moderates the discussion and synthesizes results."
              value={moderatorAgent.value || ''}
              onChange={moderatorAgent.onChange}
              options={agentOptions}
              error={moderatorAgent.error}
              disabled={moderatorAgent.disabled}
            />

            <NumberInputSetting
              label="Consensus Threshold"
              tooltip="Minimum agreement level required (0.0 - 1.0). Higher values require more consensus."
              min={0}
              max={1}
              step={0.05}
              value={moderatorThreshold.value ?? 0.85}
              onChange={moderatorThreshold.onChange}
              error={moderatorThreshold.error}
              disabled={moderatorThreshold.disabled}
            />

            <NumberInputSetting
              label="Min Rounds"
              tooltip="Minimum number of discussion rounds."
              min={1}
              max={10}
              value={moderatorMinRounds.value ?? 2}
              onChange={moderatorMinRounds.onChange}
              error={moderatorMinRounds.error}
              disabled={moderatorMinRounds.disabled}
            />

            <NumberInputSetting
              label="Max Rounds"
              tooltip="Maximum number of discussion rounds."
              min={1}
              max={10}
              value={moderatorMaxRounds.value ?? 5}
              onChange={moderatorMaxRounds.onChange}
              error={moderatorMaxRounds.error}
              disabled={moderatorMaxRounds.disabled}
            />
          </>
        )}
      </div>
    </div>
  );
}

export default PhaseCard;
