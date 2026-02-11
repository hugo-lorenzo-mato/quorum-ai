package api

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
)

func (s *Server) handleListSystemPrompts(w http.ResponseWriter, _ *http.Request) {
	prompts, err := service.ListSystemPrompts()
	if err != nil {
		s.logger.Error("failed to list system prompts", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list system prompts")
		return
	}
	respondJSON(w, http.StatusOK, prompts)
}

func (s *Server) handleGetSystemPrompt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	prompt, err := service.GetSystemPrompt(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			respondError(w, http.StatusNotFound, "system prompt not found")
			return
		}
		s.logger.Error("failed to get system prompt", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get system prompt")
		return
	}
	respondJSON(w, http.StatusOK, prompt)
}
