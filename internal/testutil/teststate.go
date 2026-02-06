package testutil

import (
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

// NewTestState creates a WorkflowState with sensible defaults for tests.
// Use functional options to override specific fields.
func NewTestState(opts ...func(*core.WorkflowState)) *core.WorkflowState {
	s := &core.WorkflowState{
		WorkflowDefinition: core.WorkflowDefinition{
			Version:    1,
			WorkflowID: "wf-test",
			Blueprint:  &core.Blueprint{},
			CreatedAt:  time.Now(),
		},
		WorkflowRun: core.WorkflowRun{
			Status:      core.WorkflowStatusPending,
			Tasks:       make(map[core.TaskID]*core.TaskState),
			TaskOrder:   make([]core.TaskID, 0),
			Checkpoints: make([]core.Checkpoint, 0),
			Metrics:     &core.StateMetrics{},
			UpdatedAt:   time.Now(),
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
