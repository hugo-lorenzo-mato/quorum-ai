package config

// Config holds all application configuration.
type Config struct {
	Log      LogConfig      `mapstructure:"log"`
	Trace    TraceConfig    `mapstructure:"trace"`
	Workflow WorkflowConfig `mapstructure:"workflow"`
	Phases   PhasesConfig   `mapstructure:"phases"`
	Agents   AgentsConfig   `mapstructure:"agents"`
	State    StateConfig    `mapstructure:"state"`
	Git      GitConfig      `mapstructure:"git"`
	GitHub   GitHubConfig   `mapstructure:"github"`
	Costs    CostsConfig    `mapstructure:"costs"`
	Chat     ChatConfig     `mapstructure:"chat"`
	Report   ReportConfig   `mapstructure:"report"`
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
	File   string `mapstructure:"file"`
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

// WorkflowConfig configures workflow execution.
type WorkflowConfig struct {
	Timeout    string   `mapstructure:"timeout"`
	MaxRetries int      `mapstructure:"max_retries"`
	DryRun     bool     `mapstructure:"dry_run"`
	Sandbox    bool     `mapstructure:"sandbox"`
	DenyTools  []string `mapstructure:"deny_tools"`
}

// PhasesConfig configures each workflow phase.
type PhasesConfig struct {
	Analyze AnalyzePhaseConfig `mapstructure:"analyze"`
	Plan    PlanPhaseConfig    `mapstructure:"plan"`
	Execute ExecutePhaseConfig `mapstructure:"execute"`
}

// AnalyzePhaseConfig configures the analysis phase.
// Flow: refiner → multi-agent analysis → moderator (consensus) → synthesizer
type AnalyzePhaseConfig struct {
	// Timeout for the entire analysis phase (e.g., "2h").
	Timeout string `mapstructure:"timeout"`
	// Refiner refines and clarifies the prompt before analysis.
	Refiner RefinerConfig `mapstructure:"refiner"`
	// Moderator evaluates consensus between agent analyses.
	Moderator ModeratorConfig `mapstructure:"moderator"`
	// Synthesizer combines all analyses into a unified report.
	Synthesizer SynthesizerConfig `mapstructure:"synthesizer"`
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
	Agent string `mapstructure:"agent"`
	// Model overrides the agent's default model for refinement.
	// If empty, uses agents.<agent>.phase_models.refine or agents.<agent>.model.
	Model string `mapstructure:"model"`
}

// ModeratorConfig configures consensus moderation between agents.
type ModeratorConfig struct {
	// Enabled activates consensus moderation via an LLM.
	Enabled bool `mapstructure:"enabled"`
	// Agent specifies which agent to use as moderator.
	Agent string `mapstructure:"agent"`
	// Model overrides the agent's default model for moderation.
	// If empty, uses agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Model string `mapstructure:"model"`
	// Threshold is the consensus score required to pass (0.0-1.0, default: 0.90).
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
	Agent string `mapstructure:"agent"`
	// Model overrides the agent's default model for synthesis.
	// If empty, uses agents.<agent>.phase_models.analyze or agents.<agent>.model.
	Model string `mapstructure:"model"`
}

// PlanSynthesizerConfig configures multi-agent plan synthesis.
type PlanSynthesizerConfig struct {
	// Enabled controls whether multi-agent planning is used.
	// When false (default), uses single-agent planning.
	Enabled bool `mapstructure:"enabled"`
	// Agent specifies which agent to use for plan synthesis.
	Agent string `mapstructure:"agent"`
	// Model overrides the agent's default model for plan synthesis.
	// If empty, uses agents.<agent>.phase_models.plan or agents.<agent>.model.
	Model string `mapstructure:"model"`
}

// AgentsConfig configures available AI agents.
type AgentsConfig struct {
	Default string      `mapstructure:"default"`
	Claude  AgentConfig `mapstructure:"claude"`
	Gemini  AgentConfig `mapstructure:"gemini"`
	Codex   AgentConfig `mapstructure:"codex"`
	Copilot AgentConfig `mapstructure:"copilot"`
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
	default:
		return nil
	}
}

// ListEnabledForPhase returns agent names that are enabled for the given phase.
func (c AgentsConfig) ListEnabledForPhase(phase string) []string {
	var result []string
	agents := map[string]AgentConfig{
		"claude":  c.Claude,
		"gemini":  c.Gemini,
		"codex":   c.Codex,
		"copilot": c.Copilot,
	}
	for name, cfg := range agents {
		if cfg.IsEnabledForPhase(phase) {
			result = append(result, name)
		}
	}
	return result
}

// AgentConfig configures a single AI agent.
type AgentConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	Path        string            `mapstructure:"path"`
	Model       string            `mapstructure:"model"`
	PhaseModels map[string]string `mapstructure:"phase_models"`
	// Phases controls which workflow phases this agent participates in.
	// If nil or empty, agent is available for all phases (backward compatible).
	// Keys: "refine", "analyze", "plan", "execute"
	Phases map[string]bool `mapstructure:"phases"`
	// ReasoningEffort is the default reasoning effort for all phases (Codex-specific).
	// Valid values: minimal, low, medium, high, xhigh.
	ReasoningEffort string `mapstructure:"reasoning_effort"`
	// ReasoningEffortPhases allows per-phase overrides of reasoning effort.
	// Keys: "refine", "analyze", "moderate", "synthesize", "plan", "execute"
	ReasoningEffortPhases map[string]string `mapstructure:"reasoning_effort_phases"`
}

// IsEnabledForPhase returns true if the agent is enabled for the given phase.
func (c AgentConfig) IsEnabledForPhase(phase string) bool {
	if !c.Enabled {
		return false
	}
	if len(c.Phases) == 0 {
		return true // No phase restrictions = enabled for all
	}
	enabled, exists := c.Phases[phase]
	if !exists {
		return true // Phase not specified = enabled (opt-out model)
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
	Path       string `mapstructure:"path"`
	BackupPath string `mapstructure:"backup_path"`
	LockTTL    string `mapstructure:"lock_ttl"`
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
type GitHubConfig struct {
	Token  string `mapstructure:"token"`
	Remote string `mapstructure:"remote"`
}

// CostsConfig configures cost limits and alerts.
type CostsConfig struct {
	MaxPerWorkflow float64 `mapstructure:"max_per_workflow"`
	MaxPerTask     float64 `mapstructure:"max_per_task"`
	AlertThreshold float64 `mapstructure:"alert_threshold"`
}

// ReportConfig configures markdown report generation.
type ReportConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	BaseDir    string `mapstructure:"base_dir"`
	UseUTC     bool   `mapstructure:"use_utc"`
	IncludeRaw bool   `mapstructure:"include_raw"`
}
