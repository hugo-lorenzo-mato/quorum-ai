package control

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// InputRequest represents a request for user input.
type InputRequest struct {
	ID      string        `json:"id"`
	Prompt  string        `json:"prompt"`
	Context string        `json:"context,omitempty"`
	Options []string      `json:"options,omitempty"`
	Timeout time.Duration `json:"timeout,omitempty"`
}

// InputResponse represents the user's response to an input request.
type InputResponse struct {
	RequestID string `json:"request_id"`
	Input     string `json:"input"`
	Cancelled bool   `json:"cancelled"`
	Error     error  `json:"-"`
}

// ControlPlane provides workflow control capabilities.
type ControlPlane struct {
	mu         sync.RWMutex
	paused     atomic.Bool
	cancelled  atomic.Bool
	retryQueue chan core.TaskID
	pauseCh    chan struct{}
	resumeCh   chan struct{}

	// Human-in-the-loop support
	inputMu        sync.RWMutex
	inputRequestCh chan InputRequest
	pendingInputs  map[string]chan InputResponse
}

// New creates a new ControlPlane.
func New() *ControlPlane {
	return &ControlPlane{
		retryQueue:     make(chan core.TaskID, 100),
		pauseCh:        make(chan struct{}),
		resumeCh:       make(chan struct{}),
		inputRequestCh: make(chan InputRequest, 10),
		pendingInputs:  make(map[string]chan InputResponse),
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

// RequestUserInput blocks until the user provides input.
// This follows the same pattern as WaitIfPaused - blocking until signal.
func (cp *ControlPlane) RequestUserInput(ctx context.Context, req InputRequest) (InputResponse, error) {
	// Create response channel for this request
	responseCh := make(chan InputResponse, 1)

	cp.inputMu.Lock()
	cp.pendingInputs[req.ID] = responseCh
	cp.inputMu.Unlock()

	// Cleanup on exit
	defer func() {
		cp.inputMu.Lock()
		delete(cp.pendingInputs, req.ID)
		cp.inputMu.Unlock()
	}()

	// Send request to TUI (non-blocking with timeout)
	select {
	case cp.inputRequestCh <- req:
	case <-ctx.Done():
		return InputResponse{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return InputResponse{}, fmt.Errorf("timeout sending input request")
	}

	// Wait for response
	var timeoutCh <-chan time.Time
	if req.Timeout > 0 {
		timeoutCh = time.After(req.Timeout)
	}

	select {
	case <-ctx.Done():
		return InputResponse{RequestID: req.ID, Cancelled: true}, ctx.Err()
	case <-timeoutCh:
		return InputResponse{RequestID: req.ID, Cancelled: true}, fmt.Errorf("user input timeout")
	case resp := <-responseCh:
		return resp, resp.Error
	}
}

// ProvideUserInput delivers the user's response to a pending input request.
// Called by the TUI when the user submits input.
func (cp *ControlPlane) ProvideUserInput(requestID, input string) error {
	cp.inputMu.RLock()
	responseCh, ok := cp.pendingInputs[requestID]
	cp.inputMu.RUnlock()

	if !ok {
		return fmt.Errorf("no pending input request with ID: %s", requestID)
	}

	select {
	case responseCh <- InputResponse{RequestID: requestID, Input: input}:
		return nil
	default:
		return fmt.Errorf("response channel full or closed")
	}
}

// CancelUserInput cancels a pending input request.
func (cp *ControlPlane) CancelUserInput(requestID string) error {
	cp.inputMu.RLock()
	responseCh, ok := cp.pendingInputs[requestID]
	cp.inputMu.RUnlock()

	if !ok {
		return fmt.Errorf("no pending input request with ID: %s", requestID)
	}

	select {
	case responseCh <- InputResponse{RequestID: requestID, Cancelled: true}:
		return nil
	default:
		return fmt.Errorf("response channel full or closed")
	}
}

// InputRequestCh returns the channel for receiving input requests.
// The TUI should listen on this channel and display prompts to the user.
func (cp *ControlPlane) InputRequestCh() <-chan InputRequest {
	return cp.inputRequestCh
}

// HasPendingInput returns true if there are pending input requests.
func (cp *ControlPlane) HasPendingInput() bool {
	cp.inputMu.RLock()
	defer cp.inputMu.RUnlock()
	return len(cp.pendingInputs) > 0
}
