package config

import "strings"

// Config holds all application configuration.
type Config struct {
	Log         LogConfig         `mapstructure:"log"`
	Trace       TraceConfig       `mapstructure:"trace"`
	Diagnostics DiagnosticsConfig `mapstructure:"diagnostics"`
	Workflow    WorkflowConfig    `mapstructure:"workflow"`
	Phases      PhasesConfig      `mapstructure:"phases"`
	Agents      AgentsConfig      `mapstructure:"agents"`
	State       StateConfig       `mapstructure:"state"`
	Git         GitConfig         `mapstructure:"git"`
	GitHub      GitHubConfig      `mapstructure:"github"`
	Chat        ChatConfig        `mapstructure:"chat"`
	Report      ReportConfig      `mapstructure:"report"`
}

// ChatConfig configures chat behavior in the TUI.
type ChatConfig struct {
	Timeout          string `mapstructure:"timeout"`           // Timeout for chat messages (e.g., "3m", "5m")
	ProgressInterval string `mapstructure:"progress_interval"` // Interval for progress logs (e.g., "15s")
	Editor           string `mapstructure:"editor"`            // Editor for file editing (e.g., "code", "nvim", "vim")
}

// LogConfig configures logging behavior.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// TraceConfig configures trace mode output.
type TraceConfig struct {
	Mode            string   `mapstructure:"mode"`
	Dir             string   `mapstructure:"dir"`
	SchemaVersion   int      `mapstructure:"schema_version"`
	Redact          bool     `mapstructure:"redact"`
	RedactPatterns  []string `mapstructure:"redact_patterns"`
	RedactAllowlist []string `mapstructure:"redact_allowlist"`
	MaxBytes        int64    `mapstructure:"max_bytes"`
	TotalMaxBytes   int64    `mapstructure:"total_max_bytes"`
	MaxFiles        int      `mapstructure:"max_files"`
	IncludePhases   []string `mapstructure:"include_phases"`
}

// DiagnosticsConfig configures system diagnostics and crash recovery.
type DiagnosticsConfig struct {
	// Enabled activates the diagnostics subsystem.
	Enabled bool `mapstructure:"enabled"`
	// ResourceMonitoring configures periodic resource tracking.
	ResourceMonitoring ResourceMonitoringConfig `mapstructure:"resource_monitoring"`
	// CrashDump configures crash dump generation on panic.
	CrashDump CrashDumpConfig `mapstructure:"crash_dump"`
	// PreflightChecks configures pre-execution health checks.
	PreflightChecks PreflightConfig `mapstructure:"preflight_checks"`
}

// ResourceMonitoringConfig configures resource usage monitoring.
type ResourceMonitoringConfig struct {
	// Interval between resource snapshots (e.g., "30s", "1m").
	Interval string `mapstructure:"interval"`
	// FDThresholdPercent triggers warning when FD usage exceeds this percentage (0-100).
	FDThresholdPercent int `mapstructure:"fd_threshold_percent"`
	// GoroutineThreshold triggers warning when goroutine count exceeds this.
	GoroutineThreshold int `mapstructure:"goroutine_threshold"`
	// MemoryThresholdMB triggers warning when heap memory exceeds this (in MB).
	MemoryThresholdMB int `mapstructure:"memory_threshold_mb"`
	// HistorySize is the number of snapshots to retain for trend analysis.
	HistorySize int `mapstructure:"history_size"`
}

// CrashDumpConfig configures crash dump generation.
type CrashDumpConfig struct {
	// Dir is the directory for crash dump files.
	Dir string `mapstructure:"dir"`
	// MaxFiles is the maximum number of crash dumps to retain.
	MaxFiles int `mapstructure:"max_files"`
	// IncludeStack includes full goroutine stack traces in crash dumps.
	IncludeStack bool `mapstructure:"include_stack"`
	// IncludeEnv includes environment variables (redacted) in crash dumps.
	IncludeEnv bool `mapstructure:"include_env"`
}

// PreflightConfig configures pre-execution health checks.
type PreflightConfig struct {
	// Enabled activates preflight checks before command execution.
	Enabled bool `mapstructure:"enabled"`
	// MinFreeFDPercent aborts execution if free FD percentage is below this.
	MinFreeFDPercent int `mapstructure:"min_free_fd_percent"`
	// MinFreeMemoryMB aborts execution if estimated free memory is below this (in MB).
	MinFreeMemoryMB int `mapstructure:"min_free_memory_mb"`
}

// WorkflowConfig configures workflow execution.
type WorkflowConfig struct {
	Timeout    string          `mapstructure:"timeout"`
	MaxRetries int             `mapstructure:"max_retries"`
	DryRun     bool            `mapstructure:"dry_run"`
	Sandbox    bool            `mapstructure:"sandbox"`
	DenyTools  []string        `mapstructure:"deny_tools"`
	Heartbeat  HeartbeatConfig `mapstructure:"heartbeat"`
}

