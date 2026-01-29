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

// Reasoning effort levels
const (
	ReasoningMinimal = "minimal"
	ReasoningLow     = "low"
	ReasoningMedium  = "medium"
	ReasoningHigh    = "high"
	ReasoningXHigh   = "xhigh"
)

// ReasoningEfforts is the ordered list of reasoning effort levels.
var ReasoningEfforts = []string{
	ReasoningMinimal,
	ReasoningLow,
	ReasoningMedium,
	ReasoningHigh,
	ReasoningXHigh,
}

// ValidReasoningEfforts is a map for O(1) reasoning effort validation.
var ValidReasoningEfforts = map[string]bool{
	ReasoningMinimal: true,
	ReasoningLow:     true,
	ReasoningMedium:  true,
	ReasoningHigh:    true,
	ReasoningXHigh:   true,
}

// IsValidReasoningEffort checks if the given reasoning effort is valid.
func IsValidReasoningEffort(effort string) bool {
	return ValidReasoningEfforts[effort]
}

// AgentsWithReasoning lists agents that support extended thinking/reasoning effort.
var AgentsWithReasoning = []string{
	AgentCodex,
	AgentCopilot,
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
