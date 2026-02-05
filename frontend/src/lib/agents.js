/**
 * Centralized agent, model and reasoning configuration.
 * Data is fetched from the backend API (single source of truth).
 * All components should import from here to ensure consistency.
 */

import { useState, useEffect } from 'react';
import { configApi } from './api';
import { useConfigStore } from '../stores/configStore';

// =============================================================================
// State Management - Data loaded from API
// =============================================================================

let enumsData = null;
let enumsPromise = null;
let enumsLoaded = false;
let enumsListeners = [];

// Fallback data (used before API loads)
const FALLBACK_AGENTS = ['claude', 'gemini', 'codex', 'copilot', 'opencode'];
const FALLBACK_REASONING_EFFORTS = ['minimal', 'low', 'medium', 'high', 'xhigh'];
const FALLBACK_AGENTS_WITH_REASONING = ['codex', 'copilot'];

/**
 * Subscribe to enums loaded event
 */
export function subscribeToEnums(listener) {
  enumsListeners.push(listener);
  // If already loaded, call immediately
  if (enumsLoaded) {
    listener(enumsData);
  }
  return () => {
    enumsListeners = enumsListeners.filter(l => l !== listener);
  };
}

/**
 * Notify all listeners that enums are loaded
 */
function notifyListeners() {
  enumsListeners.forEach(listener => listener(enumsData));
}

/**
 * Load enums from API (singleton pattern - only fetches once)
 */
export async function loadEnums() {
  if (enumsLoaded && enumsData) {
    return enumsData;
  }

  if (enumsPromise) {
    return enumsPromise;
  }

  enumsPromise = configApi.getEnums()
    .then(data => {
      enumsData = data;
      enumsLoaded = true;
      notifyListeners();
      return data;
    })
    .catch(err => {
      console.warn('Failed to load enums from API, using fallbacks:', err);
      enumsLoaded = true;
      notifyListeners();
      return null;
    });

  return enumsPromise;
}

/**
 * Get cached enums (may be null if not loaded yet)
 */
export function getEnums() {
  return enumsData;
}

/**
 * Check if enums are loaded
 */
export function isEnumsLoaded() {
  return enumsLoaded;
}

/**
 * React hook to get enums and re-render when they load
 */
export function useEnums() {
  const [, forceUpdate] = useState(0);

  useEffect(() => {
    return subscribeToEnums(() => {
      forceUpdate(n => n + 1);
    });
  }, []);

  return enumsData;
}

// =============================================================================
// Agent Configuration
// =============================================================================

// Agent display metadata (static, not from API)
export const AGENT_INFO = {
  claude: { name: 'Claude', label: 'Claude', description: 'Anthropic AI' },
  gemini: { name: 'Gemini', label: 'Gemini', description: 'Google AI' },
  codex: { name: 'Codex', label: 'Codex', description: 'OpenAI Codex' },
  opencode: { name: 'OpenCode', label: 'OpenCode', description: 'Open Source' },
  copilot: { name: 'Copilot', label: 'Copilot', description: 'GitHub Copilot' },
};

/**
 * Get list of available agents with display info
 */
export function getAgents() {
  const agents = enumsData?.agents || FALLBACK_AGENTS;
  return agents.map(value => ({
    value,
    label: AGENT_INFO[value]?.label || value,
    description: AGENT_INFO[value]?.description || '',
  }));
}

/**
 * Get list of enabled agents only.
 * Filters agents based on config.agents.{name}.enabled
 * @param {Object} agentsConfig - The agents config object (config.agents)
 * @returns {Array} List of enabled agents with display info
 */
export function getEnabledAgents(agentsConfig) {
  const allAgents = getAgents();

  // If config not loaded yet, return empty array to prevent showing disabled agents
  if (!agentsConfig) return [];

  return allAgents.filter(agent => {
    const config = agentsConfig[agent.value];
    // Agent is enabled if: config doesn't exist OR enabled !== false
    return config?.enabled !== false;
  });
}

/**
 * React hook to get enabled agents with automatic updates.
 * Uses configStore to filter agents by enabled status.
 * @returns {Array} List of enabled agents with display info
 */
export function useEnabledAgents() {
  const agentsConfig = useConfigStore((state) => state.config?.agents);

  // Also subscribe to enums updates
  useEnums();

  return getEnabledAgents(agentsConfig);
}