// HeartbeatConfig configures the heartbeat system for zombie workflow detection.
type HeartbeatConfig struct {
	// Enabled activates the heartbeat system.
	Enabled bool `mapstructure:"enabled"`
	// Interval is how often to write heartbeats (e.g., "30s").
	Interval string `mapstructure:"interval"`
	// StaleThreshold is when to consider a workflow zombie (e.g., "2m").
	StaleThreshold string `mapstructure:"stale_threshold"`
	// CheckInterval is how often to check for zombies (e.g., "60s").
	CheckInterval string `mapstructure:"check_interval"`
	// AutoResume enables automatic resume of zombie workflows.
	AutoResume bool `mapstructure:"auto_resume"`
	// MaxResumes is the maximum auto-resume attempts per workflow.
	MaxResumes int `mapstructure:"max_resumes"`
}

// PhasesConfig configures each workflow phase.
type PhasesConfig struct {
	Analyze AnalyzePhaseConfig `mapstructure:"analyze"`
	Plan    PlanPhaseConfig    `mapstructure:"plan"`
	Execute ExecutePhaseConfig `mapstructure:"execute"`
}

// AnalyzePhaseConfig configures the analysis phase.
// Flow: refiner → multi-agent analysis → moderator (consensus) → synthesizer
// When SingleAgent.Enabled=true, flow becomes: refiner → single-agent analysis → synthesizer
type AnalyzePhaseConfig struct {
	// Timeout for the entire analysis phase (e.g., "2h").
	Timeout string `mapstructure:"timeout"`
	// Refiner refines and clarifies the prompt before analysis.
	Refiner RefinerConfig `mapstructure:"refiner"`
	// Moderator evaluates consensus between agent analyses.
	Moderator ModeratorConfig `mapstructure:"moderator"`
	// Synthesizer combines all analyses into a unified report.
	Synthesizer SynthesizerConfig `mapstructure:"synthesizer"`
	// SingleAgent configures single-agent execution mode (bypasses multi-agent consensus).
	SingleAgent SingleAgentConfig `mapstructure:"single_agent"`
}

// PlanPhaseConfig configures the planning phase.
type PlanPhaseConfig struct {
	// Timeout for the entire planning phase (e.g., "2h").
	Timeout string `mapstructure:"timeout"`
	// Synthesizer combines multiple agent plans into one (multi-agent planning).
	Synthesizer PlanSynthesizerConfig `mapstructure:"synthesizer"`
}

// ExecutePhaseConfig configures the execution phase.
type ExecutePhaseConfig struct {
	// Timeout for the entire execution phase (e.g., "2h").
	Timeout string `mapstructure:"timeout"`
}

// RefinerConfig configures prompt refinement before analysis.
type RefinerConfig struct {
	// Enabled enables/disables prompt refinement.
	Enabled bool `mapstructure:"enabled"`
	// Agent specifies which agent to use for refinement.
	// Model is resolved from agents.<agent>.phase_models.refine or agents.<agent>.model.
	Agent string `mapstructure:"agent"`
}

// ModeratorConfig configures consensus moderation between agents.
type ModeratorConfig struct {
	// Enabled activates consensus moderation via an LLM.
	Enabled bool `mapstructure:"enabled"`
	// Agent specifies which agent to use as moderator.
	// Model is resolved from agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Agent string `mapstructure:"agent"`
	// Threshold is the consensus score required to pass (0.0-1.0, default: 0.80).
	Threshold float64 `mapstructure:"threshold"`
	// MinRounds is the minimum refinement rounds before accepting consensus (default: 2).
	MinRounds int `mapstructure:"min_rounds"`
	// MaxRounds limits the number of refinement rounds (default: 5).
	MaxRounds int `mapstructure:"max_rounds"`
	// AbortThreshold triggers human review if score drops below this (default: 0.30).
	AbortThreshold float64 `mapstructure:"abort_threshold"`
	// StagnationThreshold triggers early exit if score improvement is below this (default: 0.02).
	StagnationThreshold float64 `mapstructure:"stagnation_threshold"`
}

// SynthesizerConfig configures analysis synthesis.
type SynthesizerConfig struct {
	// Agent specifies which agent to use for synthesis.
	// Model is resolved from agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Agent string `mapstructure:"agent"`
}

// ExecutionMode defines how workflow phases execute agents.
// This determines whether the multi-agent consensus mechanism is used
// or if a single agent handles all phases independently.
type ExecutionMode string

