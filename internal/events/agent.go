package events

import "time"

// Event type constants for agent streaming events.
const (
	TypeAgentEvent = "agent_event"
)

// AgentEventType defines the type of agent event.
type AgentEventType string

const (
	// AgentStarted indicates the agent execution has begun.
	AgentStarted AgentEventType = "started"

	// AgentToolUse indicates the agent is using a tool.
	AgentToolUse AgentEventType = "tool_use"

	// AgentThinking indicates the agent is reasoning/thinking.
	AgentThinking AgentEventType = "thinking"

	// AgentChunk indicates a partial response chunk was received.
	AgentChunk AgentEventType = "chunk"

	// AgentProgress indicates general progress (e.g., "Analyzing...").
	AgentProgress AgentEventType = "progress"

	// AgentCompleted indicates the agent execution has finished successfully.
	AgentCompleted AgentEventType = "completed"

	// AgentError indicates an error occurred during execution.
	AgentError AgentEventType = "error"
)

// AgentStreamEvent represents a real-time streaming event from an agent.
type AgentStreamEvent struct {
	BaseEvent
	EventKind AgentEventType         `json:"event_kind"`
	Agent     string                 `json:"agent"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	EventTime time.Time              `json:"event_time"`
}

// NewAgentStreamEvent creates a new agent stream event.
func NewAgentStreamEvent(workflowID, projectID string, kind AgentEventType, agent, message string) AgentStreamEvent {
	return NewAgentStreamEventAt(time.Now(), workflowID, projectID, kind, agent, message)
}

// NewAgentStreamEventAt creates an agent stream event with a caller-supplied timestamp.
// This ensures SSE and persistence paths share the exact same timestamp, preventing
// frontend deduplication mismatches.
func NewAgentStreamEventAt(ts time.Time, workflowID, projectID string, kind AgentEventType, agent, message string) AgentStreamEvent {
	return AgentStreamEvent{
		BaseEvent: BaseEvent{
			Type:     TypeAgentEvent,
			Time:     ts,
			Workflow: workflowID,
			Project:  projectID,
		},
		EventKind: kind,
		Agent:     agent,
		Message:   message,
		EventTime: ts,
	}
}

// WithData adds additional data to the event.
func (e AgentStreamEvent) WithData(data map[string]interface{}) AgentStreamEvent {
	e.Data = data
	return e
}
