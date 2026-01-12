package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestRetryPolicy_Execute_Success(t *testing.T) {
	policy := NewRetryPolicy(WithMaxAttempts(3))
	ctx := context.Background()

	callCount := 0
	err := policy.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}
}

func TestRetryPolicy_Execute_SuccessAfterRetry(t *testing.T) {
	policy := NewRetryPolicy(
		WithMaxAttempts(3),
		WithBaseDelay(1*time.Millisecond),
	)
	ctx := context.Background()

	callCount := 0
	err := policy.Execute(ctx, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return core.ErrRateLimit("rate limited")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestRetryPolicy_Execute_NonRetryable(t *testing.T) {
	policy := NewRetryPolicy(WithMaxAttempts(3))
	ctx := context.Background()

	callCount := 0
	nonRetryableErr := core.ErrValidation("INVALID", "not retryable")

	err := policy.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return nonRetryableErr
	})

	if err == nil {
		t.Error("Execute() should return error")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not retry non-retryable errors)", callCount)
	}
}

func TestRetryPolicy_Execute_Exhausted(t *testing.T) {
	policy := NewRetryPolicy(
		WithMaxAttempts(3),
		WithBaseDelay(1*time.Millisecond),
	)
	ctx := context.Background()

	callCount := 0
	retryableErr := core.ErrTimeout("timeout")

	err := policy.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return retryableErr
	})

	if err == nil {
		t.Error("Execute() should return error")
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}

	var exhaustedErr *RetryExhaustedError
	if !errors.As(err, &exhaustedErr) {
		t.Error("error should be RetryExhaustedError")
	} else {
		if exhaustedErr.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", exhaustedErr.Attempts)
		}
	}
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := NewRetryPolicy(
		WithBaseDelay(1*time.Second),
		WithMaxDelay(30*time.Second),
		WithMultiplier(2.0),
		WithJitter(0), // Disable jitter for predictable testing
	)

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 30 * time.Second}, // Capped at max
		{7, 30 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		got := policy.CalculateDelayNoJitter(tt.attempt)
		if got != tt.want {
			t.Errorf("CalculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestRetryPolicy_Jitter(t *testing.T) {
	policy := NewRetryPolicy(
		WithBaseDelay(1*time.Second),
		WithJitter(0.2),
	)

	// Run multiple times to verify jitter produces different values
	delays := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		delay := policy.CalculateDelay(1)
		delays[delay] = true
	}

	// With 20% jitter on 1s base, we should get various values
	if len(delays) < 5 {
		t.Error("jitter should produce varied delays")
	}

	// All delays should be within ±20% of base
	baseDelay := float64(1 * time.Second)
	for delay := range delays {
		if float64(delay) < baseDelay*0.8 || float64(delay) > baseDelay*1.2 {
			t.Errorf("delay %v out of jitter range [0.8s, 1.2s]", delay)
		}
	}
}

func TestRetryPolicy_ContextCancellation(t *testing.T) {
	policy := NewRetryPolicy(
		WithMaxAttempts(5),
		WithBaseDelay(1*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	// Cancel after first call
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := policy.Execute(ctx, func(ctx context.Context) error {
		callCount++
		return core.ErrTimeout("timeout")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Execute() error = %v, want context.Canceled", err)
	}
}

func TestRetryPolicy_ExecuteWithNotify(t *testing.T) {
	policy := NewRetryPolicy(
		WithMaxAttempts(3),
		WithBaseDelay(1*time.Millisecond),
	)
	ctx := context.Background()

	notifications := make([]int, 0)
	notify := func(attempt int, err error, delay time.Duration) {
		notifications = append(notifications, attempt)
	}

	err := policy.ExecuteWithNotify(ctx, func(ctx context.Context) error {
		return core.ErrTimeout("timeout")
	}, notify)

	if err == nil {
		t.Error("ExecuteWithNotify() should return error")
	}

	// Should have 2 notifications (after attempt 1 and 2, not after 3)
	if len(notifications) != 2 {
		t.Errorf("notifications = %v, want 2 entries", notifications)
	}
	if len(notifications) >= 2 && notifications[0] != 1 {
		t.Errorf("first notification attempt = %d, want 1", notifications[0])
	}
	if len(notifications) >= 2 && notifications[1] != 2 {
		t.Errorf("second notification attempt = %d, want 2", notifications[1])
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", policy.MaxAttempts)
	}
	if policy.BaseDelay != 1*time.Second {
		t.Errorf("BaseDelay = %v, want 1s", policy.BaseDelay)
	}
	if policy.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", policy.MaxDelay)
	}
	if policy.JitterFactor != 0.2 {
		t.Errorf("JitterFactor = %v, want 0.2", policy.JitterFactor)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", policy.Multiplier)
	}
}

func TestRateLimitRetryPolicy(t *testing.T) {
	policy := RateLimitRetryPolicy()

	if policy.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", policy.MaxAttempts)
	}
	if policy.BaseDelay != 10*time.Second {
		t.Errorf("BaseDelay = %v, want 10s", policy.BaseDelay)
	}
}

func TestTimeoutRetryPolicy(t *testing.T) {
	policy := TimeoutRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", policy.MaxAttempts)
	}
	if policy.BaseDelay != 5*time.Second {
		t.Errorf("BaseDelay = %v, want 5s", policy.BaseDelay)
	}
}

func TestRetryExhaustedError(t *testing.T) {
	originalErr := core.ErrTimeout("timeout")
	exhaustedErr := &RetryExhaustedError{
		Attempts: 3,
		LastErr:  originalErr,
	}

	// Test Error() message
	msg := exhaustedErr.Error()
	if msg == "" {
		t.Error("Error() should return non-empty message")
	}

	// Test Unwrap()
	unwrapped := exhaustedErr.Unwrap()
	if unwrapped != originalErr {
		t.Error("Unwrap() should return the original error")
	}

	// Test IsRetryExhausted
	if !IsRetryExhausted(exhaustedErr) {
		t.Error("IsRetryExhausted should return true")
	}
	if IsRetryExhausted(originalErr) {
		t.Error("IsRetryExhausted should return false for non-exhausted error")
	}
}

func TestRetryPolicy_Options(t *testing.T) {
	policy := NewRetryPolicy(
		WithMaxAttempts(5),
		WithBaseDelay(2*time.Second),
		WithMaxDelay(1*time.Minute),
		WithJitter(0.3),
		WithMultiplier(3.0),
	)

	if policy.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", policy.MaxAttempts)
	}
	if policy.BaseDelay != 2*time.Second {
		t.Errorf("BaseDelay = %v, want 2s", policy.BaseDelay)
	}
	if policy.MaxDelay != 1*time.Minute {
		t.Errorf("MaxDelay = %v, want 1m", policy.MaxDelay)
	}
	if policy.JitterFactor != 0.3 {
		t.Errorf("JitterFactor = %v, want 0.3", policy.JitterFactor)
	}
	if policy.Multiplier != 3.0 {
		t.Errorf("Multiplier = %v, want 3.0", policy.Multiplier)
	}
}

func TestRetryPolicy_ImmediateContextCancel(t *testing.T) {
	policy := NewRetryPolicy(WithMaxAttempts(3))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := policy.Execute(ctx, func(ctx context.Context) error {
		return core.ErrTimeout("timeout")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Execute() error = %v, want context.Canceled", err)
	}
}
