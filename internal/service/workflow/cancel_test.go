package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestIsWorkflowCancelled(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"context.Canceled", context.Canceled, true},
		{"domain CANCELLED error", core.ErrState("CANCELLED", "user cancelled"), true},
		{"other domain error", core.ErrState("FAILED", "something else"), false},
		{"generic error", errors.New("random"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWorkflowCancelled(tt.err); got != tt.want {
				t.Errorf("isWorkflowCancelled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflowCancelledError(t *testing.T) {
	err := workflowCancelledError()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !isWorkflowCancelled(err) {
		t.Error("workflowCancelledError() should be recognized as cancelled")
	}

	var domErr *core.DomainError
	if !errors.As(err, &domErr) {
		t.Fatal("expected DomainError")
	}
	if domErr.Code != "CANCELLED" {
		t.Errorf("expected code CANCELLED, got %q", domErr.Code)
	}
}
