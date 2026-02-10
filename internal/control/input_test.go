package control

import (
	"context"
	"testing"
	"time"
)

func TestControlPlane_ProvideUserInput(t *testing.T) {
	cp := New()

	// No pending request
	err := cp.ProvideUserInput("nonexistent", "answer")
	if err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestControlPlane_CancelUserInput(t *testing.T) {
	cp := New()

	// No pending request
	err := cp.CancelUserInput("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestControlPlane_HasPendingInput(t *testing.T) {
	cp := New()
	if cp.HasPendingInput() {
		t.Error("should have no pending input initially")
	}
}

func TestControlPlane_InputRequestCh(t *testing.T) {
	cp := New()
	ch := cp.InputRequestCh()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestControlPlane_RequestAndProvideInput(t *testing.T) {
	cp := New()

	// Start a request in a goroutine
	done := make(chan InputResponse, 1)
	go func() {
		resp, _ := cp.RequestUserInput(context.Background(), InputRequest{
			ID:     "req-1",
			Prompt: "Continue?",
		})
		done <- resp
	}()

	// Wait for the request to appear on the channel
	select {
	case req := <-cp.InputRequestCh():
		if req.ID != "req-1" {
			t.Errorf("got request ID %q", req.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request")
	}

	// Provide the answer
	err := cp.ProvideUserInput("req-1", "yes")
	if err != nil {
		t.Fatalf("ProvideUserInput failed: %v", err)
	}

	select {
	case resp := <-done:
		if resp.Input != "yes" {
			t.Errorf("got input %q", resp.Input)
		}
		if resp.Cancelled {
			t.Error("should not be cancelled")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
}

func TestControlPlane_RequestInput_ContextCancelled(t *testing.T) {
	cp := New()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := cp.RequestUserInput(ctx, InputRequest{
		ID:     "req-timeout",
		Prompt: "Will timeout",
	})
	// Should error due to context cancellation (or timeout sending request)
	if err == nil {
		t.Error("expected error from context cancellation")
	}
}

func TestControlPlane_WaitIfPaused_AlreadyCancelled(t *testing.T) {
	cp := New()
	cp.Cancel()

	err := cp.WaitIfPaused(context.Background())
	if err == nil {
		t.Error("expected error when already cancelled")
	}
}
