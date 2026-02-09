package core

import (
	"errors"
	"testing"
)

func TestDomainError_ErrorAndUnwrap(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	err := &DomainError{Category: ErrCatExecution, Code: "X", Message: "msg"}
	err.WithDetail("k", "v")
	if err.Details == nil || err.Details["k"] != "v" {
		t.Fatalf("expected details to be set")
	}
}

func TestErrorFactories(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	if !IsRetryable(ErrExecution("X", "m")) {
		t.Fatalf("expected retryable error")
	}
	if IsRetryable(errors.New("plain")) {
		t.Fatalf("expected non-domain error to be non-retryable")
	}
}

func TestGetCategory(t *testing.T) {
	t.Parallel()
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

func TestErrHumanReviewRequired(t *testing.T) {
	t.Parallel()
	err := ErrHumanReviewRequired(0.45, 0.50)

	if err == nil {
		t.Fatal("expected error to be created")
	}
	if err.Category != ErrCatConsensus {
		t.Errorf("Category = %s, want %s", err.Category, ErrCatConsensus)
	}
	if err.Code != "HUMAN_REVIEW_REQUIRED" {
		t.Errorf("Code = %s, want HUMAN_REVIEW_REQUIRED", err.Code)
	}
	if err.Retryable {
		t.Error("human review error should not be retryable")
	}
	if err.Details["score"] != 0.45 {
		t.Errorf("score = %v, want 0.45", err.Details["score"])
	}
	if err.Details["human_threshold"] != 0.50 {
		t.Errorf("human_threshold = %v, want 0.50", err.Details["human_threshold"])
	}
}

func TestErrNotFound(t *testing.T) {
	t.Parallel()
	err := ErrNotFound("task", "task-123")

	if err == nil {
		t.Fatal("expected error to be created")
	}
	if err.Category != ErrCatNotFound {
		t.Errorf("Category = %s, want %s", err.Category, ErrCatNotFound)
	}
	if err.Code != "NOT_FOUND" {
		t.Errorf("Code = %s, want NOT_FOUND", err.Code)
	}
	// ErrNotFound doesn't have Details, but message contains the info
	if err.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestErrWorkflowBudgetExceeded(t *testing.T) {
	t.Parallel()
	err := ErrWorkflowBudgetExceeded(5.50, 5.00)

	if err == nil {
		t.Fatal("expected error to be created")
	}
	if err.Category != ErrCatBudget {
		t.Errorf("Category = %s, want %s", err.Category, ErrCatBudget)
	}
	if err.Code != "WORKFLOW_BUDGET_EXCEEDED" {
		t.Errorf("Code = %s, want WORKFLOW_BUDGET_EXCEEDED", err.Code)
	}
	if err.Details["current_cost"] != 5.50 {
		t.Errorf("current_cost = %v, want 5.50", err.Details["current_cost"])
	}
	if err.Details["limit"] != 5.00 {
		t.Errorf("limit = %v, want 5.00", err.Details["limit"])
	}
}

func TestErrTaskBudgetExceeded(t *testing.T) {
	t.Parallel()
	err := ErrTaskBudgetExceeded("task-1", 1.50, 1.00)

	if err == nil {
		t.Fatal("expected error to be created")
	}
	if err.Category != ErrCatBudget {
		t.Errorf("Category = %s, want %s", err.Category, ErrCatBudget)
	}
	if err.Code != "TASK_BUDGET_EXCEEDED" {
		t.Errorf("Code = %s, want TASK_BUDGET_EXCEEDED", err.Code)
	}
	if err.Details["task_id"] != "task-1" {
		t.Errorf("task_id = %v, want task-1", err.Details["task_id"])
	}
	if err.Details["cost"] != 1.50 {
		t.Errorf("cost = %v, want 1.50", err.Details["cost"])
	}
	if err.Details["limit"] != 1.00 {
		t.Errorf("limit = %v, want 1.00", err.Details["limit"])
	}
}

func TestDomainError_Error_Full(t *testing.T) {
	t.Parallel()
	cause := errors.New("underlying cause")
	err := &DomainError{
		Category: ErrCatExecution,
		Code:     "EXEC_FAILED",
		Message:  "execution failed",
	}
	err.WithCause(cause)

	// Error message should contain the message and cause
	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should not return empty string")
	}
}
