package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MessageRole represents the role of a chat message sender.
type MessageRole string

const (
	RoleUser   MessageRole = "user"
	RoleAgent  MessageRole = "agent"
	RoleSystem MessageRole = "system"
)

// Message represents a single chat message.
type Message struct {
	ID        string                 `json:"id"`
	Role      MessageRole            `json:"role"`
	Agent     string                 `json:"agent,omitempty"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      RoleUser,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewAgentMessage creates a new agent message.
func NewAgentMessage(agent, content string) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      RoleAgent,
		Agent:     agent,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      RoleSystem,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewSystemBubbleMessage creates a system message that should render like a bubble.
func NewSystemBubbleMessage(content string) Message {
	return Message{
		ID:        uuid.New().String(),
		Role:      RoleSystem,
		Content:   content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"bubble": true,
		},
	}
}

// ConversationHistory manages a thread-safe conversation history.
type ConversationHistory struct {
	mu       sync.RWMutex
	messages []Message
	maxSize  int
}

// NewConversationHistory creates a new conversation history with a maximum size.
func NewConversationHistory(maxSize int) *ConversationHistory {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ConversationHistory{
		messages: make([]Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Add adds a message to the history, removing oldest if at capacity.
func (h *ConversationHistory) Add(msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.messages) >= h.maxSize {
		// Remove oldest message
		h.messages = h.messages[1:]
	}
	h.messages = append(h.messages, msg)
}

// All returns all messages in the history.
func (h *ConversationHistory) All() []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]Message, len(h.messages))
	copy(result, h.messages)
	return result
}

// Last returns the last n messages.
func (h *ConversationHistory) Last(n int) []Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if n <= 0 || n > len(h.messages) {
		n = len(h.messages)
	}

	start := len(h.messages) - n
	result := make([]Message, n)
	copy(result, h.messages[start:])
	return result
}

// Context formats the last n messages for use as prompt context.
func (h *ConversationHistory) Context(n int) string {
	messages := h.Last(n)
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			sb.WriteString(fmt.Sprintf("User: %s\n\n", msg.Content))
		case RoleAgent:
			if msg.Agent != "" {
				sb.WriteString(fmt.Sprintf("%s: %s\n\n", msg.Agent, msg.Content))
			} else {
				sb.WriteString(fmt.Sprintf("Assistant: %s\n\n", msg.Content))
			}
		case RoleSystem:
			sb.WriteString(fmt.Sprintf("[System: %s]\n\n", msg.Content))
		}
	}

	return strings.TrimSpace(sb.String())
}

// Len returns the number of messages in the history.
func (h *ConversationHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages)
}

// Clear removes all messages from the history.
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = h.messages[:0]
}

// LastMessage returns the most recent message, or nil if empty.
func (h *ConversationHistory) LastMessage() *Message {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.messages) == 0 {
		return nil
	}
	msg := h.messages[len(h.messages)-1]
	return &msg
}
