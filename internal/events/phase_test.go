package events

import (
	"testing"
	"time"
)

func TestNewPhaseStartedEvent(t *testing.T) {
	t.Parallel()
	e := NewPhaseStartedEvent("wf-1", "proj-1", "analyze")
	if e.EventType() != TypePhaseStarted {
		t.Errorf("expected type %q, got %q", TypePhaseStarted, e.EventType())
	}
	if e.WorkflowID() != "wf-1" {
		t.Errorf("expected workflow_id 'wf-1', got %q", e.WorkflowID())
	}
	if e.Phase != "analyze" {
		t.Errorf("expected phase 'analyze', got %q", e.Phase)
	}
}

func TestNewPhaseCompletedEvent(t *testing.T) {
	t.Parallel()
	e := NewPhaseCompletedEvent("wf-2", "proj-1", "plan", 5*time.Second)
	if e.EventType() != TypePhaseCompleted {
		t.Errorf("expected type %q, got %q", TypePhaseCompleted, e.EventType())
	}
	if e.Phase != "plan" {
		t.Errorf("expected phase 'plan', got %q", e.Phase)
	}
	if e.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", e.Duration)
	}
}

func TestNewPhaseAwaitingReviewEvent(t *testing.T) {
	t.Parallel()
	e := NewPhaseAwaitingReviewEvent("wf-3", "proj-1", "analyze")
	if e.EventType() != TypePhaseAwaitingReview {
		t.Errorf("expected type %q, got %q", TypePhaseAwaitingReview, e.EventType())
	}
	if e.WorkflowID() != "wf-3" {
		t.Errorf("expected workflow_id 'wf-3', got %q", e.WorkflowID())
	}
	if e.Phase != "analyze" {
		t.Errorf("expected phase 'analyze', got %q", e.Phase)
	}
}

func TestNewPhaseReviewApprovedEvent(t *testing.T) {
	t.Parallel()
	e := NewPhaseReviewApprovedEvent("wf-4", "proj-1", "plan")
	if e.EventType() != TypePhaseReviewApproved {
		t.Errorf("expected type %q, got %q", TypePhaseReviewApproved, e.EventType())
	}
	if e.WorkflowID() != "wf-4" {
		t.Errorf("expected workflow_id 'wf-4', got %q", e.WorkflowID())
	}
	if e.Phase != "plan" {
		t.Errorf("expected phase 'plan', got %q", e.Phase)
	}
}

func TestNewPhaseReviewRejectedEvent(t *testing.T) {
	t.Parallel()
	e := NewPhaseReviewRejectedEvent("wf-5", "proj-1", "analyze", "needs more detail")
	if e.EventType() != TypePhaseReviewRejected {
		t.Errorf("expected type %q, got %q", TypePhaseReviewRejected, e.EventType())
	}
	if e.WorkflowID() != "wf-5" {
		t.Errorf("expected workflow_id 'wf-5', got %q", e.WorkflowID())
	}
	if e.Phase != "analyze" {
		t.Errorf("expected phase 'analyze', got %q", e.Phase)
	}
	if e.Feedback != "needs more detail" {
		t.Errorf("expected feedback 'needs more detail', got %q", e.Feedback)
	}
}

func TestPhaseEventConstants(t *testing.T) {
	t.Parallel()
	if TypePhaseStarted != "phase_started" {
		t.Errorf("wrong constant: %q", TypePhaseStarted)
	}
	if TypePhaseCompleted != "phase_completed" {
		t.Errorf("wrong constant: %q", TypePhaseCompleted)
	}
	if TypePhaseAwaitingReview != "phase_awaiting_review" {
		t.Errorf("wrong constant: %q", TypePhaseAwaitingReview)
	}
	if TypePhaseReviewApproved != "phase_review_approved" {
		t.Errorf("wrong constant: %q", TypePhaseReviewApproved)
	}
	if TypePhaseReviewRejected != "phase_review_rejected" {
		t.Errorf("wrong constant: %q", TypePhaseReviewRejected)
	}
}
