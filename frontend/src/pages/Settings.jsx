import { useState } from 'react';
import { useUIStore } from '../stores';
import {
  Settings as SettingsIcon,
  Sun,
  Moon,
  Monitor,
  Palette,
  Bell,
  Shield,
  Database,
  Info,
  Check,
  ExternalLink,
} from 'lucide-react';

function SettingSection({ title, description, children }) {
  return (
    <div className="p-6 rounded-xl border border-border bg-card">
      <div className="mb-4">
        <h3 className="text-lg font-semibold text-foreground">{title}</h3>
        {description && (
          <p className="text-sm text-muted-foreground mt-1">{description}</p>
        )}
      </div>
      {children}
    </div>
  );
}

function ThemeOption({ value, icon: Icon, label, description, selected, onClick }) {
  return (
    <button
      onClick={onClick}
      className={`relative flex items-start gap-4 p-4 rounded-lg border-2 transition-all text-left ${
        selected
          ? 'border-primary bg-primary/5'
          : 'border-border hover:border-muted-foreground/50'
      }`}
    >
      <div className={`p-2 rounded-lg ${selected ? 'bg-primary/10' : 'bg-muted'}`}>
        <Icon className={`w-5 h-5 ${selected ? 'text-primary' : 'text-muted-foreground'}`} />
      </div>
      <div className="flex-1">
        <p className="font-medium text-foreground">{label}</p>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>
      {selected && (
        <div className="absolute top-3 right-3 p-1 rounded-full bg-primary">
          <Check className="w-3 h-3 text-primary-foreground" />
        </div>
      )}
    </button>
  );
}

function ToggleSetting({ label, description, checked, onChange }) {
  return (
    <div className="flex items-center justify-between py-3">
      <div>
        <p className="text-sm font-medium text-foreground">{label}</p>
        {description && (
          <p className="text-xs text-muted-foreground">{description}</p>
        )}
      </div>
      <button
        onClick={() => onChange(!checked)}
        className={`relative w-11 h-6 rounded-full transition-colors ${
          checked ? 'bg-primary' : 'bg-muted'
        }`}
      >
        <span
          className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white transition-transform ${
            checked ? 'translate-x-5' : ''
          }`}
        />
      </button>
    </div>
  );
}

export default function Settings() {
  const { theme, setTheme } = useUIStore();
  const [notifications, setNotifications] = useState(true);
  const [sounds, setSounds] = useState(false);

  const themes = [
    { value: 'light', icon: Sun, label: 'Light', description: 'Clean and bright interface' },
    { value: 'dark', icon: Moon, label: 'Dark', description: 'Easy on the eyes' },
    { value: 'midnight', icon: Palette, label: 'Midnight', description: 'Pure black for OLED displays' },
    { value: 'system', icon: Monitor, label: 'System', description: 'Follow system preference' },
  ];

  return (
    <div className="max-w-3xl mx-auto space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold text-foreground tracking-tight">Settings</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Customize your Quorum AI experience
        </p>
      </div>

      {/* Appearance */}
      <SettingSection
        title="Appearance"
        description="Choose how Quorum AI looks to you"
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {themes.map((t) => (
            <ThemeOption
              key={t.value}
              value={t.value}
              icon={t.icon}
              label={t.label}
              description={t.description}
              selected={theme === t.value}
              onClick={() => setTheme(t.value)}
            />
          ))}
        </div>
      </SettingSection>

      {/* Notifications */}
      <SettingSection
        title="Notifications"
        description="Configure how you receive updates"
      >
        <div className="divide-y divide-border">
          <ToggleSetting
            label="Push Notifications"
            description="Receive notifications when workflows complete"
            checked={notifications}
            onChange={setNotifications}
          />
          <ToggleSetting
            label="Sound Effects"
            description="Play sounds for important events"
            checked={sounds}
            onChange={setSounds}
          />
        </div>
      </SettingSection>

      {/* API Configuration */}
      <SettingSection
        title="API Configuration"
        description="Configure API endpoints and authentication"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              API Endpoint
            </label>
            <input
              type="text"
              value="http://localhost:8080/api/v1"
              readOnly
              className="w-full h-10 px-4 border border-input rounded-lg bg-muted text-muted-foreground"
            />
          </div>
          <div className="flex items-center gap-2 p-3 rounded-lg bg-info/10 text-info">
            <Info className="w-4 h-4 flex-shrink-0" />
            <p className="text-sm">API configuration is managed by the server</p>
          </div>
        </div>
      </SettingSection>

      {/* About */}
      <SettingSection title="About">
        <div className="space-y-4">
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-muted-foreground">Version</span>
            <span className="text-sm font-medium text-foreground">1.0.0</span>
          </div>
          <div className="flex items-center justify-between py-2">
            <span className="text-sm text-muted-foreground">Build</span>
            <span className="text-sm font-medium text-foreground">2024.01.25</span>
          </div>
          <div className="pt-4 border-t border-border">
            <a
              href="https://github.com/hugo-lorenzo-mato/quorum-ai"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <ExternalLink className="w-4 h-4" />
              View on GitHub
            </a>
          </div>
        </div>
      </SettingSection>
    </div>
  );
}
