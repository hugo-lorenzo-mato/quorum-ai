package api

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// mockFlusher wraps httptest.ResponseRecorder to satisfy http.Flusher.
type mockFlusher struct{}

func (mockFlusher) Flush() {}

func newTestServer(bus *events.EventBus) *Server {
	return &Server{
		logger:   slog.Default(),
		eventBus: bus,
	}
}

func parseSSEPayload(t *testing.T, body string) (eventType string, payload map[string]interface{}) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			raw := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(raw), &payload); err != nil {
				t.Fatalf("failed to unmarshal SSE data: %v", err)
			}
		}
	}
	return
}

func TestSendEventToClient_PhaseStarted(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewPhaseStartedEvent("wf-1", "", "analyze")

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "phase_started" {
		t.Errorf("expected event type 'phase_started', got %q", eventType)
	}
	if payload["phase"] != "analyze" {
		t.Errorf("expected phase 'analyze', got %v", payload["phase"])
	}
	if payload["workflow_id"] != "wf-1" {
		t.Errorf("expected workflow_id 'wf-1', got %v", payload["workflow_id"])
	}
	if payload["timestamp"] == nil {
		t.Error("expected timestamp to be present")
	}
}

func TestSendEventToClient_PhaseCompleted(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewPhaseCompletedEvent("wf-1", "", "plan", 5*time.Second)

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "phase_completed" {
		t.Errorf("expected event type 'phase_completed', got %q", eventType)
	}
	if payload["phase"] != "plan" {
		t.Errorf("expected phase 'plan', got %v", payload["phase"])
	}
	if payload["duration"] != "5s" {
		t.Errorf("expected duration '5s', got %v", payload["duration"])
	}
}

func TestSendEventToClient_LogEvent(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	fields := map[string]interface{}{"source": "executor"}
	event := events.NewLogEvent("wf-1", "", "error", "[executor] merge failed", fields)

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "log" {
		t.Errorf("expected event type 'log', got %q", eventType)
	}
	if payload["level"] != "error" {
		t.Errorf("expected level 'error', got %v", payload["level"])
	}
	if payload["message"] != "[executor] merge failed" {
		t.Errorf("expected message '[executor] merge failed', got %v", payload["message"])
	}
	if payload["fields"] == nil {
		t.Error("expected fields to be present")
	}
}

func TestSSEClient_CloseUnsubscribes(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	client := NewSSEClient(bus)
	ch := client.Events()
	client.Close()

	// After Close, the channel should be closed (bus unsubscribed)
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after Close()")
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for channel to close")
	}
}

func TestSSEClient_EventsReceivable(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	client := NewSSEClient(bus)
	defer client.Close()

	// Publish an event
	bus.Publish(events.NewPhaseStartedEvent("wf-1", "", "plan"))

	// Should receive it
	select {
	case evt := <-client.Events():
		if evt.EventType() != "phase_started" {
			t.Errorf("expected phase_started, got %s", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for event")
	}
}

func TestSendEventToClient_IssuesGenerationProgress(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewIssuesGenerationProgressEvent(events.IssuesGenerationProgressParams{
		WorkflowID:  "wf-gen-1",
		ProjectID:   "proj-1",
		Stage:       "generating",
		Current:     3,
		Total:       10,
		Message:     "Generating issue 3 of 10",
		FileName:    "003-fix-auth.md",
		Title:       "Fix auth flow",
		TaskID:      "task-42",
		IsMainIssue: true,
	})

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "issues_generation_progress" {
		t.Errorf("expected event type 'issues_generation_progress', got %q", eventType)
	}
	if payload["workflow_id"] != "wf-gen-1" {
		t.Errorf("expected workflow_id 'wf-gen-1', got %v", payload["workflow_id"])
	}
	if payload["stage"] != "generating" {
		t.Errorf("expected stage 'generating', got %v", payload["stage"])
	}
	// JSON numbers are decoded as float64
	if payload["current"] != float64(3) {
		t.Errorf("expected current 3, got %v", payload["current"])
	}
	if payload["total"] != float64(10) {
		t.Errorf("expected total 10, got %v", payload["total"])
	}
	if payload["message"] != "Generating issue 3 of 10" {
		t.Errorf("expected message 'Generating issue 3 of 10', got %v", payload["message"])
	}
	if payload["file_name"] != "003-fix-auth.md" {
		t.Errorf("expected file_name '003-fix-auth.md', got %v", payload["file_name"])
	}
	if payload["title"] != "Fix auth flow" {
		t.Errorf("expected title 'Fix auth flow', got %v", payload["title"])
	}
	if payload["task_id"] != "task-42" {
		t.Errorf("expected task_id 'task-42', got %v", payload["task_id"])
	}
	if payload["is_main_issue"] != true {
		t.Errorf("expected is_main_issue true, got %v", payload["is_main_issue"])
	}
	if payload["timestamp"] == nil {
		t.Error("expected timestamp to be present")
	}
}

func TestSendEventToClient_IssuesPublishingProgress(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewIssuesPublishingProgressEvent(events.IssuesPublishingProgressParams{
		WorkflowID:  "wf-pub-1",
		ProjectID:   "proj-2",
		Stage:       "publishing",
		Current:     2,
		Total:       5,
		Message:     "Publishing issue 2 of 5",
		Title:       "Add CI pipeline",
		TaskID:      "task-99",
		IsMainIssue: false,
		IssueNumber: 42,
		DryRun:      true,
	})

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "issues_publishing_progress" {
		t.Errorf("expected event type 'issues_publishing_progress', got %q", eventType)
	}
	if payload["workflow_id"] != "wf-pub-1" {
		t.Errorf("expected workflow_id 'wf-pub-1', got %v", payload["workflow_id"])
	}
	if payload["stage"] != "publishing" {
		t.Errorf("expected stage 'publishing', got %v", payload["stage"])
	}
	if payload["current"] != float64(2) {
		t.Errorf("expected current 2, got %v", payload["current"])
	}
	if payload["total"] != float64(5) {
		t.Errorf("expected total 5, got %v", payload["total"])
	}
	if payload["message"] != "Publishing issue 2 of 5" {
		t.Errorf("expected message 'Publishing issue 2 of 5', got %v", payload["message"])
	}
	if payload["title"] != "Add CI pipeline" {
		t.Errorf("expected title 'Add CI pipeline', got %v", payload["title"])
	}
	if payload["task_id"] != "task-99" {
		t.Errorf("expected task_id 'task-99', got %v", payload["task_id"])
	}
	if payload["is_main_issue"] != false {
		t.Errorf("expected is_main_issue false, got %v", payload["is_main_issue"])
	}
	if payload["issue_number"] != float64(42) {
		t.Errorf("expected issue_number 42, got %v", payload["issue_number"])
	}
	if payload["dry_run"] != true {
		t.Errorf("expected dry_run true, got %v", payload["dry_run"])
	}
	if payload["timestamp"] == nil {
		t.Error("expected timestamp to be present")
	}
}
