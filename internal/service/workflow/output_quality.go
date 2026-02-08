package workflow

import "strings"

// isValidAnalysisOutput checks whether content looks like a real structured
// analysis rather than concatenated intermediate agent narration (e.g.,
// Codex's "Voy a auditar..." planning text that leaks into the stdout buffer
// when the process is interrupted before producing the actual analysis).
//
// Rules:
//   - Empty content is always invalid.
//   - Short content (< 1 KB) is accepted as-is — it may be a brief but
//     legitimate response or an error message.
//   - Content >= 1 KB must contain at least one newline (rejects single-line
//     concatenated blobs) AND at least one markdown header (# …).  All
//     analysis prompts request structured markdown output with headers, so
//     the absence of any header is a reliable signal of garbage output.
func isValidAnalysisOutput(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) == 0 {
		return false
	}

	// Short outputs are accepted without further checks.
	if len(trimmed) < 1024 {
		return true
	}

	// Require at least one newline — a single line of 1 KB+ is concatenated garbage.
	if strings.Count(trimmed, "\n") == 0 {
		return false
	}

	// Require at least one markdown header.
	if strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "\n#") {
		return true
	}

	return false
}
