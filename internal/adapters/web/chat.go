// Package web provides HTTP handlers for the web API.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Role      string     `json:"role"` // "user", "agent", "system"
	Agent     string     `json:"agent,omitempty"`
	Content   string     `json:"content"`
	Timestamp time.Time  `json:"timestamp"`
	Tokens    *TokenInfo `json:"tokens,omitempty"`
}

// TokenInfo contains token usage information.
type TokenInfo struct {
	Input   int     `json:"input"`
	Output  int     `json:"output"`
	CostUSD float64 `json:"cost_usd,omitempty"`
}

// ChatSession represents a chat session.
type ChatSession struct {
	ID           string        `json:"id"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Agent        string        `json:"agent"`
	Model        string        `json:"model,omitempty"`
	MessageCount int           `json:"message_count"`
	Messages     []ChatMessage `json:"messages,omitempty"`
}

// SendMessageRequest is the request body for sending a chat message.
type SendMessageRequest struct {
	Content string `json:"content"`
	Agent   string `json:"agent,omitempty"`
	Model   string `json:"model,omitempty"`
}

// SendMessageResponse is the response for sending a chat message.
type SendMessageResponse struct {
	UserMessage  ChatMessage `json:"user_message"`
	AgentMessage ChatMessage `json:"agent_message"`
}

// CreateSessionRequest is the request body for creating a chat session.
type CreateSessionRequest struct {
	Agent string `json:"agent,omitempty"`
	Model string `json:"model,omitempty"`
}

// ListSessionsResponse is the response for listing chat sessions.
type ListSessionsResponse struct {
	Sessions []ChatSession `json:"sessions"`
	Total    int           `json:"total"`
}

// ChatHandler handles chat-related HTTP requests.
type ChatHandler struct {
	mu       sync.RWMutex
	agents   core.AgentRegistry
	eventBus *events.EventBus
	sessions map[string]*chatSessionState
}

// chatSessionState holds the internal state of a chat session.
type chatSessionState struct {
	session  ChatSession
	messages []ChatMessage
	agent    string
	model    string
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(agents core.AgentRegistry, eventBus *events.EventBus) *ChatHandler {
	return &ChatHandler{
		agents:   agents,
		eventBus: eventBus,
		sessions: make(map[string]*chatSessionState),
	}
}

// RegisterRoutes registers chat routes on the given router.
func (h *ChatHandler) RegisterRoutes(r chi.Router) {
	r.Route("/chat", func(r chi.Router) {
		// Session management
		r.Post("/sessions", h.CreateSession)
		r.Get("/sessions", h.ListSessions)
		r.Get("/sessions/{sessionID}", h.GetSession)
		r.Delete("/sessions/{sessionID}", h.DeleteSession)

		// Messages
		r.Get("/sessions/{sessionID}/messages", h.GetMessages)
		r.Post("/sessions/{sessionID}/messages", h.SendMessage)

		// Session settings
		r.Put("/sessions/{sessionID}/agent", h.SetAgent)
		r.Put("/sessions/{sessionID}/model", h.SetModel)
	})
}

// CreateSession creates a new chat session.
func (h *ChatHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK - use defaults
		req = CreateSessionRequest{}
	}

	// Default to claude if no agent specified
	agent := req.Agent
	if agent == "" {
		agent = "claude"
	}

	// Validate agent exists
	if h.agents != nil {
		if _, err := h.agents.Get(agent); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid agent: %s", agent))
			return
		}
	}

	now := time.Now()
	sessionID := uuid.New().String()

	session := ChatSession{
		ID:           sessionID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Agent:        agent,
		Model:        req.Model,
		MessageCount: 0,
	}

	h.mu.Lock()
	h.sessions[sessionID] = &chatSessionState{
		session:  session,
		messages: make([]ChatMessage, 0),
		agent:    agent,
		model:    req.Model,
	}
	h.mu.Unlock()

	// Publish event
	if h.eventBus != nil {
		h.eventBus.Publish(events.NewChatMessageEvent("", events.RoleSystem, "", fmt.Sprintf("Session %s created", sessionID)))
	}

	writeJSON(w, http.StatusCreated, session)
}

// ListSessions lists all chat sessions.
func (h *ChatHandler) ListSessions(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	sessions := make([]ChatSession, 0, len(h.sessions))
	for _, state := range h.sessions {
		// Return session without messages for list view
		sess := state.session
		sess.MessageCount = len(state.messages)
		sessions = append(sessions, sess)
	}
	h.mu.RUnlock()

	writeJSON(w, http.StatusOK, ListSessionsResponse{
		Sessions: sessions,
		Total:    len(sessions),
	})
}

// GetSession returns a specific chat session.
func (h *ChatHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	state, exists := h.sessions[sessionID]
	h.mu.RUnlock()

	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Include messages in full session view
	session := state.session
	session.Messages = state.messages
	session.MessageCount = len(state.messages)

	writeJSON(w, http.StatusOK, session)
}

// DeleteSession deletes a chat session.
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.Lock()
	_, exists := h.sessions[sessionID]
	if exists {
		delete(h.sessions, sessionID)
	}
	h.mu.Unlock()

	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMessages returns messages for a chat session.
func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	state, exists := h.sessions[sessionID]
	if !exists {
		h.mu.RUnlock()
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	messages := make([]ChatMessage, len(state.messages))
	copy(messages, state.messages)
	h.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
		"total":    len(messages),
	})
}

// SendMessage sends a message in a chat session and gets an agent response.
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	h.mu.Lock()
	state, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Use request agent/model if provided, otherwise use session defaults
	agent := req.Agent
	if agent == "" {
		agent = state.agent
	}
	model := req.Model
	if model == "" {
		model = state.model
	}

	// Create user message
	now := time.Now()
	userMsg := ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Content,
		Timestamp: now,
	}
	state.messages = append(state.messages, userMsg)
	state.session.UpdatedAt = now

	h.mu.Unlock()

	// Publish user message event
	if h.eventBus != nil {
		h.eventBus.Publish(events.NewChatMessageEvent("", events.RoleUser, "", req.Content))
	}

	// Execute agent
	agentMsg, err := h.executeAgent(r.Context(), sessionID, agent, model, state.messages)
	if err != nil {
		// Add error as system message
		errMsg := ChatMessage{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Role:      "system",
			Content:   fmt.Sprintf("Error: %v", err),
			Timestamp: time.Now(),
		}
		h.mu.Lock()
		state.messages = append(state.messages, errMsg)
		h.mu.Unlock()

		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add agent message to session
	h.mu.Lock()
	state.messages = append(state.messages, agentMsg)
	state.session.UpdatedAt = agentMsg.Timestamp
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, SendMessageResponse{
		UserMessage:  userMsg,
		AgentMessage: agentMsg,
	})
}

// executeAgent runs the message through the agent and returns the response.
func (h *ChatHandler) executeAgent(ctx context.Context, sessionID, agentName, model string, history []ChatMessage) (ChatMessage, error) {
	if h.agents == nil {
		return ChatMessage{}, fmt.Errorf("no agent registry configured")
	}

	agent, err := h.agents.Get(agentName)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("agent %s not available: %w", agentName, err)
	}

	// Build context from recent history
	contextBuilder := ""
	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}
	for _, msg := range history[start:] {
		role := msg.Role
		if role == "agent" {
			role = "assistant"
		}
		contextBuilder += fmt.Sprintf("[%s]: %s\n\n", role, msg.Content)
	}

	// Get the last user message
	var lastContent string
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			lastContent = history[i].Content
			break
		}
	}

	prompt := fmt.Sprintf(`You are in an interactive chat session.

## Conversation Context
%s

## Current Message
%s

Respond helpfully and concisely.`, contextBuilder, lastContent)

	opts := core.ExecuteOptions{
		Prompt: prompt,
		Model:  model,
		Format: core.OutputFormatText,
		Phase:  core.PhaseExecute,
	}

	result, err := agent.Execute(ctx, opts)
	if err != nil {
		return ChatMessage{}, err
	}

	msg := ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "agent",
		Agent:     agentName,
		Content:   result.Output,
		Timestamp: time.Now(),
		Tokens: &TokenInfo{
			Input:   result.TokensIn,
			Output:  result.TokensOut,
			CostUSD: result.CostUSD,
		},
	}

	// Publish agent response event
	if h.eventBus != nil {
		h.eventBus.Publish(events.NewChatMessageEvent("", events.RoleAgent, agentName, result.Output))
	}

	return msg, nil
}

// SetAgent updates the agent for a chat session.
func (h *ChatHandler) SetAgent(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req struct {
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Agent == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}

	// Validate agent exists
	if h.agents != nil {
		if _, err := h.agents.Get(req.Agent); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid agent: %s", req.Agent))
			return
		}
	}

	h.mu.Lock()
	state, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	state.agent = req.Agent
	state.session.Agent = req.Agent
	state.session.UpdatedAt = time.Now()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{
		"agent": req.Agent,
	})
}

// SetModel updates the model for a chat session.
func (h *ChatHandler) SetModel(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var req struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.mu.Lock()
	state, exists := h.sessions[sessionID]
	if !exists {
		h.mu.Unlock()
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	state.model = req.Model
	state.session.Model = req.Model
	state.session.UpdatedAt = time.Now()
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{
		"model": req.Model,
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error but response is already committed
		_ = err
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
