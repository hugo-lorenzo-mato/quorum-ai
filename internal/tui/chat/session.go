package chat

import (
	"context"
	"fmt"
	"sync"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/control"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ChatSession manages a chat session with workflow integration.
type ChatSession struct {
	mu sync.RWMutex

	// Components
	agents       core.AgentRegistry
	controlPlane *control.ControlPlane
	eventBus     *events.EventBus

	// Conversation
	history      *ConversationHistory
	currentAgent string
	currentModel string

	// Callbacks
	onMessage        func(Message)
	onWorkflowUpdate func(*core.WorkflowState)
}

// NewChatSession creates a new chat session.
func NewChatSession(agents core.AgentRegistry, cp *control.ControlPlane, bus *events.EventBus) *ChatSession {
	return &ChatSession{
		agents:       agents,
		controlPlane: cp,
		eventBus:     bus,
		history:      NewConversationHistory(100),
		currentAgent: "claude",
	}
}

// SendMessage sends a message to the current agent and gets a response.
func (s *ChatSession) SendMessage(ctx context.Context, content string) error {
	s.mu.Lock()
	agent := s.currentAgent
	model := s.currentModel
	s.mu.Unlock()

	// Add user message to history
	userMsg := NewUserMessage(content)
	s.history.Add(userMsg)

	if s.onMessage != nil {
		s.onMessage(userMsg)
	}

	// Get agent
	if s.agents == nil {
		return fmt.Errorf("no agent registry configured")
	}

	agentImpl, err := s.agents.Get(agent)
	if err != nil {
		return fmt.Errorf("failed to get agent %s: %w", agent, err)
	}

	// Build context from conversation history
	conversationContext := s.history.Context(10)

	// Create prompt with context
	prompt := fmt.Sprintf(`You are in an interactive chat session.

## Conversation Context
%s

## Current Message
%s

Respond helpfully and concisely.`, conversationContext, content)

	// Execute
	opts := core.ExecuteOptions{
		Prompt: prompt,
		Model:  model,
		Format: core.OutputFormatText,
		Phase:  core.PhaseExecute,
	}

	result, err := agentImpl.Execute(ctx, opts)
	if err != nil {
		errMsg := NewSystemMessage(fmt.Sprintf("Error: %v", err))
		s.history.Add(errMsg)
		if s.onMessage != nil {
			s.onMessage(errMsg)
		}
		return err
	}

	// Add agent response to history
	agentMsg := NewAgentMessage(agent, result.Output)
	s.history.Add(agentMsg)

	if s.onMessage != nil {
		s.onMessage(agentMsg)
	}

	// Publish event
	if s.eventBus != nil {
		s.eventBus.Publish(events.NewChatMessageEvent("", "", events.RoleAgent, agent, result.Output))
	}

	return nil
}

// History returns the conversation history.
func (s *ChatSession) History() *ConversationHistory {
	return s.history
}

// SetAgent sets the current agent.
func (s *ChatSession) SetAgent(agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentAgent = agent
}

// SetModel sets the current model.
func (s *ChatSession) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentModel = model
}

// GetAgent returns the current agent.
func (s *ChatSession) GetAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentAgent
}

// GetModel returns the current model.
func (s *ChatSession) GetModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentModel
}

// OnMessage sets a callback for new messages.
func (s *ChatSession) OnMessage(fn func(Message)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMessage = fn
}

// OnWorkflowUpdate sets a callback for workflow updates.
func (s *ChatSession) OnWorkflowUpdate(fn func(*core.WorkflowState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onWorkflowUpdate = fn
}

// CancelWorkflow cancels any active workflow.
func (s *ChatSession) CancelWorkflow() error {
	if s.controlPlane == nil {
		return fmt.Errorf("no control plane configured")
	}
	s.controlPlane.Cancel()
	return nil
}

// PauseWorkflow pauses the active workflow.
func (s *ChatSession) PauseWorkflow() {
	if s.controlPlane != nil {
		s.controlPlane.Pause()
	}
}

// ResumeWorkflow resumes a paused workflow.
func (s *ChatSession) ResumeWorkflow() {
	if s.controlPlane != nil {
		s.controlPlane.Resume()
	}
}

// RetryTask queues a task for retry.
func (s *ChatSession) RetryTask(taskID core.TaskID) error {
	if s.controlPlane == nil {
		return fmt.Errorf("no control plane configured")
	}
	s.controlPlane.RetryTask(taskID)
	return nil
}

// Close cleans up the session.
func (s *ChatSession) Close() {
	// Cleanup if needed
}
