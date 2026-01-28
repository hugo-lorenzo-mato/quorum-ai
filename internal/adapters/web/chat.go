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
	ReasoningEffort string   `json:"reasoning_effort,omitempty"` // minimal, low, medium, high, xhigh
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
	store    *attachments.Store
}

// chatSessionState holds the internal state of a chat session.
type chatSessionState struct {
	session     ChatSession
	messages    []ChatMessage
	agent       string
	model       string
	projectRoot string // Directory where .quorum is located, for file access scoping
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(agents core.AgentRegistry, eventBus *events.EventBus, store *attachments.Store) *ChatHandler {
	return &ChatHandler{
		agents:   agents,
		eventBus: eventBus,
		sessions: make(map[string]*chatSessionState),
		store:    store,
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

	// Get project root for file access scoping
	projectRoot, _ := os.Getwd()

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

	// Return array directly for frontend compatibility
	writeJSON(w, http.StatusOK, sessions)
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

	// Best-effort cleanup of attachments on disk.
	if h.store != nil {
		_ = h.store.DeleteAll(attachments.OwnerChatSession, sessionID)
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

	// Return array directly for frontend compatibility
	writeJSON(w, http.StatusOK, messages)
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
		h.eventBus.Publish(events.NewChatMessageEvent("", events.RoleAgent, opts.agentName, result.Output))
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

	// Resolve the file path
	var absPath string
	if filepath.IsAbs(filePath) {
		absPath = filepath.Clean(filePath)
	} else {
		absPath = filepath.Clean(filepath.Join(projectRoot, filePath))
	}

	// Security check: ensure the file is within the project root
	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}

	if !strings.HasPrefix(absPath, absProjectRoot) {
		return "", fmt.Errorf("file path is outside project directory")
	}

	// Check if file exists and is a regular file
	info, err := os.Stat(absPath)
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
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// ListAttachments lists all stored attachments for a chat session.
func (h *ChatHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
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

	atts, err := h.store.List(attachments.OwnerChatSession, sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list attachments")
		return
	}

	writeJSON(w, http.StatusOK, atts)
}

// UploadAttachments uploads one or more files as chat session attachments.
func (h *ChatHandler) UploadAttachments(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
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
		att, err := h.store.Save(attachments.OwnerChatSession, sessionID, f, fh.Filename)
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
	if h.store == nil {
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

	meta, absPath, err := h.store.Resolve(attachments.OwnerChatSession, sessionID, attachmentID)
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
	if h.store == nil {
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

	if err := h.store.Delete(attachments.OwnerChatSession, sessionID, attachmentID); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "attachment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete attachment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
