// Package core provides centralized constants for agents, models, phases and reasoning.
// All packages should import from here to ensure consistency across the codebase.
package core

// Agent identifiers
const (
	AgentClaude   = "claude"
	AgentGemini   = "gemini"
	AgentCodex    = "codex"
	AgentCopilot  = "copilot"
	AgentOpenCode = "opencode"
)

// Agents is the ordered list of all supported agents.
var Agents = []string{
	AgentClaude,
	AgentGemini,
	AgentCodex,
	AgentCopilot,
	AgentOpenCode,
}

// ValidAgents is a map for O(1) agent validation.
var ValidAgents = map[string]bool{
	AgentClaude:   true,
	AgentGemini:   true,
	AgentCodex:    true,
	AgentCopilot:  true,
	AgentOpenCode: true,
}

// IsValidAgent checks if the given agent name is valid.
func IsValidAgent(agent string) bool {
	return ValidAgents[agent]
}

// ReasoningEfforts is the global union of all valid reasoning effort levels.
// For per-agent subsets, use AgentReasoningEfforts.
var ReasoningEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

// ValidReasoningEfforts is a map for O(1) effort validation (global union).
var ValidReasoningEfforts = map[string]bool{
	"none":    true,
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":  true,
	"max":    true,
}

// IsValidReasoningEffort checks if the given reasoning effort is valid for any agent.
func IsValidReasoningEffort(effort string) bool {
	return ValidReasoningEfforts[effort]
}

// AgentReasoningEfforts maps each reasoning-capable agent to the effort levels
// its CLI actually accepts (validated April 2026).
//   - Claude CLI v2.1.104: --effort low|medium|high|max
//   - Codex CLI v0.118.0:  -c model_reasoning_effort="none|minimal|low|medium|high|xhigh"
var AgentReasoningEfforts = map[string][]string{
	AgentClaude: {"low", "medium", "high", "max"},
	AgentCodex:  {"none", "minimal", "low", "medium", "high", "xhigh"},
}

// GetReasoningEfforts returns the valid effort levels for a specific agent.
// Returns the global union if the agent has no per-agent list.
func GetReasoningEfforts(agent string) []string {
	if efforts, ok := AgentReasoningEfforts[agent]; ok {
		return efforts
	}
	return ReasoningEfforts
}

// IsValidReasoningEffortForAgent checks if an effort level is valid for a specific agent.
func IsValidReasoningEffortForAgent(agent, effort string) bool {
	efforts := GetReasoningEfforts(agent)
	for _, e := range efforts {
		if e == effort {
			return true
		}
	}
	return false
}

// AgentsWithReasoning lists agents that support extended thinking/reasoning effort.
// Per-agent valid levels are in AgentReasoningEfforts.
// - Claude CLI v2.1.104: --effort low|medium|high|max
// - Codex CLI v0.118.0:  -c model_reasoning_effort="none|minimal|low|medium|high|xhigh"
// Note: Gemini CLI supports thinking via config.json thinkingConfig but not via CLI flag yet.
var AgentsWithReasoning = []string{
	AgentClaude,
	AgentCodex,
}

// SupportsReasoning checks if an agent supports reasoning effort configuration.
func SupportsReasoning(agent string) bool {
	for _, a := range AgentsWithReasoning {
		if a == agent {
			return true
		}
	}
	return false
}

// Task/role identifiers (not workflow phases, but config keys for models/reasoning)
const (
	TaskModerate   = "moderate"
	TaskSynthesize = "synthesize"
)

// =============================================================================
// Model Configuration (Centralized Source of Truth)
// =============================================================================

