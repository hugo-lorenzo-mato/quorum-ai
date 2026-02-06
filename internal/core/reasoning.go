package core

import "strings"

// codexEffortRank maps Codex reasoning effort levels to numeric ranks for normalization.
var codexEffortRank = map[string]int{
	"none":    0,
	"minimal": 1,
	"low":     2,
	"medium":  3,
	"high":    4,
	"xhigh":   5,
}

// SupportedReasoningEffortsForModel returns the known reasoning effort values
// supported by a given Codex/OpenAI model when used through the Codex CLI.
// Data is derived from the unified AgentModelReasoningEfforts map.
func SupportedReasoningEffortsForModel(model string) []string {
	return GetModelReasoningEfforts(AgentCodex, strings.TrimSpace(model))
}

// NormalizeReasoningEffortForModel picks the closest supported reasoning effort for the
// given Codex model, to prevent Codex CLI errors when a config requests an unsupported effort.
//
// If the model is unknown (no supported set), it returns effort unchanged.
func NormalizeReasoningEffortForModel(model, effort string) string {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort == "" {
		return ""
	}

	allowed := SupportedReasoningEffortsForModel(model)
	if len(allowed) == 0 {
		return effort
	}

	// Exact match
	for _, a := range allowed {
		if effort == a {
			return effort
		}
	}

	// Map Claude-style "max" to Codex "xhigh"
	if effort == "max" {
		for _, a := range allowed {
			if a == "xhigh" {
				return "xhigh"
			}
		}
		return allowed[len(allowed)-1] // highest available
	}

	reqRank, ok := codexEffortRank[effort]
	if !ok {
		return effort
	}

	// Find closest effort by rank
	best := allowed[0]
	bestDiff := int(^uint(0) >> 1)
	for _, a := range allowed {
		r, ok := codexEffortRank[a]
		if !ok {
			continue
		}
		diff := r - reqRank
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best = a
			bestDiff = diff
		}
	}

	return best
}

// =============================================================================
// Claude Effort Support
// =============================================================================

// claudeEffortRank maps Claude effort levels to a numeric rank for normalization.
var claudeEffortRank = map[string]int{
	"low":    0,
	"medium": 1,
	"high":   2,
	"max":    3,
}

// SupportedEffortsForClaudeModel returns the effort levels supported by a Claude model.
// Data is derived from the unified AgentModelReasoningEfforts map.
func SupportedEffortsForClaudeModel(model string) []string {
	return GetModelReasoningEfforts(AgentClaude, strings.TrimSpace(model))
}

// NormalizeClaudeEffort picks the closest supported effort level for the given Claude model.
// If the model is unknown or doesn't support effort, returns the effort unchanged.
func NormalizeClaudeEffort(model, effort string) string {
	effort = strings.TrimSpace(strings.ToLower(effort))
	if effort == "" {
		return ""
	}

	allowed := SupportedEffortsForClaudeModel(model)
	if len(allowed) == 0 {
		return effort
	}

	// Exact match
	for _, a := range allowed {
		if effort == a {
			return effort
		}
	}

	// Map Codex-style reasoning levels to Claude effort for cross-agent compatibility
	switch effort {
	case "none", "minimal":
		return "low"
	case "xhigh":
		return "max"
	}

	reqRank, ok := claudeEffortRank[effort]
	if !ok {
		return effort
	}

	// Clamp to nearest supported level
	best := allowed[0]
	bestDiff := int(^uint(0) >> 1)
	for _, a := range allowed {
		r, ok := claudeEffortRank[a]
		if !ok {
			continue
		}
		diff := r - reqRank
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best = a
			bestDiff = diff
		}
	}
	return best
}
