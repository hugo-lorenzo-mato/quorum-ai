package issues

// ProgressIssue provides lightweight issue info for progress events.
type ProgressIssue struct {
	Title       string
	TaskID      string
	FileName    string
	IsMainIssue bool
}

// ProgressReporter is an optional callback interface for reporting generation/publishing progress.
// Implementations are expected to be cheap and non-blocking.
type ProgressReporter interface {
	OnIssuesGenerationProgress(workflowID, stage string, current, total int, issue *ProgressIssue, message string)
	OnIssuesPublishingProgress(p PublishingProgressParams)
}
