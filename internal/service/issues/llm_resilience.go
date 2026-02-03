package issues

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

// ErrCircuitOpen indicates the circuit breaker is open and requests are blocked.
var ErrCircuitOpen = errors.New("circuit breaker is open: LLM service unavailable")

// ErrRetryExhausted indicates all retry attempts have been exhausted.
var ErrRetryExhausted = errors.New("retry attempts exhausted")

// LLMResilienceConfig configures the resilience behavior for LLM calls.
type LLMResilienceConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `yaml:"max_retries"`

	// InitialBackoff is the initial delay before first retry.
	InitialBackoff time.Duration `yaml:"initial_backoff"`

	// MaxBackoff is the maximum delay between retries.
	MaxBackoff time.Duration `yaml:"max_backoff"`

	// BackoffMultiplier is the exponential backoff multiplier.
	BackoffMultiplier float64 `yaml:"backoff_multiplier"`

	// FailureThreshold is the number of consecutive failures to open the circuit.
	FailureThreshold int `yaml:"failure_threshold"`

	// ResetTimeout is how long the circuit stays open before trying again.
	ResetTimeout time.Duration `yaml:"reset_timeout"`

	// Enabled indicates if resilience features are enabled.
	Enabled bool `yaml:"enabled"`
}

// DefaultLLMResilienceConfig returns default resilience configuration.
func DefaultLLMResilienceConfig() LLMResilienceConfig {
	return LLMResilienceConfig{
		MaxRetries:        3,
		InitialBackoff:    time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		FailureThreshold:  3,
		ResetTimeout:      30 * time.Second,
		Enabled:           true,
	}
}

// LLMMetrics tracks LLM execution metrics.
type LLMMetrics struct {
	TotalCalls      atomic.Int64
	SuccessfulCalls atomic.Int64
	FailedCalls     atomic.Int64
	RetryCount      atomic.Int64
	CircuitOpens    atomic.Int64
	TotalLatencyMs  atomic.Int64
}

// GetAverageLatencyMs returns the average latency in milliseconds.
func (m *LLMMetrics) GetAverageLatencyMs() float64 {
	total := m.TotalCalls.Load()
	if total == 0 {
		return 0
	}
	return float64(m.TotalLatencyMs.Load()) / float64(total)
}

// GetSuccessRate returns the success rate as a percentage.
func (m *LLMMetrics) GetSuccessRate() float64 {
	total := m.TotalCalls.Load()
	if total == 0 {
		return 0
	}
	return float64(m.SuccessfulCalls.Load()) / float64(total) * 100
}

// LLMCircuitBreaker implements a circuit breaker specifically for LLM operations.
// Unlike the generic circuit breaker, this one auto-resets after a timeout.
type LLMCircuitBreaker struct {
	mu                  sync.RWMutex
	threshold           int
	resetTimeout        time.Duration
	consecutiveFailures int
	lastFailureAt       time.Time
	state               circuitState
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// NewLLMCircuitBreaker creates a new circuit breaker for LLM operations.
func NewLLMCircuitBreaker(threshold int, resetTimeout time.Duration) *LLMCircuitBreaker {
	if threshold <= 0 {
		threshold = 3
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &LLMCircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		state:        circuitClosed,
	}
}

// IsOpen returns true if the circuit is open (blocking requests).
func (cb *LLMCircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == circuitClosed {
		return false
	}

	// Check if enough time has passed to try again (half-open)
	if cb.state == circuitOpen && time.Since(cb.lastFailureAt) >= cb.resetTimeout {
		return false // Allow one request through
	}

	return cb.state == circuitOpen
}

// AllowRequest checks if a request should be allowed.
// If the circuit is open but reset timeout has passed, transitions to half-open.
func (cb *LLMCircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(cb.lastFailureAt) >= cb.resetTimeout {
			cb.state = circuitHalfOpen
			slog.Info("circuit breaker transitioning to half-open")
			return true
		}
		return false
	case circuitHalfOpen:
		return true // Allow test request
	}
	return false
}

// RecordSuccess records a successful LLM call.
func (cb *LLMCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	if cb.state == circuitHalfOpen {
		cb.state = circuitClosed
		slog.Info("circuit breaker closed after successful request")
	}
}

// RecordFailure records a failed LLM call.
// Returns true if the circuit just opened.
func (cb *LLMCircuitBreaker) RecordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++
	cb.lastFailureAt = time.Now()

	if cb.state == circuitHalfOpen {
		// Failed during half-open, go back to open
		cb.state = circuitOpen
		slog.Warn("circuit breaker re-opened after half-open failure")
		return true
	}

	if cb.consecutiveFailures >= cb.threshold && cb.state == circuitClosed {
		cb.state = circuitOpen
		slog.Warn("circuit breaker opened after threshold failures",
			"failures", cb.consecutiveFailures,
			"threshold", cb.threshold)
		return true
	}

	return false
}

// Reset resets the circuit breaker to closed state.
func (cb *LLMCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFailures = 0
	cb.state = circuitClosed
	cb.lastFailureAt = time.Time{}
}

