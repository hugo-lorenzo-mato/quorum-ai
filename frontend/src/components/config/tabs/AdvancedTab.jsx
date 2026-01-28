import { useState } from 'react';
import { useConfigField } from '../../../hooks/useConfigField';
import { useConfigStore } from '../../../stores/configStore';
import {
  SettingSection,
  TextInputSetting,
  NumberInputSetting,
  ToggleSetting,
  ConfirmDialog,
} from '../index';

export function AdvancedTab() {
  return (
    <div className="space-y-6">
      <div className="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg">
        <p className="text-sm text-yellow-700 dark:text-yellow-300">
          <strong>Warning:</strong> These settings are for advanced users. Incorrect values may cause unexpected behavior.
        </p>
      </div>

      <TraceSection />
      <ServerSection />
      <DangerZone />
    </div>
  );
}

function TraceSection() {
  const enabled = useConfigField('trace.enabled');
  const path = useConfigField('trace.path');
  const maxSize = useConfigField('trace.max_size');
  const maxAge = useConfigField('trace.max_age');
  const maxBackups = useConfigField('trace.max_backups');
  const compress = useConfigField('trace.compress');

  const isDisabled = !enabled.value;

  return (
    <SettingSection
      title="Trace Logging"
      description="Detailed logging for debugging and troubleshooting"
    >
      <ToggleSetting
        label="Enable Tracing"
        description="Create detailed trace log files"
        tooltip="When enabled, creates verbose log files useful for debugging issues. May impact performance."
        checked={enabled.value}
        onChange={enabled.onChange}
        error={enabled.error}
        disabled={enabled.disabled}
      />

      <TextInputSetting
        label="Trace Directory"
        tooltip="Directory where trace files are stored."
        placeholder=".quorum/traces"
        value={path.value}
        onChange={path.onChange}
        error={path.error}
        disabled={path.disabled || isDisabled}
      />

      <div className="grid grid-cols-3 gap-4">
        <NumberInputSetting
          label="Max Size (MB)"
          tooltip="Maximum size of each trace file before rotation. Range: 1-1000 MB."
          min={1}
          max={1000}
          value={maxSize.value}
          onChange={maxSize.onChange}
          error={maxSize.error}
          disabled={maxSize.disabled || isDisabled}
        />

        <NumberInputSetting
          label="Max Age (days)"
          tooltip="Trace files older than this will be deleted. Range: 1-365 days."
          min={1}
          max={365}
          value={maxAge.value}
          onChange={maxAge.onChange}
          error={maxAge.error}
          disabled={maxAge.disabled || isDisabled}
        />

        <NumberInputSetting
          label="Max Backups"
          tooltip="Number of old trace files to keep. Range: 0-100."
          min={0}
          max={100}
          value={maxBackups.value}
          onChange={maxBackups.onChange}
          error={maxBackups.error}
          disabled={maxBackups.disabled || isDisabled}
        />
      </div>

      <ToggleSetting
        label="Compress Old Files"
        description="Gzip compress rotated trace files"
        tooltip="Compresses old trace files to save disk space. Recommended."
        checked={compress.value}
        onChange={compress.onChange}
        error={compress.error}
        disabled={compress.disabled || isDisabled}
      />
    </SettingSection>
  );
}

function ServerSection() {
  const enabled = useConfigField('server.enabled');
  const port = useConfigField('server.port');
  const host = useConfigField('server.host');

  const isDisabled = !enabled.value;

  return (
    <SettingSection
      title="WebUI Server"
      description="Configure the built-in web server"
    >
      <ToggleSetting
        label="Enable WebUI Server"
        description="Start the web interface server"
        tooltip="When enabled, starts an HTTP server for the web interface. Disable if using CLI only."
        checked={enabled.value}
        onChange={enabled.onChange}
        error={enabled.error}
        disabled={enabled.disabled}
      />

      <div className="grid grid-cols-2 gap-4">
        <TextInputSetting
          label="Host"
          tooltip="Host address to bind. Use '127.0.0.1' for local only, '0.0.0.0' for all interfaces (security risk)."
          placeholder="127.0.0.1"
          value={host.value}
          onChange={host.onChange}
          error={host.error}
          disabled={host.disabled || isDisabled}
        />

        <NumberInputSetting
          label="Port"
          tooltip="Port number for the WebUI server. Range: 1024-65535."
          min={1024}
          max={65535}
          value={port.value}
          onChange={port.onChange}
          error={port.error}
          disabled={port.disabled || isDisabled}
        />
      </div>

      {host.value === '0.0.0.0' && (
        <div className="p-2 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-300">
          <strong>Security Warning:</strong> Binding to 0.0.0.0 exposes the server to all network interfaces.
        </div>
      )}
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
      <div className="p-4 border border-red-200 dark:border-red-800 rounded-lg bg-red-50 dark:bg-red-900/10">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="font-medium text-red-800 dark:text-red-200">
              Reset Configuration
            </h4>
            <p className="text-sm text-red-600 dark:text-red-300">
              Reset all settings to their default values. This cannot be undone.
            </p>
          </div>
          <button
            onClick={() => setShowResetDialog(true)}
            disabled={isLoading}
            className="px-4 py-2 bg-red-600 hover:bg-red-700 disabled:bg-red-400 text-white text-sm font-medium rounded-md transition-colors"
          >
            Reset to Defaults
          </button>
        </div>
      </div>

      <ConfirmDialog
        open={showResetDialog}
        onClose={() => setShowResetDialog(false)}
        onConfirm={handleReset}
        title="Reset Configuration?"
        description="This will reset all configuration values to their defaults. Any unsaved changes will also be lost. This action cannot be undone."
        confirmText="Reset Everything"
        variant="danger"
      />
    </SettingSection>
  );
}

export default AdvancedTab;
