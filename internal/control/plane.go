package control

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// ControlPlane provides workflow control capabilities.
type ControlPlane struct {
	mu         sync.RWMutex
	paused     atomic.Bool
	cancelled  atomic.Bool
	retryQueue chan core.TaskID
	pauseCh    chan struct{}
	resumeCh   chan struct{}
}

// New creates a new ControlPlane.
func New() *ControlPlane {
	return &ControlPlane{
		retryQueue: make(chan core.TaskID, 100),
		pauseCh:    make(chan struct{}),
		resumeCh:   make(chan struct{}),
	}
}

// Pause pauses the workflow execution.
// Running tasks will complete, but no new tasks will start.
func (cp *ControlPlane) Pause() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if !cp.paused.Load() {
		cp.paused.Store(true)
		close(cp.pauseCh)
		cp.pauseCh = make(chan struct{})
	}
}

// Resume resumes a paused workflow.
func (cp *ControlPlane) Resume() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.paused.Load() {
		cp.paused.Store(false)
		close(cp.resumeCh)
		cp.resumeCh = make(chan struct{})
	}
}

// Cancel cancels the workflow execution.
func (cp *ControlPlane) Cancel() {
	cp.cancelled.Store(true)
}

// RetryTask queues a task for retry.
func (cp *ControlPlane) RetryTask(taskID core.TaskID) {
	select {
	case cp.retryQueue <- taskID:
	default:
		// Queue full, drop (shouldn't happen with reasonable buffer)
	}
}

// IsPaused returns true if the workflow is paused.
func (cp *ControlPlane) IsPaused() bool {
	return cp.paused.Load()
}

// IsCancelled returns true if the workflow is cancelled.
func (cp *ControlPlane) IsCancelled() bool {
	return cp.cancelled.Load()
}

// WaitIfPaused blocks until the workflow is resumed.
// Returns immediately if not paused or if cancelled.
func (cp *ControlPlane) WaitIfPaused(ctx context.Context) error {
	if !cp.paused.Load() {
		return nil
	}

	cp.mu.RLock()
	resumeCh := cp.resumeCh
	cp.mu.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-resumeCh:
		return nil
	}
}

// CheckCancelled returns an error if cancelled.
func (cp *ControlPlane) CheckCancelled() error {
	if cp.cancelled.Load() {
		return core.ErrState("CANCELLED", "workflow cancelled by user")
	}
	return nil
}

// GetRetryQueue returns the retry queue channel for the executor.
func (cp *ControlPlane) GetRetryQueue() <-chan core.TaskID {
	return cp.retryQueue
}

// PausedCh returns a channel that's closed when paused.
// Useful for select statements.
func (cp *ControlPlane) PausedCh() <-chan struct{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	if cp.paused.Load() {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return cp.pauseCh
}

// Status returns the current control status.
type Status struct {
	Paused    bool
	Cancelled bool
	Retries   int
}

func (cp *ControlPlane) Status() Status {
	return Status{
		Paused:    cp.paused.Load(),
		Cancelled: cp.cancelled.Load(),
		Retries:   len(cp.retryQueue),
	}
}
