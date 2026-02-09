package issues

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// mockAgent implements core.Agent for testing.
type mockAgent struct {
	executeFunc func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error)
	callCount   int
}

func (m *mockAgent) Execute(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
	m.callCount++
	if m.executeFunc != nil {
		return m.executeFunc(ctx, opts)
	}
	return &core.ExecuteResult{}, nil
}

func (m *mockAgent) Name() string { return "mock" }

func (m *mockAgent) Capabilities() core.Capabilities {
	return core.Capabilities{}
}

func (m *mockAgent) Ping(ctx context.Context) error { return nil }

func TestCircuitBreaker_InitialState(t *testing.T) {
	t.Parallel()
	cb := NewLLMCircuitBreaker(3, 30*time.Second)

	if cb.IsOpen() {
		t.Error("expected circuit breaker to be closed initially")
	}

	failures, isOpen, _ := cb.GetState()
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}
	if isOpen {
		t.Error("expected circuit breaker to be closed")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	t.Parallel()
	cb := NewLLMCircuitBreaker(3, 30*time.Second)

	// Record failures up to threshold
	cb.RecordFailure()
	cb.RecordFailure()
	opened := cb.RecordFailure()

	if !opened {
		t.Error("expected RecordFailure to return true when circuit opens")
	}

	if !cb.IsOpen() {
		t.Error("expected circuit breaker to be open after threshold failures")
	}

	failures, isOpen, _ := cb.GetState()
	if failures != 3 {
		t.Errorf("expected 3 failures, got %d", failures)
	}
	if !isOpen {
		t.Error("expected circuit to be open")
	}
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	t.Parallel()
	cb := NewLLMCircuitBreaker(3, 30*time.Second)

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()

	// Record success - should reset
	cb.RecordSuccess()

	failures, _, _ := cb.GetState()
	if failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", failures)
	}

	// Should not open after more failures since counter reset
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.IsOpen() {
		t.Error("expected circuit breaker to remain closed")
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	t.Parallel()
	// Use very short reset timeout for testing
	cb := NewLLMCircuitBreaker(2, 10*time.Millisecond)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	if !cb.IsOpen() {
		t.Error("expected circuit to be open")
	}

	// Wait for reset timeout
	time.Sleep(20 * time.Millisecond)

	// Should allow request (transitions to half-open)
	if !cb.AllowRequest() {
		t.Error("expected AllowRequest to return true after reset timeout")
	}

	// Success in half-open should close circuit
	cb.RecordSuccess()

	if cb.IsOpen() {
		t.Error("expected circuit to be closed after success in half-open")
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	t.Parallel()
	// Use very short reset timeout for testing
	cb := NewLLMCircuitBreaker(2, 10*time.Millisecond)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for reset timeout
	time.Sleep(20 * time.Millisecond)

	// Allow request (transitions to half-open)
	cb.AllowRequest()

	// Failure in half-open should re-open
	reopened := cb.RecordFailure()

	if !reopened {
		t.Error("expected RecordFailure to return true when re-opening from half-open")
	}

	if !cb.IsOpen() {
		t.Error("expected circuit to be open after failure in half-open")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	t.Parallel()
	cb := NewLLMCircuitBreaker(2, 30*time.Second)

	// Open the circuit
	cb.RecordFailure()
	cb.RecordFailure()

	// Reset manually
	cb.Reset()

	if cb.IsOpen() {
		t.Error("expected circuit to be closed after reset")
	}

	failures, isOpen, _ := cb.GetState()
	if failures != 0 {
		t.Errorf("expected 0 failures after reset, got %d", failures)
	}
	if isOpen {
		t.Error("expected circuit to be closed")
	}
}

func TestLLMMetrics_GetAverageLatencyMs(t *testing.T) {
	t.Parallel()
	metrics := &LLMMetrics{}

	// No calls yet
	if avg := metrics.GetAverageLatencyMs(); avg != 0 {
		t.Errorf("expected 0 avg latency with no calls, got %f", avg)
	}

	// Add some calls
	metrics.TotalCalls.Add(3)
	metrics.TotalLatencyMs.Add(300) // 300ms total

	avg := metrics.GetAverageLatencyMs()
	if avg != 100 {
		t.Errorf("expected avg latency 100ms, got %f", avg)
	}
}

func TestLLMMetrics_GetSuccessRate(t *testing.T) {
	t.Parallel()
	metrics := &LLMMetrics{}

	// No calls yet
	if rate := metrics.GetSuccessRate(); rate != 0 {
		t.Errorf("expected 0%% success rate with no calls, got %f", rate)
	}

	// Add calls
	metrics.TotalCalls.Add(10)
	metrics.SuccessfulCalls.Add(8)

	rate := metrics.GetSuccessRate()
	if rate != 80 {
		t.Errorf("expected 80%% success rate, got %f", rate)
	}
}

func TestIsTransientError_RateLimiting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err       error
		transient bool
	}{
		{errors.New("rate limit exceeded"), true},
		{errors.New("too many requests"), true},
		{errors.New("429 too many requests"), true},
		{errors.New("quota exceeded for this minute"), true},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestIsTransientError_Timeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err       error
		transient bool
	}{
		{errors.New("timeout waiting for response"), true},
		{errors.New("deadline exceeded"), true},
		{errors.New("context deadline exceeded"), true},
		{context.DeadlineExceeded, true},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestIsTransientError_Network(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err       error
		transient bool
	}{
		{errors.New("connection refused"), true},
		{errors.New("network unreachable"), true},
		{errors.New("no route to host"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("i/o timeout"), true},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestIsTransientError_ServerErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err       error
		transient bool
	}{
		{errors.New("500 internal server error"), true},
		{errors.New("502 bad gateway"), true},
		{errors.New("503 service unavailable"), true},
		{errors.New("504 gateway timeout"), true},
	}

	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestIsTransientError_NonTransient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err       error
		transient bool
	}{
		{errors.New("invalid api key"), false},
		{errors.New("authentication failed"), false},
		{errors.New("permission denied"), false},
		{errors.New("404 not found"), false},
		{errors.New("invalid request format"), false},
		{nil, false},
	}

	for _, tc := range tests {
		name := "nil"
		if tc.err != nil {
			name = tc.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.transient {
				t.Errorf("isTransientError(%q) = %v, want %v", tc.err, got, tc.transient)
			}
		})
	}
}

func TestDefaultLLMResilienceConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultLLMResilienceConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialBackoff != time.Second {
		t.Errorf("expected InitialBackoff=1s, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("expected MaxBackoff=30s, got %v", cfg.MaxBackoff)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %f", cfg.BackoffMultiplier)
	}
	if cfg.FailureThreshold != 3 {
		t.Errorf("expected FailureThreshold=3, got %d", cfg.FailureThreshold)
	}
	if cfg.ResetTimeout != 30*time.Second {
		t.Errorf("expected ResetTimeout=30s, got %v", cfg.ResetTimeout)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled=true by default")
	}
}

func TestResilientLLMExecutor_Success(t *testing.T) {
	t.Parallel()
	agent := &mockAgent{
		executeFunc: func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			return &core.ExecuteResult{Output: "success"}, nil
		},
	}

	executor := NewResilientLLMExecutor(agent, DefaultLLMResilienceConfig())

	result, err := executor.Execute(context.Background(), core.ExecuteOptions{})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Output != "success" {
		t.Errorf("expected output 'success', got '%s'", result.Output)
	}
	if agent.callCount != 1 {
		t.Errorf("expected 1 call, got %d", agent.callCount)
	}
}

func TestResilientLLMExecutor_DisabledCallsDirectly(t *testing.T) {
	t.Parallel()
	callCount := 0
	agent := &mockAgent{
		executeFunc: func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			callCount++
			return &core.ExecuteResult{}, nil
		},
	}

	cfg := DefaultLLMResilienceConfig()
	cfg.Enabled = false
	executor := NewResilientLLMExecutor(agent, cfg)

	_, err := executor.Execute(context.Background(), core.ExecuteOptions{})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 direct call, got %d", callCount)
	}
}

func TestResilientLLMExecutor_CircuitOpen(t *testing.T) {
	t.Parallel()
	agent := &mockAgent{
		executeFunc: func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			return nil, errors.New("service unavailable")
		},
	}

	cfg := DefaultLLMResilienceConfig()
	cfg.FailureThreshold = 2
	cfg.MaxRetries = 0 // Disable retries for this test
	executor := NewResilientLLMExecutor(agent, cfg)

	// Exhaust failures to open circuit
	executor.Execute(context.Background(), core.ExecuteOptions{})
	executor.Execute(context.Background(), core.ExecuteOptions{})

	// Next call should fail with circuit open
	_, err := executor.Execute(context.Background(), core.ExecuteOptions{})

	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestResilientLLMExecutor_ResetCircuitBreaker(t *testing.T) {
	t.Parallel()
	agent := &mockAgent{
		executeFunc: func(ctx context.Context, opts core.ExecuteOptions) (*core.ExecuteResult, error) {
			return nil, errors.New("error")
		},
	}

	cfg := DefaultLLMResilienceConfig()
	cfg.FailureThreshold = 1
	cfg.MaxRetries = 0
	executor := NewResilientLLMExecutor(agent, cfg)

	// Open the circuit
	executor.Execute(context.Background(), core.ExecuteOptions{})

	if !executor.IsCircuitOpen() {
		t.Error("expected circuit to be open")
	}

	// Reset
	executor.ResetCircuitBreaker()

	if executor.IsCircuitOpen() {
		t.Error("expected circuit to be closed after reset")
	}
}

func TestNewLLMCircuitBreaker_DefaultValues(t *testing.T) {
	t.Parallel()
	// Test with zero/negative values
	cb := NewLLMCircuitBreaker(0, 0)

	// Should use defaults
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if !cb.IsOpen() {
		t.Error("expected circuit to open after 3 failures (default threshold)")
	}
}

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s        string
		substrs  []string
		expected bool
	}{
		{"rate limit exceeded", []string{"rate limit"}, true},
		{"RATE LIMIT EXCEEDED", []string{"rate limit"}, true},
		{"error occurred", []string{"rate limit", "timeout"}, false},
		{"connection timeout", []string{"rate limit", "timeout"}, true},
		{"", []string{"anything"}, false},
	}

	for _, tc := range tests {
		result := contains(tc.s, tc.substrs...)
		if result != tc.expected {
			t.Errorf("contains(%q, %v) = %v, want %v", tc.s, tc.substrs, result, tc.expected)
		}
	}
}

func TestNonRetryableError(t *testing.T) {
	t.Parallel()
	originalErr := errors.New("invalid api key")
	nre := &nonRetryableError{err: originalErr}

	if nre.Error() != "invalid api key" {
		t.Errorf("expected error message 'invalid api key', got '%s'", nre.Error())
	}

	if nre.Unwrap() != originalErr {
		t.Error("expected Unwrap to return original error")
	}
}
