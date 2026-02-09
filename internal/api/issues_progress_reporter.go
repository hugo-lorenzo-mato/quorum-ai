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
	r.bus.Publish(events.NewIssuesGenerationProgressEvent(events.IssuesGenerationProgressParams{
		WorkflowID:  workflowID,
		ProjectID:   r.projectID,
		Stage:       stage,
		Current:     current,
		Total:       total,
		Message:     message,
		FileName:    fileName,
		Title:       title,
		TaskID:      taskID,
		IsMainIssue: isMain,
	}))
}

func (r *issuesSSEProgressReporter) OnIssuesPublishingProgress(p issues.PublishingProgressParams) {
	if r == nil || r.bus == nil {
		return
	}
	var title, taskID string
	var isMain bool
	if p.Issue != nil {
		title = p.Issue.Title
		taskID = p.Issue.TaskID
		isMain = p.Issue.IsMainIssue
	}
	r.bus.Publish(events.NewIssuesPublishingProgressEvent(events.IssuesPublishingProgressParams{
		WorkflowID:  p.WorkflowID,
		ProjectID:   r.projectID,
		Stage:       p.Stage,
		Current:     p.Current,
		Total:       p.Total,
		Message:     p.Message,
		Title:       title,
		TaskID:      taskID,
		IsMainIssue: isMain,
		IssueNumber: p.IssueNumber,
		DryRun:      p.DryRun,
	}))
}
