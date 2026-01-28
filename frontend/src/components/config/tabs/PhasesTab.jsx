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
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
          Workflow Phases
        </h2>
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Configure how each phase of the workflow is executed.
        </p>
      </div>

      {/* Phase Flow Visualization */}
      <div className="flex items-center justify-center gap-2 py-4 text-sm text-gray-500 dark:text-gray-400">
        <span className="px-3 py-1 bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300 rounded font-medium">Analyze</span>
        <span>→</span>
        <span className="px-3 py-1 bg-gray-100 dark:bg-gray-700 rounded">Plan</span>
        <span>→</span>
        <span className="px-3 py-1 bg-gray-100 dark:bg-gray-700 rounded">Execute</span>
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
