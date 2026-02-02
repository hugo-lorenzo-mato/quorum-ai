import { useState } from 'react';
import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import { useConfigStore } from '../../../stores/configStore';
import {
  SettingSection,
  TextInputSetting,
  SelectSetting,
  ToggleSetting,
  ArrayInputSetting,
  ConfirmDialog,
} from '../index';

export function AdvancedTab() {
  return (
    <div className="space-y-6">
      <div className="p-3 rounded-xl bg-warning/10 border border-warning/20">
        <p className="text-sm text-foreground">
          <strong className="text-warning">Warning:</strong> These settings are for advanced users. Incorrect values may cause unexpected behavior.
        </p>
      </div>

      <TraceSection />
      <DangerZone />
    </div>
  );
}

function TraceSection() {
  const mode = useConfigSelect('trace.mode', 'trace_modes');
  const dir = useConfigField('trace.dir');
  const redact = useConfigField('trace.redact');
  const redactPatterns = useConfigField('trace.redact_patterns');
  const redactAllowlist = useConfigField('trace.redact_allowlist');
  const maxBytes = useConfigField('trace.max_bytes');
  const totalMaxBytes = useConfigField('trace.total_max_bytes');
  const maxFiles = useConfigField('trace.max_files');
  const includePhases = useConfigField('trace.include_phases');

  const isDisabled = mode.value === 'off';

  // Format bytes for display (convert to KB/MB)
  const formatBytes = (bytes) => {
    if (!bytes) return '';
    if (bytes >= 1048576) return `${Math.round(bytes / 1048576)} MB`;
    if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${bytes} bytes`;
  };

  return (
    <SettingSection
      title="Trace Logging"
      description="Detailed logging for debugging and troubleshooting workflow execution"
    >
      <SelectSetting
        label="Trace Mode"
        description="Control the level of trace output"
        tooltip="off: No tracing. summary: High-level execution traces. full: Detailed traces including all tool calls and agent responses."
        value={mode.value || 'off'}
        onChange={mode.onChange}
        options={mode.options.length > 0 ? mode.options : [
          { value: 'off', label: 'Off' },
          { value: 'summary', label: 'Summary' },
          { value: 'full', label: 'Full' },
        ]}
        error={mode.error}
        disabled={mode.disabled}
      />

      <TextInputSetting
        label="Trace Directory"
        description="Directory where trace files are stored"
        tooltip="Relative path from project root. Default: .quorum/traces"
        placeholder=".quorum/traces"
        value={dir.value || ''}
        onChange={dir.onChange}
        error={dir.error}
        disabled={dir.disabled || isDisabled}
      />

      <ToggleSetting
        label="Redact Sensitive Data"
        description="Automatically redact API keys, tokens, and other sensitive values"
        tooltip="When enabled, trace files will have sensitive patterns replaced with [REDACTED]. Recommended for security."
        checked={redact.value}
        onChange={redact.onChange}
        error={redact.error}
        disabled={redact.disabled || isDisabled}
      />

      <ArrayInputSetting
        label="Redact Patterns"
        description="Additional regex patterns to redact (beyond built-in defaults)"
        tooltip="Add custom regex patterns to match sensitive data. Leave empty to use only built-in patterns."
        value={redactPatterns.value || []}
        onChange={redactPatterns.onChange}
        error={redactPatterns.error}
        disabled={redactPatterns.disabled || isDisabled || !redact.value}
        placeholder="Add regex pattern..."
      />

      <ArrayInputSetting
        label="Redact Allowlist"
        description="Patterns to skip from redaction"
        tooltip="Regex patterns that match values which should NOT be redacted, even if they match a redact pattern."
        value={redactAllowlist.value || []}
        onChange={redactAllowlist.onChange}
        error={redactAllowlist.error}
        disabled={redactAllowlist.disabled || isDisabled || !redact.value}
        placeholder="Add allowlist pattern..."
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="py-3">
          <label className={`text-sm font-medium ${isDisabled ? 'text-muted-foreground' : 'text-foreground'}`}>
            Max Bytes per File
          </label>
          <p className="text-xs text-muted-foreground mb-2">
            Maximum size per trace file {maxBytes.value ? `(${formatBytes(maxBytes.value)})` : ''}
          </p>
          <input
            type="number"
            value={maxBytes.value || ''}
            onChange={(e) => maxBytes.onChange(e.target.value ? parseInt(e.target.value, 10) : null)}
            disabled={maxBytes.disabled || isDisabled}
            placeholder="262144"
            min={1024}
            className={`
              w-full px-3 py-2
              border rounded-lg bg-background text-foreground
              transition-colors
              focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
              disabled:opacity-50 disabled:cursor-not-allowed
              ${maxBytes.error ? 'border-error' : 'border-input hover:border-muted-foreground'}
            `}
          />
        </div>

        <div className="py-3">
          <label className={`text-sm font-medium ${isDisabled ? 'text-muted-foreground' : 'text-foreground'}`}>
            Total Max Bytes
          </label>
          <p className="text-xs text-muted-foreground mb-2">
            Max total trace size per run {totalMaxBytes.value ? `(${formatBytes(totalMaxBytes.value)})` : ''}
          </p>
          <input
            type="number"
            value={totalMaxBytes.value || ''}
            onChange={(e) => totalMaxBytes.onChange(e.target.value ? parseInt(e.target.value, 10) : null)}
            disabled={totalMaxBytes.disabled || isDisabled}
            placeholder="10485760"
            min={1024}
            className={`
              w-full px-3 py-2
              border rounded-lg bg-background text-foreground
              transition-colors
              focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
              disabled:opacity-50 disabled:cursor-not-allowed
              ${totalMaxBytes.error ? 'border-error' : 'border-input hover:border-muted-foreground'}
            `}
          />
        </div>

        <div className="py-3">
          <label className={`text-sm font-medium ${isDisabled ? 'text-muted-foreground' : 'text-foreground'}`}>
            Max Files
          </label>
          <p className="text-xs text-muted-foreground mb-2">
            Maximum trace files per run
          </p>
          <input
            type="number"
            value={maxFiles.value || ''}
            onChange={(e) => maxFiles.onChange(e.target.value ? parseInt(e.target.value, 10) : null)}
            disabled={maxFiles.disabled || isDisabled}
            placeholder="500"
            min={1}
            max={10000}
            className={`
              w-full px-3 py-2
              border rounded-lg bg-background text-foreground
              transition-colors
              focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent
              disabled:opacity-50 disabled:cursor-not-allowed
              ${maxFiles.error ? 'border-error' : 'border-input hover:border-muted-foreground'}
            `}
          />
        </div>
      </div>

      <ArrayInputSetting
        label="Include Phases"
        description="Which workflow phases to include in traces"
        tooltip="Select phases to trace: refine, analyze, plan, execute. Leave empty to trace all phases."
        value={includePhases.value || []}
        onChange={includePhases.onChange}
        error={includePhases.error}
        disabled={includePhases.disabled || isDisabled}
        placeholder="Add phase (refine, analyze, plan, execute)..."
      />
    </SettingSection>
  );
}

function DangerZone() {
  const [showResetDialog, setShowResetDialog] = useState(false);
  const resetToDefaults = useConfigStore((state) => state.resetToDefaults);
  const isLoading = useConfigStore((state) => state.isLoading);

  const handleReset = async () => {
    await resetToDefaults();
    setShowResetDialog(false);
  };

  return (
    <SettingSection
      title="Danger Zone"
      description="Irreversible actions"
      variant="danger"
    >
      <div className="p-4 border border-destructive/20 rounded-lg bg-destructive/5">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h4 className="font-medium text-destructive">
              Reset Configuration
            </h4>
            <p className="text-sm text-muted-foreground">
              Reset all settings to their default values. This cannot be undone.
            </p>
          </div>
          <button
            onClick={() => setShowResetDialog(true)}
            disabled={isLoading}
            className="px-4 py-2 bg-destructive text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50 disabled:pointer-events-none text-sm font-medium rounded-lg transition-colors"
            type="button"
          >
            Reset to Defaults
          </button>
        </div>
      </div>

      <ConfirmDialog
        isOpen={showResetDialog}
        onClose={() => setShowResetDialog(false)}
        onConfirm={handleReset}
        title="Reset Configuration?"
        message="This will reset all configuration values to their defaults. Any unsaved changes will also be lost. This action cannot be undone."
        confirmText="Reset Everything"
        variant="danger"
      />
    </SettingSection>
  );
}

export default AdvancedTab;
