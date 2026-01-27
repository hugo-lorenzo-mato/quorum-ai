package core

import "time"

// =============================================================================
// Agent Streaming Events (Real-time visibility into agent execution)
// =============================================================================

// AgentEventType defines the type of event emitted during agent execution.
type AgentEventType string

const (
	// AgentEventStarted indicates the agent execution has begun.
	AgentEventStarted AgentEventType = "started"

	// AgentEventToolUse indicates the agent is using a tool.
	AgentEventToolUse AgentEventType = "tool_use"

	// AgentEventThinking indicates the agent is reasoning/thinking.
	AgentEventThinking AgentEventType = "thinking"

	// AgentEventChunk indicates a partial response chunk was received.
	AgentEventChunk AgentEventType = "chunk"

	// AgentEventProgress indicates general progress (e.g., "Analyzing...").
	AgentEventProgress AgentEventType = "progress"

	// AgentEventCompleted indicates the agent execution has finished successfully.
	AgentEventCompleted AgentEventType = "completed"

	// AgentEventError indicates an error occurred during execution.
	AgentEventError AgentEventType = "error"
)

// AgentEvent represents a real-time event from an agent during execution.
// These events provide visibility into what the agent is doing before
// the final result is returned. Also used for persistence in WorkflowState.
type AgentEvent struct {
	// ID is a unique identifier for the event (used for persistence)
	ID string `json:"id,omitempty"`

	// Type is the kind of event (started, tool_use, completed, etc.)
	Type AgentEventType `json:"event_kind"`

	// Agent is the name of the agent emitting the event ("claude", "gemini", etc.)
	Agent string `json:"agent"`

	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Message is a human-readable description of the event
	Message string `json:"message"`

	// Data contains optional structured data specific to the event type.
	// For tool_use: {"tool": "read_file", "args": {...}}
	// For completed: {"tokens_in": 1000, "tokens_out": 500, "cost_usd": 0.01}
	// For error: {"code": "TIMEOUT", "details": "..."}
	Data map[string]any `json:"data,omitempty"`
}

// NewAgentEvent creates a new agent event with the current timestamp.
func NewAgentEvent(eventType AgentEventType, agent, message string) AgentEvent {
	return AgentEvent{
		Type:      eventType,
		Agent:     agent,
		Timestamp: time.Now(),
		Message:   message,
	}
}

// WithData adds structured data to the event.
func (e AgentEvent) WithData(data map[string]any) AgentEvent {
	e.Data = data
	return e
}

// AgentEventHandler is a callback function that receives agent events.
type AgentEventHandler func(event AgentEvent)

// StreamingCapable is implemented by agents that support real-time event streaming.
// This is an optional interface - agents that don't support streaming will still
// work, but won't provide real-time visibility into their execution.
type StreamingCapable interface {
	// SetEventHandler sets the handler that will receive streaming events.
	// Pass nil to disable event streaming.
	SetEventHandler(handler AgentEventHandler)
}
