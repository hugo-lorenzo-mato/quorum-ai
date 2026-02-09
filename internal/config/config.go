package config

import (
	"fmt"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	Log         LogConfig         `mapstructure:"log" yaml:"log"`
	Trace       TraceConfig       `mapstructure:"trace" yaml:"trace"`
	Diagnostics DiagnosticsConfig `mapstructure:"diagnostics" yaml:"diagnostics"`
	Workflow    WorkflowConfig    `mapstructure:"workflow" yaml:"workflow"`
	Phases      PhasesConfig      `mapstructure:"phases" yaml:"phases"`
	Agents      AgentsConfig      `mapstructure:"agents" yaml:"agents"`
	State       StateConfig       `mapstructure:"state" yaml:"state"`
	Git         GitConfig         `mapstructure:"git" yaml:"git"`
	GitHub      GitHubConfig      `mapstructure:"github" yaml:"github"`
	Chat        ChatConfig        `mapstructure:"chat" yaml:"chat"`
	Report      ReportConfig      `mapstructure:"report" yaml:"report"`
	Issues      IssuesConfig      `mapstructure:"issues" yaml:"issues"`
}

// ChatConfig configures chat behavior in the TUI.
type ChatConfig struct {
	Timeout          string `mapstructure:"timeout" yaml:"timeout"`                     // Timeout for chat messages (e.g., "3m", "5m")
	ProgressInterval string `mapstructure:"progress_interval" yaml:"progress_interval"` // Interval for progress logs (e.g., "15s")
	Editor           string `mapstructure:"editor" yaml:"editor"`                       // Editor for file editing (e.g., "code", "nvim", "vim")
}

// LogConfig configures logging behavior.
type LogConfig struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
}

// TraceConfig configures trace mode output.
type TraceConfig struct {
	Mode            string   `mapstructure:"mode" yaml:"mode"`
	Dir             string   `mapstructure:"dir" yaml:"dir"`
	SchemaVersion   int      `mapstructure:"schema_version" yaml:"schema_version"`
	Redact          bool     `mapstructure:"redact" yaml:"redact"`
	RedactPatterns  []string `mapstructure:"redact_patterns" yaml:"redact_patterns"`
	RedactAllowlist []string `mapstructure:"redact_allowlist" yaml:"redact_allowlist"`
	MaxBytes        int64    `mapstructure:"max_bytes" yaml:"max_bytes"`
	TotalMaxBytes   int64    `mapstructure:"total_max_bytes" yaml:"total_max_bytes"`
	MaxFiles        int      `mapstructure:"max_files" yaml:"max_files"`
	IncludePhases   []string `mapstructure:"include_phases" yaml:"include_phases"`
}

// DiagnosticsConfig configures system diagnostics and crash recovery.
type DiagnosticsConfig struct {
	// Enabled activates the diagnostics subsystem.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// ResourceMonitoring configures periodic resource tracking.
	ResourceMonitoring ResourceMonitoringConfig `mapstructure:"resource_monitoring" yaml:"resource_monitoring"`
	// CrashDump configures crash dump generation on panic.
	CrashDump CrashDumpConfig `mapstructure:"crash_dump" yaml:"crash_dump"`
	// PreflightChecks configures pre-execution health checks.
	PreflightChecks PreflightConfig `mapstructure:"preflight_checks" yaml:"preflight_checks"`
}

// ResourceMonitoringConfig configures resource usage monitoring.
type ResourceMonitoringConfig struct {
	// Interval between resource snapshots (e.g., "30s", "1m").
	Interval string `mapstructure:"interval" yaml:"interval"`
	// FDThresholdPercent triggers warning when FD usage exceeds this percentage (0-100).
	FDThresholdPercent int `mapstructure:"fd_threshold_percent" yaml:"fd_threshold_percent"`
	// GoroutineThreshold triggers warning when goroutine count exceeds this.
	GoroutineThreshold int `mapstructure:"goroutine_threshold" yaml:"goroutine_threshold"`
	// MemoryThresholdMB triggers warning when heap memory exceeds this (in MB).
	MemoryThresholdMB int `mapstructure:"memory_threshold_mb" yaml:"memory_threshold_mb"`
	// HistorySize is the number of snapshots to retain for trend analysis.
	HistorySize int `mapstructure:"history_size" yaml:"history_size"`
}

