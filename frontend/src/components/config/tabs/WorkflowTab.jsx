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
      <HeartbeatSection />
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

function HeartbeatSection() {
  const enabled = useConfigField('workflow.heartbeat.enabled');
  const interval = useConfigField('workflow.heartbeat.interval');
  const staleThreshold = useConfigField('workflow.heartbeat.stale_threshold');
  const checkInterval = useConfigField('workflow.heartbeat.check_interval');
  const autoResume = useConfigField('workflow.heartbeat.auto_resume');
  const maxResumes = useConfigField('workflow.heartbeat.max_resumes');

  const isDisabled = !enabled.value;

  return (
    <SettingSection
      title="Heartbeat Monitoring"
      description="Detect and recover from zombie workflows that stop responding"
    >
      <ToggleSetting
        label="Enable Heartbeat"
        description="Monitor workflow health and detect zombies"
        tooltip="When enabled, workflows periodically write heartbeats. If heartbeats stop, the workflow is considered a zombie."
        checked={enabled.value}
        onChange={enabled.onChange}
        error={enabled.error}
        disabled={enabled.disabled}
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <DurationInputSetting
          label="Heartbeat Interval"
          tooltip="How often the workflow writes a heartbeat. Example: '30s'."
          value={interval.value || ''}
          onChange={interval.onChange}
          error={interval.error}
          disabled={interval.disabled || isDisabled}
        />

        <DurationInputSetting
          label="Stale Threshold"
          tooltip="How long without a heartbeat before considering workflow a zombie. Example: '2m'."
          value={staleThreshold.value || ''}
          onChange={staleThreshold.onChange}
          error={staleThreshold.error}
          disabled={staleThreshold.disabled || isDisabled}
        />

        <DurationInputSetting
          label="Check Interval"
          tooltip="How often to check for zombie workflows. Example: '60s'."
          value={checkInterval.value || ''}
          onChange={checkInterval.onChange}
          error={checkInterval.error}
          disabled={checkInterval.disabled || isDisabled}
        />
      </div>

      <ToggleSetting
        label="Auto Resume"
        description="Automatically resume zombie workflows"
        tooltip="When enabled, zombie workflows will be automatically resumed. Use with caution - ensure your workflows are idempotent."
        checked={autoResume.value}
        onChange={autoResume.onChange}
        error={autoResume.error}
        disabled={autoResume.disabled || isDisabled}
      />

      {autoResume.value && (
        <div className="p-3 bg-warning/10 border border-warning/20 rounded-lg">
          <p className="text-sm text-foreground">
            <strong className="text-warning">Caution:</strong> Auto resume may cause issues if workflows are not idempotent. Tasks may be re-executed.
          </p>
        </div>
      )}

      <NumberInputSetting
        label="Max Resumes"
        tooltip="Maximum number of auto-resume attempts per workflow before giving up."
        min={1}
        max={10}
        value={maxResumes.value}
        onChange={maxResumes.onChange}
        error={maxResumes.error}
        disabled={maxResumes.disabled || isDisabled || !autoResume.value}
      />
    </SettingSection>
  );
}

function StateSection() {
  const backend = useConfigSelect('state.backend', 'state_backends');
  const path = useConfigField('state.path');
  const lockTtl = useConfigField('state.lock_ttl');

  return (
    <SettingSection
      title="State Management"
      description="Configure workflow state persistence"
    >
      <SelectSetting
        label="State Backend"
        tooltip="'sqlite' for persistent SQLite database or 'json' for JSON file storage."
        value={backend.value}
        onChange={backend.onChange}
        options={backend.options}
        error={backend.error}
        disabled={backend.disabled}
      />

      <TextInputSetting
        label="State Path"
        tooltip="Path for state storage. For sqlite: database file path. For json: JSON file path."
        placeholder=".quorum/state/state.db"
        value={path.value}
        onChange={path.onChange}
        error={path.error}
        disabled={path.disabled}
      />

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
