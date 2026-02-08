package api

import (
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/issues"
)

// issuesSSEProgressReporter bridges issues.ProgressReporter to the EventBus for SSE streaming.
type issuesSSEProgressReporter struct {
	bus       *events.EventBus
	projectID string
}

func newIssuesSSEProgressReporter(bus *events.EventBus, projectID string) *issuesSSEProgressReporter {
	return &issuesSSEProgressReporter{
		bus:       bus,
		projectID: projectID,
	}
}

func (r *issuesSSEProgressReporter) OnIssuesGenerationProgress(workflowID, stage string, current, total int, issue *issues.ProgressIssue, message string) {
	if r == nil || r.bus == nil {
		return
	}
	var fileName, title, taskID string
	var isMain bool
	if issue != nil {
		fileName = issue.FileName
		title = issue.Title
		taskID = issue.TaskID
		isMain = issue.IsMainIssue
	}
	r.bus.Publish(events.NewIssuesGenerationProgressEvent(
		workflowID,
		r.projectID,
		stage,
		current,
		total,
		message,
		fileName,
		title,
		taskID,
		isMain,
	))
}

func (r *issuesSSEProgressReporter) OnIssuesPublishingProgress(workflowID, stage string, current, total int, issue *issues.ProgressIssue, issueNumber int, dryRun bool, message string) {
	if r == nil || r.bus == nil {
		return
	}
	var title, taskID string
	var isMain bool
	if issue != nil {
		title = issue.Title
		taskID = issue.TaskID
		isMain = issue.IsMainIssue
	}
	r.bus.Publish(events.NewIssuesPublishingProgressEvent(
		workflowID,
		r.projectID,
		stage,
		current,
		total,
		message,
		title,
		taskID,
		isMain,
		issueNumber,
		dryRun,
	))
}

