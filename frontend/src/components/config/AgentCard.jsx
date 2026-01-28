import { useConfigField } from '../../hooks/useConfigField';
import {
  TextInputSetting,
  NumberInputSetting,
  DurationInputSetting,
  SliderSetting,
  ToggleSetting,
} from './index';

const AGENT_INFO = {
  claude: {
    name: 'Claude',
    description: 'Anthropic\'s Claude - Primary agent with strong reasoning',
    icon: '🟣',
    modelPlaceholder: 'claude-sonnet-4-20250514',
  },
  codex: {
    name: 'Codex',
    description: 'OpenAI\'s GPT models - Strong code generation',
    icon: '🟢',
    modelPlaceholder: 'gpt-4o',
  },
  gemini: {
    name: 'Gemini',
    description: 'Google\'s Gemini - Fast and efficient',
    icon: '🔵',
    modelPlaceholder: 'gemini-2.0-flash',
  },
  copilot: {
    name: 'Copilot',
    description: 'GitHub Copilot - Optimized for code tasks',
    icon: '⚫',
    modelPlaceholder: 'gpt-4o',
  },
};

export function AgentCard({ agentKey }) {
  const info = AGENT_INFO[agentKey];
  const prefix = `agents.${agentKey}`;

  const enabled = useConfigField(`${prefix}.enabled`);
  const model = useConfigField(`${prefix}.model`);
  const maxTokens = useConfigField(`${prefix}.max_tokens`);
  const temperature = useConfigField(`${prefix}.temperature`);
  const timeout = useConfigField(`${prefix}.timeout`);
  const maxRetries = useConfigField(`${prefix}.max_retries`);

  const isDisabled = !enabled.value;

  return (
    <div className={`
      rounded-lg border p-4 transition-opacity bg-card border-border
      ${isDisabled ? 'opacity-60' : ''}
    `}>
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <span className="text-2xl">{info.icon}</span>
          <div>
            <h3 className="font-semibold text-foreground">
              {info.name}
            </h3>
            <p className="text-sm text-muted-foreground">
              {info.description}
            </p>
          </div>
        </div>
        <ToggleSetting
          checked={enabled.value}
          onChange={enabled.onChange}
          error={enabled.error}
          disabled={enabled.disabled}
          compact
        />
      </div>

      {/* Settings Grid */}
      <div className={`space-y-4 ${isDisabled ? 'pointer-events-none' : ''}`}>
        <TextInputSetting
          label="Model"
          tooltip={`Model identifier for ${info.name}. Check provider documentation for available models.`}
          placeholder={info.modelPlaceholder}
          value={model.value}
          onChange={model.onChange}
          error={model.error}
          disabled={model.disabled || isDisabled}
        />

        <div className="grid grid-cols-2 gap-4">
          <NumberInputSetting
            label="Max Tokens"
            tooltip="Maximum tokens in agent responses. Higher values allow longer responses but cost more."
            min={1000}
            max={128000}
            step={1000}
            value={maxTokens.value}
            onChange={maxTokens.onChange}
            error={maxTokens.error}
            disabled={maxTokens.disabled || isDisabled}
          />

          <NumberInputSetting
            label="Max Retries"
            tooltip="Number of retry attempts on transient failures like rate limits or network errors."
            min={0}
            max={10}
            value={maxRetries.value}
            onChange={maxRetries.onChange}
            error={maxRetries.error}
            disabled={maxRetries.disabled || isDisabled}
          />
        </div>

        <SliderSetting
          label="Temperature"
          tooltip="Controls randomness in responses. 0.0 = deterministic, 2.0 = very creative. Recommended: 0.5-0.8 for code tasks."
          min={0}
          max={2}
          step={0.1}
          value={temperature.value}
          onChange={temperature.onChange}
          error={temperature.error}
          disabled={temperature.disabled || isDisabled}
          showValue
          formatValue={(v) => v.toFixed(1)}
        />

        <DurationInputSetting
          label="Timeout"
          tooltip="Per-request timeout for this agent. Example: '5m' for 5 minutes."
          value={timeout.value}
          onChange={timeout.onChange}
          error={timeout.error}
          disabled={timeout.disabled || isDisabled}
        />
      </div>
    </div>
  );
}

export default AgentCard;
