import { useConfigStore } from '../../../stores/configStore';
import { AnalyzePhaseCard } from '../AnalyzePhaseCard';
import { SettingSection, TextInputSetting } from '../index';

export function PhasesTab() {
  const config = useConfigStore((state) => state.config);
  const setField = useConfigStore((state) => state.setField);

  const getFieldValue = (path) => {
    if (!config) return undefined;
    return path.split('.').reduce((obj, key) => obj?.[key], config);
  };

  return (
    <div className="space-y-6">
      <div className="mb-4">
        <h2 className="text-lg font-semibold text-foreground">Workflow Phases</h2>
        <p className="text-sm text-muted-foreground">
          Configure how each phase of the workflow is executed.
          <span className="block mt-1 text-xs">
            Note: phase-specific model and reasoning settings live under Settings → Agents (Per-phase overrides).
          </span>
        </p>
      </div>

      {/* Phase Flow Visualization */}
      <div className="flex items-center justify-center gap-2 py-4 text-sm text-muted-foreground">
        <span className="px-3 py-1 rounded-md font-medium bg-info/10 text-info border border-info/20">
          Analyze
        </span>
        <span aria-hidden="true">→</span>
        <span className="px-3 py-1 rounded-md bg-muted text-foreground border border-border">
          Plan
        </span>
        <span aria-hidden="true">→</span>
        <span className="px-3 py-1 rounded-md bg-muted text-foreground border border-border">
          Execute
        </span>
      </div>

      {/* Analyze Phase - Full configuration */}
      <AnalyzePhaseCard />

      {/* Plan Phase - Simple timeout only */}
      <SettingSection
        title="Plan Phase"
        description="Task breakdown into executable steps"
      >
        <TextInputSetting
          label="Timeout"
          tooltip="Maximum time allowed for the plan phase"
          value={getFieldValue('phases.plan.timeout') || '1h'}
          onChange={(val) => setField('phases.plan.timeout', val)}
          placeholder="1h"
        />
      </SettingSection>

      {/* Execute Phase - Simple timeout only */}
      <SettingSection
        title="Execute Phase"
        description="Implementation of planned tasks"
      >
        <TextInputSetting
          label="Timeout"
          tooltip="Maximum time allowed for the execute phase"
          value={getFieldValue('phases.execute.timeout') || '2h'}
          onChange={(val) => setField('phases.execute.timeout', val)}
          placeholder="2h"
        />
      </SettingSection>
    </div>
  );
}

export default PhasesTab;
