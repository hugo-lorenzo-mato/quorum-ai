package api

import (
	"net/http"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
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
		LogLevels:        core.LogLevels,
		LogFormats:       core.LogFormats,
		TraceModes:       core.TraceModes,
		StateBackends:    core.StateBackends,
		WorktreeModes:    core.WorktreeModes,
		MergeStrategies:  core.MergeStrategies,
		ReasoningEfforts: core.ReasoningEfforts,
		Agents:           core.Agents,
		Phases:           core.Phases,
		PhaseModelKeys:   core.PhaseModelKeys,
	}

	respondJSON(w, http.StatusOK, enums)
}
