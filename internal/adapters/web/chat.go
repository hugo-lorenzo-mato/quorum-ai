// Package web provides HTTP handlers for the web API.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/attachments"
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
	Title        string        `json:"title,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Agent        string        `json:"agent"`
	Model        string        `json:"model,omitempty"`
	MessageCount int           `json:"message_count"`
	Messages     []ChatMessage `json:"messages,omitempty"`
}

// SendMessageRequest is the request body for sending a chat message.
type SendMessageRequest struct {
	Content         string   `json:"content"`
	Agent           string   `json:"agent,omitempty"`
	Model           string   `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"` // none, minimal, low, medium, high, xhigh
	Attachments     []string `json:"attachments,omitempty"`      // File paths to include as context
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

// UpdateSessionRequest is the request body for updating a chat session.
type UpdateSessionRequest struct {
	Title *string `json:"title,omitempty"` // Use pointer to distinguish between empty and not provided
}

// ListSessionsResponse is the response for listing chat sessions.
type ListSessionsResponse struct {
	Sessions []ChatSession `json:"sessions"`
	Total    int           `json:"total"`
}

// ChatStoreResolver is a function that returns the ChatStore for the current request context.
// This allows for project-scoped chat storage.
type ChatStoreResolver func(ctx context.Context) core.ChatStore

// ProjectRootResolver is a function that returns the project root directory for the current request context.
type ProjectRootResolver func(ctx context.Context) string

// ChatHandler handles chat-related HTTP requests.
type ChatHandler struct {
	mu                  sync.RWMutex
	agents              core.AgentRegistry
	eventBus            *events.EventBus
	sessions            map[string]*chatSessionState
	loadedProjectRoots  map[string]bool // best-effort cache to avoid reloading persisted sessions repeatedly
	attachmentStore     *attachments.Store
	chatStore           core.ChatStore      // Fallback global store
	chatStoreResolver   ChatStoreResolver   // Per-request store resolver
	projectRootResolver ProjectRootResolver // Per-request project root resolver
}

// chatSessionState holds the internal state of a chat session.
type chatSessionState struct {
	session     ChatSession
	messages    []ChatMessage
	agent       string
	model       string
	title       string
	projectRoot string // Directory where .quorum is located, for file access scoping
}

// ChatHandlerOption is a functional option for configuring ChatHandler.
type ChatHandlerOption func(*ChatHandler)

// WithChatStoreResolver sets the ChatStore resolver for project-scoped storage.
func WithChatStoreResolver(resolver ChatStoreResolver) ChatHandlerOption {
	return func(h *ChatHandler) {
		h.chatStoreResolver = resolver
	}
}