// GetState returns current state information.
func (cb *LLMCircuitBreaker) GetState() (failures int, isOpen bool, lastFailure time.Time) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.consecutiveFailures, cb.state != circuitClosed, cb.lastFailureAt
}

// ResilientLLMExecutor wraps an LLM agent with retry and circuit breaker logic.
type ResilientLLMExecutor struct {
	agent          core.Agent
	config         LLMResilienceConfig
	retryPolicy    *service.RetryPolicy
	circuitBreaker *LLMCircuitBreaker
	metrics        *LLMMetrics
}

// NewResilientLLMExecutor creates a new resilient executor for an LLM agent.
func NewResilientLLMExecutor(agent core.Agent, cfg LLMResilienceConfig) *ResilientLLMExecutor {
	return &ResilientLLMExecutor{
		agent:  agent,
		config: cfg,
		retryPolicy: service.NewRetryPolicy(
			service.WithMaxAttempts(cfg.MaxRetries),
			service.WithBaseDelay(cfg.InitialBackoff),
			service.WithMaxDelay(cfg.MaxBackoff),
			service.WithMultiplier(cfg.BackoffMultiplier),
		),
		circuitBreaker: NewLLMCircuitBreaker(cfg.FailureThreshold, cfg.ResetTimeout),
		metrics:        &LLMMetrics{},
	}
}

// Execute runs the LLM agent with resilience features.
func (r *ResilientLLMExecutor) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	if !r.config.Enabled {
		// Resilience disabled, call directly
		return r.agent.Execute(ctx, opts)
	}

	// Check circuit breaker
	if !r.circuitBreaker.AllowRequest() {
		r.metrics.CircuitOpens.Add(1)
		return nil, ErrCircuitOpen
	}

	r.metrics.TotalCalls.Add(1)
	startTime := time.Now()

	var result *core.ExecuteResult
	var lastErr error

	// Execute with retry
	err := r.retryPolicy.ExecuteWithNotify(ctx, func(ctx context.Context) error {
		var execErr error
		result, execErr = r.agent.Execute(ctx, opts)
		if execErr != nil {
			lastErr = execErr
			// Check if error is retryable
			if isTransientError(execErr) {
				r.metrics.RetryCount.Add(1)
				return execErr // Will retry
			}
			// Non-retryable error
			return &nonRetryableError{err: execErr}
		}
		return nil
	}, func(attempt int, err error, delay time.Duration) {
		slog.Info("retrying LLM execution",
			"attempt", attempt,
			"error", err.Error(),
			"delay", delay.String())
	})

	latencyMs := time.Since(startTime).Milliseconds()
	r.metrics.TotalLatencyMs.Add(latencyMs)

	// Handle result
	if err != nil {
		r.metrics.FailedCalls.Add(1)
		r.circuitBreaker.RecordFailure()

		// Unwrap non-retryable error
		var nre *nonRetryableError
		if errors.As(err, &nre) {
			return nil, nre.err
		}

		// Check if retry exhausted
		if service.IsRetryExhausted(err) {
			return nil, fmt.Errorf("%w: %v", ErrRetryExhausted, lastErr)
		}

		return nil, err
	}

	r.metrics.SuccessfulCalls.Add(1)
	r.circuitBreaker.RecordSuccess()

	return result, nil
}

// GetMetrics returns the current metrics.
func (r *ResilientLLMExecutor) GetMetrics() LLMMetrics {
	return LLMMetrics{
		TotalCalls:      atomic.Int64{},
		SuccessfulCalls: atomic.Int64{},
		FailedCalls:     atomic.Int64{},
		RetryCount:      atomic.Int64{},
		CircuitOpens:    atomic.Int64{},
		TotalLatencyMs:  atomic.Int64{},
	}
}

// ResetCircuitBreaker manually resets the circuit breaker.
func (r *ResilientLLMExecutor) ResetCircuitBreaker() {
	r.circuitBreaker.Reset()
}

// IsCircuitOpen returns true if the circuit breaker is open.
func (r *ResilientLLMExecutor) IsCircuitOpen() bool {
	return r.circuitBreaker.IsOpen()
}

// nonRetryableError wraps an error that should not be retried.
type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string {
	return e.err.Error()
}

func (e *nonRetryableError) Unwrap() error {
	return e.err
}

// isTransientError determines if an error is transient and should be retried.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Rate limiting
	if contains(errStr, "rate limit", "too many requests", "429", "quota exceeded") {
		return true
	}

	// Timeout errors
	if contains(errStr, "timeout", "deadline exceeded", "context deadline") {
		return true
	}

	// Network errors
	if contains(errStr, "connection refused", "network unreachable", "no route to host",
		"connection reset", "temporary failure", "i/o timeout") {
		return true
	}

	// Server errors (5xx)
	if contains(errStr, "500", "502", "503", "504", "internal server error",
		"bad gateway", "service unavailable", "gateway timeout") {
		return true
	}

	// LLM-specific transient errors
	if contains(errStr, "overloaded", "capacity", "try again") {
		return true
	}

	return false
}

// contains checks if the string contains any of the substrings (case-insensitive).
func contains(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