// CrashDumpConfig configures crash dump generation.
type CrashDumpConfig struct {
	// Dir is the directory for crash dump files.
	Dir string `mapstructure:"dir" yaml:"dir"`
	// MaxFiles is the maximum number of crash dumps to retain.
	MaxFiles int `mapstructure:"max_files" yaml:"max_files"`
	// IncludeStack includes full goroutine stack traces in crash dumps.
	IncludeStack bool `mapstructure:"include_stack" yaml:"include_stack"`
	// IncludeEnv includes environment variables (redacted) in crash dumps.
	IncludeEnv bool `mapstructure:"include_env" yaml:"include_env"`
}

// PreflightConfig configures pre-execution health checks.
type PreflightConfig struct {
	// Enabled activates preflight checks before command execution.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// MinFreeFDPercent aborts execution if free FD percentage is below this.
	MinFreeFDPercent int `mapstructure:"min_free_fd_percent" yaml:"min_free_fd_percent"`
	// MinFreeMemoryMB aborts execution if estimated free memory is below this (in MB).
	MinFreeMemoryMB int `mapstructure:"min_free_memory_mb" yaml:"min_free_memory_mb"`
}

// WorkflowConfig configures workflow execution.
type WorkflowConfig struct {
	Timeout    string          `mapstructure:"timeout" yaml:"timeout"`
	MaxRetries int             `mapstructure:"max_retries" yaml:"max_retries"`
	DryRun     bool            `mapstructure:"dry_run" yaml:"dry_run"`
	DenyTools  []string        `mapstructure:"deny_tools" yaml:"deny_tools"`
	Heartbeat  HeartbeatConfig `mapstructure:"heartbeat" yaml:"heartbeat"`
}

// HeartbeatConfig configures the heartbeat system for zombie workflow detection.
type HeartbeatConfig struct {
	// Enabled activates the heartbeat system.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Interval is how often to write heartbeats (e.g., "30s").
	Interval string `mapstructure:"interval" yaml:"interval"`
	// StaleThreshold is when to consider a workflow zombie (e.g., "2m").
	StaleThreshold string `mapstructure:"stale_threshold" yaml:"stale_threshold"`
	// CheckInterval is how often to check for zombies (e.g., "60s").
	CheckInterval string `mapstructure:"check_interval" yaml:"check_interval"`
	// AutoResume enables automatic resume of zombie workflows.
	AutoResume bool `mapstructure:"auto_resume" yaml:"auto_resume"`
	// MaxResumes is the maximum auto-resume attempts per workflow.
	MaxResumes int `mapstructure:"max_resumes" yaml:"max_resumes"`
}

// PhasesConfig configures each workflow phase.
type PhasesConfig struct {
	Analyze AnalyzePhaseConfig `mapstructure:"analyze" yaml:"analyze"`
	Plan    PlanPhaseConfig    `mapstructure:"plan" yaml:"plan"`
	Execute ExecutePhaseConfig `mapstructure:"execute" yaml:"execute"`
}

// AnalyzePhaseConfig configures the analysis phase.
// Flow: refiner → multi-agent analysis → moderator (consensus) → synthesizer
// When SingleAgent.Enabled=true, flow becomes: refiner → single-agent analysis → synthesizer
type AnalyzePhaseConfig struct {
	// Timeout for the entire analysis phase (e.g., "2h").
	Timeout string `mapstructure:"timeout" yaml:"timeout"`
	// ProcessGracePeriod is how long to wait after an agent signals logical completion
	// before killing the process (e.g., "30s"). Default: 30s.
	ProcessGracePeriod string `mapstructure:"process_grace_period" yaml:"process_grace_period"`
	// Refiner refines and clarifies the prompt before analysis.
	Refiner RefinerConfig `mapstructure:"refiner" yaml:"refiner"`
	// Moderator evaluates consensus between agent analyses.
	Moderator ModeratorConfig `mapstructure:"moderator" yaml:"moderator"`
	// Synthesizer combines all analyses into a unified report.
	Synthesizer SynthesizerConfig `mapstructure:"synthesizer" yaml:"synthesizer"`
	// SingleAgent configures single-agent execution mode (bypasses multi-agent consensus).
	SingleAgent SingleAgentConfig `mapstructure:"single_agent" yaml:"single_agent"`
}

