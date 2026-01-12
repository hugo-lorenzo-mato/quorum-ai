package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts  int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
	JitterFactor float64 // 0.0 to 1.0
	Multiplier   float64 // Exponential factor
}

// DefaultRetryPolicy returns a default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		BaseDelay:    time.Second,
		MaxDelay:     30 * time.Second,
		JitterFactor: 0.2,
		Multiplier:   2.0,
	}
}

// RetryPolicyOption configures a retry policy.
type RetryPolicyOption func(*RetryPolicy)

// WithMaxAttempts sets the maximum number of attempts.
func WithMaxAttempts(n int) RetryPolicyOption {
	return func(p *RetryPolicy) {
		p.MaxAttempts = n
	}
}

// WithBaseDelay sets the initial delay.
func WithBaseDelay(d time.Duration) RetryPolicyOption {
	return func(p *RetryPolicy) {
		p.BaseDelay = d
	}
}

// WithMaxDelay sets the maximum delay.
func WithMaxDelay(d time.Duration) RetryPolicyOption {
	return func(p *RetryPolicy) {
		p.MaxDelay = d
	}
}

// WithJitter sets the jitter factor.
func WithJitter(factor float64) RetryPolicyOption {
	return func(p *RetryPolicy) {
		p.JitterFactor = factor
	}
}

// WithMultiplier sets the exponential multiplier.
func WithMultiplier(m float64) RetryPolicyOption {
	return func(p *RetryPolicy) {
		p.Multiplier = m
	}
}

// NewRetryPolicy creates a new retry policy.
func NewRetryPolicy(opts ...RetryPolicyOption) *RetryPolicy {
	p := DefaultRetryPolicy()
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// RetryableFunc is a function that can be retried.
type RetryableFunc func(ctx context.Context) error

// Execute runs the function with retry logic.
func (p *RetryPolicy) Execute(ctx context.Context, fn RetryableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !core.IsRetryable(err) {
			return err
		}

		// Don't wait after the last attempt
		if attempt == p.MaxAttempts {
			break
		}

		// Calculate delay with exponential backoff
		delay := p.CalculateDelay(attempt)

		// Wait with context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return &RetryExhaustedError{
		Attempts: p.MaxAttempts,
		LastErr:  lastErr,
	}
}

// CalculateDelay computes the delay for a given attempt.
func (p *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * multiplier^(attempt-1)
	delay := float64(p.BaseDelay) * math.Pow(p.Multiplier, float64(attempt-1))

	// Apply maximum delay cap
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	// Apply jitter
	if p.JitterFactor > 0 {
		delay = addJitter(delay, p.JitterFactor)
	}

	return time.Duration(delay)
}

// CalculateDelayNoJitter computes the delay without jitter (for testing).
func (p *RetryPolicy) CalculateDelayNoJitter(attempt int) time.Duration {
	delay := float64(p.BaseDelay) * math.Pow(p.Multiplier, float64(attempt-1))
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}
	return time.Duration(delay)
}

// addJitter adds random jitter to a delay.
func addJitter(delay float64, factor float64) float64 {
	jitter := delay * factor
	// Random value between -jitter and +jitter
	randomJitter := (rand.Float64()*2 - 1) * jitter
	return delay + randomJitter
}

// RetryExhaustedError indicates all retry attempts failed.
type RetryExhaustedError struct {
	Attempts int
	LastErr  error
}

func (e *RetryExhaustedError) Error() string {
	return fmt.Sprintf("retry exhausted after %d attempts: %v", e.Attempts, e.LastErr)
}

func (e *RetryExhaustedError) Unwrap() error {
	return e.LastErr
}

// IsRetryExhausted checks if an error is a RetryExhaustedError.
func IsRetryExhausted(err error) bool {
	_, ok := err.(*RetryExhaustedError)
	return ok
}

// RetryNotifyFunc is called on each retry.
type RetryNotifyFunc func(attempt int, err error, delay time.Duration)

// ExecuteWithNotify runs with retry and notifications.
func (p *RetryPolicy) ExecuteWithNotify(ctx context.Context, fn RetryableFunc, notify RetryNotifyFunc) error {
	var lastErr error

	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if !core.IsRetryable(err) {
			return err
		}

		if attempt == p.MaxAttempts {
			break
		}

		delay := p.CalculateDelay(attempt)

		// Notify about retry
		if notify != nil {
			notify(attempt, err, delay)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return &RetryExhaustedError{
		Attempts: p.MaxAttempts,
		LastErr:  lastErr,
	}
}

// RateLimitRetryPolicy is specialized for rate limit errors.
func RateLimitRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  5,
		BaseDelay:    10 * time.Second,
		MaxDelay:     2 * time.Minute,
		JitterFactor: 0.3,
		Multiplier:   2.0,
	}
}

// TimeoutRetryPolicy is specialized for timeout errors.
func TimeoutRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		BaseDelay:    5 * time.Second,
		MaxDelay:     30 * time.Second,
		JitterFactor: 0.1,
		Multiplier:   1.5,
	}
}

// NetworkRetryPolicy is specialized for network errors.
func NetworkRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  5,
		BaseDelay:    2 * time.Second,
		MaxDelay:     time.Minute,
		JitterFactor: 0.25,
		Multiplier:   2.0,
	}
}
