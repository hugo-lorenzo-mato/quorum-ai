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

// Codex reasoning effort levels (via -c model_reasoning_effort="level")
// Official values: minimal, low, medium, high, xhigh
// See: https://developers.openai.com/codex/config-reference/
var CodexReasoningEfforts = []string{"minimal", "low", "medium", "high", "xhigh"}

var ValidCodexReasoningEfforts = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
	"xhigh":  true,
}

// Claude effort levels (via CLAUDE_CODE_EFFORT_LEVEL env var)
// Official values: low, medium, high, max (Opus 4.6 only)
// See: https://platform.claude.com/docs/en/build-with-claude/effort
var ClaudeReasoningEfforts = []string{"low", "medium", "high", "max"}

var ValidClaudeReasoningEfforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"max":    true,
}

// AllReasoningEfforts is the union of all valid effort values across agents.
// Used only for config validation (any agent config can use any of these).
var AllReasoningEfforts = []string{"minimal", "low", "medium", "high", "xhigh", "max"}

var ValidReasoningEfforts = map[string]bool{
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

// AgentsWithReasoning lists agents that support extended thinking/reasoning effort.
// - Codex CLI: via -c model_reasoning_effort="level"
// - Claude CLI: via CLAUDE_CODE_EFFORT_LEVEL env var (low/medium/high/max)
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

// CodexModelMaxReasoning maps Codex models to their maximum supported reasoning effort.
// Models not in this map default to "high".
var CodexModelMaxReasoning = map[string]string{
	// These models support xhigh (maximum)
	"gpt-5.3-codex":     "xhigh",
	"gpt-5.2-codex":     "xhigh",
	"gpt-5.2":           "xhigh",
	"gpt-5.1-codex-max": "xhigh",
	// These models support up to high
	"gpt-5.1-codex":      "high",
	"gpt-5.1-codex-mini": "high",
	"gpt-5.1":            "high",
	"gpt-5-codex":        "high",
	"gpt-5-codex-mini":   "high",
	"gpt-5":              "high",
}

// ClaudeEffortLevels defines the Claude CLI effort levels.
// Claude Code CLI supports 4 levels: "low", "medium", "high" (default), "max" (Opus 4.6 only).
// Configured via CLAUDE_CODE_EFFORT_LEVEL env var.
const (
	ClaudeEffortLow    = "low"
	ClaudeEffortMedium = "medium"
	ClaudeEffortHigh   = "high"
	ClaudeEffortMax    = "max"
)

// ClaudeModelMaxEffort maps Claude models to their maximum supported effort level.
// Only Opus 4.6 supports effort in the CLI (4 levels: low/medium/high/max).
var ClaudeModelMaxEffort = map[string]string{
	"claude-opus-4-6": ClaudeEffortMax,
	"opus":            ClaudeEffortMax, // alias
}

// GetMaxReasoningEffort returns the maximum reasoning effort supported by a Codex model.
// Returns "high" as default if model is not found.
func GetMaxReasoningEffort(model string) string {
	if maxReasoning, ok := CodexModelMaxReasoning[model]; ok {
		return maxReasoning
	}
	return "high"
}

// GetMaxClaudeEffort returns the maximum effort level supported by a Claude model.
// Returns empty string if the model does not support effort.
func GetMaxClaudeEffort(model string) string {
	if maxEffort, ok := ClaudeModelMaxEffort[model]; ok {
		return maxEffort
	}
	return ""
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
		// Claude 4.6 (latest opus)
		"claude-opus-4-6", // Most powerful model, supports effort (low/medium/high/max)
		// Claude 4.5 family
		"claude-sonnet-4-5-20250929", // Best balance of intelligence, speed, and cost
		"claude-haiku-4-5-20251001",  // Fastest model with near-frontier performance
		// Claude 4 family
		"claude-opus-4-20250514",
		"claude-opus-4-1-20250805",
		"claude-sonnet-4-20250514",
		// Aliases (shortcuts accepted by claude CLI)
		"opus",   // Maps to latest opus model (currently Opus 4.6)
		"sonnet", // Maps to latest sonnet model
		"haiku",  // Maps to latest haiku model
	},
	AgentGemini: {
		// Gemini 2.5 family (stable, recommended)
		"gemini-2.5-pro",        // Most powerful, best for coding and agentic tasks
		"gemini-2.5-flash",      // Best price/performance balance with thinking
		"gemini-2.5-flash-lite", // Fast, low-cost, 1M context
		// Gemini 2.0 family (retiring March 2026)
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
		// Gemini 3 preview models
		"gemini-3-pro-preview",
		"gemini-3-flash-preview",
	},
	AgentCodex: {
		// GPT-5.3 family (latest, recommended)
		"gpt-5.3-codex", // Most advanced agentic coding model (default for Codex CLI)
		// GPT-5.2 family - supports xhigh reasoning
		"gpt-5.2-codex",
		"gpt-5.2", // Base GPT-5.2 model
		// GPT-5.1 family
		"gpt-5.1-codex-max",  // Maximum capability for extended tasks (xhigh reasoning)
		"gpt-5.1-codex",      // Code-optimized GPT-5.1 (max: high reasoning)
		"gpt-5.1-codex-mini", // Cost-effective codex (max: high reasoning)
		"gpt-5.1",            // Base GPT-5.1 model (max: high reasoning)
		// GPT-5 family
		"gpt-5-codex",      // GPT-5 codex version (max: high reasoning)
		"gpt-5-codex-mini", // GPT-5 codex mini (max: high reasoning)
		"gpt-5",            // Base GPT-5 model (max: high reasoning)
	},
	AgentCopilot: {
		// Anthropic Claude models (via Copilot) - from copilot --help
		"claude-sonnet-4.5", // Best balance, strong reasoning (default)
		"claude-opus-4.6",   // Latest opus - most powerful Claude, supports effort
		"claude-haiku-4.5",  // Fast, efficient
		"claude-sonnet-4",   // Previous gen sonnet
		// OpenAI GPT models (via Copilot) - from copilot --help
		"gpt-5.2-codex",      // Advanced agentic coding
		"gpt-5.2",            // Latest GPT-5.2
		"gpt-5.1-codex-max",  // Maximum capability codex
		"gpt-5.1-codex",      // Code-optimized GPT-5.1
		"gpt-5.1-codex-mini", // Small codex
		"gpt-5.1",            // Base GPT-5.1
		"gpt-5",              // Base GPT-5
		"gpt-5-mini",         // Small, fast GPT-5
		"gpt-4.1",            // Previous generation
		// Google Gemini models (via Copilot) - from copilot --help
		"gemini-3-pro-preview", // Gemini 3 Pro preview
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

// State backends
const (
	StateBackendSQLite = "sqlite"
	StateBackendJSON   = "json"
)

// StateBackends is the ordered list of state backends.
var StateBackends = []string{StateBackendSQLite, StateBackendJSON}

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

// Issue template languages
const (
	IssueLanguageEnglish    = "english"
	IssueLanguageSpanish    = "spanish"
	IssueLanguageFrench     = "french"
	IssueLanguageGerman     = "german"
	IssueLanguagePortuguese = "portuguese"
	IssueLanguageChinese    = "chinese"
	IssueLanguageJapanese   = "japanese"
)

// IssueLanguages is the ordered list of issue template languages.
var IssueLanguages = []string{
	IssueLanguageEnglish,
	IssueLanguageSpanish,
	IssueLanguageFrench,
	IssueLanguageGerman,
	IssueLanguagePortuguese,
	IssueLanguageChinese,
	IssueLanguageJapanese,
}

// Issue template tones
const (
	IssueToneProfessional = "professional"
	IssueToneCasual       = "casual"
	IssueToneTechnical    = "technical"
	IssueToneConcise      = "concise"
)

// IssueTones is the ordered list of issue template tones.
var IssueTones = []string{
	IssueToneProfessional,
	IssueToneCasual,
	IssueToneTechnical,
	IssueToneConcise,
}
