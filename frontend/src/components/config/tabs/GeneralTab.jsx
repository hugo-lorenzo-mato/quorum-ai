import { useConfigField, useConfigSelect } from '../../../hooks/useConfigField';
import { useUIStore } from '../../../stores';
import { Check, Moon, Sun, Laptop, Droplets, Snowflake, Ghost } from 'lucide-react';
import {
  SettingSection,
  SelectSetting,
  TextInputSetting,
  DurationInputSetting,
  ToggleSetting,
} from '../index';

const THEMES = [
  { id: 'system', name: 'System', icon: Laptop, colors: ['#52525b', '#a1a1aa'] },
  { id: 'light', name: 'Light', icon: Sun, colors: ['#ffffff', '#e4e4e7'] },
  { id: 'dark', name: 'Dark', icon: Moon, colors: ['#18181b', '#27272a'] },
  { id: 'midnight', name: 'Midnight', colors: ['#000000', '#18181b'] },
  { id: 'sepia', name: 'Sepia', colors: ['#f5f2e7', '#e0dcc7'] },
  { id: 'dracula', name: 'Dracula', icon: Ghost, colors: ['#282a36', '#44475a'] },
  { id: 'nord', name: 'Nord', icon: Snowflake, colors: ['#2e3440', '#3b4252'] },
  { id: 'ocean', name: 'Ocean', icon: Droplets, colors: ['#0f172a', '#1e293b'] },
];

export function GeneralTab() {
  return (
    <div className="space-y-8">
      <AppearanceSection />
      <LogSection />
      <ChatSection />
      <ReportSection />
    </div>
  );
}

function AppearanceSection() {
  const { theme, setTheme } = useUIStore();

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-1">
        <h3 className="text-lg font-medium text-foreground">Appearance</h3>
        <p className="text-sm text-muted-foreground">
          Customize the look and feel of the application.
        </p>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-4 gap-4">
        {THEMES.map((t) => {
          const isActive = theme === t.id;
          const Icon = t.icon;

          return (
            <button
              key={t.id}
              onClick={() => setTheme(t.id)}
              className={`
                group relative flex flex-col items-center gap-3 p-4 rounded-xl border-2 transition-all cursor-pointer
                hover:bg-secondary/50 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2
                ${isActive 
                  ? 'border-primary bg-secondary/30' 
                  : 'border-transparent bg-card hover:border-border'
                }
              `}
            >
              {/* Theme Preview Circle */}
              <div 
                className="w-12 h-12 rounded-full shadow-inner flex items-center justify-center border border-border/50 overflow-hidden relative ring-1 ring-black/5 dark:ring-white/10"
                style={{ 
                  background: `linear-gradient(135deg, ${t.colors[0]} 50%, ${t.colors[1]} 50%)`
                }}
              >
                {Icon && (
                  <Icon className={`w-5 h-5 relative z-10 text-foreground/80 drop-shadow-sm`} 
                    style={{
                      // Ensure icon is visible against dark/light backgrounds without blending artifacts
                      filter: 'drop-shadow(0 1px 2px rgb(0 0 0 / 0.1))',
                      color: t.id === 'light' || t.id === 'sepia' || t.id === 'system' ? '#18181b' : '#ffffff'
                    }}
                  />
                )}
              </div>

              {/* Label */}
              <div className="flex items-center gap-2">
                <span className={`text-sm font-medium ${isActive ? 'text-primary' : 'text-muted-foreground group-hover:text-foreground'}`}>
                  {t.name}
                </span>
                {isActive && (
                  <div className="w-4 h-4 rounded-full bg-primary flex items-center justify-center">
                    <Check className="w-2.5 h-2.5 text-primary-foreground" />
                  </div>
                )}
              </div>
            </button>
          );
        })}
      </div>
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
