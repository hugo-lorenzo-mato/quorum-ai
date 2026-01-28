import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import {
  SettingSection,
  SelectSetting,
  TextInputSetting,
  DurationInputSetting,
  ToggleSetting,
} from '../index';

export function GeneralTab() {
  return (
    <div className="space-y-6">
      <LogSection />
      <ChatSection />
      <ReportSection />
    </div>
  );
}

function LogSection() {
  const level = useConfigSelect('log.level', 'log_levels');
  const format = useConfigSelect('log.format', 'log_formats');

  return (
    <SettingSection
      title="Logging"
      description="Configure logging behavior"
    >
      <SelectSetting
        label="Log Level"
        tooltip="Use 'debug' for troubleshooting, 'info' for normal operation, 'warn' for warnings only, or 'error' for errors only."
        value={level.value}
        onChange={level.onChange}
        options={level.options}
        error={level.error}
        disabled={level.disabled}
      />

      <SelectSetting
        label="Log Format"
        tooltip="'auto' detects terminal capability, 'text' for human-readable, 'json' for machine-parseable logs."
        value={format.value}
        onChange={format.onChange}
        options={format.options}
        error={format.error}
        disabled={format.disabled}
      />
    </SettingSection>
  );
}

function ChatSection() {
  const timeout = useConfigField('chat.timeout');
  const progressInterval = useConfigField('chat.progress_interval');
  const editor = useConfigField('chat.editor');

  return (
    <SettingSection
      title="Chat Settings"
      description="Configure chat behavior in TUI"
    >
      <DurationInputSetting
        label="Chat Timeout"
        tooltip="Chat response timeout. Example: '3m' for 3 minutes."
        value={timeout.value}
        onChange={timeout.onChange}
        error={timeout.error}
        disabled={timeout.disabled}
      />

      <DurationInputSetting
        label="Progress Interval"
        tooltip="Interval for progress logs. Example: '15s'."
        value={progressInterval.value}
        onChange={progressInterval.onChange}
        error={progressInterval.error}
        disabled={progressInterval.disabled}
      />

      <TextInputSetting
        label="Editor Command"
        tooltip="Editor command for file edits. Example: 'code', 'nvim', 'vim'."
        placeholder="vim"
        value={editor.value}
        onChange={editor.onChange}
        error={editor.error}
        disabled={editor.disabled}
      />
    </SettingSection>
  );
}

function ReportSection() {
  const enabled = useConfigField('report.enabled');
  const baseDir = useConfigField('report.base_dir');
  const useUtc = useConfigField('report.use_utc');
  const includeRaw = useConfigField('report.include_raw');

  return (
    <SettingSection
      title="Report Generation"
      description="Configure markdown report generation"
    >
      <ToggleSetting
        label="Enable Reports"
        description="Generate markdown reports for workflow runs"
        tooltip="When enabled, generates a markdown report after each workflow run."
        checked={enabled.value}
        onChange={enabled.onChange}
        error={enabled.error}
        disabled={enabled.disabled}
      />

      <TextInputSetting
        label="Report Directory"
        tooltip="Base directory for workflow reports."
        placeholder=".quorum/runs"
        value={baseDir.value}
        onChange={baseDir.onChange}
        error={baseDir.error}
        disabled={baseDir.disabled || !enabled.value}
      />

      <ToggleSetting
        label="Use UTC Timestamps"
        description="Use UTC instead of local time"
        tooltip="When enabled, all timestamps in reports use UTC timezone."
        checked={useUtc.value}
        onChange={useUtc.onChange}
        error={useUtc.error}
        disabled={useUtc.disabled || !enabled.value}
      />

      <ToggleSetting
        label="Include Raw Output"
        description="Include raw LLM outputs in reports"
        tooltip="When enabled, includes the raw LLM responses. Increases report size."
        checked={includeRaw.value}
        onChange={includeRaw.onChange}
        error={includeRaw.error}
        disabled={includeRaw.disabled || !enabled.value}
      />
    </SettingSection>
  );
}

export default GeneralTab;
