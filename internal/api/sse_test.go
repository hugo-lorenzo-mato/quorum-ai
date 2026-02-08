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
