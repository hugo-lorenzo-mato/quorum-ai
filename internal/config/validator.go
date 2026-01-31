package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("config validation: %s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors collects multiple validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator validates configuration.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	return &Validator{
		errors: make(ValidationErrors, 0),
	}
}

// Validate validates the entire configuration.
func (v *Validator) Validate(cfg *Config) error {
	v.validateLog(&cfg.Log)
	v.validateTrace(&cfg.Trace)
	v.validateWorkflow(&cfg.Workflow)
	v.validateAgents(&cfg.Agents)
	v.validatePhases(&cfg.Phases, &cfg.Agents)
	v.validateState(&cfg.State)
	v.validateGit(&cfg.Git)
	v.validateGitHub(&cfg.GitHub)

	if len(v.errors) > 0 {
		return v.errors
	}
	return nil
}

// Errors returns the collected validation errors.
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

func (v *Validator) addError(field string, value interface{}, msg string) {
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Value:   value,
		Message: msg,
	})
}

func (v *Validator) validateLog(cfg *LogConfig) {
	validLevels := map[string]bool{
		core.LogDebug: true, core.LogInfo: true, core.LogWarn: true, core.LogError: true,
	}
	if !validLevels[cfg.Level] {
		v.addError("log.level", cfg.Level, "must be one of: debug, info, warn, error")
	}

	validFormats := map[string]bool{
		core.LogFormatAuto: true, core.LogFormatText: true, core.LogFormatJSON: true,
	}
	if !validFormats[cfg.Format] {
		v.addError("log.format", cfg.Format, "must be one of: auto, text, json")
	}
}

func (v *Validator) validateTrace(cfg *TraceConfig) {
	validModes := map[string]bool{
		core.TraceModeOff: true, core.TraceModeSummary: true, core.TraceModeFull: true,
	}
	if !validModes[cfg.Mode] {
		v.addError("trace.mode", cfg.Mode, "must be one of: off, summary, full")
	}

	if cfg.Dir == "" {
		v.addError("trace.dir", cfg.Dir, "directory required")
	} else if !isValidPath(cfg.Dir) {
		v.addError("trace.dir", cfg.Dir, "invalid directory path")
	}

	if cfg.SchemaVersion <= 0 {
		v.addError("trace.schema_version", cfg.SchemaVersion, "must be positive")
	}

	if cfg.MaxBytes <= 0 {
		v.addError("trace.max_bytes", cfg.MaxBytes, "must be positive")
	}

	if cfg.TotalMaxBytes <= 0 {
		v.addError("trace.total_max_bytes", cfg.TotalMaxBytes, "must be positive")
	}

	if cfg.TotalMaxBytes < cfg.MaxBytes {
		v.addError("trace.total_max_bytes", cfg.TotalMaxBytes, "must be >= trace.max_bytes")
	}

	if cfg.MaxFiles <= 0 {
		v.addError("trace.max_files", cfg.MaxFiles, "must be positive")
	}

	if len(cfg.IncludePhases) > 0 {
		validPhases := map[string]bool{
			string(core.PhaseRefine): true, string(core.PhaseAnalyze): true, string(core.PhasePlan): true, string(core.PhaseExecute): true,
		}
		for _, phase := range cfg.IncludePhases {
			if !validPhases[phase] {
				v.addError("trace.include_phases", phase, "unknown phase")
			}
		}
	}
}

func (v *Validator) validateWorkflow(cfg *WorkflowConfig) {
	if cfg.Timeout == "" {
		cfg.Timeout = "12h"
	}
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		v.addError("workflow.timeout", cfg.Timeout, "invalid duration format")
	}

	if cfg.MaxRetries < 0 || cfg.MaxRetries > 10 {
		v.addError("workflow.max_retries", cfg.MaxRetries, "must be between 0 and 10")
	}
}

