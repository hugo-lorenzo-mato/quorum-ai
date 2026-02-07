package cli

import (
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

// ConfigureRegistryFromConfig configures agents in the registry using unified config.
// Agents are configured only if their enabled flag is true in the config.
//
// NOTE: This intentionally does not attempt to "disable" previously configured agents.
// Callers that need hot-reload should rebuild a fresh Registry and swap it in.
func ConfigureRegistryFromConfig(registry *Registry, cfg *config.Config) error {
	// Configure Claude
	if cfg.Agents.Claude.Enabled {
		registry.Configure("claude", AgentConfig{
			Name:                      "claude",
			Path:                      cfg.Agents.Claude.Path,
			Model:                     cfg.Agents.Claude.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Claude.Phases,
			ReasoningEffort:           cfg.Agents.Claude.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Claude.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Claude.TokenDiscrepancyThreshold),
		})
	}

	// Configure Gemini
	if cfg.Agents.Gemini.Enabled {
		registry.Configure("gemini", AgentConfig{
			Name:                      "gemini",
			Path:                      cfg.Agents.Gemini.Path,
			Model:                     cfg.Agents.Gemini.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Gemini.Phases,
			ReasoningEffort:           cfg.Agents.Gemini.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Gemini.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Gemini.TokenDiscrepancyThreshold),
		})
	}

	// Configure Codex
	if cfg.Agents.Codex.Enabled {
		registry.Configure("codex", AgentConfig{
			Name:                      "codex",
			Path:                      cfg.Agents.Codex.Path,
			Model:                     cfg.Agents.Codex.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Codex.Phases,
			ReasoningEffort:           cfg.Agents.Codex.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Codex.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Codex.TokenDiscrepancyThreshold),
		})
	}

	// Configure Copilot
	if cfg.Agents.Copilot.Enabled {
		registry.Configure("copilot", AgentConfig{
			Name:                      "copilot",
			Path:                      cfg.Agents.Copilot.Path,
			Model:                     cfg.Agents.Copilot.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.Copilot.Phases,
			ReasoningEffort:           cfg.Agents.Copilot.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.Copilot.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.Copilot.TokenDiscrepancyThreshold),
		})
	}

	// Configure OpenCode
	if cfg.Agents.OpenCode.Enabled {
		registry.Configure("opencode", AgentConfig{
			Name:                      "opencode",
			Path:                      cfg.Agents.OpenCode.Path,
			Model:                     cfg.Agents.OpenCode.Model,
			Timeout:                   5 * time.Minute,
			Phases:                    cfg.Agents.OpenCode.Phases,
			ReasoningEffort:           cfg.Agents.OpenCode.ReasoningEffort,
			ReasoningEffortPhases:     cfg.Agents.OpenCode.ReasoningEffortPhases,
			TokenDiscrepancyThreshold: getTokenDiscrepancyThreshold(cfg.Agents.OpenCode.TokenDiscrepancyThreshold),
		})
	}

	return nil
}

func getTokenDiscrepancyThreshold(configured float64) float64 {
	if configured > 0 {
		return configured
	}
	return DefaultTokenDiscrepancyThreshold
}
