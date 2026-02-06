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
	// Model configuration (centralized source of truth)
	AgentModels         map[string][]string `json:"agent_models"`
	AgentDefaultModels  map[string]string   `json:"agent_default_models"`
	AgentsWithReasoning []string            `json:"agents_with_reasoning"`
	// Issue configuration enums
	IssueProviders    []string `json:"issue_providers"`
	TemplateLanguages []string `json:"template_languages"`
	TemplateTones     []string `json:"template_tones"`
}

// handleGetEnums returns all enum values for UI dropdowns.
// The Agents list includes all supported agents (enabled/disabled).
func (s *Server) handleGetEnums(w http.ResponseWriter, _ *http.Request) {
	enums := EnumsResponse{
		LogLevels:           core.LogLevels,
		LogFormats:          core.LogFormats,
		TraceModes:          core.TraceModes,
		StateBackends:       core.StateBackends,
		WorktreeModes:       core.WorktreeModes,
		MergeStrategies:     core.MergeStrategies,
		ReasoningEfforts:    core.AllReasoningEfforts,
		Agents:              core.Agents,
		Phases:              core.Phases,
		PhaseModelKeys:      core.PhaseModelKeys,
		AgentModels:         core.AgentModels,
		AgentDefaultModels:  core.AgentDefaultModels,
		AgentsWithReasoning: core.AgentsWithReasoning,
		IssueProviders:      core.IssueProviders,
		TemplateLanguages:   core.IssueLanguages,
		TemplateTones:       core.IssueTones,
	}

	respondJSON(w, http.StatusOK, enums)
}