func (v *Validator) validateAgents(cfg *AgentsConfig) {
	if !core.IsValidAgent(cfg.Default) {
		v.addError("agents.default", cfg.Default, "unknown agent")
	}

	// Validate that default agent is enabled
	defaultEnabled := map[string]bool{
		core.AgentClaude:   cfg.Claude.Enabled,
		core.AgentGemini:   cfg.Gemini.Enabled,
		core.AgentCodex:    cfg.Codex.Enabled,
		core.AgentCopilot:  cfg.Copilot.Enabled,
		core.AgentOpenCode: cfg.OpenCode.Enabled,
	}
	if !defaultEnabled[cfg.Default] {
		v.addError("agents.default", cfg.Default, "default agent must be enabled")
	}

	v.validateAgent("agents.claude", &cfg.Claude)
	v.validateAgent("agents.gemini", &cfg.Gemini)
	v.validateAgent("agents.codex", &cfg.Codex)
	v.validateAgent("agents.copilot", &cfg.Copilot)
	v.validateAgent("agents.opencode", &cfg.OpenCode)
}

func (v *Validator) validateAgent(prefix string, cfg *AgentConfig) {
	if !cfg.Enabled {
		return
	}

	if cfg.Path == "" {
		v.addError(prefix+".path", cfg.Path, "path required when enabled")
	}

	v.validatePhaseModels(prefix+".phase_models", cfg.PhaseModels)
	v.validateReasoningEffortDefault(prefix+".reasoning_effort", cfg.ReasoningEffort)
	v.validateReasoningEffortPhases(prefix+".reasoning_effort_phases", cfg.ReasoningEffortPhases)
}

func (v *Validator) validatePhaseModels(prefix string, phaseModels map[string]string) {
	if len(phaseModels) == 0 {
		return
	}

	for key, model := range phaseModels {
		if !core.IsValidPhaseModelKey(key) {
			v.addError(prefix, key, "unknown phase or task (valid: refine, analyze, moderate, synthesize, plan, execute)")
			continue
		}
		if strings.TrimSpace(model) == "" {
			v.addError(prefix+"."+key, model, "model cannot be empty")
		}
	}
}

func (v *Validator) validateReasoningEffortDefault(prefix, effort string) {
	if effort == "" {
		return
	}

	if !core.IsValidReasoningEffort(effort) {
		v.addError(prefix, effort, "invalid reasoning effort (valid: minimal, low, medium, high, xhigh)")
	}
}

func (v *Validator) validateReasoningEffortPhases(prefix string, phases map[string]string) {
	if len(phases) == 0 {
		return
	}

	for key, effort := range phases {
		if !core.IsValidPhaseModelKey(key) {
			v.addError(prefix, key, "unknown phase (valid: refine, analyze, moderate, synthesize, plan, execute)")
			continue
		}
		if !core.IsValidReasoningEffort(effort) {
			v.addError(prefix+"."+key, effort, "invalid reasoning effort (valid: minimal, low, medium, high, xhigh)")
		}
	}
}

func (v *Validator) validateState(cfg *StateConfig) {
	// Validate backend (empty string is valid - means use default "json")
	if cfg.Backend != "" {
		switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
		case core.StateBackendJSON, core.StateBackendSQLite:
			// Valid backends
		default:
			v.addError("state.backend", cfg.Backend, "must be 'json' or 'sqlite'")
		}
	}

	if cfg.Path == "" {
		v.addError("state.path", cfg.Path, "path required")
	}

	if _, err := time.ParseDuration(cfg.LockTTL); err != nil {
		v.addError("state.lock_ttl", cfg.LockTTL, "invalid duration format")
	}
}

