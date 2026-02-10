package api

import (
	"log/slog"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// eventBus helper
func newTestEventBus() *events.EventBus {
	return events.New(10)
}

func TestNewWorkflowExecutor(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	eventBus := newTestEventBus()

	executor := NewWorkflowExecutor(nil, nil, eventBus, logger, nil)
	if executor == nil {
		t.Fatal("NewWorkflowExecutor() returned nil")
	}
	if executor.executionTimeout != 4*time.Hour {
		t.Errorf("executionTimeout = %v, want 4h", executor.executionTimeout)
	}
}

func TestWorkflowExecutor_WithExecutionTimeout(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	result := executor.WithExecutionTimeout(30 * time.Minute)

	if result != executor {
		t.Error("WithExecutionTimeout should return same executor")
	}
	if executor.executionTimeout != 30*time.Minute {
		t.Errorf("executionTimeout = %v, want 30m", executor.executionTimeout)
	}
}

func TestWorkflowExecutor_IsRunning_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	if executor.IsRunning("wf-123") {
		t.Error("IsRunning should return false when tracker is nil")
	}
}

func TestWorkflowExecutor_StateManager(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	if executor.StateManager() != nil {
		t.Error("StateManager() should return nil when not set")
	}
}

func TestWorkflowExecutor_EventBus(t *testing.T) {
	t.Parallel()

	eventBus := newTestEventBus()
	executor := NewWorkflowExecutor(nil, nil, eventBus, slog.Default(), nil)

	if executor.EventBus() != eventBus {
		t.Error("EventBus() should return configured event bus")
	}
}

func TestWorkflowExecutor_Cancel_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	err := executor.Cancel("wf-123")
	if err == nil {
		t.Error("Cancel should return error when tracker is nil")
	}
}

func TestWorkflowExecutor_Pause_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	err := executor.Pause("wf-123")
	if err == nil {
		t.Error("Pause should return error when tracker is nil")
	}
}

func TestWorkflowExecutor_GetControlPlane_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	cp, ok := executor.GetControlPlane("wf-123")
	if ok {
		t.Error("GetControlPlane should return false when tracker is nil")
	}
	if cp != nil {
		t.Error("GetControlPlane should return nil when tracker is nil")
	}
}

func TestWorkflowExecutor_Run_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	err := executor.Run(nil, "wf-123")
	if err == nil {
		t.Error("Run should return error when tracker is nil")
	}
}

func TestWorkflowExecutor_Resume_NilTracker(t *testing.T) {
	t.Parallel()

	executor := NewWorkflowExecutor(nil, nil, nil, slog.Default(), nil)
	err := executor.Resume(nil, "wf-123")
	if err == nil {
		t.Error("Resume should return error when tracker is nil")
	}
}
