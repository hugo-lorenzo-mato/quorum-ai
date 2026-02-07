package workflow

import (
	"context"
	"errors"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// isWorkflowCancelled returns true if the error represents a user-requested cancellation.
// This is used to map cancellations to WorkflowStatusAborted rather than failed.
func isWorkflowCancelled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	// DomainError comparison only checks Category + Code.
	return errors.Is(err, core.ErrState("CANCELLED", ""))
}

func workflowCancelledError() error {
	return core.ErrState("CANCELLED", "workflow cancelled by user")
}