func (v *Validator) validateGit(cfg *GitConfig) {
	if cfg.WorktreeDir == "" {
		v.addError("git.worktree_dir", cfg.WorktreeDir, "worktree directory required")
	}
	if strings.TrimSpace(cfg.WorktreeMode) != "" {
		switch strings.ToLower(strings.TrimSpace(cfg.WorktreeMode)) {
		case core.WorktreeModeAlways, core.WorktreeModeParallel, core.WorktreeModeDisabled:
			// ok
		default:
			v.addError("git.worktree_mode", cfg.WorktreeMode, "must be always, parallel, or disabled")
		}
	}

	// Validate merge strategy
	if cfg.MergeStrategy != "" {
		switch strings.ToLower(cfg.MergeStrategy) {
		case core.MergeStrategyMerge, core.MergeStrategySquash, core.MergeStrategyRebase:
			// ok
		default:
			v.addError("git.merge_strategy", cfg.MergeStrategy, "must be merge, squash, or rebase")
		}
	}

	// Validate dependency chain: auto_pr requires auto_push, auto_merge requires auto_pr
	if cfg.AutoPR && !cfg.AutoPush {
		v.addError("git.auto_pr", cfg.AutoPR, "auto_pr requires auto_push to be enabled")
	}
	if cfg.AutoMerge && !cfg.AutoPR {
		v.addError("git.auto_merge", cfg.AutoMerge, "auto_merge requires auto_pr to be enabled")
	}

	// CRITICAL: Prevent data loss - auto_clean=true with auto_commit=false would delete uncommitted changes
	if cfg.AutoClean && !cfg.AutoCommit {
		v.addError("git.auto_clean", cfg.AutoClean,
			"auto_clean cannot be true when auto_commit is false (uncommitted changes would be lost)")
	}
}

func (v *Validator) validateGitHub(cfg *GitHubConfig) {
	// Token validation is optional - may come from environment
	if cfg.Remote == "" {
		v.addError("github.remote", cfg.Remote, "remote name required")
	}
}

func (v *Validator) validatePhases(cfg *PhasesConfig, agents *AgentsConfig) {
	// Validate analyze phase
	v.validatePhaseTimeout("phases.analyze.timeout", cfg.Analyze.Timeout)
	v.validateRefiner(&cfg.Analyze.Refiner, agents)
	v.validateModerator(&cfg.Analyze.Moderator, agents)
	v.validateSingleAgent(&cfg.Analyze.SingleAgent, &cfg.Analyze.Moderator, agents)
	v.validateSynthesizer("phases.analyze.synthesizer", cfg.Analyze.Synthesizer.Agent, agents)

	// Validate plan phase
	v.validatePhaseTimeout("phases.plan.timeout", cfg.Plan.Timeout)
	if cfg.Plan.Synthesizer.Enabled {
		v.validateSynthesizer("phases.plan.synthesizer", cfg.Plan.Synthesizer.Agent, agents)
	}

	// Validate execute phase
	v.validatePhaseTimeout("phases.execute.timeout", cfg.Execute.Timeout)

	// Fail-fast: validate phase participation consistency
	v.validatePhaseParticipation(cfg, agents)
}

