package events

// Issue-related SSE event types.
const (
	TypeIssuesGenerationProgress = "issues_generation_progress"
	TypeIssuesPublishingProgress = "issues_publishing_progress"
)

// IssuesGenerationProgressEvent reports progress while generating issue draft files.
// It is emitted during AI generation (GenerateIssueFiles) and can be used by the UI
// to render incremental progress.
type IssuesGenerationProgressEvent struct {
	BaseEvent

	Stage       string `json:"stage"`
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	Message     string `json:"message,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	Title       string `json:"title,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	IsMainIssue bool   `json:"is_main_issue"`
}

func NewIssuesGenerationProgressEvent(workflowID, projectID, stage string, current, total int, message, fileName, title, taskID string, isMainIssue bool) IssuesGenerationProgressEvent {
	return IssuesGenerationProgressEvent{
		BaseEvent:    NewBaseEvent(TypeIssuesGenerationProgress, workflowID, projectID),
		Stage:       stage,
		Current:     current,
		Total:       total,
		Message:     message,
		FileName:    fileName,
		Title:       title,
		TaskID:      taskID,
		IsMainIssue: isMainIssue,
	}
}

// IssuesPublishingProgressEvent reports progress while creating/publishing issues to the provider.
type IssuesPublishingProgressEvent struct {
	BaseEvent

	Stage       string `json:"stage"`
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	Message     string `json:"message,omitempty"`
	Title       string `json:"title,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	IsMainIssue bool   `json:"is_main_issue"`
	IssueNumber int    `json:"issue_number,omitempty"`
	DryRun      bool   `json:"dry_run"`
}

func NewIssuesPublishingProgressEvent(workflowID, projectID, stage string, current, total int, message, title, taskID string, isMainIssue bool, issueNumber int, dryRun bool) IssuesPublishingProgressEvent {
	return IssuesPublishingProgressEvent{
		BaseEvent:    NewBaseEvent(TypeIssuesPublishingProgress, workflowID, projectID),
		Stage:       stage,
		Current:     current,
		Total:       total,
		Message:     message,
		Title:       title,
		TaskID:      taskID,
		IsMainIssue: isMainIssue,
		IssueNumber: issueNumber,
		DryRun:      dryRun,
	}
}