// PlanPhaseConfig configures the planning phase.
type PlanPhaseConfig struct {
	// Timeout for the entire planning phase (e.g., "2h").
	Timeout string `mapstructure:"timeout" yaml:"timeout"`
	// Synthesizer combines multiple agent plans into one (multi-agent planning).
	Synthesizer PlanSynthesizerConfig `mapstructure:"synthesizer" yaml:"synthesizer"`
}

// ExecutePhaseConfig configures the execution phase.
type ExecutePhaseConfig struct {
	// Timeout for the entire execution phase (e.g., "2h").
	Timeout string `mapstructure:"timeout" yaml:"timeout"`
}

// RefinerConfig configures prompt refinement before analysis.
type RefinerConfig struct {
	// Enabled enables/disables prompt refinement.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Agent specifies which agent to use for refinement.
	// Model is resolved from agents.<agent>.phase_models.refine or agents.<agent>.model.
	Agent string `mapstructure:"agent" yaml:"agent"`
}

// ModeratorConfig configures consensus moderation between agents.
type ModeratorConfig struct {
	// Enabled activates consensus moderation via an LLM.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Agent specifies which agent to use as moderator.
	// Model is resolved from agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Agent string `mapstructure:"agent" yaml:"agent"`
	// Threshold is the consensus score required to pass (0.0-1.0, default: 0.80).
	Threshold float64 `mapstructure:"threshold" yaml:"threshold"`
	// Thresholds provides adaptive thresholds based on task type.
	// Keys: "analysis", "design", "bugfix", "refactor". If a task type matches,
	// its threshold is used instead of the default Threshold.
	Thresholds map[string]float64 `mapstructure:"thresholds" yaml:"thresholds"`
	// MinSuccessfulAgents is the minimum number of agents that must succeed in a
	// given analysis/refinement round before continuing (default: 2).
	//
	// This is different from MinRounds/MaxRounds:
	// - MinSuccessfulAgents: per-round success requirement (availability)
	// - MinRounds/MaxRounds: number of moderator refinement rounds (quality loop)
	MinSuccessfulAgents int `mapstructure:"min_successful_agents" yaml:"min_successful_agents"`
	// MinRounds is the minimum refinement rounds before accepting consensus (default: 2).
	MinRounds int `mapstructure:"min_rounds" yaml:"min_rounds"`
	// MaxRounds limits the number of refinement rounds (default: 5).
	MaxRounds int `mapstructure:"max_rounds" yaml:"max_rounds"`
	// WarningThreshold logs a warning if consensus score drops below this (default: 0.30).
	WarningThreshold float64 `mapstructure:"warning_threshold" yaml:"warning_threshold"`
	// StagnationThreshold triggers early exit if score improvement is below this (default: 0.02).
	StagnationThreshold float64 `mapstructure:"stagnation_threshold" yaml:"stagnation_threshold"`
}

// SynthesizerConfig configures analysis synthesis.
type SynthesizerConfig struct {
	// Agent specifies which agent to use for synthesis.
	// Model is resolved from agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Agent string `mapstructure:"agent" yaml:"agent"`
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
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Agent is the name of the agent to use for single-agent analysis.
	// Required when Enabled is true.
	Agent string `mapstructure:"agent" yaml:"agent"`
	// Model is an optional override for the agent's default model.
	Model string `mapstructure:"model" yaml:"model"`
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
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Agent specifies which agent to use for plan synthesis.
	// Model is resolved from agents.<agent>.phase_models.plan or agents.<agent>.model.
	Agent string `mapstructure:"agent" yaml:"agent"`
}

