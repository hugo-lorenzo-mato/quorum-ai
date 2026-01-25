import { useEffect, useState } from 'react';
import { configApi } from '../lib/api';
import { useUIStore } from '../stores';

function SettingsSection({ title, description, children }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm p-6">
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-1">{title}</h3>
      {description && (
        <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">{description}</p>
      )}
      {children}
    </div>
  );
}

function Toggle({ label, description, checked, onChange, disabled }) {
  return (
    <div className="flex items-center justify-between py-3">
      <div>
        <p className="text-sm font-medium text-gray-900 dark:text-white">{label}</p>
        {description && (
          <p className="text-xs text-gray-500 dark:text-gray-400">{description}</p>
        )}
      </div>
      <button
        onClick={() => onChange(!checked)}
        disabled={disabled}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
          checked ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'
        } ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
      >
        <span
          className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
            checked ? 'translate-x-6' : 'translate-x-1'
          }`}
        />
      </button>
    </div>
  );
}

function InputField({ label, description, value, onChange, placeholder, type = 'text', disabled }) {
  return (
    <div className="py-3">
      <label className="block text-sm font-medium text-gray-900 dark:text-white mb-1">{label}</label>
      {description && (
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{description}</p>
      )}
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:opacity-50"
      />
    </div>
  );
}

function SelectField({ label, description, value, onChange, options, disabled }) {
  return (
    <div className="py-3">
      <label className="block text-sm font-medium text-gray-900 dark:text-white mb-1">{label}</label>
      {description && (
        <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{description}</p>
      )}
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:opacity-50"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}

function AgentCard({ agent }) {
  const statusColors = {
    available: 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400',
    unavailable: 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-400',
    unknown: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300',
  };

  return (
    <div className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg">
      <div className="flex items-center justify-between mb-2">
        <h4 className="font-medium text-gray-900 dark:text-white">{agent.name}</h4>
        <span className={`px-2 py-1 text-xs font-medium rounded-full ${statusColors[agent.status] || statusColors.unknown}`}>
          {agent.status || 'unknown'}
        </span>
      </div>
      <p className="text-sm text-gray-500 dark:text-gray-400">{agent.model}</p>
      {agent.description && (
        <p className="mt-2 text-xs text-gray-400 dark:text-gray-500">{agent.description}</p>
      )}
    </div>
  );
}

export default function Settings() {
  const { theme, setTheme } = useUIStore();
  const [config, setConfig] = useState(null);
  const [agents, setAgents] = useState([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [localConfig, setLocalConfig] = useState({});

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      setError(null);
      try {
        const [configData, agentsData] = await Promise.all([
          configApi.get(),
          configApi.getAgents(),
        ]);
        setConfig(configData);
        setLocalConfig(configData);
        setAgents(agentsData.agents || []);
      } catch (err) {
        setError(err.message || 'Failed to load settings');
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSuccess(null);
    try {
      await configApi.update(localConfig);
      setConfig(localConfig);
      setSuccess('Settings saved successfully');
      setTimeout(() => setSuccess(null), 3000);
    } catch (err) {
      setError(err.message || 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const updateConfig = (path, value) => {
    setLocalConfig((prev) => {
      const newConfig = { ...prev };
      const keys = path.split('.');
      let current = newConfig;
      for (let i = 0; i < keys.length - 1; i++) {
        if (!current[keys[i]]) current[keys[i]] = {};
        current = current[keys[i]];
      }
      current[keys[keys.length - 1]] = value;
      return newConfig;
    });
  };

  const hasChanges = JSON.stringify(config) !== JSON.stringify(localConfig);

  if (loading) {
    return (
      <div className="flex justify-center py-16">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Settings</h1>
        <button
          onClick={handleSave}
          disabled={!hasChanges || saving}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {saving ? 'Saving...' : 'Save Changes'}
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 rounded-lg">
          {error}
        </div>
      )}

      {success && (
        <div className="p-4 bg-green-50 dark:bg-green-900/30 text-green-600 dark:text-green-400 rounded-lg">
          {success}
        </div>
      )}

      {/* Appearance */}
      <SettingsSection title="Appearance" description="Customize how the application looks">
        <SelectField
          label="Theme"
          value={theme}
          onChange={setTheme}
          options={[
            { value: 'light', label: 'Light' },
            { value: 'dark', label: 'Dark' },
            { value: 'system', label: 'System' },
          ]}
        />
      </SettingsSection>

      {/* Workflow Settings */}
      <SettingsSection title="Workflow Settings" description="Configure default workflow behavior">
        <InputField
          label="Default Max Retries"
          description="Maximum number of retries for failed tasks"
          value={localConfig.workflow?.max_retries ?? 3}
          onChange={(v) => updateConfig('workflow.max_retries', parseInt(v) || 0)}
          type="number"
        />
        <InputField
          label="Default Concurrency"
          description="Maximum number of concurrent tasks"
          value={localConfig.workflow?.concurrency ?? 1}
          onChange={(v) => updateConfig('workflow.concurrency', parseInt(v) || 1)}
          type="number"
        />
        <Toggle
          label="Enable Parallel Execution"
          description="Run independent tasks in parallel when possible"
          checked={localConfig.workflow?.parallel ?? false}
          onChange={(v) => updateConfig('workflow.parallel', v)}
        />
      </SettingsSection>

      {/* Agent Settings */}
      <SettingsSection title="Agent Settings" description="Configure AI agent defaults">
        <SelectField
          label="Default Agent"
          description="Agent to use when none is specified"
          value={localConfig.agents?.default ?? 'claude'}
          onChange={(v) => updateConfig('agents.default', v)}
          options={[
            { value: 'claude', label: 'Claude' },
            { value: 'gemini', label: 'Gemini' },
            { value: 'codex', label: 'Codex' },
          ]}
        />
        <InputField
          label="Default Temperature"
          description="Temperature for agent responses (0.0 - 1.0)"
          value={localConfig.agents?.temperature ?? 0.7}
          onChange={(v) => updateConfig('agents.temperature', parseFloat(v) || 0.7)}
          type="number"
        />
      </SettingsSection>

      {/* Available Agents */}
      <SettingsSection title="Available Agents" description="Status of configured AI agents">
        {agents.length > 0 ? (
          <div className="grid gap-4 sm:grid-cols-2">
            {agents.map((agent) => (
              <AgentCard key={agent.name} agent={agent} />
            ))}
          </div>
        ) : (
          <p className="text-gray-500 dark:text-gray-400 text-center py-4">
            No agents configured
          </p>
        )}
      </SettingsSection>

      {/* Storage Settings */}
      <SettingsSection title="Storage" description="Configure data storage locations">
        <InputField
          label="Data Directory"
          description="Directory for storing workflow data and artifacts"
          value={localConfig.storage?.data_dir ?? ''}
          onChange={(v) => updateConfig('storage.data_dir', v)}
          placeholder="/path/to/data"
        />
        <InputField
          label="Workspace Directory"
          description="Root directory for workflow workspaces"
          value={localConfig.storage?.workspace_dir ?? ''}
          onChange={(v) => updateConfig('storage.workspace_dir', v)}
          placeholder="/path/to/workspaces"
        />
      </SettingsSection>

      {/* About */}
      <SettingsSection title="About">
        <div className="text-sm text-gray-500 dark:text-gray-400 space-y-1">
          <p>Quorum AI - Multi-agent AI orchestration platform</p>
          <p className="text-xs">
            <a
              href="https://github.com/hugolma/quorum-ai"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              View on GitHub
            </a>
          </p>
        </div>
      </SettingsSection>
    </div>
  );
}
