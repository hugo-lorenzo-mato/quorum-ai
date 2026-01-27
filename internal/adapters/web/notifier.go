// Package web provides HTTP adapters for the web interface.
package web

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service/workflow"
)

// Compile-time check that WebOutputNotifier implements workflow.OutputNotifier.
var _ workflow.OutputNotifier = (*WebOutputNotifier)(nil)

// MaxAgentEvents is the maximum number of agent events to persist per workflow.
const MaxAgentEvents = 100

// saveThrottleInterval is the minimum time between state saves to avoid excessive disk I/O.
const saveThrottleInterval = 2 * time.Second

// StateSaver is the interface for persisting workflow state.
type StateSaver interface {
	Save(ctx context.Context, state *core.WorkflowState) error
}

// WebOutputNotifier bridges workflow.OutputNotifier to EventBus for SSE streaming.
// It implements the workflow.OutputNotifier interface and provides additional
// lifecycle methods (WorkflowStarted, WorkflowCompleted, WorkflowFailed) that
// the interface doesn't include but the web context needs.
type WebOutputNotifier struct {
	eventBus    *events.EventBus
	workflowID  string
	state       *core.WorkflowState // Optional: for persisting agent events
	stateSaver  StateSaver          // Optional: for saving state to disk
	stateMu     sync.Mutex          // Protects state access
	lastSave    time.Time           // Last time state was saved
	pendingSave bool                // Whether there are unsaved changes
}

// NewWebOutputNotifier creates a new web output notifier.
func NewWebOutputNotifier(eventBus *events.EventBus, workflowID string) *WebOutputNotifier {
	return &WebOutputNotifier{
		eventBus:   eventBus,
		workflowID: workflowID,
	}
}

// SetState sets the workflow state for persisting agent events.
// Must be called before agent events will be persisted.
func (n *WebOutputNotifier) SetState(state *core.WorkflowState) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.state = state
}

// SetStateSaver sets the state saver for persisting state to disk.
// When set, agent events will be periodically saved to disk.
func (n *WebOutputNotifier) SetStateSaver(saver StateSaver) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	n.stateSaver = saver
}

// saveStateIfNeeded saves the state to disk if enough time has passed since the last save.
// Must be called with stateMu locked.
func (n *WebOutputNotifier) saveStateIfNeeded() {
	if n.state == nil || n.stateSaver == nil || !n.pendingSave {
		return
	}

	if time.Since(n.lastSave) < saveThrottleInterval {
		return
	}

	// Save in background to avoid blocking
	stateCopy := n.state
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = n.stateSaver.Save(ctx, stateCopy)
	}()

	n.lastSave = time.Now()
	n.pendingSave = false
}

// FlushState forces an immediate save of any pending state changes.
func (n *WebOutputNotifier) FlushState() {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()

	if n.state == nil || n.stateSaver == nil || !n.pendingSave {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = n.stateSaver.Save(ctx, n.state)
	n.pendingSave = false
}

// PhaseStarted is called when a phase begins.
func (n *WebOutputNotifier) PhaseStarted(phase core.Phase) {
	n.eventBus.Publish(events.NewPhaseStartedEvent(n.workflowID, string(phase)))
}

// TaskStarted is called when a task begins.
func (n *WebOutputNotifier) TaskStarted(task *core.Task) {
	n.eventBus.Publish(events.NewTaskStartedEvent(n.workflowID, string(task.ID), ""))
}

// TaskCompleted is called when a task finishes successfully.
func (n *WebOutputNotifier) TaskCompleted(task *core.Task, duration time.Duration) {
	n.eventBus.Publish(events.NewTaskCompletedEvent(
		n.workflowID,
		string(task.ID),
		duration,
		task.TokensIn,
		task.TokensOut,
		task.CostUSD,
	))
}

// TaskFailed is called when a task fails.
func (n *WebOutputNotifier) TaskFailed(task *core.Task, err error) {
	n.eventBus.Publish(events.NewTaskFailedEvent(n.workflowID, string(task.ID), err, false))
}

// TaskSkipped is called when a task is skipped.
func (n *WebOutputNotifier) TaskSkipped(task *core.Task, reason string) {
	n.eventBus.Publish(events.NewTaskSkippedEvent(n.workflowID, string(task.ID), reason))
}

// WorkflowStateUpdated is called when the workflow state changes.
func (n *WebOutputNotifier) WorkflowStateUpdated(state *core.WorkflowState) {
	var completed, failed, skipped int
	for _, task := range state.Tasks {
		switch task.Status {
		case core.TaskStatusCompleted:
			completed++
		case core.TaskStatusFailed:
			failed++
		case core.TaskStatusSkipped:
			skipped++
		}
	}
	n.eventBus.Publish(events.NewWorkflowStateUpdatedEvent(
		n.workflowID,
		string(state.CurrentPhase),
		len(state.Tasks),
		completed,
		failed,
		skipped,
	))
}

// Log sends a log message to the UI.
func (n *WebOutputNotifier) Log(level, source, message string) {
	fullMessage := "[" + source + "] " + message
	n.eventBus.Publish(events.NewLogEvent(n.workflowID, level, fullMessage, nil))
}

// AgentEvent is called when an agent emits a streaming event.
func (n *WebOutputNotifier) AgentEvent(kind, agent, message string, data map[string]interface{}) {
	// Publish to SSE for real-time updates
	n.eventBus.Publish(events.NewAgentStreamEvent(n.workflowID, events.AgentEventType(kind), agent, message).WithData(data))

	// Also persist to workflow state for reload recovery
	n.stateMu.Lock()
	defer n.stateMu.Unlock()
	if n.state != nil {
		event := core.AgentEvent{
			ID:        fmt.Sprintf("%d-%s", time.Now().UnixNano(), agent),
			Type:      core.AgentEventType(kind),
			Agent:     agent,
			Message:   message,
			Data:      data,
			Timestamp: time.Now(),
		}
		n.state.AgentEvents = append(n.state.AgentEvents, event)
		// Limit to last MaxAgentEvents
		if len(n.state.AgentEvents) > MaxAgentEvents {
			n.state.AgentEvents = n.state.AgentEvents[len(n.state.AgentEvents)-MaxAgentEvents:]
		}
		n.pendingSave = true
		n.saveStateIfNeeded()
	}
}

// WorkflowStarted emits a workflow_started event.
// NOTE: This is NOT part of the OutputNotifier interface but is needed for web context.
func (n *WebOutputNotifier) WorkflowStarted(prompt string) {
	n.eventBus.Publish(events.NewWorkflowStartedEvent(n.workflowID, prompt))
}

// WorkflowCompleted emits a workflow_completed event using priority channel.
// NOTE: This is NOT part of the OutputNotifier interface but is needed for web context.
func (n *WebOutputNotifier) WorkflowCompleted(duration time.Duration, totalCost float64) {
	n.eventBus.PublishPriority(events.NewWorkflowCompletedEvent(n.workflowID, duration, totalCost))
}

// WorkflowFailed emits a workflow_failed event using priority channel.
// NOTE: This is NOT part of the OutputNotifier interface but is needed for web context.
func (n *WebOutputNotifier) WorkflowFailed(phase string, err error) {
	n.eventBus.PublishPriority(events.NewWorkflowFailedEvent(n.workflowID, phase, err))
}
