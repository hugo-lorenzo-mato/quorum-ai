package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestTokenCounting_Accuracy tests token counting accuracy across models.
func TestTokenCounting_Accuracy(t *testing.T) {
	t.Parallel()

	// Known token examples for different types of content
	testCases := []struct {
		name        string
		content     string
		expectedMin int // Minimum expected tokens
		expectedMax int // Maximum expected tokens
		description string
	}{
		{
			name:        "simple_english",
			content:     "The quick brown fox jumps over the lazy dog.",
			expectedMin: 8,
			expectedMax: 12,
			description: "Basic English sentence",
		},
		{
			name:        "programming_code",
			content:     `func main() { fmt.Println("Hello, World!") }`,
			expectedMin: 10,
			expectedMax: 15,
			description: "Go code snippet",
		},
		{
			name:        "markdown_content",
			content:     "# Header\n\n**Bold** and *italic* text with [links](http://example.com).",
			expectedMin: 12,
			expectedMax: 18,
			description: "Markdown with formatting",
		},
		{
			name:        "unicode_characters",
			content:     "Hello 世界! 🌍 Émojis and Ñoñó characters",
			expectedMin: 8,
			expectedMax: 14,
			description: "Unicode and emoji content",
		},
		{
			name:        "empty_string",
			content:     "",
			expectedMin: 0,
			expectedMax: 0,
			description: "Empty content",
		},
	}

	// Test with base adapter (all CLI adapters inherit from this)
	base := NewBaseAdapter(AgentConfig{}, nil)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			estimate := base.TokenEstimate(tc.content)

			// Verify estimate is within reasonable bounds
			if estimate < tc.expectedMin || estimate > tc.expectedMax {
				t.Errorf("TokenEstimate(%q) = %d, want between %d and %d (%s)",
					truncateForDisplay(tc.content), estimate, tc.expectedMin, tc.expectedMax, tc.description)
			}

			// Verify estimate is non-negative
			if estimate < 0 {
				t.Errorf("TokenEstimate returned negative value: %d", estimate)
			}

			// Verify estimate scales reasonably with content length
			if len(tc.content) > 0 && estimate == 0 {
				t.Errorf("TokenEstimate should not be zero for non-empty content")
			}
		})
	}
}

// TestTokenCounting_EdgeCases tests edge cases and corner scenarios.
func TestTokenCounting_EdgeCases(t *testing.T) {
	t.Parallel()

	base := NewBaseAdapter(AgentConfig{}, nil)

	testCases := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "very_long_word",
			content:     strings.Repeat("a", 1000),
			description: "Single very long word",
		},
		{
			name:        "many_short_words",
			content:     strings.Repeat("a ", 500),
			description: "Many single-character words",
		},
		{
			name:        "nested_quotes",
			content:     `"He said 'she said \"it works\"' and left"`,
			description: "Nested quotation marks",
		},
		{
			name:        "whitespace_only",
			content:     "   \n\t  \n  ",
			description: "Whitespace and newlines",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			estimate := base.TokenEstimate(tc.content)

			// Basic sanity checks
			if estimate < 0 {
				t.Errorf("TokenEstimate returned negative value: %d", estimate)
			}

			if len(tc.content) > 0 && estimate == 0 {
				t.Errorf("TokenEstimate should not be zero for non-empty content")
			}

			t.Logf("%s: %d tokens for %d chars (%s)", tc.name, estimate, len(tc.content), tc.description)
		})
	}
}

// TestTokenCounting_PerformanceWithLargeContent tests performance.
func TestTokenCounting_PerformanceWithLargeContent(t *testing.T) {
	t.Parallel()

	base := NewBaseAdapter(AgentConfig{}, nil)

	// Create increasingly large content
	sizes := []int{1000, 10000, 100000}

	for _, size := range sizes {
		size := size
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {

			content := strings.Repeat("This is test content for performance testing. ", size/45)
			if len(content) < size {
				content += strings.Repeat("x", size-len(content))
			}

			start := time.Now()
			estimate := base.TokenEstimate(content)
			duration := time.Since(start)

			// Performance should be reasonable (< 100ms for large content)
			maxDuration := 100 * time.Millisecond
			if duration > maxDuration {
				t.Errorf("TokenEstimate took too long for %d chars: %v (max: %v)", size, duration, maxDuration)
			}

			// Estimate should be reasonable
			if estimate <= 0 {
				t.Errorf("TokenEstimate should be positive for non-empty content, got %d", estimate)
			}

			t.Logf("Size: %d chars, Estimate: %d tokens, Duration: %v", size, estimate, duration)
		})
	}
}

// Helper function to truncate strings for display
func truncateForDisplay(s string) string {
	if len(s) <= 50 {
		return s
	}
	return s[:50] + "..."
}