const (
	// ExecutionModeMultiAgent uses multiple agents with consensus mechanism.
	// This is the default mode where agents iterate through V1→V2→...→Vn
	// with a moderator evaluating consensus and a synthesizer consolidating results.
	// Best for: Complex features, critical code, architectural decisions.
	// Typical execution time: 5-15 minutes.
	ExecutionModeMultiAgent ExecutionMode = "multi_agent"

	// ExecutionModeSingleAgent uses a single agent without iteration.
	// Bypasses the moderator and synthesizer, running each phase with one agent.
	// Best for: Simple tasks, bug fixes, documentation, quick iterations.
	// Typical execution time: 1-3 minutes.
	ExecutionModeSingleAgent ExecutionMode = "single_agent"
)

// IsValid checks if the execution mode is a known valid value.
// An empty string is considered valid and defaults to multi-agent mode.
func (m ExecutionMode) IsValid() bool {
	switch m {
	case ExecutionModeMultiAgent, ExecutionModeSingleAgent, "":
		return true
	default:
		return false
	}
}

// IsSingleAgent returns true if this mode represents single-agent execution.
func (m ExecutionMode) IsSingleAgent() bool {
	return m == ExecutionModeSingleAgent
}

// String returns the string representation of the execution mode.
func (m ExecutionMode) String() string {
	return string(m)
}

// DefaultExecutionMode returns the default execution mode (multi-agent).
func DefaultExecutionMode() ExecutionMode {
	return ExecutionModeMultiAgent
}

// SingleAgentConfig configures single-agent execution mode for the analyze phase.
// When enabled, the analysis phase runs with a single agent, bypassing the
// multi-agent consensus mechanism. This is useful for simpler tasks that don't
// require multiple perspectives.
type SingleAgentConfig struct {
	// Enabled activates single-agent mode. When true, the moderator is ignored
	// and only the specified agent is used for analysis.
	Enabled bool `mapstructure:"enabled"`
	// Agent is the name of the agent to use for single-agent analysis.
	// Required when Enabled is true.
	Agent string `mapstructure:"agent"`
	// Model is an optional override for the agent's default model.
	Model string `mapstructure:"model"`
}

// IsValid returns true if the SingleAgentConfig is properly configured.
// A valid config either has Enabled=false, or has Enabled=true with a non-empty Agent.
func (c SingleAgentConfig) IsValid() bool {
	if !c.Enabled {
		return true
	}
	return strings.TrimSpace(c.Agent) != ""
}

// PlanSynthesizerConfig configures multi-agent plan synthesis.
type PlanSynthesizerConfig struct {
	// Enabled controls whether multi-agent planning is used.
	// When false (default), uses single-agent planning.
	Enabled bool `mapstructure:"enabled"`
	// Agent specifies which agent to use for plan synthesis.
	// Model is resolved from agents.<agent>.phase_models.plan or agents.<agent>.model.
	Agent string `mapstructure:"agent"`
}

// AgentsConfig configures available AI agents.
type AgentsConfig struct {
	Default  string      `mapstructure:"default"`
	Claude   AgentConfig `mapstructure:"claude"`
	Gemini   AgentConfig `mapstructure:"gemini"`
	Codex    AgentConfig `mapstructure:"codex"`
	Copilot  AgentConfig `mapstructure:"copilot"`
	OpenCode AgentConfig `mapstructure:"opencode"`
}

// GetAgentConfig returns the config for a named agent, or nil if not found.
func (c AgentsConfig) GetAgentConfig(name string) *AgentConfig {
	switch name {
	case "claude":
		return &c.Claude
	case "gemini":
		return &c.Gemini
	case "codex":
		return &c.Codex
	case "copilot":
		return &c.Copilot
	case "opencode":
		return &c.OpenCode
	default:
		return nil
	}
}

// ListEnabledForPhase returns agent names that are enabled for the given phase.
func (c AgentsConfig) ListEnabledForPhase(phase string) []string {
	var result []string
	agents := map[string]AgentConfig{
		"claude":   c.Claude,
		"gemini":   c.Gemini,
		"codex":    c.Codex,
		"copilot":  c.Copilot,
		"opencode": c.OpenCode,
	}
	for name, cfg := range agents {
		if cfg.IsEnabledForPhase(phase) {
			result = append(result, name)
		}
	}
	return result
}

