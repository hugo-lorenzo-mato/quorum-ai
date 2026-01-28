// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
)

// ValidationErrorResponse represents validation errors for the API.
type ValidationErrorResponse struct {
	Message string                 `json:"message"`
	Errors  []ValidationFieldError `json:"errors"`
}

// ValidationFieldError represents a single field validation error.
type ValidationFieldError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value"`
	Message string      `json:"message"`
	Code    string      `json:"code"`
}

// ValidationResult represents the result of a validation check.
type ValidationResult struct {
	Valid  bool                   `json:"valid"`
	Errors []ValidationFieldError `json:"errors"`
}

// Error code constants.
const (
	ErrCodeRequired        = "REQUIRED"
	ErrCodeInvalidEnum     = "INVALID_ENUM"
	ErrCodeInvalidDuration = "INVALID_DURATION"
	ErrCodeInvalidRange    = "INVALID_RANGE"
	ErrCodeInvalidPath     = "INVALID_PATH"
	ErrCodeDependencyChain = "DEPENDENCY_CHAIN"
	ErrCodeMutualExclusion = "MUTUAL_EXCLUSION"
	ErrCodeAgentNotEnabled = "AGENT_NOT_ENABLED"
	ErrCodeUnknownAgent    = "UNKNOWN_AGENT"
	ErrCodeUnknownPhase    = "UNKNOWN_PHASE"
)

// convertValidationErrors converts config.ValidationErrors to API response format.
func convertValidationErrors(errs config.ValidationErrors) ValidationErrorResponse {
	response := ValidationErrorResponse{
		Message: "Configuration validation failed",
		Errors:  make([]ValidationFieldError, 0, len(errs)),
	}

	for _, err := range errs {
		fieldError := ValidationFieldError{
			Field:   err.Field,
			Value:   err.Value,
			Message: err.Message,
			Code:    inferErrorCode(err.Field, err.Message),
		}
		response.Errors = append(response.Errors, fieldError)
	}

	return response
}

// inferErrorCode determines the error code from the error message.
func inferErrorCode(_, message string) string {
	msg := strings.ToLower(message)

	switch {
	case strings.Contains(msg, "required"):
		return ErrCodeRequired
	case strings.Contains(msg, "must be one of"):
		return ErrCodeInvalidEnum
	case strings.Contains(msg, "invalid duration"):
		return ErrCodeInvalidDuration
	case strings.Contains(msg, "must be between") || strings.Contains(msg, "must be >=") || strings.Contains(msg, "must be positive"):
		return ErrCodeInvalidRange
	case strings.Contains(msg, "invalid") && strings.Contains(msg, "path"):
		return ErrCodeInvalidPath
	case strings.Contains(msg, "requires"):
		return ErrCodeDependencyChain
	case strings.Contains(msg, "cannot be true when"):
		return ErrCodeMutualExclusion
	case strings.Contains(msg, "must be enabled"):
		return ErrCodeAgentNotEnabled
	case strings.Contains(msg, "unknown agent"):
		return ErrCodeUnknownAgent
	case strings.Contains(msg, "unknown phase"):
		return ErrCodeUnknownPhase
	default:
		return "VALIDATION_ERROR"
	}
}

// validateConfig validates a typed config and returns errors if any.
// Returns true if validation passed, false otherwise (response already written).
func validateConfig(w http.ResponseWriter, cfg *config.Config, logger *slog.Logger) bool {
	if err := config.ValidateConfig(cfg); err != nil {
		if validationErrs, ok := err.(config.ValidationErrors); ok {
			response := convertValidationErrors(validationErrs)
			logger.Warn("config validation failed", "errors", len(response.Errors))
			respondJSON(w, http.StatusUnprocessableEntity, response)
			return false
		}
		// Unexpected error type
		logger.Error("unexpected validation error type", "error", err)
		respondError(w, http.StatusInternalServerError, "validation error")
		return false
	}
	return true
}

// handleValidateConfig validates configuration without saving.
// POST /api/v1/config/validate
func (s *Server) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	var req FullConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load current config as base
	cfg, err := s.loadConfig()
	if err != nil {
		s.logger.Error("failed to load config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load configuration")
		return
	}

	// Apply updates to config (in memory only)
	applyFullConfigUpdates(cfg, &req)

	// Validate
	if err := config.ValidateConfig(cfg); err != nil {
		if validationErrs, ok := err.(config.ValidationErrors); ok {
			response := convertValidationErrors(validationErrs)
			respondJSON(w, http.StatusOK, ValidationResult{
				Valid:  false,
				Errors: response.Errors,
			})
			return
		}
		respondError(w, http.StatusInternalServerError, "validation error")
		return
	}

	respondJSON(w, http.StatusOK, ValidationResult{
		Valid:  true,
		Errors: []ValidationFieldError{},
	})
}
