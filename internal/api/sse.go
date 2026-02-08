package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// handleSSE handles Server-Sent Events for real-time updates.
// Uses project-scoped EventBus if a project is specified via ?project= query parameter.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Check if streaming is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Get context for cancellation
	ctx := r.Context()

	// Get project-scoped EventBus (falls back to global if no project context)
	eventBus := s.getProjectEventBus(ctx)
	if eventBus == nil {
		respondError(w, http.StatusServiceUnavailable, "event bus not available")
		return
	}

	// Subscribe to project-scoped events
	eventCh := eventBus.Subscribe()

	projectID := getProjectID(ctx)
	s.logger.Info("SSE client connected", "remote_addr", r.RemoteAddr, "project_id", projectID)

	// Send initial connection event
	s.sendSSEEvent(w, flusher, "connected", map[string]string{
		"status": "connected",
	})

	// Stream events until client disconnects
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("SSE client disconnected", "remote_addr", r.RemoteAddr)
			return

		case event, ok := <-eventCh:
			if !ok {
				// EventBus closed
				s.logger.Info("EventBus closed, ending SSE stream")
				return
			}

			// Convert event to SSE format
			s.sendEventToClient(w, flusher, event)
		}
	}
}

// sendSSEEvent writes an event to the SSE stream.
func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("failed to marshal SSE data", "error", err)
		return
	}

	// SSE format: event: type\ndata: json\n\n
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

