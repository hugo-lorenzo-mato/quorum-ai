package events_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestNewBaseEvent(t *testing.T) {
	e := events.NewBaseEvent("test_type", "wf-1", "proj-1")
	if e.EventType() != "test_type" {
		t.Errorf("got type %q, want %q", e.EventType(), "test_type")
	}
	if e.WorkflowID() != "wf-1" {
		t.Errorf("got workflow %q, want %q", e.WorkflowID(), "wf-1")
	}
	if e.ProjectID() != "proj-1" {
		t.Errorf("got project %q, want %q", e.ProjectID(), "proj-1")
	}
	if e.Timestamp().IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestNewBaseEventLegacy(t *testing.T) {
	e := events.NewBaseEventLegacy("test_type", "wf-1")
	if e.ProjectID() != "" {
		t.Errorf("expected empty project ID, got %q", e.ProjectID())
	}
}

// --- Agent events ---

func TestNewAgentStreamEvent(t *testing.T) {
	e := events.NewAgentStreamEvent("wf-1", "proj-1", events.AgentStarted, "claude", "Initialized")
	if e.EventType() != events.TypeAgentEvent {
		t.Errorf("got type %q, want %q", e.EventType(), events.TypeAgentEvent)
	}
	if e.Agent != "claude" {
		t.Errorf("got agent %q, want %q", e.Agent, "claude")
	}
	if e.Message != "Initialized" {
		t.Errorf("got message %q, want %q", e.Message, "Initialized")
	}
	if e.EventKind != events.AgentStarted {
		t.Errorf("got kind %q, want %q", e.EventKind, events.AgentStarted)
	}
}

func TestNewAgentStreamEventAt(t *testing.T) {
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	e := events.NewAgentStreamEventAt(ts, "wf-1", "proj-1", events.AgentCompleted, "gemini", "Done")
	if e.EventTime != ts {
		t.Errorf("got time %v, want %v", e.EventTime, ts)
	}
	if e.Timestamp() != ts {
		t.Errorf("base time mismatch: got %v, want %v", e.Timestamp(), ts)
	}
}

func TestAgentStreamEvent_WithData(t *testing.T) {
	e := events.NewAgentStreamEvent("wf-1", "proj-1", events.AgentToolUse, "codex", "tool call")
	e2 := e.WithData(map[string]interface{}{"tool": "bash"})
	if e2.Data["tool"] != "bash" {
		t.Errorf("expected data to contain tool=bash")
	}
}

// --- Chat events ---

func TestNewUserInputRequestedEvent(t *testing.T) {
	e := events.NewUserInputRequestedEvent("wf-1", "proj-1", "req-1", "Continue?", []string{"yes", "no"})
	if e.EventType() != events.TypeUserInputRequested {
		t.Errorf("got type %q", e.EventType())
	}
	if e.RequestID != "req-1" {
		t.Errorf("got request ID %q", e.RequestID)
	}
	if e.Prompt != "Continue?" {
		t.Errorf("got prompt %q", e.Prompt)
	}
	if len(e.Options) != 2 {
		t.Errorf("got %d options, want 2", len(e.Options))
	}
}

func TestNewUserInputProvidedEvent(t *testing.T) {
	e := events.NewUserInputProvidedEvent("wf-1", "proj-1", "req-1", "yes", false)
	if e.EventType() != events.TypeUserInputProvided {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Input != "yes" {
		t.Errorf("got input %q", e.Input)
	}
	if e.Cancelled {
		t.Error("should not be cancelled")
	}
}

func TestNewChatMessageEvent(t *testing.T) {
	e := events.NewChatMessageEvent("wf-1", "proj-1", events.RoleUser, "", "hello")
	if e.EventType() != events.TypeChatMessageReceived {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Role != events.RoleUser {
		t.Errorf("got role %q", e.Role)
	}
	if e.Content != "hello" {
		t.Errorf("got content %q", e.Content)
	}
}

func TestNewAgentResponseChunkEvent(t *testing.T) {
	e := events.NewAgentResponseChunkEvent("wf-1", "proj-1", "claude", "chunk text", true)
	if e.EventType() != events.TypeAgentResponseChunk {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Agent != "claude" {
		t.Errorf("got agent %q", e.Agent)
	}
	if !e.IsFinal {
		t.Error("expected final chunk")
	}
}

// --- Config events ---

func TestNewConfigLoadedEvent(t *testing.T) {
	e := events.NewConfigLoadedEvent("wf-1", "proj-1", "/path/to/config.yaml", "project", "custom", "etag1", "etag2", 1, "/snapshots/snap.yaml", "")
	if e.EventType() != events.TypeConfigLoaded {
		t.Errorf("got type %q", e.EventType())
	}
	if e.ConfigPath != "/path/to/config.yaml" {
		t.Errorf("got path %q", e.ConfigPath)
	}
	if e.ConfigScope != "project" {
		t.Errorf("got scope %q", e.ConfigScope)
	}
	if e.ExecutionID != 1 {
		t.Errorf("got execution ID %d", e.ExecutionID)
	}
}

// --- Control events ---

func TestNewPauseRequestEvent(t *testing.T) {
	e := events.NewPauseRequestEvent("wf-1", "proj-1", "user requested")
	if e.EventType() != events.TypePauseRequest {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Reason != "user requested" {
		t.Errorf("got reason %q", e.Reason)
	}
}

func TestNewResumeRequestEvent(t *testing.T) {
	e := events.NewResumeRequestEvent("wf-1", "proj-1")
	if e.EventType() != events.TypeResumeRequest {
		t.Errorf("got type %q", e.EventType())
	}
}

func TestNewAbortRequestEvent(t *testing.T) {
	e := events.NewAbortRequestEvent("wf-1", "proj-1", "timeout", true)
	if e.EventType() != events.TypeAbortRequest {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Reason != "timeout" {
		t.Errorf("got reason %q", e.Reason)
	}
	if !e.Force {
		t.Error("expected force=true")
	}
}

func TestNewRetryRequestEvent(t *testing.T) {
	e := events.NewRetryRequestEvent("wf-1", "proj-1", "task-1")
	if e.EventType() != events.TypeRetryRequest {
		t.Errorf("got type %q", e.EventType())
	}
	if e.TaskID != "task-1" {
		t.Errorf("got task ID %q", e.TaskID)
	}
}

func TestNewSkipRequestEvent(t *testing.T) {
	e := events.NewSkipRequestEvent("wf-1", "proj-1", "task-1", "not relevant")
	if e.EventType() != events.TypeSkipRequest {
		t.Errorf("got type %q", e.EventType())
	}
	if e.TaskID != "task-1" || e.Reason != "not relevant" {
		t.Errorf("unexpected fields: task=%q reason=%q", e.TaskID, e.Reason)
	}
}

// --- Kanban events ---

func TestNewKanbanWorkflowMovedEvent(t *testing.T) {
	e := events.NewKanbanWorkflowMovedEvent("wf-1", "proj-1", "backlog", "in_progress", 0, true)
	if e.EventType() != events.TypeKanbanWorkflowMoved {
		t.Errorf("got type %q", e.EventType())
	}
	if e.FromColumn != "backlog" || e.ToColumn != "in_progress" {
		t.Errorf("columns: from=%q to=%q", e.FromColumn, e.ToColumn)
	}
	if !e.UserInitiated {
		t.Error("expected user initiated")
	}
}

func TestNewKanbanExecutionStartedEvent(t *testing.T) {
	e := events.NewKanbanExecutionStartedEvent("wf-1", "proj-1", 3)
	if e.QueuePosition != 3 {
		t.Errorf("got position %d", e.QueuePosition)
	}
}

func TestNewKanbanExecutionCompletedEvent(t *testing.T) {
	e := events.NewKanbanExecutionCompletedEvent("wf-1", "proj-1", "https://github.com/pr/1", 1)
	if e.PRURL != "https://github.com/pr/1" || e.PRNumber != 1 {
		t.Errorf("pr: url=%q number=%d", e.PRURL, e.PRNumber)
	}
}

func TestNewKanbanExecutionFailedEvent(t *testing.T) {
	e := events.NewKanbanExecutionFailedEvent("wf-1", "proj-1", "build failed", 3)
	if e.Error != "build failed" || e.ConsecutiveFailures != 3 {
		t.Errorf("error=%q failures=%d", e.Error, e.ConsecutiveFailures)
	}
}

func TestNewKanbanEngineStateChangedEvent(t *testing.T) {
	wfID := "wf-1"
	e := events.NewKanbanEngineStateChangedEvent("proj-1", true, &wfID, false)
	if !e.Enabled {
		t.Error("expected enabled")
	}
	if e.CircuitBreakerOpen {
		t.Error("expected circuit breaker closed")
	}
	if *e.CurrentWorkflowID != "wf-1" {
		t.Errorf("got workflow ID %q", *e.CurrentWorkflowID)
	}
}

func TestNewKanbanEngineStateChangedEvent_NilWorkflowID(t *testing.T) {
	e := events.NewKanbanEngineStateChangedEvent("proj-1", false, nil, true)
	if e.CurrentWorkflowID != nil {
		t.Error("expected nil workflow ID")
	}
	if e.WorkflowID() != "" {
		t.Errorf("expected empty workflow ID, got %q", e.WorkflowID())
	}
}

func TestNewKanbanCircuitBreakerOpenedEvent(t *testing.T) {
	ts := time.Now()
	e := events.NewKanbanCircuitBreakerOpenedEvent("proj-1", 5, 3, ts)
	if e.ConsecutiveFailures != 5 || e.Threshold != 3 {
		t.Errorf("failures=%d threshold=%d", e.ConsecutiveFailures, e.Threshold)
	}
	if e.LastFailureAt != ts {
		t.Errorf("got time %v", e.LastFailureAt)
	}
}

// --- Metrics events ---

func TestNewMetricsUpdateEvent(t *testing.T) {
	e := events.NewMetricsUpdateEvent("wf-1", "proj-1", 1000, 500, 0.85, 5*time.Second)
	if e.EventType() != events.TypeMetricsUpdate {
		t.Errorf("got type %q", e.EventType())
	}
	if e.TotalTokensIn != 1000 || e.TotalTokensOut != 500 {
		t.Errorf("tokens: in=%d out=%d", e.TotalTokensIn, e.TotalTokensOut)
	}
	if e.ConsensusScore != 0.85 {
		t.Errorf("got score %f", e.ConsensusScore)
	}
	if e.Duration != 5*time.Second {
		t.Errorf("got duration %v", e.Duration)
	}
}

// --- Task events ---

func TestNewTaskCreatedEvent(t *testing.T) {
	e := events.NewTaskCreatedEvent("wf-1", "proj-1", "task-1", "execute", "analyze code", "claude", "opus")
	if e.EventType() != events.TypeTaskCreated {
		t.Errorf("got type %q", e.EventType())
	}
	if e.TaskID != "task-1" || e.Phase != "execute" || e.Agent != "claude" {
		t.Errorf("task=%q phase=%q agent=%q", e.TaskID, e.Phase, e.Agent)
	}
}

func TestNewTaskStartedEvent(t *testing.T) {
	e := events.NewTaskStartedEvent("wf-1", "proj-1", "task-1", "/tmp/worktree")
	if e.TaskID != "task-1" || e.WorktreePath != "/tmp/worktree" {
		t.Errorf("task=%q path=%q", e.TaskID, e.WorktreePath)
	}
}

func TestNewTaskProgressEvent(t *testing.T) {
	e := events.NewTaskProgressEvent("wf-1", "proj-1", "task-1", 0.5, 100, 50, "halfway")
	if e.Progress != 0.5 || e.TokensIn != 100 || e.Message != "halfway" {
		t.Errorf("progress=%f tokens_in=%d msg=%q", e.Progress, e.TokensIn, e.Message)
	}
}

func TestNewTaskCompletedEvent(t *testing.T) {
	e := events.NewTaskCompletedEvent("wf-1", "proj-1", "task-1", 5*time.Second, 1000, 500)
	if e.Duration != 5*time.Second || e.TokensIn != 1000 {
		t.Errorf("duration=%v tokens_in=%d", e.Duration, e.TokensIn)
	}
}

func TestNewTaskFailedEvent(t *testing.T) {
	e := events.NewTaskFailedEvent("wf-1", "proj-1", "task-1", errors.New("boom"), true)
	if e.Error != "boom" || !e.Retryable {
		t.Errorf("error=%q retryable=%v", e.Error, e.Retryable)
	}
}

func TestNewTaskFailedEvent_NilError(t *testing.T) {
	e := events.NewTaskFailedEvent("wf-1", "proj-1", "task-1", nil, false)
	if e.Error != "" {
		t.Errorf("expected empty error, got %q", e.Error)
	}
}

func TestNewTaskSkippedEvent(t *testing.T) {
	e := events.NewTaskSkippedEvent("wf-1", "proj-1", "task-1", "not needed")
	if e.TaskID != "task-1" || e.Reason != "not needed" {
		t.Errorf("task=%q reason=%q", e.TaskID, e.Reason)
	}
}

func TestNewTaskRetryEvent(t *testing.T) {
	e := events.NewTaskRetryEvent("wf-1", "proj-1", "task-1", 2, 3, errors.New("timeout"))
	if e.AttemptNum != 2 || e.MaxAttempts != 3 || e.Error != "timeout" {
		t.Errorf("attempt=%d max=%d error=%q", e.AttemptNum, e.MaxAttempts, e.Error)
	}
}

func TestNewTaskRetryEvent_NilError(t *testing.T) {
	e := events.NewTaskRetryEvent("wf-1", "proj-1", "task-1", 1, 3, nil)
	if e.Error != "" {
		t.Errorf("expected empty error, got %q", e.Error)
	}
}

// --- Workflow events ---

func TestNewWorkflowStartedEvent(t *testing.T) {
	e := events.NewWorkflowStartedEvent("wf-1", "proj-1", "fix the bug")
	if e.EventType() != events.TypeWorkflowStarted {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Prompt != "fix the bug" {
		t.Errorf("got prompt %q", e.Prompt)
	}
}

func TestNewWorkflowStateUpdatedEvent(t *testing.T) {
	e := events.NewWorkflowStateUpdatedEvent("wf-1", "proj-1", "execute", 5, 3, 1, 0)
	if e.Phase != "execute" || e.TotalTasks != 5 || e.Completed != 3 || e.Failed != 1 {
		t.Errorf("phase=%q total=%d completed=%d failed=%d", e.Phase, e.TotalTasks, e.Completed, e.Failed)
	}
}

func TestNewWorkflowCompletedEvent(t *testing.T) {
	e := events.NewWorkflowCompletedEvent("wf-1", "proj-1", 10*time.Second)
	if e.EventType() != events.TypeWorkflowCompleted {
		t.Errorf("got type %q", e.EventType())
	}
	if e.Duration != 10*time.Second {
		t.Errorf("got duration %v", e.Duration)
	}
}

func TestNewWorkflowFailedEvent(t *testing.T) {
	e := events.NewWorkflowFailedEvent("wf-1", "proj-1", "execute", errors.New("agent timeout"))
	if e.Phase != "execute" || e.Error != "agent timeout" {
		t.Errorf("phase=%q error=%q", e.Phase, e.Error)
	}
	if e.ErrorCode != "" {
		t.Errorf("expected empty error code for plain error, got %q", e.ErrorCode)
	}
}

func TestNewWorkflowFailedEvent_DomainError(t *testing.T) {
	domErr := core.ErrTimeout("agent timed out")
	e := events.NewWorkflowFailedEvent("wf-1", "proj-1", "execute", domErr)
	if e.ErrorCategory != string(core.ErrCatTimeout) {
		t.Errorf("got category %q", e.ErrorCategory)
	}
}

func TestNewWorkflowFailedEvent_NilError(t *testing.T) {
	e := events.NewWorkflowFailedEvent("wf-1", "proj-1", "execute", nil)
	if e.Error != "" {
		t.Errorf("expected empty error, got %q", e.Error)
	}
}

func TestNewWorkflowPausedEvent(t *testing.T) {
	e := events.NewWorkflowPausedEvent("wf-1", "proj-1", "execute", "user pause")
	if e.Phase != "execute" || e.Reason != "user pause" {
		t.Errorf("phase=%q reason=%q", e.Phase, e.Reason)
	}
}

func TestNewWorkflowResumedEvent(t *testing.T) {
	e := events.NewWorkflowResumedEvent("wf-1", "proj-1", "execute")
	if e.FromPhase != "execute" {
		t.Errorf("got from_phase %q", e.FromPhase)
	}
}
