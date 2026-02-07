package api

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestUnifiedTracker_CancelCancelsExecContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := &UnifiedTracker{
		logger:         logger,
		handles:        make(map[core.WorkflowID]*ExecutionHandle),
		confirmTimeout: 5 * time.Second,
	}

	id := core.WorkflowID("wf-1")
	handle := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}

	called := make(chan struct{})
	handle.SetExecCancel(func() { close(called) })

	tracker.handles[id] = handle

	if err := tracker.Cancel(id); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	select {
	case <-called:
		// Expected
	default:
		t.Fatal("expected exec cancel to be called")
	}

	if !handle.ControlPlane.IsCancelled() {
		t.Fatal("expected control plane to be cancelled")
	}
}

func TestUnifiedTracker_CancelBeforeSetExecCancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tracker := &UnifiedTracker{
		logger:         logger,
		handles:        make(map[core.WorkflowID]*ExecutionHandle),
		confirmTimeout: 5 * time.Second,
	}

	id := core.WorkflowID("wf-2")
	handle := &ExecutionHandle{
		WorkflowID:   id,
		ControlPlane: control.New(),
		confirmCh:    make(chan struct{}),
		errorCh:      make(chan error, 1),
		doneCh:       make(chan struct{}),
	}
	tracker.handles[id] = handle

	// Cancel before the exec cancel func is known.
	if err := tracker.Cancel(id); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}

	called := make(chan struct{})
	handle.SetExecCancel(func() { close(called) })

	select {
	case <-called:
		// Expected: SetExecCancel should immediately cancel if already requested.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected SetExecCancel to immediately cancel when already cancelled")
	}
}