// Legacy export for compatibility
export const AGENTS = new Proxy([], {
  get(target, prop) {
    const agents = getAgents();
    if (prop === 'length') return agents.length;
    if (prop === Symbol.iterator) return agents[Symbol.iterator].bind(agents);
    if (typeof prop === 'string' && !isNaN(prop)) return agents[parseInt(prop)];
    if (prop === 'find') return agents.find.bind(agents);
    if (prop === 'map') return agents.map.bind(agents);
    if (prop === 'filter') return agents.filter.bind(agents);
    if (prop === 'forEach') return agents.forEach.bind(agents);
    return target[prop];
  },
});

// =============================================================================
// Model Configuration
// =============================================================================

/**
 * Get models for a specific agent
 */
export function getModelsForAgent(agent) {
  const agentModels = enumsData?.agent_models || {};
  const defaultModel = enumsData?.agent_default_models?.[agent] || '';
  const models = agentModels[agent] || [];

  // Always include "Default" option first
  const result = [
    { value: '', label: 'Default', description: `Use config default${defaultModel ? ` (${defaultModel})` : ''}` },
  ];

  // Add all models from API
  for (const model of models) {
    result.push({
      value: model,
      label: formatModelLabel(model),
      description: formatModelDescription(model),
    });
  }

  return result;
}

/**
 * Format model name for display
 */
function formatModelLabel(model) {
  // Handle aliases
  if (['opus', 'sonnet', 'haiku'].includes(model)) {
    return `${model.charAt(0).toUpperCase()}${model.slice(1)} (alias)`;
  }

  // Handle Claude models
  if (model.startsWith('claude-')) {
    const parts = model.replace('claude-', '').split('-');
    const variant = parts[0].charAt(0).toUpperCase() + parts[0].slice(1);
    const version = parts.slice(1, -1).join('.');
    return `${variant} ${version}`;
  }

  // Handle GPT models
  if (model.startsWith('gpt-')) {
    return model.replace('gpt-', 'GPT-').replace('-codex', ' Codex').replace('-mini', ' Mini').replace('-max', ' Max');
  }

  // Handle Gemini models
  if (model.startsWith('gemini-')) {
    return model.replace('gemini-', 'Gemini ').replace('-pro', ' Pro').replace('-flash', ' Flash').replace('-lite', ' Lite').replace('-preview', ' Preview');
  }

  // Handle o-series reasoning models
  if (model.match(/^o\d/)) {
    return model;
  }

  // Handle Ollama models (qwen, deepseek, etc.)
  if (model.includes(':')) {
    const [name, size] = model.split(':');
    return `${name.charAt(0).toUpperCase()}${name.slice(1).replace(/[.-]/g, ' ')} ${size.toUpperCase()}`;
  }

  return model;
}

/**
 * Format model description
 */
function formatModelDescription(model) {
  // Aliases
  if (model === 'opus') return 'Maps to latest opus';
  if (model === 'sonnet') return 'Maps to latest sonnet';
  if (model === 'haiku') return 'Maps to latest haiku';

  // Claude
  if (model.includes('opus')) return 'Most powerful';
  if (model.includes('sonnet')) return 'Best balance';
  if (model.includes('haiku')) return 'Fast & efficient';

  // GPT
  if (model.includes('codex-max')) return 'Maximum capability';
  if (model.includes('codex-mini')) return 'Small, cost-effective';
  if (model.includes('codex')) return 'Code-optimized';
  if (model.includes('mini')) return 'Small, fast';

  // Gemini
  if (model.includes('pro')) return 'Most powerful';
  if (model.includes('flash-lite')) return 'Fast, low-cost';
  if (model.includes('flash')) return 'Best price/performance';
  if (model.includes('preview')) return 'Preview model';

  // Reasoning models
  if (model === 'o3') return 'Advanced reasoning';
  if (model === 'o4-mini') return 'Fast reasoning';

  // Ollama
  if (model.includes('qwen') && model.includes('coder')) return 'Best local coding model';
  if (model.includes('deepseek-r1')) return 'Reasoning model';
  if (model.includes('codestral')) return 'Mistral code model';

  return '';
}

// Legacy export for compatibility (dynamic proxy)
export const AGENT_MODELS = new Proxy({}, {
  get(target, agent) {
    if (typeof agent === 'string') {
      return getModelsForAgent(agent);
    }
    return target[agent];
  },
});

