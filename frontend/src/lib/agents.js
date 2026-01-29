/**
 * Centralized agent, model and reasoning configuration.
 * All components should import from here to ensure consistency.
 */

export const AGENTS = [
  { value: 'claude', label: 'Claude', description: 'Anthropic AI' },
  { value: 'gemini', label: 'Gemini', description: 'Google AI' },
  { value: 'codex', label: 'Codex', description: 'OpenAI Codex' },
  { value: 'opencode', label: 'OpenCode', description: 'Open Source' },
  { value: 'copilot', label: 'Copilot', description: 'GitHub Copilot' },
];

export const AGENT_MODELS = {
  claude: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'claude-sonnet-4-5-20250929', label: 'Sonnet 4.5', description: 'Fast & capable' },
    { value: 'claude-opus-4-5-20251101', label: 'Opus 4.5', description: 'Most powerful' },
    { value: 'claude-haiku-3-5-20241022', label: 'Haiku 3.5', description: 'Quick & efficient' },
  ],
  gemini: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'gemini-2.5-pro-preview-05-06', label: 'Gemini 2.5 Pro', description: 'Advanced reasoning' },
    { value: 'gemini-2.5-flash-preview-05-20', label: 'Gemini 2.5 Flash', description: 'Fast responses' },
    { value: 'gemini-2.0-flash', label: 'Gemini 2.0 Flash', description: 'Quick & capable' },
  ],
  codex: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'gpt-5.2', label: 'GPT-5.2', description: 'Latest model' },
    { value: 'gpt-5.1-codex', label: 'GPT-5.1 Codex', description: 'Code optimized' },
    { value: 'o3', label: 'o3', description: 'Reasoning model' },
    { value: 'o4-mini', label: 'o4-mini', description: 'Fast reasoning' },
  ],
  opencode: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'anthropic/claude-sonnet-4-5-20250929', label: 'Sonnet 4.5', description: 'Via OpenCode' },
    { value: 'openai/gpt-4o', label: 'GPT-4o', description: 'OpenAI via OpenCode' },
    { value: 'google/gemini-2.5-pro', label: 'Gemini 2.5 Pro', description: 'Google via OpenCode' },
  ],
  copilot: [
    { value: '', label: 'Default', description: 'Use config default' },
    { value: 'gpt-4o', label: 'GPT-4o', description: 'OpenAI GPT-4o' },
    { value: 'claude-3.5-sonnet', label: 'Claude 3.5 Sonnet', description: 'Anthropic Sonnet' },
    { value: 'o3-mini', label: 'o3-mini', description: 'Fast reasoning' },
  ],
};

export const REASONING_LEVELS = [
  { value: 'minimal', label: 'Minimal', description: 'Quick responses' },
  { value: 'low', label: 'Low', description: 'Light reasoning' },
  { value: 'medium', label: 'Medium', description: 'Balanced (default)' },
  { value: 'high', label: 'High', description: 'Deep analysis' },
  { value: 'xhigh', label: 'Max', description: 'Maximum reasoning' },
];

// Agents that support reasoning effort configuration
export const REASONING_AGENTS = ['codex', 'copilot'];

// Helper functions
export function getAgentByValue(value) {
  return AGENTS.find(a => a.value === value) || AGENTS[0];
}

export function getModelsForAgent(agent) {
  return AGENT_MODELS[agent] || AGENT_MODELS.claude;
}

export function getModelByValue(agent, value) {
  const models = getModelsForAgent(agent);
  return models.find(m => m.value === value) || models[0];
}

export function getReasoningLevelByValue(value) {
  return REASONING_LEVELS.find(r => r.value === value) || REASONING_LEVELS[2];
}

export function supportsReasoning(agent) {
  return REASONING_AGENTS.includes(agent);
}

export const DEFAULT_AGENT = 'claude';
export const DEFAULT_REASONING = 'medium';

// Phase identifiers
export const PHASES = ['refine', 'analyze', 'plan', 'execute'];

// Phase model keys (all phases that can have model overrides)
export const PHASE_MODEL_KEYS = ['refine', 'analyze', 'moderate', 'synthesize', 'plan', 'execute'];

// Human-readable phase labels
export const PHASE_LABELS = {
  refine: 'Refine',
  analyze: 'Analyze',
  moderate: 'Moderate',
  synthesize: 'Synthesize',
  plan: 'Plan',
  execute: 'Execute',
};

// Agent metadata for UI display
export const AGENT_INFO = {
  claude: {
    name: 'Claude',
    description: "Anthropic's Claude - primary agent with strong reasoning",
  },
  gemini: {
    name: 'Gemini',
    description: "Google's Gemini - fast and efficient",
  },
  codex: {
    name: 'Codex',
    description: "OpenAI Codex - strong code generation",
  },
  copilot: {
    name: 'Copilot',
    description: 'GitHub Copilot CLI - optimized for code tasks',
  },
  opencode: {
    name: 'OpenCode',
    description: 'Open source multi-provider agent',
  },
};

// Get agent info with fallback
export function getAgentInfo(agent) {
  return AGENT_INFO[agent] || { name: agent, description: '' };
}
