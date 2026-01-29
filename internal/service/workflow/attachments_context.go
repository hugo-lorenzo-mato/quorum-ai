package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// BuildAttachmentsContext returns a prompt-friendly section describing workflow attachments.
// workDir is optional; when provided, the section includes paths relative to the working directory.
func BuildAttachmentsContext(state *core.WorkflowState, workDir string) string {
	if state == nil || len(state.Attachments) == 0 {
		return ""
	}

	projectRoot, _ := os.Getwd()

	var sb strings.Builder
	sb.WriteString("## Workflow Attachments\n")
	sb.WriteString("The user provided the following documents. Read them from disk if needed.\n\n")

	for _, a := range state.Attachments {
		sb.WriteString(fmt.Sprintf("- %s (%d bytes)\n", a.Name, a.Size))
		sb.WriteString(fmt.Sprintf("  - Project path: %s\n", a.Path))

		if projectRoot != "" {
			abs := filepath.Join(projectRoot, filepath.FromSlash(a.Path))
			sb.WriteString(fmt.Sprintf("  - Absolute path: %s\n", abs))

			if strings.TrimSpace(workDir) != "" {
				if rel, err := filepath.Rel(workDir, abs); err == nil {
					sb.WriteString(fmt.Sprintf("  - From working dir: %s\n", filepath.ToSlash(rel)))
				}
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}