// AgentModels maps each agent to its supported models.
// This is the single source of truth for model availability.
var AgentModels = map[string][]string{
	AgentClaude: {
		// Claude 4.6 (latest)
		"claude-opus-4-6",   // Most powerful, 128K output, supports effort
		"claude-sonnet-4-6", // Best balance of speed and intelligence, 64K output
		// Claude 4.5 family
		"claude-opus-4-5-20251101",  // Previous opus generation
		"claude-sonnet-4-5-20250929", // Previous sonnet generation
		"claude-haiku-4-5-20251001",  // Fastest model with near-frontier performance
		// Claude 4 family
		"claude-opus-4-20250514",
		"claude-opus-4-1-20250805",
		"claude-sonnet-4-20250514",
		// Aliases (shortcuts accepted by claude CLI)
		"opus",   // Maps to latest opus model (currently Opus 4.6)
		"sonnet", // Maps to latest sonnet model (currently Sonnet 4.6)
		"haiku",  // Maps to latest haiku model
		// Virtual models (resolved by adapter, not by CLI)
		"opus-fast", // Opus 4.6 with fast mode (2.5x faster, higher cost)
	},
	AgentGemini: {
		// Gemini 3 family (CLI uses -preview suffix for 3.x models)
		"gemini-3.1-pro-preview",  // Advanced reasoning and agentic capabilities
		"gemini-3-flash-preview",  // Frontier-class performance at lower cost
		// Gemini 2.5 family (stable, recommended)
		"gemini-2.5-pro",        // Most powerful 2.5, best for complex tasks
		"gemini-2.5-flash",      // Best price/performance balance with thinking
		"gemini-2.5-flash-lite", // Fast, low-cost, 1M context
	},
	AgentCodex: {
		// GPT-5.4 family (latest flagship, reasoning: none/minimal/low/medium/high/xhigh)
		"gpt-5.4",       // Flagship frontier model for professional coding work
		"gpt-5.4-codex", // Code-optimized 5.4 for agentic coding tasks
		"gpt-5.4-mini",  // Fast, efficient mini for responsive tasks and subagents
		// GPT-5.3 family (agentic coding specialist)
		"gpt-5.3-codex",       // Agentic coding model (Codex CLI migrates this → gpt-5.4)
		"gpt-5.3-codex-spark", // Real-time coding iteration, text-only (Pro only)
		// GPT-5.2 family
		"gpt-5.2", // General-purpose model for coding and agentic tasks
		// GPT-5.1 family (retiring April 2026)
		"gpt-5.1-codex-max",  // Maximum capability for extended tasks (xhigh reasoning)
		"gpt-5.1-codex",      // Code-optimized GPT-5.1
		"gpt-5.1-codex-mini", // Cost-effective codex
		"gpt-5.1",            // Base GPT-5.1 model
		// GPT-5 family (legacy)
		"gpt-5-codex",      // GPT-5 codex version
		"gpt-5-codex-mini", // GPT-5 codex mini
		"gpt-5",            // Base GPT-5 model
	},
	AgentCopilot: {
		// Anthropic Claude models (via Copilot) - from docs.github.com/copilot/reference
		"claude-sonnet-4.6", // Latest sonnet — adaptive thinking support
		"claude-opus-4.6",   // Most powerful Claude, supports effort
		"claude-opus-4.5",   // Previous opus generation
		"claude-sonnet-4.5", // Strong reasoning (CLI default)
		"claude-haiku-4.5",  // Fast, efficient
		"claude-sonnet-4",   // Previous gen sonnet
		// OpenAI GPT models (via Copilot)
		"gpt-5.4",       // Latest flagship model
		"gpt-5.4-mini",  // Fast, efficient mini
		"gpt-5.3-codex", // Advanced agentic coding
		"gpt-5.2-codex", // Agentic coding
		"gpt-5.2",       // General-purpose
		"gpt-5",         // Base GPT-5
		"gpt-5-mini",    // Included in subscription (no premium requests)
		"gpt-4.1",       // Included in subscription (no premium requests)
		// Google Gemini models (via Copilot)
		"gemini-3.1-pro", // Gemini 3.1 Pro (preview)
		"gemini-3-flash", // Gemini 3 Flash (preview)
		"gemini-2.5-pro", // Gemini 2.5 Pro (stable)
	},
	AgentOpenCode: {
		// Local Ollama models
		"qwen2.5-coder:32b", // Best local coding model
		"qwen3-coder:30b",   // Latest Qwen coder
		"deepseek-r1:32b",   // Reasoning model
		"codestral:22b",     // Mistral code model
		"gpt-oss:20b",       // Open source GPT
	},
}

// AgentDefaultModels maps each agent to its default model.
var AgentDefaultModels = map[string]string{
	AgentClaude:   "sonnet",
	AgentGemini:   "gemini-2.5-flash",
	AgentCodex:    "gpt-5.3-codex",
	AgentCopilot:  "claude-sonnet-4.5",
	AgentOpenCode: "qwen2.5-coder:32b",
}

// GetSupportedModels returns the list of supported models for an agent.
// Returns nil if the agent is not recognized.
func GetSupportedModels(agent string) []string {
	return AgentModels[agent]
}

// GetDefaultModel returns the default model for an agent.
// Returns empty string if the agent is not recognized.
func GetDefaultModel(agent string) string {
	return AgentDefaultModels[agent]
}

