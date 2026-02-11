package workflow

import (
	"strings"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ---------------------------------------------------------------------------
// BuildAttachmentsContext coverage tests
// ---------------------------------------------------------------------------

func TestBuildAttachmentsContext_NilState(t *testing.T) {
	t.Parallel()
	result := BuildAttachmentsContext(nil, "/tmp")
	if result != "" {
		t.Errorf("expected empty string for nil state, got %q", result)
	}
}

func TestBuildAttachmentsContext_NoAttachments(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: nil,
		},
	}
	result := BuildAttachmentsContext(state, "/tmp")
	if result != "" {
		t.Errorf("expected empty string for no attachments, got %q", result)
	}
}

func TestBuildAttachmentsContext_EmptyAttachments(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: []core.Attachment{},
		},
	}
	result := BuildAttachmentsContext(state, "/tmp")
	if result != "" {
		t.Errorf("expected empty string for empty attachments, got %q", result)
	}
}

func TestBuildAttachmentsContext_WithAttachments(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: []core.Attachment{
				{
					Name: "spec.pdf",
					Path: ".quorum/attachments/spec.pdf",
					Size: 12345,
				},
				{
					Name: "design.png",
					Path: ".quorum/attachments/design.png",
					Size: 67890,
				},
			},
		},
	}
	result := BuildAttachmentsContext(state, "")
	if !strings.Contains(result, "## Workflow Attachments") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "spec.pdf") {
		t.Error("missing first attachment name")
	}
	if !strings.Contains(result, "12345 bytes") {
		t.Error("missing first attachment size")
	}
	if !strings.Contains(result, "design.png") {
		t.Error("missing second attachment name")
	}
	if !strings.Contains(result, "67890 bytes") {
		t.Error("missing second attachment size")
	}
}

func TestBuildAttachmentsContext_WithWorkDir(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: []core.Attachment{
				{
					Name: "spec.pdf",
					Path: ".quorum/attachments/spec.pdf",
					Size: 100,
				},
			},
		},
	}
	// Pass a workDir to get relative path output
	result := BuildAttachmentsContext(state, "/tmp/worktree")
	if !strings.Contains(result, "Absolute path") {
		t.Error("missing absolute path")
	}
	if !strings.Contains(result, "Project path") {
		t.Error("missing project path")
	}
	// With a non-empty workDir, "From working dir" should appear
	if !strings.Contains(result, "From working dir") {
		t.Error("missing relative path from working dir")
	}
}

func TestBuildAttachmentsContext_EmptyWorkDir(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: []core.Attachment{
				{
					Name: "file.txt",
					Path: ".quorum/attachments/file.txt",
					Size: 50,
				},
			},
		},
	}
	result := BuildAttachmentsContext(state, "")
	// With empty workDir, should NOT contain "From working dir"
	if strings.Contains(result, "From working dir") {
		t.Error("should not contain 'From working dir' with empty workDir")
	}
}

func TestBuildAttachmentsContext_WhitespaceOnlyWorkDir(t *testing.T) {
	t.Parallel()
	state := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Attachments: []core.Attachment{
				{
					Name: "file.txt",
					Path: ".quorum/attachments/file.txt",
					Size: 50,
				},
			},
		},
	}
	result := BuildAttachmentsContext(state, "   ")
	// With whitespace-only workDir, should NOT contain "From working dir"
	if strings.Contains(result, "From working dir") {
		t.Error("should not contain 'From working dir' with whitespace-only workDir")
	}
}
