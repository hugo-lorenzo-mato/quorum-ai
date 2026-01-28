import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import {
  SettingSection,
  SelectSetting,
  TextInputSetting,
  NumberInputSetting,
  DurationInputSetting,
  ArrayInputSetting,
  ToggleSetting,
} from '../index';

export function WorkflowTab() {
  return (
    <div className="space-y-6">
      <WorkflowSection />
      <StateSection />
    </div>
  );
}

function WorkflowSection() {
  const timeout = useConfigField('workflow.timeout');
  const maxRetries = useConfigField('workflow.max_retries');
  const dryRun = useConfigField('workflow.dry_run');
  const sandbox = useConfigField('workflow.sandbox');
  const denyTools = useConfigField('workflow.deny_tools');

  return (
    <SettingSection
      title="Workflow Execution"
      description="Configure how workflows are executed"
    >
      <DurationInputSetting
        label="Workflow Timeout"
        tooltip="Maximum duration for entire workflow execution. Example: '30m' for 30 minutes."
        value={timeout.value}
        onChange={timeout.onChange}
        error={timeout.error}
        disabled={timeout.disabled}
      />

      <NumberInputSetting
        label="Max Retries"
        tooltip="Number of retry attempts for failed agent tasks."
        min={0}
        max={10}
        value={maxRetries.value}
        onChange={maxRetries.onChange}
        error={maxRetries.error}
        disabled={maxRetries.disabled}
      />

      <ToggleSetting
        label="Dry Run Mode"
        description="Show what would happen without making changes"
        tooltip="When enabled, agents will simulate actions without modifying files or executing commands."
        checked={dryRun.value}
        onChange={dryRun.onChange}
        error={dryRun.error}
        disabled={dryRun.disabled}
      />

      <ToggleSetting
        label="Sandbox Mode"
        description="Run commands in sandboxed environment"
        tooltip="Provides an additional layer of security by isolating agent command execution."
        checked={sandbox.value}
        onChange={sandbox.onChange}
        error={sandbox.error}
        disabled={sandbox.disabled}
      />

      <ArrayInputSetting
        label="Denied Tools"
        tooltip="List of tool names to deny agents from using. Example: 'Bash', 'Write', 'Edit'."
        placeholder="Add tool name..."
        value={denyTools.value || []}
        onChange={denyTools.onChange}
        error={denyTools.error}
        disabled={denyTools.disabled}
        suggestions={['Bash', 'Write', 'Edit', 'Read', 'Glob', 'Grep', 'WebFetch', 'WebSearch']}
      />
    </SettingSection>
  );
}

function StateSection() {
  const backend = useConfigSelect('state.backend', 'state_backends');
  const path = useConfigField('state.path');
  const lockTtl = useConfigField('state.lock_ttl');

  const showPath = backend.value !== 'memory';

  return (
    <SettingSection
      title="State Management"
      description="Configure workflow state persistence"
    >
      <SelectSetting
        label="State Backend"
        tooltip="'memory' for ephemeral state (lost on restart), 'sqlite' for persistent SQLite database, 'file' for JSON file storage."
        value={backend.value}
        onChange={backend.onChange}
        options={backend.options}
        error={backend.error}
        disabled={backend.disabled}
      />

      {showPath && (
        <TextInputSetting
          label="State Path"
          tooltip="Path for state storage. For sqlite: database file path. For file: JSON file path."
          placeholder=".quorum/state.db"
          value={path.value}
          onChange={path.onChange}
          error={path.error}
          disabled={path.disabled}
        />
      )}

      <DurationInputSetting
        label="Lock TTL"
        tooltip="Time-to-live for state locks. Prevents stale locks from blocking workflows if a process crashes."
        value={lockTtl.value}
        onChange={lockTtl.onChange}
        error={lockTtl.error}
        disabled={lockTtl.disabled}
      />
    </SettingSection>
  );
}

export default WorkflowTab;