// validatePhaseParticipation ensures agents are properly configured for their assigned roles.
// This is a fail-fast check to avoid wasting tokens on invalid configurations.
func (v *Validator) validatePhaseParticipation(cfg *PhasesConfig, agents *AgentsConfig) {
	// Build agent config map for easy lookup
	agentConfigs := map[string]*AgentConfig{
		core.AgentClaude:   &agents.Claude,
		core.AgentGemini:   &agents.Gemini,
		core.AgentCodex:    &agents.Codex,
		core.AgentCopilot:  &agents.Copilot,
		core.AgentOpenCode: &agents.OpenCode,
	}

	// 1. Validate refiner agent has phases.refine: true
	if cfg.Analyze.Refiner.Enabled && cfg.Analyze.Refiner.Agent != "" {
		agent := cfg.Analyze.Refiner.Agent
		if ac, ok := agentConfigs[agent]; ok && ac.Enabled {
			if !ac.IsEnabledForPhase(string(core.PhaseRefine)) {
				v.addError("agents."+agent+".phases.refine", false,
					"agent is assigned as refiner but phases.refine is false")
			}
		}
	}

	// 2. Validate moderator agent has phases.moderate: true
	if cfg.Analyze.Moderator.Enabled && cfg.Analyze.Moderator.Agent != "" {
		agent := cfg.Analyze.Moderator.Agent
		if ac, ok := agentConfigs[agent]; ok && ac.Enabled {
			if !ac.IsEnabledForPhase(core.TaskModerate) {
				v.addError("agents."+agent+".phases.moderate", false,
					"agent is assigned as moderator but phases.moderate is false")
			}
		}
	}

	// 3. Validate synthesizer agent has phases.synthesize: true
	if cfg.Analyze.Synthesizer.Agent != "" {
		agent := cfg.Analyze.Synthesizer.Agent
		if ac, ok := agentConfigs[agent]; ok && ac.Enabled {
			if !ac.IsEnabledForPhase(core.TaskSynthesize) {
				v.addError("agents."+agent+".phases.synthesize", false,
					"agent is assigned as synthesizer but phases.synthesize is false")
			}
		}
	}

	// 4. Count agents enabled for each phase
	var analyzeCount, planCount, executeCount int
	var analyzeAgents, planAgents, executeAgents []string

	for name, ac := range agentConfigs {
		if !ac.Enabled {
			continue
		}
		if ac.IsEnabledForPhase(string(core.PhaseAnalyze)) {
			analyzeCount++
			analyzeAgents = append(analyzeAgents, name)
		}
		if ac.IsEnabledForPhase(string(core.PhasePlan)) {
			planCount++
			planAgents = append(planAgents, name)
		}
		if ac.IsEnabledForPhase(string(core.PhaseExecute)) {
			executeCount++
			executeAgents = append(executeAgents, name)
		}
	}

	// 5. Validate minimum agents for multi-agent analysis (only when moderator enabled AND single_agent disabled)
	// Single-agent workflows are valid when single_agent.enabled=true or moderator is disabled
	if cfg.Analyze.Moderator.Enabled && !cfg.Analyze.SingleAgent.Enabled && analyzeCount < 2 {
		v.addError("agents.*.phases.analyze", analyzeAgents,
			"at least 2 agents must have phases.analyze: true for multi-agent consensus (moderator enabled)")
	}

	// 6. Validate at least 1 agent for plan phase
	if planCount < 1 {
		v.addError("agents.*.phases.plan", planAgents,
			"at least 1 agent must have phases.plan: true")
	}

	// 7. Validate at least 1 agent for execute phase
	if executeCount < 1 {
		v.addError("agents.*.phases.execute", executeAgents,
			"at least 1 agent must have phases.execute: true")
	}
}

func (v *Validator) validatePhaseTimeout(field, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, err := time.ParseDuration(value); err != nil {
		v.addError(field, value, "invalid duration format")
	}
}

func (v *Validator) validateRefiner(cfg *RefinerConfig, agents *AgentsConfig) {
	if !cfg.Enabled {
		return
	}

	if !core.IsValidAgent(cfg.Agent) {
		v.addError("phases.analyze.refiner.agent", cfg.Agent, "unknown agent")
		return
	}

	// Validate that the specified agent is enabled
	agentEnabled := map[string]bool{
		core.AgentClaude:   agents.Claude.Enabled,
		core.AgentGemini:   agents.Gemini.Enabled,
		core.AgentCodex:    agents.Codex.Enabled,
		core.AgentCopilot:  agents.Copilot.Enabled,
		core.AgentOpenCode: agents.OpenCode.Enabled,
	}
	if !agentEnabled[cfg.Agent] {
		v.addError("phases.analyze.refiner.agent", cfg.Agent, "specified agent must be enabled")
	}
}

