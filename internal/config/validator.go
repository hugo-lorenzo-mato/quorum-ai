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

// Validation message constants for duplicated strings (S1192).
const msgInvalidReasoningEffort = "invalid reasoning effort (valid: none, minimal, low, medium, high, xhigh, max)"

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
	v.validateIssues(&cfg.Issues)

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
		cfg.Timeout = "16h"
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

	// Strict phases: at least one phase must be explicitly enabled for any enabled agent.
	// Missing/empty phases map means "enabled for no phases" and would make the agent unusable.
	if len(cfg.Phases) == 0 {
		v.addError(prefix+".phases", cfg.Phases, "at least 1 phase must be set to true (strict allowlist)")
	} else {
		enabledCount := 0
		for _, enabled := range cfg.Phases {
			if enabled {
				enabledCount++
			}
		}
		if enabledCount == 0 {
			v.addError(prefix+".phases", cfg.Phases, "at least 1 phase must be set to true (strict allowlist)")
		}
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
		v.addError(prefix, effort, msgInvalidReasoningEffort)
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
			v.addError(prefix+"."+key, effort, msgInvalidReasoningEffort)
		}
	}
}

func (v *Validator) validateState(cfg *StateConfig) {
	if cfg.Path == "" {
		v.addError("state.path", cfg.Path, "path required")
	}

	if _, err := time.ParseDuration(cfg.LockTTL); err != nil {
		v.addError("state.lock_ttl", cfg.LockTTL, "invalid duration format")
	}
}

func (v *Validator) validateGit(cfg *GitConfig) {
	// Worktree validation
	if cfg.Worktree.Dir == "" {
		v.addError("git.worktree.dir", cfg.Worktree.Dir, "worktree directory required")
	}
	if strings.TrimSpace(cfg.Worktree.Mode) != "" {
		switch strings.ToLower(strings.TrimSpace(cfg.Worktree.Mode)) {
		case core.WorktreeModeAlways, core.WorktreeModeParallel, core.WorktreeModeDisabled:
			// ok
		default:
			v.addError("git.worktree.mode", cfg.Worktree.Mode, "must be always, parallel, or disabled")
		}
	}

	// CRITICAL: Prevent data loss - auto_clean=true with auto_commit=false would delete uncommitted changes
	if cfg.Worktree.AutoClean && !cfg.Task.AutoCommit {
		v.addError("git.worktree.auto_clean", cfg.Worktree.AutoClean,
			"auto_clean cannot be true when task.auto_commit is false (uncommitted changes would be lost)")
	}

	// Finalization validation
	if cfg.Finalization.MergeStrategy != "" {
		switch strings.ToLower(cfg.Finalization.MergeStrategy) {
		case core.MergeStrategyMerge, core.MergeStrategySquash, core.MergeStrategyRebase:
			// ok
		default:
			v.addError("git.finalization.merge_strategy", cfg.Finalization.MergeStrategy, "must be merge, squash, or rebase")
		}
	}

	// Validate dependency chain: auto_pr requires auto_push, auto_merge requires auto_pr
	if cfg.Finalization.AutoPR && !cfg.Finalization.AutoPush {
		v.addError("git.finalization.auto_pr", cfg.Finalization.AutoPR, "auto_pr requires auto_push to be enabled")
	}
	if cfg.Finalization.AutoMerge && !cfg.Finalization.AutoPR {
		v.addError("git.finalization.auto_merge", cfg.Finalization.AutoMerge, "auto_merge requires auto_pr to be enabled")
	}
}

func (v *Validator) validateGitHub(cfg *GitHubConfig) {
	// Token validation is optional - may come from environment
	if cfg.Remote == "" {
		v.addError("github.remote", cfg.Remote, "remote name required")
	}
}

func (v *Validator) validateIssues(cfg *IssuesConfig) {
	if !cfg.Enabled {
		return
	}

	if cfg.Provider != "" && cfg.Provider != "github" && cfg.Provider != "gitlab" {
		v.addError("issues.provider", cfg.Provider, "must be one of: github, gitlab")
	}

	// Validate mode
	if cfg.Mode != "" {
		validModes := map[string]bool{
			core.IssueModeDirect: true, core.IssueModeAgent: true,
		}
		if !validModes[cfg.Mode] {
			v.addError("issues.mode", cfg.Mode, "must be one of: direct, agent")
		}
	}

	// Validate repository format (owner/repo)
	if cfg.Repository != "" {
		parts := strings.Split(cfg.Repository, "/")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			v.addError("issues.repository", cfg.Repository, "must be in 'owner/repo' format")
		}
	}

	// Validate timeout duration
	if cfg.Timeout != "" {
		if _, err := time.ParseDuration(cfg.Timeout); err != nil {
			v.addError("issues.timeout", cfg.Timeout, "invalid duration format")
		}
	}

	v.validateIssueTemplate(&cfg.Template)

	if cfg.Provider == "gitlab" && cfg.GitLab.ProjectID == "" {
		v.addError("issues.gitlab.project_id", cfg.GitLab.ProjectID,
			"required when provider is 'gitlab'")
	}

	// Validate generator
	v.validateIssueGenerator(&cfg.Generator)
}

func (v *Validator) validateIssueTemplate(tmpl *IssueTemplateConfig) {
	validTones := map[string]bool{
		"professional": true, "casual": true, "technical": true, "concise": true, "": true,
	}
	if !validTones[tmpl.Tone] {
		v.addError("issues.template.tone", tmpl.Tone, "must be one of: professional, casual, technical, concise")
	}

	validLanguages := map[string]bool{
		"english": true, "spanish": true, "french": true, "german": true,
		"portuguese": true, "chinese": true, "japanese": true, "": true,
	}
	language := normalizeIssueLanguage(tmpl.Language)
	if !validLanguages[language] {
		v.addError("issues.template.language", tmpl.Language,
			"must be one of: english, spanish, french, german, portuguese, chinese, japanese")
	}
}

func (v *Validator) validateIssueGenerator(cfg *IssueGeneratorConfig) {
	if !cfg.Enabled {
		return
	}

	if cfg.Agent != "" && !core.IsValidAgent(cfg.Agent) {
		v.addError("issues.generator.agent", cfg.Agent, "unknown agent")
	}

	if cfg.ReasoningEffort != "" && !core.IsValidReasoningEffort(cfg.ReasoningEffort) {
		v.addError("issues.generator.reasoning_effort", cfg.ReasoningEffort,
			msgInvalidReasoningEffort)
	}

	if cfg.MaxBodyLength < 0 {
		v.addError("issues.generator.max_body_length", cfg.MaxBodyLength, "must be non-negative")
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

	// 5b. Validate min_successful_agents does not exceed available analyze agents
	if cfg.Analyze.Moderator.Enabled && !cfg.Analyze.SingleAgent.Enabled {
		if cfg.Analyze.Moderator.MinSuccessfulAgents > analyzeCount {
			v.addError("phases.analyze.moderator.min_successful_agents", cfg.Analyze.Moderator.MinSuccessfulAgents,
				fmt.Sprintf("must be <= number of agents with phases.analyze: true (%d)", analyzeCount))
		}
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
	if cfg.MinSuccessfulAgents < 1 {
		v.addError("phases.analyze.moderator.min_successful_agents", cfg.MinSuccessfulAgents, "must be at least 1")
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
