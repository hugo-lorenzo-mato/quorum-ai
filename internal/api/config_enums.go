package api

import (
	"net/http"
)

// EnumsResponse contains all enum values used in configuration.
type EnumsResponse struct {
	LogLevels        []string `json:"log_levels"`
	LogFormats       []string `json:"log_formats"`
	TraceModes       []string `json:"trace_modes"`
	StateBackends    []string `json:"state_backends"`
	WorktreeModes    []string `json:"worktree_modes"`
	MergeStrategies  []string `json:"merge_strategies"`
	ReasoningEfforts []string `json:"reasoning_efforts"`
	Agents           []string `json:"agents"`
	Phases           []string `json:"phases"`
	PhaseModelKeys   []string `json:"phase_model_keys"`
}

// handleGetEnums returns all enum values for UI dropdowns.
func (s *Server) handleGetEnums(w http.ResponseWriter, _ *http.Request) {
	enums := EnumsResponse{
		LogLevels:        []string{"debug", "info", "warn", "error"},
		LogFormats:       []string{"auto", "text", "json"},
		TraceModes:       []string{"off", "summary", "full"},
		StateBackends:    []string{"sqlite", "json"},
		WorktreeModes:    []string{"always", "parallel", "disabled"},
		MergeStrategies:  []string{"merge", "squash", "rebase"},
		ReasoningEfforts: []string{"minimal", "low", "medium", "high", "xhigh"},
		Agents:           []string{"claude", "gemini", "codex", "copilot"},
		Phases:           []string{"refine", "analyze", "plan", "execute"},
		PhaseModelKeys:   []string{"refine", "analyze", "moderate", "synthesize", "plan", "execute"},
	}

	respondJSON(w, http.StatusOK, enums)
}