// AgentConfig configures a single AI agent.
//
// Agent names (the key in the agents map) are aliases - you can use any name.
// The actual CLI type is determined by the 'path' field or built-in mappings.
// This allows defining multiple agent entries using the same CLI but with
// different models, which is useful for multi-agent analysis with CLIs like
// copilot that support multiple models:
//
//	agents:
//	  copilot-claude:    # Alias - can be any name
//	    path: copilot    # Determines the CLI type
//	    model: claude-sonnet-4-5
//	  copilot-gpt:       # Another alias using same CLI
//	    path: copilot
//	    model: gpt-5
type AgentConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Path        string            `mapstructure:"path"`
	Model       string            `mapstructure:"model"`
	PhaseModels map[string]string `mapstructure:"phase_models"`
	// Phases controls which workflow phases/roles this agent participates in.
	// If nil or empty, agent is available for all phases (backward compatible).
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	Phases map[string]bool `mapstructure:"phases"`
	// ReasoningEffort is the default reasoning effort for all phases (Codex-specific).
	// Valid values: minimal, low, medium, high, xhigh.
	ReasoningEffort string `mapstructure:"reasoning_effort"`
	// ReasoningEffortPhases allows per-phase overrides of reasoning effort.
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	ReasoningEffortPhases map[string]string `mapstructure:"reasoning_effort_phases"`
	// TokenDiscrepancyThreshold is the ratio for detecting token reporting errors.
	// If reported tokens differ from estimated by more than this factor, use estimated.
	// Default: 5 (reported must be within 1/5 to 5x of estimated). Set to 0 to disable.
	TokenDiscrepancyThreshold float64 `mapstructure:"token_discrepancy_threshold"`
}

// IsEnabledForPhase returns true if the agent is enabled for the given phase.
// Uses opt-in model: phases must be explicitly set to true to be enabled.
// If phases map is empty, agent is enabled for all phases (backward compatible).
func (c AgentConfig) IsEnabledForPhase(phase string) bool {
	if !c.Enabled {
		return false
	}
	if len(c.Phases) == 0 {
		return true // No phase restrictions = enabled for all
	}
	enabled, exists := c.Phases[phase]
	if !exists {
		return false // Phase not specified = disabled (opt-in model)
	}
	return enabled
}

// GetModelForPhase returns the model to use for a given phase.
// Priority: phase_models[phase] > model (default).
func (c AgentConfig) GetModelForPhase(phase string) string {
	if c.PhaseModels != nil {
		if model, ok := c.PhaseModels[phase]; ok && model != "" {
			return model
		}
	}
	return c.Model
}

// GetReasoningEffortForPhase returns the reasoning effort for a given phase.
// Priority: phase-specific > default > empty (adapter uses hardcoded defaults).
func (c AgentConfig) GetReasoningEffortForPhase(phase string) string {
	// Check phase-specific override first
	if c.ReasoningEffortPhases != nil {
		if effort, ok := c.ReasoningEffortPhases[phase]; ok && effort != "" {
			return effort
		}
	}
	// Fall back to default
	return c.ReasoningEffort
}

// StateConfig configures state persistence.
type StateConfig struct {
	Backend    string `mapstructure:"backend"` // Backend type: "sqlite" (default) or "json"
	Path       string `mapstructure:"path"`
	BackupPath string `mapstructure:"backup_path"`
	LockTTL    string `mapstructure:"lock_ttl"`
}

// EffectiveBackend returns the normalized backend value.
// Returns "sqlite" if Backend is empty or unset.
func (s *StateConfig) EffectiveBackend() string {
	backend := strings.ToLower(strings.TrimSpace(s.Backend))
	if backend == "" {
		return "sqlite"
	}
	return backend
}

// GitConfig configures git operations.
type GitConfig struct {
	WorktreeDir  string `mapstructure:"worktree_dir"`
	AutoClean    bool   `mapstructure:"auto_clean"`
	WorktreeMode string `mapstructure:"worktree_mode"`
	// AutoCommit commits changes after each task completes.
	AutoCommit bool `mapstructure:"auto_commit"`
	// AutoPush pushes the task branch to remote after commit.
	AutoPush bool `mapstructure:"auto_push"`
	// AutoPR creates a pull request for each task branch.
	AutoPR bool `mapstructure:"auto_pr"`
	// AutoMerge merges the PR automatically after creation.
	AutoMerge bool `mapstructure:"auto_merge"`
	// PRBaseBranch is the target branch for PRs (default: current branch).
	PRBaseBranch string `mapstructure:"pr_base_branch"`
	// MergeStrategy for auto-merge: merge, squash, rebase (default: squash).
	MergeStrategy string `mapstructure:"merge_strategy"`
}

// GitHubConfig configures GitHub integration.
// Note: GitHub token should be provided via GITHUB_TOKEN or GH_TOKEN environment variable.
type GitHubConfig struct {
	Remote string `mapstructure:"remote"`
}

// ReportConfig configures markdown report generation.
type ReportConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	BaseDir    string `mapstructure:"base_dir"`
	UseUTC     bool   `mapstructure:"use_utc"`
	IncludeRaw bool   `mapstructure:"include_raw"`
}
