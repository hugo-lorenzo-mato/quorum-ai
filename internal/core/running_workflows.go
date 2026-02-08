package core

import "time"

// RunningWorkflowRecord represents the row stored in the running_workflows table.
// It is used to separate persisted "running" state from in-memory control handles.
type RunningWorkflowRecord struct {
	WorkflowID     WorkflowID `json:"workflow_id"`
	StartedAt      time.Time  `json:"started_at"`
	LockHolderPID  *int       `json:"lock_holder_pid,omitempty"`
	LockHolderHost string     `json:"lock_holder_host,omitempty"`
	HeartbeatAt    *time.Time `json:"heartbeat_at,omitempty"`
}
