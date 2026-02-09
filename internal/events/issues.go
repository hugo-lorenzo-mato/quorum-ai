package events

// Issue-related SSE event types.
const (
	TypeIssuesGenerationProgress = "issues_generation_progress"
	TypeIssuesPublishingProgress = "issues_publishing_progress"
)

// IssuesGenerationProgressParams holds parameters for creating an IssuesGenerationProgressEvent.
type IssuesGenerationProgressParams struct {
	WorkflowID  string
	ProjectID   string
	Stage       string
	Current     int
	Total       int
	Message     string
	FileName    string
	Title       string
	TaskID      string
	IsMainIssue bool
}

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

// NewIssuesGenerationProgressEvent creates an IssuesGenerationProgressEvent from params.
func NewIssuesGenerationProgressEvent(p IssuesGenerationProgressParams) IssuesGenerationProgressEvent {
	return IssuesGenerationProgressEvent{
		BaseEvent:   NewBaseEvent(TypeIssuesGenerationProgress, p.WorkflowID, p.ProjectID),
		Stage:       p.Stage,
		Current:     p.Current,
		Total:       p.Total,
		Message:     p.Message,
		FileName:    p.FileName,
		Title:       p.Title,
		TaskID:      p.TaskID,
		IsMainIssue: p.IsMainIssue,
	}
}

// IssuesPublishingProgressParams holds parameters for creating an IssuesPublishingProgressEvent.
type IssuesPublishingProgressParams struct {
	WorkflowID  string
	ProjectID   string
	Stage       string
	Current     int
	Total       int
	Message     string
	Title       string
	TaskID      string
	IsMainIssue bool
	IssueNumber int
	DryRun      bool
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

// NewIssuesPublishingProgressEvent creates an IssuesPublishingProgressEvent from params.
func NewIssuesPublishingProgressEvent(p IssuesPublishingProgressParams) IssuesPublishingProgressEvent {
	return IssuesPublishingProgressEvent{
		BaseEvent:   NewBaseEvent(TypeIssuesPublishingProgress, p.WorkflowID, p.ProjectID),
		Stage:       p.Stage,
		Current:     p.Current,
		Total:       p.Total,
		Message:     p.Message,
		Title:       p.Title,
		TaskID:      p.TaskID,
		IsMainIssue: p.IsMainIssue,
		IssueNumber: p.IssueNumber,
		DryRun:      p.DryRun,
	}
}

