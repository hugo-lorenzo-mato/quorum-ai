package core

import (
	"errors"
	"fmt"
)

// ErrorCategory classifies errors for handling decisions.
type ErrorCategory string

const (
	ErrCatValidation ErrorCategory = "validation" // Invalid input
	ErrCatExecution  ErrorCategory = "execution"  // Runtime failure
	ErrCatTimeout    ErrorCategory = "timeout"    // Operation timed out
	ErrCatRateLimit  ErrorCategory = "rate_limit" // API rate limited
	ErrCatState      ErrorCategory = "state"      // State corruption/conflict
	ErrCatConsensus  ErrorCategory = "consensus"  // Consensus not reached
	ErrCatAuth       ErrorCategory = "auth"       // Authentication failure
	ErrCatNetwork    ErrorCategory = "network"    // Network connectivity
	ErrCatNotFound   ErrorCategory = "not_found"  // Resource not found
	ErrCatConflict   ErrorCategory = "conflict"   // Concurrent modification
	ErrCatInternal   ErrorCategory = "internal"   // Unexpected internal error
)

// DomainError represents a structured error from the domain layer.
type DomainError struct {
	Category  ErrorCategory
	Code      string
	Message   string
	Retryable bool
	Cause     error
	Details   map[string]interface{}
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (%v)", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Category, e.Code, e.Message)
}

// Unwrap returns the underlying cause.
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// Is checks if this error matches a target.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Category == t.Category && e.Code == t.Code
}

// WithCause wraps an underlying error.
func (e *DomainError) WithCause(cause error) *DomainError {
	e.Cause = cause
	return e
}

// WithDetail adds contextual information.
func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// ErrValidation creates a validation error.
func ErrValidation(code, message string) *DomainError {
	return &DomainError{
		Category:  ErrCatValidation,
		Code:      code,
		Message:   message,
		Retryable: false,
	}
}

// ErrExecution creates an execution error.
func ErrExecution(code, message string) *DomainError {
	return &DomainError{
		Category:  ErrCatExecution,
		Code:      code,
		Message:   message,
		Retryable: true,
	}
}

// ErrTimeout creates a timeout error.
func ErrTimeout(message string) *DomainError {
	return &DomainError{
		Category:  ErrCatTimeout,
		Code:      "TIMEOUT",
		Message:   message,
		Retryable: true,
	}
}

// ErrRateLimit creates a rate limit error.
func ErrRateLimit(message string) *DomainError {
	return &DomainError{
		Category:  ErrCatRateLimit,
		Code:      "RATE_LIMITED",
		Message:   message,
		Retryable: true,
	}
}

// ErrState creates a state error.
func ErrState(code, message string) *DomainError {
	return &DomainError{
		Category:  ErrCatState,
		Code:      code,
		Message:   message,
		Retryable: false,
	}
}

// ErrConsensus creates a consensus error.
func ErrConsensus(message string) *DomainError {
	return &DomainError{
		Category:  ErrCatConsensus,
		Code:      "CONSENSUS_FAILED",
		Message:   message,
		Retryable: false,
	}
}

// ErrAuth creates an authentication error.
func ErrAuth(message string) *DomainError {
	return &DomainError{
		Category:  ErrCatAuth,
		Code:      "AUTH_FAILED",
		Message:   message,
		Retryable: false,
	}
}

// ErrNotFound creates a not found error.
func ErrNotFound(resource, id string) *DomainError {
	return &DomainError{
		Category:  ErrCatNotFound,
		Code:      "NOT_FOUND",
		Message:   fmt.Sprintf("%s not found: %s", resource, id),
		Retryable: false,
	}
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	var domErr *DomainError
	if errors.As(err, &domErr) {
		return domErr.Retryable
	}
	return false
}

// GetCategory extracts the error category.
func GetCategory(err error) ErrorCategory {
	var domErr *DomainError
	if errors.As(err, &domErr) {
		return domErr.Category
	}
	return ErrCatInternal
}

// IsCategory checks if an error belongs to a category.
func IsCategory(err error, cat ErrorCategory) bool {
	return GetCategory(err) == cat
}

// Predefined error codes
const (
	CodeTaskNotFound      = "TASK_NOT_FOUND"
	CodeWorkflowNotFound  = "WORKFLOW_NOT_FOUND"
	CodeInvalidState      = "INVALID_STATE"
	CodeLockAcquireFailed = "LOCK_ACQUIRE_FAILED"
	CodeStateCorrupted    = "STATE_CORRUPTED"
	CodeAgentUnavailable  = "AGENT_UNAVAILABLE"
	CodeConsensusLow      = "CONSENSUS_BELOW_THRESHOLD"
	CodeChecksFailed      = "CHECKS_FAILED"
	CodeMergeConflict     = "MERGE_CONFLICT"

	// Validation error codes
	CodeEmptyPrompt    = "EMPTY_PROMPT"
	CodePromptTooLong  = "PROMPT_TOO_LONG"
	CodeInvalidConfig  = "INVALID_CONFIG"
	CodeNoAgents       = "NO_AGENTS"
	CodeInvalidTimeout = "INVALID_TIMEOUT"

	// Execution error codes
	CodeAgentFailed    = "AGENT_FAILED"
	CodeExecutionStuck = "EXECUTION_STUCK"
	CodeParseFailed    = "PARSE_FAILED"
	CodeDAGCycle       = "DAG_CYCLE"
)

// MaxPromptLength is the maximum allowed prompt length.
const MaxPromptLength = 100000
