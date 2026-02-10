package workflow

import (
	"strings"
	"testing"
)

func TestIsValidAnalysisOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		// --- Invalid cases ---
		{
			name:    "empty string",
			content: "",
			want:    false,
		},
		{
			name:    "whitespace only",
			content: "   \n\n  \t  ",
			want:    false,
		},
		{
			name: "concatenated Codex narration (original bug, single line >1KB)",
			content: "Voy a auditar el codebase completo (backend Go, frontend React, config YAML, tests y adapters) " +
				"buscando todo lo relacionado con creación/publicación de GitHub issues y sub-issues, y luego " +
				"escribir un documento de análisis autocontenido con evidencia file:line en la ruta indicada." +
				"Empiezo con un escaneo global de términos clave (gh issue, sub_issues, endpoints /issues, " +
				"config issues:) para ubicar todas las piezas activas y detectar código muerto o no cableado." +
				"Voy a auditar el codebase real (backend Go + frontend React + config + adapters + tests) para " +
				"todo lo relacionado con creación de GitHub Issues y sub-issues, verificando cada hallazgo con " +
				"file:line antes de redactar el análisis final." +
				"Revisando el router del backend y middleware (especialmente el timeout global y el montaje " +
				"de endpoints de issues/SSE) para fijar evidencia con líneas exactas." +
				"Ahora fijo evidencia exacta de los handlers de issues (/issues, /issues/preview, /issues/files, " +
				"/issues/single) porque ahí están los flujos reales (directo vs agente), defaults peligrosos, " +
				"y el comportamiento de linking.",
			want: false,
		},
		{
			name: "concatenated Codex narration with newlines (after streaming fix, still no headers)",
			content: "Voy a auditar el codebase completo buscando todo lo relacionado con issues.\n" +
				"Empiezo con un escaneo global de términos clave.\n" +
				"Voy a auditar el codebase real verificando cada hallazgo.\n" +
				"Revisando el router del backend y middleware.\n" +
				"Ahora fijo evidencia exacta de los handlers de issues.\n" +
				"Voy a inspeccionar internal/service/issues/generator.go en puntos críticos.\n" +
				strings.Repeat("Revisando más código de la aplicación para completar auditoría.\n", 10),
			want: false,
		},
		{
			name:    "long plain text without markdown structure",
			content: strings.Repeat("This is some plain text without any markdown structure whatsoever. ", 30),
			want:    false,
		},

		// --- Skeleton cases ---
		{
			name: "skeleton with headings only",
			content: "# Analysis\n## Overview\n## Architecture\n## Findings\n" +
				"## Risks\n## Recommendations\n## Security\n## Performance\n" +
				"## Testing\n## Conclusion\n",
			want: false,
		},
		{
			name: "skeleton with HTML comment placeholders",
			content: "# Analysis\n## Overview\n<!-- TODO -->\n## Architecture\n" +
				"<!-- TODO -->\n## Findings\n<!-- to be written -->\n## Risks\n",
			want: false,
		},
		{
			name: "skeleton with 2 substantive lines",
			content: "# Analysis\n## Overview\nBrief intro.\n## Architecture\n" +
				"Some notes here.\n## Findings\n## Risks\n## Recommendations\n",
			want: false,
		},
		{
			name: "short analysis with few headers",
			content: "# Analysis\n\nThis is a complete analysis of the system.\n\n" +
				"## Findings\n\nThe code has several important patterns.\n" +
				"First, the architecture uses a layered approach.\n" +
				"Second, the error handling is consistent.\n" +
				"Third, the test coverage is adequate.\n",
			want: true,
		},
		{
			name: "headers with substantial content",
			content: "# Analysis\n\nDetailed intro paragraph with real info.\n\n" +
				"## Architecture\n\nThe system uses microservices pattern.\n" +
				"Each service communicates via gRPC calls.\n\n" +
				"## Findings\n\nThe main entry point is at cmd/main.go:42.\n" +
				"The config is loaded from internal/config/loader.go:15.\n\n" +
				"## Risks\n\nConcurrency issues in the event bus.\n" +
				"Race condition at internal/events/bus.go:89.\n\n" +
				"## Recommendations\n\nAdd mutex to protect shared state.\n" +
				"Increase test coverage for edge cases.\n",
			want: true,
		},

		// --- Valid cases ---
		{
			name:    "short plain text under 1KB threshold",
			content: "Brief analysis: the function works correctly.",
			want:    true,
		},
		{
			name:    "short error message under 1KB threshold",
			content: "Error: unable to read source files",
			want:    true,
		},
		{
			name: "short text just under 1KB",
			content: strings.Repeat("x", 1023),
			want: true,
		},
		{
			name: "minimal valid markdown analysis at 1KB boundary",
			content: "# Analysis\n\n" + strings.Repeat("Content here. ", 70),
			want: true,
		},
		{
			name: "full structured markdown analysis",
			content: "# Analysis Report\n\n" +
				"## Overview\n\n" +
				"This analysis examines the GitHub issues integration.\n\n" +
				"## Findings\n\n" +
				"### Backend\n\n" +
				"- The API handler at `internal/api/issues.go:45` processes issue creation.\n" +
				"- Rate limiting is applied via middleware.\n\n" +
				"### Frontend\n\n" +
				"- The React component at `src/components/Issues.jsx` renders the form.\n\n" +
				"## Recommendations\n\n" +
				"1. Add input validation for issue titles.\n" +
				"2. Implement retry logic for GitHub API failures.\n" +
				strings.Repeat("Additional detail about the implementation. ", 20),
			want: true,
		},
		{
			name: "analysis with code blocks",
			content: "# Code Review\n\n" +
				"The main function:\n\n" +
				"```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n" +
				strings.Repeat("More analysis content. ", 40),
			want: true,
		},
		{
			name: "header at start of content (no preceding newline needed)",
			content: "# Summary\n" + strings.Repeat("Content paragraph. ", 60),
			want: true,
		},
		{
			name: "header after newline",
			content: "Preamble text\n# Main Analysis\n" + strings.Repeat("Content paragraph. ", 60),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidAnalysisOutput(tt.content)
			if got != tt.want {
				trimmed := tt.content
				if len(trimmed) > 100 {
					trimmed = trimmed[:100] + "..."
				}
				t.Errorf("isValidAnalysisOutput() = %v, want %v\n  content (%d bytes): %q",
					got, tt.want, len(tt.content), trimmed)
			}
		})
	}
}

func TestIsValidModeratorOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "empty string",
			content: "",
			want:    false,
		},
		{
			name:    "conversational stdout",
			content: "I'll now evaluate the analyses provided by the agents. Let me read each file carefully and compare their findings.",
			want:    false,
		},
		{
			name: "YAML frontmatter",
			content: "---\nconsensus_score: 78\nhigh_impact_divergences: 1\n---\n\n## Score Rationale\nGood agreement.",
			want: true,
		},
		{
			name:    "backup anchor",
			content: "Some text\n>> FINAL SCORE: 78 <<",
			want:    true,
		},
		{
			name:    "consensus_score key in text",
			content: "The consensus_score is 85 based on the analysis.",
			want:    true,
		},
		{
			name: "markdown headers with evaluation keywords",
			content: "## Score Rationale\nGood overall agreement.\n\n## Agreements\n- Point one\n\n## Divergences\n- Point two",
			want: true,
		},
		{
			name:    "bold formatting with agreement keyword",
			content: "**Agreement Areas**: The agents show strong consensus on architecture.",
			want:    true,
		},
		{
			name:    "case insensitive keywords",
			content: "## Analysis\n**DIVERGENCE** in approach detected between agents.",
			want:    true,
		},
		{
			name: "headers without evaluation keywords",
			content: "## Introduction\nSome text here.\n## Summary\nMore text.",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidModeratorOutput(tt.content)
			if got != tt.want {
				trimmed := tt.content
				if len(trimmed) > 100 {
					trimmed = trimmed[:100] + "..."
				}
				t.Errorf("isValidModeratorOutput() = %v, want %v\n  content (%d bytes): %q",
					got, tt.want, len(tt.content), trimmed)
			}
		})
	}
}