// IsValidModel checks if a model is valid for a given agent.
func IsValidModel(agent, model string) bool {
	models := AgentModels[agent]
	for _, m := range models {
		if m == model {
			return true
		}
	}
	return false
}

// Phases is the ordered list of workflow phases (uses Phase type from phase.go).
var Phases = []string{
	string(PhaseRefine),
	string(PhaseAnalyze),
	string(PhasePlan),
	string(PhaseExecute),
}

// PhaseModelKeys are the valid keys for phase-specific model configuration.
// Includes both phases and special tasks (moderate, synthesize).
var PhaseModelKeys = []string{
	string(PhaseRefine),
	string(PhaseAnalyze),
	TaskModerate,
	TaskSynthesize,
	string(PhasePlan),
	string(PhaseExecute),
}

// ValidPhaseModelKeys is a map for O(1) phase model key validation.
var ValidPhaseModelKeys = map[string]bool{
	string(PhaseRefine):  true,
	string(PhaseAnalyze): true,
	TaskModerate:         true,
	TaskSynthesize:       true,
	string(PhasePlan):    true,
	string(PhaseExecute): true,
}

// IsValidPhaseModelKey checks if the given phase model key is valid.
func IsValidPhaseModelKey(key string) bool {
	return ValidPhaseModelKeys[key]
}

// Log levels
const (
	LogDebug = "debug"
	LogInfo  = "info"
	LogWarn  = "warn"
	LogError = "error"
)

// LogLevels is the ordered list of log levels.
var LogLevels = []string{LogDebug, LogInfo, LogWarn, LogError}

// Log formats
const (
	LogFormatAuto = "auto"
	LogFormatText = "text"
	LogFormatJSON = "json"
)

// LogFormats is the ordered list of log formats.
var LogFormats = []string{LogFormatAuto, LogFormatText, LogFormatJSON}

// Trace modes
const (
	TraceModeOff     = "off"
	TraceModeSummary = "summary"
	TraceModeFull    = "full"
)

// TraceModes is the ordered list of trace modes.
var TraceModes = []string{TraceModeOff, TraceModeSummary, TraceModeFull}

// Worktree modes
const (
	WorktreeModeAlways   = "always"
	WorktreeModeParallel = "parallel"
	WorktreeModeDisabled = "disabled"
)

// WorktreeModes is the ordered list of worktree modes.
var WorktreeModes = []string{WorktreeModeAlways, WorktreeModeParallel, WorktreeModeDisabled}

// Merge strategies
const (
	MergeStrategyMerge  = "merge"
	MergeStrategySquash = "squash"
	MergeStrategyRebase = "rebase"
)

// MergeStrategies is the ordered list of merge strategies.
var MergeStrategies = []string{MergeStrategyMerge, MergeStrategySquash, MergeStrategyRebase}

// IssueProviders is the ordered list of issue providers.
// Constants are defined in issue_ports.go as IssueProvider type.
var IssueProviders = []string{string(IssueProviderGitHub), string(IssueProviderGitLab)}

// Issue prompt languages
const (
	IssueLanguageEnglish    = "english"
	IssueLanguageSpanish    = "spanish"
	IssueLanguageFrench     = "french"
	IssueLanguageGerman     = "german"
	IssueLanguagePortuguese = "portuguese"
	IssueLanguageChinese    = "chinese"
	IssueLanguageJapanese   = "japanese"
)

// IssueLanguages is the ordered list of issue prompt languages.
var IssueLanguages = []string{
	IssueLanguageEnglish,
	IssueLanguageSpanish,
	IssueLanguageFrench,
	IssueLanguageGerman,
	IssueLanguagePortuguese,
	IssueLanguageChinese,
	IssueLanguageJapanese,
}

// Issue prompt tones
const (
	IssueToneProfessional = "professional"
	IssueToneCasual       = "casual"
	IssueToneTechnical    = "technical"
	IssueToneConcise      = "concise"
)

// IssueTones is the ordered list of issue prompt tones.
var IssueTones = []string{
	IssueToneProfessional,
	IssueToneCasual,
	IssueToneTechnical,
	IssueToneConcise,
}

// Issue creation modes
const (
	IssueModeDirect = "direct"
	IssueModeAgent  = "agent"
)

// IssueModes defines the valid issue creation modes.
var IssueModes = []string{IssueModeDirect, IssueModeAgent}
