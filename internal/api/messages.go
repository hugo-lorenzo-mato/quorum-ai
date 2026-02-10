package api

// Shared message constants (primarily to avoid duplicated string literals; Sonar rule S1192).
const (
	msgInvalidRequestBody        = "invalid request body"
	msgTaskNotFound              = "task not found"
	msgFailedToSaveWorkflowState = "failed to save workflow state"
)
