package kanban

import (
	"sync"
	"time"
)

// DefaultCircuitBreakerThreshold is the number of consecutive failures before the circuit opens.
const DefaultCircuitBreakerThreshold = 2

// CircuitBreaker implements the circuit breaker pattern for the Kanban engine.
// It tracks consecutive failures and opens (stops execution) when threshold is reached.
//
// The circuit breaker has three conceptual states:
// - Closed: Normal operation, failures < threshold
// - Open: Execution halted, failures >= threshold, requires manual reset
//
// Note: This is a simplified circuit breaker without half-open state,
// since the Kanban engine requires explicit user acknowledgment to resume.
type CircuitBreaker struct {
	mu                  sync.RWMutex
	threshold           int
	consecutiveFailures int
	open                bool
	lastFailureAt       time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given threshold.
// If threshold <= 0, DefaultCircuitBreakerThreshold (2) is used.
func NewCircuitBreaker(threshold int) *CircuitBreaker {
	if threshold <= 0 {
		threshold = DefaultCircuitBreakerThreshold
	}
	return &CircuitBreaker{
		threshold: threshold,
	}
}

// RecordSuccess records a successful workflow execution.
// This resets the consecutive failure count but does NOT automatically close the breaker.
// The breaker must be manually reset if it was open.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFailures = 0
	// Note: We do NOT set cb.open = false here.
	// The circuit breaker requires manual reset once tripped.
}

// RecordFailure records a workflow execution failure.
// Returns true if this failure caused the circuit breaker to trip (open).
func (cb *CircuitBreaker) RecordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++
	cb.lastFailureAt = time.Now()

	if cb.consecutiveFailures >= cb.threshold && !cb.open {
		cb.open = true
		return true // Breaker just tripped
	}
	return false
}

// IsOpen returns true if the circuit breaker is open (tripped).
// When open, the Kanban engine should not pick new workflows for execution.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.open
}

// ConsecutiveFailures returns the current count of consecutive failures.
func (cb *CircuitBreaker) ConsecutiveFailures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.consecutiveFailures
}

// LastFailureAt returns the time of the last recorded failure.
// Returns zero time if no failures have been recorded.
func (cb *CircuitBreaker) LastFailureAt() time.Time {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.lastFailureAt
}

// Threshold returns the number of consecutive failures required to trip the breaker.
func (cb *CircuitBreaker) Threshold() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.threshold
}

// Reset closes the circuit breaker and resets the failure count.
// This is typically called when the user manually resets the circuit breaker
// after investigating and addressing the cause of failures.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFailures = 0
	cb.open = false
	cb.lastFailureAt = time.Time{}
}

// Open forces the circuit breaker into the open state.
// This is used during recovery when loading persisted state indicates
// the breaker was previously open.
func (cb *CircuitBreaker) Open() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.open = true
}

// SetState sets the circuit breaker state from persisted values.
// This is used during initialization to restore state from the database.
func (cb *CircuitBreaker) SetState(failures int, open bool, lastFailure time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFailures = failures
	cb.open = open
	cb.lastFailureAt = lastFailure
}

// GetState returns the current state for persistence.
// Returns: consecutiveFailures, isOpen, lastFailureAt
func (cb *CircuitBreaker) GetState() (failures int, open bool, lastFailure time.Time) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.consecutiveFailures, cb.open, cb.lastFailureAt
}