func (v *Validator) validateModerator(cfg *ModeratorConfig, agents *AgentsConfig) {
	if !cfg.Enabled {
		return
	}

	if !core.IsValidAgent(cfg.Agent) {
		v.addError("phases.analyze.moderator.agent", cfg.Agent, "unknown agent")
		return
	}

	// Validate that the specified agent is enabled
	agentEnabled := map[string]bool{
		core.AgentClaude:   agents.Claude.Enabled,
		core.AgentGemini:   agents.Gemini.Enabled,
		core.AgentCodex:    agents.Codex.Enabled,
		core.AgentCopilot:  agents.Copilot.Enabled,
		core.AgentOpenCode: agents.OpenCode.Enabled,
	}
	if !agentEnabled[cfg.Agent] {
		v.addError("phases.analyze.moderator.agent", cfg.Agent, "specified agent must be enabled")
	}

	if cfg.Threshold < 0 || cfg.Threshold > 1 {
		v.addError("phases.analyze.moderator.threshold", cfg.Threshold, "must be between 0 and 1")
	}
	if cfg.WarningThreshold < 0 || cfg.WarningThreshold > 1 {
		v.addError("phases.analyze.moderator.warning_threshold", cfg.WarningThreshold, "must be between 0 and 1")
	}
	if cfg.StagnationThreshold < 0 || cfg.StagnationThreshold > 1 {
		v.addError("phases.analyze.moderator.stagnation_threshold", cfg.StagnationThreshold, "must be between 0 and 1")
	}
	if cfg.MinRounds < 1 {
		v.addError("phases.analyze.moderator.min_rounds", cfg.MinRounds, "must be at least 1")
	}
	if cfg.MaxRounds < cfg.MinRounds {
		v.addError("phases.analyze.moderator.max_rounds", cfg.MaxRounds, "must be >= min_rounds")
	}
}

func (v *Validator) validateSingleAgent(cfg *SingleAgentConfig, moderator *ModeratorConfig, agents *AgentsConfig) {
	// Mutual exclusivity check
	if cfg.Enabled && moderator.Enabled {
		v.addError("phases.analyze.single_agent.enabled", cfg.Enabled,
			"cannot be true when moderator.enabled is also true; single-agent mode bypasses consensus")
	}

	if !cfg.Enabled {
		return
	}

	if !core.IsValidAgent(cfg.Agent) {
		v.addError("phases.analyze.single_agent.agent", cfg.Agent, "unknown agent")
		return
	}

	// Validate that the specified agent is enabled
	agentEnabled := map[string]bool{
		core.AgentClaude:   agents.Claude.Enabled,
		core.AgentGemini:   agents.Gemini.Enabled,
		core.AgentCodex:    agents.Codex.Enabled,
		core.AgentCopilot:  agents.Copilot.Enabled,
		core.AgentOpenCode: agents.OpenCode.Enabled,
	}
	if !agentEnabled[cfg.Agent] {
		v.addError("phases.analyze.single_agent.agent", cfg.Agent, "specified agent must be enabled")
	}
}

func (v *Validator) validateSynthesizer(prefix, agent string, agents *AgentsConfig) {
	if agent == "" {
		// Synthesizer agent is optional - will use default agent
		return
	}

	if !core.IsValidAgent(agent) {
		v.addError(prefix+".agent", agent, "unknown agent")
		return
	}

	// Validate that the specified agent is enabled
	agentEnabled := map[string]bool{
		core.AgentClaude:   agents.Claude.Enabled,
		core.AgentGemini:   agents.Gemini.Enabled,
		core.AgentCodex:    agents.Codex.Enabled,
		core.AgentCopilot:  agents.Copilot.Enabled,
		core.AgentOpenCode: agents.OpenCode.Enabled,
	}
	if !agentEnabled[agent] {
		v.addError(prefix+".agent", agent, "specified agent must be enabled")
	}
}

func isValidPath(path string) bool {
	dir := filepath.Dir(path)
	_, err := os.Stat(dir)
	return err == nil || os.IsNotExist(err)
}

// ValidateConfig is a convenience function that creates a validator and validates config.
func ValidateConfig(cfg *Config) error {
	v := NewValidator()
	return v.Validate(cfg)
}
