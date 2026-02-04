// Package sse provides Server-Sent Events handlers for streaming events to web clients.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// Handler streams events from the EventBus to connected SSE clients.
type Handler struct {
	bus           *events.EventBus
	mu            sync.RWMutex
	clients       map[*client]struct{}
	heartbeatFreq time.Duration
}

// client represents a connected SSE client.
type client struct {
	id       string
	done     chan struct{}
	events   chan []byte
	project  string // optional filter by project ID
	workflow string // optional filter by workflow ID
	closed   bool   // tracks if done channel is already closed
}

// NewHandler creates a new SSE handler connected to the given EventBus.
func NewHandler(bus *events.EventBus) *Handler {
	return &Handler{
		bus:           bus,
		clients:       make(map[*client]struct{}),
		heartbeatFreq: 30 * time.Second,
	}
}

// SetHeartbeatFrequency sets the interval between heartbeat messages.
func (h *Handler) SetHeartbeatFrequency(d time.Duration) {
	h.heartbeatFreq = d
}

// ServeHTTP implements http.Handler for SSE connections.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if we can flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Create client with optional project and workflow filters
	projectID := r.URL.Query().Get("project")
	workflowID := r.URL.Query().Get("workflow")
	c := &client{
		id:       fmt.Sprintf("%d", time.Now().UnixNano()),
		done:     make(chan struct{}),
		events:   make(chan []byte, 100),
		project:  projectID,
		workflow: workflowID,
	}

	// Register client
	h.addClient(c)
	defer h.removeClient(c)

	// Subscribe to EventBus
	eventCh := h.bus.Subscribe()
	defer h.bus.Unsubscribe(eventCh)

	// Send initial connection event
	h.sendEvent(w, flusher, "connected", map[string]string{
		"client_id": c.id,
		"project":   projectID,
		"workflow":  workflowID,
	})

	// Heartbeat ticker
	heartbeat := time.NewTicker(h.heartbeatFreq)
	defer heartbeat.Stop()

	// Main event loop
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-heartbeat.C:
			h.sendComment(w, flusher, "heartbeat")
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			// Filter by project if specified
			if c.project != "" && event.ProjectID() != c.project {
				continue
			}
			// Filter by workflow if specified
			if c.workflow != "" && event.WorkflowID() != c.workflow {
				continue
			}
			h.sendEvent(w, flusher, event.EventType(), event)
		case data := <-c.events:
			// Direct message to this client
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// sendEvent sends a typed SSE event.
func (h *Handler) sendEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
	flusher.Flush()
}

// sendComment sends an SSE comment (used for heartbeats).
func (h *Handler) sendComment(w http.ResponseWriter, flusher http.Flusher, comment string) {
	fmt.Fprintf(w, ": %s\n\n", comment)
	flusher.Flush()
}

// addClient registers a client.
func (h *Handler) addClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

// removeClient unregisters a client.
func (h *Handler) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	if !c.closed {
		c.closed = true
		close(c.done)
	}
}

// ClientCount returns the number of connected clients.
func (h *Handler) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected clients.
func (h *Handler) Broadcast(eventType string, data interface{}) {
	jsonData, err := json.Marshal(map[string]interface{}{
		"event": eventType,
		"data":  data,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		select {
		case c.events <- jsonData:
		default:
			// Client buffer full, skip
		}
	}
}

// Shutdown gracefully disconnects all clients.
func (h *Handler) Shutdown(_ context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for c := range h.clients {
		if !c.closed {
			c.closed = true
			close(c.done)
		}
	}
	h.clients = make(map[*client]struct{})
	return nil
}
