package events

import (
	"testing"
	"time"
)

func TestNewIssuesGenerationProgressEvent(t *testing.T) {
	t.Parallel()
	before := time.Now()
	event := NewIssuesGenerationProgressEvent(IssuesGenerationProgressParams{
		WorkflowID:  "wf-gen-1",
		ProjectID:   "proj-1",
		Stage:       "generating",
		Current:     3,
		Total:       10,
		Message:     "Generating issue 3 of 10",
		FileName:    "003-fix-auth.md",
		Title:       "Fix auth flow",
		TaskID:      "task-42",
		IsMainIssue: true,
	})
	after := time.Now()

	if event.EventType() != TypeIssuesGenerationProgress {
		t.Errorf("expected type %q, got %q", TypeIssuesGenerationProgress, event.EventType())
	}
	if event.WorkflowID() != "wf-gen-1" {
		t.Errorf("expected workflow_id 'wf-gen-1', got %q", event.WorkflowID())
	}
	if event.ProjectID() != "proj-1" {
		t.Errorf("expected project_id 'proj-1', got %q", event.ProjectID())
	}
	if event.Stage != "generating" {
		t.Errorf("expected stage 'generating', got %q", event.Stage)
	}
	if event.Current != 3 {
		t.Errorf("expected current 3, got %d", event.Current)
	}
	if event.Total != 10 {
		t.Errorf("expected total 10, got %d", event.Total)
	}
	if event.Message != "Generating issue 3 of 10" {
		t.Errorf("expected message 'Generating issue 3 of 10', got %q", event.Message)
	}
	if event.FileName != "003-fix-auth.md" {
		t.Errorf("expected file_name '003-fix-auth.md', got %q", event.FileName)
	}
	if event.Title != "Fix auth flow" {
		t.Errorf("expected title 'Fix auth flow', got %q", event.Title)
	}
	if event.TaskID != "task-42" {
		t.Errorf("expected task_id 'task-42', got %q", event.TaskID)
	}
	if !event.IsMainIssue {
		t.Error("expected is_main_issue to be true")
	}
	if event.Timestamp().Before(before) || event.Timestamp().After(after) {
		t.Errorf("expected timestamp between %v and %v, got %v", before, after, event.Timestamp())
	}
}

func TestNewIssuesPublishingProgressEvent(t *testing.T) {
	t.Parallel()
	before := time.Now()
	event := NewIssuesPublishingProgressEvent(IssuesPublishingProgressParams{
		WorkflowID:  "wf-pub-1",
		ProjectID:   "proj-2",
		Stage:       "publishing",
		Current:     2,
		Total:       5,
		Message:     "Publishing issue 2 of 5",
		Title:       "Add CI pipeline",
		TaskID:      "task-99",
		IsMainIssue: false,
		IssueNumber: 42,
		DryRun:      true,
	})
	after := time.Now()

	if event.EventType() != TypeIssuesPublishingProgress {
		t.Errorf("expected type %q, got %q", TypeIssuesPublishingProgress, event.EventType())
	}
	if event.WorkflowID() != "wf-pub-1" {
		t.Errorf("expected workflow_id 'wf-pub-1', got %q", event.WorkflowID())
	}
	if event.ProjectID() != "proj-2" {
		t.Errorf("expected project_id 'proj-2', got %q", event.ProjectID())
	}
	if event.Stage != "publishing" {
		t.Errorf("expected stage 'publishing', got %q", event.Stage)
	}
	if event.Current != 2 {
		t.Errorf("expected current 2, got %d", event.Current)
	}
	if event.Total != 5 {
		t.Errorf("expected total 5, got %d", event.Total)
	}
	if event.Message != "Publishing issue 2 of 5" {
		t.Errorf("expected message 'Publishing issue 2 of 5', got %q", event.Message)
	}
	if event.Title != "Add CI pipeline" {
		t.Errorf("expected title 'Add CI pipeline', got %q", event.Title)
	}
	if event.TaskID != "task-99" {
		t.Errorf("expected task_id 'task-99', got %q", event.TaskID)
	}
	if event.IsMainIssue {
		t.Error("expected is_main_issue to be false")
	}
	if event.IssueNumber != 42 {
		t.Errorf("expected issue_number 42, got %d", event.IssueNumber)
	}
	if !event.DryRun {
		t.Error("expected dry_run to be true")
	}
	if event.Timestamp().Before(before) || event.Timestamp().After(after) {
		t.Errorf("expected timestamp between %v and %v, got %v", before, after, event.Timestamp())
	}
}

func TestIssuesEventTypeConstants(t *testing.T) {
	t.Parallel()
	if TypeIssuesGenerationProgress != "issues_generation_progress" {
		t.Errorf("expected TypeIssuesGenerationProgress to be 'issues_generation_progress', got %q", TypeIssuesGenerationProgress)
	}
	if TypeIssuesPublishingProgress != "issues_publishing_progress" {
		t.Errorf("expected TypeIssuesPublishingProgress to be 'issues_publishing_progress', got %q", TypeIssuesPublishingProgress)
	}
}

func TestIssuesGenerationProgressEvent_ImplementsEventInterface(t *testing.T) {
	t.Parallel()
	event := NewIssuesGenerationProgressEvent(IssuesGenerationProgressParams{WorkflowID: "wf-1", ProjectID: "proj-1", Stage: "start"})
	var _ Event = event // Compile-time check
}

func TestIssuesPublishingProgressEvent_ImplementsEventInterface(t *testing.T) {
	t.Parallel()
	event := NewIssuesPublishingProgressEvent(IssuesPublishingProgressParams{WorkflowID: "wf-1", ProjectID: "proj-1", Stage: "start"})
	var _ Event = event // Compile-time check
}
