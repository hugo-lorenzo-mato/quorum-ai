package api

import (
	"net/http/httptest"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

func TestSendEventToClient_PhaseAwaitingReview(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewPhaseAwaitingReviewEvent("wf-1", "", "analyze")

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "phase_awaiting_review" {
		t.Errorf("expected event type 'phase_awaiting_review', got %q", eventType)
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

func TestSendEventToClient_PhaseReviewApproved(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewPhaseReviewApprovedEvent("wf-2", "", "plan")

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "phase_review_approved" {
		t.Errorf("expected event type 'phase_review_approved', got %q", eventType)
	}
	if payload["phase"] != "plan" {
		t.Errorf("expected phase 'plan', got %v", payload["phase"])
	}
	if payload["workflow_id"] != "wf-2" {
		t.Errorf("expected workflow_id 'wf-2', got %v", payload["workflow_id"])
	}
}

func TestSendEventToClient_PhaseReviewRejected(t *testing.T) {
	t.Parallel()
	bus := events.New(10)
	s := newTestServer(bus)

	rec := httptest.NewRecorder()
	event := events.NewPhaseReviewRejectedEvent("wf-3", "", "analyze", "needs more work")

	s.sendEventToClient(rec, mockFlusher{}, event)

	eventType, payload := parseSSEPayload(t, rec.Body.String())
	if eventType != "phase_review_rejected" {
		t.Errorf("expected event type 'phase_review_rejected', got %q", eventType)
	}
	if payload["phase"] != "analyze" {
		t.Errorf("expected phase 'analyze', got %v", payload["phase"])
	}
	if payload["feedback"] != "needs more work" {
		t.Errorf("expected feedback 'needs more work', got %v", payload["feedback"])
	}
	if payload["workflow_id"] != "wf-3" {
		t.Errorf("expected workflow_id 'wf-3', got %v", payload["workflow_id"])
	}
}
