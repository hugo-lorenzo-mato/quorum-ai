// Package api provides HTTP REST API handlers for the quorum-ai workflow system.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/config"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/project"
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
	ctx := r.Context()

	mode, _ := s.getProjectConfigMode(ctx)
	if mode == project.ConfigModeInheritGlobal {
		respondJSON(w, http.StatusConflict, map[string]interface{}{
			"error": "project inherits global configuration; switch to custom config to validate project settings",
			"code":  "INHERITS_GLOBAL",
		})
		return
	}

	cfg, err := s.loadConfigForContext(ctx)
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

// handleValidateGlobalConfig validates the global configuration without saving.
// POST /api/v1/config/global/validate
func (s *Server) handleValidateGlobalConfig(w http.ResponseWriter, r *http.Request) {
	var req FullConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	globalPath, err := config.EnsureGlobalConfigFile()
	if err != nil {
		s.logger.Error("failed to ensure global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	// Load current global config as base (preserve relative paths for editing).
	cfg, err := config.NewLoader().WithConfigFile(globalPath).WithResolvePaths(false).Load()
	if err != nil {
		s.logger.Error("failed to load global config", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load global configuration")
		return
	}

	applyFullConfigUpdates(cfg, &req)

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

// ValidateBlueprint validates the execution mode configuration in a BlueprintDTO.
// It checks:
// 1. execution_mode is a valid value ("multi_agent", "single_agent", or empty)
// 2. single_agent_name is provided when execution_mode is "single_agent"
// 3. The specified agent exists and is enabled in the application config
//
// Parameters:
//   - bp: The blueprint DTO to validate (can be nil)
//   - agents: The agents configuration from the application config
//
// Returns nil if valid, or a ValidationFieldError describing the validation failure.
func ValidateBlueprint(bp *BlueprintDTO, agents config.AgentsConfig) *ValidationFieldError {
	// Nil blueprint is valid (uses defaults)
	if bp == nil {
		return nil
	}

	// Validate numeric fields (independent of execution mode).
	// Note: These fields are optional and default to 0 when omitted.
	// - consensus_threshold: 0 means "unset", otherwise must be in [0, 1]
	// - max_retries: 0 means "unset" (or "use global"), otherwise must be in [0, 10]
	// - timeout_seconds: 0 means "unset", otherwise must be >= 0
	if bp.ConsensusThreshold < 0 || bp.ConsensusThreshold > 1 {
		return &ValidationFieldError{
			Field:   "consensus_threshold",
			Value:   bp.ConsensusThreshold,
			Message: "must be between 0 and 1",
			Code:    ErrCodeInvalidRange,
		}
	}
	if bp.MaxRetries < 0 || bp.MaxRetries > 10 {
		return &ValidationFieldError{
			Field:   "max_retries",
			Value:   bp.MaxRetries,
			Message: "must be between 0 and 10",
			Code:    ErrCodeInvalidRange,
		}
	}
	if bp.TimeoutSeconds < 0 {
		return &ValidationFieldError{
			Field:   "timeout_seconds",
			Value:   bp.TimeoutSeconds,
			Message: "must be >= 0",
			Code:    ErrCodeInvalidRange,
		}
	}

	// Validate execution_mode value
	mode := strings.TrimSpace(bp.ExecutionMode)
	validModes := map[string]bool{
		"":             true, // Empty defaults to multi-agent
		"multi_agent":  true,
		"single_agent": true,
	}

	if !validModes[mode] {
		return &ValidationFieldError{
			Field:   "execution_mode",
			Value:   mode,
			Message: "invalid value: must be 'multi_agent', 'single_agent', or empty",
			Code:    ErrCodeInvalidEnum,
		}
	}

	// If single-agent mode, validate agent configuration
	if mode == "single_agent" {
		if err := validateSingleAgentBlueprint(bp, agents); err != nil {
			return err
		}
	}

	// Multi-agent mode: single_agent_name is ignored if provided
	// No validation needed for multi-agent specific fields

	return nil
}

// validateSingleAgentBlueprint validates the agent configuration for single-agent mode.
func validateSingleAgentBlueprint(bp *BlueprintDTO, agents config.AgentsConfig) *ValidationFieldError {
	agentName := strings.TrimSpace(bp.SingleAgentName)

	// Agent name is required for single-agent mode
	if agentName == "" {
		return &ValidationFieldError{
			Field:   "single_agent_name",
			Value:   agentName,
			Message: "required when execution_mode is 'single_agent'",
			Code:    ErrCodeRequired,
		}
	}

	// Verify agent exists in configuration
	agentConfig := agents.GetAgentConfig(agentName)
	if agentConfig == nil {
		availableAgents := agents.EnabledAgentNames()
		msg := "agent is not configured"
		if len(availableAgents) > 0 {
			msg = "agent is not configured. Available agents: " + strings.Join(availableAgents, ", ")
		}
		return &ValidationFieldError{
			Field:   "single_agent_name",
			Value:   agentName,
			Message: msg,
			Code:    ErrCodeUnknownAgent,
		}
	}

	// Verify agent is enabled
	if !agentConfig.Enabled {
		enabledAgents := agents.EnabledAgentNames()
		msg := "agent is disabled"
		if len(enabledAgents) > 0 {
			msg = "agent is disabled. Enable it in config or use one of: " + strings.Join(enabledAgents, ", ")
		}
		return &ValidationFieldError{
			Field:   "single_agent_name",
			Value:   agentName,
			Message: msg,
			Code:    ErrCodeAgentNotEnabled,
		}
	}

	// Validate optional reasoning effort override
	effort := strings.TrimSpace(bp.SingleAgentReasoningEffort)
	if effort != "" {
		if !core.SupportsReasoning(agentName) {
			return &ValidationFieldError{
				Field:   "single_agent_reasoning_effort",
				Value:   effort,
				Message: "agent does not support reasoning effort",
				Code:    ErrCodeInvalidEnum,
			}
		}
		if !core.IsValidReasoningEffort(effort) {
			return &ValidationFieldError{
				Field:   "single_agent_reasoning_effort",
				Value:   effort,
				Message: "invalid value: must be none, minimal, low, medium, high, or xhigh",
				Code:    ErrCodeInvalidEnum,
			}
		}
	}

	return nil
}
