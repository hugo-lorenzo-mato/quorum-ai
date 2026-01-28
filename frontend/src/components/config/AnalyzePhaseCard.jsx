import { useConfigStore } from '../../stores/configStore';
import {
  SettingSection,
  SelectSetting,
  ToggleSetting,
  NumberInputSetting,
  TextInputSetting,
} from './index';

const AGENT_OPTIONS = [
  { value: 'claude', label: 'Claude' },
  { value: 'codex', label: 'Codex' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'copilot', label: 'Copilot' },
];

export function AnalyzePhaseCard() {
  const config = useConfigStore((state) => state.config);
  const setField = useConfigStore((state) => state.setField);

  // Helper to get nested values safely
  const getValue = (path) => {
    if (!config) return undefined;
    return path.split('.').reduce((obj, key) => obj?.[key], config);
  };

  const analyzeConfig = config?.phases?.analyze;
  if (!analyzeConfig) {
    return (
      <SettingSection
        title="Analyze Phase"
        description="Loading configuration..."
      >
        <div className="text-sm text-muted-foreground">Loading...</div>
      </SettingSection>
    );
  }

  return (
    <div className="space-y-4">
      <SettingSection
        title="Analyze Phase"
        description="Initial analysis of the task and codebase exploration"
      >
        <TextInputSetting
          label="Timeout"
          tooltip="Maximum time allowed for the analyze phase"
          value={analyzeConfig.timeout || '2h'}
          onChange={(val) => setField('phases.analyze.timeout', val)}
          placeholder="2h"
        />
      </SettingSection>

      {/* Single Agent Mode */}
      <SettingSection
        title="Single Agent Mode"
        description="Use a single agent for analysis (faster, simpler)"
      >
        <ToggleSetting
          label="Enable Single Agent"
          tooltip="When enabled, use a single agent instead of moderated discussion"
          checked={analyzeConfig.single_agent?.enabled ?? false}
          onChange={(val) => setField('phases.analyze.single_agent.enabled', val)}
        />
        {analyzeConfig.single_agent?.enabled && (
          <>
            <SelectSetting
              label="Agent"
              tooltip="The agent to use for analysis"
              value={analyzeConfig.single_agent?.agent || 'claude'}
              onChange={(val) => setField('phases.analyze.single_agent.agent', val)}
              options={AGENT_OPTIONS}
            />
            <TextInputSetting
              label="Model Override"
              tooltip="Optional: specific model to use (leave empty for default)"
              value={analyzeConfig.single_agent?.model || ''}
              onChange={(val) => setField('phases.analyze.single_agent.model', val)}
              placeholder="e.g., claude-3-opus"
            />
          </>
        )}
      </SettingSection>

      {/* Moderator Mode */}
      <SettingSection
        title="Moderated Discussion"
        description="Multi-agent consensus through moderated rounds"
      >
        <ToggleSetting
          label="Enable Moderator"
          tooltip="When enabled, multiple agents discuss and reach consensus"
          checked={analyzeConfig.moderator?.enabled ?? true}
          onChange={(val) => setField('phases.analyze.moderator.enabled', val)}
        />
        {analyzeConfig.moderator?.enabled && (
          <>
            <SelectSetting
              label="Moderator Agent"
              tooltip="Agent that moderates the discussion"
              value={analyzeConfig.moderator?.agent || 'copilot'}
              onChange={(val) => setField('phases.analyze.moderator.agent', val)}
              options={AGENT_OPTIONS}
            />
            <NumberInputSetting
              label="Consensus Threshold"
              tooltip="Minimum agreement level (0.0 - 1.0)"
              min={0}
              max={1}
              step={0.05}
              value={analyzeConfig.moderator?.threshold ?? 0.85}
              onChange={(val) => setField('phases.analyze.moderator.threshold', val)}
            />
            <NumberInputSetting
              label="Min Rounds"
              tooltip="Minimum discussion rounds"
              min={1}
              max={10}
              value={analyzeConfig.moderator?.min_rounds ?? 2}
              onChange={(val) => setField('phases.analyze.moderator.min_rounds', val)}
            />
            <NumberInputSetting
              label="Max Rounds"
              tooltip="Maximum discussion rounds"
              min={1}
              max={10}
              value={analyzeConfig.moderator?.max_rounds ?? 5}
              onChange={(val) => setField('phases.analyze.moderator.max_rounds', val)}
            />
            <NumberInputSetting
              label="Abort Threshold"
              tooltip="Disagreement level that triggers abort (0.0 - 1.0)"
              min={0}
              max={1}
              step={0.05}
              value={analyzeConfig.moderator?.abort_threshold ?? 0.3}
              onChange={(val) => setField('phases.analyze.moderator.abort_threshold', val)}
            />
          </>
        )}
      </SettingSection>

      {/* Refiner */}
      <SettingSection
        title="Prompt Refiner"
        description="Refine user prompts before analysis"
      >
        <ToggleSetting
          label="Enable Refiner"
          tooltip="Improve and clarify user prompts automatically"
          checked={analyzeConfig.refiner?.enabled ?? true}
          onChange={(val) => setField('phases.analyze.refiner.enabled', val)}
        />
        {analyzeConfig.refiner?.enabled && (
          <SelectSetting
            label="Refiner Agent"
            tooltip="Agent used to refine prompts"
            value={analyzeConfig.refiner?.agent || 'claude'}
            onChange={(val) => setField('phases.analyze.refiner.agent', val)}
            options={AGENT_OPTIONS}
          />
        )}
      </SettingSection>

      {/* Synthesizer */}
      <SettingSection
        title="Result Synthesizer"
        description="Synthesize analysis results"
      >
        <SelectSetting
          label="Synthesizer Agent"
          tooltip="Agent used to synthesize final analysis"
          value={analyzeConfig.synthesizer?.agent || 'claude'}
          onChange={(val) => setField('phases.analyze.synthesizer.agent', val)}
          options={AGENT_OPTIONS}
        />
      </SettingSection>
    </div>
  );
}

export default AnalyzePhaseCard;
