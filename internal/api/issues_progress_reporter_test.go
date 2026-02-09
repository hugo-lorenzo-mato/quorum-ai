package api

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
)

func TestIssuesSSEProgressReporter_Generation(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()

	ch := bus.Subscribe(events.TypeIssuesGenerationProgress)
	reporter := newIssuesSSEProgressReporter(bus, "proj-1")

	issue := &issues.ProgressIssue{
		FileName:    "003-fix-auth.md",
		Title:       "Fix auth flow",
		TaskID:      "task-42",
		IsMainIssue: true,
	}

	reporter.OnIssuesGenerationProgress("wf-gen-1", "generating", 3, 10, issue, "Generating issue 3 of 10")

	select {
	case evt := <-ch:
		if evt.EventType() != events.TypeIssuesGenerationProgress {
			t.Errorf("expected event type %q, got %q", events.TypeIssuesGenerationProgress, evt.EventType())
		}
		if evt.WorkflowID() != "wf-gen-1" {
			t.Errorf("expected workflow_id 'wf-gen-1', got %q", evt.WorkflowID())
		}
		if evt.ProjectID() != "proj-1" {
			t.Errorf("expected project_id 'proj-1', got %q", evt.ProjectID())
		}

		genEvt, ok := evt.(events.IssuesGenerationProgressEvent)
		if !ok {
			t.Fatal("expected IssuesGenerationProgressEvent type assertion to succeed")
		}
		if genEvt.Stage != "generating" {
			t.Errorf("expected stage 'generating', got %q", genEvt.Stage)
		}
		if genEvt.Current != 3 {
			t.Errorf("expected current 3, got %d", genEvt.Current)
		}
		if genEvt.Total != 10 {
			t.Errorf("expected total 10, got %d", genEvt.Total)
		}
		if genEvt.Message != "Generating issue 3 of 10" {
			t.Errorf("expected message 'Generating issue 3 of 10', got %q", genEvt.Message)
		}
		if genEvt.FileName != "003-fix-auth.md" {
			t.Errorf("expected file_name '003-fix-auth.md', got %q", genEvt.FileName)
		}
		if genEvt.Title != "Fix auth flow" {
			t.Errorf("expected title 'Fix auth flow', got %q", genEvt.Title)
		}
		if genEvt.TaskID != "task-42" {
			t.Errorf("expected task_id 'task-42', got %q", genEvt.TaskID)
		}
		if !genEvt.IsMainIssue {
			t.Error("expected is_main_issue to be true")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for generation progress event")
	}
}

func TestIssuesSSEProgressReporter_Publishing(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()

	ch := bus.Subscribe(events.TypeIssuesPublishingProgress)
	reporter := newIssuesSSEProgressReporter(bus, "proj-2")

	issue := &issues.ProgressIssue{
		Title:       "Add CI pipeline",
		TaskID:      "task-99",
		IsMainIssue: false,
	}

	reporter.OnIssuesPublishingProgress(issues.PublishingProgressParams{
		WorkflowID:  "wf-pub-1",
		Stage:       "publishing",
		Current:     2,
		Total:       5,
		Issue:       issue,
		IssueNumber: 42,
		DryRun:      true,
		Message:     "Publishing issue 2 of 5",
	})

	select {
	case evt := <-ch:
		if evt.EventType() != events.TypeIssuesPublishingProgress {
			t.Errorf("expected event type %q, got %q", events.TypeIssuesPublishingProgress, evt.EventType())
		}
		if evt.WorkflowID() != "wf-pub-1" {
			t.Errorf("expected workflow_id 'wf-pub-1', got %q", evt.WorkflowID())
		}
		if evt.ProjectID() != "proj-2" {
			t.Errorf("expected project_id 'proj-2', got %q", evt.ProjectID())
		}

		pubEvt, ok := evt.(events.IssuesPublishingProgressEvent)
		if !ok {
			t.Fatal("expected IssuesPublishingProgressEvent type assertion to succeed")
		}
		if pubEvt.Stage != "publishing" {
			t.Errorf("expected stage 'publishing', got %q", pubEvt.Stage)
		}
		if pubEvt.Current != 2 {
			t.Errorf("expected current 2, got %d", pubEvt.Current)
		}
		if pubEvt.Total != 5 {
			t.Errorf("expected total 5, got %d", pubEvt.Total)
		}
		if pubEvt.Message != "Publishing issue 2 of 5" {
			t.Errorf("expected message 'Publishing issue 2 of 5', got %q", pubEvt.Message)
		}
		if pubEvt.Title != "Add CI pipeline" {
			t.Errorf("expected title 'Add CI pipeline', got %q", pubEvt.Title)
		}
		if pubEvt.TaskID != "task-99" {
			t.Errorf("expected task_id 'task-99', got %q", pubEvt.TaskID)
		}
		if pubEvt.IsMainIssue {
			t.Error("expected is_main_issue to be false")
		}
		if pubEvt.IssueNumber != 42 {
			t.Errorf("expected issue_number 42, got %d", pubEvt.IssueNumber)
		}
		if !pubEvt.DryRun {
			t.Error("expected dry_run to be true")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for publishing progress event")
	}
}

func TestIssuesSSEProgressReporter_NilSafe(t *testing.T) {
	t.Parallel()
	// nil reporter should not panic
	var nilReporter *issuesSSEProgressReporter
	nilReporter.OnIssuesGenerationProgress("wf-1", "start", 0, 0, nil, "msg")
	nilReporter.OnIssuesPublishingProgress(issues.PublishingProgressParams{WorkflowID: "wf-1", Stage: "start", Message: "msg"})

	// reporter with nil bus should not panic
	reporterNilBus := &issuesSSEProgressReporter{bus: nil, projectID: "proj-1"}
	reporterNilBus.OnIssuesGenerationProgress("wf-1", "start", 0, 0, nil, "msg")
	reporterNilBus.OnIssuesPublishingProgress(issues.PublishingProgressParams{WorkflowID: "wf-1", Stage: "start", Message: "msg"})
}

func TestIssuesSSEProgressReporter_WithNilIssue(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	defer bus.Close()

	ch := bus.Subscribe()
	reporter := newIssuesSSEProgressReporter(bus, "proj-1")

	// Call with nil issue pointer - should use zero-value defaults
	reporter.OnIssuesGenerationProgress("wf-1", "start", 0, 5, nil, "Starting generation")

	select {
	case evt := <-ch:
		genEvt, ok := evt.(events.IssuesGenerationProgressEvent)
		if !ok {
			t.Fatal("expected IssuesGenerationProgressEvent type assertion to succeed")
		}
		if genEvt.FileName != "" {
			t.Errorf("expected empty file_name for nil issue, got %q", genEvt.FileName)
		}
		if genEvt.Title != "" {
			t.Errorf("expected empty title for nil issue, got %q", genEvt.Title)
		}
		if genEvt.TaskID != "" {
			t.Errorf("expected empty task_id for nil issue, got %q", genEvt.TaskID)
		}
		if genEvt.IsMainIssue {
			t.Error("expected is_main_issue to be false for nil issue")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	// Also test nil issue for publishing
	reporter.OnIssuesPublishingProgress(issues.PublishingProgressParams{WorkflowID: "wf-2", Stage: "start", Total: 3, Message: "Starting publishing"})

	select {
	case evt := <-ch:
		pubEvt, ok := evt.(events.IssuesPublishingProgressEvent)
		if !ok {
			t.Fatal("expected IssuesPublishingProgressEvent type assertion to succeed")
		}
		if pubEvt.Title != "" {
			t.Errorf("expected empty title for nil issue, got %q", pubEvt.Title)
		}
		if pubEvt.TaskID != "" {
			t.Errorf("expected empty task_id for nil issue, got %q", pubEvt.TaskID)
		}
		if pubEvt.IsMainIssue {
			t.Error("expected is_main_issue to be false for nil issue")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}