// WithProjectRootResolver sets the project root resolver for project-scoped sessions.
func WithProjectRootResolver(resolver ProjectRootResolver) ChatHandlerOption {
	return func(h *ChatHandler) {
		h.projectRootResolver = resolver
	}
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(agents core.AgentRegistry, eventBus *events.EventBus, attachmentStore *attachments.Store, chatStore core.ChatStore, opts ...ChatHandlerOption) *ChatHandler {
	h := &ChatHandler{
		agents:             agents,
		eventBus:           eventBus,
		sessions:           make(map[string]*chatSessionState),
		loadedProjectRoots: make(map[string]bool),
		attachmentStore:    attachmentStore,
		chatStore:          chatStore,
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Load existing sessions from persistent store (only from global store at startup)
	if chatStore != nil {
		h.loadPersistedSessions()
	}

	return h
}

// ensureProjectSessionsLoaded loads persisted sessions for a project root into memory if needed.
// This is necessary because in multi-project mode, the ChatStore is resolved per request and cannot
// be loaded at process startup.
func (h *ChatHandler) ensureProjectSessionsLoaded(ctx context.Context, projectRoot string) {
	if projectRoot == "" {
		projectRoot = h.getProjectRoot(ctx)
	}

	h.mu.RLock()
	alreadyLoaded := h.loadedProjectRoots[projectRoot]
	h.mu.RUnlock()
	if alreadyLoaded {
		return
	}

	store := h.getChatStore(ctx)
	if store == nil {
		return
	}

	h.loadPersistedSessionsFromStore(ctx, store, projectRoot)

	h.mu.Lock()
	h.loadedProjectRoots[projectRoot] = true
	h.mu.Unlock()
}

// ensureSessionLoaded ensures a session exists in memory by loading it from the resolved ChatStore on-demand.
func (h *ChatHandler) ensureSessionLoaded(ctx context.Context, sessionID string) bool {
	h.mu.RLock()
	_, ok := h.sessions[sessionID]
	h.mu.RUnlock()
	if ok {
		return true
	}

	store := h.getChatStore(ctx)
	if store == nil {
		return false
	}

	sess, err := store.LoadSession(ctx, sessionID)
	if err != nil || sess == nil {
		return false
	}

	projectRoot := sess.ProjectRoot
	if projectRoot == "" {
		projectRoot = h.getProjectRoot(ctx)
	}

	messages, err := store.LoadMessages(ctx, sess.ID)
	if err != nil {
		// If we can't load messages, still expose the session.
		messages = nil
	}

	chatMessages := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMessages = append(chatMessages, ChatMessage{
			ID:        msg.ID,
			SessionID: msg.SessionID,
			Role:      msg.Role,
			Agent:     msg.Agent,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Tokens: &TokenInfo{
				Input:   msg.TokensIn,
				Output:  msg.TokensOut,
				CostUSD: msg.CostUSD,
			},
		})
	}

	state := &chatSessionState{
		session: ChatSession{
			ID:           sess.ID,
			Title:        sess.Title,
			CreatedAt:    sess.CreatedAt,
			UpdatedAt:    sess.UpdatedAt,
			Agent:        sess.Agent,
			Model:        sess.Model,
			MessageCount: len(chatMessages),
		},
		messages:    chatMessages,
		agent:       sess.Agent,
		model:       sess.Model,
		title:       sess.Title,
		projectRoot: projectRoot,
	}

	h.mu.Lock()
	// Re-check under lock to avoid overwriting newer in-memory state.
	if _, exists := h.sessions[sessionID]; !exists {
		h.sessions[sessionID] = state
	}
	h.mu.Unlock()

	return true
}

// getChatStore returns the ChatStore for the given context.
// If a resolver is configured and returns a non-nil store, that is used.
// Otherwise, falls back to the global chatStore.
func (h *ChatHandler) getChatStore(ctx context.Context) core.ChatStore {
	if h.chatStoreResolver != nil {
		if store := h.chatStoreResolver(ctx); store != nil {
			return store
		}
	}
	return h.chatStore
}

// getProjectRoot returns the project root for the given context.
// If a resolver is configured, uses that. Otherwise, uses current working directory.
func (h *ChatHandler) getProjectRoot(ctx context.Context) string {
	if h.projectRootResolver != nil {
		if root := h.projectRootResolver(ctx); root != "" {
			return root
		}
	}
	// Fallback to current working directory
	root, _ := os.Getwd()
	return root
}

// loadPersistedSessions loads all sessions from the persistent store.
func (h *ChatHandler) loadPersistedSessions() {
	ctx := context.Background()

	sessions, err := h.chatStore.ListSessions(ctx)
	if err != nil {
		// Log error but continue - sessions will be empty
		return
	}

	for _, sess := range sessions {
		messages, err := h.chatStore.LoadMessages(ctx, sess.ID)
		if err != nil {
			continue
		}

		// Convert persisted messages to ChatMessage
		chatMessages := make([]ChatMessage, 0, len(messages))
		for _, msg := range messages {
			chatMessages = append(chatMessages, ChatMessage{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Role:      msg.Role,
				Agent:     msg.Agent,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
				Tokens: &TokenInfo{
					Input:   msg.TokensIn,
					Output:  msg.TokensOut,
					CostUSD: msg.CostUSD,
				},
			})
		}

		h.sessions[sess.ID] = &chatSessionState{
			session: ChatSession{
				ID:           sess.ID,
				Title:        sess.Title,
				CreatedAt:    sess.CreatedAt,
				UpdatedAt:    sess.UpdatedAt,
				Agent:        sess.Agent,
				Model:        sess.Model,
				MessageCount: len(chatMessages),
			},
			messages:    chatMessages,
			agent:       sess.Agent,
			model:       sess.Model,
			title:       sess.Title,
			projectRoot: sess.ProjectRoot,
		}
	}
}

// loadPersistedSessionsFromStore loads sessions and their messages from the given store, filtering by project root.
// It merges results into the in-memory sessions map.
func (h *ChatHandler) loadPersistedSessionsFromStore(ctx context.Context, store core.ChatStore, projectRoot string) {
	sessions, err := store.ListSessions(ctx)
	if err != nil {
		return
	}

	for _, sess := range sessions {
		// Project isolation: if the session has a stored project root, enforce it.
		// If it is empty (older data), treat it as belonging to the current project.
		if sess.ProjectRoot != "" && sess.ProjectRoot != projectRoot {
			continue
		}
		if sess.ProjectRoot == "" {
			sess.ProjectRoot = projectRoot
		}

		messages, err := store.LoadMessages(ctx, sess.ID)
		if err != nil {
			continue
		}

		chatMessages := make([]ChatMessage, 0, len(messages))
		for _, msg := range messages {
			chatMessages = append(chatMessages, ChatMessage{
				ID:        msg.ID,
				SessionID: msg.SessionID,
				Role:      msg.Role,
				Agent:     msg.Agent,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
				Tokens: &TokenInfo{
					Input:   msg.TokensIn,
					Output:  msg.TokensOut,
					CostUSD: msg.CostUSD,
				},
			})
		}

		loaded := &chatSessionState{
			session: ChatSession{
				ID:           sess.ID,
				Title:        sess.Title,
				CreatedAt:    sess.CreatedAt,
				UpdatedAt:    sess.UpdatedAt,
				Agent:        sess.Agent,
				Model:        sess.Model,
				MessageCount: len(chatMessages),
			},
			messages:    chatMessages,
			agent:       sess.Agent,
			model:       sess.Model,
			title:       sess.Title,
			projectRoot: sess.ProjectRoot,
		}

		h.mu.Lock()
		existing, exists := h.sessions[sess.ID]
		if !exists {
			h.sessions[sess.ID] = loaded
		} else {
			// Prefer the version with more messages (covers "restart + restore" and multi-process cases).
			if len(existing.messages) < len(loaded.messages) || existing.session.UpdatedAt.Before(loaded.session.UpdatedAt) {
				h.sessions[sess.ID] = loaded
			}
		}
		h.mu.Unlock()
	}
}

// RegisterRoutes registers chat routes on the given router.
func (h *ChatHandler) RegisterRoutes(r chi.Router) {
	r.Route("/chat", func(r chi.Router) {
		// Session management
		r.Post("/sessions", h.CreateSession)
		r.Get("/sessions", h.ListSessions)
		r.Get("/sessions/{sessionID}", h.GetSession)
		r.Patch("/sessions/{sessionID}", h.UpdateSession)
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
	ctx := r.Context()

	// Get project root for file access scoping
	projectRoot := h.getProjectRoot(ctx)

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
		session:     session,
		messages:    make([]ChatMessage, 0),
		agent:       agent,
		model:       req.Model,
		projectRoot: projectRoot,
	}
	h.mu.Unlock()

	// Persist session to store
	chatStore := h.getChatStore(ctx)
	if chatStore != nil {
		persistedSession := &core.ChatSessionState{
			ID:          sessionID,
			CreatedAt:   now,
			UpdatedAt:   now,
			Agent:       agent,
			Model:       req.Model,
			ProjectRoot: projectRoot,
		}
		if err := chatStore.SaveSession(ctx, persistedSession); err != nil {
			// Log error but don't fail the request - session is already in memory
			_ = err
		}
	}

	// Publish event
	if h.eventBus != nil {
		h.eventBus.Publish(events.NewChatMessageEvent("", "", events.RoleSystem, "", fmt.Sprintf("Session %s created", sessionID)))
	}

	writeJSON(w, http.StatusCreated, session)
}

// ListSessions lists all chat sessions for the current project.
func (h *ChatHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectRoot := h.getProjectRoot(ctx)

	// Ensure persisted sessions for this project are available after restart.
	h.ensureProjectSessionsLoaded(ctx, projectRoot)

	h.mu.RLock()
	sessions := make([]ChatSession, 0)
	for _, state := range h.sessions {
		// Filter by project root for project isolation
		if state.projectRoot != projectRoot {
			continue
		}
		// Return session without messages for list view
		sess := state.session
		sess.MessageCount = len(state.messages)
		sessions = append(sessions, sess)
	}
	h.mu.RUnlock()

	// Return array directly for frontend compatibility
	writeJSON(w, http.StatusOK, sessions)
}

// GetSession returns a specific chat session.
func (h *ChatHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	if !h.ensureSessionLoaded(ctx, sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

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

// UpdateSession updates a chat session (e.g., title).
func (h *ChatHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	if !h.ensureSessionLoaded(ctx, sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	var req UpdateSessionRequest
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

	now := time.Now()
	updated := false

	// Update title if provided
	if req.Title != nil {
		state.title = *req.Title
		state.session.Title = *req.Title
		updated = true
	}

	if updated {
		state.session.UpdatedAt = now
	}

	// Copy session for response
	session := state.session
	session.MessageCount = len(state.messages)
	h.mu.Unlock()

	// Persist changes
	chatStore := h.getChatStore(ctx)
	if updated && chatStore != nil {
		persistedSession := &core.ChatSessionState{
			ID:          sessionID,
			Title:       state.title,
			CreatedAt:   state.session.CreatedAt,
			UpdatedAt:   now,
			Agent:       state.agent,
			Model:       state.model,
			ProjectRoot: state.projectRoot,
		}
		_ = chatStore.SaveSession(ctx, persistedSession)
	}

	writeJSON(w, http.StatusOK, session)
}

// DeleteSession deletes a chat session.
func (h *ChatHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	// If the session isn't in memory (e.g., after restart), try to load it so we can delete it.
	if !h.ensureSessionLoaded(ctx, sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

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

	// Delete from persistent store
	chatStore := h.getChatStore(ctx)
	if chatStore != nil {
		_ = chatStore.DeleteSession(ctx, sessionID)
	}

	// Best-effort cleanup of attachments on disk.
	if h.attachmentStore != nil {
		_ = h.attachmentStore.DeleteAll(attachments.OwnerChatSession, sessionID)
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetMessages returns messages for a chat session.
func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	if !h.ensureSessionLoaded(ctx, sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

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

	// Return array directly for frontend compatibility
	writeJSON(w, http.StatusOK, messages)
}

// SendMessage sends a message in a chat session and gets an agent response.
func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	ctx := r.Context()

	if !h.ensureSessionLoaded(ctx, sessionID) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

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

	// Persist user message
	chatStore := h.getChatStore(ctx)
	if chatStore != nil {
		persistedMsg := &core.ChatMessageState{
			ID:        userMsg.ID,
			SessionID: sessionID,
			Role:      userMsg.Role,
			Content:   userMsg.Content,
			Timestamp: userMsg.Timestamp,
		}
		_ = chatStore.SaveMessage(ctx, persistedMsg)
	}

	// Publish user message event
	if h.eventBus != nil {
		h.eventBus.Publish(events.NewChatMessageEvent("", "", events.RoleUser, "", req.Content))
	}

	// Execute agent with all options
	agentMsg, err := h.executeAgent(r.Context(), executeAgentOptions{
		sessionID:       sessionID,
		agentName:       agent,
		model:           model,
		reasoningEffort: req.ReasoningEffort,
		attachments:     req.Attachments,
		projectRoot:     state.projectRoot,
		history:         state.messages,
	})
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

	// Persist agent message (reuse chatStore from earlier in this function)
	if chatStore != nil {
		var tokensIn, tokensOut int
		var costUSD float64
		if agentMsg.Tokens != nil {
			tokensIn = agentMsg.Tokens.Input
			tokensOut = agentMsg.Tokens.Output
			costUSD = agentMsg.Tokens.CostUSD
		}
		persistedMsg := &core.ChatMessageState{
			ID:        agentMsg.ID,
			SessionID: sessionID,
			Role:      agentMsg.Role,
			Agent:     agentMsg.Agent,
			Content:   agentMsg.Content,
			Timestamp: agentMsg.Timestamp,
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
			CostUSD:   costUSD,
		}
		_ = chatStore.SaveMessage(ctx, persistedMsg)
	}

	// Return agent message directly for frontend compatibility
	// Frontend expects {id, content, timestamp} at top level
	writeJSON(w, http.StatusOK, agentMsg)
}

// executeAgentOptions contains options for executing an agent.
type executeAgentOptions struct {
	sessionID       string
	agentName       string
	model           string
	reasoningEffort string
	attachments     []string
	projectRoot     string
	history         []ChatMessage
}

// buildChatSystemPrompt creates a system prompt for chat interactions.
// This prompt encourages concise, direct responses without meta-commentary.
func buildChatSystemPrompt() string {
	return `## Response Guidelines
- Provide direct, concise answers to user questions
- Do NOT include self-identification statements unless explicitly asked "who are you"
- Do NOT explain technical limitations, access restrictions, or capabilities
- Do NOT show reasoning process unless explicitly requested
- Be helpful and accurate, but brief`
}

// executeAgent runs the message through the agent and returns the response.
func (h *ChatHandler) executeAgent(ctx context.Context, opts executeAgentOptions) (ChatMessage, error) {
	if h.agents == nil {
		return ChatMessage{}, fmt.Errorf("no agent registry configured")
	}

	agent, err := h.agents.Get(opts.agentName)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("agent %s not available: %w", opts.agentName, err)
	}

	// Build context from recent history
	contextBuilder := ""
	start := 0
	if len(opts.history) > 10 {
		start = len(opts.history) - 10
	}
	for _, msg := range opts.history[start:] {
		role := msg.Role
		if role == "agent" {
			role = "assistant"
		}
		contextBuilder += fmt.Sprintf("[%s]: %s\n\n", role, msg.Content)
	}

	// Get the last user message
	var lastContent string
	for i := len(opts.history) - 1; i >= 0; i-- {
		if opts.history[i].Role == "user" {
			lastContent = opts.history[i].Content
			break
		}
	}

	// Parse @file references from the message content
	fileRefs := parseFileReferences(lastContent)

	// Combine explicit attachments with @file references
	allFiles := make(map[string]bool)
	for _, path := range opts.attachments {
		allFiles[path] = true
	}
	for _, path := range fileRefs {
		allFiles[path] = true
	}

	// Build file context
	fileContext := ""
	if len(allFiles) > 0 {
		fileContext = "\n## Attached Files\n\n"
		for filePath := range allFiles {
			content, err := h.loadFileContent(filePath, opts.projectRoot)
			if err != nil {
				fileContext += fmt.Sprintf("[File: %s]\nError loading file: %s\n\n", filePath, err.Error())
			} else {
				fileContext += fmt.Sprintf("[File: %s]\n```\n%s\n```\n\n", filePath, content)
			}
		}
	}

	prompt := fmt.Sprintf(`You are in an interactive chat session.

## Conversation Context
%s
%s
## Current Message
%s

Respond helpfully and concisely.`, contextBuilder, fileContext, lastContent)

	execOpts := core.ExecuteOptions{
		Prompt:          prompt,
		SystemPrompt:    buildChatSystemPrompt(),
		Model:           opts.model,
		Format:          core.OutputFormatText,
		Phase:           core.PhaseExecute,
		ReasoningEffort: opts.reasoningEffort,
	}

	result, err := agent.Execute(ctx, execOpts)
	if err != nil {
		return ChatMessage{}, err
	}

	msg := ChatMessage{
		ID:        uuid.New().String(),
		SessionID: opts.sessionID,
		Role:      "agent",
		Agent:     opts.agentName,
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
		h.eventBus.Publish(events.NewChatMessageEvent("", "", events.RoleAgent, opts.agentName, result.Output))
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
	now := time.Now()
	state.session.UpdatedAt = now
	h.mu.Unlock()

	// Persist session update
	ctx := r.Context()
	chatStore := h.getChatStore(ctx)
	if chatStore != nil {
		persistedSession := &core.ChatSessionState{
			ID:          sessionID,
			CreatedAt:   state.session.CreatedAt,
			UpdatedAt:   now,
			Agent:       req.Agent,
			Model:       state.model,
			ProjectRoot: state.projectRoot,
		}
		_ = chatStore.SaveSession(ctx, persistedSession)
	}

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
	now := time.Now()
	state.session.UpdatedAt = now
	h.mu.Unlock()

	// Persist session update
	ctx := r.Context()
	chatStore := h.getChatStore(ctx)
	if chatStore != nil {
		persistedSession := &core.ChatSessionState{
			ID:          sessionID,
			CreatedAt:   state.session.CreatedAt,
			UpdatedAt:   now,
			Agent:       state.agent,
			Model:       req.Model,
			ProjectRoot: state.projectRoot,
		}
		_ = chatStore.SaveSession(ctx, persistedSession)
	}

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

// parseFileReferences extracts @file references from message content.
// It matches patterns like @path/to/file.ext and returns unique file paths.
func parseFileReferences(content string) []string {
	// Match @path/to/file.ext patterns (file must have an extension)
	re := regexp.MustCompile(`@([^\s@]+\.[a-zA-Z0-9]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	// Deduplicate results
	seen := make(map[string]bool)
	var result []string
	for _, match := range matches {
		if len(match) > 1 {
			path := match[1]
			if !seen[path] {
				seen[path] = true
				result = append(result, path)
			}
		}
	}
	return result
}

// loadFileContent loads a file's content, ensuring it's within the project root.
// Returns the file content as a string, or an error if the file cannot be read
// or is outside the allowed directory.
func (h *ChatHandler) loadFileContent(filePath, projectRoot string) (string, error) {
	// If no project root specified, use current directory
	if projectRoot == "" {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Security check: ensure the file is within the project root
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}

	// Resolve the file path
	var absPath string
	if filepath.IsAbs(filePath) {
		absPath = filepath.Clean(filePath)
	} else {
		absPath = filepath.Clean(filepath.Join(absProjectRoot, filePath))
	}

	relPath, err := filepath.Rel(absProjectRoot, absPath)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path is outside project directory")
	}

	root, err := os.OpenRoot(absProjectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to open project root: %w", err)
	}
	defer func() { _ = root.Close() }()

	// Check if file exists and is a regular file
	info, err := root.Stat(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", filePath)
		}
		return "", fmt.Errorf("failed to access file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Limit file size (10MB max)
	const maxFileSize = 10 * 1024 * 1024
	if info.Size() > maxFileSize {
		return "", fmt.Errorf("file too large (max 10MB): %s", filePath)
	}

	// Read file content
	content, err := root.ReadFile(relPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// ListAttachments lists all stored attachments for a chat session.
func (h *ChatHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	if h.attachmentStore == nil {
		writeError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	h.mu.RLock()
	_, exists := h.sessions[sessionID]
	h.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	atts, err := h.attachmentStore.List(attachments.OwnerChatSession, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list attachments")
		return
	}

	writeJSON(w, http.StatusOK, atts)
}

// UploadAttachments uploads one or more files as chat session attachments.
func (h *ChatHandler) UploadAttachments(w http.ResponseWriter, r *http.Request) {
	if h.attachmentStore == nil {
		writeError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	h.mu.RLock()
	_, exists := h.sessions[sessionID]
	h.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Limit total upload size (best-effort). Per-file limits are enforced by the store.
	r.Body = http.MaxBytesReader(w, r.Body, int64(attachments.MaxAttachmentSizeBytes)*10)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	form := r.MultipartForm
	if form == nil || len(form.File) == 0 || len(form.File["files"]) == 0 {
		writeError(w, http.StatusBadRequest, "no files provided")
		return
	}

	files := form.File["files"]
	saved := make([]core.Attachment, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to open uploaded file")
			return
		}
		att, err := h.attachmentStore.Save(attachments.OwnerChatSession, sessionID, f, fh.Filename)
		_ = f.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		saved = append(saved, att)
	}

	writeJSON(w, http.StatusCreated, saved)
}

// DownloadAttachment downloads a stored chat session attachment.
func (h *ChatHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	if h.attachmentStore == nil {
		writeError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	attachmentID := chi.URLParam(r, "attachmentID")
	if sessionID == "" || attachmentID == "" {
		writeError(w, http.StatusBadRequest, "session id and attachment id are required")
		return
	}

	h.mu.RLock()
	_, exists := h.sessions[sessionID]
	h.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	meta, absPath, err := h.attachmentStore.Resolve(attachments.OwnerChatSession, sessionID, attachmentID)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "attachment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve attachment")
		return
	}

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", meta.Name))
	http.ServeFile(w, r, absPath)
}

// DeleteAttachment deletes a stored chat session attachment.
func (h *ChatHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	if h.attachmentStore == nil {
		writeError(w, http.StatusServiceUnavailable, "attachments store not available")
		return
	}

	sessionID := chi.URLParam(r, "sessionID")
	attachmentID := chi.URLParam(r, "attachmentID")
	if sessionID == "" || attachmentID == "" {
		writeError(w, http.StatusBadRequest, "session id and attachment id are required")
		return
	}

	h.mu.RLock()
	_, exists := h.sessions[sessionID]
	h.mu.RUnlock()
	if !exists {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if err := h.attachmentStore.Delete(attachments.OwnerChatSession, sessionID, attachmentID); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "attachment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
