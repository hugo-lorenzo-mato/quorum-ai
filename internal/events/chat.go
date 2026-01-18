package events

import "time"

// Chat and user input event type constants.
const (
	TypeUserInputRequested  = "user_input_requested"
	TypeUserInputProvided   = "user_input_provided"
	TypeChatMessageReceived = "chat_message_received"
	TypeAgentResponseChunk  = "agent_response_chunk"
)

// MessageRole represents the role of a chat message sender.
type MessageRole string

const (
	RoleUser   MessageRole = "user"
	RoleAgent  MessageRole = "agent"
	RoleSystem MessageRole = "system"
)

// UserInputRequestedEvent is emitted when the workflow needs user input.
type UserInputRequestedEvent struct {
	BaseEvent
	RequestID string        `json:"request_id"`
	Prompt    string        `json:"prompt"`
	Context   string        `json:"context,omitempty"`
	Options   []string      `json:"options,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
}

// NewUserInputRequestedEvent creates a new user input request event.
func NewUserInputRequestedEvent(workflowID, requestID, prompt string, options []string) UserInputRequestedEvent {
	return UserInputRequestedEvent{
		BaseEvent: NewBaseEvent(TypeUserInputRequested, workflowID),
		RequestID: requestID,
		Prompt:    prompt,
		Options:   options,
	}
}

// UserInputProvidedEvent is emitted when the user provides input.
type UserInputProvidedEvent struct {
	BaseEvent
	RequestID string `json:"request_id"`
	Input     string `json:"input"`
	Cancelled bool   `json:"cancelled"`
}

// NewUserInputProvidedEvent creates a new user input provided event.
func NewUserInputProvidedEvent(workflowID, requestID, input string, cancelled bool) UserInputProvidedEvent {
	return UserInputProvidedEvent{
		BaseEvent: NewBaseEvent(TypeUserInputProvided, workflowID),
		RequestID: requestID,
		Input:     input,
		Cancelled: cancelled,
	}
}

// ChatMessageReceivedEvent is emitted for chat messages.
type ChatMessageReceivedEvent struct {
	BaseEvent
	Role      MessageRole `json:"role"`
	Agent     string      `json:"agent,omitempty"`
	Content   string      `json:"content"`
	MessageID string      `json:"message_id,omitempty"`
}

// NewChatMessageEvent creates a new chat message event.
func NewChatMessageEvent(workflowID string, role MessageRole, agent, content string) ChatMessageReceivedEvent {
	return ChatMessageReceivedEvent{
		BaseEvent: NewBaseEvent(TypeChatMessageReceived, workflowID),
		Role:      role,
		Agent:     agent,
		Content:   content,
	}
}

// AgentResponseChunkEvent is emitted for streaming agent responses.
type AgentResponseChunkEvent struct {
	BaseEvent
	Agent   string `json:"agent"`
	Chunk   string `json:"chunk"`
	IsFinal bool   `json:"is_final"`
}

// NewAgentResponseChunkEvent creates a new agent response chunk event.
func NewAgentResponseChunkEvent(workflowID, agent, chunk string, isFinal bool) AgentResponseChunkEvent {
	return AgentResponseChunkEvent{
		BaseEvent: NewBaseEvent(TypeAgentResponseChunk, workflowID),
		Agent:     agent,
		Chunk:     chunk,
		IsFinal:   isFinal,
	}
}