// AgentsConfig configures available AI agents.
type AgentsConfig struct {
	Default  string      `mapstructure:"default" yaml:"default"`
	Claude   AgentConfig `mapstructure:"claude" yaml:"claude"`
	Gemini   AgentConfig `mapstructure:"gemini" yaml:"gemini"`
	Codex    AgentConfig `mapstructure:"codex" yaml:"codex"`
	Copilot  AgentConfig `mapstructure:"copilot" yaml:"copilot"`
	OpenCode AgentConfig `mapstructure:"opencode" yaml:"opencode"`
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

// EnabledAgentNames returns a slice of all enabled agent names.
func (c AgentsConfig) EnabledAgentNames() []string {
	var names []string
	agents := map[string]AgentConfig{
		"claude":   c.Claude,
		"gemini":   c.Gemini,
		"codex":    c.Codex,
		"copilot":  c.Copilot,
		"opencode": c.OpenCode,
	}
	for name, cfg := range agents {
		if cfg.Enabled {
			names = append(names, name)
		}
	}
	return names
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
	Enabled     bool              `mapstructure:"enabled" yaml:"enabled"`
	Path        string            `mapstructure:"path" yaml:"path"`
	Model       string            `mapstructure:"model" yaml:"model"`
	PhaseModels map[string]string `mapstructure:"phase_models" yaml:"phase_models"`
	// Phases controls which workflow phases/roles this agent participates in.
	// If nil or empty, agent is available for all phases (backward compatible).
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	Phases map[string]bool `mapstructure:"phases" yaml:"phases"`
	// ReasoningEffort is the default reasoning effort for all phases.
	// Codex values: none, minimal, low, medium, high, xhigh.
	// Claude values: low, medium, high, max (Opus 4.6 only).
	ReasoningEffort string `mapstructure:"reasoning_effort" yaml:"reasoning_effort"`
	// ReasoningEffortPhases allows per-phase overrides of reasoning effort.
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	ReasoningEffortPhases map[string]string `mapstructure:"reasoning_effort_phases" yaml:"reasoning_effort_phases"`
	// TokenDiscrepancyThreshold is the ratio for detecting token reporting errors.
	// If reported tokens differ from estimated by more than this factor, use estimated.
	// Default: 5 (reported must be within 1/5 to 5x of estimated). Set to 0 to disable.
	TokenDiscrepancyThreshold float64 `mapstructure:"token_discrepancy_threshold" yaml:"token_discrepancy_threshold"`
}

// IsEnabledForPhase returns true if the agent is enabled for the given phase.
// Uses strict opt-in model: phases must be explicitly set to true to be enabled.
// If phases map is empty or missing, the agent is enabled for NO phases.
func (c AgentConfig) IsEnabledForPhase(phase string) bool {
	if !c.Enabled {
		return false
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
	Path       string `mapstructure:"path" yaml:"path"`
	BackupPath string `mapstructure:"backup_path" yaml:"backup_path"`
	LockTTL    string `mapstructure:"lock_ttl" yaml:"lock_ttl"`
}

// GitConfig configures git operations with semantic grouping.
// Fields are organized by their lifecycle and scope:
// - Worktree: temporary environment during task execution
// - Task: incremental progress saving (per-task commits)
// - Finalization: final workflow delivery (push, PR, merge)
type GitConfig struct {
	Worktree     WorktreeConfig        `mapstructure:"worktree" yaml:"worktree"`
	Task         GitTaskConfig         `mapstructure:"task" yaml:"task"`
	Finalization GitFinalizationConfig `mapstructure:"finalization" yaml:"finalization"`
}

// WorktreeConfig configures temporary worktree management during execution.
type WorktreeConfig struct {
	// Dir is the directory where worktrees are created.
	Dir string `mapstructure:"dir" yaml:"dir"`
	// Mode controls when worktrees are created: always, parallel, or disabled.
	Mode string `mapstructure:"mode" yaml:"mode"`
	// AutoClean removes worktrees after task completion.
	// IMPORTANT: Requires Task.AutoCommit=true to prevent data loss.
	AutoClean bool `mapstructure:"auto_clean" yaml:"auto_clean"`
}

// GitTaskConfig configures per-task progress saving.
type GitTaskConfig struct {
	// AutoCommit commits changes after each task completes.
	// This ensures work is saved even if the workflow crashes.
	AutoCommit bool `mapstructure:"auto_commit" yaml:"auto_commit"`
}

// GitFinalizationConfig configures workflow result delivery.
// These settings control how the final workflow branch is pushed and merged.
// Note: Different from FinalizationConfig in context.go which is a runtime struct.
type GitFinalizationConfig struct {
	// AutoPush pushes the workflow branch to remote after all tasks complete.
	AutoPush bool `mapstructure:"auto_push" yaml:"auto_push"`
	// AutoPR creates a single pull request for the entire workflow.
	AutoPR bool `mapstructure:"auto_pr" yaml:"auto_pr"`
	// AutoMerge merges the PR automatically after creation.
	AutoMerge bool `mapstructure:"auto_merge" yaml:"auto_merge"`
	// PRBaseBranch is the target branch for PRs (empty = repository default).
	PRBaseBranch string `mapstructure:"pr_base_branch" yaml:"pr_base_branch"`
	// MergeStrategy for auto-merge: merge, squash, rebase (default: squash).
	MergeStrategy string `mapstructure:"merge_strategy" yaml:"merge_strategy"`
}

// GitHubConfig configures GitHub integration.
// Note: GitHub token should be provided via GITHUB_TOKEN or GH_TOKEN environment variable.
type GitHubConfig struct {
	Remote string `mapstructure:"remote" yaml:"remote"`
}

// IssuesConfig configures GitHub/GitLab issue generation.
type IssuesConfig struct {
	// Enabled activates issue generation feature.
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// Provider specifies the issue tracking system: github or gitlab.
	Provider string `mapstructure:"provider" yaml:"provider" json:"provider"`

	// AutoGenerate creates issues automatically after planning phase.
	AutoGenerate bool `mapstructure:"auto_generate" yaml:"auto_generate" json:"auto_generate"`

	// Timeout for issue generation operations (e.g., "3m", "5m").
	Timeout string `mapstructure:"timeout" yaml:"timeout" json:"timeout"`

	// Mode specifies issue creation mode: "direct" (gh CLI) or "agent" (LLM-based).
	Mode string `mapstructure:"mode" yaml:"mode" json:"mode"`

	// DraftDirectory is the directory for draft issue files (empty = default .quorum/issues/).
	DraftDirectory string `mapstructure:"draft_directory" yaml:"draft_directory" json:"draft_directory"`

	// Repository overrides auto-detected repository (format: "owner/repo").
	Repository string `mapstructure:"repository" yaml:"repository" json:"repository"`

	// ParentTemplate is the template for parent issues (empty = default template).
	ParentTemplate string `mapstructure:"parent_template" yaml:"parent_template" json:"parent_template"`

	// Template configures issue content generation.
	Template IssueTemplateConfig `mapstructure:"template" yaml:"template" json:"template"`

	// Labels to apply to all generated issues.
	Labels []string `mapstructure:"labels" yaml:"labels" json:"labels"`

	// Assignees for generated issues.
	Assignees []string `mapstructure:"assignees" yaml:"assignees" json:"assignees"`

	// GitLab contains GitLab-specific configuration.
	GitLab GitLabIssueConfig `mapstructure:"gitlab" yaml:"gitlab" json:"gitlab"`

	// Generator configures LLM-based issue generation.
	Generator IssueGeneratorConfig `mapstructure:"generator" yaml:"generator" json:"generator"`
}

// IssueTemplateConfig configures issue content formatting.
type IssueTemplateConfig struct {
	// Language for generated content (english, spanish, french, german, portuguese, chinese, japanese).
	Language string `mapstructure:"language" yaml:"language" json:"language"`

	// Tone of writing: formal, informal, technical, friendly.
	Tone string `mapstructure:"tone" yaml:"tone" json:"tone"`

	// IncludeDiagrams adds ASCII diagrams to issue body when available.
	IncludeDiagrams bool `mapstructure:"include_diagrams" yaml:"include_diagrams" json:"include_diagrams"`

	// TitleFormat specifies the issue title template.
	// Supports variables: {workflow_id}, {workflow_title}, {task_id}, {task_name}
	TitleFormat string `mapstructure:"title_format" yaml:"title_format" json:"title_format"`

	// BodyTemplateFile path to custom body template (relative to config directory).
	BodyTemplateFile string `mapstructure:"body_template_file" yaml:"body_template_file" json:"body_template_file"`

	// Convention name for style reference (e.g., "conventional-commits", "angular").
	Convention string `mapstructure:"convention" yaml:"convention" json:"convention"`

	// CustomInstructions are free-form instructions for LLM when generating content.
	// Users can specify tone, formatting preferences, diagram usage, conventions, etc.
	// Example: "Use bullet points, include code snippets, write in Spanish"
	CustomInstructions string `mapstructure:"custom_instructions" yaml:"custom_instructions" json:"custom_instructions"`
}

// GitLabIssueConfig contains GitLab-specific options.
type GitLabIssueConfig struct {
	// UseEpics groups sub-issues under an epic instead of linking.
	UseEpics bool `mapstructure:"use_epics" yaml:"use_epics" json:"use_epics"`

	// ProjectID is the GitLab project identifier (required for GitLab).
	ProjectID string `mapstructure:"project_id" yaml:"project_id" json:"project_id"`
}

// IssueGeneratorConfig configures LLM-based issue generation.
type IssueGeneratorConfig struct {
	// Enabled activates LLM-based issue generation.
	// When false, issues are generated by copying artifacts directly.
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// Agent to use for generation (claude, gemini, codex, etc.).
	Agent string `mapstructure:"agent" yaml:"agent" json:"agent"`

	// Model to use (optional, uses agent default if empty).
	Model string `mapstructure:"model" yaml:"model" json:"model"`

	// Summarize content instead of copying verbatim.
	Summarize bool `mapstructure:"summarize" yaml:"summarize" json:"summarize"`

	// MaxBodyLength limits the generated body length in characters.
	MaxBodyLength int `mapstructure:"max_body_length" yaml:"max_body_length" json:"max_body_length"`

	// ReasoningEffort controls the reasoning effort level for the generator agent.
	ReasoningEffort string `mapstructure:"reasoning_effort" yaml:"reasoning_effort" json:"reasoning_effort"`

	// Instructions are custom instructions for the generator agent when generating issue body.
	Instructions string `mapstructure:"instructions" yaml:"instructions" json:"instructions"`

	// TitleInstructions are custom instructions for the generator agent when generating issue title.
	TitleInstructions string `mapstructure:"title_instructions" yaml:"title_instructions" json:"title_instructions"`

	// Resilience configures retry and circuit breaker behavior for LLM calls.
	Resilience LLMResilienceConfig `mapstructure:"resilience" yaml:"resilience" json:"resilience"`

	// Validation configures issue content validation.
	Validation IssueValidationConfig `mapstructure:"validation" yaml:"validation" json:"validation"`

	// RateLimit configures rate limiting for GitHub API calls.
	RateLimit GitHubRateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`
}

// LLMResilienceConfig configures retry and circuit breaker behavior.
type LLMResilienceConfig struct {
	// Enabled activates resilience features (retry, circuit breaker).
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `mapstructure:"max_retries" yaml:"max_retries" json:"max_retries"`

	// InitialBackoff is the initial delay before first retry.
	InitialBackoff string `mapstructure:"initial_backoff" yaml:"initial_backoff" json:"initial_backoff"`

	// MaxBackoff is the maximum delay between retries.
	MaxBackoff string `mapstructure:"max_backoff" yaml:"max_backoff" json:"max_backoff"`

	// BackoffMultiplier is the exponential backoff multiplier.
	BackoffMultiplier float64 `mapstructure:"backoff_multiplier" yaml:"backoff_multiplier" json:"backoff_multiplier"`

	// FailureThreshold is the number of consecutive failures to open the circuit.
	FailureThreshold int `mapstructure:"failure_threshold" yaml:"failure_threshold" json:"failure_threshold"`

	// ResetTimeout is how long the circuit stays open before trying again.
	ResetTimeout string `mapstructure:"reset_timeout" yaml:"reset_timeout" json:"reset_timeout"`
}

// IssueValidationConfig configures issue content validation.
type IssueValidationConfig struct {
	// Enabled activates issue content validation.
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// SanitizeForbidden enables automatic removal of forbidden content.
	SanitizeForbidden bool `mapstructure:"sanitize_forbidden" yaml:"sanitize_forbidden" json:"sanitize_forbidden"`

	// RequiredSections lists sections that must be present in issues.
	RequiredSections []string `mapstructure:"required_sections" yaml:"required_sections" json:"required_sections"`

	// ForbiddenPatterns lists regex patterns that should not appear in issues.
	ForbiddenPatterns []string `mapstructure:"forbidden_patterns" yaml:"forbidden_patterns" json:"forbidden_patterns"`
}

// GitHubRateLimitConfig configures rate limiting for GitHub API.
type GitHubRateLimitConfig struct {
	// Enabled activates rate limiting.
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// MaxPerMinute is the maximum requests per minute.
	MaxPerMinute int `mapstructure:"max_per_minute" yaml:"max_per_minute" json:"max_per_minute"`
}

// Validate validates the issues configuration.
func (c *IssuesConfig) Validate() error {
	// Validate provider
	if c.Enabled && c.Provider != "github" && c.Provider != "gitlab" && c.Provider != "" {
		return fmt.Errorf("issues.provider must be 'github' or 'gitlab'")
	}

	// Validate tone (must match core.IssueTones)
	validTones := map[string]bool{
		"professional": true, "casual": true, "technical": true, "concise": true, "": true,
	}
	if !validTones[c.Template.Tone] {
		return fmt.Errorf("issues.template.tone must be one of: professional, casual, technical, concise")
	}

	// Validate language (must match core.IssueLanguages)
	validLanguages := map[string]bool{
		"english": true, "spanish": true, "french": true, "german": true,
		"portuguese": true, "chinese": true, "japanese": true, "": true,
	}
	language := normalizeIssueLanguage(c.Template.Language)
	if !validLanguages[language] {
		return fmt.Errorf("issues.template.language must be one of: english, spanish, french, german, portuguese, chinese, japanese")
	}

	// GitLab requires project_id
	if c.Enabled && c.Provider == "gitlab" && c.GitLab.ProjectID == "" {
		return fmt.Errorf("issues.gitlab.project_id is required when provider is 'gitlab'")
	}

	return nil
}

// ReportConfig configures markdown report generation.
type ReportConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	BaseDir    string `mapstructure:"base_dir" yaml:"base_dir"`
	UseUTC     bool   `mapstructure:"use_utc" yaml:"use_utc"`
	IncludeRaw bool   `mapstructure:"include_raw" yaml:"include_raw"`
}

// ExtractAgentPhases extracts the enabled phases for each agent.
// Returns a map of agent name -> list of enabled phases.
// An empty list means no phases are enabled (strict allowlist).
func (c *Config) ExtractAgentPhases() map[string][]string {
	phases := make(map[string][]string)
	agents := map[string]AgentConfig{
		"claude":   c.Agents.Claude,
		"gemini":   c.Agents.Gemini,
		"codex":    c.Agents.Codex,
		"copilot":  c.Agents.Copilot,
		"opencode": c.Agents.OpenCode,
	}
	for name, cfg := range agents {
		if !cfg.Enabled {
			continue
		}
		// Build list of enabled phases (strict allowlist).
		enabledPhases := make([]string, 0)
		for phase, enabled := range cfg.Phases {
			if enabled {
				enabledPhases = append(enabledPhases, phase)
			}
		}
		phases[name] = enabledPhases
	}
	return phases
}
