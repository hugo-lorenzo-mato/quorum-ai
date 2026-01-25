package sse

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/events"
)

// RegisterRoutes registers the SSE handler on the given chi router.
// The handler will be available at the specified path (e.g., "/api/events").
func RegisterRoutes(r chi.Router, bus *events.EventBus) *Handler {
	h := NewHandler(bus)
	r.Get("/events", h.ServeHTTP)
	return h
}

// RegisterRoutesWithHandler registers a pre-configured handler.
func RegisterRoutesWithHandler(r chi.Router, h *Handler) {
	r.Get("/events", h.ServeHTTP)
}

// HandlerFunc returns the SSE handler as http.HandlerFunc.
// Useful for non-chi routers.
func (h *Handler) HandlerFunc() http.HandlerFunc {
	return h.ServeHTTP
}
