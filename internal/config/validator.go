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
	v.validateState(&cfg.State)
	v.validateGit(&cfg.Git)
	v.validateGitHub(&cfg.GitHub)
	v.validateConsensus(&cfg.Consensus)
	v.validateCosts(&cfg.Costs)

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
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[cfg.Level] {
		v.addError("log.level", cfg.Level, "must be one of: debug, info, warn, error")
	}

	validFormats := map[string]bool{
		"auto": true, "text": true, "json": true,
	}
	if !validFormats[cfg.Format] {
		v.addError("log.format", cfg.Format, "must be one of: auto, text, json")
	}

	if cfg.File != "" && !isValidPath(cfg.File) {
		v.addError("log.file", cfg.File, "invalid file path")
	}
}

func (v *Validator) validateTrace(cfg *TraceConfig) {
	validModes := map[string]bool{
		"off": true, "summary": true, "full": true,
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
			"analyze": true, "plan": true, "execute": true, "consensus": true,
		}
		for _, phase := range cfg.IncludePhases {
			if !validPhases[phase] {
				v.addError("trace.include_phases", phase, "unknown phase")
			}
		}
	}
}

func (v *Validator) validateWorkflow(cfg *WorkflowConfig) {
	if _, err := time.ParseDuration(cfg.Timeout); err != nil {
		v.addError("workflow.timeout", cfg.Timeout, "invalid duration format")
	}

	if cfg.MaxRetries < 0 || cfg.MaxRetries > 10 {
		v.addError("workflow.max_retries", cfg.MaxRetries, "must be between 0 and 10")
	}
}

func (v *Validator) validateAgents(cfg *AgentsConfig) {
	validDefaults := map[string]bool{
		"claude": true, "gemini": true, "codex": true, "copilot": true, "aider": true,
	}
	if !validDefaults[cfg.Default] {
		v.addError("agents.default", cfg.Default, "unknown agent")
	}

	// Validate that default agent is enabled
	defaultEnabled := map[string]bool{
		"claude":  cfg.Claude.Enabled,
		"gemini":  cfg.Gemini.Enabled,
		"codex":   cfg.Codex.Enabled,
		"copilot": cfg.Copilot.Enabled,
		"aider":   cfg.Aider.Enabled,
	}
	if !defaultEnabled[cfg.Default] {
		v.addError("agents.default", cfg.Default, "default agent must be enabled")
	}

	v.validateAgent("agents.claude", &cfg.Claude)
	v.validateAgent("agents.gemini", &cfg.Gemini)
	v.validateAgent("agents.codex", &cfg.Codex)
	v.validateAgent("agents.copilot", &cfg.Copilot)
	v.validateAgent("agents.aider", &cfg.Aider)
}

func (v *Validator) validateAgent(prefix string, cfg *AgentConfig) {
	if !cfg.Enabled {
		return
	}

	if cfg.Path == "" {
		v.addError(prefix+".path", cfg.Path, "path required when enabled")
	}

	v.validatePhaseModels(prefix+".phase_models", cfg.PhaseModels)

	if cfg.MaxTokens < 0 || cfg.MaxTokens > 200000 {
		v.addError(prefix+".max_tokens", cfg.MaxTokens, "must be between 0 and 200000")
	}

	if cfg.Temperature < 0 || cfg.Temperature > 2 {
		v.addError(prefix+".temperature", cfg.Temperature, "must be between 0 and 2")
	}
}

func (v *Validator) validatePhaseModels(prefix string, phaseModels map[string]string) {
	if len(phaseModels) == 0 {
		return
	}

	for phase, model := range phaseModels {
		if !core.ValidPhase(core.Phase(phase)) {
			v.addError(prefix, phase, "unknown phase")
			continue
		}
		if strings.TrimSpace(model) == "" {
			v.addError(prefix+"."+phase, model, "model cannot be empty")
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
	if cfg.WorktreeDir == "" {
		v.addError("git.worktree_dir", cfg.WorktreeDir, "worktree directory required")
	}
}

func (v *Validator) validateGitHub(cfg *GitHubConfig) {
	// Token validation is optional - may come from environment
	if cfg.Remote == "" {
		v.addError("github.remote", cfg.Remote, "remote name required")
	}
}

func (v *Validator) validateConsensus(cfg *ConsensusConfig) {
	if cfg.Threshold < 0 || cfg.Threshold > 1 {
		v.addError("consensus.threshold", cfg.Threshold, "must be between 0 and 1")
	}

	totalWeight := cfg.Weights.Claims + cfg.Weights.Risks + cfg.Weights.Recommendations
	if totalWeight < 0.99 || totalWeight > 1.01 {
		v.addError("consensus.weights", totalWeight, "weights must sum to 1.0")
	}

	for name, weight := range map[string]float64{
		"claims":          cfg.Weights.Claims,
		"risks":           cfg.Weights.Risks,
		"recommendations": cfg.Weights.Recommendations,
	} {
		if weight < 0 || weight > 1 {
			v.addError("consensus.weights."+name, weight, "must be between 0 and 1")
		}
	}
}

func (v *Validator) validateCosts(cfg *CostsConfig) {
	if cfg.MaxPerWorkflow < 0 {
		v.addError("costs.max_per_workflow", cfg.MaxPerWorkflow, "must be non-negative")
	}

	if cfg.MaxPerTask < 0 {
		v.addError("costs.max_per_task", cfg.MaxPerTask, "must be non-negative")
	}

	if cfg.AlertThreshold < 0 || cfg.AlertThreshold > 1 {
		v.addError("costs.alert_threshold", cfg.AlertThreshold, "must be between 0 and 1")
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
