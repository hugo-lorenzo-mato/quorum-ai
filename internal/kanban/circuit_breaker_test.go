package kanban

import (
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	tests := []struct {
		name              string
		threshold         int
		expectedThreshold int
	}{
		{"positive threshold", 3, 3},
		{"zero threshold uses default", 0, DefaultCircuitBreakerThreshold},
		{"negative threshold uses default", -1, DefaultCircuitBreakerThreshold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.threshold)
			if got := cb.Threshold(); got != tt.expectedThreshold {
				t.Errorf("Threshold() = %v, want %v", got, tt.expectedThreshold)
			}
			if cb.IsOpen() {
				t.Error("new circuit breaker should be closed")
			}
			if cb.ConsecutiveFailures() != 0 {
				t.Error("new circuit breaker should have 0 failures")
			}
		})
	}
}

func TestCircuitBreaker_RecordFailure(t *testing.T) {
	cb := NewCircuitBreaker(3)

	// First two failures should not trip
	if tripped := cb.RecordFailure(); tripped {
		t.Error("first failure should not trip breaker")
	}
	if cb.ConsecutiveFailures() != 1 {
		t.Errorf("expected 1 failure, got %d", cb.ConsecutiveFailures())
	}

	if tripped := cb.RecordFailure(); tripped {
		t.Error("second failure should not trip breaker")
	}
	if cb.ConsecutiveFailures() != 2 {
		t.Errorf("expected 2 failures, got %d", cb.ConsecutiveFailures())
	}

	// Third failure should trip
	if tripped := cb.RecordFailure(); !tripped {
		t.Error("third failure should trip breaker")
	}
	if !cb.IsOpen() {
		t.Error("breaker should be open after threshold")
	}

	// Fourth failure should not return tripped again
	if tripped := cb.RecordFailure(); tripped {
		t.Error("already tripped breaker should return false")
	}
	if cb.ConsecutiveFailures() != 4 {
		t.Errorf("expected 4 failures, got %d", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker(3)

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()

	// Success resets failure count
	cb.RecordSuccess()
	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.ConsecutiveFailures())
	}

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Error("breaker should be open")
	}

	// Success does NOT close the breaker once open
	cb.RecordSuccess()
	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.ConsecutiveFailures())
	}
	if !cb.IsOpen() {
		t.Error("breaker should still be open after success (requires manual reset)")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(2)

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Error("breaker should be open")
	}

	// Reset
	cb.Reset()
	if cb.IsOpen() {
		t.Error("breaker should be closed after reset")
	}
	if cb.ConsecutiveFailures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.ConsecutiveFailures())
	}
	if !cb.LastFailureAt().IsZero() {
		t.Error("last failure time should be zero after reset")
	}
}

func TestCircuitBreaker_Open(t *testing.T) {
	cb := NewCircuitBreaker(5)

	cb.Open()
	if !cb.IsOpen() {
		t.Error("breaker should be open after Open()")
	}
}

func TestCircuitBreaker_SetState(t *testing.T) {
	cb := NewCircuitBreaker(3)
	lastFailure := time.Now().Add(-time.Hour)

	cb.SetState(5, true, lastFailure)

	if cb.ConsecutiveFailures() != 5 {
		t.Errorf("expected 5 failures, got %d", cb.ConsecutiveFailures())
	}
	if !cb.IsOpen() {
		t.Error("breaker should be open")
	}
	if !cb.LastFailureAt().Equal(lastFailure) {
		t.Errorf("last failure time mismatch: got %v, want %v", cb.LastFailureAt(), lastFailure)
	}
}

func TestCircuitBreaker_GetState(t *testing.T) {
	cb := NewCircuitBreaker(2)

	cb.RecordFailure()
	cb.RecordFailure()

	failures, isOpen, lastFailure := cb.GetState()

	if failures != 2 {
		t.Errorf("expected 2 failures, got %d", failures)
	}
	if !isOpen {
		t.Error("breaker should be open")
	}
	if lastFailure.IsZero() {
		t.Error("last failure time should be set")
	}
}

func TestCircuitBreaker_LastFailureAt(t *testing.T) {
	cb := NewCircuitBreaker(5)

	if !cb.LastFailureAt().IsZero() {
		t.Error("new breaker should have zero last failure time")
	}

	before := time.Now()
	cb.RecordFailure()
	after := time.Now()

	lastFailure := cb.LastFailureAt()
	if lastFailure.Before(before) || lastFailure.After(after) {
		t.Errorf("last failure time %v should be between %v and %v", lastFailure, before, after)
	}
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	cb := NewCircuitBreaker(100)
	done := make(chan bool)

	// Simulate concurrent access
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				cb.RecordFailure()
				cb.ConsecutiveFailures()
				cb.IsOpen()
				cb.GetState()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if cb.ConsecutiveFailures() != 200 {
		t.Errorf("expected 200 failures from concurrent access, got %d", cb.ConsecutiveFailures())
	}
}
