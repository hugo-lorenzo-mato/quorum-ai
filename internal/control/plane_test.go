package control

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestControlPlane_PauseResume(t *testing.T) {
	cp := New()

	if cp.IsPaused() {
		t.Error("Should not be paused initially")
	}

	cp.Pause()
	if !cp.IsPaused() {
		t.Error("Should be paused after Pause()")
	}

	cp.Resume()
	if cp.IsPaused() {
		t.Error("Should not be paused after Resume()")
	}
}

func TestControlPlane_WaitIfPaused(t *testing.T) {
	cp := New()
	ctx := context.Background()

	// Should return immediately if not paused
	start := time.Now()
	err := cp.WaitIfPaused(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if time.Since(start) > 10*time.Millisecond {
		t.Error("Should return immediately when not paused")
	}

	// Should block when paused
	cp.Pause()
	done := make(chan struct{})
	go func() {
		cp.WaitIfPaused(ctx)
		close(done)
	}()

	// Should still be waiting
	select {
	case <-done:
		t.Error("Should be waiting")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	// Resume should unblock
	cp.Resume()
	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Should have resumed")
	}
}

func TestControlPlane_WaitIfPaused_CancelUnblocks(t *testing.T) {
	cp := New()
	cp.Pause()

	done := make(chan error, 1)
	go func() {
		done <- cp.WaitIfPaused(context.Background())
	}()

	// Should still be waiting
	select {
	case err := <-done:
		t.Fatalf("expected WaitIfPaused to block, got err=%v", err)
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	cp.Cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after cancel, got nil")
		}
		if !strings.Contains(err.Error(), "CANCELLED") {
			t.Fatalf("expected CANCELLED error, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected WaitIfPaused to unblock after cancel")
	}
}

func TestControlPlane_Cancel(t *testing.T) {
	cp := New()

	if cp.IsCancelled() {
		t.Error("Should not be cancelled initially")
	}

	if err := cp.CheckCancelled(); err != nil {
		t.Errorf("Should not return error initially: %v", err)
	}

	cp.Cancel()

	if !cp.IsCancelled() {
		t.Error("Should be cancelled")
	}

	if err := cp.CheckCancelled(); err == nil {
		t.Error("Should return error after cancel")
	}
}

func TestControlPlane_RetryTask(t *testing.T) {
	cp := New()

	cp.RetryTask("task-1")
	cp.RetryTask("task-2")

	queue := cp.GetRetryQueue()

	select {
	case taskID := <-queue:
		if taskID != "task-1" {
			t.Errorf("Expected task-1, got %s", taskID)
		}
	default:
		t.Error("Expected task in queue")
	}
}

func TestControlPlane_Status(t *testing.T) {
	cp := New()

	status := cp.Status()
	if status.Paused {
		t.Error("Status.Paused should be false initially")
	}
	if status.Cancelled {
		t.Error("Status.Cancelled should be false initially")
	}
	if status.Retries != 0 {
		t.Errorf("Status.Retries should be 0, got %d", status.Retries)
	}

	cp.Pause()
	cp.RetryTask("task-1")

	status = cp.Status()
	if !status.Paused {
		t.Error("Status.Paused should be true after pause")
	}
	if status.Retries != 1 {
		t.Errorf("Status.Retries should be 1, got %d", status.Retries)
	}
}

func TestControlPlane_PausedCh(t *testing.T) {
	cp := New()

	// When not paused, PausedCh should not be closed
	ch := cp.PausedCh()
	select {
	case <-ch:
		t.Error("Channel should not be closed when not paused")
	default:
		// Expected
	}

	// When paused, PausedCh should return a closed channel
	cp.Pause()
	ch = cp.PausedCh()
	select {
	case <-ch:
		// Expected - channel is closed
	default:
		t.Error("Channel should be closed when paused")
	}
}

func TestControlPlane_WaitIfPaused_ContextCancellation(t *testing.T) {
	cp := New()
	cp.Pause()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := cp.WaitIfPaused(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestControlPlane_DoublePause(t *testing.T) {
	cp := New()

	// Double pause should not cause issues
	cp.Pause()
	cp.Pause()
	if !cp.IsPaused() {
		t.Error("Should still be paused after double pause")
	}
}

func TestControlPlane_DoubleResume(t *testing.T) {
	cp := New()

	cp.Pause()
	cp.Resume()
	cp.Resume() // Should be a no-op
	if cp.IsPaused() {
		t.Error("Should not be paused after double resume")
	}
}

func TestControlPlane_RetryQueueFull(t *testing.T) {
	cp := New()

	// Fill the queue (buffer size is 100)
	for i := 0; i < 100; i++ {
		cp.RetryTask("task")
	}

	// This should not block or panic, just be dropped
	cp.RetryTask("overflow")

	// Verify queue has 100 items
	status := cp.Status()
	if status.Retries != 100 {
		t.Errorf("Expected 100 retries in queue, got %d", status.Retries)
	}
}
