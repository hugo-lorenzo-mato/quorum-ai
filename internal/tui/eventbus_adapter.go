package tui

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// EventBusAdapter bridges EventBus events to Bubbletea messages.
type EventBusAdapter struct {
	bus        *events.EventBus
	eventCh    <-chan events.Event
	priorityCh <-chan events.Event
	msgCh      chan tea.Msg
	closeCh    chan struct{}
	mu         sync.Mutex
	closed     bool

	// State tracking for agent mapping
	taskToAgent   map[string]string    // taskID -> agentID
	agentDuration map[string]time.Time // agentID -> start time
	totalRequests int
}

// NewEventBusAdapter creates a new adapter.
func NewEventBusAdapter(bus *events.EventBus) *EventBusAdapter {
	adapter := &EventBusAdapter{
		bus:           bus,
		eventCh:       bus.Subscribe(), // Subscribe to all events
		priorityCh:    bus.SubscribePriority(),
		msgCh:         make(chan tea.Msg, 100),
		closeCh:       make(chan struct{}),
		taskToAgent:   make(map[string]string),
		agentDuration: make(map[string]time.Time),
	}

	go adapter.run()
	return adapter
}

// MsgChannel returns the channel for Bubbletea to read from.
func (a *EventBusAdapter) MsgChannel() <-chan tea.Msg {
	return a.msgCh
}

// Close shuts down the adapter.
func (a *EventBusAdapter) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return
	}
	a.closed = true
	close(a.closeCh)
}

// run processes events and converts them to tea.Msg.
func (a *EventBusAdapter) run() {
	for {
		select {
		case <-a.closeCh:
			close(a.msgCh)
			return

		case event, ok := <-a.priorityCh:
			if !ok {
				return
			}
			a.handleEvent(event)

		case event, ok := <-a.eventCh:
			if !ok {
				return
			}
			a.handleEvent(event)
		}
	}
}

// handleEvent converts an event to tea.Msg and sends it.
func (a *EventBusAdapter) handleEvent(event events.Event) {
	msg := a.eventToMsg(event)
	if msg == nil {
		return
	}

	select {
	case a.msgCh <- msg:
	default:
		// Drop if channel full (shouldn't happen often)
	}
}

// eventToMsg converts an events.Event to a tea.Msg.
func (a *EventBusAdapter) eventToMsg(event events.Event) tea.Msg {
	switch e := event.(type) {
	case events.WorkflowStartedEvent:
		return LogMsg{
			Level:   "info",
			Message: "Workflow started",
		}

	case events.WorkflowStateUpdatedEvent:
		// Create a minimal WorkflowState for UI
		return WorkflowUpdateMsg{
			State: &core.WorkflowState{
				WorkflowRun: core.WorkflowRun{
					CurrentPhase: core.Phase(e.Phase),
					Status:       core.WorkflowStatusRunning,
				},
			},
		}

	case events.WorkflowCompletedEvent:
		return WorkflowUpdateMsg{
			State: &core.WorkflowState{
				WorkflowRun: core.WorkflowRun{
					Status: core.WorkflowStatusCompleted,
				},
			},
		}

	case events.WorkflowFailedEvent:
		return ErrorMsg{
			Error: &eventError{message: e.Error},
		}

	case events.PhaseStartedEvent:
		return PhaseUpdateMsg{
			Phase: core.Phase(e.Phase),
		}

	case events.TaskCreatedEvent:
		// Track task to agent mapping
		agentID := normalizeAgentID(e.Agent)
		a.mu.Lock()
		a.taskToAgent[e.TaskID] = agentID
		a.mu.Unlock()
		return nil

	case events.TaskStartedEvent:
		a.mu.Lock()
		agentID := a.taskToAgent[e.TaskID]
		a.agentDuration[agentID] = time.Now()
		a.totalRequests++
		a.mu.Unlock()

		// Return both task update and agent status update
		a.sendMsg(TaskUpdateMsg{
			TaskID: core.TaskID(e.TaskID),
			Status: core.TaskStatusRunning,
		})
		return AgentStatusUpdateMsg{
			AgentID: agentID,
			Status:  1, // Working
		}

	case events.TaskCompletedEvent:
		a.mu.Lock()
		agentID := a.taskToAgent[e.TaskID]
		startTime := a.agentDuration[agentID]
		duration := time.Since(startTime)
		a.mu.Unlock()

		// Return both task update and agent status update
		a.sendMsg(TaskUpdateMsg{
			TaskID: core.TaskID(e.TaskID),
			Status: core.TaskStatusCompleted,
		})
		return AgentStatusUpdateMsg{
			AgentID:  agentID,
			Status:   2, // Done
			Duration: duration,
		}

	case events.TaskFailedEvent:
		a.mu.Lock()
		agentID := a.taskToAgent[e.TaskID]
		startTime := a.agentDuration[agentID]
		duration := time.Since(startTime)
		a.mu.Unlock()

		a.sendMsg(TaskUpdateMsg{
			TaskID: core.TaskID(e.TaskID),
			Status: core.TaskStatusFailed,
			Error:  e.Error,
		})
		return AgentStatusUpdateMsg{
			AgentID:  agentID,
			Status:   3, // Error
			Duration: duration,
			Error:    e.Error,
		}

	case events.TaskSkippedEvent:
		return TaskUpdateMsg{
			TaskID: core.TaskID(e.TaskID),
			Status: core.TaskStatusSkipped,
			Error:  e.Reason,
		}

	case events.LogEvent:
		return LogMsg{
			Level:   e.Level,
			Message: e.Message,
		}

	case events.MetricsUpdateEvent:
		a.mu.Lock()
		requests := a.totalRequests
		a.mu.Unlock()

		// Send workflow progress update
		a.sendMsg(WorkflowProgressMsg{
			Title:      "workflow",
			Percentage: 0.5, // TODO: calculate from task progress
			Requests:   requests,
		})

		return MetricsUpdateMsg{
			TotalTokensIn:  e.TotalTokensIn,
			TotalTokensOut: e.TotalTokensOut,
			Duration:       e.Duration,
		}

	case events.AgentStreamEvent:
		return AgentEventMsg{
			Kind:    string(e.EventKind),
			Agent:   e.Agent,
			Message: e.Message,
			Data:    e.Data,
		}

	default:
		return nil
	}
}

// sendMsg sends a message to the channel without blocking.
func (a *EventBusAdapter) sendMsg(msg tea.Msg) {
	select {
	case a.msgCh <- msg:
	default:
		// Drop if full
	}
}

// normalizeAgentID normalizes agent names to standard IDs.
func normalizeAgentID(agent string) string {
	agent = strings.ToLower(agent)
	switch {
	case strings.Contains(agent, "claude"):
		return "claude"
	case strings.Contains(agent, "gemini"):
		return "gemini"
	case strings.Contains(agent, "codex"), strings.Contains(agent, "openai"), strings.Contains(agent, "gpt"):
		return "codex"
	default:
		return agent
	}
}

// eventError wraps an error string.
type eventError struct {
	message string
}

func (e *eventError) Error() string {
	return e.message
}
