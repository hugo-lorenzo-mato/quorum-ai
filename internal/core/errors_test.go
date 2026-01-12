package core

import (
	"errors"
	"testing"
)

func TestDomainError_ErrorAndUnwrap(t *testing.T) {
	cause := errors.New("root")
	err := (&DomainError{
		Category: ErrCatValidation,
		Code:     "CODE",
		Message:  "message",
	}).WithCause(cause)

	if err.Unwrap() != cause {
		t.Fatalf("expected cause to be unwrapped")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected errors.Is to match cause")
	}

	match := &DomainError{Category: ErrCatValidation, Code: "CODE"}
	if !errors.Is(err, match) {
		t.Fatalf("expected errors.Is to match category and code")
	}
}

func TestDomainError_WithDetail(t *testing.T) {
	err := &DomainError{Category: ErrCatExecution, Code: "X", Message: "msg"}
	err.WithDetail("k", "v")
	if err.Details == nil || err.Details["k"] != "v" {
		t.Fatalf("expected details to be set")
	}
}

func TestErrorFactories(t *testing.T) {
	if ErrValidation("C", "m").Retryable {
		t.Fatalf("validation should not be retryable")
	}
	if !ErrExecution("C", "m").Retryable {
		t.Fatalf("execution should be retryable")
	}
	if !ErrTimeout("m").Retryable {
		t.Fatalf("timeout should be retryable")
	}
	if !ErrRateLimit("m").Retryable {
		t.Fatalf("rate limit should be retryable")
	}
	if ErrState("C", "m").Retryable {
		t.Fatalf("state should not be retryable")
	}
	if ErrConsensus("m").Retryable {
		t.Fatalf("consensus should not be retryable")
	}
	if ErrAuth("m").Retryable {
		t.Fatalf("auth should not be retryable")
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(ErrExecution("X", "m")) {
		t.Fatalf("expected retryable error")
	}
	if IsRetryable(errors.New("plain")) {
		t.Fatalf("expected non-domain error to be non-retryable")
	}
}

func TestGetCategory(t *testing.T) {
	if GetCategory(ErrRateLimit("m")) != ErrCatRateLimit {
		t.Fatalf("expected rate_limit category")
	}
	if GetCategory(errors.New("plain")) != ErrCatInternal {
		t.Fatalf("expected internal category for non-domain error")
	}
	if !IsCategory(ErrAuth("m"), ErrCatAuth) {
		t.Fatalf("expected category match")
	}
}