// =============================================================================
// Reasoning Configuration
// =============================================================================

const REASONING_LABELS = {
  minimal: { label: 'Minimal', description: 'Quick responses' },
  low: { label: 'Low', description: 'Light reasoning' },
  medium: { label: 'Medium', description: 'Balanced (default)' },
  high: { label: 'High', description: 'Deep analysis' },
  xhigh: { label: 'Max', description: 'Maximum reasoning' },
};

/**
 * Get available reasoning levels
 */
export function getReasoningLevels() {
  const levels = enumsData?.reasoning_efforts || FALLBACK_REASONING_EFFORTS;
  return levels.map(value => ({
    value,
    label: REASONING_LABELS[value]?.label || value,
    description: REASONING_LABELS[value]?.description || '',
  }));
}

// Legacy export
export const REASONING_LEVELS = new Proxy([], {
  get(target, prop) {
    const levels = getReasoningLevels();
    if (prop === 'length') return levels.length;
    if (prop === Symbol.iterator) return levels[Symbol.iterator].bind(levels);
    if (typeof prop === 'string' && !isNaN(prop)) return levels[parseInt(prop)];
    if (prop === 'find') return levels.find.bind(levels);
    if (prop === 'map') return levels.map.bind(levels);
    if (prop === 'filter') return levels.filter.bind(levels);
    if (prop === 'forEach') return levels.forEach.bind(levels);
    return target[prop];
  },
});

/**
 * Get agents that support reasoning effort
 */
export function getAgentsWithReasoning() {
  return enumsData?.agents_with_reasoning || FALLBACK_AGENTS_WITH_REASONING;
}

// Legacy export
export const REASONING_AGENTS = new Proxy([], {
  get(target, prop) {
    const agents = getAgentsWithReasoning();
    if (prop === 'length') return agents.length;
    if (prop === Symbol.iterator) return agents[Symbol.iterator].bind(agents);
    if (typeof prop === 'string' && !isNaN(prop)) return agents[parseInt(prop)];
    if (prop === 'includes') return agents.includes.bind(agents);
    return target[prop];
  },
});

/**
 * Check if an agent supports reasoning effort
 */
export function supportsReasoning(agent) {
  return getAgentsWithReasoning().includes(agent);
}

// =============================================================================
// Helper Functions
// =============================================================================

export function getAgentByValue(value) {
  const agents = getAgents();
  return agents.find(a => a.value === value) || agents[0];
}

export function getModelByValue(agent, value) {
  const models = getModelsForAgent(agent);
  return models.find(m => m.value === value) || models[0];
}

export function getReasoningLevelByValue(value) {
  const levels = getReasoningLevels();
  return levels.find(r => r.value === value) || levels.find(r => r.value === 'medium') || levels[0];
}

export function getAgentInfo(agent) {
  return AGENT_INFO[agent] || { label: agent, description: '' };
}

// =============================================================================
// Constants (for compatibility)
// =============================================================================

export const DEFAULT_AGENT = 'claude';
export const DEFAULT_REASONING = 'medium';

// Phase identifiers
export function getPhases() {
  return enumsData?.phases || ['refine', 'analyze', 'plan', 'execute'];
}

export const PHASES = new Proxy([], {
  get(target, prop) {
    const phases = getPhases();
    if (prop === 'length') return phases.length;
    if (prop === Symbol.iterator) return phases[Symbol.iterator].bind(phases);
    if (typeof prop === 'string' && !isNaN(prop)) return phases[parseInt(prop)];
    return target[prop];
  },
});

// Phase model keys
export function getPhaseModelKeys() {
  return enumsData?.phase_model_keys || ['refine', 'analyze', 'moderate', 'synthesize', 'plan', 'execute'];
}

export const PHASE_MODEL_KEYS = new Proxy([], {
  get(target, prop) {
    const keys = getPhaseModelKeys();
    if (prop === 'length') return keys.length;
    if (prop === Symbol.iterator) return keys[Symbol.iterator].bind(keys);
    if (typeof prop === 'string' && !isNaN(prop)) return keys[parseInt(prop)];
    return target[prop];
  },
});

// Human-readable phase labels
export const PHASE_LABELS = {
  refine: 'Refine',
  analyze: 'Analyze',
  moderate: 'Moderate',
  synthesize: 'Synthesize',
  plan: 'Plan',
  execute: 'Execute',
};
