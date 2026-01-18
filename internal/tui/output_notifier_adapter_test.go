package tui

import (
	"testing"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

type mockOutput struct {
	onWorkflowCompleted    func(*core.WorkflowState)
	onWorkflowStateUpdated func(*core.WorkflowState)
}

func (m *mockOutput) WorkflowStarted(_ string)                    {}
func (m *mockOutput) PhaseStarted(_ core.Phase)                   {}
func (m *mockOutput) TaskStarted(_ *core.Task)                    {}
func (m *mockOutput) TaskCompleted(_ *core.Task, _ time.Duration) {}
func (m *mockOutput) TaskFailed(_ *core.Task, _ error)            {}
func (m *mockOutput) TaskSkipped(_ *core.Task, _ string)          {}
func (m *mockOutput) WorkflowStateUpdated(state *core.WorkflowState) {
	if m.onWorkflowStateUpdated != nil {
		m.onWorkflowStateUpdated(state)
	}
}
func (m *mockOutput) WorkflowCompleted(state *core.WorkflowState) {
	if m.onWorkflowCompleted != nil {
		m.onWorkflowCompleted(state)
	}
}
func (m *mockOutput) WorkflowFailed(_ error) {}
func (m *mockOutput) Log(_, _ string)        {}
func (m *mockOutput) Close() error           { return nil }

func TestWorkflowCompletedEmittedOnce(t *testing.T) {
	var completedCount int
	output := &mockOutput{
		onWorkflowCompleted: func(_ *core.WorkflowState) {
			completedCount++
		},
		onWorkflowStateUpdated: func(_ *core.WorkflowState) {
			// Should NOT increment completedCount
		},
	}

	adapter := NewOutputNotifierAdapter(output)

	// Simulate planner calling WorkflowStateUpdated
	adapter.WorkflowStateUpdated(&core.WorkflowState{})
	adapter.WorkflowStateUpdated(&core.WorkflowState{})

	// Only CLI should call WorkflowCompleted at the end
	// (not simulated here, just verify adapter doesn't call it)

	if completedCount != 0 {
		t.Errorf("WorkflowStateUpdated should not trigger WorkflowCompleted, got %d", completedCount)
	}
}
