import { useMemo } from 'react';
import { useConfigField } from '../../../hooks/useConfigField';
import { useConfigStore } from '../../../stores/configStore';
import { AgentCard } from '../AgentCard';
import { SettingSection, SelectSetting } from '../index';

const FALLBACK_AGENTS = ['claude', 'gemini', 'codex', 'copilot'];

export function AgentsTab() {
  const defaultAgent = useConfigField('agents.default');
  const agents = useConfigStore((state) => state.enums?.agents) || FALLBACK_AGENTS;
  const config = useConfigStore((state) => state.config);

  const agentOptions = useMemo(
    () => agents.map((a) => ({ value: a, label: a.charAt(0).toUpperCase() + a.slice(1) })),
    [agents]
  );

  const defaultEnabled = config?.agents?.[defaultAgent.value]?.enabled;

  return (
    <div className="space-y-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-foreground">AI Agents</h2>
        <p className="text-sm text-muted-foreground">
          Configure available agents, their default model, and which phases they can be used in.
        </p>
      </div>

      <SettingSection
        title="Default Agent"
        description="Used when a phase does not explicitly specify an agent."
      >
        <SelectSetting
          label="Default agent"
          value={defaultAgent.value}
          onChange={defaultAgent.onChange}
          options={agentOptions}
          error={defaultAgent.error}
          disabled={defaultAgent.disabled}
          placeholder="Select agent..."
        />
        {defaultAgent.value && defaultEnabled === false && (
          <p className="text-xs text-warning mt-2">
            The selected default agent is currently disabled.
          </p>
        )}
      </SettingSection>

      <div className="grid gap-6 md:grid-cols-2">
        {agents.map((agentKey) => (
          <AgentCard key={agentKey} agentKey={agentKey} />
        ))}
      </div>

      <AgentWarnings />
    </div>
  );
}

function AgentWarnings() {
  // This component could show warnings based on agent configuration
  // For example: "No agents enabled" or "Consider enabling multiple agents for redundancy"
  return null; // Warnings handled by validation system
}

export default AgentsTab;
