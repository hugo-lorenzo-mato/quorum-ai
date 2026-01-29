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
    // Aliases (recommended for CLI use)
    { value: 'sonnet', label: 'Sonnet (alias)', description: 'Maps to latest sonnet' },
    { value: 'opus', label: 'Opus (alias)', description: 'Maps to latest opus' },
    { value: 'haiku', label: 'Haiku (alias)', description: 'Maps to latest haiku' },
    // Claude 4.5 family (full model names)
    { value: 'claude-opus-4-5-20251101', label: 'Opus 4.5', description: 'Most powerful model' },
    { value: 'claude-sonnet-4-5-20250929', label: 'Sonnet 4.5', description: 'Best balance' },
    { value: 'claude-haiku-4-5-20251001', label: 'Haiku 4.5', description: 'Fast & efficient' },
    // Claude 4 family
    { value: 'claude-opus-4-20250514', label: 'Opus 4', description: 'Previous gen opus' },
    { value: 'claude-opus-4-1-20250805', label: 'Opus 4.1', description: 'Opus 4 update' },
    { value: 'claude-sonnet-4-20250514', label: 'Sonnet 4', description: 'Previous gen sonnet' },
  ],
  gemini: [
    { value: '', label: 'Default', description: 'Use config default' },
    // Gemini 2.5 family (stable, recommended)
    { value: 'gemini-2.5-pro', label: 'Gemini 2.5 Pro', description: 'Most powerful, agentic tasks' },
    { value: 'gemini-2.5-flash', label: 'Gemini 2.5 Flash', description: 'Best price/performance' },
    { value: 'gemini-2.5-flash-lite', label: 'Gemini 2.5 Flash Lite', description: 'Fast, low-cost' },
    // Gemini 2.0 family (retiring March 2026)
    { value: 'gemini-2.0-flash', label: 'Gemini 2.0 Flash', description: 'Legacy (retiring 2026)' },
    { value: 'gemini-2.0-flash-lite', label: 'Gemini 2.0 Flash Lite', description: 'Legacy lite' },
    // Gemini 3 preview
    { value: 'gemini-3-pro-preview', label: 'Gemini 3 Pro Preview', description: 'Preview model' },
    { value: 'gemini-3-flash-preview', label: 'Gemini 3 Flash Preview', description: 'Preview flash' },
  ],
  codex: [
    { value: '', label: 'Default', description: 'Use config default' },
    // GPT-5.2 family (latest)
    { value: 'gpt-5.2-codex', label: 'GPT-5.2 Codex', description: 'Best agentic coding' },
    { value: 'gpt-5.2', label: 'GPT-5.2', description: 'Latest GPT-5.2' },
    // GPT-5.1 family
    { value: 'gpt-5.1-codex-max', label: 'GPT-5.1 Codex Max', description: 'Maximum capability' },
    { value: 'gpt-5.1-codex', label: 'GPT-5.1 Codex', description: 'Code-optimized' },
    { value: 'gpt-5.1-codex-mini', label: 'GPT-5.1 Codex Mini', description: 'Small, cost-effective' },
    { value: 'gpt-5.1', label: 'GPT-5.1', description: 'Base GPT-5.1' },
    // GPT-5 family
    { value: 'gpt-5', label: 'GPT-5', description: 'Base GPT-5' },
    { value: 'gpt-5-mini', label: 'GPT-5 Mini', description: 'Small, fast' },
    // GPT-4.1
    { value: 'gpt-4.1', label: 'GPT-4.1', description: 'Previous flagship' },
    // Reasoning models (o-series)
    { value: 'o3', label: 'o3', description: 'Advanced reasoning' },
    { value: 'o4-mini', label: 'o4-mini', description: 'Fast reasoning' },
  ],
  opencode: [
    { value: '', label: 'Default', description: 'Use config default' },
    // Local Ollama models
    { value: 'qwen2.5-coder:32b', label: 'Qwen 2.5 Coder 32B', description: 'Best local coding model' },
    { value: 'qwen3-coder:30b', label: 'Qwen 3 Coder 30B', description: 'Latest Qwen coder' },
    { value: 'deepseek-r1:32b', label: 'DeepSeek R1 32B', description: 'Reasoning model' },
    { value: 'codestral:22b', label: 'Codestral 22B', description: 'Mistral code model' },
    { value: 'gpt-oss:20b', label: 'GPT-OSS 20B', description: 'Open source GPT' },
  ],
  copilot: [
    { value: '', label: 'Default', description: 'Use config default' },
    // Anthropic Claude models (via Copilot)
    { value: 'claude-sonnet-4.5', label: 'Claude Sonnet 4.5', description: 'Best balance (default)' },
    { value: 'claude-haiku-4.5', label: 'Claude Haiku 4.5', description: 'Fast, efficient' },
    { value: 'claude-opus-4.5', label: 'Claude Opus 4.5', description: 'Most powerful' },
    { value: 'claude-sonnet-4', label: 'Claude Sonnet 4', description: 'Previous gen' },
    // OpenAI GPT models (via Copilot)
    { value: 'gpt-5.2-codex', label: 'GPT-5.2 Codex', description: 'Agentic coding' },
    { value: 'gpt-5.2', label: 'GPT-5.2', description: 'Latest GPT-5.2' },
    { value: 'gpt-5.1-codex-max', label: 'GPT-5.1 Codex Max', description: 'Maximum capability' },
    { value: 'gpt-5.1-codex', label: 'GPT-5.1 Codex', description: 'Code-optimized' },
    { value: 'gpt-5.1', label: 'GPT-5.1', description: 'Base GPT-5.1' },
    { value: 'gpt-5', label: 'GPT-5', description: 'Base GPT-5' },
    { value: 'gpt-5.1-codex-mini', label: 'GPT-5.1 Codex Mini', description: 'Small, efficient' },
    { value: 'gpt-5-mini', label: 'GPT-5 Mini', description: 'Small, fast' },
    { value: 'gpt-4.1', label: 'GPT-4.1', description: 'Previous flagship' },
    // Google Gemini models (via Copilot)
    { value: 'gemini-3-pro-preview', label: 'Gemini 3 Pro Preview', description: 'Gemini preview' },
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
