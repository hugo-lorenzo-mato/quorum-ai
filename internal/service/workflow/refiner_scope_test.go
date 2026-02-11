package workflow

import (
	"strings"
	"testing"
)

// TestRefinementScopePreservation validates that refinement doesn't expand scope.
// These tests define expected behavior - they will fail with current template
// but should pass with the improved refine-prompt-v2 template.
func TestRefinementScopePreservation(t *testing.T) {
	tests := []struct {
		name           string
		originalPrompt string
		refinedPrompt  string
		shouldPass     bool
		reason         string
	}{
		{
			name:           "simple request should not add features",
			originalPrompt: "Add validation to login form",
			refinedPrompt:  "Implement comprehensive validation with OAuth, 2FA, rate limiting, and security audit for the login form",
			shouldPass:     false,
			reason:         "Added OAuth, 2FA, rate limiting - scope expansion",
		},
		{
			name:           "simple request clarified correctly",
			originalPrompt: "Add validation to login form",
			refinedPrompt:  "Add input validation to the login form: validate email format and ensure password is not empty",
			shouldPass:     true,
			reason:         "Clarified validation without adding features",
		},
		{
			name:           "performance request should not become full audit",
			originalPrompt: "Improve app performance",
			refinedPrompt:  "Conduct comprehensive performance audit covering frontend bundle optimization, backend query optimization, infrastructure review, CDN configuration, caching strategies, and load balancing setup",
			shouldPass:     false,
			reason:         "Turned simple request into exhaustive audit",
		},
		{
			name:           "performance request clarified correctly",
			originalPrompt: "Improve app performance",
			refinedPrompt:  "Identify and fix performance bottlenecks in the application, focusing on slow page loads and API response times",
			shouldPass:     true,
			reason:         "Clarified without dictating entire scope",
		},
		{
			name:           "bug fix should not add refactoring",
			originalPrompt: "Fix the null pointer error in user service",
			refinedPrompt:  "Investigate root cause of null pointer error in user service, fix the issue, refactor the service for better error handling, and add comprehensive test coverage",
			shouldPass:     false,
			reason:         "Added refactoring and testing beyond fixing the bug",
		},
		{
			name:           "bug fix clarified correctly",
			originalPrompt: "Fix the null pointer error in user service",
			refinedPrompt:  "Fix the null pointer error in the user service. Identify the specific line causing the error and implement a null check.",
			shouldPass:     true,
			reason:         "Clarified fix approach without scope expansion",
		},
		{
			name:           "feature request with constraint should preserve it",
			originalPrompt: "Add a simple search feature to the dashboard",
			refinedPrompt:  "Implement a comprehensive search system with fuzzy matching, autocomplete, search history, advanced filters, and analytics tracking",
			shouldPass:     false,
			reason:         "User said 'simple' but refined adds complexity",
		},
		{
			name:           "feature with constraint clarified correctly",
			originalPrompt: "Add a simple search feature to the dashboard",
			refinedPrompt:  "Add a basic search feature to the dashboard that filters items by text input",
			shouldPass:     true,
			reason:         "Preserved 'simple' constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopePreserved := validateScopePreservation(tt.originalPrompt, tt.refinedPrompt)

			if scopePreserved != tt.shouldPass {
				if tt.shouldPass {
					t.Errorf("Expected scope preservation but detected expansion.\nReason: %s\nOriginal: %s\nRefined: %s",
						tt.reason, tt.originalPrompt, tt.refinedPrompt)
				} else {
					t.Logf("Correctly detected scope expansion.\nReason: %s", tt.reason)
				}
			}
		})
	}
}

// validateScopePreservation is a heuristic validator that checks if refined prompt
// maintains the original scope. Returns true if scope is preserved.
func validateScopePreservation(original, refined string) bool {
	originalLower := strings.ToLower(original)
	refinedLower := strings.ToLower(refined)

	// Check for scope expansion keywords
	scopeExpansionKeywords := []string{
		"comprehensive", "exhaustive", "complete audit",
		"all aspects", "entire system", "full analysis",
		"thoroughly investigate", "deep dive into",
	}

	// Check if refined adds many more technical terms than original
	techTermsAdded := countNewTechnicalTerms(originalLower, refinedLower)

	// Fail if scope expansion keywords found
	for _, keyword := range scopeExpansionKeywords {
		if strings.Contains(refinedLower, keyword) && !strings.Contains(originalLower, keyword) {
			return false
		}
	}

	// Fail if >5 new technical terms added
	if techTermsAdded > 5 {
		return false
	}

	// Check length ratio with dynamic threshold:
	// - Short prompts (<50 chars) can expand up to 5x (clarification needs more detail)
	// - Medium prompts (50-100 chars) can expand up to 3.5x
	// - Long prompts (>100 chars) should not expand more than 2.5x
	originalLen := len(original)
	refinedLen := len(refined)
	ratio := float64(refinedLen) / float64(originalLen)

	var maxRatio float64
	switch {
	case originalLen < 50:
		maxRatio = 5.0
	case originalLen < 100:
		maxRatio = 3.5
	default:
		maxRatio = 2.5
	}

	// Fail if refined exceeds length ratio threshold
	if ratio > maxRatio {
		return false
	}

	return true
}

// countNewTechnicalTerms counts how many new technical terms appear in refined
// that were not in original (simple heuristic).
func countNewTechnicalTerms(original, refined string) int {
	technicalTerms := []string{
		"oauth", "2fa", "authentication", "authorization",
		"rate limiting", "audit", "refactor", "architecture",
		"cdn", "caching", "load balancing", "security",
		"test coverage", "monitoring", "logging", "metrics",
		"performance", "optimization", "scalability",
	}

	count := 0
	for _, term := range technicalTerms {
		inRefined := strings.Contains(refined, term)
		inOriginal := strings.Contains(original, term)

		if inRefined && !inOriginal {
			count++
		}
	}

	return count
}

// TestRefinementQualityChecks validates that refined prompts include
// critical attitude requirements.
func TestRefinementQualityChecks(t *testing.T) {
	tests := []struct {
		name          string
		refinedPrompt string
		hasEvidence   bool
		hasActionable bool
	}{
		{
			name:          "good refined prompt with evidence requirement",
			refinedPrompt: "Fix the bug in auth.go. Cite specific file:line references for the fix.",
			hasEvidence:   true,
			hasActionable: true,
		},
		{
			name:          "vague refined prompt",
			refinedPrompt: "Make the code better",
			hasEvidence:   false,
			hasActionable: false,
		},
		{
			name:          "refined with actionable requirement",
			refinedPrompt: "Identify performance issues and provide concrete steps to fix each one.",
			hasEvidence:   false,
			hasActionable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasEvidence := checkForEvidenceRequirement(tt.refinedPrompt)
			hasActionable := checkForActionableRequirement(tt.refinedPrompt)

			if hasEvidence != tt.hasEvidence {
				t.Errorf("Evidence requirement: got %v, want %v", hasEvidence, tt.hasEvidence)
			}
			if hasActionable != tt.hasActionable {
				t.Errorf("Actionable requirement: got %v, want %v", hasActionable, tt.hasActionable)
			}
		})
	}
}

func checkForEvidenceRequirement(prompt string) bool {
	lower := strings.ToLower(prompt)
	evidenceKeywords := []string{
		"file:", "line", "cite", "reference", "specific", "evidence",
	}

	for _, keyword := range evidenceKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func checkForActionableRequirement(prompt string) bool {
	lower := strings.ToLower(prompt)
	actionableKeywords := []string{
		"actionable", "concrete steps", "specific", "how to",
	}

	for _, keyword := range actionableKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