// sendEventToClient converts an Event to SSE format and sends it.
func (s *Server) sendEventToClient(w http.ResponseWriter, flusher http.Flusher, event events.Event) {
	// Build SSE payload based on event type
	var payload interface{}

	switch e := event.(type) {
	case events.WorkflowStartedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"prompt":      e.Prompt,
			"timestamp":   e.Timestamp(),
		}

	case events.WorkflowStateUpdatedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"phase":       e.Phase,
			"total_tasks": e.TotalTasks,
			"completed":   e.Completed,
			"failed":      e.Failed,
			"skipped":     e.Skipped,
			"timestamp":   e.Timestamp(),
		}

	case events.WorkflowCompletedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"duration":    e.Duration.String(),
			"timestamp":   e.Timestamp(),
		}

	case events.WorkflowFailedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"phase":       e.Phase,
			"error":       e.Error,
			"timestamp":   e.Timestamp(),
		}

	case events.WorkflowPausedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"phase":       e.Phase,
			"reason":      e.Reason,
			"timestamp":   e.Timestamp(),
		}

	case events.WorkflowResumedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"from_phase":  e.FromPhase,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskCreatedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"task_id":     e.TaskID,
			"phase":       e.Phase,
			"name":        e.Name,
			"agent":       e.Agent,
			"model":       e.Model,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskStartedEvent:
		payload = map[string]interface{}{
			"workflow_id":   e.WorkflowID(),
			"task_id":       e.TaskID,
			"worktree_path": e.WorktreePath,
			"timestamp":     e.Timestamp(),
		}

	case events.TaskProgressEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"task_id":     e.TaskID,
			"progress":    e.Progress,
			"tokens_in":   e.TokensIn,
			"tokens_out":  e.TokensOut,
			"message":     e.Message,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskCompletedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"task_id":     e.TaskID,
			"duration":    e.Duration.String(),
			"tokens_in":   e.TokensIn,
			"tokens_out":  e.TokensOut,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskFailedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"task_id":     e.TaskID,
			"error":       e.Error,
			"retryable":   e.Retryable,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskSkippedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"task_id":     e.TaskID,
			"reason":      e.Reason,
			"timestamp":   e.Timestamp(),
		}

	case events.TaskRetryEvent:
		payload = map[string]interface{}{
			"workflow_id":  e.WorkflowID(),
			"task_id":      e.TaskID,
			"attempt_num":  e.AttemptNum,
			"max_attempts": e.MaxAttempts,
			"error":        e.Error,
			"timestamp":    e.Timestamp(),
		}

	case events.AgentStreamEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"event_kind":  string(e.EventKind),
			"agent":       e.Agent,
			"message":     e.Message,
			"data":        e.Data,
			"event_time":  e.EventTime,
			"timestamp":   e.Timestamp(),
		}

	// Issues events
	case events.IssuesGenerationProgressEvent:
		payload = map[string]interface{}{
			"workflow_id":    e.WorkflowID(),
			"stage":          e.Stage,
			"current":        e.Current,
			"total":          e.Total,
			"message":        e.Message,
			"file_name":      e.FileName,
			"title":          e.Title,
			"task_id":        e.TaskID,
			"is_main_issue":  e.IsMainIssue,
			"timestamp":      e.Timestamp(),
		}

	case events.IssuesPublishingProgressEvent:
		payload = map[string]interface{}{
			"workflow_id":    e.WorkflowID(),
			"stage":          e.Stage,
			"current":        e.Current,
			"total":          e.Total,
			"message":        e.Message,
			"title":          e.Title,
			"task_id":        e.TaskID,
			"is_main_issue":  e.IsMainIssue,
			"issue_number":   e.IssueNumber,
			"dry_run":        e.DryRun,
			"timestamp":      e.Timestamp(),
		}

	// Kanban events
	case events.KanbanWorkflowMovedEvent:
		payload = map[string]interface{}{
			"workflow_id":    e.WorkflowID(),
			"from_column":    e.FromColumn,
			"to_column":      e.ToColumn,
			"new_position":   e.NewPosition,
			"user_initiated": e.UserInitiated,
			"timestamp":      e.Timestamp(),
		}

	case events.KanbanExecutionStartedEvent:
		payload = map[string]interface{}{
			"workflow_id":    e.WorkflowID(),
			"queue_position": e.QueuePosition,
			"timestamp":      e.Timestamp(),
		}

	case events.KanbanExecutionCompletedEvent:
		payload = map[string]interface{}{
			"workflow_id": e.WorkflowID(),
			"pr_url":      e.PRURL,
			"pr_number":   e.PRNumber,
			"timestamp":   e.Timestamp(),
		}

	case events.KanbanExecutionFailedEvent:
		payload = map[string]interface{}{
			"workflow_id":          e.WorkflowID(),
			"error":                e.Error,
			"consecutive_failures": e.ConsecutiveFailures,
			"timestamp":            e.Timestamp(),
		}

	case events.KanbanEngineStateChangedEvent:
		payload = map[string]interface{}{
			"workflow_id":          e.WorkflowID(),
			"enabled":              e.Enabled,
			"current_workflow_id":  e.CurrentWorkflowID,
			"circuit_breaker_open": e.CircuitBreakerOpen,
			"timestamp":            e.Timestamp(),
		}

	case events.KanbanCircuitBreakerOpenedEvent:
		payload = map[string]interface{}{
			"workflow_id":          e.WorkflowID(),
			"consecutive_failures": e.ConsecutiveFailures,
			"threshold":            e.Threshold,
			"last_failure_at":      e.LastFailureAt,
			"timestamp":            e.Timestamp(),
		}

	default:
		// Generic event handling
		payload = map[string]interface{}{
			"workflow_id": event.WorkflowID(),
			"timestamp":   event.Timestamp(),
		}
	}

	s.sendSSEEvent(w, flusher, event.EventType(), payload)
}

// SSEClient represents a connected SSE client for testing.
type SSEClient struct {
	ctx      context.Context
	cancel   context.CancelFunc
	eventCh  <-chan events.Event
	eventBus *events.EventBus
}

// NewSSEClient creates a new SSE client for testing.
func NewSSEClient(eventBus *events.EventBus) *SSEClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &SSEClient{
		ctx:      ctx,
		cancel:   cancel,
		eventCh:  eventBus.Subscribe(),
		eventBus: eventBus,
	}
}

// Close closes the SSE client.
func (c *SSEClient) Close() {
	c.cancel()
}

// Events returns the event channel.
func (c *SSEClient) Events() <-chan events.Event {
	return c.eventCh
}
