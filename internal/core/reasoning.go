package core

import "strings"

var reasoningRank = map[string]int{
	ReasoningNone:    0,
	ReasoningMinimal: 0,
	ReasoningLow:     1,
	ReasoningMedium:  2,
	ReasoningHigh:    3,
	ReasoningXHigh:   4,
}

// SupportedReasoningEffortsForModel returns the known set of reasoning effort values
// supported by a given Codex/OpenAI model when used through the Codex CLI.
//
// Notes:
//   - Some models support `minimal` (e.g. gpt-5), some support `none` (e.g. gpt-5.1),
//     and some support neither (older codex variants).
//   - The returned slice is ordered from lowest to highest effort.
func SupportedReasoningEffortsForModel(model string) []string {
	switch strings.TrimSpace(model) {
	// Latest codex
	case "gpt-5.3-codex":
		return []string{ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh}

	// Codex variants (generally do not support none/minimal)
	case "gpt-5.2-codex", "gpt-5.1-codex-max":
		return []string{ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh}
	case "gpt-5.1-codex", "gpt-5.1-codex-mini", "gpt-5-codex", "gpt-5-codex-mini":
		return []string{ReasoningLow, ReasoningMedium, ReasoningHigh}

	// Base GPT-5 family
	case "gpt-5.2":
		return []string{ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh}
	case "gpt-5.1":
		return []string{ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh}
	case "gpt-5":
		return []string{ReasoningMinimal, ReasoningLow, ReasoningMedium, ReasoningHigh}
	default:
		return nil
	}
}

// NormalizeReasoningEffortForModel picks the closest supported reasoning effort for the
// given model, to prevent Codex CLI/API errors when a config requests an unsupported effort.
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

	reqRank, ok := reasoningRank[effort]
	if !ok {
		// Unknown value; keep as-is (it will be validated elsewhere).
		return effort
	}

	// Find closest effort by rank. If requested is outside the allowed range, clamp.
	best := allowed[0]
	bestDiff := int(^uint(0) >> 1) // max int
	for _, a := range allowed {
		r, ok := reasoningRank[a]
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

	// Prefer "none" over "minimal" (and vice-versa) when both share the same rank and
	// the requested effort was one of those values.
	if reqRank == 0 {
		if effort == ReasoningNone {
			for _, a := range allowed {
				if a == ReasoningNone {
					return ReasoningNone
				}
			}
		}
		if effort == ReasoningMinimal {
			for _, a := range allowed {
				if a == ReasoningMinimal {
					return ReasoningMinimal
				}
			}
		}
	}

	return best
}
